<script setup lang="ts">
import { provide, ref, onMounted, onBeforeUnmount, computed, watch, defineAsyncComponent, nextTick } from "vue";
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRoute, useRouter } from "vue-router";
import { useQueryClient } from "@tanstack/vue-query";
import { postApi, taxonomyApi, previewApi, editorImportApi, getApiErrorMessage } from "@/api";
import { queryKeys } from "@/api/query-client";
import { stellarEditorContextKey } from "@/components/editor/stellar-context";
import StellarContentConfig from "@/components/editor/StellarContentConfig.vue";
import MarkdownEditor from "@/components/MarkdownEditor.vue";
// Spec 62：tiptap 190KB gzip chunk 按需挂载——仅富文本模式启用时才下载
const RichTextEditor = defineAsyncComponent(() => import("@/components/RichTextEditor.vue"));
import AttachmentPickerModal from "@/components/AttachmentPickerModal.vue";
import EditorToolbar from "./EditorToolbar.vue";
import EditorSidebar from "./EditorSidebar.vue";
import AiGenerateModal from "./AiGenerateModal.vue";
import RevisionPanel from "./RevisionPanel.vue";
import { Icon } from "@iconify/vue";
import { Toast, Dialog, VButton } from "@/components/ui";

const queryClient = useQueryClient();

// 文章编辑器：Halo 式三栏布局——顶部全宽工具栏 + 左侧主编辑区 + 右侧大纲/详情面板。
// 双编辑器模式：默认富文本所见即得（TipTap），可切 Markdown 分屏兑底；
// 两者绑定同一份 form.content_md，切换即天然同步。
const route = useRoute();
const router = useRouter();

const persistedSlug = ref((route.params.slug as string) || "");
const isEdit = computed(() => !!persistedSlug.value);
const slug = computed(() => persistedSlug.value);
const draftStorageKey = ref("post:" + (persistedSlug.value || "new"));

const form = ref({
  stellar_version: 2,
  stellar_config: "{}",
  blog_extensions: "{}",
  slug: "",
  title: "",
  excerpt: "",
  cover: "",
  banner: "",
  category: "",
  categories: [] as string[],
  topic_id: null as number | null,
  topic: "",
  tags: "",
  content_md: "",
  status: "draft",
  pinned: false,
  password: "",
  publish_at: "",
  meta_description: "",
  meta_keywords: "",
  ai_label: "",
  authors: [] as string[],
  author: "",
});
provide(
  stellarEditorContextKey,
  computed(() => ({
    stellar_version: form.value.stellar_version,
    stellar_config: form.value.stellar_config,
    blog_extensions: form.value.blog_extensions,
    title: form.value.title,
    layout: "post",
    path: "/post/" + form.value.slug,
  })),
);

const emptyForm = JSON.stringify(form.value);
const loading = ref(false);
let editorSession = 0;
const editorInstanceKey = ref(0);
const saving = ref(false);
const publishing = ref(false);
const editorRef = ref<InstanceType<typeof MarkdownEditor> | null>(null);
// RTE 异步组件暴露方法接口（defineAsyncComponent 无法推导 InstanceType）
interface RichEditorExpose {
  activeStates: Record<string, boolean>;
  applyFormat: (key: string) => void;
  insertMarkdown: (text: string) => void;
  scrollToHeading: (index: number) => void;
  openImageUpload: () => void;
  openImagePicker: () => void;
  clearDraft: () => void;
  flushPendingChanges: () => Promise<void>;
}

interface CategoryNode {
  id: number;
  name: string;
  path: string;
  parent_id: number;
  count: number;
  children?: CategoryNode[];
}
const richRef = ref<RichEditorExpose | null>(null);
const coverPickerOpen = ref(false);
const revisionOpen = ref(false);
const aiOpen = ref(false);

// 编辑器种类：富文本（默认）/ Markdown 分屏，选择持久化
const KIND_KEY = "post-editor:kind";
type EditorKind = "rich" | "markdown";
function loadKind(): EditorKind {
  return localStorage.getItem(KIND_KEY) === "markdown" ? "markdown" : "rich";
}
const editorKind = ref<EditorKind>(loadKind());
const editorKindOpen = ref(false);
const editorKindHighlight = ref<EditorKind>(editorKind.value);
const editorKindTrigger = ref<HTMLButtonElement | null>(null);
const editorKindMenu = ref<HTMLDivElement | null>(null);
const editorKindItems: Array<{ value: EditorKind; label: string; icon: string }> = [
  { value: "rich", label: "默认编辑器", icon: "ri:edit-box-line" },
  { value: "markdown", label: "Markdown 编辑器", icon: "ri:markdown-line" },
];
const currentEditorKind = computed(
  () => editorKindItems.find((item) => item.value === editorKind.value) || editorKindItems[0],
);

function openEditorKindMenu() {
  editorKindHighlight.value = editorKind.value;
  editorKindOpen.value = true;
}

function toggleEditorKindMenu() {
  if (editorKindOpen.value) {
    editorKindOpen.value = false;
  } else {
    openEditorKindMenu();
  }
}

async function onEditorKindSelect(value: EditorKind) {
  await flushPendingChanges();
  preserveBodyDraft();
  editorKind.value = value;
  localStorage.setItem(KIND_KEY, value);
  editorKindOpen.value = false;
  nextTick(() => editorKindTrigger.value?.focus());
}

function onEditorKindKeydown(e: KeyboardEvent) {
  if (e.key === "Escape") {
    if (editorKindOpen.value) {
      e.preventDefault();
      editorKindOpen.value = false;
    }
    return;
  }

  if (!editorKindOpen.value) {
    if (["ArrowDown", "ArrowUp", "Enter", " "].includes(e.key)) {
      e.preventDefault();
      openEditorKindMenu();
    }
    return;
  }

  const currentIndex = editorKindItems.findIndex((item) => item.value === editorKindHighlight.value);
  if (e.key === "ArrowDown" || e.key === "ArrowRight") {
    e.preventDefault();
    editorKindHighlight.value = editorKindItems[Math.min(currentIndex + 1, editorKindItems.length - 1)].value;
  } else if (e.key === "ArrowUp" || e.key === "ArrowLeft") {
    e.preventDefault();
    editorKindHighlight.value = editorKindItems[Math.max(currentIndex - 1, 0)].value;
  } else if (e.key === "Home") {
    e.preventDefault();
    editorKindHighlight.value = editorKindItems[0].value;
  } else if (e.key === "End") {
    e.preventDefault();
    editorKindHighlight.value = editorKindItems[editorKindItems.length - 1].value;
  } else if (e.key === "Enter" || e.key === " ") {
    e.preventDefault();
    onEditorKindSelect(editorKindHighlight.value);
  }
}

