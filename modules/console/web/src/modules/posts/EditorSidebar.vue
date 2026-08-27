<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from "vue";
import { Icon } from "@iconify/vue";
import { previewApi, settingsApi, postApi, aiApi, topicApi, type TopicItem } from "@/api";
import PluginParamForm from "@/modules/plugins/PluginParamForm.vue";
import { VButton, VSwitch, Toast } from "@/components/ui";
import { type PluginMeta, type PluginCategory } from "@/modules/plugins/plugin-types";
import { PLUGIN_CATEGORIES } from "@/modules/plugins/plugin-types";
import { loadPluginCatalog, runtimePlugins } from "@/modules/plugins/usePluginCatalog";

// 编辑器右侧辅助面板（对照 Halo）：大纲 / 详情 两个 Tab。
// 大纲：解析 Markdown 标题可点击跳转；详情：合并原右栏全部设置字段与标签插件参数面板。
interface PostForm {
  slug: string;
  title: string;
  excerpt: string;
  cover: string;
  category: string;
  categories: string[];
  topic_id: number | null;
  topic: string;
  tags: string;
  content_md: string;
  status: string;
  pinned: boolean;
  password: string;
  publish_at: string;
  meta_description: string;
  meta_keywords: string;
  authors: string[];
  author: string;
}

interface CategoryNode {
  id: number;
  name: string;
  path: string;
  parent_id: number;
  count: number;
  children?: CategoryNode[];
}

const props = defineProps<{
  form: PostForm;
  categories: string[];
  categoryTree: CategoryNode[];
  tags: string[];
  aiSummary: string; // Spec 53：AI 摘要由 PostEditor 持有（与 AiGenerateModal 回填共用一份状态）
}>();

const emit = defineEmits<{
  (e: "jump", line: number, index: number): void;
  (e: "insert", code: string): void;
  (e: "openCoverPicker"): void;
  (e: "slugEdited"): void;
  (e: "update:aiSummary", value: string): void;
}>();

const activeTab = ref<"outline" | "detail">("outline");

// ---------- 作者可选项（Spec 38 Wave 5）：authors 为空 map 时不渲染控件 ----------
interface AuthorOption {
  name: string;
  avatar?: string;
  description?: string;
  url?: string;
  hidden?: boolean;
}
const authorOptions = ref<Record<string, AuthorOption>>({});

function normalizeAuthors(raw: unknown): Record<string, AuthorOption> {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return {};

  const normalized: Record<string, AuthorOption> = {};
  for (const [rawID, rawValue] of Object.entries(raw as Record<string, unknown>)) {
    const id = rawID.trim();
    if (!id || !rawValue || typeof rawValue !== "object" || Array.isArray(rawValue)) continue;

    const value = rawValue as Record<string, unknown>;
    const name = typeof value.name === "string" ? value.name.trim() : "";
    // AuthorsTab allows an unfinished row while editing. It must not become a
    // selectable author until it has a display name.
    if (!name) continue;

    normalized[id] = {
      name,
      avatar: typeof value.avatar === "string" ? value.avatar.trim() : "",
      description: typeof value.description === "string" ? value.description.trim() : "",
      url: typeof value.url === "string" ? value.url.trim() : "",
      hidden: value.hidden === true,
    };
  }
  return normalized;
}

async function loadAuthors() {
  try {
    const { data } = await settingsApi.all();
    authorOptions.value = normalizeAuthors(data?.authors);
  } catch {
    authorOptions.value = {};
    /* 静默：作者列表缺失不阻塞编辑 */
  }
}

// 作者选择器（头像式）：当前作者头像 / 未选时「+」按钮，点击弹出选择 popover，
// hover 头像显示悬浮气泡（名称 + 简介 + 主页）。
const authorPopOpen = ref(false);
const authorPopRef = ref<HTMLElement | null>(null);

const selectedAuthorIDs = computed(() =>
  Array.isArray(props.form.authors) && props.form.authors.length
    ? props.form.authors.filter((id, index, ids) => id && ids.indexOf(id) === index)
    : props.form.author
      ? [props.form.author]
      : [],
);

const selectedAuthors = computed(() =>
  selectedAuthorIDs.value
    .map((id) => ({ id, author: authorOptions.value[id] }))
    .filter((item): item is { id: string; author: AuthorOption } => !!item.author),
);

const visibleAuthors = computed(() =>
  Object.entries(authorOptions.value)
    .filter(([, a]) => !a.hidden)
    .map(([id, a]) => ({ id, ...a })),
);

// 已隐藏的当前作者仍保留在候选项中，避免编辑旧文章时作者名称突然回退成站长；
// 其他文章只能选择未隐藏作者。
const authorChoices = computed(() => {
  const choices = [...visibleAuthors.value];
  for (const currentID of selectedAuthorIDs.value) {
    const current = authorOptions.value[currentID];
    if (current?.hidden && !choices.some((author) => author.id === currentID))
      choices.push({ id: currentID, ...current });
  }
  return choices;
});

