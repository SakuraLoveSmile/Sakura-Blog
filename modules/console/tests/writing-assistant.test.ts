// 写作助手状态机：请求的生命周期、停止与取消、错误、应用/撤销、编辑器切换。
// 机械校验入口：make test → console-test → tsx --test ../tests/*.test.ts
import { test } from "node:test";
import assert from "node:assert/strict";
import type { AIAssistEvent, AIAssistRequest } from "../../../packages/api-client/src/generated";
import type {
  AssistantApplyResult,
  AssistantBridge,
  AssistantIntent,
  AssistantSelectionInfo,
  AssistantSnapshot,
} from "../../../packages/editor/src/assistant-bridge.ts";
import { useWritingAssistant } from "../web/src/components/editor/assistant/useWritingAssistant.ts";

interface FakeBridge extends AssistantBridge {
  selection: AssistantSelectionInfo | null;
  snapshot: Partial<AssistantSnapshot>;
  applyResult: AssistantApplyResult;
  applyable: boolean;
  undoable: boolean;
  undoWorks: boolean;
  revertedNext: boolean;
  applied: string[];
  captured: AssistantIntent[];
  focused: number;
  fire(): void;
  subscribers: number;
}

function fakeBridge(patch: Partial<FakeBridge> = {}): FakeBridge {
  const listeners = new Set<() => void>();
  let id = 0;
  const bridge: FakeBridge = {
    mode: "markdown",
    selection: { empty: false, text: "写博客是整理思路的一种方式。", expanded: false },
    snapshot: {},
    applyResult: { ok: true, undoable: true },
    applyable: true,
    undoable: true,
    undoWorks: true,
    revertedNext: false,
    applied: [],
    captured: [],
    focused: 0,
    get subscribers() {
      return listeners.size;
    },
    peek() {
      return this.selection;
    },
    capture(intent) {
      this.captured.push(intent);
      if (!this.selection) return null;
      const replace = intent === "selection" && !this.selection.empty;
      return {
        id: ++id,
        mode: "markdown",
        intent,
        placement: intent === "continue" ? "after-block" : replace ? "replace" : "at-caret",
        text: replace ? this.selection.text : "",
        before: "前文。",
        after: "后文。",
        expanded: false,
        ...this.snapshot,
      };
    },
    canApply() {
      return this.applyable;
    },
    apply(_snapshot, text) {
      this.applied.push(text);
      return this.applyResult;
    },
    canUndo() {
      return this.undoable;
    },
    undo() {
      return this.undoWorks;
    },
    reverted() {
      const was = this.revertedNext;
      this.revertedNext = false;
      return was;
    },
    subscribe(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    focus() {
      this.focused++;
    },
    fire() {
      listeners.forEach((listener) => listener());
    },
    ...patch,
  };
  return bridge;
}

/** 一个由测试手动推进的流：先 begin，再逐个 emit，最后 end。 */
function controlledStream() {
  const requests: AIAssistRequest[] = [];
  const signals: AbortSignal[] = [];
  let emit: (event: AIAssistEvent) => void = () => {};
  let finish: () => void = () => {};
  let failWith: (error: unknown) => void = () => {};
  const stream = (request: AIAssistRequest, options: { signal: AbortSignal; onEvent: (e: AIAssistEvent) => void }) => {
    requests.push(request);
    signals.push(options.signal);
    emit = options.onEvent;
    return new Promise<void>((resolve, reject) => {
      finish = resolve;
      failWith = reject;
      options.signal.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError")));
    });
  };
  return {
    stream,
    requests,
    signals,
    emit: (event: AIAssistEvent) => emit(event),
    end: () => finish(),
    fail: (error: unknown) => failWith(error),
  };
}

function setup(bridgePatch: Partial<FakeBridge> = {}, opts: { markdown?: string } = {}) {
  const s = controlledStream();
  const bridge = fakeBridge(bridgePatch);
  const copied: string[] = [];
  const assistant = useWritingAssistant({
    stream: s.stream,
    getTitle: () => "我的博客",
    getMarkdown: () => opts.markdown ?? "## 为什么写博客\n\n正文\n\n## 工具与习惯\n\n正文",
    copyText: async (text) => {
      copied.push(text);
    },
  });
  assistant.attach(bridge);
  assistant.openPanel();
  return { assistant, bridge, s, copied };
}

const tick = () => new Promise((resolve) => setTimeout(resolve, 5));

async function complete(s: ReturnType<typeof controlledStream>, text: string, extra: Partial<AIAssistEvent> = {}) {
  s.emit({ type: "delta", text });
  s.emit({ type: "done", ...extra });
  s.end();
  await tick();
}

test("assistant: a polish run streams, finishes with a diff and sends the full context", async () => {
  const { assistant, s } = setup();
  const running = assistant.run("polish");
  assert.equal(assistant.status.value, "streaming");
  assert.equal(assistant.action.value, "polish");
  assert.deepEqual(s.requests[0], {
    action: "polish",
    selection: "写博客是整理思路的一种方式。",
    before: "前文。",
    after: "后文。",
    title: "我的博客",
    outline: ["为什么写博客", "工具与习惯"],
  });

  s.emit({ type: "delta", text: "写博客是梳理" });
  assert.equal(assistant.result.value, "写博客是梳理");
  assert.equal(assistant.status.value, "streaming");
  s.emit({ type: "delta", text: "思路的一种方式。" });
  s.emit({ type: "done", finish_reason: "stop" });
  s.end();
  await running;

  assert.equal(assistant.status.value, "done");
  assert.equal(assistant.result.value, "写博客是梳理思路的一种方式。");
  assert.ok(assistant.diff.value, "a replace-style action gets a word diff");
  assert.equal(assistant.view.value, "diff", "a small edit defaults to the diff view");
  assert.equal(assistant.stale.value, false);
  assert.equal(assistant.notice.value, "");
});

test("assistant: translate, custom and continue build the right requests and skip the diff", async () => {
  const translate = setup();
  translate.assistant.targetLang.value = "ja";
  const t = translate.assistant.run("translate");
  assert.equal(translate.s.requests[0].target_lang, "ja");
  await complete(translate.s, "ブログを書く");
  await t;
  assert.equal(translate.assistant.diff.value, null);
  assert.equal(translate.assistant.view.value, "result");

  const custom = setup({ selection: { empty: true, text: "", expanded: false } });
  custom.assistant.instruction.value = "  补一个例子  ";
  const c = custom.assistant.run("custom");
  assert.equal(custom.s.requests[0].instruction, "补一个例子");
  assert.equal(custom.s.requests[0].selection, undefined);
  await complete(custom.s, "比如这样。");
  await c;
  assert.equal(custom.assistant.placement.value, "at-caret");
  assert.equal(custom.assistant.diff.value, null);

  const cont = setup();
  const k = cont.assistant.run("continue");
  assert.equal(cont.s.requests[0].selection, undefined, "continue never sends the selection");
  assert.deepEqual(cont.bridge.captured, ["continue"]);
  await complete(cont.s, "接着往下写。");
  await k;
  assert.equal(cont.assistant.placement.value, "after-block");
  assert.equal(cont.assistant.diff.value, null);
});

test("assistant: blocked actions explain themselves and never call the stream", async () => {
  const { assistant, s } = setup({ selection: { empty: true, text: "", expanded: false } });
  assert.match(assistant.blockedReason("polish"), /选中/);
  assert.equal(assistant.blockedReason("continue"), "");
  assert.match(assistant.blockedReason("custom"), /输入/);
  await assistant.run("polish");
  assert.equal(s.requests.length, 0);
  assert.equal(assistant.status.value, "error");
  assert.match(assistant.errorMessage.value, /选中/);

  const hidden = setup({ selection: null });
  assert.match(hidden.assistant.blockedReason("continue"), /编辑模式/);
  assert.equal(hidden.assistant.selection.value.available, false);
});

test("assistant: stopping keeps what was generated and ignores late events", async () => {
  const { assistant, s } = setup();
  const running = assistant.run("rewrite");
  s.emit({ type: "delta", text: "改写到一半" });
  assistant.stop();
  assert.equal(assistant.status.value, "stopped");
  assert.equal(s.signals[0].aborted, true);
  assert.equal(assistant.result.value, "改写到一半");
  await running; // 取消产生的 AbortError 不能变成错误状态
  s.emit({ type: "delta", text: "迟到的内容" });
  s.emit({ type: "done" });
  assert.equal(assistant.status.value, "stopped");
  assert.equal(assistant.result.value, "改写到一半");
  assert.equal(assistant.errorMessage.value, "");

  // 还没生成任何内容就停止：回到空闲。
  const early = setup();
  const pending = early.assistant.run("polish");
  early.assistant.stop();
  await pending;
  assert.equal(early.assistant.status.value, "idle");
  assert.equal(early.assistant.action.value, null);
});

test("assistant: error events, rejected streams, silent endings and empty output all end in the error state", async () => {
  const eventError = setup();
  const a = eventError.assistant.run("polish");
  eventError.s.emit({ type: "delta", text: "半截" });
  eventError.s.emit({ type: "error", message: "AI 调用失败: 上游超时" });
  eventError.s.end();
  await a;
  assert.equal(eventError.assistant.status.value, "error");
  assert.equal(eventError.assistant.errorMessage.value, "AI 调用失败: 上游超时");
  assert.equal(eventError.assistant.apply(), false, "a failed generation cannot be applied");
  assert.equal(eventError.bridge.applied.length, 0);

  const rejected = setup();
  const b = rejected.assistant.run("polish");
  rejected.s.fail(Object.assign(new Error("写作助手正忙，请稍后再试"), { status: 429 }));
  await b;
  assert.equal(rejected.assistant.status.value, "error");
  assert.equal(rejected.assistant.errorMessage.value, "写作助手正忙，请稍后再试");

  const silent = setup();
  const c = silent.assistant.run("polish");
  silent.s.emit({ type: "delta", text: "没有收尾" });
  silent.s.end();
  await c;
  assert.equal(silent.assistant.status.value, "error");
  assert.match(silent.assistant.errorMessage.value, /中断/);

  const empty = setup();
  const d = empty.assistant.run("polish");
  empty.s.emit({ type: "delta", text: "  \n " });
  empty.s.emit({ type: "done" });
  empty.s.end();
  await d;
  assert.equal(empty.assistant.status.value, "error");
  assert.match(empty.assistant.errorMessage.value, /没有返回内容/);
});

test("assistant: truncation and length stops are flagged", async () => {
  const truncated = setup();
  const a = truncated.assistant.run("expand");
  await complete(truncated.s, "很长的输出", { truncated: true });
  await a;
  assert.match(truncated.assistant.notice.value, /截断.*结果可能不完整/);

  const length = setup();
  const b = length.assistant.run("expand");
  await complete(length.s, "被长度限制", { finish_reason: "length" });
  await b;
  assert.match(length.assistant.notice.value, /长度.*结果可能不完整/);
});

test("assistant: applying marks the result applied, and undo returns to a re-appliable result", async () => {
  const { assistant, s, bridge } = setup();
  const running = assistant.run("polish");
  await complete(s, "\n\n润色后的内容。\n");
  await running;

  assert.equal(bridge.focused, 0);
  assert.equal(assistant.apply(), true);
  assert.deepEqual(bridge.applied, ["润色后的内容。"], "the applied text is normalized");
  assert.equal(assistant.status.value, "applied");
  assert.equal(assistant.canUndo.value, true);
  assert.equal(bridge.focused, 1, "focus goes back to the editor so Ctrl/Cmd+Z and typing land on the body");

  assert.equal(assistant.undo(), true);
  assert.equal(assistant.status.value, "done");
  assert.equal(bridge.focused, 2, "focus returns to the editor after undoing too");
  assert.equal(assistant.apply(), true, "after undoing, the same result can be applied again");
  assert.equal(bridge.applied.length, 2);

  bridge.undoWorks = false;
  assert.equal(assistant.undo(), false);
  assert.match(assistant.applyError.value, /撤销失败/);
  assert.equal(assistant.status.value, "applied");
});

test("assistant: undoing the application with the editor's own undo makes the result appliable again", async () => {
  const { assistant, s, bridge } = setup();
  const running = assistant.run("polish");
  await complete(s, "润色后的内容。");
  await running;
  assert.equal(assistant.apply(), true);
  assert.equal(assistant.status.value, "applied");

  // 文档变了（不再是应用后的样子），但等于应用前：用户按了 Ctrl/⌘+Z。
  bridge.undoable = false;
  bridge.revertedNext = true;
  assistant.refresh();
  assert.equal(assistant.status.value, "done");
  assert.equal(assistant.canUndo.value, false);
  assert.equal(assistant.apply(), true, "the same result can be applied again");
  assert.equal(bridge.applied.length, 2);

  // 文档被继续编辑（不等于应用前）时，保持「已应用」，只是不能一键撤销。
  bridge.undoable = false;
  assistant.refresh();
  assert.equal(assistant.status.value, "applied");
  assert.equal(assistant.canUndo.value, false);
});

test("assistant: an application that cannot be undone natively says so and points to the panel's undo", async () => {
  const { assistant, s, bridge } = setup({ applyResult: { ok: true, undoable: false } });
  const running = assistant.run("polish");
  await complete(s, "润色后的内容。");
  await running;
  assert.equal(assistant.apply(), true);
  assert.equal(assistant.status.value, "applied");
  assert.match(assistant.notice.value, /不能用 Ctrl\/⌘\+Z 撤销.*「撤销」/);

  bridge.applyResult = { ok: true, undoable: true };
  assert.equal(assistant.undo(), true);
  assert.equal(assistant.apply(), true);
  assert.equal(assistant.notice.value, "", "no warning when native undo works");
});

test("assistant: a result whose original text changed is stale and cannot be applied", async () => {
  const { assistant, s, bridge } = setup({ applyable: false });
  const running = assistant.run("polish");
  await complete(s, "润色");
  await running;
  assert.equal(assistant.stale.value, true, "staleness is known as soon as the result is ready");

  // 作者随后又改回来了：刷新后恢复可用。
  bridge.applyable = true;
  assistant.refresh();
  assert.equal(assistant.stale.value, false);

  bridge.applyable = false;
  bridge.applyResult = { ok: false, reason: "stale" };
  assert.equal(assistant.apply(), false);
  assert.equal(assistant.stale.value, true);
  assert.equal(assistant.status.value, "done");

  bridge.applyResult = { ok: false, reason: "failed" };
  assert.equal(assistant.apply(), false);
  assert.match(assistant.applyError.value, /应用失败/);
});

test("assistant: a stopped partial result can still be applied", async () => {
  const { assistant, s, bridge } = setup();
  const running = assistant.run("polish");
  s.emit({ type: "delta", text: "只写了一半" });
  assistant.stop();
  await running;
  assert.equal(assistant.apply(), true);
  assert.deepEqual(bridge.applied, ["只写了一半"]);
});

test("assistant: retry reruns the same action; discard clears the result", async () => {
  const { assistant, s } = setup();
  const first = assistant.run("shorten");
  await complete(s, "精简版");
  await first;
  const second = assistant.retry();
  assert.equal(assistant.status.value, "streaming");
  assert.equal(s.requests.length, 2);
  assert.equal(s.requests[1].action, "shorten");
  assert.equal(assistant.result.value, "", "a retry starts from an empty result");
  await complete(s, "另一个精简版");
  await second;
  assert.equal(assistant.result.value, "另一个精简版");

  assistant.discard();
  assert.equal(assistant.status.value, "idle");
  assert.equal(assistant.result.value, "");
  assert.equal(assistant.diff.value, null);
  assert.equal(await assistant.retry(), undefined, "nothing to retry after a discard");
  assert.equal(s.requests.length, 2);
});

test("assistant: only one run at a time; closing stops a running request and clears an applied one", async () => {
  const { assistant, s } = setup();
  const first = assistant.run("polish");
  await assistant.run("rewrite"); // 生成中再点会被忽略
  assert.equal(s.requests.length, 1);
  assistant.closePanel();
  assert.equal(assistant.open.value, false);
  assert.equal(s.signals[0].aborted, true);
  await first;
  assert.equal(assistant.status.value, "idle", "stopped before any output");

  const applied = setup();
  const run = applied.assistant.run("polish");
  await complete(applied.s, "好了");
  await run;
  applied.assistant.apply();
  applied.assistant.closePanel();
  assert.equal(applied.assistant.status.value, "idle");
  assert.equal(applied.assistant.result.value, "");
});

test("assistant: switching to another editor cancels the run and resets everything", async () => {
  const { assistant, s, bridge } = setup();
  const running = assistant.run("polish");
  s.emit({ type: "delta", text: "进行中" });
  assert.equal(bridge.subscribers, 1);

  const other = fakeBridge();
  assistant.attach(other);
  assert.equal(s.signals[0].aborted, true);
  assert.equal(assistant.status.value, "idle");
  assert.equal(assistant.result.value, "");
  assert.equal(bridge.subscribers, 0, "the old editor is unsubscribed");
  assert.equal(other.subscribers, 1);
  await running;
  s.emit({ type: "delta", text: "旧请求的残留" });
  assert.equal(assistant.result.value, "", "events from the cancelled run are ignored");

  assistant.attach(null);
  assert.equal(other.subscribers, 0);
  assert.equal(assistant.selection.value.available, false);
});

test("assistant: dispose aborts the request and disconnects the editor", async () => {
  const { assistant, s, bridge } = setup();
  const running = assistant.run("polish");
  assistant.dispose();
  assert.equal(s.signals[0].aborted, true);
  assert.equal(bridge.subscribers, 0);
  assert.equal(assistant.open.value, false);
  await running;
});

test("assistant: tag changes in the result raise a warning", async () => {
  const tagged = "前文 {% note info %}提示{% endnote %} 后文";
  const changed = setup({ selection: { empty: false, text: tagged, expanded: false } });
  const a = changed.assistant.run("polish");
  await complete(changed.s, "前文 提示 后文");
  await a;
  assert.equal(changed.assistant.tagWarning.value, true);

  const kept = setup({ selection: { empty: false, text: tagged, expanded: false } });
  const b = kept.assistant.run("polish");
  await complete(kept.s, "前文  {% note info %}提示。{% endnote %}  后文");
  await b;
  assert.equal(kept.assistant.tagWarning.value, false);

  const plain = setup();
  const c = plain.assistant.run("polish");
  await complete(plain.s, "没有标签");
  await c;
  assert.equal(plain.assistant.tagWarning.value, false);
});

test("assistant: copy writes the normalized result through the host and reports success", async () => {
  const { assistant, s, copied } = setup();
  const running = assistant.run("polish");
  await complete(s, "\n结果文本\n");
  await running;
  assert.equal(await assistant.copy(), true);
  assert.deepEqual(copied, ["结果文本"]);
  assert.equal(assistant.copied.value, true);
  assistant.dispose();
});

test("assistant: selection info follows the editor while the panel is open and idles while it is closed", async () => {
  const { assistant, bridge } = setup();
  assert.equal(assistant.selection.value.chars, 14);
  assert.equal(assistant.selection.value.empty, false);

  bridge.selection = { empty: false, text: "换了一段新的选区", expanded: true };
  bridge.fire();
  await tick();
  assert.equal(assistant.selection.value.chars, 8);
  assert.equal(assistant.selection.value.expanded, true);
  assert.equal(assistant.selection.value.excerpt, "换了一段新的选区");

  assistant.closePanel();
  bridge.selection = { empty: true, text: "", expanded: false };
  bridge.fire();
  await tick();
  assert.equal(assistant.selection.value.chars, 8, "no work is done while the panel is closed");
  assistant.openPanel();
  assert.equal(assistant.selection.value.chars, 0, "reopening refreshes immediately");
});
