<script setup lang="ts">
import { ref, reactive, watch, onBeforeUnmount } from"vue";
import { Icon } from"@iconify/vue";
import { useEditor, EditorContent } from"@tiptap/vue-3";
import StarterKit from"@tiptap/starter-kit";
import Link from"@tiptap/extension-link";
import Image from"@tiptap/extension-image";
import Table from"@tiptap/extension-table";
import TableRow from"@tiptap/extension-table-row";
import TableCell from"@tiptap/extension-table-cell";
import TableHeader from"@tiptap/extension-table-header";
import TaskList from"@tiptap/extension-task-list";
import TaskItem from"@tiptap/extension-task-item";
import Placeholder from"@tiptap/extension-placeholder";
import { Markdown } from"tiptap-markdown";
import { StellarTag } from"@/components/editor/stellar-tag";
import { attachmentApi } from"@/api";
import AttachmentPickerModal from"@/components/AttachmentPickerModal.vue";
import { useLocalDraft } from"@/composables/useLocalDraft";

// 所见即所得富文本编辑器（Halo 同款 TipTap 内核）。
// content_md 仍是唯一数据源：tiptap-markdown 负责 Markdown ↔ ProseMirror 双向转换。
// Stellar 标签（{% note %} 等）由 StellarTag 节点可视化渲染（iframe 前台真实效果），
// 序列化时原样回写 shortcode 文本；仅识别独占整行/整块的标签，句中内联用法保留为纯文本。
// 已知限制：序列化可能对普通文本中的 */_ 等符号转义，复杂场景可切 Markdown 模式编辑。
const props = defineProps<{
 modelValue: string;
 storageKey?: string;
}>();
const emit = defineEmits<{
 (e:"update:modelValue", value: string): void;
 (e:"save"): void;
}>();

const uploadError = ref("");
const pickerOpen = ref(false);
const fileInputRef = ref<HTMLInputElement | null>(null);

// 编辑器规范化基线（onCreate 后的自序列化值）：用于区分"真实编辑"与"加载时 round-trip 重排"。
let baseline = props.modelValue;
// baseline 是否已对齐到"真实内容"（API 加载后的内容，非初始空串）。
// onCreate 时若 modelValue 非空则直接就绪；否则等 watch 首次收到外部内容后再就绪。
let baselineReady = false;

// 工具栏激活态（随选区/事务变化更新，供 PostEditor 传给 EditorToolbar 高亮）
const activeStates = reactive<Record<string, boolean>>({});
function syncActive() {
 const ed = editor.value;
 if (!ed) return;
 activeStates.bold = ed.isActive("bold");
 activeStates.italic = ed.isActive("italic");
 activeStates.strike = ed.isActive("strike");
 activeStates.heading = ed.isActive("heading");
 activeStates.link = ed.isActive("link");
 activeStates.quote = ed.isActive("blockquote");
 activeStates.code = ed.isActive("code");
 activeStates.codeblock = ed.isActive("codeBlock");
 activeStates.ul = ed.isActive("bulletList");
 activeStates.ol = ed.isActive("orderedList");
}

// 本地草稿保护（与 MarkdownEditor 共用同一 storageKey 草稿，双模式切换不冲突）
// baseline 传入编辑器规范化基线，避免 round-trip 幻影草稿误弹条幅
const { draftBanner, scheduleDraft, clearDraft, restoreDraft, discardDraft, checkDraft } = useLocalDraft({
 storageKey: props.storageKey,
 getValue: () => props.modelValue,
 setValue: (v) => emit("update:modelValue", v),
 baseline: () => baseline,
});

// ---------- 编辑器实例与 Markdown 双向同步 ----------

// 上一次序列化产物：外部值 === lastMarkdown 说明是自己 emit 出去的回流，跳过 setContent 防循环
let lastMarkdown = props.modelValue;

