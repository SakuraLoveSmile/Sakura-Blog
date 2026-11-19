<script setup lang="ts">
import { inject, ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from "vue";
import { NodeViewWrapper, nodeViewProps } from "@tiptap/vue-3";
import { Icon } from "@iconify/vue";
import { stellarEditorContextKey } from "./stellar-context";
import { usePluginPreview } from "@/modules/plugins/preview/usePluginPreview";
import { PreviewCache, sharedPreviewQueue } from "@/modules/plugins/preview/render-gate";
import { withPreviewContentWidth } from "@/modules/plugins/preview/preview-document";

// 外部 HTML 节点的 NodeView：iframe 渲染真实效果，选中浮出操作气泡（编辑源码 / 删除）。
// 预览沿用标签插件的共享调度与 token/resize 协议，不另造一套；sandbox 保持 opaque，
// 外部脚本在隔离文档内执行，发布后正文才是同源直出。
const stellarContext = inject(stellarEditorContextKey, undefined);
const props = defineProps(nodeViewProps);

const raw = computed(() => (props.node.attrs.raw as string) || "");

// 编辑器实际露出的底色（NodeView 自身透明）；页面画布本身就是正式页背景，正文嵌入保持透明。
function editorBackdropColor(): string | undefined {
  if (typeof document === "undefined" || typeof window === "undefined") return undefined;
  if (typeof window.getComputedStyle !== "function") return undefined;
  for (const element of [document.body, document.documentElement]) {
    if (!element) continue;
    const color = window.getComputedStyle(element).backgroundColor;
    if (!color || color === "transparent") continue;
    if (/^rgba\([^)]*,\s*0(?:\.0+)?\s*\)$/.test(color)) continue;
    return color;
  }
  return undefined;
}
const pageCanvas = stellarContext?.value.layout === "page";
const hostBackground = pageCanvas ? undefined : editorBackdropColor();

const preview = usePluginPreview(() => ({ capability: "static", height: 64 }), undefined, {
  delayMs: 150,
  queue: sharedPreviewQueue,
  cache: new PreviewCache(20),
  surface: pageCanvas ? "embedded" : "standalone",
  hostBackground,
  frame: { minHeight: 32, maxHeight: Number.MAX_SAFE_INTEGER, margin: 0 },
});
const contentWidth = ref(0);
const viewportWidth = ref(window.innerWidth);
const srcdoc = computed(() => withPreviewContentWidth(preview.srcdoc.value, contentWidth.value));
const hasPreview = computed(() => Boolean(preview.srcdoc.value));
const loading = computed(() => preview.status.value === "loading");
const updating = computed(() => preview.status.value === "updating");
const failed = computed(() => preview.status.value === "error" && !hasPreview.value);

const iframeRef = ref<HTMLIFrameElement | null>(null);
const wrapperRef = ref<HTMLElement | null>(null);
let widthObserver: ResizeObserver | undefined;
let messageWindow: Window | null = null;
function bindPreviewMessages() {
  const ownerWindow = iframeRef.value?.ownerDocument.defaultView || wrapperRef.value?.ownerDocument.defaultView || null;
  if (messageWindow === ownerWindow) return;
  messageWindow?.removeEventListener("message", onPreviewMessage);
  messageWindow = ownerWindow;
  messageWindow?.addEventListener("message", onPreviewMessage);
}
function measurePreviewWidth() {
  viewportWidth.value = window.innerWidth;
  contentWidth.value = wrapperRef.value?.clientWidth || 0;
  bindPreviewMessages();
}
function setWrapperRef(el: any) {
  wrapperRef.value = el?.$el ?? el ?? null;
}

function onHandleDragStart() {
  wrapperRef.value?.classList.add("is-dragging");
}
function onHandleDragEnd() {
  wrapperRef.value?.classList.remove("is-dragging");
}

function refreshPreview(delayMs: number) {
  if (!raw.value.trim()) {
    preview.clearPreview();
    return;
  }
  preview.holdHeight();
  preview.scheduleRender(raw.value, {
    delayMs,
    profile: null,
    metadata: stellarContext?.value as Record<string, unknown> | undefined,
  });
}

