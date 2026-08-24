<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed, watch, defineAsyncComponent, nextTick } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useQueryClient } from "@tanstack/vue-query";
import { postApi, taxonomyApi, previewApi } from "@/api";
import { queryKeys } from "@/api/query-client";
import MarkdownEditor from "@/components/MarkdownEditor.vue";
// Spec 62：tiptap 190KB gzip chunk 按需挂载——仅富文本模式启用时才下载
const RichTextEditor = defineAsyncComponent(() => import("@/components/RichTextEditor.vue"));
import AttachmentPickerModal from "@/components/AttachmentPickerModal.vue";
import EditorToolbar from "./EditorToolbar.vue";
import EditorSidebar from "./EditorSidebar.vue";
import AiGenerateModal from "./AiGenerateModal.vue";
import RevisionPanel from "./RevisionPanel.vue";
import { Icon } from "@iconify/vue";
import { Toast, Dialog } from "@/components/ui";

const queryClient = useQueryClient();

// 文章编辑器：Halo 式三栏布局——顶部全宽工具栏 + 左侧主编辑区 + 右侧大纲/详情面板。
// 双编辑器模式：默认富文本所见即得（TipTap），可切 Markdown 分屏兑底；
// 两者绑定同一份 form.content_md，切换即天然同步。
const route = useRoute();
const router = useRouter();

const isEdit = computed(() => !!route.params.slug);
const slug = computed(() => route.params.slug as string);

const form = ref({
  slug: "",
  title: "",
  excerpt: "",
  cover: "",
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
  authors: [] as string[],
  author: "",
});

const loading = ref(false);
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

function onEditorKindSelect(value: EditorKind) {
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
  if (e.key === "Escape" && editorKindOpen.value) {
    editorKindOpen.value = false;
    nextTick(() => editorKindTrigger.value?.focus());
  }
}

