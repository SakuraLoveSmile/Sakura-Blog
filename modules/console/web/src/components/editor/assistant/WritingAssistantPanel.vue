<script setup lang="ts">
// 写作助手面板：展示选区概况、可执行的动作，以及一次请求的流式结果、对比、应用与撤销。
// 纯展示：状态与动作都在 useWritingAssistant 里，编辑器通过 AssistantBridge 接入；
// 这里只用文本插值渲染（结果可能含 HTML 字符），不使用 v-html。
import { computed, nextTick, ref, watch } from "vue";
import { Icon } from "@iconify/vue";
import { VButton } from "@/components/ui";
import {
  ASSIST_ACTIONS,
  ASSIST_LANGUAGES,
  ASSIST_LIMITS,
  countChars,
  normalizeResult,
  type AssistAction,
} from "./assistant-core";
import type { WritingAssistantState } from "./useWritingAssistant";

const props = defineProps<{
  assistant: WritingAssistantState;
  /** dock：并入右栏；sheet：窄屏下浮在编辑区上的底部面板。 */
  variant: "dock" | "sheet";
  /** 快捷键的展示文字，如 ⌘J。 */
  hotkeyLabel: string;
  /** 编辑器处于全屏：底部导航被盖住，面板不用再给它让位。 */
  fullscreen?: boolean;
}>();

const emit = defineEmits<{ (event: "close"): void }>();

const rootRef = ref<HTMLElement | null>(null);
const resultRef = ref<HTMLElement | null>(null);
const bodyRef = ref<HTMLElement | null>(null);

const ACTION_ICONS: Record<AssistAction, string> = {
  polish: "ri:magic-line",
  rewrite: "ri:loop-left-line",
  expand: "ri:expand-diagonal-line",
  shorten: "ri:contract-up-down-line",
  continue: "ri:edit-2-line",
  translate: "ri:translate-2",
  custom: "ri:send-plane-2-line",
};

/** 一排五个常用动作；翻译与自定义指令各占一行。 */
const quickActions = ASSIST_ACTIONS.filter((item) => item.id !== "translate");

/** 快捷键里的修饰键，用于「⌘+Enter 执行」之类的提示。 */
const modLabel = computed(() => (props.hotkeyLabel.startsWith("⌘") ? "⌘" : "Ctrl"));

const streaming = computed(() => props.assistant.status === "streaming");
const settled = computed(() => props.assistant.status === "done" || props.assistant.status === "stopped");
const hasResult = computed(() => props.assistant.status !== "idle");
const showDiff = computed(() => props.assistant.view === "diff" && props.assistant.diff !== null && !streaming.value);
const shownText = computed(() => (streaming.value ? props.assistant.result : normalizeResult(props.assistant.result)));
const actionLabel = computed(() => {
  const id = props.assistant.action;
  if (id === "custom") return "自定义指令";
  return ASSIST_ACTIONS.find((item) => item.id === id)?.label ?? "";
});

const selectionTitle = computed(() => {
  const s = props.assistant.selection;
  if (!s.available) return "编辑区当前不可用";
  if (s.empty) return "没有选中文字";
  return `已选 ${s.chars} 字`;
});
const overLimit = computed(() => props.assistant.selection.chars > ASSIST_LIMITS.selection);

const instructionChars = computed(() => countChars(props.assistant.instruction.trim()));

const applyLabel = computed(() => {
  switch (props.assistant.placement) {
    case "at-caret":
      return "插入光标处";
    case "after-block":
      return "插入段落后";
    default:
      return "替换选区";
  }
});

const statusText = computed(() => {
  const a = props.assistant;
  const head = actionLabel.value ? `${actionLabel.value} · ` : "";
  switch (a.status) {
    case "streaming":
      return `${head}正在生成…`;
    case "done":
      return a.stale ? `${head}原文已改动` : `${head}已生成，确认后再应用`;
    case "stopped":
      return a.stale ? `${head}原文已改动` : `${head}已停止，保留了已生成的部分`;
    case "applied":
      return `${head}已写入正文（尚未保存）`;
    default:
      return "";
  }
});

function onKeydown(event: KeyboardEvent) {
  if (event.key !== "Escape" || event.isComposing) return;
  event.preventDefault();
  event.stopPropagation();
  // 生成中先停下，保留已有的部分；再按一次才关闭。
  if (props.assistant.status === "streaming") props.assistant.stop();
  else emit("close");
}

