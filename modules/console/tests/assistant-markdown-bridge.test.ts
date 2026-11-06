// 写作助手的 Markdown（<textarea>）桥接：抓取快照、原生插入与撤销、写回降级、编辑后重新定位。
// 机械校验入口：make test → console-test → tsx --test ../tests/*.test.ts
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  createMarkdownBridge,
  type TextareaLike,
} from "../web/src/components/editor/assistant/markdown-bridge.ts";

class FakeTextarea implements TextareaLike {
  value: string;
  selectionStart = 0;
  selectionEnd = 0;
  isConnected = true;
  visible = true;
  focused = false;
  listeners = new Map<string, Set<() => void>>();

  constructor(value: string) {
    this.value = value;
  }
  focus() {
    if (this.visible) this.focused = true;
  }
  setSelectionRange(start: number, end: number) {
    this.selectionStart = start;
    this.selectionEnd = end;
  }
  getClientRects() {
    return { length: this.visible ? 1 : 0 };
  }
  addEventListener(type: string, listener: () => void) {
    if (!this.listeners.has(type)) this.listeners.set(type, new Set());
    this.listeners.get(type)!.add(listener);
  }
  removeEventListener(type: string, listener: () => void) {
    this.listeners.get(type)?.delete(listener);
  }
  select(from: number, to = from) {
    this.selectionStart = from;
    this.selectionEnd = to;
  }
  listenerCount() {
    return [...this.listeners.values()].reduce((sum, set) => sum + set.size, 0);
  }
}

function setup(value: string, { native = true }: { native?: boolean } = {}) {
  const ta = new FakeTextarea(value);
  const undoStack: string[] = [];
  const writes: Array<{ next: string; from: number; to: number }> = [];
  const bridge = createMarkdownBridge({
    getTextarea: () => ta,
    setValue: (next, from, to) => {
      writes.push({ next, from, to });
      ta.value = next;
      ta.select(from, to);
    },
    isFocused: (el) => (el as FakeTextarea).focused,
    insertText: native
      ? (el, text) => {
          const t = el as FakeTextarea;
          undoStack.push(t.value);
          t.value = t.value.slice(0, t.selectionStart) + text + t.value.slice(t.selectionEnd);
          t.select(t.selectionStart + text.length);
          return true;
        }
      : () => false,
    undoNative: () => {
      const previous = undoStack.pop();
      if (previous === undefined) return false;
      ta.value = previous;
      return true;
    },
  });
  return { ta, bridge, undoStack, writes };
}

const doc = "# 标题\n\n第一段。\n\n第二段第一行\n第二段第二行\n\n第三段";

test("markdown bridge: peek reports the selection and is unavailable when the surface is hidden", () => {
  const { ta, bridge } = setup(doc);
  ta.select(doc.indexOf("第一段"), doc.indexOf("第一段") + 4);
  assert.deepEqual(bridge.peek(), { empty: false, text: "第一段。", expanded: false });
  ta.select(3);
  assert.deepEqual(bridge.peek(), { empty: true, text: "", expanded: false });
  ta.visible = false; // 仅预览模式：textarea 是 display:none
  assert.equal(bridge.peek(), null);
  assert.equal(bridge.capture("selection"), null);
  ta.visible = true;
  ta.isConnected = false;
  assert.equal(bridge.peek(), null);
});

test("markdown bridge: capture builds replace, at-caret and continue snapshots", () => {
  const { ta, bridge } = setup(doc);
  const from = doc.indexOf("第一段");
  ta.select(from, from + 4);
  const replace = bridge.capture("selection");
  assert.ok(replace);
  assert.equal(replace.placement, "replace");
  assert.equal(replace.text, "第一段。");
  assert.equal(replace.before, doc.slice(0, from));
  assert.equal(replace.after, doc.slice(from + 4));

  ta.select(from + 2);
  const caret = bridge.capture("selection");
  assert.ok(caret);
  assert.equal(caret.placement, "at-caret");
  assert.equal(caret.text, "");
  assert.equal(caret.before + caret.after, doc);

  // 续写：选区（或光标）所在块的末尾是起点，前文含整段，后文从空行开始。
  ta.select(doc.indexOf("第二段第一行") + 2);
  const cont = bridge.capture("continue");
  assert.ok(cont);
  assert.equal(cont.placement, "after-block");
  assert.equal(cont.text, "");
  assert.ok(cont.before.endsWith("第二段第二行"));
  assert.ok(cont.after.startsWith("\n\n第三段"));
  assert.ok(cont.id > caret.id && caret.id > replace.id);
});

