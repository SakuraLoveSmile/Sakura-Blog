<script setup lang="ts">
import { inject, ref, computed, watch, onMounted, onBeforeUnmount } from "vue";
import { stellarEditorContextKey } from "./stellar-context";
import { NodeViewWrapper, nodeViewProps } from "@tiptap/vue-3";
import { Icon } from "@iconify/vue";
import { pluginApi, previewApi } from "@/api";
import StellarTagPropertyPopover from "./StellarTagPropertyPopover.vue";
import type { PluginMeta, PluginVariant } from "@/modules/plugins/plugin-types";
import { loadPluginCatalog } from "@/modules/plugins/usePluginCatalog";
import { formForVariant } from "@/modules/plugins/shortcode";
import { tagNameOf, typeLabelOf } from "./stellar-tag";

// Stellar 标签节点的 NodeView：选中时浮出 Halo 同款操作气泡（类型标签 + 编辑/删除），iframe 渲染前台真实效果。
const stellarContext = inject(stellarEditorContextKey, undefined);
const props = defineProps(nodeViewProps);

const raw = computed(() => (props.node.attrs.raw as string) || "");
const typeLabel = computed(() => typeLabelOf(raw.value));

const html = ref("");
const previewToken = ref("");
const loading = ref(false);
const failed = ref(false);
const iframeH = ref(64);
const iframeRef = ref<HTMLIFrameElement | null>(null);
const wrapperRef = ref<HTMLElement | null>(null);
function setWrapperRef(el: any) {
  wrapperRef.value = el?.$el ?? el ?? null;
}

function onHandleDragStart() {
  wrapperRef.value?.classList.add("is-dragging");
}
function onHandleDragEnd() {
  wrapperRef.value?.classList.remove("is-dragging");
}

let previewGeneration = 0;
async function renderPreview() {
  const generation = ++previewGeneration;
  const key = raw.value;
  if (!key.trim()) {
    html.value = "";
    previewToken.value = "";
    return;
  }
  loading.value = true;
  failed.value = false;
  try {
    const { data } = await previewApi.render(key, undefined, { ...stellarContext?.value, mode: "content" });
    if (generation !== previewGeneration) return;
    html.value = data.document;
    previewToken.value = data.preview_token;
  } catch {
    if (generation === previewGeneration) failed.value = true;
  } finally {
    if (generation === previewGeneration) loading.value = false;
  }
}
onBeforeUnmount(() => {
  previewGeneration++;
});

onMounted(renderPreview);
onMounted(() => {
  void loadPluginCatalog();
});
watch([raw, () => stellarContext?.value], renderPreview);

const srcdoc = computed(() => html.value);
function onPreviewMessage(event: MessageEvent) {
  if (
    event.source !== iframeRef.value?.contentWindow ||
    event.data?.token !== previewToken.value ||
    event.data?.type !== "component-preview-resize"
  )
    return;
  const height = event.data.height;
  if (typeof height === "number" && Number.isFinite(height) && height > 0)
    iframeH.value = Math.max(Math.ceil(height), 32);
}
onMounted(() => window.addEventListener("message", onPreviewMessage));
onBeforeUnmount(() => window.removeEventListener("message", onPreviewMessage));
function onIframeLoad() {
  /* 尺寸由子文档ResizeObserver异步回传。 */
}

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
      class="rounded-lg transition-shadow"
      :class="
        selected
          ? 'ring-2 ring-chart-4/40'
          : 'cursor-pointer hover:outline hover:outline-1 hover:outline-dashed hover:outline-[var(--color-border-strong)]'
      "
    >
      <p v-if="failed" class="rounded-lg border border-state-error/30 px-3 py-4 text-center text-xs text-state-error">
        预览渲染失败，可点击编辑检查标签语法
      </p>
      <!-- 未选中时 pointer-events 关闭：整卡任意处点击即选中节点出气泡；选中后恢复可交互/滚动 -->
      <iframe
        v-else-if="html"
        ref="iframeRef"
        :srcdoc="srcdoc"
        :style="{ height: iframeH + 'px' }"
        :class="['block w-full border-0 bg-transparent', !selected && 'pointer-events-none']"
        allowtransparency="true"
        sandbox="allow-scripts"
        title="标签预览"
        @load="onIframeLoad"
      ></iframe>
      <p
        v-else
        class="rounded-lg border border-dashed border-border px-3 py-4 text-center text-xs text-text-quaternary"
      >
        {{ loading ? "渲染中…" : "空标签，选中后点击编辑填写内容" }}
      </p>
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
