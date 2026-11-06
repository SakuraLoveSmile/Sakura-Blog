// 写作助手的状态机：选区概况，以及一次请求从流式生成到完成/停止/出错、应用/撤销/放弃的生命周期。
// 不依赖 DOM：编辑器通过 AssistantBridge 接入，请求通过注入的 stream 函数发出，便于测试。
import { reactive, ref, shallowRef, type UnwrapNestedRefs } from "vue";
import type { AIAssistEvent, AIAssistRequest } from "@blog/api-client";
import type { AssistantBridge, AssistantPlacement, AssistantSnapshot } from "@blog/editor/assistant-bridge";
import {
  assistBlockedReason,
  buildAssistRequest,
  countChars,
  diffApplicable,
  excerpt,
  extractOutline,
  normalizeResult,
  preferredView,
  shortcodeTags,
  shortcodesChanged,
  type AssistAction,
  type AssistView,
} from "./assistant-core";
import { buildInlineDiff, type InlineDiff } from "./inline-diff";

export type AssistantStatus = "idle" | "streaming" | "done" | "stopped" | "error" | "applied";

export interface SelectionView {
  /** 能取到编辑面（Markdown 仅预览时不能）。 */
  available: boolean;
  empty: boolean;
  chars: number;
  excerpt: string;
  /** 富文本跨块选区被扩到完整的块。 */
  expanded: boolean;
}

export interface AssistantDeps {
  stream(
    request: AIAssistRequest,
    options: { signal: AbortSignal; onEvent: (event: AIAssistEvent) => void },
  ): Promise<void>;
  getTitle(): string;
  getMarkdown(): string;
  copyText?(text: string): Promise<void>;
}

export function emptySelection(): SelectionView {
  return { available: false, empty: true, chars: 0, excerpt: "", expanded: false };
}

const COPIED_MS = 1800;

function messageOf(error: unknown): string {
  if (error instanceof Error && error.message.trim() !== "") return error.message;
  return "写作助手出错了，请重试";
}

function schedule(fn: () => void): void {
  if (typeof requestAnimationFrame === "function") requestAnimationFrame(fn);
  else setTimeout(fn, 0);
}