function onInstructionInput(event: Event) {
  props.assistant.setInstruction((event.target as HTMLTextAreaElement).value);
}

function onInstructionKeydown(event: KeyboardEvent) {
  if (event.key !== "Enter" || event.isComposing || !(event.ctrlKey || event.metaKey)) return;
  event.preventDefault();
  if (!props.assistant.blockedReason("custom")) void props.assistant.run("custom");
}

function onLangChange(event: Event) {
  props.assistant.setTargetLang((event.target as HTMLSelectElement).value);
}

// 流式输出时，正文停在底部附近就跟着滚；用户往上翻看了就不打扰。
watch(
  () => props.assistant.result,
  async () => {
    if (!streaming.value) return;
    const el = bodyRef.value;
    if (!el) return;
    const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 48;
    await nextTick();
    if (nearBottom && bodyRef.value) bodyRef.value.scrollTop = bodyRef.value.scrollHeight;
  },
);

// 把结果区带进可视范围：开始生成时一律来一次；结束（完成、停止、出错）时只在底部面板里再来一次——
// 那里高度有限，结果排在选区与动作下面，结束时多出来的提示和「应用」按钮要一起露出来。
// 桌面右栏随页面滚动，结束时不去抢作者正在看的位置。
watch(
  () => props.assistant.status,
  (status) => {
    const finished = status === "done" || status === "stopped" || status === "error";
    if (status !== "streaming" && !(finished && props.variant === "sheet")) return;
    void nextTick(() => {
      const calm = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
      resultRef.value?.scrollIntoView({ block: "nearest", behavior: calm ? "auto" : "smooth" });
    });
  },
);

/** 焦点没有落在任何可用的控件上（按钮被禁用或移除后，焦点会掉到页面上）。 */
function focusLost(): boolean {
  const el = document.activeElement;
  return !el || el === document.body || el === document.documentElement || el.matches(":disabled");
}

// 触发动作的按钮在生成中会被禁用，「停止」也会在结束时消失：焦点跟着交给接手的控件，
// 这样 Esc、Enter 和读屏都不会断；焦点本来就在别处（比如已回到编辑器）时不去抢。
watch(
  () => props.assistant.status,
  async (status) => {
    await nextTick();
    if (status === "streaming") {
      if (focusLost() || rootRef.value?.contains(document.activeElement)) {
        rootRef.value?.querySelector<HTMLElement>('[data-action="stop"]')?.focus({ preventScroll: true });
      }
      return;
    }
    if (focusLost()) (bodyRef.value ?? rootRef.value)?.focus({ preventScroll: true });
  },
);

defineExpose({
  focus: () => rootRef.value?.focus({ preventScroll: true }),
});
</script>

