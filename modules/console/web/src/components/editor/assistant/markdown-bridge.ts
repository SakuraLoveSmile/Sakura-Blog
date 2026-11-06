// Markdown（<textarea>）编辑器的写作助手桥接。
// 选区就是字符偏移；应用时优先用 execCommand("insertText")，这样结果进入浏览器原生撤销栈，
// Ctrl/⌘+Z 一步还原；做不到时才直接写回值（此时只能用面板上的「撤销」）。
import type {
  AssistantApplyResult,
  AssistantBridge,
  AssistantIntent,
  AssistantPlacement,
  AssistantSelectionInfo,
  AssistantSnapshot,
} from "@blog/editor/assistant-bridge";
import { blockEnd, lastChars, relocate, relocateAfter } from "./assistant-core";

/** <textarea> 里这里用到的最小接口，测试里用替身。 */
export interface TextareaLike {
  value: string;
  selectionStart: number;
  selectionEnd: number;
  isConnected: boolean;
  focus(): void;
  setSelectionRange(start: number, end: number): void;
  getClientRects(): { length: number };
  addEventListener(type: string, listener: () => void): void;
  removeEventListener(type: string, listener: () => void): void;
}

export interface MarkdownBridgeOptions {
  getTextarea(): TextareaLike | null;
  /** 通过宿主写入新值（走 v-model，触发草稿与未保存标记），并把选区放到 [selStart, selEnd]。 */
  setValue(next: string, selStart: number, selEnd: number): void;
  /** 原生插入，保留浏览器撤销栈；返回是否成功。默认用 document.execCommand。 */
  insertText?(ta: TextareaLike, text: string): boolean;
  /** 原生撤销；返回是否成功。 */
  undoNative?(): boolean;
  isFocused?(ta: TextareaLike): boolean;
}

/** 编辑后重新定位用的前文长度。 */
const LOCATOR_CHARS = 80;

const SURFACE_EVENTS = ["select", "keyup", "mouseup", "touchend", "input", "focus", "blur"];

function nativeInsert(_ta: TextareaLike, text: string): boolean {
  try {
    return document.execCommand("insertText", false, text);
  } catch {
    return false;
  }
}

function nativeUndo(): boolean {
  try {
    return document.execCommand("undo");
  } catch {
    return false;
  }
}

function defaultIsFocused(ta: TextareaLike): boolean {
  return document.activeElement === (ta as unknown as Element);
}

interface Anchor {
  from: number;
  to: number;
  /** 抓取快照时的全文，没变过就直接用 from/to。 */
  value: string;
}

interface AppliedRecord {
  valueBefore: string;
  valueAfter: string;
  native: boolean;
  /** 应用前被替换的范围，撤销时用来还原选区。 */
  from: number;
  to: number;
}

