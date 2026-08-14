<script setup lang="ts">
import { ref, watch, nextTick, onMounted, onBeforeUnmount } from "vue";
import { Icon } from"@iconify/vue";
import { previewApi, attachmentApi } from"@/api";
import AttachmentPickerModal from"@/components/AttachmentPickerModal.vue";
import { useLocalDraft } from"@/composables/useLocalDraft";

// 可复用 Markdown 编辑器：工具栏 + 分屏实时预览（后端 goldmark 渲染）
// + 图片粘贴/拖拽直传 + 快捷键 + localStorage 草稿保护。
// toolbar=false 时隐藏内置工具栏，由宿主页面通过 expose 的动作自建工具栏（如 PostEditor）。
// 注意：Boolean prop 缺省会被 Vue 转为 false，必须用 withDefaults 默认 true 保证老调用方不受影响。
const props = withDefaults(
 defineProps<{
 modelValue: string;
 storageKey?: string;
 toolbar?: boolean;
 }>(),
 { storageKey: undefined, toolbar: true }
);
const emit = defineEmits<{
 (e:"update:modelValue", value: string): void;
 (e:"save"): void;
 (e:"modeChange", mode: ViewMode): void;
}>();

const textareaRef = ref<HTMLTextAreaElement | null>(null);
const iframeRef = ref<HTMLIFrameElement | null>(null);

type ViewMode ="edit" |"split" |"preview";
// 默认即时渲染分屏（左 MD 右实时预览），选择持久化到 localStorage
const MODE_KEY ="md-editor:mode";
function loadMode(): ViewMode {
 const saved = localStorage.getItem(MODE_KEY);
 return saved ==="edit" || saved ==="split" || saved ==="preview" ? saved :"split";
}
const mode = ref<ViewMode>(loadMode());
const rendering = ref(false);
const pickerOpen = ref(false);
const uploadError = ref("");

// 本地草稿保护（与 RichTextEditor 共用同一 storageKey 草稿）
const { draftBanner, scheduleDraft, clearDraft, restoreDraft, discardDraft, checkDraft } = useLocalDraft({
 storageKey: props.storageKey,
 getValue: () => props.modelValue,
 setValue: (v) => emit("update:modelValue", v),
});

// ---------- iframe 沙箱预览：引入前台全量样式（main.css/tag-plugins.css/highlight.css），
// 与 PluginLibrary.vue 的 srcdoc 预览同模式；标签插件/代码高亮/KaTeX 与前台完全一致，
// 且样式与 Console UI 完全隔离。srcdoc 只加载一次，内容更新直接写 .md-text，避免闪烁。 ----------

// KaTeX 定界符与前台 head.html 保持一致
const mathDelimiters = [
 { left:"$$", right:"$$", display: true },
 { left:"\\[", right:"\\]", display: true },
 { left:"$", right:"$", display: false },
 { left:"\\(", right:"\\)", display: false },
];

