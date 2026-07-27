<script setup lang="ts">
import { ref, watch, nextTick, onMounted, onBeforeUnmount } from "vue";
import { Icon } from "@iconify/vue";
import { previewApi, attachmentApi } from "@/api";
import AttachmentPickerModal from "@/components/AttachmentPickerModal.vue";

// 可复用 Markdown 编辑器：工具栏 + 分屏实时预览（后端 goldmark 渲染）
// + 图片粘贴/拖拽直传 + 快捷键 + localStorage 草稿保护。
const props = defineProps<{
  modelValue: string;
  storageKey?: string;
}>();
const emit = defineEmits<{
  (e: "update:modelValue", value: string): void;
  (e: "save"): void;
  (e: "modeChange", mode: ViewMode): void;
}>();

const textareaRef = ref<HTMLTextAreaElement | null>(null);
const iframeRef = ref<HTMLIFrameElement | null>(null);

type ViewMode = "edit" | "split" | "preview";
// 默认即时渲染分屏（左 MD 右实时预览），选择持久化到 localStorage
const MODE_KEY = "md-editor:mode";
function loadMode(): ViewMode {
  const saved = localStorage.getItem(MODE_KEY);
  return saved === "edit" || saved === "split" || saved === "preview" ? saved : "split";
}
const mode = ref<ViewMode>(loadMode());
const rendering = ref(false);
const pickerOpen = ref(false);
const draftBanner = ref(false);
const uploadError = ref("");

const draftKey = () => (props.storageKey ? `draft:${props.storageKey}` : "");

// ---------- iframe 沙箱预览：引入前台全量样式（main.css/tag-plugins.css/highlight.css），
// 与 PluginLibrary.vue 的 srcdoc 预览同模式；标签插件/代码高亮/KaTeX 与前台完全一致，
// 且样式与 Console UI 完全隔离。srcdoc 只加载一次，内容更新直接写 .md-text，避免闪烁。 ----------

// KaTeX 定界符与前台 head.html 保持一致
const mathDelimiters = [
  { left: "$$", right: "$$", display: true },
  { left: "\\[", right: "\\]", display: true },
  { left: "$", right: "$", display: false },
  { left: "\\(", right: "\\)", display: false },
];

// 初始主题跟随 Console（theme store 已桥接到 html[data-theme]）
const previewSrcdoc = `<!DOCTYPE html>
<html data-theme="${document.documentElement.dataset.theme || "light"}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="stylesheet" href="/assets/css/main.css">
<link rel="stylesheet" href="/assets/css/tag-plugins.css">
<link rel="stylesheet" href="/assets/css/highlight.css">
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/katex@0.16/dist/katex.min.css">
<script defer src="https://cdn.jsdelivr.net/npm/katex@0.16/dist/katex.min.js"><\/script>
<script defer src="https://cdn.jsdelivr.net/npm/katex@0.16/dist/contrib/auto-render.min.js"><\/script>
<style>
  html,body{margin:0;padding:0;}
  /* 关闭 Stellar main.css 的平滑滚动，使分屏同步滚动即时跟随而非动画滞后 */
  html{scroll-behavior:auto !important;}
  body{padding:16px 20px;background:#fff;}
  html[data-theme="dark"] body{background:#1c1c1e;}
  .md-text{max-width:100%;}
</style>
</head>
<body><article class="md-text content"></article></body>
</html>`;

let iframeReady = false;
let pendingHtml: string | null = null;

function onIframeLoad() {
  iframeReady = true;
  if (pendingHtml !== null) {
    applyPreview(pendingHtml);
    pendingHtml = null;
  }
  bindPreviewScroll();
}

// 把渲染结果写入 iframe 内的 .md-text 容器，并补渲染 KaTeX 公式
function applyPreview(html: string) {
  if (!iframeReady) {
    pendingHtml = html;
    return;
  }
  const doc = iframeRef.value?.contentDocument;
  const container = doc?.querySelector(".md-text");
  if (!container) return;
  container.innerHTML = html;
  const win = iframeRef.value?.contentWindow as any;
  if (typeof win?.renderMathInElement === "function") {
    win.renderMathInElement(container, { delimiters: mathDelimiters, throwOnError: false });
  }
}

