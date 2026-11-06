<script setup lang="ts">
import { inject, ref, computed, watch, onMounted, onBeforeUnmount } from "vue";

import { stellarEditorContextKey } from "./stellar-context";
import { NodeViewWrapper, nodeViewProps } from "@tiptap/vue-3";
import { Icon } from "@iconify/vue";
import { pluginApi } from "@/api";
import StellarTagPropertyPopover from "./StellarTagPropertyPopover.vue";
import type { PluginMeta, PluginVariant } from "@/modules/plugins/plugin-types";
import { loadPluginCatalog } from "@/modules/plugins/usePluginCatalog";
import { formForVariant } from "@/modules/plugins/shortcode";
import { usePluginPreview } from "@/modules/plugins/preview/usePluginPreview";
import { PreviewCache, sharedPreviewQueue } from "@/modules/plugins/preview/render-gate";
import { withPreviewContentWidth } from "@/modules/plugins/preview/preview-document";
import { tagNameOf, typeLabelOf } from "./stellar-tag";

// Stellar 标签节点的 NodeView：选中时浮出 Halo 同款操作气泡（类型标签 + 编辑/删除），iframe 渲染前台真实效果。
const stellarContext = inject(stellarEditorContextKey, undefined);
const props = defineProps(nodeViewProps);

const raw = computed(() => (props.node.attrs.raw as string) || "");
const typeLabel = computed(() => typeLabelOf(raw.value));

// 共享预览调度：raw 短防抖（150ms）、context 长合并（550ms）；同 raw+context 命中缓存不重发，
// 刷新期间保留旧 iframe/高度；失败保留上次成功预览并可重试。
const preview = usePluginPreview(() => ({ capability: "static", height: 64 }), undefined, {
  delayMs: 150,
  queue: sharedPreviewQueue,
  cache: new PreviewCache(20),
  // 页面画布已提供真实主题背景；其他编辑器仍需要独立表面保证文字对比度。
  surface: stellarContext?.value.layout === "page" ? "embedded" : "standalone",
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
    // 正文预览沿用真实站点上下文；context 指纹参与缓存键，标题/配置变化才失效。
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
  void loadPluginCatalog();
});
// raw 走短防抖，编辑标题/slug/配置走较长合并，避免每个标签逐字整站重渲染。
watch(raw, () => refreshPreview(150));
watch(
  () => stellarContext?.value,
  () => refreshPreview(550),
  { deep: true },
);

// 高度消息统一交给共享 composable（单一 token/上下限规则），NodeView 不再维护第二套校验。
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

// ---------- 编辑（优先属性表单，解析失败时保留源码编辑） ----------

const editing = ref(false);
const editingLoading = ref(false);
const editingError = ref("");
const editingPlugin = ref<PluginMeta | null>(null);
const editingVariant = ref<PluginVariant | undefined>(undefined);
const editingPresent = ref<string[] | undefined>(undefined);
const editingValues = ref<Record<string, unknown> | undefined>(undefined);

// hybrid 标签（frame 单行）需要按 variant 换一套属性字段；其余标签原样返回。
const editingFormPlugin = computed(() =>
  editingPlugin.value ? formForVariant(editingPlugin.value, editingVariant.value) : null,
);

async function startEdit() {
  editing.value = true;
  editingLoading.value = true;
  editingError.value = "";
  editingPlugin.value = null;
  editingVariant.value = undefined;
  editingValues.value = undefined;

  const sourceRaw = raw.value;
  const tagName = tagNameOf(sourceRaw).toLowerCase();
  try {
    const plugins = await loadPluginCatalog();
    const plugin = plugins.find((item) => item.name.toLowerCase() === tagName) || null;
    if (!plugin) {
      editingError.value = "当前标签未在 shortcode catalog 中登记，可直接编辑源码。";
      return;
    }
    const { data } = await pluginApi.parseShortcode(sourceRaw);
    if (!editing.value || raw.value !== sourceRaw) return;
    // 零参数组件（links）也算支持：面板会提示「没有可编辑属性」并保留源码编辑。
    if (!data.editable || data.name.toLowerCase() !== tagName) {
      editingError.value = "当前标签暂不支持属性面板，可直接编辑源码。";
      return;
    }
    editingPlugin.value = plugin;
    editingVariant.value = data.variant;
    editingValues.value = data.values;
    editingPresent.value = data.present || [];
  } catch {
    if (editing.value && raw.value === sourceRaw) editingError.value = "属性解析失败，可直接编辑源码。";
  } finally {
    if (raw.value === sourceRaw) editingLoading.value = false;
  }
}

