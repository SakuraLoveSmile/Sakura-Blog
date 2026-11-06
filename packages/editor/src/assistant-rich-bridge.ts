// 富文本（TipTap / ProseMirror）编辑器的写作助手桥接。
// - 选区取成 Markdown：单个文本块内的选区只取行内内容（不带标题、列表等外层标记）；
//   跨块选区先扩到完整的顶层块，应用时整块替换。
// - 应用是一次 ProseMirror 事务并用 closeHistory 另起撤销组，Ctrl/⌘+Z 一步还原，
//   前面的输入不会被连带撤销。
// - 抓取快照后，位置随每个事务一起映射，所以别处的编辑不会让快照失效；
//   应用前还要核对原文没变。
import type { Editor } from "@tiptap/core";
import { closeHistory } from "@tiptap/pm/history";
import { DOMParser as PMDOMParser, Fragment, Slice, type Node as PMNode } from "@tiptap/pm/model";
import type { Transaction } from "@tiptap/pm/state";
import type {
  AssistantApplyResult,
  AssistantBridge,
  AssistantIntent,
  AssistantPlacement,
  AssistantSelectionInfo,
  AssistantSnapshot,
} from "./assistant-bridge";

interface MarkdownStorage {
  serializer: { serialize(content: PMNode): string };
  parser: { parse(content: string, options?: { inline?: boolean }): string };
}

/** 重新核对插入点时，取插入点之前多少个字符的纯文本。 */
const ANCHOR_CHARS = 120;

function markdownOf(editor: Editor): MarkdownStorage {
  return (editor.storage as unknown as { markdown: MarkdownStorage }).markdown;
}

interface Span {
  from: number;
  to: number;
  /** 整个范围在同一个普通文本块内，按行内内容处理。 */
  inline: boolean;
  /** 跨块选区被扩到了完整的顶层块。 */
  expanded: boolean;
}

function isPlainTextblock(node: PMNode): boolean {
  return node.isTextblock && !node.type.spec.code;
}

function inlineWithin(editor: Editor, from: number, to: number): boolean {
  const { doc } = editor.state;
  const $from = doc.resolve(from);
  const $to = doc.resolve(to);
  return $from.sameParent($to) && isPlainTextblock($from.parent);
}

/** 非空选区要处理的范围：文本块内的选区原样，其余扩到完整的顶层块。 */
function selectionSpan(editor: Editor): Span {
  const { doc, selection } = editor.state;
  const { from, to } = selection;
  if (inlineWithin(editor, from, to)) return { from, to, inline: true, expanded: false };
  const $from = doc.resolve(from);
  const $to = doc.resolve(to);
  const start = $from.depth >= 1 ? $from.before(1) : from;
  const end = $to.depth >= 1 ? $to.after(1) : to;
  return { from: start, to: end, inline: false, expanded: start !== from || end !== to };
}

/** 把 [from, to] 序列化成 Markdown；inline 时包进一个段落，只取行内内容。 */
function serializeRange(editor: Editor, from: number, to: number, inline: boolean): string {
  const { doc, schema } = editor.state;
  const content = doc.slice(from, to).content;
  const body = inline ? Fragment.from(schema.nodes.paragraph.create(null, content)) : content;
  return markdownOf(editor).serializer.serialize(schema.topNodeType.create(null, body));
}

function plainTail(editor: Editor, pos: number): string {
  return editor.state.doc.textBetween(Math.max(0, pos - ANCHOR_CHARS), pos, "\n", "\n");
}

/** Markdown → 文档节点（块级解析）。不经过 insertContentAt，避免它按行内解析并拆掉首段。 */
function parseMarkdown(editor: Editor, text: string): PMNode {
  const html = markdownOf(editor).parser.parse(text);
  const body = new window.DOMParser().parseFromString(`<body>${html}</body>`, "text/html").body;
  return PMDOMParser.fromSchema(editor.schema).parse(body, { preserveWhitespace: false });
}

interface Tracked {
  id: number;
  placement: AssistantPlacement;
  from: number;
  to: number;
  inline: boolean;
  /** 抓取时插入点之前的纯文本，核对 at-caret 与 after-block 的位置还对不对。 */
  tail: string;
}

export interface RichAssistantBridge extends AssistantBridge {
  /** 组件卸载时调用，解除对编辑器事件的订阅。 */
  dispose(): void;
}