// Console 亮/暗切换时同步到 iframe（main.css/highlight.css 暗色选择器为 :root[data-theme=dark]）
const themeObserver = new MutationObserver(() => {
  const doc = iframeRef.value?.contentDocument;
  if (doc) {
    doc.documentElement.dataset.theme = document.documentElement.dataset.theme || "light";
  }
});
onMounted(() => {
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
  // 默认分屏/预览模式时首次挂载即渲染
  if (mode.value !== "edit") schedulePreview(true);
});
onBeforeUnmount(() => themeObserver.disconnect());

// ---------- 实时预览（防抖 400ms，走后端 goldmark 管线，Stellar 标签插件真实呈现） ----------

let previewTimer: ReturnType<typeof setTimeout> | undefined;

async function refreshPreview() {
  rendering.value = true;
  try {
    const { data } = await previewApi.render(props.modelValue);
    applyPreview(data.html);
  } catch {
    applyPreview('<p style="color:#ef4444">预览渲染失败</p>');
  } finally {
    rendering.value = false;
  }
}

function schedulePreview(immediate = false) {
  if (mode.value === "edit") return;
  clearTimeout(previewTimer);
  previewTimer = setTimeout(refreshPreview, immediate ? 0 : 400);
}

function setMode(next: ViewMode) {
  mode.value = next;
  localStorage.setItem(MODE_KEY, next);
  emit("modeChange", next);
  if (next !== "edit") {
    schedulePreview(true);
  }
}

// ---------- 编辑区 ↔ 预览区按比例同步滚动（分屏模式） ----------

let syncingScroll = false;
let syncReleaseTimer: ReturnType<typeof setTimeout> | undefined;

function syncScrollRatio(from: HTMLElement, to: HTMLElement) {
  const fromMax = from.scrollHeight - from.clientHeight;
  const toMax = to.scrollHeight - to.clientHeight;
  if (fromMax <= 0 || toMax <= 0) return;
  to.scrollTop = (from.scrollTop / fromMax) * toMax;
}

// 同步完成后用 setTimeout 短延时释放守卫标志（而非 requestAnimationFrame：
// rAF 在后台标签/非渲染场景会被节流甚至不触发，会把 syncingScroll 永久卡在 true）。
function releaseSyncGuard() {
  clearTimeout(syncReleaseTimer);
  syncReleaseTimer = setTimeout(() => (syncingScroll = false), 60);
}

function onEditorScroll() {
  if (mode.value !== "split" || syncingScroll) return;
  const ta = textareaRef.value;
  const doc = iframeRef.value?.contentDocument;
  if (!ta || !doc?.scrollingElement) return;
  syncingScroll = true;
  syncScrollRatio(ta, doc.scrollingElement as HTMLElement);
  releaseSyncGuard();
}

function onPreviewScroll() {
  if (mode.value !== "split" || syncingScroll) return;
  const ta = textareaRef.value;
  const doc = iframeRef.value?.contentDocument;
  if (!ta || !doc?.scrollingElement) return;
  syncingScroll = true;
  syncScrollRatio(doc.scrollingElement as HTMLElement, ta);
  releaseSyncGuard();
}

// iframe 加载完成后绑定预览侧滚动监听（srcdoc 只加载一次）
function bindPreviewScroll() {
  iframeRef.value?.contentWindow?.addEventListener("scroll", onPreviewScroll);
}

watch(
  () => props.modelValue,
  () => {
    schedulePreview();
    scheduleDraft();
  }
);

// ---------- 本地草稿保护（防抖 2s 落 localStorage） ----------

let draftTimer: ReturnType<typeof setTimeout> | undefined;

function scheduleDraft() {
  const key = draftKey();
  if (!key) return;
  clearTimeout(draftTimer);
  draftTimer = setTimeout(() => {
    localStorage.setItem(key, props.modelValue);
  }, 2000);
}