const editor = useEditor({
 content: props.modelValue,
 extensions: [
 StarterKit,
 Link.configure({ openOnClick: false }),
 // inline：图片作为段落内联节点——块级 Image 序列化时不补空行，
 // 会把紧随其后的标题合并到同一行破坏 Markdown 结构（round-trip 实测）
 Image.configure({ inline: true }),
 Table.configure({ resizable: false }),
 TableRow,
 TableCell,
 TableHeader,
 TaskList,
 TaskItem.configure({ nested: true }),
 Placeholder.configure({ placeholder:"开始写作…（支持粘贴/拖拽图片直接上传）" }),
 StellarTag,
 Markdown.configure({
 html: true,
 tightLists: true,
 linkify: false,
 breaks: false,
 transformPastedText: true,
 }),
 ],
 editorProps: {
 handlePaste: (_view, event) => handleImageFiles(event.clipboardData?.files),
 handleDrop: (_view, event) => handleImageFiles(event.dataTransfer?.files),
 handleKeyDown: (_view, event) => {
 if ((event.metaKey || event.ctrlKey) && event.key ==="s") {
 event.preventDefault();
 emit("save");
 return true;
 }
 return false;
 },
 },
 onCreate: ({ editor }) => {
 // 记录规范化基线（解析->序列化后的值），供草稿判定与 no-op 拦截
 baseline = editor.storage.markdown.getMarkdown() as string;
 lastMarkdown = baseline;
 syncActive();
 // 立即检测一次草稿（覆盖新建文章 / API 已就绪的场景）；
 // 若 API 尚未返回（modelValue 为空），watch 首次收到真实内容时会
 // 重新固化 baseline 并再次 checkDraft，覆盖此次结果
 checkDraft();
 },
 onSelectionUpdate: syncActive,
 onUpdate: ({ editor }) => {
 syncActive();
 const md = editor.storage.markdown.getMarkdown() as string;
 // 与基线相同说明只是加载时规范化回流（非用户编辑），不回写、不落草稿，避免幻影草稿
 if (md === baseline && md === lastMarkdown) return;
 lastMarkdown = md;
 emit("update:modelValue", md);
 // 真实用户编辑：防抖落草稿（watch 中的 scheduleDraft 仅处理外部变更）
 scheduleDraft();
 },
});

// 外部变更（AI 回填 / 版本回滚 / 模式切换 / 草稿恢复 / API 首次加载）-> 重新解析 Markdown 进编辑器
watch(
 () => props.modelValue,
 (v) => {
 // v === lastMarkdown 说明是 onUpdate 自己 emit 的回流，跳过 setContent 防循环；
 // scheduleDraft 已在 onUpdate 中调用，此处不再重复，避免 round-trip 回流落幻影草稿
 if (v === lastMarkdown) return;
 lastMarkdown = v;
 editor.value?.commands.setContent(v, false);
 if (!baselineReady) {
 // 首次外部内容加载（API 在 onCreate 之后才返回）：此时 baseline 仍是空串，
 // 需用真实内容重新固化 baseline，再检测草稿，避免幻影草稿误弹条幅。
 // 注意：此处不调 scheduleDraft，否则会用 API 内容覆盖 localStorage 中的真实草稿
 baseline = editor.value?.storage.markdown.getMarkdown() as string;
 baselineReady = true;
 checkDraft();
 } else {
 // 后续外部变更（AI 回填 / 版本回滚等）：防抖落草稿
 scheduleDraft();
 }
 }
);

onBeforeUnmount(() => editor.value?.destroy());

// ---------- 图片上传（粘贴 / 拖拽 / 工具栏按钮 / 图床选择） ----------

/** 从粘贴/拖拽事件提取图片文件上传并插入；处理了返回 true 阻止默认行为 */
function handleImageFiles(files?: FileList | null): boolean {
 const images = Array.from(files || []).filter((f) => f.type.startsWith("image/"));
 if (images.length === 0) return false;
 images.forEach(uploadAndInsert);
 return true;
}

async function uploadAndInsert(file: File) {
 uploadError.value ="";
 try {
 const { data } = await attachmentApi.upload(file);
 editor.value?.chain().focus().setImage({ src: data.url, alt: file.name }).run();
 } catch (e: any) {
 uploadError.value = e.response?.data?.error || `图片 ${file.name} 上传失败`;
 }
}

function onPickFiles(e: Event) {
 const input = e.target as HTMLInputElement;
 handleImageFiles(input.files);
 input.value ="";
}

function onPickerSelect(url: string) {
 pickerOpen.value = false;
 editor.value?.chain().focus().setImage({ src: url }).run();
}

function openImageUpload() {
 fileInputRef.value?.click();
}

function openImagePicker() {
 pickerOpen.value = true;
}