<template>
  <section
    id="writing-assistant-panel"
    ref="rootRef"
    class="wa"
    :class="[variant === 'dock' ? 'wa--dock' : 'wa--sheet', { 'wa--fullscreen': fullscreen }]"
    role="region"
    aria-labelledby="writing-assistant-title"
    tabindex="-1"
    @keydown="onKeydown"
  >
    <header class="wa-head">
      <div class="wa-title">
        <Icon icon="ri:quill-pen-ai-line" class="wa-title-icon" aria-hidden="true" />
        <h2 id="writing-assistant-title">写作助手</h2>
        <kbd class="wa-kbd" aria-hidden="true">{{ hotkeyLabel }}</kbd>
      </div>
      <button type="button" class="wa-close" aria-label="关闭写作助手" title="关闭（Esc）" @click="emit('close')">
        <Icon icon="ri:close-line" aria-hidden="true" />
      </button>
    </header>

    <!-- 选区概况 -->
    <div class="wa-selection" data-testid="wa-selection">
      <div class="wa-selection-head">
        <span :class="{ 'is-warn': overLimit }">{{ selectionTitle }}</span>
        <span v-if="overLimit" class="wa-limit">超过 {{ ASSIST_LIMITS.selection }} 字上限，请缩小范围</span>
      </div>
      <p v-if="!assistant.selection.available" class="wa-hint">
        请先切换到「仅编辑」或「即时渲染」模式，助手才能读取和修改正文。
      </p>
      <p v-else-if="assistant.selection.empty" class="wa-hint">
        选中一段正文后，可以润色、改写、扩写、缩写或翻译。不选也能「续写」光标所在段落之后的内容，或用自定义指令在光标处生成文字。
      </p>
      <template v-else>
        <p class="wa-excerpt">{{ assistant.selection.excerpt }}</p>
        <p v-if="assistant.selection.expanded" class="wa-hint">选区跨越了多个块，已扩展到完整的块一起处理。</p>
      </template>
    </div>

    <!-- 动作 -->
    <div class="wa-actions" role="group" aria-label="对选区执行的操作">
      <button
        v-for="item in quickActions"
        :key="item.id"
        type="button"
        class="wa-action"
        :data-action="item.id"
        :disabled="!!assistant.blockedReason(item.id)"
        :title="assistant.blockedReason(item.id) || item.hint"
        @click="assistant.run(item.id)"
      >
        <Icon :icon="ACTION_ICONS[item.id]" class="wa-action-icon" aria-hidden="true" />
        <span>{{ item.label }}</span>
      </button>
    </div>

    <div class="wa-translate">
      <label class="sr-only" for="writing-assistant-lang">翻译成</label>
      <select
        id="writing-assistant-lang"
        class="wa-select"
        :value="assistant.targetLang"
        :disabled="streaming"
        @change="onLangChange"
      >
        <option v-for="lang in ASSIST_LANGUAGES" :key="lang.code" :value="lang.code">翻译成{{ lang.label }}</option>
      </select>
      <VButton
        type="default"
        data-action="translate"
        :disabled="!!assistant.blockedReason('translate')"
        :title="assistant.blockedReason('translate') || '翻译选区，保留格式和标签'"
        @click="assistant.run('translate')"
      >
        <template #icon><Icon :icon="ACTION_ICONS.translate" aria-hidden="true" /></template>
        翻译
      </VButton>
    </div>

    <div class="wa-custom">
      <label class="wa-label" for="writing-assistant-instruction">自定义指令</label>
      <textarea
        id="writing-assistant-instruction"
        class="wa-textarea"
        rows="2"
        placeholder="例如：改成更口语的语气；补一个生活里的类比"
        aria-describedby="writing-assistant-instruction-hint"
        :value="assistant.instruction"
        :disabled="streaming"
        @input="onInstructionInput"
        @keydown="onInstructionKeydown"
      ></textarea>
      <div class="wa-custom-foot">
        <span id="writing-assistant-instruction-hint" class="wa-hint">
          有选区时按指令处理选区，没有选区就在光标处写入。{{ modLabel }}+Enter 执行。
          <span :class="{ 'is-warn': instructionChars > ASSIST_LIMITS.instruction }"
            >{{ instructionChars }}/{{ ASSIST_LIMITS.instruction }}</span
          >
        </span>
        <VButton
          type="secondary"
          data-action="custom"
          :disabled="!!assistant.blockedReason('custom')"
          :title="assistant.blockedReason('custom') || '按指令生成'"
          @click="assistant.run('custom')"
        >
          <template #icon><Icon :icon="ACTION_ICONS.custom" aria-hidden="true" /></template>
          执行
        </VButton>
      </div>
    </div>

    <!-- 结果 -->
    <div v-if="hasResult" ref="resultRef" class="wa-result" data-testid="wa-result">
      <div class="wa-result-head" :class="{ 'is-empty': assistant.status === 'error' }">
        <p class="wa-status" role="status" aria-live="polite" :class="{ 'is-error': assistant.status === 'error' }">
          <span v-if="assistant.status !== 'error'">{{ statusText }}</span>
        </p>
        <div v-if="assistant.diff && !streaming" class="wa-seg" role="group" aria-label="结果视图">
          <button
            type="button"
            class="wa-seg-btn"
            :aria-pressed="assistant.view === 'diff'"
            @click="assistant.setView('diff')"
          >
            对比
          </button>
          <button
            type="button"
            class="wa-seg-btn"
            :aria-pressed="assistant.view === 'result'"
            @click="assistant.setView('result')"
          >
            结果
          </button>
        </div>
      </div>

      <p v-if="assistant.status === 'error'" class="wa-note is-error" role="alert">
        <Icon icon="ri:error-warning-line" aria-hidden="true" />
        <span>{{ assistant.errorMessage }}</span>
      </p>

      <div
        v-if="shownText !== '' || streaming"
        ref="bodyRef"
        class="wa-body"
        :class="{ 'is-streaming': streaming }"
        role="region"
        aria-label="AI 生成结果"
        tabindex="0"
        :aria-busy="streaming"
        data-testid="wa-body"
      >
        <template v-if="showDiff && assistant.diff">
          <template v-for="(seg, index) in assistant.diff.segments" :key="index">
            <ins v-if="seg.type === 'insert'">{{ seg.text }}</ins>
            <del v-else-if="seg.type === 'delete'">{{ seg.text }}</del>
            <span v-else>{{ seg.text }}</span>
          </template>
        </template>
        <template v-else>{{ shownText }}</template>
      </div>

      <p v-if="showDiff && assistant.diff" class="wa-hint">
        新增 {{ assistant.diff.added }} 字，删除 {{ assistant.diff.removed }} 字
      </p>

      <p v-if="assistant.notice && (settled || assistant.status === 'applied')" class="wa-note is-warn">
        <Icon icon="ri:information-line" aria-hidden="true" />
        <span>{{ assistant.notice }}</span>
      </p>
      <p v-if="assistant.tagWarning && settled" class="wa-note is-warn">
        <Icon icon="ri:information-line" aria-hidden="true" />
        <span>结果里的标签（{% … %}）与原文不一致，应用前请逐个核对。</span>
      </p>
      <p v-if="assistant.stale && settled" class="wa-note is-warn">
        <Icon icon="ri:information-line" aria-hidden="true" />
        <span>原文在生成期间被改动，结果已经不能直接应用。可以复制结果，或重新生成。</span>
      </p>
      <p v-if="assistant.applyError" class="wa-note is-error" role="alert">
        <Icon icon="ri:error-warning-line" aria-hidden="true" />
        <span>{{ assistant.applyError }}</span>
      </p>

      <div class="wa-bar">
        <template v-if="streaming">
          <VButton type="secondary" data-action="stop" @click="assistant.stop()">
            <template #icon><Icon icon="ri:stop-circle-line" aria-hidden="true" /></template>
            停止
          </VButton>
        </template>
        <template v-else-if="settled">
          <VButton type="primary" data-action="apply" :disabled="assistant.stale" @click="assistant.apply()">
            {{ applyLabel }}
          </VButton>
          <VButton type="default" data-action="copy" @click="assistant.copy()">
            <template #icon><Icon icon="ri:file-copy-line" aria-hidden="true" /></template>
            {{ assistant.copied ? "已复制" : "复制" }}
          </VButton>
          <span class="wa-bar-more">
            <VButton type="ghost" data-action="retry" @click="assistant.retry()">重试</VButton>
            <VButton type="ghost" data-action="discard" @click="assistant.discard()">放弃</VButton>
          </span>
        </template>
        <template v-else-if="assistant.status === 'applied'">
          <VButton v-if="assistant.canUndo" type="secondary" data-action="undo" @click="assistant.undo()">
            <template #icon><Icon icon="ri:arrow-go-back-line" aria-hidden="true" /></template>
            撤销
          </VButton>
          <p v-else class="wa-hint wa-bar-note">正文在应用后又改过了，无法一键撤销；可以用编辑器自带的撤销逐步回退。</p>
          <VButton type="ghost" data-action="discard" @click="assistant.discard()">收起</VButton>
        </template>
        <template v-else-if="assistant.status === 'error'">
          <VButton v-if="assistant.action" type="secondary" data-action="retry" @click="assistant.retry()">
            <template #icon><Icon icon="ri:refresh-line" aria-hidden="true" /></template>
            重试
          </VButton>
          <VButton type="ghost" data-action="discard" @click="assistant.discard()">关闭提示</VButton>
        </template>
      </div>
    </div>

    <p class="wa-foot">
      选中的文字及其前后文会发送给站点配置的 AI 服务。结果只有点击「应用」才会写入正文，写入后仍需手动保存。
    </p>
  </section>