function clearDraft() {
  clearTimeout(draftTimer);
  const key = draftKey();
  if (key) localStorage.removeItem(key);
}

function restoreDraft() {
  const key = draftKey();
  if (!key) return;
  const draft = localStorage.getItem(key);
  if (draft !== null) emit("update:modelValue", draft);
  draftBanner.value = false;
}

function discardDraft() {
  clearDraft();
  draftBanner.value = false;
}

onMounted(() => {
  const key = draftKey();
  if (!key) return;
  const draft = localStorage.getItem(key);
  if (draft !== null && draft !== "" && draft !== props.modelValue) {
    draftBanner.value = true;
  }
});

onBeforeUnmount(() => {
  clearTimeout(previewTimer);
  clearTimeout(draftTimer);
  clearTimeout(syncReleaseTimer);
});

// ---------- 工具栏动作（基于选区包裹/插入，操作后恢复焦点与选区） ----------

function updateValue(next: string, selStart: number, selEnd: number) {
  emit("update:modelValue", next);
  nextTick(() => {
    const ta = textareaRef.value;
    if (!ta) return;
    ta.focus();
    ta.setSelectionRange(selStart, selEnd);
  });
}

// 用 before/after 包裹选区；无选区时插入 placeholder 并选中它
function wrapSelection(before: string, after: string, placeholder = "") {
  const ta = textareaRef.value;
  if (!ta) return;
  const { selectionStart: start, selectionEnd: end } = ta;
  const value = props.modelValue;
  const selected = value.slice(start, end) || placeholder;
  const next =
    value.slice(0, start) + before + selected + after + value.slice(end);
  updateValue(next, start + before.length, start + before.length + selected.length);
}

// 选区各行加前缀（引用/列表）；ordered 时用递增序号
function prefixLines(prefix: string, ordered = false) {
  const ta = textareaRef.value;
  if (!ta) return;
  const value = props.modelValue;
  const { selectionStart: start, selectionEnd: end } = ta;
  const lineStart = value.lastIndexOf("\n", start - 1) + 1;
  const segment = value.slice(lineStart, end);
  const prefixed = segment
    .split("\n")
    .map((line, i) => (ordered ? `${i + 1}. ${line}` : prefix + line))
    .join("\n");
  const next = value.slice(0, lineStart) + prefixed + value.slice(end);
  updateValue(next, lineStart, lineStart + prefixed.length);
}

// 光标处插入文本块
function insertBlock(text: string) {
  const ta = textareaRef.value;
  if (!ta) return;
  const value = props.modelValue;
  const { selectionStart: start, selectionEnd: end } = ta;
  const next = value.slice(0, start) + text + value.slice(end);
  updateValue(next, start + text.length, start + text.length);
}

// 对外暴露：草稿清理 + 光标处插入（供标签插件面板一键插入 shortcode，Spec 29）+ 模式切换（页头下拉同步）
defineExpose({ clearDraft, insertBlock, setMode, getMode: () => mode.value });

const tableTemplate =
  "\n| 表头 | 表头 |\n| --- | --- |\n| 内容 | 内容 |\n";

const toolbar = [
  { icon: "ri:bold", title: "加粗 (Cmd/Ctrl+B)", action: () => wrapSelection("**", "**", "加粗文本") },
  { icon: "ri:italic", title: "斜体 (Cmd/Ctrl+I)", action: () => wrapSelection("*", "*", "斜体文本") },
  { icon: "ri:strikethrough", title: "删除线", action: () => wrapSelection("~~", "~~", "删除线") },
  { icon: "ri:heading", title: "标题", action: () => prefixLines("## ") },
  { icon: "ri:link", title: "链接 (Cmd/Ctrl+K)", action: () => wrapSelection("[", "](url)", "链接文字") },
  { icon: "ri:double-quotes-l", title: "引用", action: () => prefixLines("> ") },
  { icon: "ri:code-line", title: "行内代码", action: () => wrapSelection("`", "`", "code") },
  { icon: "ri:code-box-line", title: "代码块", action: () => wrapSelection("\n```\n", "\n```\n", "code") },
  { icon: "ri:list-unordered", title: "无序列表", action: () => prefixLines("- ") },
  { icon: "ri:list-ordered", title: "有序列表", action: () => prefixLines("", true) },
  { icon: "ri:table-2", title: "插入表格", action: () => insertBlock(tableTemplate) },
];