test("markdown bridge: replace goes through native insert, selects the new text and undoes natively", () => {
  const { ta, bridge, undoStack, writes } = setup(doc);
  const from = doc.indexOf("第一段");
  ta.select(from, from + 4);
  const snap = bridge.capture("selection")!;
  const outcome = bridge.apply(snap, "润色后的第一段。");
  assert.deepEqual(outcome, { ok: true, undoable: true });
  assert.equal(ta.value, doc.replace("第一段。", "润色后的第一段。"));
  assert.equal(writes.length, 0, "native insert must not fall back to writing the value");
  assert.equal(undoStack.length, 1);
  assert.equal(ta.value.slice(ta.selectionStart, ta.selectionEnd), "润色后的第一段。");

  assert.equal(bridge.canUndo(), true);
  assert.equal(bridge.undo(), true);
  assert.equal(ta.value, doc);
  assert.equal(bridge.canUndo(), false);
  assert.equal(bridge.undo(), false);
});

test("markdown bridge: whitespace around the selection survives the replacement", () => {
  const { ta, bridge } = setup("甲\n\n乙段落\n\n丙");
  const from = "甲\n\n".length;
  ta.select(from - 1, from + 3); // 选中了前面的换行和「乙段落」
  const snap = bridge.capture("selection")!;
  assert.equal(snap.text, "\n乙段落");
  bridge.apply(snap, "新的乙");
  assert.equal(ta.value, "甲\n\n新的乙\n\n丙");

  const blank = setup("a   b");
  blank.ta.select(1, 4);
  const spaces = blank.bridge.capture("selection")!;
  blank.bridge.apply(spaces, "X");
  assert.equal(blank.ta.value, "a   Xb", "a whitespace-only selection is kept, not doubled");
});

test("markdown bridge: continue inserts a new paragraph after the block; an empty document gets no leading gap", () => {
  const { ta, bridge } = setup(doc);
  ta.select(doc.indexOf("第二段第一行"));
  const snap = bridge.capture("continue")!;
  bridge.apply(snap, "续写出来的一段。");
  assert.equal(ta.value, doc.replace("第二段第二行\n\n", "第二段第二行\n\n续写出来的一段。\n\n"));

  const empty = setup("");
  const first = empty.bridge.capture("selection")!;
  assert.equal(first.placement, "at-caret");
  empty.bridge.apply(first, "从零开始");
  assert.equal(empty.ta.value, "从零开始");

  const onlyTitle = setup("# 只有标题");
  onlyTitle.ta.select(2);
  const continued = onlyTitle.bridge.capture("continue")!;
  onlyTitle.bridge.apply(continued, "正文开头");
  assert.equal(onlyTitle.ta.value, "# 只有标题\n\n正文开头");
});

test("markdown bridge: at-caret inserts at the cursor", () => {
  const { ta, bridge } = setup("前文后文");
  ta.select(2);
  const snap = bridge.capture("selection")!;
  bridge.apply(snap, "【插入】");
  assert.equal(ta.value, "前文【插入】后文");
});

test("markdown bridge: edits elsewhere are tolerated; the selected text changing or vanishing is stale", () => {
  const { ta, bridge } = setup(doc);
  const from = doc.indexOf("第一段");
  ta.select(from, from + 4);
  const snap = bridge.capture("selection")!;
  assert.equal(bridge.canApply(snap), true);

  // 上方插入内容，位置整体后移：唯一出现时仍能定位。
  ta.value = "新增的开头\n\n" + doc;
  assert.equal(bridge.canApply(snap), true);
  assert.deepEqual(bridge.apply(snap, "改后"), { ok: true, undoable: true });
  assert.equal(ta.value, "新增的开头\n\n" + doc.replace("第一段。", "改后"));

  // 选中的文字被改了：不能再应用。
  const second = setup(doc);
  second.ta.select(from, from + 4);
  const edited = second.bridge.capture("selection")!;
  second.ta.value = doc.replace("第一段。", "第一段，已被作者改过。");
  assert.equal(second.bridge.canApply(edited), false);
  assert.deepEqual(second.bridge.apply(edited, "x"), { ok: false, reason: "stale" });
  assert.equal(second.ta.value, doc.replace("第一段。", "第一段，已被作者改过。"), "a stale apply changes nothing");

  // 出现了第二处一样的文字：无法确定是哪一处。
  const third = setup("相同的话。\n\n别的。");
  third.ta.select(0, 5);
  const ambiguous = third.bridge.capture("selection")!;
  third.ta.value = "别的。\n\n相同的话。\n\n相同的话。";
  assert.equal(third.bridge.canApply(ambiguous), false);
});