</template>

<style scoped>
.wa {
  display: flex;
  flex-direction: column;
  gap: 14px;
  color: var(--color-foreground);
  outline: none;
}

/* 并入右栏：与「大纲 / 设置」同宽，用 hairline 分隔，不再套卡片。 */
.wa--dock {
  padding: 0 16px 16px;
}

/* 窄屏：固定在页面底部的面板，不遮罩、不抢焦点，编辑区仍可操作。 */
.wa--sheet {
  position: fixed;
  z-index: 60;
  inset-inline: 0;
  /* 紧贴底部标签栏：栏高 57px（1px 上边框 + 48px 内容 + 8px 内边距），再加安全区。 */
  bottom: calc(57px + env(safe-area-inset-bottom));
  max-height: min(58dvh, 520px);
  overflow-y: auto;
  overscroll-behavior: contain;
  padding: 0 16px 16px;
  background: var(--color-card);
  border-top: 1px solid var(--color-border);
  border-radius: var(--radius-lg) var(--radius-lg) 0 0;
  box-shadow: var(--shadow-xl);
}

@media (min-width: 768px) {
  .wa--sheet {
    inset-inline: auto 16px;
    bottom: 16px;
    width: min(420px, calc(100vw - 32px));
    max-height: min(72dvh, 640px);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
  }
}

