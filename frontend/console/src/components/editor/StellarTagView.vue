<script lang="ts">
// module 级预览缓存（按 raw 全文缓存渲染结果，跨节点实例共享）
const previewCache = new Map<string, string>();
export default {};
</script>

<script setup lang="ts">
import { ref, computed, watch, onMounted, nextTick } from "vue";
import { NodeViewWrapper, nodeViewProps } from "@tiptap/vue-3";
import { Icon } from "@iconify/vue";
import { previewApi } from "@/api";
import { typeLabelOf } from "./stellar-tag";

// Stellar 标签节点的 NodeView：选中时浮出 Halo 同款操作气泡（类型标签 + 编辑/删除），iframe 渲染前台真实效果。
const props = defineProps(nodeViewProps);

const raw = computed(() => (props.node.attrs.raw as string) || "");
const typeLabel = computed(() => typeLabelOf(raw.value));

const html = ref("");
const loading = ref(false);
const failed = ref(false);
const iframeH = ref(64);
const iframeRef = ref<HTMLIFrameElement | null>(null);

async function renderPreview() {
  const key = raw.value;
  if (!key.trim()) {
    html.value = "";
    return;
  }
  const cached = previewCache.get(key);
  if (cached !== undefined) {
    html.value = cached;
    failed.value = false;
    return;
  }
  loading.value = true;
  failed.value = false;
  try {
    const { data } = await previewApi.render(key);
    previewCache.set(key, data.html);
    // 请求返回时 raw 可能已被编辑更新，只有仍一致才展示
    if (raw.value === key) html.value = data.html;
  } catch {
    if (raw.value === key) failed.value = true;
  } finally {
    loading.value = false;
  }
}

onMounted(renderPreview);
watch(raw, renderPreview);

// iframe srcdoc：前台全量样式管线（同 MarkdownEditor 分屏预览 / 工具栏悬停气泡）。
// body 背景透明与编辑器融合（无缝渲染）；color-scheme 跟随主题避免暗色下白色滚动条；
// 首尾 margin 归零减少卡片余白
const srcdoc = computed(() => {
  const dark = document.documentElement.dataset.theme === "dark";
  return `<!DOCTYPE html>
<html data-theme="${dark ? "dark" : "light"}">
<head>
<meta charset="utf-8">
<link rel="stylesheet" href="/assets/css/main.css">
<link rel="stylesheet" href="/assets/css/tag-plugins.css">
<link rel="stylesheet" href="/assets/css/highlight.css">
<style>
html,body{margin:0;padding:0;}
html{scroll-behavior:auto !important;color-scheme:${dark ? "dark" : "light"};background:transparent;}
body{padding:2px;background:transparent;overflow-x:hidden;}
.md-text{max-width:100%;}
.md-text>:first-child{margin-top:0;}
.md-text>:last-child{margin-bottom:0;}
</style>
</head>
<body><article class="md-text content">${html.value}</article></body>
</html>`;
});

// iframe 加载后按内容自适应高度（上限 500px，超出内部滚动）；
// 用 .md-text 实际盒高 + body padding 测量，避免 body.scrollHeight 含 margin 导致底部余白；
// 图片等异步资源加载后延时补测一次
function measure() {
  const doc = iframeRef.value?.contentDocument;
  const art = doc?.querySelector(".md-text");
  if (!art) return;
  const h = Math.ceil(art.getBoundingClientRect().height) + 4;
  iframeH.value = Math.min(Math.max(h, 32), 500);
}
function onIframeLoad() {
  measure();
  setTimeout(measure, 600);
}

// ---------- 编辑（内联 textarea 改 raw；复杂参数用右侧详情 Tab 的参数表单） ----------

const editing = ref(false);
const draft = ref("");
const textareaRef = ref<HTMLTextAreaElement | null>(null);

function startEdit() {
  draft.value = raw.value;
  editing.value = true;
  nextTick(() => textareaRef.value?.focus());
}

function confirmEdit() {
  editing.value = false;
  const next = draft.value;
  if (next !== raw.value) props.updateAttributes({ raw: next });
}

function cancelEdit() {
  editing.value = false;
}
</script>