export function useWritingAssistant(deps: AssistantDeps) {
  const open = ref(false);
  const status = ref<AssistantStatus>("idle");
  const action = ref<AssistAction | null>(null);
  const placement = ref<AssistantPlacement | null>(null);
  const result = ref("");
  const errorMessage = ref("");
  /** 完成时的提示：输出被截断、模型因长度提前结束。 */
  const notice = ref("");
  /** 应用失败或撤销失败的提示。 */
  const applyError = ref("");
  const instruction = ref("");
  const targetLang = ref("en");
  const view = ref<AssistView>("result");
  const diff = shallowRef<InlineDiff | null>(null);
  const selection = ref<SelectionView>(emptySelection());
  /** 结果对应的原文已经变了，不能再应用。 */
  const stale = ref(false);
  const canUndo = ref(false);
  const copied = ref(false);
  /** 结果里的标签（{% … %}）与原文不一致。 */
  const tagWarning = ref(false);

  let bridge: AssistantBridge | null = null;
  let unsubscribe: (() => void) | null = null;
  let snapshot: AssistantSnapshot | null = null;
  let controller: AbortController | null = null;
  let runId = 0;
  let refreshQueued = false;
  let copiedTimer: ReturnType<typeof setTimeout> | undefined;

  // ---------- 选区与状态刷新 ----------

  function refresh() {
    const info = bridge?.peek() ?? null;
    selection.value = info
      ? {
          available: true,
          empty: info.empty,
          chars: countChars(info.text),
          excerpt: excerpt(info.text),
          expanded: info.expanded,
        }
      : emptySelection();
    // 用户自己按 Ctrl/⌘+Z 退回了这次应用：结果回到「可以再次应用」。
    if (status.value === "applied" && bridge && snapshot && !bridge.canUndo() && bridge.reverted?.()) {
      status.value = "done";
    }
    if (bridge && snapshot && (status.value === "done" || status.value === "stopped")) {
      stale.value = !bridge.canApply(snapshot);
    }
    canUndo.value = status.value === "applied" && bridge !== null && bridge.canUndo();
  }

  /** 编辑器的选区与内容变化会连续触发，合并成一帧刷新一次；面板关着时不做事。 */
  function scheduleRefresh() {
    if (!open.value || refreshQueued) return;
    refreshQueued = true;
    schedule(() => {
      refreshQueued = false;
      if (open.value) refresh();
    });
  }

  function resetResult() {
    status.value = "idle";
    action.value = null;
    placement.value = null;
    result.value = "";
    errorMessage.value = "";
    notice.value = "";
    applyError.value = "";
    diff.value = null;
    stale.value = false;
    canUndo.value = false;
    tagWarning.value = false;
    snapshot = null;
  }

  function cancelRun() {
    runId++;
    controller?.abort();
    controller = null;
  }

  // ---------- 与编辑器的连接 ----------

  function detach() {
    unsubscribe?.();
    unsubscribe = null;
    cancelRun();
    bridge = null;
    resetResult();
    selection.value = emptySelection();
  }

  /** 接入编辑器；换成另一个编辑器（或离开）时，进行中的请求会被取消，结果清空。 */
  function attach(next: AssistantBridge | null) {
    if (next === bridge) return;
    detach();
    bridge = next;
    if (next) unsubscribe = next.subscribe(scheduleRefresh);
    refresh();
  }

  // ---------- 一次请求 ----------

  function fail(message: string) {
    status.value = "error";
    errorMessage.value = message;
  }

  /** 流结束（正常或被停止）：整理结果、算差异、核对标签。 */
  function finish(kind: "done" | "stopped", event?: AIAssistEvent) {
    const text = normalizeResult(result.value);
    if (text === "" || !snapshot) {
      fail("模型没有返回内容，请重试");
      return;
    }
    if (event?.truncated) notice.value = "结果过长，已被截断，结果可能不完整，请核对后再用";
    else if (event?.finish_reason === "length") notice.value = "模型因长度限制提前结束，结果可能不完整";

    diff.value =
      action.value && diffApplicable(action.value, snapshot.placement) ? buildInlineDiff(snapshot.text, text) : null;
    view.value = diff.value ? preferredView(diff.value.similarity) : "result";
    tagWarning.value =
      (shortcodeTags(snapshot.text).length > 0 || shortcodeTags(text).length > 0) &&
      shortcodesChanged(snapshot.text, text);
    status.value = kind;
    refresh();
  }

  function onEvent(event: AIAssistEvent) {
    if (event.type === "delta") {
      result.value += event.text ?? "";
    } else if (event.type === "done") {
      finish("done", event);
    } else if (event.type === "error") {
      fail(event.message || "AI 调用失败，请重试");
    }
  }

  function blockedReason(id: AssistAction): string {
    if (status.value === "streaming") return "正在生成，请稍候";
    return assistBlockedReason({
      action: id,
      selectionChars: selection.value.chars,
      editable: selection.value.available,
      instruction: instruction.value,
    });
  }

  async function run(id: AssistAction): Promise<void> {
    if (!bridge || status.value === "streaming") return;
    refresh();
    const blocked = blockedReason(id);
    if (blocked) {
      resetResult();
      action.value = id;
      fail(blocked);
      return;
    }
    const snap = bridge.capture(id === "continue" ? "continue" : "selection");
    if (!snap) {
      resetResult();
      action.value = id;
      fail("编辑区当前不可用，请切换到编辑模式");
      return;
    }

    resetResult();
    snapshot = snap;
    action.value = id;
    placement.value = snap.placement;
    status.value = "streaming";

    const mine = ++runId;
    const own = new AbortController();
    controller = own;
    const request = buildAssistRequest({
      action: id,
      context: snap,
      title: deps.getTitle(),
      outline: extractOutline(deps.getMarkdown()),
      instruction: instruction.value,
      targetLang: targetLang.value,
    });
    try {
      await deps.stream(request, {
        signal: own.signal,
        onEvent: (event) => {
          if (mine === runId) onEvent(event);
        },
      });
    } catch (error) {
      if (mine === runId) fail(messageOf(error));
      return;
    } finally {
      if (mine === runId) controller = null;
    }
    // stream 约定以 done 或 error 事件收尾；没有的话按中断处理，不让界面停在「生成中」。
    if (mine === runId && status.value === "streaming") fail("连接中断，结果可能不完整");
  }

  function stop() {
    if (status.value !== "streaming") return;
    cancelRun();
    if (normalizeResult(result.value) === "") {
      resetResult();
      return;
    }
    finish("stopped");
  }

  function retry(): Promise<void> {
    return action.value ? run(action.value) : Promise.resolve();
  }

  function discard() {
    cancelRun();
    resetResult();
  }

  // ---------- 应用、撤销、复制 ----------

  function apply(): boolean {
    if (!bridge || !snapshot) return false;
    if (status.value !== "done" && status.value !== "stopped") return false;
    const text = normalizeResult(result.value);
    if (text === "") return false;
    applyError.value = "";
    const outcome = bridge.apply(snapshot, text);
    if (!outcome.ok) {
      if (outcome.reason === "stale") stale.value = true;
      else applyError.value = "应用失败，请重试；也可以复制结果手动粘贴";
      return false;
    }
    status.value = "applied";
    // 浏览器不支持原生撤销、改成了直接写值时，Ctrl/⌘+Z 不一定能退回这次应用：提示改用面板里的「撤销」。
    notice.value = outcome.undoable ? "" : "这次写入不能用 Ctrl/⌘+Z 撤销，要退回请点面板里的「撤销」";
    // 「应用」按钮随之消失：焦点交回编辑器，之后继续输入或按 Ctrl/⌘+Z 撤销都落在正文上。
    bridge.focus();
    refresh();
    return true;
  }

  function undo(): boolean {
    if (!bridge || status.value !== "applied") return false;
    applyError.value = "";
    if (!bridge.undo()) {
      applyError.value = "撤销失败，请使用编辑器自带的撤销";
      refresh();
      return false;
    }
    status.value = "done";
    bridge.focus();
    refresh();
    return true;
  }

  async function copy(): Promise<boolean> {
    const text = normalizeResult(result.value);
    if (text === "") return false;
    try {
      if (deps.copyText) await deps.copyText(text);
      else await navigator.clipboard.writeText(text);
    } catch {
      applyError.value = "复制失败，请手动选中文字复制";
      return false;
    }
    copied.value = true;
    clearTimeout(copiedTimer);
    copiedTimer = setTimeout(() => (copied.value = false), COPIED_MS);
    return true;
  }

  // ---------- 面板里的输入 ----------

  function setInstruction(value: string) {
    instruction.value = value;
  }

  function setTargetLang(value: string) {
    targetLang.value = value;
  }

  function setView(value: AssistView) {
    view.value = value;
  }

  // ---------- 面板开关 ----------

  function openPanel() {
    open.value = true;
    refresh();
  }

  function closePanel() {
    if (status.value === "streaming") stop();
    if (status.value === "applied") resetResult();
    open.value = false;
  }

  function toggle() {
    if (open.value) closePanel();
    else openPanel();
  }

  /** 组件卸载时调用：断开编辑器、取消请求、清定时器。 */
  function dispose() {
    clearTimeout(copiedTimer);
    detach();
    open.value = false;
  }

  return {
    open,
    status,
    action,
    placement,
    result,
    errorMessage,
    notice,
    applyError,
    instruction,
    targetLang,
    view,
    diff,
    selection,
    stale,
    canUndo,
    copied,
    tagWarning,
    attach,
    detach,
    dispose,
    refresh,
    blockedReason,
    run,
    stop,
    retry,
    discard,
    apply,
    undo,
    copy,
    setInstruction,
    setTargetLang,
    setView,
    openPanel,
    closePanel,
    toggle,
  };
}

export type WritingAssistant = ReturnType<typeof useWritingAssistant>;

/** 模板里直接读写的形态：ref 已被解包（`assistant.status` 而不是 `assistant.status.value`）。 */
export type WritingAssistantState = UnwrapNestedRefs<WritingAssistant>;

export function createWritingAssistant(deps: AssistantDeps): WritingAssistantState {
  return reactive(useWritingAssistant(deps));
}