.wa-head {
  position: sticky;
  top: 0;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-height: 44px;
  margin-inline: -16px;
  padding-inline: 16px 8px;
  border-bottom: 1px solid var(--color-border);
  background: var(--color-card);
}

.wa--dock .wa-head {
  background: transparent;
  position: static;
}

.wa-title {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 8px;
}

.wa-title h2 {
  font-size: 13px;
  font-weight: 600;
}

.wa-title-icon {
  flex: 0 0 auto;
  color: var(--color-text-secondary);
  font-size: 16px;
}

.wa-kbd {
  padding: 1px 6px;
  border: 1px solid var(--color-border-strong);
  border-radius: var(--radius-xs);
  color: var(--color-text-tertiary);
  font-family: inherit;
  font-size: 11px;
  line-height: 1.5;
}

.wa-close {
  display: inline-flex;
  width: 36px;
  height: 36px;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-sm);
  color: var(--color-text-secondary);
  font-size: 18px;
  transition:
    background-color var(--duration-fast) var(--ease-standard),
    color var(--duration-fast) var(--ease-standard);
}

.wa-close:hover {
  background: var(--color-accent);
  color: var(--color-foreground);
}

.wa-close:focus-visible,
.wa-action:focus-visible,
.wa-seg-btn:focus-visible,
.wa-select:focus-visible,
.wa-textarea:focus-visible,
.wa-body:focus-visible {
  outline: 2px solid var(--color-ring);
  outline-offset: 2px;
}

.wa-selection {
  padding: 10px 12px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-secondary);
}

.wa-selection-head {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  justify-content: space-between;
  gap: 4px 8px;
  color: var(--color-text-secondary);
  font-size: 12px;
  font-weight: 500;
}

.wa-limit,
.is-warn {
  color: var(--color-state-warning);
}

.wa-excerpt {
  display: -webkit-box;
  margin-top: 6px;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
  line-clamp: 3;
  color: var(--color-foreground);
  font-size: 13px;
  line-height: 1.55;
  overflow-wrap: anywhere;
}

.wa-hint {
  margin-top: 4px;
  color: var(--color-text-tertiary);
  font-size: 12px;
  line-height: 1.6;
}

.wa-actions {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 6px;
}

.wa-action {
  display: flex;
  min-height: 52px;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  padding: 6px 2px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  color: var(--color-text-secondary);
  font-size: 12.5px;
  line-height: 1.2;
  transition:
    background-color var(--duration-fast) var(--ease-standard),
    border-color var(--duration-fast) var(--ease-standard),
    color var(--duration-fast) var(--ease-standard);
}

.wa-action-icon {
  font-size: 17px;
}

.wa-action:hover:not(:disabled) {
  border-color: var(--color-border-strong);
  background: var(--color-accent);
  color: var(--color-foreground);
}

.wa-action:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.wa-translate {
  display: flex;
  align-items: center;
  gap: 8px;
}

.wa-select {
  min-width: 0;
  flex: 1;
  height: 36px;
  padding: 0 8px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background: var(--color-background);
  color: var(--color-foreground);
  font-size: 13px;
}