const hasAuthorSettings = computed(() => Object.keys(authorOptions.value).length > 0);

function toggleAuthorPicker() {
  authorPopOpen.value = !authorPopOpen.value;
}

function setAuthorIDs(ids: string[]) {
  const next = ids.filter((id, index) => id && ids.indexOf(id) === index);
  props.form.authors = next;
  props.form.author = next[0] || "";
}

function toggleAuthor(id: string) {
  if (!id) {
    setAuthorIDs([]);
    return;
  }
  const current = selectedAuthorIDs.value;
  const next = current.includes(id) ? current.filter((item) => item !== id) : [...current, id];
  setAuthorIDs(next);
}

function onDocClickAuthorPop(e: MouseEvent) {
  if (!authorPopOpen.value) return;
  const el = authorPopRef.value;
  if (el && !el.contains(e.target as Node)) authorPopOpen.value = false;
}

onMounted(() => {
  loadAuthors();
  loadPluginCatalog();
  document.addEventListener("click", onDocClickAuthorPop);
});
onBeforeUnmount(() => document.removeEventListener("click", onDocClickAuthorPop));

// ---------- 专栏选择器：仅选择已有专栏，不在编辑器内创建 ----------
const topicOptions = ref<TopicItem[]>([]);
const topicQuery = ref("");
const topicPickerOpen = ref(false);
const topicPickerRoot = ref<HTMLElement | null>(null);

const selectedTopic = computed(() => {
  const id = Number(props.form.topic_id || 0);
  return id > 0 ? topicOptions.value.find((topic) => topic.id === id) || null : null;
});

const filteredTopics = computed(() => {
  const query = topicQuery.value.trim().toLowerCase();
  if (!query) return topicOptions.value;
  return topicOptions.value.filter((topic) => topic.name.toLowerCase().includes(query));
});

function syncTopicFromLegacy() {
  if (!topicOptions.value.length) return;
  const currentID = Number(props.form.topic_id || 0);
  if (currentID > 0) {
    const current = topicOptions.value.find((topic) => topic.id === currentID);
    if (current && props.form.topic !== current.name) props.form.topic = current.name;
    return;
  }
  const legacyName = props.form.topic.trim();
  if (!legacyName) return;
  const legacy = topicOptions.value.find((topic) => topic.name === legacyName);
  if (legacy) {
    props.form.topic_id = legacy.id;
    props.form.topic = legacy.name;
  } else {
    props.form.topic_id = null;
    props.form.topic = "";
  }
}

async function loadTopics() {
  try {
    const { data } = await topicApi.list();
    topicOptions.value = data.items || [];
    syncTopicFromLegacy();
  } catch {
    // 专栏是可选元数据，列表失败不阻塞正文编辑。
  }
}

function toggleTopicPicker() {
  topicPickerOpen.value = !topicPickerOpen.value;
  if (topicPickerOpen.value) topicQuery.value = "";
}

function selectTopic(topic: TopicItem) {
  props.form.topic_id = topic.id;
  props.form.topic = topic.name;
  topicPickerOpen.value = false;
}

function clearTopic() {
  props.form.topic_id = null;
  props.form.topic = "";
  topicPickerOpen.value = false;
}

function onTopicClickOutside(event: MouseEvent) {
  if (topicPickerRoot.value && !topicPickerRoot.value.contains(event.target as Node)) {
    topicPickerOpen.value = false;
  }
}

watch(() => [props.form.topic_id, props.form.topic, topicOptions.value.length], syncTopicFromLegacy);
onMounted(() => {
  loadTopics();
  document.addEventListener("mousedown", onTopicClickOutside);
});

// ---------- 大纲：解析 ATX 标题（跳过代码围栏），点击跳转到对应行 ----------

interface OutlineItem {
  level: number;
  text: string;
  line: number;
}