// ---------- 工具栏格式化命令（id 与 EditorToolbar formatTools 对应） ----------

function applyFormat(id: string) {
 const chain = editor.value?.chain().focus();
 if (!chain) return;
 switch (id) {
 case"bold":
 chain.toggleBold().run();
 break;
 case"italic":
 chain.toggleItalic().run();
 break;
 case"strike":
 chain.toggleStrike().run();
 break;
 case"heading":
 chain.toggleHeading({ level: 2 }).run();
 break;
 case"link": {
 if (editor.value?.isActive("link")) {
 chain.unsetLink().run();
 break;
 }
 const url = window.prompt("链接地址","https://");
 if (url) chain.setLink({ href: url }).run();
 break;
 }
 case"quote":
 chain.toggleBlockquote().run();
 break;
 case"code":
 chain.toggleCode().run();
 break;
 case"codeblock":
 chain.toggleCodeBlock().run();
 break;
 case"ul":
 chain.toggleBulletList().run();
 break;
 case"ol":
 chain.toggleOrderedList().run();
 break;
 case"table":
 chain.insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run();
 break;
 }
}

/** 光标处插入 Markdown 片段（tiptap-markdown 使 insertContent 支持 Markdown 解析），
 * Stellar shortcode 会成为纯文本段落 */
function insertMarkdown(text: string) {
 editor.value?.chain().focus().insertContent(text).run();
}

/** 跳转到第 index 个标题节点（供右侧大纲面板点击定位） */
function scrollToHeading(index: number) {
 const ed = editor.value;
 if (!ed) return;
 let n = -1;
 let target = -1;
 ed.state.doc.descendants((node, pos) => {
 if (node.type.name ==="heading") {
 n++;
 if (n === index) {
 target = pos;
 return false;
 }
 }
 return true;
 });
 if (target < 0) return;
 ed.chain().focus().setTextSelection(target + 1).run();
 const dom = ed.view.nodeDOM(target) as HTMLElement | null;
 dom?.scrollIntoView({ block:"start", behavior:"smooth" });
}

defineExpose({ applyFormat, insertMarkdown, scrollToHeading, openImageUpload, openImagePicker, clearDraft, activeStates });
</script>

<template>
 <!-- 不用 overflow-hidden：避免文首 Stellar 标签节点的悬浮操作气泡被裁剪 -->
 <div class="rounded-xl border border-border bg-card">
 <!-- 草稿恢复条幅（与 MarkdownEditor 同款） -->
 <div
 v-if="draftBanner"
 class="flex items-center justify-between gap-3 rounded-t-xl border-b border-state-warning/30 bg-state-warning/100/10 px-4 py-2 text-sm text-state-warning"
 >
 <span class="flex items-center gap-1.5">
 <Icon icon="ri:draft-line" />检测到本地未保存草稿，是否恢复？
 </span>
 <span class="flex shrink-0 gap-2">
 <button class="rounded-md bg-state-warning px-2.5 py-1 text-xs font-medium text-white hover:bg-state-warning" @click="restoreDraft">恢复</button>
 <button class="rounded-md px-2.5 py-1 text-xs text-state-warning hover:bg-state-warning/15" @click="discardDraft">丢弃</button>
 </span>
 </div>

 <p v-if="uploadError" class="border-b border-state-error/20 bg-state-error/10 px-4 py-2 text-xs text-state-error">
 {{ uploadError }}
 </p>

 <div class="rte-content">
 <EditorContent :editor="editor" />
 </div>

 <input ref="fileInputRef" type="file" accept="image/*" multiple class="hidden" @change="onPickFiles" />
 <AttachmentPickerModal :open="pickerOpen" @select="onPickerSelect" @close="pickerOpen = false" />
 </div>
</template>

<style>
/* ProseMirror 内容为运行时生成，scoped 属性不会附加到内部节点，故用非 scoped +
 .rte-content 命名空间限定。排版观感对照 Halo 编辑器截图。 */
.rte-content .ProseMirror {
 min-height: 560px;
 padding: 1rem 1.5rem;
 outline: none;
 font-size: 0.9375rem;
 line-height: 1.75;
 color: #e5e7eb;
 word-break: break-word;
}