onMounted(() => {
  widthObserver = new ResizeObserver(measurePreviewWidth);
  if (wrapperRef.value) widthObserver.observe(wrapperRef.value);
  window.addEventListener("resize", measurePreviewWidth);
  measurePreviewWidth();
  refreshPreview(0);
});
watch(raw, () => refreshPreview(150));
watch(
  () => stellarContext?.value,
  () => refreshPreview(550),
  { deep: true },
);

function onPreviewMessage(event: MessageEvent) {
  preview.onMessage(event, iframeRef.value);
}
// EditorContent 可被 Teleport 到同源画布；监听实际父窗口，保留 composable 的 source/token 校验。
watch(iframeRef, bindPreviewMessages, { flush: "post" });
onMounted(bindPreviewMessages);
onBeforeUnmount(() => {
  messageWindow?.removeEventListener("message", onPreviewMessage);
  messageWindow = null;
  window.removeEventListener("resize", measurePreviewWidth);
  widthObserver?.disconnect();
});

// ---------- 源码编辑（内联 popover，避免跨 document Teleport） ----------

const editing = ref(false);
const draft = ref("");
const sourceRef = ref<HTMLTextAreaElement | null>(null);

function startEdit() {
  draft.value = raw.value;
  editing.value = true;
  nextTick(() => sourceRef.value?.focus());
}
function cancelEdit() {
  editing.value = false;
}
function saveEdit() {
  const next = draft.value;
  editing.value = false;
  if (next !== raw.value) props.updateAttributes({ raw: next });
}
</script>