function onEditorKindPointerDown(e: PointerEvent) {
  const target = e.target as Node;
  if (!editorKindTrigger.value?.contains(target) && !editorKindMenu.value?.contains(target)) {
    editorKindOpen.value = false;
  }
}

function onEditorKindDocumentKeydown(e: KeyboardEvent) {
  if (e.key === "Escape" && mobileSidebarOpen.value) {
    e.preventDefault();
    closeMobileSidebar();
  }
  if (e.key === "Escape" && editorKindOpen.value) {
    editorKindOpen.value = false;
    nextTick(() => editorKindTrigger.value?.focus());
  }
}

// 草稿头状态：originalStatus 为服务器线上状态，hasDraft 表示存在已保存未发布的草稿
const originalStatus = ref("draft");
const hasDraft = ref(false);
// 服务器保存基线覆盖完整表单，AI 摘要继续使用独立保存契约。
type PostForm = typeof form.value;
const savedSnapshot = ref(JSON.stringify(form.value));
const baselineReady = ref(!isEdit.value);
const richPending = ref(false);
const saveFailed = ref<false | "save" | "publish">(false);
const savedAt = ref("");
const busy = computed(() => saving.value || publishing.value);
const isDirty = computed(
  () => baselineReady.value && (richPending.value || JSON.stringify(form.value) !== savedSnapshot.value),
);
const saveStatus = computed(() => {
  if (loading.value) return "正在加载文章…";
  if (!baselineReady.value) return "文章加载失败，请刷新重试";
  if (publishing.value) return "正在发布…";
  if (saving.value) return "正在保存…";
  if (saveFailed.value) return saveFailed.value === "publish" ? "发布失败，修改尚未保存" : "保存失败，修改尚未保存";
  if (isDirty.value) return "有未保存的修改";
  if (savedAt.value) return `已保存 ${savedAt.value}${hasDraft.value ? " · 尚未发布" : ""}`;
  return isEdit.value ? (hasDraft.value ? "草稿已保存 · 尚未发布" : "所有修改已保存") : "新文章尚未保存";
});
function snapshotForm(): PostForm {
  return JSON.parse(JSON.stringify(form.value));
}
async function flushPendingChanges() {
  await richRef.value?.flushPendingChanges();
  await nextTick();
}
// 离开/刷新时同步落正文备份，不依赖编辑器的 2s 防抖。
function preserveBodyDraft() {
  if (!isDirty.value) return;
  try {
    localStorage.setItem(`draft:${draftStorageKey.value}`, form.value.content_md);
  } catch {
    // 存储不可用时仍保留离开确认，不能把本地备份失败当作服务器保存失败。
  }
}
function handleBeforeUnload(event: BeforeUnloadEvent) {
  richRef.value?.flushPendingChanges();
  if (!isDirty.value && !busy.value) return;
  preserveBodyDraft();
  event.preventDefault();
  event.returnValue = "";
}
let leaveConfirmation: Promise<boolean> | null = null;
async function confirmLeave() {
  await flushPendingChanges();
  if (!isDirty.value && !busy.value) return true;
  preserveBodyDraft();
  if (leaveConfirmation) return leaveConfirmation;
  leaveConfirmation = new Promise<boolean>((resolve) => {
    Dialog.warning({
      title: busy.value ? "操作尚未完成，确定离开？" : "有未保存的修改，确定离开？",
      description: "离开后未保存的标题和设置会丢失。请先保存，或继续编辑。",
      confirmText: "离开页面",
      cancelText: "继续编辑",
      confirmType: "danger",
      onConfirm: () => {
        resolve(true);
      },
      onCancel: () => {
        resolve(false);
      },
    });
  });
  try {
    return await leaveConfirmation;
  } finally {
    leaveConfirmation = null;
  }
}
onBeforeRouteLeave(confirmLeave);
onBeforeRouteUpdate(async (to, from) => {
  if (to.params.slug === from.params.slug || to.params.slug === persistedSlug.value) return true;
  return confirmLeave();
});
// AI 文章摘要（Spec 53）：独立于主表单保存链路，EditorSidebar 面板与 AiGenerateModal 共用此状态
const aiSummary = ref("");
const isPublishedPost = computed(() => isEdit.value && originalStatus.value === "published");

// 编辑器模式（页头下拉与工具栏三态按钮双向同步）
type EditorMode = "edit" | "split" | "preview";
const editorMode = ref<EditorMode>("split");
const editorModeItems: { value: EditorMode; label: string }[] = [
  { value: "edit", label: "仅编辑" },
  { value: "split", label: "即时渲染" },
  { value: "preview", label: "仅预览" },
];
function onEditorModeSelect(e: Event) {
  const v = (e.target as HTMLSelectElement).value as EditorMode;
  setEditorMode(v);
}
async function setEditorMode(m: EditorMode) {
  await flushPendingChanges();
  editorMode.value = m;
  editorRef.value?.setMode(m);
}

// 工具栏格式按钮激活态（仅富文本模式有；Markdown 模式不高亮）
const toolbarActive = computed<Record<string, boolean>>(() =>
  editorKind.value === "rich" ? (richRef.value?.activeStates ?? {}) : {},
);

// 发布前预览：当前表单 POST 拿临时 token → 新标签页打开前台草稿渲染
const previewing = ref(false);
async function handlePreview() {
  if (previewing.value) return;
  previewing.value = true;
  try {
    await flushPendingChanges();
    const { data } = await previewApi.createPostPreview(snapshotForm());
    window.open(`/preview/draft/${data.token}`, "_blank");
  } catch {
  } finally {
    previewing.value = false;
  }
}

// 全屏模式：覆盖侧边栏，编辑器占满视口
const fullscreen = ref(false);
function toggleFullscreen() {
  fullscreen.value = !fullscreen.value;
}