// ---------- 图片上传（工具栏按钮 / 粘贴 / 拖拽） ----------

const fileInputRef = ref<HTMLInputElement | null>(null);

async function uploadAndInsert(file: File) {
  uploadError.value = "";
  const placeholder = `![上传中 ${file.name}...]()`;
  insertBlock(placeholder);
  try {
    const { data } = await attachmentApi.upload(file);
    const md = `![${file.name}](${data.url})`;
    const replaced = props.modelValue.replace(placeholder, md);
    emit("update:modelValue", replaced);
  } catch (e: any) {
    emit("update:modelValue", props.modelValue.replace(placeholder, ""));
    uploadError.value = e.response?.data?.error || `图片 ${file.name} 上传失败`;
  }
}

function onPickFiles(e: Event) {
  const input = e.target as HTMLInputElement;
  for (const file of Array.from(input.files || [])) {
    if (file.type.startsWith("image/")) uploadAndInsert(file);
  }
  input.value = "";
}

function onPaste(e: ClipboardEvent) {
  const files = Array.from(e.clipboardData?.files || []).filter((f) =>
    f.type.startsWith("image/")
  );
  if (files.length === 0) return;
  e.preventDefault();
  files.forEach(uploadAndInsert);
}

function onDrop(e: DragEvent) {
  const files = Array.from(e.dataTransfer?.files || []).filter((f) =>
    f.type.startsWith("image/")
  );
  if (files.length === 0) return;
  e.preventDefault();
  files.forEach(uploadAndInsert);
}

function onPickerSelect(url: string) {
  pickerOpen.value = false;
  insertBlock(`![](${url})`);
}

// ---------- 快捷键 ----------

function onKeydown(e: KeyboardEvent) {
  const mod = e.metaKey || e.ctrlKey;
  if (mod && e.key === "s") {
    e.preventDefault();
    emit("save");
    return;
  }
  if (mod && e.key === "b") {
    e.preventDefault();
    wrapSelection("**", "**", "加粗文本");
    return;
  }
  if (mod && e.key === "i") {
    e.preventDefault();
    wrapSelection("*", "*", "斜体文本");
    return;
  }
  if (mod && e.key === "k") {
    e.preventDefault();
    wrapSelection("[", "](url)", "链接文字");
    return;
  }
  if (e.key === "Tab") {
    e.preventDefault();
    insertBlock("  ");
  }
}

const modeButtons: { value: ViewMode; icon: string; title: string }[] = [
  { value: "edit", icon: "ri:edit-2-line", title: "仅编辑" },
  { value: "split", icon: "ri:layout-column-line", title: "即时渲染（分屏预览）" },
  { value: "preview", icon: "ri:eye-line", title: "仅预览" },
];
</script>