<template>
  <NodeViewWrapper :ref="setWrapperRef" class="raw-html-node group relative my-2">
    <!-- 左侧 gutter 拖拽把手 -->
    <span
      v-if="!editing"
      data-drag-handle
      contenteditable="false"
      title="拖拽移动"
      class="absolute -left-6 top-1.5 z-40 flex h-6 w-5 cursor-grab items-center justify-center rounded text-text-quaternary opacity-0 transition-opacity hover:bg-accent hover:text-foreground group-hover:opacity-100"
      @dragstart="onHandleDragStart"
      @dragend="onHandleDragEnd"
    >
      <Icon icon="ri:draggable" class="text-base" />
    </span>

    <!-- 选中操作气泡 -->
    <div
      v-if="selected && !editing"
      contenteditable="false"
      class="raw-html-bubble absolute -top-11 left-0 z-50 flex max-w-full items-center gap-1 rounded-[10px] border border-border bg-card px-1.5 py-1 shadow-lg"
    >
      <span title="外部 HTML 片段" class="flex items-center gap-1.5 rounded px-1.5 py-0.5">
        <Icon icon="ri:html5-line" class="text-sm text-primary" />
        <span class="text-[13px] font-medium text-text-secondary">HTML 片段</span>
        <span v-if="loading" class="text-[10px] text-text-quaternary">渲染中…</span>
        <span v-else-if="updating" class="text-[10px] text-text-quaternary">更新中…</span>
      </span>

      <span class="mx-0.5 h-4 w-px bg-accent"></span>

      <button
        type="button"
        aria-label="编辑 HTML 源码"
        class="group/btn relative rounded p-1.5 text-text-tertiary transition-colors hover:bg-accent hover:text-foreground"
        @mousedown.prevent
        @click="startEdit"
      >
        <Icon icon="ri:file-edit-line" class="text-base" />
        <span
          class="pointer-events-none absolute -top-9 left-1/2 hidden -translate-x-1/2 whitespace-nowrap rounded border border-border bg-secondary px-2 py-1 text-[11px] text-foreground shadow-md group-hover/btn:block"
          >编辑源码</span
        >
      </button>

      <button
        type="button"
        aria-label="删除 HTML 片段"
        class="group/btn relative rounded p-1.5 text-state-error transition-colors hover:bg-state-error/10"
        @mousedown.prevent
        @click="deleteNode()"
      >
        <Icon icon="ri:delete-bin-line" class="text-base" />
        <span
          class="pointer-events-none absolute -top-9 left-1/2 hidden -translate-x-1/2 whitespace-nowrap rounded border border-border bg-secondary px-2 py-1 text-[11px] text-foreground shadow-md group-hover/btn:block"
          >删除</span
        >
      </button>

      <span class="absolute -bottom-1 left-5 h-2 w-2 rotate-45 border-b border-r border-border bg-card"></span>
    </div>

    <!-- 内联源码编辑 -->
    <div
      v-if="editing"
      contenteditable="false"
      class="relative z-50 overflow-hidden rounded-lg border border-border bg-card shadow-xl"
    >
      <div class="flex items-center justify-between gap-2 border-b border-border px-3 py-2">
        <span class="flex items-center gap-1.5 text-xs font-medium text-text-secondary">
          <Icon icon="ri:html5-line" class="text-sm text-primary" />
          HTML 源码
        </span>
        <span class="text-[10px] text-text-quaternary">原样保存，不做净化</span>
      </div>
      <textarea
        ref="sourceRef"
        v-model="draft"
        spellcheck="false"
        autocapitalize="off"
        autocomplete="off"
        aria-label="HTML 源码"
        class="block max-h-72 min-h-40 w-full resize-y bg-transparent px-3 py-2 font-mono text-[12px] leading-relaxed text-foreground outline-none"
        @keydown.esc.stop="cancelEdit"
        @keydown.meta.enter.stop="saveEdit"
        @keydown.ctrl.enter.stop="saveEdit"
      ></textarea>
      <div class="flex items-center justify-end gap-2 border-t border-border px-3 py-2">
        <button
          type="button"
          class="rounded-md px-2.5 py-1 text-xs text-text-secondary transition-colors hover:bg-secondary"
          @mousedown.prevent
          @click="cancelEdit"
        >
          取消
        </button>
        <button
          type="button"
          class="rounded-md bg-primary px-2.5 py-1 text-xs font-medium text-primary-foreground transition-colors hover:bg-primary-hover"
          @mousedown.prevent
          @click="saveEdit"
        >
          保存
        </button>
      </div>
    </div>

    <!-- 无缝预览：与正文融合，悬停淡描边提示可点选，选中 indigo ring -->
    <div
      v-else
      class="relative rounded-lg transition-shadow"
      :class="
        selected
          ? 'ring-2 ring-chart-4/40'
          : 'cursor-pointer hover:outline hover:outline-1 hover:outline-dashed hover:outline-[var(--color-border-strong)]'
      "
    >
      <p v-if="failed" class="rounded-lg border border-state-error/30 px-3 py-4 text-center text-xs text-state-error">
        预览渲染失败，可点击编辑检查 HTML
        <button type="button" class="ml-1 text-primary" @mousedown.prevent @click="preview.retry()">重试</button>
      </p>
      <div v-else-if="hasPreview" class="raw-html-viewport overflow-hidden">
        <iframe
          ref="iframeRef"
          :srcdoc="srcdoc"
          :style="{ height: preview.frameHeight.value + 'px', width: viewportWidth + 'px' }"
          :class="[
            'block max-w-none border-0 bg-transparent transition-[height] duration-150',
            !selected && 'pointer-events-none',
          ]"
          allowtransparency="true"
          sandbox="allow-scripts"
          title="HTML 片段预览"
          @load="bindPreviewMessages"
        ></iframe>
      </div>
      <p
        v-else
        class="rounded-lg border border-dashed border-border px-3 py-4 text-center text-xs text-text-quaternary"
      >
        {{ loading ? "渲染中…" : "空 HTML 片段，选中后点击编辑填写源码" }}
      </p>
      <div
        v-if="hasPreview && preview.status.value === 'error'"
        class="absolute inset-x-0 bottom-0 flex items-center justify-between gap-2 border-t border-state-error/30 bg-card/95 px-2 py-1 text-[11px] text-state-error"
      >
        <span>预览更新失败，仍显示上一次结果</span>
        <button type="button" class="text-primary" @mousedown.prevent @click="preview.retry()">重试</button>
      </div>
    </div>
  </NodeViewWrapper>
</template>

<style scoped>
.raw-html-node iframe {
  /* 与子文档的默认配色方案一致，让透明背景透出编辑器底色。 */
  color-scheme: normal;
}
.raw-html-node.is-dragging iframe {
  display: none !important;
}
.raw-html-node.is-dragging {
  opacity: 0.5;
}
</style>