export function createMarkdownBridge(options: MarkdownBridgeOptions): AssistantBridge {
  let nextId = 0;
  let anchor: { id: number; at: Anchor } | null = null;
  let applied: AppliedRecord | null = null;

  /** 编辑面可用：已挂载且可见（仅预览模式下 textarea 是 display:none）。 */
  function surface(): TextareaLike | null {
    const ta = options.getTextarea();
    return ta && ta.isConnected && ta.getClientRects().length > 0 ? ta : null;
  }

  function locate(ta: TextareaLike, snapshot: AssistantSnapshot): { from: number; to: number } | null {
    if (!anchor || anchor.id !== snapshot.id) return null;
    const { from, to, value: captured } = anchor.at;
    const value = ta.value;
    if (value === captured) return { from, to };
    if (snapshot.placement === "replace") {
      const at = relocate(value, snapshot.text, from);
      return at < 0 ? null : { from: at, to: at + snapshot.text.length };
    }
    const at = relocateAfter(value, lastChars(snapshot.before, LOCATOR_CHARS), from);
    return at < 0 ? null : { from: at, to: at };
  }

  /** 原生插入并确认结果；失败时调用方改走直接写值。 */
  function insertNatively(ta: TextareaLike, from: number, to: number, insert: string, expected: string): boolean {
    ta.focus();
    // 聚焦失败（例如 textarea 被隐藏）时，execCommand 会把文字插进别的输入框。
    if (!(options.isFocused ?? defaultIsFocused)(ta)) return false;
    ta.setSelectionRange(from, to);
    if (!(options.insertText ?? nativeInsert)(ta, insert)) return false;
    return ta.value === expected;
  }

  return {
    mode: "markdown",

    peek(): AssistantSelectionInfo | null {
      const ta = surface();
      if (!ta) return null;
      const { selectionStart: start, selectionEnd: end } = ta;
      return { empty: start === end, text: ta.value.slice(start, end), expanded: false };
    },

    capture(intent: AssistantIntent): AssistantSnapshot | null {
      const ta = surface();
      if (!ta) return null;
      const value = ta.value;
      const start = ta.selectionStart;
      const end = ta.selectionEnd;
      let placement: AssistantPlacement;
      let text = "";
      let from: number;
      let to: number;
      if (intent === "continue") {
        placement = "after-block";
        from = to = blockEnd(value, end);
      } else if (start === end) {
        placement = "at-caret";
        from = to = start;
      } else {
        placement = "replace";
        from = start;
        to = end;
        text = value.slice(start, end);
      }
      const id = ++nextId;
      anchor = { id, at: { from, to, value } };
      return {
        id,
        mode: "markdown",
        intent,
        placement,
        text,
        before: value.slice(0, from),
        after: value.slice(to),
        expanded: false,
      };
    },

    canApply(snapshot: AssistantSnapshot): boolean {
      const ta = options.getTextarea();
      return ta !== null && locate(ta, snapshot) !== null;
    },

    apply(snapshot: AssistantSnapshot, text: string): AssistantApplyResult {
      const ta = options.getTextarea();
      if (!ta) return { ok: false, reason: "unavailable" };
      const spot = locate(ta, snapshot);
      if (!spot) return { ok: false, reason: "stale" };

      const value = ta.value;
      let lead = "";
      let trail = "";
      if (snapshot.placement === "replace") {
        // 保留选区首尾的空白（例如选中时带上的换行），避免相邻的行被并到一起。
        const original = value.slice(spot.from, spot.to);
        if (original.trim() === "") {
          lead = original;
        } else {
          lead = /^\s*/.exec(original)?.[0] ?? "";
          trail = /\s*$/.exec(original)?.[0] ?? "";
        }
      } else if (snapshot.placement === "after-block" && value.slice(0, spot.from).trim() !== "") {
        lead = "\n\n";
      }
      const insert = lead + text + trail;
      const next = value.slice(0, spot.from) + insert + value.slice(spot.to);
      const selStart = spot.from + lead.length;
      const selEnd = selStart + text.length;

      const native = insertNatively(ta, spot.from, spot.to, insert, next);
      if (native) ta.setSelectionRange(selStart, selEnd);
      else options.setValue(next, selStart, selEnd);
      applied = { valueBefore: value, valueAfter: next, native, from: spot.from, to: spot.to };
      return { ok: true, undoable: native };
    },

    canUndo(): boolean {
      const ta = options.getTextarea();
      return ta !== null && applied !== null && ta.value === applied.valueAfter;
    },

    undo(): boolean {
      const ta = options.getTextarea();
      const record = applied;
      if (!ta || !record || ta.value !== record.valueAfter) return false;
      applied = null;
      if (record.native) {
        ta.focus();
        if ((options.undoNative ?? nativeUndo)() && ta.value === record.valueBefore) return true;
      }
      // 原生撤销不可用或没能还原：直接写回应用前的内容。
      options.setValue(record.valueBefore, record.from, record.to);
      return true;
    },

    reverted(): boolean {
      const ta = options.getTextarea();
      if (!ta || !applied || ta.value !== applied.valueBefore) return false;
      applied = null;
      return true;
    },

    subscribe(listener: () => void): () => void {
      const ta = options.getTextarea();
      if (ta) for (const type of SURFACE_EVENTS) ta.addEventListener(type, listener);
      const doc = typeof document === "undefined" ? null : document;
      doc?.addEventListener("selectionchange", listener);
      return () => {
        if (ta) for (const type of SURFACE_EVENTS) ta.removeEventListener(type, listener);
        doc?.removeEventListener("selectionchange", listener);
      };
    },

    focus() {
      options.getTextarea()?.focus();
    },
  };
}
