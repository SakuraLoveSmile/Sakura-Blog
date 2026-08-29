<script lang="ts">
// module 级预览缓存（按 raw 全文缓存渲染结果，跨节点实例共享）
const previewCache = new Map<string, string>();
export default {};
</script>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from "vue";
import { NodeViewWrapper, nodeViewProps } from "@tiptap/vue-3";
import { Icon } from "@iconify/vue";
import { pluginApi, previewApi } from "@/api";
import StellarTagPropertyPopover from "./StellarTagPropertyPopover.vue";
import type { PluginMeta, PluginVariant } from "@/modules/plugins/plugin-types";
import { loadPluginCatalog } from "@/modules/plugins/usePluginCatalog";
import { formForVariant } from "@/modules/plugins/shortcode";
import { tagNameOf, typeLabelOf } from "./stellar-tag";

// Stellar 标签节点的 NodeView：选中时浮出 Halo 同款操作气泡（类型标签 + 编辑/删除），iframe 渲染前台真实效果。
const props = defineProps(nodeViewProps);

const raw = computed(() => (props.node.attrs.raw as string) || "");
const typeLabel = computed(() => typeLabelOf(raw.value));

const html = ref("");
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

async function renderPreview() {
  const key = raw.value;
  if (!key.trim()) {
    html.value = "";
    return;
  }
  // links/moments 读取当前站点数据，不能使用跨节点/跨页面的长期缓存；否则新增友链、
  // 编辑瞬间后重新打开编辑器仍会展示旧卡片。其它纯内容标签继续复用缓存，避免重复请求。
  const dynamicTags = new Set(["links", "moments"]);
  const cacheable = !dynamicTags.has(tagNameOf(key).toLowerCase());
  if (cacheable) {
    const cached = previewCache.get(key);
    if (cached !== undefined) {
      html.value = cached;
      failed.value = false;
      return;
    }
  }
  loading.value = true;
  failed.value = false;
  try {
    const { data } = await previewApi.render(key);
    if (cacheable) previewCache.set(key, data.html);
    // 请求返回时 raw 可能已被编辑更新，只有仍一致才展示
    if (raw.value === key) html.value = data.html;
  } catch {
    if (raw.value === key) failed.value = true;
  } finally {
    loading.value = false;
  }
}

onMounted(renderPreview);
onMounted(() => {
  void loadPluginCatalog();
});
watch(raw, renderPreview);

// iframe srcdoc：深色-only，固定 dark 主题
// body 背景透明与编辑器融合（无缝渲染）；首尾 margin 归零减少卡片余白
const srcdoc = computed(() => {
  // {% friends %} 等数据服务标签输出 ds-* 容器，需前端 service JS 抓数据渲染卡片：
  // 预览注入最小 utils/def shim + 对应 service 脚本（仅含 ds-friends 容器时）。
  const hasFriendsService = /ds-friends/.test(html.value);
  const hasMarkdownService = /ds-mdrender/.test(html.value);
  const dsBootstrap =
    hasFriendsService || hasMarkdownService
      ? `<script>
window.def = { avatar: 'https://gcore.jsdelivr.net/gh/cdn-x/placeholder@1.0.12/avatar/round/3442075.svg' };
window.wrapLazyloadImages = function () {};
window.utils = {
 request: function (el, api, cb) { fetch(api).then(cb).catch(function () {}); },
 dom: function (sel) {
  var els = typeof sel === 'string' ? Array.prototype.slice.call(document.querySelectorAll(sel)) : (sel && sel.nodeType === 1 ? [sel] : Array.prototype.slice.call(sel || []));
  var apiObj = {
   find: function (s) { var r = []; els.forEach(function (el) { Array.prototype.push.apply(r, el.querySelectorAll(s)); }); return window.utils.dom(r); },
   append: function (c) { els.forEach(function (el) { el.insertAdjacentHTML('beforeend', c); }); return apiObj; }
  };
  return apiObj;
 }
};
<\/script>${
          hasMarkdownService
            ? `<script src="/assets/vendor/marked/marked.umd.min.js?v=tagprev1"><\/script>
<script src="/assets/js/services/mdrender.js?v=tagprev1"><\/script>`
            : ""
        }${
          hasFriendsService
            ? `<script src="/assets/js/services/friends_and_posts.js?v=tagprev1"><\/script>
<script src="/assets/js/services/friends.js?v=tagprev1"><\/script>`
            : ""
        }`
      : "";
  return `<!DOCTYPE html>
<html data-theme="dark">
<head>
<meta charset="utf-8">
<link rel="stylesheet" href="/assets/css/main.css">
<link rel="stylesheet" href="/assets/css/tag-plugins.css">
<link rel="stylesheet" href="/assets/css/highlight.css">
<style>
html,body{margin:0;padding:0;}
html{scroll-behavior:auto !important;color-scheme:dark;background:transparent;}
body{padding:0;background:transparent;overflow:hidden;}
.md-text{max-width:100%;}
.md-text,.md-text.content{padding:0 !important;}
.md-text>:first-child{margin-top:0;}
.md-text>:last-child{margin-bottom:0;}
</style>
</head>
<body><article class="md-text content">${html.value}</article>${dsBootstrap}</body>
</html>`;
});

// iframe 加载后按内容自适应高度（上限 500px，超出内部滚动）；
// 用 .md-text 实际盒高测量，body 无 padding 保证所见即所得；
// 图片等异步资源加载后延时补测一次
function measure() {
  const doc = iframeRef.value?.contentDocument;
  const art = doc?.querySelector(".md-text");
  if (!art) return;
  const h = Math.ceil(art.getBoundingClientRect().height);
  iframeH.value = Math.min(Math.max(h, 32), 500);
}
function onIframeLoad() {
  measure();
  setTimeout(measure, 600);
  setTimeout(measure, 1600); // service 异步抓数据渲染后补测高度
}

// ---------- 编辑（优先属性表单，解析失败时保留源码编辑） ----------

const editing = ref(false);
const editingLoading = ref(false);
const editingError = ref("");
const editingPlugin = ref<PluginMeta | null>(null);
const editingVariant = ref<PluginVariant | undefined>(undefined);
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
        sandbox="allow-same-origin allow-scripts"
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
.stellar-tag-node.is-dragging iframe {
  display: none !important;
}
.stellar-tag-node.is-dragging {
  opacity: 0.5;
}
</style>