// 右侧面板开关（批注：设置参数合并成大组件、可从右侧收起），状态持久化
const SIDEBAR_KEY = "post-editor:sidebar-open";
const sidebarOpen = ref(localStorage.getItem(SIDEBAR_KEY) !== "0");
const mobileSidebarOpen = ref(false);
const isMobile = ref(window.matchMedia("(max-width: 639px)").matches);
const mobileSidebarRef = ref<HTMLElement | null>(null);
const mobileSidebarTrigger = ref<HTMLButtonElement | null>(null);
const mobileSidebarClose = ref<HTMLButtonElement | null>(null);
const sidebarVisible = computed(() => (isMobile.value ? mobileSidebarOpen.value : sidebarOpen.value));
function syncMobileViewport() {
  isMobile.value = window.matchMedia("(max-width: 639px)").matches;
  if (!isMobile.value) mobileSidebarOpen.value = false;
}
function toggleSidebar() {
  if (isMobile.value) {
    mobileSidebarOpen.value = !mobileSidebarOpen.value;
    if (mobileSidebarOpen.value) nextTick(() => mobileSidebarClose.value?.focus());
  } else {
    sidebarOpen.value = !sidebarOpen.value;
    localStorage.setItem(SIDEBAR_KEY, sidebarOpen.value ? "1" : "0");
  }
}
function closeMobileSidebar() {
  mobileSidebarOpen.value = false;
  nextTick(() => mobileSidebarTrigger.value?.focus());
}
function onMobileSidebarKeydown(event: KeyboardEvent) {
  if (event.key !== "Tab") return;
  const elements = mobileSidebarRef.value?.querySelectorAll<HTMLElement>(
    'button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex="0"]',
  );
  if (!elements?.length) return;
  const first = elements[0];
  const last = elements[elements.length - 1];
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault();
    last.focus();
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault();
    first.focus();
  }
}

// Slug 自动生成：新建文章时标题变化自动推导 slug，用户手动编辑后停止覆盖（Spec Task 6）
const slugManuallyEdited = ref(false);

/** 从标题推导 URL 友好的 slug（与后端 generateSlug 对齐：保留 Unicode 字母/数字） */
function autoSlug(title: string): string {
  let s = (title || "").trim().toLowerCase();
  s = s.replace(/\s+/g, "-");
  s = s.replace(/[^\p{L}\p{N}-]/gu, "");
  s = s.replace(/-{2,}/g, "-");
  s = s.replace(/^-+|-+$/g, "");
  return s;
}

// 标题变化时自动填充 slug（仅新建模式 + 用户未手动编辑）
watch(
  () => form.value.title,
  (newTitle) => {
    if (!isEdit.value && !slugManuallyEdited.value) {
      form.value.slug = autoSlug(newTitle);
    }
  },
);

// 当前正文字数（非空白 rune），供版本面板计算字数变化。
const currentWordCount = computed(() => {
  const s = form.value.content_md || "";
  let n = 0;
  for (const ch of s) {
    if (!/\s/.test(ch)) n++;
  }
  return n;
});

// ---------- 分类/标签可选项（Spec 29 P0-4） ----------
const categories = ref<string[]>([]);
const categoryTree = ref<CategoryNode[]>([]);
const tags = ref<string[]>([]);

async function loadTaxonomy() {
  try {
    const [cRes, tRes] = await Promise.all([taxonomyApi.categories(), taxonomyApi.tags()]);
    categories.value = (cRes.data.items || []).map((i: any) => i.path || i.name);
    categoryTree.value = Array.isArray(cRes.data.tree) ? cRes.data.tree : [];
    tags.value = (tRes.data.items || []).map((i: any) => i.name);
  } catch {
    /* 静默：可选项缺失不阻塞编辑 */
  }
}

// ---------- 工具栏 → 编辑器动作路由（按当前编辑器种类分发） ----------

// Markdown 模式的 format(id) 映射表（与 EditorToolbar formatTools id 一致）
const tableTemplate = "\n| 表头 | 表头 |\n| --- | --- |\n| 内容 | 内容 |\n";
const mdFormatMap: Record<string, () => void> = {
  bold: () => editorRef.value?.wrapSelection("**", "**", "加粗文本"),
  italic: () => editorRef.value?.wrapSelection("*", "*", "斜体文本"),
  strike: () => editorRef.value?.wrapSelection("~~", "~~", "删除线"),
  heading: () => editorRef.value?.prefixLines("##", false),
  link: () => editorRef.value?.wrapSelection("[", "](url)", "链接文字"),
  quote: () => editorRef.value?.prefixLines(">", false),
  code: () => editorRef.value?.wrapSelection("`", "`", "code"),
  codeblock: () => editorRef.value?.wrapSelection("\n```\n", "\n```\n", "code"),
  ul: () => editorRef.value?.prefixLines("-", false),
  ol: () => editorRef.value?.prefixLines("", true),
  table: () => editorRef.value?.insertBlock(tableTemplate),
};

function onFormat(id: string) {
  if (editorKind.value === "rich") {
    richRef.value?.applyFormat(id);
  } else {
    mdFormatMap[id]?.();
  }
}

function onInsert(text: string) {
  if (editorKind.value === "rich") {
    richRef.value?.insertMarkdown(text);
  } else {
    editorRef.value?.insertBlock(text);
  }
}

function onUpload() {
  (editorKind.value === "rich" ? richRef.value : editorRef.value)?.openImageUpload();
}

function onPick() {
  (editorKind.value === "rich" ? richRef.value : editorRef.value)?.openImagePicker();
}

const importing = ref(false);
function applyImportedPost(data: Awaited<ReturnType<typeof editorImportApi.stellar>>["data"]) {
  form.value.stellar_version = 2;
  form.value.stellar_config = data.front_matter;
  form.value.blog_extensions = data.blog_extensions;
  form.value.content_md = data.content_md;
  // 这里只读取表单支持的字符串，完整配置始终使用原文，绝不 stringify。
  const front = JSON.parse(data.front_matter);
  if (typeof front.title === "string") form.value.title = front.title;
  clearActiveDraft();
  Toast.success("导入已回填，请检查后保存；尚未发布");
}

async function onImport(file: File) {
  if (importing.value) return;
  if (!/\.(md|markdown)$/i.test(file.name)) {
    Toast.error("请选择 .md 或 .markdown 文件");
    return;
  }
  importing.value = true;
  try {
    const { data } = await editorImportApi.stellar({ filename: file.name, content: await file.text(), kind: "post" });
    Dialog.warning({
      title: "审查 Stellar 2 导入预览",
      description: `将替换正文与完整原文配置，不会自动保存或发布。\nStellar：\n${data.front_matter}\nBlog 扩展：\n${data.blog_extensions}\n正文：\n${data.content_md}`,
      confirmText: "确认回填",
      confirmType: "primary",
      onConfirm: () => applyImportedPost(data),
    });
  } catch (error) {
    Toast.error(`Markdown 导入失败：${getApiErrorMessage(error)}`);
  } finally {
    importing.value = false;
  }
}

