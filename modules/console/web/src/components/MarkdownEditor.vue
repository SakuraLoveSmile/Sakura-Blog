<script setup lang="ts">
import { inject, ref, watch, nextTick, onMounted, onBeforeUnmount } from "vue";
import { stellarEditorContextKey } from "./editor/stellar-context";
import { Icon } from "@iconify/vue";
import { previewApi, attachmentApi, ucApi } from "@/api";
import { preparePreviewDocument, previewFailureMessage } from "@/modules/plugins/preview/preview-document";
import AttachmentPickerModal from "@/components/AttachmentPickerModal.vue";
import { useLocalDraft } from "@/composables/useLocalDraft";
import { createMarkdownBridge } from "./editor/assistant/markdown-bridge";

// 可复用 Markdown 编辑器：工具栏 + 分屏实时预览（后端 goldmark 渲染）
// + 图片粘贴/拖拽直传 + 快捷键 + localStorage 草稿保护。
// toolbar=false 时隐藏内置工具栏，由宿主页面通过 expose 的动作自建工具栏（如 PostEditor）。
// 注意：Boolean prop 缺省会被 Vue 转为 false，必须用 withDefaults 默认 true 保证老调用方不受影响。
const stellarContext = inject(stellarEditorContextKey, undefined);
const props = withDefaults(
  defineProps<{
    modelValue: string;
    storageKey?: string;
    followStorageKey?: boolean;
    toolbar?: boolean;
    userCenter?: boolean;
  }>(),
  { storageKey: undefined, toolbar: true },
);
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
const uploadError = ref("");

// 本地草稿保护（与 RichTextEditor 共用同一 storageKey 草稿）
const { draftBanner, scheduleDraft, clearDraft, restoreDraft, discardDraft, checkDraft } = useLocalDraft({
  storageKey: props.followStorageKey ? () => props.storageKey : props.storageKey,
  getValue: () => props.modelValue,
  setValue: (v) => emit("update:modelValue", v),
});

// 完整预览文档由前台组装，opaque iframe通过token消息同步滚动。
const previewSrcdoc = ref("");
const previewToken = ref("");
const previewError = ref("");
// 仅预览模式重渲染后恢复滚动比例：iframe 是 opaque origin，父窗口读不到 contentDocument，
// 因此只记录子文档主动回传的最后一个比例，加载完成后按 token 发回。
let lastPreviewRatio = 0;
let pendingScrollRatio: number | null = null;
function onIframeLoad() {
  if (pendingScrollRatio !== null) {
    const ratio = pendingScrollRatio;
    pendingScrollRatio = null;
    if (mode.value === "preview" && previewToken.value) {
      iframeRef.value?.contentWindow?.postMessage(
        { type: "stellar-preview-scroll-to", token: previewToken.value, ratio },
        "*",
      );
      return;
    }
  }
  onEditorScroll();
}

// 深色-only：iframe 固定 data-theme=dark，无需观察切换
onMounted(() => {
  // 检测 localStorage 是否有未保存草稿（MarkdownEditor 无 round-trip，直接检测即可）
  checkDraft();
  // 默认分屏/预览模式时首次挂载即渲染
  if (mode.value !== "edit") schedulePreview(true);
});

// ---------- 实时预览（防抖 400ms，走后端 goldmark 管线，Stellar 标签插件真实呈现） ----------

let previewTimer: ReturnType<typeof setTimeout> | undefined;
// 卸载后 nextTick 里的补调度不得再发请求。
let previewActive = true;

// Spec 62：预览请求序号守卫——连续输入时先发出的慢响应不得覆盖新内容；
// 仅当请求发起时是最新一版（序号匹配）才应用渲染结果。
let previewSeq = 0;
const PREVIEW_DEBOUNCE_MS = 400;
// 大文档保护：正文超过阈值时提高防抖间隔，降低每停顿一轮的全量网络渲染频率
const PREVIEW_LARGE_DOC_MS = 800;
const PREVIEW_LARGE_DOC_BYTES = 50 * 1024;