test("markdown bridge: a continue snapshot follows its block when text is added above", () => {
  const { ta, bridge } = setup(doc);
  ta.select(doc.indexOf("第一段"));
  const snap = bridge.capture("continue")!;
  ta.value = "开头新增\n\n" + doc;
  assert.equal(bridge.canApply(snap), true);
  bridge.apply(snap, "补一段");
  assert.equal(ta.value, "开头新增\n\n" + doc.replace("第一段。\n\n", "第一段。\n\n补一段\n\n"));
});

test("markdown bridge: without native insert it writes the value and undoes by writing it back", () => {
  const { ta, bridge, writes } = setup(doc, { native: false });
  const from = doc.indexOf("第一段");
  ta.select(from, from + 4);
  const snap = bridge.capture("selection")!;
  const outcome = bridge.apply(snap, "改后。");
  assert.deepEqual(outcome, { ok: true, undoable: false });
  assert.equal(writes.length, 1);
  assert.deepEqual(writes[0], { next: doc.replace("第一段。", "改后。"), from, to: from + 3 });

  assert.equal(bridge.canUndo(), true);
  assert.equal(bridge.undo(), true);
  assert.equal(ta.value, doc);
  assert.deepEqual(writes[1], { next: doc, from, to: from + 4 });
});

test("markdown bridge: never inserts natively when the textarea cannot take focus", () => {
  const { ta, bridge, writes, undoStack } = setup(doc);
  const from = doc.indexOf("第一段");
  ta.select(from, from + 4);
  const snap = bridge.capture("selection")!;
  ta.visible = false; // 抓快照之后切到了仅预览：focus() 不会生效
  const outcome = bridge.apply(snap, "改后。");
  assert.deepEqual(outcome, { ok: true, undoable: false });
  assert.equal(undoStack.length, 0, "execCommand would have typed into whatever else had focus");
  assert.equal(writes.length, 1);
  assert.equal(ta.value, doc.replace("第一段。", "改后。"));
});

test("markdown bridge: canUndo goes false once the document changes after applying", () => {
  const { ta, bridge } = setup("甲乙丙");
  ta.select(0, 1);
  const snap = bridge.capture("selection")!;
  bridge.apply(snap, "X");
  assert.equal(bridge.canUndo(), true);
  ta.value = ta.value + "，作者又写了一点";
  assert.equal(bridge.canUndo(), false);
  assert.equal(bridge.undo(), false);
});

test("markdown bridge: reverted notices the user undoing the application (once), and the snapshot can be applied again", () => {
  const { ta, bridge, undoStack } = setup(doc);
  const from = doc.indexOf("第一段");
  ta.select(from, from + 4);
  const snap = bridge.capture("selection")!;
  assert.equal(bridge.reverted!(), false, "nothing has been applied yet");

  assert.deepEqual(bridge.apply(snap, "改写后。"), { ok: true, undoable: true });
  assert.equal(bridge.reverted!(), false, "the applied text is still in the document");

  // 用户自己按 Ctrl/⌘+Z：浏览器的撤销把正文退回应用前的样子。
  ta.value = undoStack.pop()!;
  assert.equal(bridge.canUndo(), false);
  assert.equal(bridge.reverted!(), true);
  assert.equal(bridge.reverted!(), false, "reported only once per application");
  assert.equal(bridge.canApply(snap), true);
  assert.deepEqual(bridge.apply(snap, "再改一次。"), { ok: true, undoable: true });
  assert.ok(ta.value.includes("再改一次。"));
});

test("markdown bridge: subscribe attaches to the textarea and unsubscribes cleanly", () => {
  const { ta, bridge } = setup(doc);
  let calls = 0;
  const unsubscribe = bridge.subscribe(() => calls++);
  assert.ok(ta.listenerCount() >= 5);
  ta.listeners.get("input")?.forEach((listener) => listener());
  ta.listeners.get("select")?.forEach((listener) => listener());
  assert.equal(calls, 2);
  unsubscribe();
  assert.equal(ta.listenerCount(), 0);
});

test("markdown bridge: a snapshot is only valid for the most recent capture", () => {
  const { ta, bridge } = setup(doc);
  ta.select(0, 2);
  const first = bridge.capture("selection")!;
  ta.select(4, 6);
  const second = bridge.capture("selection")!;
  assert.equal(bridge.canApply(first), false);
  assert.deepEqual(bridge.apply(first, "x"), { ok: false, reason: "stale" });
  assert.equal(bridge.canApply(second), true);
});