/** 清除当前编辑器的本地草稿（同一 storageKey，两实例任一清除即可） */
function clearActiveDraft() {
  (editorKind.value === "rich" ? richRef.value : editorRef.value)?.clearDraft();
}

// 大纲点击跳转：富文本按标题序号定位节点，Markdown 按行号滚动
function onJump(line: number, index: number) {
  if (editorKind.value === "rich") {
    richRef.value?.scrollToHeading(index);
  } else {
    editorRef.value?.scrollToLine(line);
  }
}

// AI 属性生成结果回填（tags 为逗号串；summary 走独立端点直接落库）
async function applyAi(values: Record<string, string>) {
  if (values.title !== undefined) form.value.title = values.title;
  if (values.slug !== undefined) {
    form.value.slug = values.slug;
    slugManuallyEdited.value = true;
  }
  if (values.category !== undefined) {
    form.value.category = values.category;
    form.value.categories = values.category ? [values.category] : [];
  }
  if (values.tags !== undefined) form.value.tags = values.tags;
  if (values.excerpt !== undefined) form.value.excerpt = values.excerpt;
  if (values.summary !== undefined) {
    aiSummary.value = values.summary;
    if (slug.value) {
      try {
        await postApi.updateAISummary(slug.value, values.summary);
      } catch {
        Toast.warning("AI 摘要已回填但落库失败，请在侧栏重试");
      }
    }
  }
  Toast.success("已应用 AI 生成结果，记得保存");
}

// ---------- 加载 / 保存 / 发布 ----------
async function loadPost() {
  const session = editorSession;
  const requestedSlug = slug.value;
  const { data } = await postApi.get(requestedSlug);
  if (session !== editorSession || requestedSlug !== slug.value) return false;
  originalStatus.value = data.status;
  hasDraft.value = !!data.has_draft;
  // 有草稿头时优先载入草稿内容（线上内容不受影响）
  const src = data.has_draft && data.draft ? { ...data, ...data.draft } : data;
  form.value = {
    stellar_version: src.stellar_version ?? 0,
    stellar_config: src.stellar_config || "{}",
    blog_extensions: src.blog_extensions || "{}",
    slug: src.slug || data.slug,
    title: src.title,
    excerpt: src.excerpt,
    cover: src.cover,
    banner: src.banner || "",
    category: src.category,
    categories: Array.isArray(src.categories)
      ? src.categories.map((c: unknown) => String(c).trim()).filter(Boolean)
      : src.category
        ? [src.category]
        : [],
    topic_id: src.topic_id ? Number(src.topic_id) : null,
    topic: src.topic,
    tags: src.tags,
    content_md: src.content_md,
    status: data.status,
    pinned: src.pinned,
    password: src.password,
    publish_at: src.publish_at || "",
    meta_description: src.meta_description || "",
    meta_keywords: src.meta_keywords || "",
    ai_label: src.ai_label || "",
    authors: Array.isArray(src.authors)
      ? src.authors.map((id: unknown) => String(id).trim()).filter(Boolean)
      : src.author
        ? [String(src.author)]
        : [],
    author: src.author || (Array.isArray(src.authors) && src.authors[0] ? src.authors[0] : ""),
  };
  savedSnapshot.value = JSON.stringify(form.value);
  baselineReady.value = true;
  saveFailed.value = false;
  savedAt.value = "";
  aiSummary.value = data.ai_summary || ""; // Spec 53：AI 摘要随文章详情载入
  return true;
}

// 同组件路由切换时重新加载目标文章；创建后的地址同步保留当前编辑实例。
watch(
  () => route.params.slug,
  async (value) => {
    const target = typeof value === "string" ? value : "";
    if (target === persistedSlug.value) return;
    editorSession++;
    editorInstanceKey.value++;
    const session = editorSession;
    persistedSlug.value = target;
    draftStorageKey.value = "post:" + (target || "new");
    form.value = JSON.parse(emptyForm);
    slugManuallyEdited.value = false;
    richPending.value = false;
    originalStatus.value = "draft";
    hasDraft.value = false;
    savedAt.value = "";
    saveFailed.value = false;
    aiSummary.value = "";
    savedSnapshot.value = JSON.stringify(form.value);
    baselineReady.value = !target;
    loading.value = false;
    if (!target) return;
    loading.value = true;
    try {
      await loadPost();
    } catch {
    } finally {
      if (session === editorSession) loading.value = false;
    }
  },
);

onMounted(async () => {
  window.addEventListener("beforeunload", handleBeforeUnload);
  window.addEventListener("resize", syncMobileViewport);
  document.addEventListener("pointerdown", onEditorKindPointerDown);
  document.addEventListener("keydown", onEditorKindDocumentKeydown);
  loadTaxonomy();
  editorMode.value = (editorRef.value?.getMode?.() as EditorMode) || "split";
  if (!isEdit.value) return;
  const session = editorSession;
  loading.value = true;
  try {
    await loadPost();
  } catch {
  } finally {
    if (session === editorSession) loading.value = false;
  }
});

onBeforeUnmount(() => {
  editorSession++;
  window.removeEventListener("beforeunload", handleBeforeUnload);
  window.removeEventListener("resize", syncMobileViewport);
  document.removeEventListener("pointerdown", onEditorKindPointerDown);
  document.removeEventListener("keydown", onEditorKindDocumentKeydown);
});

// 回滚成功后重新拉取文章内容到编辑器。
async function handleRestored() {
  try {
    if (await loadPost()) clearActiveDraft();
  } catch {}
}

/** 只把本次提交的快照记为已保存；请求期间的新输入继续保持未保存。 */
async function acceptSaved(submitted: PostForm, status: string, session: number) {
  await flushPendingChanges();
  if (session !== editorSession) return;
  if (form.value.status === submitted.status) form.value.status = status;
  savedSnapshot.value = JSON.stringify({ ...submitted, status });
  savedAt.value = new Date().toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
  saveFailed.value = false;
  if (!isDirty.value) {
    try {
      clearActiveDraft();
    } catch {
      // 本地存储不可用不能把服务器已经成功的保存误报为失败。
    }
  } else preserveBodyDraft();
}

/** 保存：已发布文章存草稿头，线上内容保持不变。 */
function adoptCreatedSlug(createdSlug: string, submitted: PostForm) {
  const previousKey = draftStorageKey.value;
  persistedSlug.value = createdSlug;
  if (form.value.slug === submitted.slug) form.value.slug = createdSlug;
  submitted.slug = createdSlug;
  draftStorageKey.value = `post:${createdSlug}`;
  try {
    localStorage.removeItem(`draft:${previousKey}`);
  } catch {}
}