async function refreshPreview() {
  const seq = ++previewSeq;
  // 文档身份：请求期间若 storageKey 已切换（A→B），A 的正文不得渲染到 B 的身份下。
  const identity = props.storageKey;
  const markdown = props.modelValue;
  rendering.value = true;
  previewError.value = "";
  try {
    const { data } = await previewApi.render(markdown, undefined, stellarContext?.value);
    if (seq !== previewSeq || identity !== props.storageKey) return; // 过期响应丢弃（输入或身份已变）
    // 仅预览模式重渲染：保留滚动比例，避免 iframe 回顶
    if (mode.value === "preview" && previewSrcdoc.value) pendingScrollRatio = lastPreviewRatio;
    previewToken.value = data.preview_token;
    // 真实站上下文只注入写守卫与不透明表面，不注入内存 Storage（rating/vote 保持不可交互）。
    previewSrcdoc.value = preparePreviewDocument(data.document, null);
  } catch {
    if (seq !== previewSeq || identity !== props.storageKey) return;
    // 失败不清空已有文档：有历史预览时保留并提示，首次失败不谎称“仍显示上一次结果”。
    previewError.value = previewFailureMessage(Boolean(previewSrcdoc.value));
  } finally {
    if (seq === previewSeq) rendering.value = false;
  }
}

function previewDebounceMs() {
  const bytes = new TextEncoder().encode(props.modelValue).length;
  return bytes > PREVIEW_LARGE_DOC_BYTES ? PREVIEW_LARGE_DOC_MS : PREVIEW_DEBOUNCE_MS;
}

function schedulePreview(immediate = false) {
  if (!previewActive || mode.value === "edit") return;
  clearTimeout(previewTimer);
  previewTimer = setTimeout(refreshPreview, immediate ? 0 : previewDebounceMs());
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

// 同步完成后用 setTimeout 短延时释放守卫标志（而非 requestAnimationFrame：
// rAF 在后台标签/非渲染场景会被节流甚至不触发，会把 syncingScroll 永久卡在 true）。
function releaseSyncGuard() {
  clearTimeout(syncReleaseTimer);
  syncReleaseTimer = setTimeout(() => (syncingScroll = false), 60);
}

function onEditorScroll() {
  if (mode.value !== "split" || syncingScroll) return;
  const ta = textareaRef.value;
  if (!ta || !previewToken.value) return;
  const range = ta.scrollHeight - ta.clientHeight;
  syncingScroll = true;
  iframeRef.value?.contentWindow?.postMessage(
    { type: "stellar-preview-scroll-to", token: previewToken.value, ratio: range > 0 ? ta.scrollTop / range : 0 },
    "*",
  );
  releaseSyncGuard();
}
function onPreviewMessage(event: MessageEvent) {
  if (
    event.source !== iframeRef.value?.contentWindow ||
    event.data?.token !== previewToken.value ||
    event.data?.type !== "stellar-preview-scroll"
  )
    return;
  const ratio = event.data.ratio;
  // 编辑区不在分屏时不回写 textarea，但仍记录预览滚动比例供下次重渲染恢复。
  if (typeof ratio === "number" && Number.isFinite(ratio)) lastPreviewRatio = Math.max(0, Math.min(1, ratio));
  if (mode.value !== "split" || syncingScroll) return;
  const ta = textareaRef.value;
  if (!ta || typeof ratio !== "number" || !Number.isFinite(ratio)) return;
  syncingScroll = true;
  ta.scrollTop = Math.max(0, Math.min(1, ratio)) * Math.max(0, ta.scrollHeight - ta.clientHeight);
  releaseSyncGuard();
}
onMounted(() => window.addEventListener("message", onPreviewMessage));
onBeforeUnmount(() => window.removeEventListener("message", onPreviewMessage));

watch([() => props.modelValue, () => stellarContext?.value], () => {
  schedulePreview();
});

/** 文档身份切换（A→B）时清空旧预览，避免 B 首次失败仍显示 A 的内容。 */
function clearPreviewIdentity() {
  previewSeq++;
  clearTimeout(previewTimer);
  previewSrcdoc.value = "";
  previewToken.value = "";
  previewError.value = "";
  rendering.value = false;
  lastPreviewRatio = 0;
  pendingScrollRatio = null;
  if (mode.value === "edit") return;
  // 身份切换后必须重建预览：
  // - 正文也换了时，这里先清空再立即渲染，B 首次失败不会残留 A；
  // - 正文未变时（如新建文章保存拿到 slug，storageKey 从 post:new 变成 post:<slug>）
  //   没有任何 modelValue watcher 会再触发，只清空会让预览长期空白；
  //   上面的 clearTimeout 也可能取消同 tick 内 context watcher 排好的调度。
  // 统一放到 nextTick：合并同 tick 的 modelValue/context watcher，只发一次请求。
  nextTick(() => schedulePreview(true));
}
watch(
  () => props.storageKey,
  (next, previous) => {
    if (next !== previous) clearPreviewIdentity();
  },
);

// ---------- 本地草稿保护（逻辑已抽到 useLocalDraft composable） ----------

onBeforeUnmount(() => {
  previewActive = false;
  clearTimeout(previewTimer);
  clearTimeout(syncReleaseTimer);
});

// ---------- 工具栏动作（基于选区包裹/插入，操作后恢复焦点与选区） ----------

function editValue(next: string) {
  if (next === props.modelValue) return;
  emit("update:modelValue", next);
  // API 水合和预览上下文更新不是编辑；未决旧草稿必须先恢复或丢弃，不能被新输入覆盖。
  if (!draftBanner.value) scheduleDraft();
}

function onInput(event: Event) {
  editValue((event.target as HTMLTextAreaElement).value);
}

function updateValue(next: string, selStart: number, selEnd: number) {
  editValue(next);
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
  const next = value.slice(0, start) + before + selected + after + value.slice(end);
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
  if (mode.value !== "preview") {
    const ta = textareaRef.value;
    if (!ta) return;
    let offset = 0;
    for (let i = 0; i < line && i < lines.length; i++) offset += lines[i].length + 1;
    ta.focus();
    ta.setSelectionRange(offset, offset);
    ta.scrollTop = ratio * (ta.scrollHeight - ta.clientHeight);
  } else {
    iframeRef.value?.contentWindow?.postMessage(
      { type: "stellar-preview-scroll-to", token: previewToken.value, ratio },
      "*",
    );
  }
}

function openImageUpload() {
  fileInputRef.value?.click();
}

function openImagePicker() {
  pickerOpen.value = true;
}

// 写作助手桥接：选区就是 textarea 的字符偏移；结果优先原生插入以保留撤销栈，
// 做不到时才经 updateValue 写回（走 v-model，同样会触发未保存标记与本地草稿）。
const assistantBridge = createMarkdownBridge({
  getTextarea: () => textareaRef.value,
  setValue: (next, selStart, selEnd) => updateValue(next, selStart, selEnd),
});

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
  getAssistantBridge: () => assistantBridge,
});