function updateEditedRaw(next: string) {
  if (next !== raw.value) props.updateAttributes({ raw: next });
}

function closeEditor() {
  editing.value = false;
}
</script>

<template>
  <NodeViewWrapper :ref="setWrapperRef" class="stellar-tag-node group relative my-2">
    <!-- 左侧 gutter 拖拽把手（悬停卡片浮现，直接拖拽移动节点，无需先选中） -->
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

    <!-- 悬浮操作气泡（对照 Halo：白色胶囊 + 底部指向节点的小箭头） -->
    <div
      v-if="selected && !editing"
      contenteditable="false"
      class="stellar-bubble absolute -top-11 left-0 z-50 flex max-w-full items-center gap-1 rounded-[10px] border border-border bg-card px-1.5 py-1 shadow-lg"
    >
      <!-- 左侧类型标签（纯显示；拖拽由左侧 gutter 把手负责） -->
      <span :title="typeLabel" class="flex items-center gap-1.5 rounded px-1.5 py-0.5">
        <Icon icon="ri:puzzle-line" class="text-sm text-primary" />
        <span class="text-[13px] font-medium text-text-secondary">{{ typeLabel }}</span>
        <span v-if="loading" class="text-[10px] text-text-quaternary">渲染中…</span>
        <span v-else-if="updating" class="text-[10px] text-text-quaternary">更新中…</span>
      </span>

      <span class="mx-0.5 h-4 w-px bg-accent"></span>

      <!-- 编辑属性 -->
      <button
        type="button"
        aria-label="编辑属性"
        class="group/btn relative rounded p-1.5 text-text-tertiary transition-colors hover:bg-accent hover:text-foreground"
        @mousedown.prevent
        @click="startEdit"
      >
        <Icon icon="ri:file-edit-line" class="text-base" />
        <span
          class="pointer-events-none absolute -top-9 left-1/2 hidden -translate-x-1/2 whitespace-nowrap rounded border border-border bg-secondary px-2 py-1 text-[11px] text-foreground shadow-md group-hover/btn:block"
          >编辑属性</span
        >
      </button>

      <!-- 删除（红色） -->
      <button
        type="button"
        aria-label="删除组件"
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

      <!-- 底部指向节点的小箭头 -->
      <span class="absolute -bottom-1 left-5 h-2 w-2 rotate-45 border-b border-r border-border bg-card"></span>
    </div>

    <StellarTagPropertyPopover
      v-if="editing"
      :plugin="editingFormPlugin"
      :initial-values="editingValues"
      :present="editingPresent"
      :variant="editingVariant"
      :raw="raw"
      :loading="editingLoading"
      :error="editingError"
      @update:raw="updateEditedRaw"
      @close="closeEditor"
    />

    <!-- 无缝预览：与正文融合，悬停淡描边提示可点选，选中 indigo ring -->
    <div
      class="relative rounded-lg transition-shadow"
      :class="
        selected
          ? 'ring-2 ring-chart-4/40'
          : 'cursor-pointer hover:outline hover:outline-1 hover:outline-dashed hover:outline-[var(--color-border-strong)]'
      "
    >
      <p v-if="failed" class="rounded-lg border border-state-error/30 px-3 py-4 text-center text-xs text-state-error">
        预览渲染失败，可点击编辑检查标签语法
        <button type="button" class="ml-1 text-primary" @mousedown.prevent @click="preview.retry()">重试</button>
      </p>
      <!-- 未选中时 pointer-events 关闭：整卡任意处点击即选中节点出气泡；选中后恢复可交互/滚动 -->
      <div
        v-else-if="hasPreview"
        class="stellar-tag-viewport overflow-hidden"
      >
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
        title="标签预览"
        @load="bindPreviewMessages"
      ></iframe>
      </div>
      <p
        v-else
        class="rounded-lg border border-dashed border-border px-3 py-4 text-center text-xs text-text-quaternary"
      >
        {{ loading ? "渲染中…" : "空标签，选中后点击编辑填写内容" }}
      </p>
      <!-- 刷新失败时保留上一次成功预览，仅叠加可重试提示 -->
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
.stellar-tag-node iframe {
  /* 与子文档的默认配色方案一致，让透明背景透出编辑器底色。 */
  color-scheme: normal;
}
.stellar-tag-node.is-dragging iframe {
  display: none !important;
}
.stellar-tag-node.is-dragging {
  opacity: 0.5;
}
</style>