// 草稿头状态：originalStatus 为服务器线上状态，hasDraft 表示存在已保存未发布的草稿
const originalStatus = ref("draft");
const hasDraft = ref(false);
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
function setEditorMode(m: EditorMode) {
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
    const { data } = await previewApi.createPostPreview({ ...form.value });
    window.open(`/preview/draft/${data.token}`, "_blank");
  } catch (e: any) {
    Toast.error(e.response?.data?.error || "预览生成失败");
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
function toggleSidebar() {
  sidebarOpen.value = !sidebarOpen.value;
  localStorage.setItem(SIDEBAR_KEY, sidebarOpen.value ? "1" : "0");
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
  const { data } = await postApi.get(slug.value);
  originalStatus.value = data.status;
  hasDraft.value = !!data.has_draft;
  // 有草稿头时优先载入草稿内容（线上内容不受影响）
  const src = data.has_draft && data.draft ? { ...data, ...data.draft } : data;
  form.value = {
    slug: src.slug || data.slug,
    title: src.title,
    excerpt: src.excerpt,
    cover: src.cover,
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
    authors: Array.isArray(src.authors)
      ? src.authors.map((id: unknown) => String(id).trim()).filter(Boolean)
      : src.author
        ? [String(src.author)]
        : [],
    author: src.author || (Array.isArray(src.authors) && src.authors[0] ? src.authors[0] : ""),
  };
  aiSummary.value = data.ai_summary || ""; // Spec 53：AI 摘要随文章详情载入
}

onMounted(async () => {
  document.addEventListener("pointerdown", onEditorKindPointerDown);
  document.addEventListener("keydown", onEditorKindDocumentKeydown);
  loadTaxonomy();
  editorMode.value = (editorRef.value?.getMode?.() as EditorMode) || "split";
  if (!isEdit.value) return;
  loading.value = true;
  try {
    await loadPost();
  } catch (e: any) {
    Toast.error("加载失败");
  } finally {
    loading.value = false;
  }
});

onBeforeUnmount(() => {
  document.removeEventListener("pointerdown", onEditorKindPointerDown);
  document.removeEventListener("keydown", onEditorKindDocumentKeydown);
});

// 回滚成功后重新拉取文章内容到编辑器。
async function handleRestored() {
  try {
    await loadPost();
    clearActiveDraft();
  } catch (e: any) {
    Toast.error("回滚后重载失败");
  }
}

/** 保存：已发布文章 → 存草稿头（线上不变，列表出小绿点）；草稿/新建 → 直接保存为草稿 */
async function handleSave() {
  saving.value = true;
  try {
    if (isPublishedPost.value) {
      await postApi.saveDraft(slug.value, { ...form.value, status: "published" });
      hasDraft.value = true;
      clearActiveDraft();
      Toast.success("内容已保存，但还未发布");
    } else if (isEdit.value) {
      await postApi.update(slug.value, { ...form.value, status: "draft" });
      clearActiveDraft();
      Toast.success("保存成功");
    } else {
      const { data } = await postApi.create({ ...form.value, status: "draft" });
      clearActiveDraft();
      Toast.success("保存成功");
      // 转入编辑模式，后续保存走更新
      router.replace(`/posts/edit/${data.slug}`);
      originalStatus.value = "draft";
    }
    // Spec 62：保存后失效文章列表缓存（返回列表时草稿小绿点/标题即时反映，不等 staleTime）
    queryClient.invalidateQueries({ queryKey: queryKeys.posts() });
  } catch (e: any) {
    Toast.error(e.response?.data?.error || "保存失败");
  } finally {
    saving.value = false;
  }
}

/** 发布：应用草稿头并上线（新建/草稿文章直接以发布状态保存） */
async function handlePublish() {
  publishing.value = true;
  try {
    if (isEdit.value) {
      await postApi.publish(slug.value, form.value);
    } else {
      await postApi.create({ ...form.value, status: "published" });
    }
    clearActiveDraft();
    Toast.success("发布成功");
    // Spec 62：发布后失效文章列表缓存，列表页立即显示新状态
    queryClient.invalidateQueries({ queryKey: queryKeys.posts() });
    router.push("/posts");
  } catch (e: any) {
    Toast.error(e.response?.data?.error || "发布失败");
  } finally {
    publishing.value = false;
  }
}

/** 放弃草稿头，回到线上内容 */
function handleDiscardDraft() {
  Dialog.warning({
    title: "确定要放弃未发布的草稿吗？",
    description: "将丢弃已保存的草稿内容，恢复为线上发布版本",
    confirmType: "danger",
    onConfirm: async () => {
      try {
        await postApi.discardDraft(slug.value);
        hasDraft.value = false;
        await loadPost();
        clearActiveDraft();
        Toast.success("已放弃草稿");
      } catch (e: any) {
        Toast.error(e.response?.data?.error || "操作失败");
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
        class="flex h-8 w-8 items-center justify-center rounded-lg text-text-secondary transition-colors hover:bg-secondary hover:text-foreground"
        @click="router.push('/posts')"
      >
        <Icon icon="ri:arrow-left-s-line" class="text-2xl" />
      </button>
      <h1 class="shrink-0 whitespace-nowrap text-base font-semibold tracking-tight text-foreground sm:text-[17px]">
        {{ isEdit ? "编辑文章" : "新建文章" }}
      </h1>
      <div class="flex-1"></div>
      <div class="flex shrink-0 items-center gap-1.5 sm:gap-2">
        <!-- 编辑器选择（富文本/Markdown） -->
        <div class="editor-kind-picker relative hidden sm:block">
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
        <button
          type="button"
          class="inline-flex h-8 w-8 items-center justify-center rounded-full text-text-secondary transition-colors hover:bg-secondary hover:text-foreground sm:h-[34px] sm:w-auto sm:gap-1.5 sm:px-3.5 sm:text-[13px] sm:font-semibold"
          :disabled="previewing"
          @click="handlePreview"
        >
          <Icon icon="ri:eye-line" class="text-base sm:text-[15px]" />
          <span class="hidden sm:inline">预览</span>
        </button>
        <!-- 保存草稿（移动端仅图标） -->
        <button
          type="button"
          class="inline-flex h-8 w-8 items-center justify-center rounded-full border border-border bg-secondary text-foreground transition-colors hover:bg-accent disabled:opacity-50 sm:h-[34px] sm:w-auto sm:gap-1.5 sm:px-3.5 sm:text-[13px] sm:font-semibold"
          :disabled="saving"
          @click="handleSave"
          :title="saving ? '保存中…' : '保存草稿'"
        >
          <Icon icon="ri:save-line" class="text-base sm:text-[15px]" />
          <span class="hidden sm:inline">{{ saving ? "保存中…" : "保存草稿" }}</span>
        </button>
        <!-- 发布（primary，移动端保持文字但更紧凑） -->
        <button
          type="button"
          class="inline-flex h-8 items-center gap-1 rounded-full bg-primary px-3.5 text-xs font-semibold text-primary-foreground transition-colors hover:bg-primary-hover disabled:opacity-50 sm:h-[34px] sm:gap-1.5 sm:px-4 sm:text-[13px]"
          :disabled="publishing"
          @click="handlePublish"
        >
          <Icon icon="ri:send-plane-fill" class="text-sm sm:text-[15px]" />
          <span>{{ publishing ? "发布中…" : "发布" }}</span>
        </button>
      </div>
    </header>

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
          <input
            v-model="form.title"
            placeholder="文章标题"
            class="w-full border-0 bg-transparent text-2xl font-bold leading-tight tracking-tight text-foreground outline-none placeholder:text-text-quaternary placeholder:font-semibold sm:text-[28px]"
          />
        </div>

        <!-- 工具栏（独立卡片条） -->
        <div class="border border-b-0 border-border bg-card">
          <EditorToolbar
            :kind="editorKind"
            :mode="editorMode"
            :sidebar-open="sidebarOpen"
            :is-edit="isEdit"
            :fullscreen="fullscreen"
            :active-states="toolbarActive"
            @format="onFormat"
            @insert="onInsert"
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
          <RichTextEditor
            v-if="editorKind === 'rich'"
            ref="richRef"
            v-model="form.content_md"
            :storage-key="'post:' + (slug || 'new')"
            class="!border-0"
            @save="handleSave"
          />
          <MarkdownEditor
            v-else
            ref="editorRef"
            v-model="form.content_md"
            :toolbar="false"
            :storage-key="'post:' + (slug || 'new')"
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
        v-if="sidebarOpen"
        class="sidebar-mobile fixed inset-x-0 bottom-0 z-[80] sm:hidden"
        style="
          max-height: 85vh;
          background: var(--color-card);
          border-top: 1px solid var(--color-border);
          border-top-left-radius: var(--radius-lg);
          border-top-right-radius: var(--radius-lg);
          box-shadow: var(--shadow-xl);
        "
      >
        <div class="flex justify-center pt-2 pb-1">
          <div class="h-1 w-10 rounded-full bg-border"></div>
        </div>
        <div class="overflow-y-auto" style="max-height: calc(85vh - 24px)">
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
    </Transition>
    <!-- 移动端抽屉遮罩 -->
    <Transition name="fade">
      <div v-if="sidebarOpen" class="fixed inset-0 z-[70] bg-background/40 sm:hidden" @click="toggleSidebar"></div>
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
  min-height: 36px;
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