const outline = computed<OutlineItem[]>(() => {
  const items: OutlineItem[] = [];
  let inFence = false;
  const lines = (props.form.content_md || "").split("\n");
  lines.forEach((raw, i) => {
    const line = raw.trim();
    if (/^(```|~~~)/.test(line)) {
      inFence = !inFence;
      return;
    }
    if (inFence) return;
    const m = /^(#{1,6})\s+(.+?)\s*#*\s*$/.exec(line);
    if (m) items.push({ level: m[1].length, text: m[2], line: i });
  });
  return items;
});

// 顶层标题级别（缩进以此为基准，如全文最高是 h2 则 h2 不缩进）
const minLevel = computed(() => (outline.value.length ? Math.min(...outline.value.map((i) => i.level)) : 1));

// ---------- 详情：分类/标签树 ----------

const categoryQuery = ref("");

interface CategoryOption {
  path: string;
  name: string;
  count: number;
  depth: number;
}

function flattenCategoryTree(nodes: CategoryNode[], depth = 0): CategoryOption[] {
  const out: CategoryOption[] = [];
  for (const node of nodes) {
    out.push({ path: node.path || node.name, name: node.name, count: node.count, depth });
    if (node.children?.length) out.push(...flattenCategoryTree(node.children, depth + 1));
  }
  return out;
}

const categoryOptions = computed<CategoryOption[]>(() => {
  const tree = flattenCategoryTree(props.categoryTree || []);
  if (tree.length) return tree;
  return props.categories.map((path) => {
    const parts = path.split("/");
    return { path, name: parts[parts.length - 1], count: 0, depth: Math.max(0, parts.length - 1) };
  });
});

const filteredCategoryOptions = computed(() => {
  const query = categoryQuery.value.trim().toLowerCase();
  if (!query) return categoryOptions.value;
  return categoryOptions.value.filter(
    (category) => category.path.toLowerCase().includes(query) || category.name.toLowerCase().includes(query),
  );
});

function pickCategory(name: string) {
  const selected = Array.isArray(props.form.categories) ? [...props.form.categories] : [];
  const index = selected.indexOf(name);
  if (index >= 0) selected.splice(index, 1);
  else selected.push(name);
  props.form.categories = selected;
  props.form.category = selected[0] || "";
}

function addCategoryFromInput() {
  const path = categoryQuery.value.trim().replace(/^\/+|\/+$/g, "");
  if (!path) return;
  const selected = Array.isArray(props.form.categories) ? [...props.form.categories] : [];
  if (!selected.includes(path)) selected.push(path);
  props.form.categories = selected;
  props.form.category = selected[0] || "";
  categoryQuery.value = "";
}

// 点标签 chip：toggle 语义——已选中则移除（可取消标签），未选中则追加；逗号分隔去重
function pickTag(name: string) {
  const list = props.form.tags
    .split(/[,，]/)
    .map((s) => s.trim())
    .filter(Boolean);
  const idx = list.indexOf(name);
  if (idx >= 0) list.splice(idx, 1);
  else list.push(name);
  props.form.tags = list.join(",");
}

/** 随机封面图：picsum seed 模式，保存后 URL 固定图片不再变化 */
function fetchRandomCover() {
  const seed = Math.random().toString(36).slice(2, 10);
  props.form.cover = `https://picsum.photos/seed/${seed}/1200/675`;
}

// ---------- 标签插件参数面板（自 PostEditor 迁入，Spec 29） ----------

const pluginCat = ref<PluginCategory | "全部">("全部");
const pluginList = computed(() =>
  pluginCat.value === "全部"
    ? runtimePlugins.value
    : runtimePlugins.value.filter((p) => p.category === pluginCat.value),
);
const selectedPlugin = ref<PluginMeta | null>(null);
const pluginCode = ref("");
const pluginPreviewHtml = ref("");
const pluginRendering = ref(false);
let pluginTimer: ReturnType<typeof setTimeout> | undefined;

function selectPlugin(p: PluginMeta) {
  selectedPlugin.value = p;
  // 有 form schema：由 PluginParamForm 初始化后立即产出 shortcode；否则回退示例文本
  pluginCode.value = p.form ? "" : p.example;
  schedulePluginPreview(true);
}

function onPluginCode(code: string) {
  pluginCode.value = code;
  schedulePluginPreview();
}

async function refreshPluginPreview() {
  if (!pluginCode.value) {
    pluginPreviewHtml.value = "";
    return;
  }
  pluginRendering.value = true;
  try {
    const { data } = await previewApi.render(pluginCode.value);
    pluginPreviewHtml.value = data.html;
  } catch {
    pluginPreviewHtml.value = '<p style="color:var(--color-state-error, #FF453A)">预览渲染失败</p>';
  } finally {
    pluginRendering.value = false;
  }
}

function schedulePluginPreview(immediate = false) {
  clearTimeout(pluginTimer);
  pluginTimer = setTimeout(refreshPluginPreview, immediate ? 0 : 400);
}

const pluginSrcdoc = computed(() => {
  return (
    `<!DOCTYPE html>
<html data-theme="dark">
<head>
<meta charset="utf-8">
<link rel="stylesheet" href="/assets/css/main.css">
<link rel="stylesheet" href="/assets/css/tag-plugins.css">
<link rel="stylesheet" href="/assets/css/highlight.css">
<style>html,body{margin:0;padding:0;}/* isolated iframe fallback keeps the console card token */body{padding:16px;background:var(--color-card, #1c1c1e);}.md-text{max-width:100%;}</style>
</head>
<body><article class="md-text content">${pluginPreviewHtml.value}</article><scr` +
    "ipt>\n" +
    "document.querySelectorAll('.tabs .nav-tabs .tab').forEach(function(el) {\n" +
    " el.addEventListener('click', function(e) {\n" +
    " e.preventDefault();\n" +
    " if (el.classList.contains('active')) return;\n" +
    " Array.prototype.forEach.call(el.parentNode.children, function(t) {\n" +
    " t.classList.toggle('active', t === el);\n" +
    " });\n" +
    " var a = el.querySelector('a');\n" +
    " if (!a) return;\n" +
    " var target = document.getElementById(a.getAttribute('href').replace('#', ''));\n" +
    " if (!target) return;\n" +
    " Array.prototype.forEach.call(target.parentNode.children, function(t) {\n" +
    " t.classList.toggle('active', t === target);\n" +
    " });\n" +
    " });\n" +
    "});\n" +
    "<\/script></body>\n" +
    "</html>"
  );
});

function insertPlugin() {
  if (!pluginCode.value) return;
  emit("insert", "\n" + pluginCode.value + "\n");
}

onBeforeUnmount(() => {
  clearTimeout(pluginTimer);
  document.removeEventListener("mousedown", onTopicClickOutside);
});

// ---------- AI 摘要（Spec 53）：textarea 手动编辑 + AI 生成，保存走独立端点 ----------
const aiSaving = ref(false);
const aiGenerating = ref(false);

async function saveAiSummary() {
  if (!props.form.slug) {
    Toast.warning("请先保存文章（生成 slug）后再保存摘要");
    return;
  }
  aiSaving.value = true;
  try {
    await postApi.updateAISummary(props.form.slug, props.aiSummary.trim());
    Toast.success("AI 摘要已保存");
  } catch {
  } finally {
    aiSaving.value = false;
  }
}

// 清空：直接传空串落库（不依赖 props 回写时序）
async function clearAiSummary() {
  if (!props.form.slug) {
    emit("update:aiSummary", "");
    return;
  }
  aiSaving.value = true;
  try {
    await postApi.updateAISummary(props.form.slug, "");
    emit("update:aiSummary", "");
    Toast.success("AI 摘要已清空");
  } catch {
  } finally {
    aiSaving.value = false;
  }
}

async function generateAiSummary() {
  if (!props.form.title && !props.form.content_md) {
    Toast.warning("标题与正文均为空，无法生成");
    return;
  }
  aiGenerating.value = true;
  try {
    const { data } = await aiApi.generate({
      fields: ["summary"],
      title: props.form.title,
      content: props.form.content_md,
      categories: [],
      tags: [],
    });
    if (data.summary) {
      emit("update:aiSummary", data.summary);
      // 已有 slug 时直接落库，免去再点一次保存
      if (props.form.slug) {
        await postApi.updateAISummary(props.form.slug, data.summary);
        Toast.success("AI 摘要已生成并保存");
      } else {
        Toast.success("AI 摘要已生成，保存文章后可落库");
      }
    } else {
      Toast.error("生成结果为空，请重试");
    }
  } catch {
  } finally {
    aiGenerating.value = false;
  }
}

const fieldCls = "apple-input";
const textareaCls = "apple-textarea";
const selectCls = "apple-select";
const labelCls = "mb-1.5 block text-[12.5px] font-medium text-text-secondary";
const cardCls = "rounded-[14px] border border-border bg-card";
const cardHeaderCls = "px-4 pb-3 pt-3.5 text-[11px] font-semibold uppercase tracking-[0.6px] text-text-tertiary";
const cardBodyCls = "px-4 pb-4";
const chipCls = "inline-flex items-center gap-1 rounded-md px-2.5 py-1 text-xs font-medium transition-colors";
const chipIdleCls = "bg-secondary text-text-secondary hover:bg-accent";
const chipActiveCls = "bg-primary/15 text-primary";
</script>

<template>
  <div class="flex flex-col gap-3">
    <!-- Tab 切换（大纲 / 设置） -->
    <div :class="cardCls">
      <div class="p-1.5">
        <div class="grid grid-cols-2 gap-0.5 rounded-lg bg-secondary p-0.5">
          <button
            v-for="tab in [
              { key: 'outline', label: '大纲' },
              { key: 'detail', label: '设置' },
            ]"
            :key="tab.key"
            type="button"
            :class="[
              'rounded-md py-1.5 text-[13px] transition-colors',
              activeTab === tab.key
                ? 'bg-card font-medium text-foreground shadow-sm'
                : 'text-text-tertiary hover:text-foreground',
            ]"
            @click="activeTab = tab.key as 'outline' | 'detail'"
          >
            {{ tab.label }}
          </button>
        </div>
      </div>
    </div>

    <!-- 大纲 -->
    <div v-if="activeTab === 'outline'" :class="cardCls">
      <div class="min-h-0 flex-1 overflow-y-auto px-2 py-3">
        <p v-if="outline.length === 0" class="py-10 text-center text-sm text-text-quaternary">暂无大纲</p>
        <ul v-else class="space-y-0.5">
          <li v-for="(item, i) in outline" :key="i">
            <button
              type="button"
              class="block w-full truncate rounded-md px-2 py-1.5 text-left text-[13px] text-text-secondary transition-colors hover:bg-accent hover:text-foreground"
              :style="{ paddingLeft: 8 + (item.level - minLevel) * 14 + 'px' }"
              :title="item.text"
              @click="emit('jump', item.line, i)"
            >
              {{ item.text }}
            </button>
          </li>
        </ul>
      </div>
    </div>

    <!-- 设置：四张卡片 -->
    <template v-else>
      <!-- 卡片 1：发布设置 -->
      <div :class="cardCls">
        <div :class="cardHeaderCls">发布设置</div>
        <div :class="cardBodyCls + ' space-y-3.5'">
          <div>
            <label :class="labelCls">别名 (Slug)</label>
            <input v-model="form.slug" :class="fieldCls" @input="emit('slugEdited')" />
            <p class="mt-1 text-[11px] text-text-quaternary">用于文章 URL，留空保存时从标题自动生成</p>
          </div>
          <div v-if="hasAuthorSettings">
            <label :class="labelCls">作者（可多选）</label>
            <div ref="authorPopRef" class="relative">
              <div class="flex items-center gap-2.5">
                <button
                  type="button"
                  class="flex min-h-9 min-w-9 shrink-0 items-center justify-center overflow-hidden rounded-full border border-border bg-secondary px-1.5 text-text-secondary transition-colors hover:border-primary/50 hover:text-primary"
                  :aria-expanded="authorPopOpen"
                  aria-haspopup="listbox"
                  title="选择作者"
                  @click="toggleAuthorPicker"
                >
                  <span v-if="selectedAuthors.length" class="flex -space-x-1">
                    <span
                      v-for="item in selectedAuthors.slice(0, 3)"
                      :key="item.id"
                      class="flex h-6 w-6 items-center justify-center overflow-hidden rounded-full border border-card bg-secondary text-[11px] text-text-secondary"
                    >
                      <img
                        v-if="item.author.avatar"
                        :src="item.author.avatar"
                        :alt="item.author.name"
                        class="h-full w-full object-cover"
                      />
                      <span v-else>{{ item.author.name.slice(0, 1) }}</span>
                    </span>
                  </span>
                  <Icon v-else icon="ri:add-line" class="h-4 w-4" />
                </button>
                <div class="min-w-0 flex-1">
                  <div class="truncate text-[12.5px] text-foreground">
                    {{
                      selectedAuthors.length
                        ? selectedAuthors.map((item) => item.author.name).join("、")
                        : "默认（站长）"
                    }}
                  </div>
                  <button
                    type="button"
                    class="text-[11px] text-text-quaternary transition-colors hover:text-primary"
                    :aria-expanded="authorPopOpen"
                    aria-haspopup="listbox"
                    @click="toggleAuthorPicker"
                  >
                    {{ selectedAuthors.length ? "调整作者" : "选择作者" }}
                  </button>
                </div>
              </div>
              <!-- 选择 popover：默认（站长）+ 全部可见作者，保持打开以便连续多选 -->
              <div
                v-if="authorPopOpen"
                role="listbox"
                aria-label="作者列表"
                class="absolute left-0 top-full z-30 mt-1.5 w-full rounded-lg border border-border bg-card p-1 shadow-xl"
              >
                <button
                  type="button"
                  role="option"
                  :aria-selected="selectedAuthorIDs.length === 0"
                  class="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-[12.5px] transition-colors hover:bg-accent"
                  :class="selectedAuthorIDs.length === 0 ? 'text-primary' : 'text-foreground'"
                  @click="toggleAuthor('')"
                >
                  <span
                    class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-secondary text-[11px] text-text-secondary"
                    >站</span
                  >
                  默认（站长）
                  <Icon
                    v-if="selectedAuthorIDs.length === 0"
                    icon="ri:check-line"
                    class="ml-auto h-3.5 w-3.5 shrink-0"
                  />
                </button>
                <button
                  v-for="a in authorChoices"
                  :key="a.id"
                  type="button"
                  role="option"
                  :aria-selected="selectedAuthorIDs.includes(a.id)"
                  class="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-[12.5px] transition-colors hover:bg-accent"
                  :class="selectedAuthorIDs.includes(a.id) ? 'text-primary' : 'text-foreground'"
                  @click="toggleAuthor(a.id)"
                >
                  <img
                    v-if="a.avatar"
                    :src="a.avatar"
                    :alt="a.name"
                    class="aspect-square h-6 w-6 shrink-0 rounded-full object-cover"
                  />
                  <span
                    v-else
                    class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-secondary text-[11px] text-text-secondary"
                  >
                    {{ a.name.slice(0, 1) }}
                  </span>
                  <span class="min-w-0 flex-1 truncate">{{ a.name }}</span>
                  <span v-if="a.hidden" class="shrink-0 text-[10px] text-text-quaternary">已隐藏</span>
                  <Icon v-if="selectedAuthorIDs.includes(a.id)" icon="ri:check-line" class="h-3.5 w-3.5 shrink-0" />
                </button>
              </div>
            </div>
          </div>
          <div class="flex items-center justify-between py-0.5">
            <div>
              <div class="text-[13px] font-medium text-foreground">置顶</div>
              <div class="mt-0.5 text-[11px] text-text-tertiary">在文章列表顶部显示</div>
            </div>
            <VSwitch v-model="form.pinned" />
          </div>
          <div>
            <label :class="labelCls">定时发布</label>
            <input v-model="form.publish_at" type="datetime-local" :class="fieldCls" />
          </div>
          <div>
            <label :class="labelCls">阅读密码</label>
            <input v-model="form.password" type="password" :class="fieldCls" />
          </div>
        </div>
      </div>

      <!-- 卡片 2：分类与标签 -->
      <div :class="cardCls">
        <div :class="cardHeaderCls">分类与标签</div>
        <div :class="cardBodyCls + ' space-y-3.5'">
          <div>
            <label :class="labelCls">分类</label>
            <input
              v-model="categoryQuery"
              :class="fieldCls"
              placeholder="搜索分类，回车可创建路径，如 产研/技术"
              @keyup.enter="addCategoryFromInput"
            />
            <div v-if="form.categories?.length" class="mt-2 flex flex-wrap gap-1.5">
              <span
                v-for="c in form.categories"
                :key="c"
                class="inline-flex items-center gap-1 rounded-full bg-accent px-2 py-1 text-xs text-foreground"
              >
                {{ c }}
                <button type="button" class="text-text-quaternary hover:text-foreground" @click="pickCategory(c)">
                  ×
                </button>
              </span>
            </div>
            <div
              v-if="categoryOptions.length"
              class="mt-2 max-h-48 overflow-y-auto rounded-md border border-border bg-secondary/40 p-1.5"
            >
              <button
                v-for="c in filteredCategoryOptions"
                :key="c.path"
                type="button"
                :class="[
                  'flex w-full items-center gap-1.5 rounded-md px-2 py-1.5 text-left text-xs transition-colors',
                  form.categories?.includes(c.path)
                    ? 'bg-accent text-primary'
                    : 'text-text-secondary hover:bg-accent/60 hover:text-foreground',
                ]"
                :style="{ paddingLeft: 8 + c.depth * 16 + 'px' }"
                @click="pickCategory(c.path)"
              >
                <Icon
                  :icon="c.depth ? 'ri:arrow-right-s-line' : 'ri:folder-2-line'"
                  class="shrink-0 text-text-quaternary"
                />
                <span class="min-w-0 flex-1 truncate">{{ c.name }}</span>
                <span class="shrink-0 text-[10px] text-text-quaternary">{{ c.path }}</span>
                <Icon v-if="form.categories?.includes(c.path)" icon="ri:check-line" class="shrink-0 text-primary" />
              </button>
              <p v-if="filteredCategoryOptions.length === 0" class="px-2 py-2 text-xs text-text-quaternary">
                没有匹配的分类，按回车创建路径
              </p>
            </div>
          </div>
          <div>
            <label :class="labelCls">专栏</label>
            <div ref="topicPickerRoot" class="relative">
              <button
                type="button"
                class="flex h-9 w-full items-center justify-between gap-2 rounded-md border border-border bg-secondary px-3 text-left text-sm text-foreground transition-colors hover:border-border-strong"
                @click="toggleTopicPicker"
              >
                <span class="truncate" :class="!selectedTopic && 'text-text-quaternary'">
                  {{ selectedTopic?.name || "无专栏" }}
                </span>
                <Icon icon="ri:arrow-down-s-line" class="shrink-0 text-text-quaternary" />
              </button>
              <div
                v-if="topicPickerOpen"
                class="absolute inset-x-0 top-full z-40 mt-1 overflow-hidden rounded-lg border border-border bg-card p-2 shadow-xl"
              >
                <div class="apple-search h-8">
                  <Icon icon="ri:search-line" class="text-text-quaternary" />
                  <input v-model="topicQuery" placeholder="搜索专栏名称" @click.stop />
                </div>
                <div class="mt-1 max-h-48 overflow-y-auto">
                  <button
                    type="button"
                    class="flex w-full items-center justify-between rounded-md px-2.5 py-2 text-left text-sm text-text-tertiary hover:bg-secondary hover:text-foreground"
                    @click="clearTopic"
                  >
                    <span>无专栏</span>
                    <Icon v-if="!selectedTopic" icon="ri:check-line" class="text-primary" />
                  </button>
                  <button
                    v-for="topic in filteredTopics"
                    :key="topic.id"
                    type="button"
                    class="flex w-full items-center justify-between rounded-md px-2.5 py-2 text-left text-sm text-text-tertiary hover:bg-secondary hover:text-foreground"
                    @click="selectTopic(topic)"
                  >
                    <span class="truncate">{{ topic.name }}</span>
                    <Icon v-if="selectedTopic?.id === topic.id" icon="ri:check-line" class="text-primary" />
                  </button>
                  <p v-if="filteredTopics.length === 0" class="px-2.5 py-3 text-xs text-text-quaternary">
                    暂无匹配专栏
                  </p>
                </div>
              </div>
            </div>
          </div>
          <div>
            <label :class="labelCls">标签（逗号分隔）</label>
            <input v-model="form.tags" :class="fieldCls" />
            <div v-if="tags.length" class="mt-2 flex flex-wrap gap-1.5">
              <button
                v-for="t in tags"
                :key="t"
                type="button"
                :class="[
                  chipCls,
                  form.tags
                    .split(/[,，]/)
                    .map((s) => s.trim())
                    .includes(t)
                    ? chipActiveCls
                    : chipIdleCls,
                ]"
                @click="pickTag(t)"
              >
                {{ t }}
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- 卡片 3：封面设置 -->
      <div :class="cardCls">
        <div :class="cardHeaderCls">封面设置</div>
        <div :class="cardBodyCls + ' space-y-3'">
          <!-- 封面预览 16:9 深色渐变 -->
          <div
            class="cover-preview-blue relative flex aspect-video w-full flex-col items-center justify-center gap-2 overflow-hidden rounded-[10px] border border-border"
          >
            <div
              class="pointer-events-none absolute inset-0"
              style="
                background:
                  radial-gradient(
                    circle at 30% 30%,
                    color-mix(in srgb, var(--color-primary) 15%, transparent) 0%,
                    transparent 50%
                  ),
                  radial-gradient(
                    circle at 70% 70%,
                    color-mix(in srgb, var(--color-chart-5) 12%, transparent) 0%,
                    transparent 50%
                  );
              "
            ></div>
            <template v-if="form.cover">
              <img :src="form.cover" alt="封面预览" class="relative z-10 h-full w-full object-cover" />
            </template>
            <template v-else>
              <Icon icon="ri:image-line" class="relative z-10 text-[36px] text-text-tertiary" />
              <div class="relative z-10 text-[12px] text-text-tertiary">点击上传封面图片</div>
            </template>
          </div>
          <div class="flex gap-2">
            <button
              type="button"
              class="inline-flex h-8 flex-1 items-center justify-center gap-1.5 rounded-lg border border-border bg-secondary px-3 text-[12.5px] font-medium text-foreground transition-colors hover:bg-accent"
              @click="emit('openCoverPicker')"
            >
              <Icon icon="ri:image-upload-line" class="text-[13px]" />
              从附件库选择
            </button>
            <button
              type="button"
              class="inline-flex h-8 items-center justify-center rounded-lg border border-border bg-secondary px-3 text-[12.5px] font-medium text-foreground transition-colors hover:bg-accent"
              @click="fetchRandomCover"
            >
              随机
            </button>
            <button
              v-if="form.cover"
              type="button"
              class="inline-flex h-8 items-center justify-center rounded-lg px-3 text-[12.5px] transition-colors"
              style="
                color: var(--color-state-error);
                border: 1px solid color-mix(in srgb, var(--color-state-error) 25%, transparent);
              "
              @click="form.cover = ''"
            >
              移除
            </button>
          </div>
          <div>
            <label :class="labelCls">封面 URL</label>
            <input v-model="form.cover" :class="fieldCls" />
          </div>
        </div>
      </div>

      <!-- 卡片 4：高级选项 -->
      <div :class="cardCls">
        <div :class="cardHeaderCls">高级选项</div>
        <div :class="cardBodyCls + ' space-y-3.5'">
          <div>
            <label :class="labelCls">摘要</label>
            <textarea v-model="form.excerpt" rows="3" :class="textareaCls + ' resize-y'"></textarea>
          </div>
          <!-- AI 文章摘要（Spec 53） -->
          <div>
            <label :class="labelCls">AI 摘要（前台导读）</label>
            <textarea
              :value="aiSummary"
              rows="4"
              :class="textareaCls + ' resize-y'"
              placeholder="后台预生成的导读摘要，前台文章顶部展示；可手动编辑或用 AI 生成"
              @input="emit('update:aiSummary', ($event.target as HTMLTextAreaElement).value)"
            ></textarea>
            <div class="mt-2 flex items-center gap-1.5">
              <button
                type="button"
                class="inline-flex h-7 items-center rounded-lg border border-border px-2.5 text-xs text-text-secondary transition-colors hover:bg-accent disabled:opacity-50"
                :disabled="aiGenerating"
                @click="generateAiSummary"
              >
                {{ aiGenerating ? "生成中…" : "AI 生成" }}
              </button>
              <button
                type="button"
                class="inline-flex h-7 items-center rounded-lg border border-border px-2.5 text-xs text-text-secondary transition-colors hover:bg-accent disabled:opacity-50"
                :disabled="aiSaving"
                @click="saveAiSummary"
              >
                {{ aiSaving ? "保存中…" : "保存摘要" }}
              </button>
              <button
                v-if="aiSummary"
                type="button"
                class="inline-flex h-7 items-center rounded-lg border border-border px-2.5 text-xs text-text-quaternary transition-colors hover:bg-accent"
                :disabled="aiSaving"
                @click="clearAiSummary"
              >
                清空
              </button>
            </div>
            <p class="mt-1 text-[11px] text-text-quaternary">
              AI 未配置时生成不可用（设置 → AI）；读者也可在前台用自己的 API 临时生成
            </p>
          </div>
          <!-- SEO -->
          <div>
            <label :class="labelCls">Meta Description</label>
            <textarea v-model="form.meta_description" rows="3" :class="textareaCls + ' resize-y'"></textarea>
            <p class="mt-1 text-[11px] text-text-quaternary">留空时依次回退：站点默认 description → 文章摘要</p>
          </div>
          <div>
            <label :class="labelCls">Meta Keywords（逗号分隔）</label>
            <input v-model="form.meta_keywords" :class="fieldCls" />
            <p class="mt-1 text-[11px] text-text-quaternary">留空时使用站点默认 keywords</p>
          </div>
        </div>
      </div>

      <!-- 标签插件参数面板（折叠为独立卡片） -->
      <div :class="cardCls">
        <div :class="cardHeaderCls">标签插件</div>
        <div :class="cardBodyCls + ' space-y-3'">
          <div class="flex flex-wrap gap-1.5">
            <button
              type="button"
              :class="[chipCls, pluginCat === '全部' ? chipActiveCls : chipIdleCls]"
              @click="pluginCat = '全部'"
            >
              全部
            </button>
            <button
              v-for="c in PLUGIN_CATEGORIES"
              :key="c"
              type="button"
              :class="[chipCls, pluginCat === c ? chipActiveCls : chipIdleCls]"
              @click="pluginCat = c"
            >
              {{ c }}
            </button>
          </div>
          <select
            :value="selectedPlugin?.name"
            :class="selectCls"
            @change="selectPlugin(runtimePlugins.find((p) => p.name === ($event.target as HTMLSelectElement).value)!)"
          >
            <option :value="undefined" disabled>选择插件…</option>
            <option v-for="p in pluginList" :key="p.name" :value="p.name">{{ p.title }}</option>
          </select>

          <div v-if="selectedPlugin" class="space-y-3">
            <PluginParamForm
              v-if="selectedPlugin.form"
              :plugin="selectedPlugin"
              :model-value="pluginCode"
              @update:model-value="onPluginCode"
            />
            <div v-else>
              <label :class="labelCls">示例（可编辑）</label>
              <textarea
                v-model="pluginCode"
                rows="4"
                spellcheck="false"
                :class="textareaCls + ' resize-y font-mono text-xs leading-relaxed'"
                @input="schedulePluginPreview()"
              ></textarea>
            </div>

            <div>
              <label :class="labelCls"
                >预览<span v-if="pluginRendering" class="ml-1 text-text-quaternary">渲染中…</span></label
              >
              <div class="overflow-hidden rounded-lg border border-border">
                <iframe
                  v-if="pluginPreviewHtml"
                  :srcdoc="pluginSrcdoc"
                  class="w-full border-0"
                  style="min-height: 200px; max-height: 400px; resize: vertical; overflow: auto"
                  sandbox="allow-same-origin allow-scripts"
                ></iframe>
                <p v-else class="px-3 py-4 text-center text-xs text-text-quaternary">填写参数后预览</p>
              </div>
            </div>

            <VButton type="secondary" block :disabled="!pluginCode" @click="insertPlugin">插入到正文</VButton>
          </div>
          <p v-else class="text-xs text-text-quaternary">选择插件后可编辑参数并插入正文。</p>
        </div>
      </div>
    </template>
  </div>
</template>