// 初始主题跟随 Console（深色-only，html[data-theme=dark] 恒挂）
const previewSrcdoc = `<!DOCTYPE html>
<html data-theme="dark">
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
 /* isolated iframe fallback keeps the console card token */ body{padding:16px 20px;background:var(--color-card, #1c1c1e);}
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
 if (typeof win?.renderMathInElement ==="function") {
 win.renderMathInElement(container, { delimiters: mathDelimiters, throwOnError: false });
 }
}

// 深色-only：iframe 固定 data-theme=dark，无需观察切换
onMounted(() => {
 // 检测 localStorage 是否有未保存草稿（MarkdownEditor 无 round-trip，直接检测即可）
 checkDraft();
 // 默认分屏/预览模式时首次挂载即渲染
 if (mode.value !=="edit") schedulePreview(true);
});

// ---------- 实时预览（防抖 400ms，走后端 goldmark 管线，Stellar 标签插件真实呈现） ----------

let previewTimer: ReturnType<typeof setTimeout> | undefined;

// Spec 62：预览请求序号守卫——连续输入时先发出的慢响应不得覆盖新内容；
// 仅当请求发起时是最新一版（序号匹配）才应用渲染结果。
let previewSeq = 0;
const PREVIEW_DEBOUNCE_MS = 400;
// 大文档保护：正文超过阈值时提高防抖间隔，降低每停顿一轮的全量网络渲染频率
const PREVIEW_LARGE_DOC_MS = 800;
const PREVIEW_LARGE_DOC_BYTES = 50 * 1024;

async function refreshPreview() {
 const seq = ++previewSeq;
 rendering.value = true;
 try {
 const { data } = await previewApi.render(props.modelValue);
 if (seq !== previewSeq) return; // 过期响应丢弃（已有更新的输入在途）
 applyPreview(data.html);
 } catch {
 if (seq !== previewSeq) return;
 applyPreview('<p style="color:var(--color-state-error, #FF453A)">预览渲染失败</p>');
 } finally {
 if (seq === previewSeq) rendering.value = false;
 }
}

function previewDebounceMs() {
 const bytes = new TextEncoder().encode(props.modelValue).length;
 return bytes > PREVIEW_LARGE_DOC_BYTES ? PREVIEW_LARGE_DOC_MS : PREVIEW_DEBOUNCE_MS;
}

function schedulePreview(immediate = false) {
 if (mode.value ==="edit") return;
 clearTimeout(previewTimer);
 previewTimer = setTimeout(refreshPreview, immediate ? 0 : previewDebounceMs());
}

function setMode(next: ViewMode) {
 mode.value = next;
 localStorage.setItem(MODE_KEY, next);
 emit("modeChange", next);
 if (next !=="edit") {
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
 if (mode.value !=="split" || syncingScroll) return;
 const ta = textareaRef.value;
 const doc = iframeRef.value?.contentDocument;
 if (!ta || !doc?.scrollingElement) return;
 syncingScroll = true;
 syncScrollRatio(ta, doc.scrollingElement as HTMLElement);
 releaseSyncGuard();
}

function onPreviewScroll() {
 if (mode.value !=="split" || syncingScroll) return;
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

// ---------- 本地草稿保护（逻辑已抽到 useLocalDraft composable） ----------

onBeforeUnmount(() => {
 clearTimeout(previewTimer);
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
function wrapSelection(before: string, after: string, placeholder ="") {
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

// 按行号滚动定位（供右侧大纲面板点击跳转）：光标置于行首，编辑区按行比例滚动，
// 预览区在分屏模式下由 scroll 事件自动同步；仅预览模式直接按比例滚 iframe。
function scrollToLine(line: number) {
 const lines = props.modelValue.split("\n");
 const total = Math.max(lines.length, 1);
 const ratio = Math.min(line / total, 1);
 if (mode.value !=="preview") {
 const ta = textareaRef.value;
 if (!ta) return;
 let offset = 0;
 for (let i = 0; i < line && i < lines.length; i++) offset += lines[i].length + 1;
 ta.focus();
 ta.setSelectionRange(offset, offset);
 ta.scrollTop = ratio * (ta.scrollHeight - ta.clientHeight);
 } else {
 const el = iframeRef.value?.contentDocument?.scrollingElement as HTMLElement | null;
 if (el) el.scrollTop = ratio * (el.scrollHeight - el.clientHeight);
 }
}

function openImageUpload() {
 fileInputRef.value?.click();
}

function openImagePicker() {
 pickerOpen.value = true;
}

// 对外暴露：草稿清理 + 选区动作/插入 + 模式切换 + 滚动定位，供 PostEditor 外置工具栏与大纲面板调用
defineExpose({
 clearDraft,
 insertBlock,
 wrapSelection,
 prefixLines,
 setMode,
 getMode: () => mode.value,
 scrollToLine,
 openImageUpload,
 openImagePicker,
});

const tableTemplate =
"\n| 表头 | 表头 |\n| --- | --- |\n| 内容 | 内容 |\n";

// 内置工具栏动作定义（命名避开 toolbar prop）
const toolbarActions = [
 { icon:"ri:bold", title:"加粗 (Cmd/Ctrl+B)", action: () => wrapSelection("**","**","加粗文本") },
 { icon:"ri:italic", title:"斜体 (Cmd/Ctrl+I)", action: () => wrapSelection("*","*","斜体文本") },
 { icon:"ri:strikethrough", title:"删除线", action: () => wrapSelection("~~","~~","删除线") },
 { icon:"ri:heading", title:"标题", action: () => prefixLines("##") },
 { icon:"ri:link", title:"链接 (Cmd/Ctrl+K)", action: () => wrapSelection("[","](url)","链接文字") },
 { icon:"ri:double-quotes-l", title:"引用", action: () => prefixLines(">") },
 { icon:"ri:code-line", title:"行内代码", action: () => wrapSelection("`","`","code") },
 { icon:"ri:code-box-line", title:"代码块", action: () => wrapSelection("\n```\n","\n```\n","code") },
 { icon:"ri:list-unordered", title:"无序列表", action: () => prefixLines("-") },
 { icon:"ri:list-ordered", title:"有序列表", action: () => prefixLines("", true) },
 { icon:"ri:table-2", title:"插入表格", action: () => insertBlock(tableTemplate) },
];

// ---------- 图片上传（工具栏按钮 / 粘贴 / 拖拽） ----------

const fileInputRef = ref<HTMLInputElement | null>(null);

async function uploadAndInsert(file: File) {
 uploadError.value ="";
 const placeholder = `![上传中 ${file.name}...]()`;
 insertBlock(placeholder);
 try {
 const { data } = await attachmentApi.upload(file);
 const md = `![${file.name}](${data.url})`;
 const replaced = props.modelValue.replace(placeholder, md);
 emit("update:modelValue", replaced);
 } catch (e: any) {
 emit("update:modelValue", props.modelValue.replace(placeholder,""));
 uploadError.value = e.response?.data?.error || `图片 ${file.name} 上传失败`;
 }
}

function onPickFiles(e: Event) {
 const input = e.target as HTMLInputElement;
 for (const file of Array.from(input.files || [])) {
 if (file.type.startsWith("image/")) uploadAndInsert(file);
 }
 input.value ="";
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
 if (mod && e.key ==="s") {
 e.preventDefault();
 emit("save");
 return;
 }
 if (mod && e.key ==="b") {
 e.preventDefault();
 wrapSelection("**","**","加粗文本");
 return;
 }
 if (mod && e.key ==="i") {
 e.preventDefault();
 wrapSelection("*","*","斜体文本");
 return;
 }
 if (mod && e.key ==="k") {
 e.preventDefault();
 wrapSelection("[","](url)","链接文字");
 return;
 }
 if (e.key ==="Tab") {
 e.preventDefault();
 insertBlock("");
 }
}

const modeButtons: { value: ViewMode; icon: string; title: string }[] = [
 { value:"edit", icon:"ri:edit-2-line", title:"仅编辑" },
 { value:"split", icon:"ri:layout-column-line", title:"即时渲染（分屏预览）" },
 { value:"preview", icon:"ri:eye-line", title:"仅预览" },
];
</script>

<template>
 <div class="border border-border rounded-xl overflow-hidden bg-card">
 <!-- 草稿恢复条幅 -->
 <div
 v-if="draftBanner"
 class="flex items-center justify-between gap-3 px-4 py-2 text-sm text-state-warning bg-state-warning/100/10 border-b border-state-warning/30"
 >
 <span class="flex items-center gap-1.5">
 <Icon icon="ri:draft-line" />检测到本地未保存草稿，是否恢复？
 </span>
 <span class="flex gap-2 shrink-0">
 <button @click="restoreDraft" class="px-2.5 py-1 text-xs font-medium rounded-md bg-state-warning text-foreground hover:bg-state-warning">恢复</button>
 <button @click="discardDraft" class="px-2.5 py-1 text-xs rounded-md text-state-warning hover:bg-state-warning/15">丢弃</button>
 </span>
 </div>

 <!-- 工具栏（toolbar=false 时由宿主页面外置） -->
 <div v-if="toolbar" class="flex items-center gap-0.5 px-2 py-1.5 border-b border-border flex-wrap">
 <button
 v-for="tool in toolbarActions"
 :key="tool.icon"
 type="button"
 :title="tool.title"
 @click="tool.action"
 class="p-1.5 rounded-md text-text-tertiary hover:text-foreground hover:bg-accent"
 >
 <Icon :icon="tool.icon" class="text-lg" />
 </button>

 <span class="w-px h-5 mx-1 bg-accent"></span>

 <button
 type="button"
 title="上传图片"
 @click="fileInputRef?.click()"
 class="p-1.5 rounded-md text-text-tertiary hover:text-foreground hover:bg-accent"
 >
 <Icon icon="ri:image-add-line" class="text-lg" />
 </button>
 <button
 type="button"
 title="从图床选择"
 @click="pickerOpen = true"
 class="p-1.5 rounded-md text-text-tertiary hover:text-foreground hover:bg-accent"
 >
 <Icon icon="ri:gallery-line" class="text-lg" />
 </button>
 <input ref="fileInputRef" type="file" accept="image/*" multiple class="hidden" @change="onPickFiles" />

 <span class="flex-1"></span>

 <!-- 编辑 / 分屏 / 预览 三态切换 -->
 <div class="flex items-center gap-0.5 p-0.5 bg-secondary rounded-lg">
 <button
 v-for="btn in modeButtons"
 :key="btn.value"
 type="button"
 :title="btn.title"
 @click="setMode(btn.value)"
 :class="[
 'p-1.5 rounded-md text-base transition-colors',
 mode === btn.value
 ? 'bg-card text-primary shadow-sm'
 : 'text-text-tertiary hover:text-foreground',
 ]"
 >
 <Icon :icon="btn.icon" />
 </button>
 </div>
 </div>

 <p v-if="uploadError" class="px-4 py-2 text-xs text-state-error bg-state-error/10 border-b border-state-error/20">
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
 'w-full px-4 py-3 text-sm font-mono leading-relaxed border-0 focus:outline-none focus:ring-0 resize-y min-h-[560px] bg-transparent ',
 mode === 'split' ? 'border-r border-border resize-none' : '',
 ]"
 ></textarea>

 <div
 v-show="mode !== 'edit'"
 class="relative min-h-[560px]"
 >
 <span
 v-if="rendering"
 class="absolute top-2 right-3 z-10 text-[11px] text-text-quaternary"
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