const tableTemplate = "\n| 表头 | 表头 |\n| --- | --- |\n| 内容 | 内容 |\n";

// 内置工具栏动作定义（命名避开 toolbar prop）
const toolbarActions = [
  { icon: "ri:bold", title: "加粗 (Cmd/Ctrl+B)", action: () => wrapSelection("**", "**", "加粗文本") },
  { icon: "ri:italic", title: "斜体 (Cmd/Ctrl+I)", action: () => wrapSelection("*", "*", "斜体文本") },
  { icon: "ri:strikethrough", title: "删除线", action: () => wrapSelection("~~", "~~", "删除线") },
  { icon: "ri:heading", title: "标题", action: () => prefixLines("##") },
  { icon: "ri:link", title: "链接 (Cmd/Ctrl+K)", action: () => wrapSelection("[", "](url)", "链接文字") },
  { icon: "ri:double-quotes-l", title: "引用", action: () => prefixLines(">") },
  { icon: "ri:code-line", title: "行内代码", action: () => wrapSelection("`", "`", "code") },
  { icon: "ri:code-box-line", title: "代码块", action: () => wrapSelection("\n```\n", "\n```\n", "code") },
  { icon: "ri:list-unordered", title: "无序列表", action: () => prefixLines("-") },
  { icon: "ri:list-ordered", title: "有序列表", action: () => prefixLines("", true) },
  { icon: "ri:table-2", title: "插入表格", action: () => insertBlock(tableTemplate) },
];