<template>
  <div class="border border-gray-200 rounded-xl overflow-hidden bg-white dark:bg-gray-800 dark:border-gray-600">
    <!-- 草稿恢复条幅 -->
    <div
      v-if="draftBanner"
      class="flex items-center justify-between gap-3 px-4 py-2 text-sm bg-amber-50 text-amber-800 border-b border-amber-200 dark:bg-amber-500/10 dark:text-amber-400 dark:border-amber-500/20"
    >
      <span class="flex items-center gap-1.5">
        <Icon icon="ri:draft-line" />检测到本地未保存草稿，是否恢复？
      </span>
      <span class="flex gap-2 shrink-0">
        <button @click="restoreDraft" class="px-2.5 py-1 text-xs font-medium rounded-md bg-amber-600 text-white hover:bg-amber-700">恢复</button>
        <button @click="discardDraft" class="px-2.5 py-1 text-xs rounded-md text-amber-700 hover:bg-amber-100 dark:text-amber-400 dark:hover:bg-amber-500/15">丢弃</button>
      </span>
    </div>

    <!-- 工具栏 -->
    <div class="flex items-center gap-0.5 px-2 py-1.5 border-b border-gray-200 flex-wrap dark:border-gray-600">
      <button
        v-for="tool in toolbar"
        :key="tool.icon"
        type="button"
        :title="tool.title"
        @click="tool.action"
        class="p-1.5 rounded-md text-gray-500 hover:text-gray-800 hover:bg-gray-100 dark:text-gray-400 dark:hover:text-gray-100 dark:hover:bg-gray-700"
      >
        <Icon :icon="tool.icon" class="text-lg" />
      </button>

      <span class="w-px h-5 mx-1 bg-gray-200 dark:bg-gray-600"></span>

      <button
        type="button"
        title="上传图片"
        @click="fileInputRef?.click()"
        class="p-1.5 rounded-md text-gray-500 hover:text-gray-800 hover:bg-gray-100 dark:text-gray-400 dark:hover:text-gray-100 dark:hover:bg-gray-700"
      >
        <Icon icon="ri:image-add-line" class="text-lg" />
      </button>
      <button
        type="button"
        title="从图床选择"
        @click="pickerOpen = true"
        class="p-1.5 rounded-md text-gray-500 hover:text-gray-800 hover:bg-gray-100 dark:text-gray-400 dark:hover:text-gray-100 dark:hover:bg-gray-700"
      >
        <Icon icon="ri:gallery-line" class="text-lg" />
      </button>
      <input ref="fileInputRef" type="file" accept="image/*" multiple class="hidden" @change="onPickFiles" />

      <span class="flex-1"></span>

      <!-- 编辑 / 分屏 / 预览 三态切换 -->
      <div class="flex items-center gap-0.5 p-0.5 bg-gray-100 rounded-lg dark:bg-gray-700/50">
        <button
          v-for="btn in modeButtons"
          :key="btn.value"
          type="button"
          :title="btn.title"
          @click="setMode(btn.value)"
          :class="[
            'p-1.5 rounded-md text-base transition-colors',
            mode === btn.value
              ? 'bg-white text-indigo-600 shadow-sm dark:bg-gray-800 dark:text-indigo-400'
              : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200',
          ]"
        >
          <Icon :icon="btn.icon" />
        </button>
      </div>
    </div>

    <p v-if="uploadError" class="px-4 py-2 text-xs text-red-600 bg-red-50 border-b border-red-100 dark:text-red-400 dark:bg-red-500/10 dark:border-red-500/20">
      {{ uploadError }}
    </p>

    <!-- 编辑 / 预览区 -->
    <div :class="mode === 'split' ? 'grid grid-cols-2' : ''">
      <textarea
        v-show="mode !== 'preview'"
        ref="textareaRef"
        :value="modelValue"
        @input="emit('update:modelValue', ($event.target as HTMLTextAreaElement).value)"
        @keydown="onKeydown"
        @paste="onPaste"
        @drop="onDrop"
        @scroll="onEditorScroll"
        placeholder="Markdown 内容...（支持粘贴/拖拽图片直接上传）"
        :class="[
          'w-full px-4 py-3 text-sm font-mono leading-relaxed border-0 focus:outline-none focus:ring-0 resize-y min-h-[560px] bg-transparent dark:text-gray-100 dark:placeholder-gray-500',
          mode === 'split' ? 'border-r border-gray-200 dark:border-gray-600 resize-none' : '',
        ]"
      ></textarea>

      <div
        v-show="mode !== 'edit'"
        class="relative min-h-[560px]"
      >
        <span
          v-if="rendering"
          class="absolute top-2 right-3 z-10 text-[11px] text-gray-400 dark:text-gray-500"
        >渲染中...</span>
        <iframe
          ref="iframeRef"
          :srcdoc="previewSrcdoc"
          @load="onIframeLoad"
          class="w-full min-h-[560px] h-[75vh] border-0 block"
          title="预览"
        ></iframe>
      </div>
    </div>

    <AttachmentPickerModal :open="pickerOpen" @select="onPickerSelect" @close="pickerOpen = false" />
  </div>
</template>