.rte-content .ProseMirror p {
 margin: 0.5em 0;
}
.rte-content .ProseMirror h1,
.rte-content .ProseMirror h2,
.rte-content .ProseMirror h3,
.rte-content .ProseMirror h4,
.rte-content .ProseMirror h5,
.rte-content .ProseMirror h6 {
 font-weight: 700;
 line-height: 1.4;
 margin: 1.2em 0 0.5em;
}
.rte-content .ProseMirror h1 {
 font-size: 1.75em;
 padding-bottom: 0.3em;
 border-bottom: 1px solid var(--color-border);
}
.rte-content .ProseMirror h2 {
 font-size: 1.4em;
}
.rte-content .ProseMirror h3 {
 font-size: 1.2em;
}

.rte-content .ProseMirror ul,
.rte-content .ProseMirror ol {
 padding-left: 1.5em;
 margin: 0.5em 0;
}
.rte-content .ProseMirror ul {
 list-style: disc;
}
.rte-content .ProseMirror ol {
 list-style: decimal;
}
.rte-content .ProseMirror li > p {
 margin: 0.15em 0;
}

/* 任务列表 */
.rte-content .ProseMirror ul[data-type="taskList"] {
 list-style: none;
 padding-left: 0.25em;
}
.rte-content .ProseMirror ul[data-type="taskList"] li {
 display: flex;
 gap: 0.5em;
}
.rte-content .ProseMirror ul[data-type="taskList"] li > label {
 flex-shrink: 0;
 margin-top: 0.35em;
}

.rte-content .ProseMirror blockquote {
 border-left: 3px solid #4b5563;
 padding-left: 1em;
 margin: 0.75em 0;
 color: #9ca3af;
}

.rte-content .ProseMirror code {
 background: var(--color-accent);
 border-radius: 4px;
 padding: 0.15em 0.4em;
 font-size: 0.875em;
 font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
 color: #f87171;
}

/* 代码块：对齐 Stellar 前台观感——深色卡片 + 顶部 macOS 三点灯 + 右上语言标签 */
.rte-content .ProseMirror pre {
 position: relative;
 background: #282c34;
 color: #abb2bf;
 border-radius: var(--radius-base);
 padding: 2.5em 1em 1em;
 margin: 0.75em 0;
 overflow-x: auto;
 font-size: 0.85em;
 line-height: 1.6;
}
.rte-content .ProseMirror pre::before {
 content:"";
 position: absolute;
 top: 0.85em;
 left: 1em;
 width: 12px;
 height: 12px;
 border-radius: 50%;
 background: #fc625d;
 box-shadow: 20px 0 #fdbc40, 40px 0 #35cd4b;
}
/* 右上角语言标签略（CSS 无法去前缀，不显 language- 字样） */
.rte-content .ProseMirror pre code {
 background: none;
 color: inherit;
 padding: 0;
 font-size: inherit;
 font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}

.rte-content .ProseMirror img {
 max-width: 100%;
 border-radius: var(--radius-sm);
 margin: 0.5em 0;
}
.rte-content .ProseMirror img.ProseMirror-selectednode {
 outline: 2px solid #6366f1;
}

.rte-content .ProseMirror table {
 border-collapse: collapse;
 margin: 0.75em 0;
 width: 100%;
 table-layout: fixed;
}
.rte-content .ProseMirror th,
.rte-content .ProseMirror td {
 border: 1px solid #4b5563;
 padding: 0.4em 0.75em;
 vertical-align: top;
}
.rte-content .ProseMirror th {
 background: var(--color-accent);
 font-weight: 600;
 text-align: left;
}
.rte-content .ProseMirror .selectedCell:after {
 content:"";
 position: absolute;
 inset: 0;
 background: color-mix(in srgb, var(--color-chart-4) 12%, transparent);
 pointer-events: none;
}
.rte-content .ProseMirror td,
.rte-content .ProseMirror th {
 position: relative;
}

.rte-content .ProseMirror hr {
 border: none;
 border-top: 1px solid var(--color-border);
 margin: 1.5em 0;
}

.rte-content .ProseMirror a {
 color: var(--color-primary);
 text-decoration: underline;
 cursor: pointer;
}

/* Placeholder（空文档提示） */
.rte-content .ProseMirror p.is-editor-empty:first-child::before {
 content: attr(data-placeholder);
 float: left;
 height: 0;
 pointer-events: none;
 color: var(--color-muted-foreground);
}
</style>