<template>
  <NodeViewWrapper class="stellar-tag-node group relative my-2">
    <!-- 左侧 gutter 拖拽把手（悬停卡片浮现，直接拖拽移动节点，无需先选中） -->
    <span
      v-if="!editing"
      data-drag-handle
      contenteditable="false"
      title="拖拽移动"
      class="absolute -left-6 top-1.5 z-40 flex h-6 w-5 cursor-grab items-center justify-center rounded text-gray-300 opacity-0 transition-opacity hover:bg-gray-100 hover:text-gray-500 group-hover:opacity-100 dark:text-gray-600 dark:hover:bg-gray-700 dark:hover:text-gray-300"
    >
      <Icon icon="ri:draggable" class="text-base" />
    </span>

    <!-- 悬浮操作气泡（对照 Halo：白色胶囊 + 底部指向节点的小箭头） -->
    <div
      v-if="selected && !editing"
      contenteditable="false"
      class="stellar-bubble absolute -top-11 left-0 z-50 flex items-center gap-1 rounded-[10px] border border-gray-200 bg-white px-1.5 py-1 shadow-lg dark:border-gray-600 dark:bg-gray-800"
    >
      <!-- 左侧类型标签（纯显示；拖拽由左侧 gutter 把手负责） -->
      <span
        :title="typeLabel"
        class="flex items-center gap-1.5 rounded px-1.5 py-0.5"
      >
        <Icon icon="ri:puzzle-line" class="text-sm text-emerald-500" />
        <span class="text-[13px] font-medium text-gray-700 dark:text-gray-200">{{ typeLabel }}</span>
        <span v-if="loading" class="text-[10px] text-gray-400">渲染中…</span>
      </span>

      <span class="mx-0.5 h-4 w-px bg-gray-200 dark:bg-gray-600"></span>

      <!-- 编辑属性 -->
      <button
        type="button"
        class="group relative rounded p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-gray-400 dark:hover:bg-gray-700 dark:hover:text-gray-200"
        @mousedown.prevent
        @click="startEdit"
      >
        <Icon icon="ri:file-edit-line" class="text-base" />
        <span class="pointer-events-none absolute -top-8 left-1/2 hidden -translate-x-1/2 whitespace-nowrap rounded bg-gray-800 px-2 py-1 text-[11px] text-white group-hover:block dark:bg-gray-700">编辑属性</span>
      </button>

      <!-- 删除（红色） -->
      <button
        type="button"
        class="group relative rounded p-1.5 text-red-500 transition-colors hover:bg-red-50 dark:hover:bg-red-500/10"
        @mousedown.prevent
        @click="deleteNode()"
      >
        <Icon icon="ri:delete-bin-line" class="text-base" />
        <span class="pointer-events-none absolute -top-8 left-1/2 hidden -translate-x-1/2 whitespace-nowrap rounded bg-gray-800 px-2 py-1 text-[11px] text-white group-hover:block dark:bg-gray-700">删除</span>
      </button>

      <!-- 底部指向节点的小箭头 -->
      <span class="absolute -bottom-1 left-5 h-2 w-2 rotate-45 border-b border-r border-gray-200 bg-white dark:border-gray-600 dark:bg-gray-800"></span>
    </div>

    <!-- 编辑态：等宽 textarea 直改 raw -->
    <div v-if="editing" class="space-y-2 rounded-lg border border-gray-200 p-2.5 dark:border-gray-600">
      <textarea
        ref="textareaRef"
        v-model="draft"
        :rows="Math.min(Math.max(draft.split('\n').length + 1, 3), 14)"
        spellcheck="false"
        class="w-full resize-y rounded-lg border border-gray-200 px-3 py-2 font-mono text-xs leading-relaxed focus:border-primary focus:outline-none dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
        @keydown.stop
      ></textarea>
      <div class="flex justify-end gap-2">
        <button
          type="button"
          class="rounded-md px-2.5 py-1 text-xs text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700"
          @click="cancelEdit"
        >取消</button>
        <button
          type="button"
          class="rounded-md bg-secondary px-2.5 py-1 text-xs text-white hover:opacity-90"
          @click="confirmEdit"
        >确定</button>
      </div>
    </div>

    <!-- 无缝预览：与正文融合，悬停淡描边提示可点选，选中 indigo ring -->
    <div
      v-else
      class="rounded-lg transition-shadow"
      :class="selected
        ? 'ring-2 ring-indigo-300 dark:ring-indigo-500/40'
        : 'cursor-pointer hover:outline hover:outline-1 hover:outline-dashed hover:outline-gray-300 dark:hover:outline-gray-600'"
    >
      <p v-if="failed" class="rounded-lg border border-red-200 px-3 py-4 text-center text-xs text-red-500 dark:border-red-500/30">
        预览渲染失败，可点击编辑检查标签语法
      </p>
      <!-- 未选中时 pointer-events 关闭：整卡任意处点击即选中节点出气泡；选中后恢复可交互/滚动 -->
      <iframe
        v-else-if="html"
        ref="iframeRef"
        :srcdoc="srcdoc"
        :style="{ height: iframeH + 'px' }"
        :class="['block w-full rounded-lg border-0 bg-transparent', !selected && 'pointer-events-none']"
        allowtransparency="true"
        sandbox="allow-same-origin allow-scripts"
        title="标签预览"
        @load="onIframeLoad"
      ></iframe>
      <p v-else class="rounded-lg border border-dashed border-gray-200 px-3 py-4 text-center text-xs text-gray-400 dark:border-gray-600">
        {{ loading ? "渲染中…" : "空标签，选中后点击编辑填写内容" }}
      </p>
    </div>
  </NodeViewWrapper>
</template>
