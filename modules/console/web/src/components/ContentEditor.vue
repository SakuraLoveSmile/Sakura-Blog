<script setup lang="ts">
import { ref, computed, defineAsyncComponent } from "vue";
import { Icon } from "@iconify/vue";
import MarkdownEditor from "@/components/MarkdownEditor.vue";
// Spec 62：tiptap chunk 按需挂载；编辑器偏好与文章编辑器统一（post-editor:kind）
const RichTextEditor = defineAsyncComponent(() => import("@/components/RichTextEditor.vue"));

// 通用内容编辑器：内置富文本/Markdown 双模式切换 + 简化工具栏
// 供笔记页、文库文档、笔记本笔记等非文章/页面的编辑场景复用
// 文章/页面编辑器已使用更复杂的 EditorToolbar，不走本组件
type EditorKind = "rich" | "markdown";
type EditorMode = "edit" | "split" | "preview";

const props = withDefaults(
  defineProps<{
    modelValue: string;
    storageKey?: string;
    placeholder?: string;
    minHeight?: string;
  }>(),
  {
    placeholder: "开始写作…",
    minHeight: "400px",
  },
);

const emit = defineEmits<{
  (e: "update:modelValue", value: string): void;
  (e: "save"): void;
}>();

const mdRef = ref<InstanceType<typeof MarkdownEditor> | null>(null);
// RTE 异步组件暴露方法接口（defineAsyncComponent 无法推导 InstanceType）
interface RichEditorExpose {
  activeStates: Record<string, boolean>;
  applyFormat: (key: string) => void;
  insertMarkdown: (text: string) => void;
  scrollToHeading: (index: number) => void;
  openImageUpload: () => void;
  openImagePicker: () => void;
  clearDraft: () => void;
}
const richRef = ref<RichEditorExpose | null>(null);

// 编辑器种类选择（localStorage 持久化）
// Spec 62：与文章编辑器共用 post-editor:kind，文库/笔记跟随全局编辑器偏好
const KIND_KEY = "post-editor:kind";
function loadKind(): EditorKind {
  return localStorage.getItem(KIND_KEY) === "markdown" ? "markdown" : "rich";
}
const editorKind = ref<EditorKind>(loadKind());
function onKindChange(e: Event) {
  const v = (e.target as HTMLSelectElement).value as EditorKind;
  editorKind.value = v;
  localStorage.setItem(KIND_KEY, v);
}

// Markdown 模式下的视图三态
const editorMode = ref<EditorMode>("split");
const editorModeItems: { value: EditorMode; label: string }[] = [
  { value: "edit", label: "仅编辑" },
  { value: "split", label: "即时渲染" },
  { value: "preview", label: "仅预览" },
];
function onModeChange(e: Event) {
  const v = (e.target as HTMLSelectElement).value as EditorMode;
  setEditorMode(v);
}
function setEditorMode(m: EditorMode) {
  editorMode.value = m;
  mdRef.value?.setMode(m);
}

// 工具栏格式按钮激活态（仅富文本模式）
const toolbarActive = computed<Record<string, boolean>>(() =>
  editorKind.value === "rich" ? (richRef.value?.activeStates ?? {}) : {},
);

// Markdown 格式映射表
const tableTemplate = "\n| 表头 | 表头 |\n| --- | --- |\n| 内容 | 内容 |\n";
const mdFormatMap: Record<string, () => void> = {
  bold: () => mdRef.value?.wrapSelection("**", "**", "加粗文本"),
  italic: () => mdRef.value?.wrapSelection("*", "*", "斜体文本"),
  strike: () => mdRef.value?.wrapSelection("~~", "~~", "删除线"),
  heading: () => mdRef.value?.prefixLines("## ", false),
  link: () => mdRef.value?.wrapSelection("[", "](url)", "链接文字"),
  quote: () => mdRef.value?.prefixLines("> ", false),
  code: () => mdRef.value?.wrapSelection("`", "`", "code"),
  codeblock: () => mdRef.value?.wrapSelection("\n```\n", "\n```\n", "code"),
  ul: () => mdRef.value?.prefixLines("- ", false),
  ol: () => mdRef.value?.prefixLines("", true),
  table: () => mdRef.value?.insertBlock(tableTemplate),
};

// 工具栏动作分发
function onFormat(id: string) {
  if (editorKind.value === "rich") {
    richRef.value?.applyFormat(id);
  } else {
    mdFormatMap[id]?.();
  }
}

function onUpload() {
  (editorKind.value === "rich" ? richRef.value : mdRef.value)?.openImageUpload();
}

function onPick() {
  (editorKind.value === "rich" ? richRef.value : mdRef.value)?.openImagePicker();
}

function onInsert(text: string) {
  if (editorKind.value === "rich") {
    richRef.value?.insertMarkdown(text);
  } else {
    mdRef.value?.insertBlock(text);
  }
}