function saveSessionIsCurrent(session: number) {
  // 用户已确认离开时，旧响应不得覆盖新页面输入、清新备份或把用户带回编辑页。
  if (session === editorSession) return true;
  queryClient.invalidateQueries({ queryKey: queryKeys.posts() });
  return false;
}

async function handleSave() {
  if (busy.value || !baselineReady.value) return;
  const session = editorSession;
  saving.value = true;
  try {
    await flushPendingChanges();
    if (!saveSessionIsCurrent(session)) return;
    const submitted = snapshotForm();
    if (isPublishedPost.value) {
      await postApi.saveDraft(slug.value, { ...submitted, status: "published" });
      if (!saveSessionIsCurrent(session)) return;
      hasDraft.value = true;
      await acceptSaved(submitted, "published", session);
      Toast.success("内容已保存，但还未发布");
    } else {
      if (isEdit.value) {
        await postApi.update(slug.value, { ...submitted, status: "draft" });
        if (!saveSessionIsCurrent(session)) return;
      } else {
        const { data } = await postApi.create({ ...submitted, status: "draft" });
        if (!saveSessionIsCurrent(session)) return;
        await flushPendingChanges();
        if (!saveSessionIsCurrent(session)) return;
        adoptCreatedSlug(data.slug, submitted);
        originalStatus.value = "draft";
      }
      await acceptSaved(submitted, "draft", session);
      Toast.success("保存成功");
    }
    queryClient.invalidateQueries({ queryKey: queryKeys.posts() });
    // 创建期间若继续写作，留在原编辑实例；下一次保存后再同步地址。
    if (!isDirty.value && !route.params.slug) {
      saving.value = false;
      await router.replace(`/posts/edit/${slug.value}`);
    }
  } catch {
    if (!saveSessionIsCurrent(session)) return;
    saveFailed.value = "save";
    preserveBodyDraft();
  } finally {
    saving.value = false;
  }
}

/** 发布本次快照；请求期间继续编辑时保留编辑页和新修改。 */
async function handlePublish() {
  if (busy.value || !baselineReady.value) return;
  const session = editorSession;
  publishing.value = true;
  try {
    await flushPendingChanges();
    if (!saveSessionIsCurrent(session)) return;
    const submitted = snapshotForm();
    if (isEdit.value) {
      await postApi.publish(slug.value, submitted);
      if (!saveSessionIsCurrent(session)) return;
    } else {
      const { data } = await postApi.create({ ...submitted, status: "published" });
      if (!saveSessionIsCurrent(session)) return;
      await flushPendingChanges();
      if (!saveSessionIsCurrent(session)) return;
      adoptCreatedSlug(data.slug, submitted);
    }
    originalStatus.value = "published";
    hasDraft.value = false;
    await acceptSaved(submitted, "published", session);
    Toast.success(isDirty.value ? "本次内容已发布，后续修改尚未保存" : "发布成功");
    queryClient.invalidateQueries({ queryKey: queryKeys.posts() });
    if (!isDirty.value) {
      publishing.value = false;
      await router.push("/posts");
    }
  } catch {
    if (!saveSessionIsCurrent(session)) return;
    saveFailed.value = "publish";
    preserveBodyDraft();
  } finally {
    publishing.value = false;
  }
}

/** 放弃草稿头，回到线上内容 */
function handleDiscardDraft() {
  if (busy.value) return;
  Dialog.warning({
    title: "确定要放弃未发布的草稿吗？",
    description: "将丢弃已保存的草稿内容，恢复为线上发布版本",
    confirmType: "danger",
    onConfirm: async () => {
      if (busy.value) return;
      saving.value = true;
      const session = editorSession;
      try {
        await postApi.discardDraft(slug.value);
        if (!saveSessionIsCurrent(session)) return;
        hasDraft.value = false;
        if (await loadPost()) {
          clearActiveDraft();
          Toast.success("已放弃草稿");
        }
      } catch {
      } finally {
        saving.value = false;
      }
    },
  });
}
</script>