export function createRichAssistantBridge(getEditor: () => Editor | undefined): RichAssistantBridge {
  let nextId = 0;
  let tracked: Tracked | null = null;
  /**
   * 最近一次应用：应用后的文档（判断之后有没有再被改动）、应用前的文档（判断用户是否自己撤销回去了）
   * 与应用前的位置（撤销后还原）。
   */
  let applied: { doc: PMNode; docBefore: PMNode; before: Tracked } | null = null;
  let hooked: Editor | null = null;
  const listeners = new Set<() => void>();

  function live(): Editor | null {
    const editor = getEditor();
    return editor && !editor.isDestroyed ? editor : null;
  }

  function onTransaction({ transaction }: { transaction: Transaction }) {
    if (tracked && transaction.docChanged) {
      const map = transaction.mapping;
      // 范围两端各自偏向范围外：紧贴着选区的输入不算选区的一部分。
      if (tracked.placement === "replace") {
        tracked.from = map.map(tracked.from, 1);
        tracked.to = Math.max(tracked.from, map.map(tracked.to, -1));
      } else {
        tracked.from = tracked.to = map.map(tracked.from, -1);
      }
    }
    listeners.forEach((listener) => listener());
  }

  function ensureHooked(): Editor | null {
    const editor = live();
    if (editor && editor !== hooked) {
      hooked?.off("transaction", onTransaction);
      editor.on("transaction", onTransaction);
      hooked = editor;
    }
    return editor;
  }

  /** 位置还在文档里，并且原文（或插入点之前的文字）与抓取时一致。 */
  function isCurrent(editor: Editor, snapshot: AssistantSnapshot): Tracked | null {
    if (!tracked || tracked.id !== snapshot.id) return null;
    const size = editor.state.doc.content.size;
    const { from, to } = tracked;
    if (from < 0 || to > size || from > to) return null;
    try {
      if (tracked.placement === "replace") {
        return serializeRange(editor, from, to, tracked.inline) === snapshot.text ? tracked : null;
      }
      if (plainTail(editor, from) !== tracked.tail) return null;
      const $pos = editor.state.doc.resolve(from);
      if (tracked.placement === "after-block" ? $pos.depth !== 0 : !isPlainTextblock($pos.parent)) return null;
      return tracked;
    } catch {
      return null;
    }
  }

  /** 行内插入：单个段落取其行内内容；多个段落像粘贴一样拆开当前块。 */
  function replaceInline(tr: Transaction, from: number, to: number, parsed: PMNode) {
    if (parsed.childCount === 1 && parsed.firstChild && parsed.firstChild.isTextblock) {
      tr.replaceWith(from, to, parsed.firstChild.content);
      return;
    }
    let onlyTextblocks = true;
    parsed.forEach((child) => {
      if (!child.isTextblock) onlyTextblocks = false;
    });
    const open = onlyTextblocks ? 1 : 0;
    tr.replaceRange(from, to, new Slice(parsed.content, open, open));
  }

  return {
    mode: "rich",

    peek(): AssistantSelectionInfo | null {
      const editor = ensureHooked();
      if (!editor) return null;
      if (editor.state.selection.empty) return { empty: true, text: "", expanded: false };
      const span = selectionSpan(editor);
      return { empty: false, text: serializeRange(editor, span.from, span.to, span.inline), expanded: span.expanded };
    },

    capture(intent: AssistantIntent): AssistantSnapshot | null {
      const editor = ensureHooked();
      if (!editor) return null;
      const { doc, selection } = editor.state;
      let placement: AssistantPlacement;
      let from: number;
      let to: number;
      let inline = true;
      let expanded = false;
      let text = "";
      if (intent === "continue") {
        const $to = doc.resolve(selection.to);
        placement = "after-block";
        from = to = $to.depth >= 1 ? $to.after(1) : selection.to;
      } else if (selection.empty) {
        placement = "at-caret";
        from = to = selection.from;
      } else {
        const span = selectionSpan(editor);
        placement = "replace";
        ({ from, to, inline, expanded } = span);
        text = serializeRange(editor, from, to, inline);
      }
      const id = ++nextId;
      tracked = { id, placement, from, to, inline, tail: placement === "replace" ? "" : plainTail(editor, from) };
      return {
        id,
        mode: "rich",
        intent,
        placement,
        text,
        before: serializeRange(editor, 0, from, false),
        after: serializeRange(editor, to, doc.content.size, false),
        expanded,
      };
    },

    canApply(snapshot: AssistantSnapshot): boolean {
      const editor = live();
      return editor !== null && isCurrent(editor, snapshot) !== null;
    },

    apply(snapshot: AssistantSnapshot, text: string): AssistantApplyResult {
      const editor = live();
      if (!editor) return { ok: false, reason: "unavailable" };
      const target = isCurrent(editor, snapshot);
      if (!target) return { ok: false, reason: "stale" };

      const tr = editor.state.tr;
      try {
        const parsed = parseMarkdown(editor, text);
        if (parsed.childCount === 0) return { ok: false, reason: "failed" };
        if (target.placement === "after-block") tr.insert(target.from, parsed.content);
        else if (target.placement === "at-caret") replaceInline(tr, target.from, target.from, parsed);
        else if (target.inline) replaceInline(tr, target.from, target.to, parsed);
        else tr.replaceWith(target.from, target.to, parsed.content);
      } catch {
        return { ok: false, reason: "failed" };
      }
      if (!tr.docChanged) return { ok: false, reason: "failed" };

      // 这一步自成一个撤销组，之后的输入也不会并进来。
      closeHistory(tr);
      const before = { ...target };
      const docBefore = editor.state.doc;
      editor.view.dispatch(tr.scrollIntoView());
      editor.view.dispatch(closeHistory(editor.state.tr));
      applied = { doc: editor.state.doc, docBefore, before };
      return { ok: true, undoable: true };
    },

    canUndo(): boolean {
      const editor = live();
      return editor !== null && applied !== null && editor.state.doc === applied.doc;
    },

    undo(): boolean {
      const editor = live();
      if (!editor || !applied || editor.state.doc !== applied.doc) return false;
      const { before } = applied;
      applied = null;
      if (!editor.commands.undo()) return false;
      // 文档已回到应用前的样子，位置也原样还原，这份快照可以再次应用。
      tracked = { ...before };
      return true;
    },

    reverted(): boolean {
      const editor = live();
      if (!editor || !applied || !editor.state.doc.eq(applied.docBefore)) return false;
      // 用户用编辑器自带的撤销退回去了：位置跟着还原，这份快照可以再次应用。
      tracked = { ...applied.before };
      applied = null;
      return true;
    },

    subscribe(listener: () => void): () => void {
      listeners.add(listener);
      ensureHooked();
      return () => {
        listeners.delete(listener);
      };
    },

    focus() {
      live()?.commands.focus();
    },

    dispose() {
      hooked?.off("transaction", onTransaction);
      hooked = null;
      listeners.clear();
      tracked = null;
      applied = null;
    },
  };
}