// 清除草稿（供外部保存后调用）
function clearDraft() {
  (editorKind.value === "rich" ? richRef.value : mdRef.value)?.clearDraft();
}

defineExpose({ clearDraft, onInsert });

// 工具栏格式化按钮
const formatTools = [
  { id: "bold", icon: "ri:bold", title: "加粗 (Cmd/Ctrl+B)" },
  { id: "italic", icon: "ri:italic", title: "斜体 (Cmd/Ctrl+I)" },
  { id: "strike", icon: "ri:strikethrough", title: "删除线" },
  { id: "heading", icon: "ri:heading", title: "标题" },
  { id: "link", icon: "ri:link", title: "链接" },
  { id: "quote", icon: "ri:double-quotes-l", title: "引用" },
  { id: "code", icon: "ri:code-line", title: "行内代码" },
  { id: "codeblock", icon: "ri:code-box-line", title: "代码块" },
  { id: "ul", icon: "ri:list-unordered", title: "无序列表" },
  { id: "ol", icon: "ri:list-ordered", title: "有序列表" },
  { id: "table", icon: "ri:table-2", title: "表格" },
];

const iconBtnCls =
  "p-1.5 rounded-md text-text-quaternary hover:text-text-quaternary hover:bg-secondary transition-colors dark:text-muted-foreground dark:hover:text-foreground dark:hover:bg-accent";
</script>

<template>
  <div class="rounded-xl border border-border bg-white dark:border-border-strong dark:bg-secondary">
    <!-- 简化工具栏 -->
    <div class="flex flex-wrap items-center gap-0.5 border-b border-border px-2 py-1.5 dark:border-border-strong">
      <!-- 格式化按钮组 -->
      <button
        v-for="tool in formatTools"
        :key="tool.id"
        type="button"
        :title="tool.title"
        :class="[
          'p-1.5 rounded-md transition-colors',
          toolbarActive[tool.id] ? 'bg-secondary text-chart-4 dark:bg-accent dark:text-chart-4' : iconBtnCls,
        ]"
        @click="onFormat(tool.id)"
      >
        <Icon :icon="tool.icon" class="text-lg" />
      </button>

      <span class="mx-1 h-5 w-px bg-accent dark:bg-accent"></span>

      <!-- 图片按钮 -->
      <button type="button" title="上传图片" :class="iconBtnCls" @click="onUpload">
        <Icon icon="ri:image-add-line" class="text-lg" />
      </button>
      <button type="button" title="从图床选择" :class="iconBtnCls" @click="onPick">
        <Icon icon="ri:gallery-line" class="text-lg" />
      </button>

      <span class="flex-1"></span>

      <!-- 编辑器种类选择 -->
      <div class="relative flex items-center gap-1">
        <select
          :value="editorKind"
          @change="onKindChange"
          class="cursor-pointer appearance-none rounded-md border border-border bg-white px-2 py-1 text-xs text-text-quaternary outline-none dark:border-border-strong dark:bg-accent dark:text-text-secondary"
          title="切换编辑器"
        >
          <option value="rich">富文本</option>
          <option value="markdown">Markdown</option>
        </select>
      </div>

      <!-- Markdown 视图三态 -->
      <div v-if="editorKind === 'markdown'" class="relative flex items-center gap-1 ml-1">
        <select
          :value="editorMode"
          @change="onModeChange"
          class="cursor-pointer appearance-none rounded-md border border-border bg-white px-2 py-1 text-xs text-text-quaternary outline-none dark:border-border-strong dark:bg-accent dark:text-text-secondary"
          title="视图模式"
        >
          <option v-for="m in editorModeItems" :key="m.value" :value="m.value">{{ m.label }}</option>
        </select>
      </div>
    </div>

    <!-- 编辑器主体 -->
    <RichTextEditor
      v-if="editorKind === 'rich'"
      ref="richRef"
      :model-value="modelValue"
      :storage-key="storageKey"
      @update:model-value="emit('update:modelValue', $event)"
      @save="emit('save')"
    />
    <MarkdownEditor
      v-else
      ref="mdRef"
      :model-value="modelValue"
      :toolbar="false"
      :storage-key="storageKey"
      @update:model-value="emit('update:modelValue', $event)"
      @save="emit('save')"
      @mode-change="(m: any) => (editorMode = m)"
    />
  </div>
</template>

<style scoped>
/* 调整内嵌编辑器的边框，因为外层已经有边框了 */
:deep(.rounded-xl.border) {
  border: none;
  border-radius: 0;
}
:deep(.rte-content) {
  border: none;
}
:deep(.rte-content .ProseMirror) {
  min-height: v-bind(minHeight);
}
:deep(textarea) {
  min-height: v-bind(minHeight) !important;
}
</style>