<template>
  <div :class="{ 'fixed inset-0 z-[9999] overflow-y-auto bg-background': fullscreen }">
    <!-- 紧凑 sticky 毛玻璃页头 -->
    <header
      class="editor-header sticky top-0 z-50 flex items-center gap-2 px-4 py-2.5 sm:gap-4 sm:px-7 sm:py-3"
      style="
        background: color-mix(in srgb, #000 75%, transparent);
        backdrop-filter: saturate(180%) blur(20px);
        -webkit-backdrop-filter: saturate(180%) blur(20px);
        border-bottom: 1px solid var(--color-border);
      "
    >
      <button
        type="button"
        title="返回"
        aria-label="返回文章列表"
        class="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg text-text-secondary transition-colors hover:bg-secondary hover:text-foreground"
        @click="router.push('/posts')"
      >
        <Icon icon="ri:arrow-left-s-line" class="text-2xl" />
      </button>
      <h1 class="shrink-0 whitespace-nowrap text-base font-semibold tracking-tight text-foreground sm:text-[17px]">
        {{ isEdit ? "编辑文章" : "新建文章" }}
      </h1>
      <div class="flex-1"></div>
      <div class="flex shrink-0 items-center gap-1.5 sm:gap-2">
        <!-- Markdown 模式下的视图三态切换（与工具栏同步，桌面端显示） -->
        <div
          v-if="editorKind === 'markdown'"
          class="editor-select-control relative hidden min-w-[132px] items-center gap-1.5 rounded-lg border border-border bg-card px-2.5 py-1.5 text-sm text-text-secondary sm:flex"
        >
          <select
            :value="editorMode"
            @change="onEditorModeSelect"
            class="editor-select cursor-pointer select-none whitespace-nowrap border-0 bg-transparent pr-4 outline-none"
            title="切换视图模式"
          >
            <option v-for="m in editorModeItems" :key="m.value" :value="m.value">{{ m.label }}</option>
          </select>
          <Icon
            icon="ri:arrow-down-s-line"
            class="pointer-events-none absolute right-1.5 text-base text-text-quaternary"
          />
        </div>
        <!-- 预览（移动端仅图标） -->
        <VButton
          type="ghost"
          size="md"
          :disabled="previewing || !baselineReady"
          aria-label="预览文章"
          @click="handlePreview"
        >
          <template #icon>
            <Icon icon="ri:eye-line" class="text-base sm:text-[15px]" />
          </template>
          <span class="hidden sm:inline">预览</span>
        </VButton>
        <!-- 保存草稿（移动端仅图标） -->
        <VButton
          type="secondary"
          size="md"
          :disabled="busy || !baselineReady"
          :loading="saving"
          @click="handleSave"
          aria-label="保存草稿"
          :title="saving ? '保存中…' : '保存草稿'"
        >
          <template #icon>
            <Icon icon="ri:save-line" class="text-base sm:text-[15px]" />
          </template>
          <span class="hidden sm:inline">{{ saving ? "保存中…" : "保存草稿" }}</span>
        </VButton>
        <!-- 发布（primary，移动端保持文字但更紧凑） -->
        <VButton
          type="primary"
          size="md"
          :disabled="busy || !baselineReady"
          :loading="publishing"
          @click="handlePublish"
        >
          <template #icon>
            <Icon icon="ri:send-plane-fill" class="text-sm sm:text-[15px]" />
          </template>
          <span>{{ publishing ? "发布中…" : "发布" }}</span>
        </VButton>
      </div>
    </header>

    <div
      class="editor-status-bar flex flex-wrap items-center justify-between gap-2 border-b border-border px-4 py-2 sm:px-7"
    >
      <p
        role="status"
        aria-live="polite"
        class="text-xs text-text-secondary"
        :class="{ 'text-state-warning': isDirty || saveFailed }"
      >
        {{ saveStatus }}
      </p>
      <div class="flex items-center gap-2">
        <!-- 编辑器选择（富文本/Markdown） -->
        <div class="editor-kind-picker relative">
          <button
            ref="editorKindTrigger"
            type="button"
            class="editor-kind-trigger"
            :class="{ 'is-open': editorKindOpen }"
            title="切换编辑器"
            aria-label="切换编辑器"
            aria-haspopup="listbox"
            :aria-expanded="editorKindOpen"
            :aria-controls="editorKindOpen ? 'post-editor-kind-menu' : undefined"
            :aria-activedescendant="editorKindOpen ? `post-editor-kind-${editorKindHighlight}` : undefined"
            @click="toggleEditorKindMenu"
            @keydown="onEditorKindKeydown"
          >
            <span class="editor-kind-trigger-icon" :class="`editor-kind-icon--${editorKind}`">
              <Icon :icon="currentEditorKind.icon" />
            </span>
            <span class="editor-kind-trigger-label">{{ currentEditorKind.label }}</span>
            <Icon icon="ri:arrow-down-s-line" class="editor-kind-chevron" aria-hidden="true" />
          </button>

          <Transition name="editor-kind-menu">
            <div
              v-if="editorKindOpen"
              id="post-editor-kind-menu"
              ref="editorKindMenu"
              class="editor-kind-menu"
              role="listbox"
              aria-label="编辑器类型"
            >
              <button
                v-for="item in editorKindItems"
                :id="`post-editor-kind-${item.value}`"
                :key="item.value"
                type="button"
                role="option"
                class="editor-kind-option"
                :class="{
                  'is-selected': editorKind === item.value,
                  'is-highlighted': editorKindHighlight === item.value,
                }"
                :aria-selected="editorKind === item.value"
                @mouseenter="editorKindHighlight = item.value"
                @focus="editorKindHighlight = item.value"
                @click="onEditorKindSelect(item.value)"
              >
                <span class="editor-kind-option-icon" :class="`editor-kind-icon--${item.value}`">
                  <Icon :icon="item.icon" />
                </span>
                <span class="editor-kind-option-label">{{ item.label }}</span>
                <Icon
                  v-if="editorKind === item.value"
                  icon="ri:check-line"
                  class="editor-kind-option-check"
                  aria-hidden="true"
                />
              </button>
            </div>
          </Transition>
        </div>
        <button
          ref="mobileSidebarTrigger"
          type="button"
          class="flex min-h-11 items-center gap-1.5 rounded-lg px-3 text-sm text-text-secondary hover:bg-secondary sm:hidden"
          :aria-expanded="mobileSidebarOpen"
          aria-controls="post-settings-drawer"
          @click="toggleSidebar"
        >
          <Icon icon="ri:settings-3-line" />文章设置
        </button>
      </div>
    </div>

    <!-- 草稿头条幅：令牌化绿色提示条 -->
    <div
      v-if="hasDraft && isPublishedPost"
      class="mx-4 mt-3 flex items-center justify-between gap-3 rounded-lg px-3 py-2 text-xs sm:mx-7 sm:mt-4 sm:px-4 sm:text-sm"
      style="
        background: color-mix(in srgb, var(--color-state-success) 12%, transparent);
        color: var(--color-state-success);
        border: 1px solid color-mix(in srgb, var(--color-state-success) 20%, transparent);
      "
    >
      <span class="flex items-start gap-2 leading-snug">
        <span
          class="mt-1 inline-block h-1.5 w-1.5 shrink-0 rounded-full"
          style="background: var(--color-state-success)"
        ></span>
        <span>当前有内容已保存，但还未发布。正在编辑的是草稿版本，线上内容不受影响。</span>
      </span>
      <button
        class="shrink-0 text-xs underline opacity-80 transition-opacity hover:opacity-100"
        :disabled="busy"
        @click="handleDiscardDraft"
      >
        放弃草稿
      </button>
    </div>

    <!-- 编辑区：<640 单列+底部抽屉；640–1024 单列、设置面板下置；≥1024 左右两栏 -->
    <div class="editor-wrap flex flex-col gap-4 px-4 py-4 sm:gap-6 sm:px-7 sm:py-6 lg:flex-row">
      <!-- 左侧自适应编辑区 -->
      <div class="flex min-w-0 flex-1 flex-col">
        <!-- 标题区 -->
        <div
          class="rounded-t-xl border border-b-0 border-border bg-card px-5 pb-4 pt-5 sm:rounded-t-[14px] sm:px-9 sm:pb-5 sm:pt-7"
        >
          <label for="post-title" class="sr-only">文章标题</label>
          <input
            id="post-title"
            v-model="form.title"
            :disabled="!baselineReady"
            placeholder="文章标题"
            class="w-full border-0 bg-transparent text-2xl font-bold leading-tight tracking-tight text-foreground outline-none placeholder:text-text-quaternary placeholder:font-semibold sm:text-[28px]"
          />
        </div>

        <!-- 工具栏（独立卡片条） -->
        <div class="border border-b-0 border-border bg-card">
          <EditorToolbar
            :kind="editorKind"
            :mode="editorMode"
            :sidebar-open="sidebarVisible"
            :is-edit="isEdit"
            :fullscreen="fullscreen"
            :active-states="toolbarActive"
            @format="onFormat"
            @insert="onInsert"
            @import="onImport"
            @upload="onUpload"
            @pick="onPick"
            @update:mode="setEditorMode"
            @toggle-sidebar="toggleSidebar"
            @open-ai="aiOpen = true"
            @preview="handlePreview"
            @toggle-fullscreen="toggleFullscreen"
            @open-revision="revisionOpen = true"
          />
        </div>

        <!-- 编辑器内核 -->
        <div class="rounded-b-xl border border-border bg-card sm:rounded-b-[14px]">
          <StellarContentConfig
            v-model:config="form.stellar_config"
            v-model:blog="form.blog_extensions"
            :version="form.stellar_version"
            profile="post"
            :content="form.content_md"
            class="mb-3"
          />
          <RichTextEditor
            v-if="editorKind === 'rich'"
            :key="'rich:' + editorInstanceKey"
            ref="richRef"
            v-model="form.content_md"
            :storage-key="draftStorageKey"
            :follow-storage-key="true"
            class="!border-0"
            @save="handleSave"
            @pending-change="richPending = $event"
          />
          <MarkdownEditor
            v-else
            :key="'markdown:' + editorInstanceKey"
            ref="editorRef"
            v-model="form.content_md"
            :toolbar="false"
            :storage-key="draftStorageKey"
            :follow-storage-key="true"
            class="!border-0"
            @save="handleSave"
            @mode-change="(m: any) => (editorMode = m)"
          />
        </div>
      </div>

      <!-- 右侧设置面板：≥1024 固定 320px 右栏；640–1024 全宽下置于编辑区 -->
      <div
        v-show="sidebarOpen"
        class="sidebar-desktop hidden w-full shrink-0 sm:block lg:sticky lg:top-[72px] lg:w-80"
        style="max-height: calc(100vh - 88px); overflow-y: auto"
      >
        <EditorSidebar
          :form="form"
          :categories="categories"
          :category-tree="categoryTree"
          :tags="tags"
          :ai-summary="aiSummary"
          @jump="onJump"
          @insert="onInsert"
          @open-cover-picker="coverPickerOpen = true"
          @slug-edited="slugManuallyEdited = true"
          @update:ai-summary="aiSummary = $event"
        />
      </div>
    </div>

    <!-- 移动端：底部抽屉式侧边栏 -->
    <Transition name="drawer">
      <div
        v-if="mobileSidebarOpen"
        id="post-settings-drawer"
        ref="mobileSidebarRef"
        role="dialog"
        aria-modal="true"
        aria-labelledby="post-settings-title"
        @keydown="onMobileSidebarKeydown"
        class="sidebar-mobile fixed inset-x-0 z-[80] sm:hidden"
        style="
          bottom: calc(64px + env(safe-area-inset-bottom));
          max-height: calc(85dvh - 64px - env(safe-area-inset-bottom));
          background: var(--color-card);
          border-top: 1px solid var(--color-border);
          border-top-left-radius: var(--radius-lg);
          border-top-right-radius: var(--radius-lg);
          box-shadow: var(--shadow-xl);
        "
      >
        <div class="flex items-center justify-between border-b border-border px-4 py-2">
          <h2 id="post-settings-title" class="text-sm font-semibold text-foreground">文章设置</h2>
          <button
            ref="mobileSidebarClose"
            type="button"
            aria-label="关闭文章设置"
            class="flex h-11 w-11 items-center justify-center rounded-lg text-text-secondary hover:bg-secondary"
            @click="closeMobileSidebar"
          >
            <Icon icon="ri:close-line" class="text-xl" />
          </button>
        </div>
        <div class="overflow-y-auto" style="max-height: calc(85dvh - 128px - env(safe-area-inset-bottom))">
          <EditorSidebar
            initial-tab="detail"
            :form="form"
            :categories="categories"
            :category-tree="categoryTree"
            :tags="tags"
            :ai-summary="aiSummary"
            @jump="onJump"
            @insert="onInsert"
            @open-cover-picker="coverPickerOpen = true"
            @slug-edited="slugManuallyEdited = true"
            @update:ai-summary="aiSummary = $event"
          />
        </div>
      </div>
    </Transition>
    <!-- 移动端抽屉遮罩 -->
    <Transition name="fade">
      <div
        v-if="mobileSidebarOpen"
        class="fixed inset-0 z-[70] bg-background/40 sm:hidden"
        @click="closeMobileSidebar"
      ></div>
    </Transition>

    <AttachmentPickerModal
      :open="coverPickerOpen"
      @select="
        (url: string) => {
          form.cover = url;
          coverPickerOpen = false;
        }
      "
      @close="coverPickerOpen = false"
    />

    <AiGenerateModal
      :open="aiOpen"
      :form="form"
      :categories="categories"
      :tags="tags"
      @close="aiOpen = false"
      @apply="applyAi"
    />

    <RevisionPanel
      :open="revisionOpen"
      :slug="slug"
      :current-content="form.content_md"
      :current-word-count="currentWordCount"
      @close="revisionOpen = false"
      @restored="handleRestored"
    />
  </div>