// ---------- 图片上传（工具栏按钮 / 粘贴 / 拖拽） ----------

const fileInputRef = ref<HTMLInputElement | null>(null);
let uploadActive = true;
const uploadControllers = new Set<AbortController>();
onBeforeUnmount(() => {
  uploadActive = false;
  for (const controller of uploadControllers) controller.abort();
  uploadControllers.clear();
});

async function uploadAndInsert(file: File) {
  if (!uploadActive) return;
  const controller = new AbortController();
  uploadControllers.add(controller);
  uploadError.value = "";
  const placeholder = `![上传中 ${file.name}...]()`;
  insertBlock(placeholder);
  try {
    const { data } = props.userCenter
      ? await ucApi.attachments.upload(file, controller.signal)
      : await attachmentApi.upload(file);
    if (!uploadActive || controller.signal.aborted) return;
    const md = `![${file.name}](${data.url})`;
    const replaced = props.modelValue.replace(placeholder, md);
    editValue(replaced);
  } catch {
    if (!uploadActive || controller.signal.aborted) return;
    editValue(props.modelValue.replace(placeholder, ""));
    uploadError.value = `图片 ${file.name} 上传失败`;
  } finally {
    uploadControllers.delete(controller);
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
  const files = Array.from(e.clipboardData?.files || []).filter((f) => f.type.startsWith("image/"));
  if (files.length === 0) return;
  e.preventDefault();
  files.forEach(uploadAndInsert);
}

function onDrop(e: DragEvent) {
  const files = Array.from(e.dataTransfer?.files || []).filter((f) => f.type.startsWith("image/"));
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
    insertBlock("");
  }
}

const modeButtons: { value: ViewMode; icon: string; title: string }[] = [
  { value: "edit", icon: "ri:edit-2-line", title: "仅编辑" },
  { value: "split", icon: "ri:layout-column-line", title: "即时渲染（分屏预览）" },
  { value: "preview", icon: "ri:eye-line", title: "仅预览" },
];
</script>

<template>
  <div class="border border-border rounded-xl overflow-hidden bg-card">
    <!-- 草稿恢复条幅 -->
    <div
      v-if="draftBanner"
      class="flex items-center justify-between gap-3 px-4 py-2 text-sm text-state-warning bg-state-warning/100/10 border-b border-state-warning/30"
    >
      <span class="flex items-center gap-1.5"> <Icon icon="ri:draft-line" />检测到本地未保存草稿，是否恢复？ </span>
      <span class="flex gap-2 shrink-0">
        <button
          @click="restoreDraft"
          class="px-2.5 py-1 text-xs font-medium rounded-md bg-state-warning text-foreground hover:bg-state-warning"
        >
          恢复
        </button>
        <button
          @click="discardDraft"
          class="px-2.5 py-1 text-xs rounded-md text-state-warning hover:bg-state-warning/15"
        >
          丢弃
        </button>
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
            mode === btn.value ? 'bg-card text-primary shadow-sm' : 'text-text-tertiary hover:text-foreground',
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
        @input="onInput"
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

      <div v-show="mode !== 'edit'" class="relative min-h-[560px]">
        <span v-if="rendering" class="absolute top-2 right-3 z-10 text-[11px] text-text-quaternary">渲染中...</span>
        <div
          v-if="previewError"
          class="absolute inset-x-3 top-2 z-10 flex items-center justify-between gap-2 rounded-md border border-state-error/30 bg-card/95 px-2.5 py-1.5 text-[11px] text-state-error"
        >
          <span>{{ previewError }}</span>
          <button type="button" class="shrink-0 text-primary" @click="schedulePreview(true)">重试</button>
        </div>
        <iframe
          ref="iframeRef"
          :srcdoc="previewSrcdoc"
          sandbox="allow-scripts"
          @load="onIframeLoad"
          class="w-full min-h-[560px] h-[75vh] border-0 block"
          title="预览"
        ></iframe>
      </div>
    </div>

    <AttachmentPickerModal
      :user-center="userCenter"
      :open="pickerOpen"
      @select="onPickerSelect"
      @close="pickerOpen = false"
    />
  </div>
</template>