.wa-select:disabled,
.wa-textarea:disabled {
  opacity: 0.6;
}

.wa-custom {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.wa-label {
  color: var(--color-text-secondary);
  font-size: 12px;
  font-weight: 500;
}

.wa-textarea {
  width: 100%;
  min-height: 60px;
  resize: vertical;
  padding: 8px 10px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
  background: var(--color-background);
  color: var(--color-foreground);
  font-size: 13px;
  line-height: 1.55;
}

.wa-textarea::placeholder {
  color: var(--color-text-quaternary);
}

.wa-custom-foot {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}

.wa-custom-foot .wa-hint {
  margin-top: 0;
}

.wa-result {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding-top: 14px;
  border-top: 1px solid var(--color-border);
}

.wa-result-head {
  display: flex;
  min-height: 28px;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

/* 出错时状态文字留空（错误信息在下面的提示里），头部不再撑出一截空白；状态区本身仍在，供读屏器播报。 */
.wa-result-head.is-empty {
  min-height: 0;
}
.wa-status {
  min-width: 0;
  color: var(--color-text-secondary);
  font-size: 12px;
  font-weight: 500;
}

.wa-seg {
  display: inline-flex;
  flex: 0 0 auto;
  padding: 2px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-sm);
}

.wa-seg-btn {
  min-height: 24px;
  padding: 0 10px;
  border-radius: var(--radius-xs);
  color: var(--color-text-tertiary);
  font-size: 12px;
}

.wa-seg-btn[aria-pressed="true"] {
  background: var(--color-accent);
  color: var(--color-foreground);
}

.wa-body {
  max-height: 260px;
  overflow-y: auto;
  padding: 10px 12px;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  background: var(--color-background);
  font-size: 13.5px;
  line-height: 1.7;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}

.wa-body ins {
  border-radius: 2px;
  background: color-mix(in srgb, var(--color-state-success) 20%, transparent);
  color: var(--color-foreground);
  text-decoration: underline;
  text-decoration-color: color-mix(in srgb, var(--color-state-success) 70%, transparent);
  text-underline-offset: 3px;
}

.wa-body del {
  border-radius: 2px;
  background: color-mix(in srgb, var(--color-state-error) 18%, transparent);
  color: var(--color-text-tertiary);
  text-decoration: line-through;
}

.wa-body.is-streaming::after {
  content: "";
  display: inline-block;
  width: 2px;
  height: 1em;
  margin-left: 2px;
  background: var(--color-foreground);
  vertical-align: -2px;
  animation: wa-blink 1s steps(2, start) infinite;
}

@keyframes wa-blink {
  to {
    visibility: hidden;
  }
}

@media (prefers-reduced-motion: reduce) {
  .wa-body.is-streaming::after {
    animation: none;
  }
}

.wa-note {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  font-size: 12px;
  line-height: 1.6;
}

.wa-note.is-warn {
  color: var(--color-state-warning);
}

.wa-note.is-error,
.wa-status.is-error {
  color: var(--color-state-error);
}

.wa-note > svg {
  flex: 0 0 auto;
  margin-top: 3px;
  font-size: 14px;
}

.wa-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

/* 重试与放弃是一组：右栏里放不下时整组换到下一行，不让「放弃」单独落单。 */
.wa-bar-more {
  display: inline-flex;
  gap: 8px;
}

.wa-bar-note {
  flex: 1 1 100%;
  margin-top: 0;
}

.wa-foot {
  color: var(--color-text-quaternary);
  font-size: 11.5px;
  line-height: 1.6;
}

/* 手机：每个可点击控件至少 44px，输入框 16px 避免 iOS 聚焦时放大页面。 */
@media (max-width: 767px) {
  .wa--sheet.wa--fullscreen {
    bottom: env(safe-area-inset-bottom);
  }
  .wa-close {
    width: 44px;
    height: 44px;
  }
  .wa-action {
    min-height: 56px;
  }
  .wa-select {
    height: 44px;
    font-size: 16px;
  }
  .wa-textarea {
    font-size: 16px;
  }
  .wa-seg-btn {
    min-width: 56px;
    min-height: 44px;
    padding: 0 14px;
  }
  .wa :deep(.btn) {
    min-height: 44px;
    min-width: 44px;
  }
  .wa-custom-foot {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