</template>

<style scoped>
/* ===== 编辑器类型浮层选择器 ===== */
.editor-kind-picker {
  z-index: 70;
}

.editor-kind-trigger {
  display: inline-flex;
  min-width: 176px;
  min-height: 44px;
  align-items: center;
  gap: 7px;
  padding: 4px 8px;
  border: 1px solid color-mix(in srgb, #fff 14%, transparent);
  border-radius: 10px;
  background: color-mix(in srgb, var(--color-card) 78%, transparent);
  color: var(--color-text-secondary);
  font-size: 13.5px;
  font-weight: 500;
  white-space: nowrap;
  box-shadow:
    inset 0 1px 0 color-mix(in srgb, #fff 7%, transparent),
    0 1px 2px color-mix(in srgb, #000 28%, transparent);
  -webkit-backdrop-filter: blur(18px) saturate(160%);
  backdrop-filter: blur(18px) saturate(160%);
  transition:
    border-color var(--duration-fast) var(--ease-standard),
    background-color var(--duration-fast) var(--ease-standard),
    box-shadow var(--duration-fast) var(--ease-standard);
}

.editor-kind-trigger:hover,
.editor-kind-trigger.is-open {
  border-color: color-mix(in srgb, #fff 21%, transparent);
  background: color-mix(in srgb, var(--color-accent) 78%, transparent);
}

.editor-kind-trigger:focus-visible {
  border-color: var(--color-primary);
  outline: none;
  box-shadow:
    0 0 0 2px color-mix(in srgb, var(--color-ring) 18%, transparent),
    inset 0 1px 0 color-mix(in srgb, #fff 7%, transparent);
}

.editor-kind-trigger-icon,
.editor-kind-option-icon {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  color: #fff;
}

.editor-kind-trigger-icon {
  width: 25px;
  height: 25px;
  border-radius: 8px;
  font-size: 16px;
}

.editor-kind-option-icon {
  width: 34px;
  height: 34px;
  border-radius: 10px;
  font-size: 19px;
  box-shadow:
    inset 0 1px 0 color-mix(in srgb, #fff 22%, transparent),
    inset 0 -1px 1px color-mix(in srgb, #000 18%, transparent);
}

.editor-kind-icon--rich {
  background: #0a84ff;
}

.editor-kind-icon--markdown {
  background: #64d2ff;
}

.editor-kind-trigger-label {
  overflow: hidden;
  flex: 1;
  color: var(--color-foreground);
  text-align: left;
  text-overflow: ellipsis;
}

.editor-kind-chevron {
  flex: 0 0 auto;
  color: var(--color-text-tertiary);
  font-size: 17px;
  transition:
    transform var(--duration-fast) var(--ease-standard),
    color var(--duration-fast) var(--ease-standard);
}

.editor-kind-trigger.is-open .editor-kind-chevron {
  transform: rotate(180deg);
  color: var(--color-text-secondary);
}

.editor-kind-menu {
  position: absolute;
  top: calc(100% + 10px);
  right: 0;
  width: min(304px, calc(100vw - 24px));
  padding: 6px;
  border: 1px solid color-mix(in srgb, #fff 14%, transparent);
  border-radius: 14px;
  background: color-mix(in srgb, var(--color-popover) 86%, transparent);
  box-shadow:
    0 20px 44px color-mix(in srgb, #000 52%, transparent),
    0 3px 10px color-mix(in srgb, #000 24%, transparent),
    inset 0 1px 0 color-mix(in srgb, #fff 7%, transparent);
  -webkit-backdrop-filter: blur(24px) saturate(150%);
  backdrop-filter: blur(24px) saturate(150%);
}

.editor-kind-menu::before {
  position: absolute;
  top: -6px;
  right: 78px;
  width: 11px;
  height: 11px;
  border-top: 1px solid color-mix(in srgb, #fff 14%, transparent);
  border-left: 1px solid color-mix(in srgb, #fff 14%, transparent);
  background: color-mix(in srgb, var(--color-popover) 86%, transparent);
  content: "";
  transform: rotate(45deg);
}

.editor-kind-menu::after {
  position: absolute;
  top: 0;
  right: 14px;
  left: 14px;
  height: 1px;
  background: color-mix(in srgb, #fff 7%, transparent);
  content: "";
  pointer-events: none;
}

.editor-kind-option {
  display: flex;
  width: 100%;
  min-height: 52px;
  align-items: center;
  gap: 11px;
  padding: 8px 9px;
  border-radius: 10px;
  color: var(--color-text-tertiary);
  text-align: left;
  transition:
    background-color var(--duration-fast) var(--ease-standard),
    color var(--duration-fast) var(--ease-standard),
    transform var(--duration-fast) var(--ease-standard);
}

.editor-kind-option:hover,
.editor-kind-option.is-highlighted {
  background: color-mix(in srgb, #fff 8%, transparent);
  color: var(--color-foreground);
}

.editor-kind-option.is-selected {
  background: color-mix(in srgb, #fff 10%, transparent);
  color: var(--color-foreground);
}

.editor-kind-option:active {
  transform: scale(0.985);
}

.editor-kind-option:focus-visible {
  outline: 2px solid var(--color-ring);
  outline-offset: -2px;
}

.editor-kind-option-label {
  overflow: hidden;
  flex: 1;
  font-size: 14px;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.editor-kind-option-check {
  flex: 0 0 auto;
  color: var(--color-primary);
  font-size: 18px;
}

.editor-kind-menu-enter-active,
.editor-kind-menu-leave-active {
  transform-origin: top right;
  transition:
    opacity var(--duration-fast) ease,
    transform var(--duration-normal) var(--ease-standard);
}

.editor-kind-menu-enter-from,
.editor-kind-menu-leave-to {
  opacity: 0;
  transform: translateY(-5px) scale(0.98);
}

/* ===== 底部抽屉动画 ===== */
.drawer-enter-active,
.drawer-leave-active {
  transition: transform 0.3s cubic-bezier(0.32, 0.72, 0, 1);
}
.drawer-enter-from,
.drawer-leave-to {
  transform: translateY(100%);
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.25s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

/* 手机保存/预览操作保持足够触控区域，状态与模式选择独立于操作栏。 */
@media (max-width: 639px) {
  .editor-header :deep(.btn) {
    min-height: 44px;
    min-width: 44px;
    padding-right: 12px;
    padding-left: 12px;
  }
  .editor-status-bar > p {
    flex: 1 0 100%;
  }
  .editor-status-bar > div {
    width: 100%;
    justify-content: space-between;
  }
}

/* ===== 移动端 Markdown 编辑器上下分屏 ===== */
@media (max-width: 640px) {
  .editor-wrap :deep(.grid.grid-cols-2) {
    grid-template-columns: 1fr !important;
    grid-template-rows: 1fr 1fr;
  }
  .editor-wrap :deep(.grid.grid-cols-2 textarea) {
    border-right: none !important;
    border-bottom: 1px solid var(--color-border) !important;
    min-height: 40vh !important;
    max-height: 50vh !important;
  }
  .editor-wrap :deep(.grid.grid-cols-2 > div:last-child),
  .editor-wrap :deep(.grid.grid-cols-2 > div.relative) {
    min-height: 40vh !important;
    max-height: 50vh !important;
  }
  .editor-wrap :deep(.grid.grid-cols-2 iframe) {
    min-height: 40vh !important;
    height: 50vh !important;
  }
}

/* ===== 移动端 editor-wrap：保证富文本/Markdown 编辑区高度合理 ===== */
@media (max-width: 640px) {
  .editor-wrap {
    min-height: calc(100vh - 56px - 48px);
  }
  /* 富文本编辑器内容区最小高度 */
  :deep(.ProseMirror),
  :deep(.md-editor-content) {
    min-height: 50vh;
  }
}
</style>
