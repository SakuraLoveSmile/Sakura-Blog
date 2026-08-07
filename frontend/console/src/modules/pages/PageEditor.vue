<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { pageApi } from "@/api";
import MarkdownEditor from "@/components/MarkdownEditor.vue";
import RichTextEditor from "@/components/RichTextEditor.vue";
import EditorToolbar from "../posts/EditorToolbar.vue";
import PageEditorSidebar from "./PageEditorSidebar.vue";
import CollapsibleSection from "./CollapsibleSection.vue";
import { Icon } from "@iconify/vue";
import { VPageHeader, VButton, Toast } from "@/components/ui";

// 页面编辑器：Halo 式三栏布局——顶部全宽工具栏 + 左侧主编辑区 + 右侧大纲/详情面板。
// 双编辑器模式：默认富文本所见即得（TipTap），可切 Markdown 分屏兑底；
// 两者绑定同一份 form.content_md，切换即天然同步。
const route = useRoute();
const router = useRouter();

const slugParam = computed(() => route.params.slug as string);
const isNew = computed(() => slugParam.value === "new");

// 页面级 widget 配置项：组件名 + 页面级参数覆盖（与全局 library 浅合并）
type SidebarWidget = { name: string; params: Record<string, any> };

// null = 未配置（继承全局 page 级默认）；[] = 显式空侧栏；[{name, params}, ...] = 自定义序列
type SidebarMode = "inherit" | "custom";
const form = ref<{
  slug: string;
  title: string;
  content_md: string;
  status: "draft" | "published";
  leftbar_widgets: SidebarWidget[] | null;
  rightbar_widgets: SidebarWidget[] | null;
}>({
  slug: "",
  title: "",
  content_md: "",
  status: "draft",
  leftbar_widgets: null,
  rightbar_widgets: null,
});

// 把后端返回的任意格式（旧 string[] / 新对象数组）归一化成 SidebarWidget[]
function normalizeWidgetList(raw: any): SidebarWidget[] | null {
  if (raw === undefined || raw === null) return null;
  if (!Array.isArray(raw)) return null;
  return raw.map((it: any) => {
    if (typeof it === "string") return { name: it, params: {} };
    return { name: String(it?.name ?? ""), params: it?.params && typeof it.params === "object" ? it.params : {} };
  });
}

// 可用 widget 库（组件名 → 中文名）。tree/tagtree/wiki_related 等特殊组件仅在对应页面类型有数据，
// 独立页面也可勾选（BuildBar 会兜底过滤掉 Data 为空的无效项）。
const WIDGET_LIBRARY: { name: string; label: string; hint?: string }[] = [
  { name: "related", label: "专栏系列", hint: "文章属专栏时显示" },
  { name: "recent", label: "近期发布" },
  { name: "tagcloud", label: "标签云" },
  { name: "popular", label: "热门文章" },
  { name: "welcome", label: "欢迎语" },
  { name: "author", label: "作者卡片" },
  { name: "siteinfo", label: "站点统计" },
  { name: "latest_comment", label: "最新评论" },
  { name: "toc", label: "文章目录", hint: "页面无标题时为空" },
  { name: "linklist", label: "自定义链接列表", hint: "需 library 配 items 参数" },
  { name: "timeline", label: "近期动态", hint: "友链 RSS 聚合，需配置" },
  { name: "ghrepo", label: "GitHub 仓库卡", hint: "需 library 配 repo" },
  { name: "ghuser", label: "GitHub 用户卡", hint: "需 library 配 username" },
  { name: "ghissues", label: "GitHub Issues", hint: "需 library 配 repo" },
  { name: "tree", label: "文库项目树", hint: "仅 wiki-doc 页面有数据" },
  { name: "tagtree", label: "笔记本标签树", hint: "仅笔记本页面有数据" },
  { name: "wiki_related", label: "文库相关项目", hint: "仅 wiki-doc 页面有数据" },
];
const WIDGET_OPTIONS = computed(() =>
  WIDGET_LIBRARY.map((w) => ({ label: `${w.label}（${w.name}）${w.hint ? " — " + w.hint : ""}`, value: w.name }))
);

// 从 SidebarWidget[] 提取纯 name 列表（用于 disabled 判断、预览等）
function widgetNames(list: SidebarWidget[] | null | undefined): string[] {
  return (list ?? []).map((w) => w.name);
}

const leftMode = computed<SidebarMode>({
  get: () => (form.value.leftbar_widgets === null ? "inherit" : "custom"),
  set: (m: SidebarMode) => {
    if (m === "inherit") form.value.leftbar_widgets = null;
    else if (form.value.leftbar_widgets === null) form.value.leftbar_widgets = [];
  },
});
const rightMode = computed<SidebarMode>({
  get: () => (form.value.rightbar_widgets === null ? "inherit" : "custom"),
  set: (m: SidebarMode) => {
    if (m === "inherit") form.value.rightbar_widgets = null;
    else if (form.value.rightbar_widgets === null) form.value.rightbar_widgets = [];
  },
});

function moveWidget(side: "left" | "right", idx: number, dir: -1 | 1) {
  const list = side === "left" ? form.value.leftbar_widgets! : form.value.rightbar_widgets!;
  const to = idx + dir;
  if (to < 0 || to >= list.length) return;
  [list[idx], list[to]] = [list[to], list[idx]];
}
function removeWidget(side: "left" | "right", idx: number) {
  const list = side === "left" ? form.value.leftbar_widgets! : form.value.rightbar_widgets!;
  list.splice(idx, 1);
  const key = `${side}:${idx}`;
  if (expandedWidget.value === key) expandedWidget.value = null;
}
function addWidget(side: "left" | "right", name: string) {
  const list = side === "left" ? form.value.leftbar_widgets! : form.value.rightbar_widgets!;
  if (widgetNames(list).includes(name)) return;
  list.push({ name, params: {} });
}

// 展开的 widget key："left:0" / "right:1"，null = 无展开
const expandedWidget = ref<string | null>(null);
function toggleExpand(side: "left" | "right", idx: number) {
  const key = `${side}:${idx}`;
  expandedWidget.value = expandedWidget.value === key ? null : key;
}

// ---------------- 参数表单（对齐 Settings → WidgetsTab 的 knownFields） ----------------
type FieldDef = { key: string; label: string; type: "string" | "number" | "boolean" | "textarea"; placeholder?: string };
const KNOWN_WIDGET_FIELDS: Record<string, FieldDef[]> = {
  toc: [
    { key: "title", label: "标题", type: "string", placeholder: "默认为空（不显示 header 标题）" },
    { key: "list_number", label: "显示序号", type: "boolean" },
    { key: "min_depth", label: "最小层级（1-6）", type: "number" },
    { key: "max_depth", label: "最大层级（1-6）", type: "number" },
    { key: "collapse", label: "折叠策略", type: "string", placeholder: "false / true / auto" },
  ],
  recent: [
    { key: "limit", label: "显示条数", type: "number" },
    { key: "title", label: "标题", type: "string" },
    { key: "rss", label: "RSS 链接（留空=不显示图标）", type: "string" },
  ],
  popular: [
    { key: "limit", label: "显示条数", type: "number" },
    { key: "title", label: "标题", type: "string" },
  ],
  tagcloud: [
    { key: "title", label: "标题", type: "string" },
    { key: "amount", label: "最多显示标签数（≤0=全部）", type: "number" },
    { key: "min_font", label: "最小字号（px）", type: "number" },
    { key: "max_font", label: "最大字号（px）", type: "number" },
    { key: "orderby", label: "排序维度：count/name/random", type: "string" },
    { key: "order", label: "顺序：1=升 / -1=降", type: "number" },
    { key: "show_count", label: "显示计数角标", type: "boolean" },
    { key: "color", label: "按热度渐变色", type: "boolean" },
  ],
  latest_comment: [
    { key: "limit", label: "显示条数", type: "number" },
    { key: "title", label: "标题", type: "string" },
  ],
  welcome: [
    { key: "title", label: "欢迎语标题", type: "string" },
    { key: "content", label: "欢迎语正文（Markdown）", type: "textarea" },
    { key: "columns", label: "链接列数（linklist 兼容）", type: "number" },
  ],
  author: [
    { key: "title", label: "标题", type: "string" },
  ],
  siteinfo: [
    { key: "title", label: "标题", type: "string" },
  ],
  related: [
    { key: "title", label: "标题", type: "string" },
    { key: "limit", label: "显示条数", type: "number" },
  ],
  ghrepo: [
    { key: "repo", label: "仓库（owner/name）", type: "string" },
    { key: "title", label: "卡片标题", type: "string" },
  ],
  ghuser: [
    { key: "username", label: "GitHub 用户名", type: "string" },
    { key: "avatar", label: "显示头像", type: "boolean" },
  ],
  ghissues: [
    { key: "repo", label: "仓库（owner/name）", type: "string" },
    { key: "limit", label: "显示条数", type: "number" },
    { key: "title", label: "标题", type: "string" },
    { key: "labels", label: "标签过滤（逗号分隔）", type: "string" },
  ],
  timeline: [
    { key: "title", label: "标题", type: "string" },
    { key: "api", label: "JSON Feed 地址", type: "string" },
    { key: "user", label: "用户名（部分源需要）", type: "string" },
    { key: "limit", label: "显示条数", type: "number" },
    { key: "show_title", label: "显示条目标题", type: "boolean" },
    { key: "show_content", label: "显示条目正文", type: "boolean" },
  ],
  linklist: [
    { key: "title", label: "标题", type: "string" },
    { key: "columns", label: "列数（1=图标+文字竖排；>=3=多列纯图标）", type: "number" },
  ],
};

// 获取某个 SidebarWidget 的 params（保证引用可变可写）
function ensureParams(side: "left" | "right", idx: number): Record<string, any> {
  const list = side === "left" ? form.value.leftbar_widgets! : form.value.rightbar_widgets!;
  if (!list[idx].params) list[idx].params = {};
  return list[idx].params;
}
// 读取单个字段值，类型不安全但足够用（输入控件本身会类型转换）
function getField(p: Record<string, any>, f: FieldDef): any {
  const v = p[f.key];
  if (v === undefined || v === null) {
    if (f.type === "boolean") return false;
    if (f.type === "number") return "";
    return "";
  }
  return v;
}
function setField(p: Record<string, any>, f: FieldDef, value: any) {
  if (f.type === "number") {
    const n = Number(value);
    if (value === "" || Number.isNaN(n)) {
      delete p[f.key];
    } else {
      p[f.key] = n;
    }
  } else if (f.type === "boolean") {
    if (!value) {
      delete p[f.key];
    } else {
      p[f.key] = true;
    }
  } else {
    if (value === "" || value === undefined || value === null) {
      delete p[f.key];
    } else {
      p[f.key] = value;
    }
  }
}
// JSON 兜底：未知组件的原始 JSON 编辑
function jsonOfParams(p: Record<string, any>): string {
  if (!p || Object.keys(p).length === 0) return "";
  try {
    return JSON.stringify(p, null, 2);
  } catch {
    return "";
  }
}
function applyParamsFromJson(p: Record<string, any>, raw: string): string {
  raw = raw.trim();
  if (raw === "") {
    for (const k of Object.keys(p)) delete p[k];
    return "";
  }
  try {
    const obj = JSON.parse(raw);
    if (obj && typeof obj === "object" && !Array.isArray(obj)) {
      for (const k of Object.keys(p)) delete p[k];
      for (const [k, v] of Object.entries(obj)) p[k] = v;
      return "";
    }
    return "必须是 JSON 对象 {}";
  } catch (e: any) {
    return "JSON 解析失败：" + e.message;
  }
}

const loading = ref(false);
const saving = ref(false);
const editorRef = ref<InstanceType<typeof MarkdownEditor> | null>(null);
const richRef = ref<InstanceType<typeof RichTextEditor> | null>(null);

// 编辑器种类：富文本（默认）/ Markdown 分屏，选择持久化
const KIND_KEY = "page-editor:kind";
type EditorKind = "rich" | "markdown";
function loadKind(): EditorKind {
  return localStorage.getItem(KIND_KEY) === "markdown" ? "markdown" : "rich";
}
const editorKind = ref<EditorKind>(loadKind());
function onEditorKindSelect(e: Event) {
  const v = (e.target as HTMLSelectElement).value as EditorKind;
  editorKind.value = v;
  localStorage.setItem(KIND_KEY, v);
}

// 全屏模式：覆盖侧边栏，编辑器占满视口
const fullscreen = ref(false);
function toggleFullscreen() {
  fullscreen.value = !fullscreen.value;
}

// 右侧面板开关，状态持久化
const SIDEBAR_KEY = "page-editor:sidebar-open";
const sidebarOpen = ref(localStorage.getItem(SIDEBAR_KEY) !== "0");
function toggleSidebar() {
  sidebarOpen.value = !sidebarOpen.value;
  localStorage.setItem(SIDEBAR_KEY, sidebarOpen.value ? "1" : "0");
}

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
  editorKind.value === "rich" ? (richRef.value?.activeStates ?? {}) : {}
);

// 侧边栏配置面板（从底部移入可折叠区，右侧面板有按钮跳转过来）
const configSectionOpen = ref(false);
function scrollToConfig() {
  configSectionOpen.value = true;
  // 下一帧滚动到底部配置区
  requestAnimationFrame(() => {
    document.getElementById("page-sidebar-config")?.scrollIntoView({ behavior: "smooth", block: "start" });
  });
}

// Markdown 模式的 format(id) 映射表（与 EditorToolbar formatTools id 一致）
const tableTemplate = "\n| 表头 | 表头 |\n| --- | --- |\n| 内容 | 内容 |\n";
const mdFormatMap: Record<string, () => void> = {
  bold: () => editorRef.value?.wrapSelection("**", "**", "加粗文本"),
  italic: () => editorRef.value?.wrapSelection("*", "*", "斜体文本"),
  strike: () => editorRef.value?.wrapSelection("~~", "~~", "删除线"),
  heading: () => editorRef.value?.prefixLines("## ", false),
  link: () => editorRef.value?.wrapSelection("[", "](url)", "链接文字"),
  quote: () => editorRef.value?.prefixLines("> ", false),
  code: () => editorRef.value?.wrapSelection("`", "`", "code"),
  codeblock: () => editorRef.value?.wrapSelection("\n```\n", "\n```\n", "code"),
  ul: () => editorRef.value?.prefixLines("- ", false),
  ol: () => editorRef.value?.prefixLines("", true),
  table: () => editorRef.value?.insertBlock(tableTemplate),
};

// 工具栏事件：format → 富文本/Markdown 分别路由
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
// 大纲点击跳转：富文本按标题序号定位节点，Markdown 按行号滚动
function onJump(line: number, index: number) {
  if (editorKind.value === "rich") {
    richRef.value?.scrollToHeading(index);
  } else {
    editorRef.value?.scrollToLine(line);
  }
}

// 选择器：每侧的「待添加 widget」下拉框
const newLeftWidget = ref("");
const newRightWidget = ref("");

onMounted(async () => {
  if (isNew.value) return;
  loading.value = true;
  try {
    const { data } = await pageApi.get(slugParam.value);
    form.value = {
      slug: data.slug,
      title: data.title,
      content_md: data.content_md,
      status: (data.status as any) || "draft",
      // API 返回：undefined/missing = null（未配置）；[] = 显式空；[...] = 自定义（含旧 string[] / 新对象数组两种）
      leftbar_widgets: normalizeWidgetList((data as any).leftbar_widgets),
      rightbar_widgets: normalizeWidgetList((data as any).rightbar_widgets),
    };
  } catch {
    Toast.error("加载失败");
  } finally {
    loading.value = false;
  }
});

async function handleSave() {
  saving.value = true;
  try {
    // 提交时：null = 不覆盖（沿用现有/空）；非 null = 覆盖（含空数组显式清空）
    const payload: any = { ...form.value };
    if (isNew.value) {
      await pageApi.create(payload);
    } else {
      await pageApi.update(slugParam.value, payload);
    }
    editorRef.value?.clearDraft();
    Toast.success("保存成功");
    router.push("/pages");
  } catch (e: any) {
    Toast.error(e.response?.data?.error || "保存失败");
  } finally {
    saving.value = false;
  }
}

// 全局 widgets.pages.page 默认值（对齐 Go 侧 DefaultPageBars）。
const GLOBAL_DEFAULT_PAGE_BARS = {
  left: ["related", "recent", "tagcloud", "popular"],
  right: ["welcome", "author", "siteinfo", "latest_comment", "toc"],
};
function widgetLabelOf(name: string) {
  return WIDGET_LIBRARY.find((w) => w.name === name)?.label ?? name;
}

// 预览渲染的「最终组件序列」
const resolvedLeftbar = computed<string[]>(
  () => leftMode.value === "inherit" ? GLOBAL_DEFAULT_PAGE_BARS.left : widgetNames(form.value.leftbar_widgets)
);
const resolvedRightbar = computed<string[]>(
  () => rightMode.value === "inherit" ? GLOBAL_DEFAULT_PAGE_BARS.right : widgetNames(form.value.rightbar_widgets)
);

// 取预览里显示的「关键参数摘要」行（2 行以内，避免过高）
function widgetSummaryLines(name: string, side?: "left" | "right", idx?: number): string[] {
  let params: Record<string, any> | null = null;
  if (side && idx !== undefined) {
    const list = side === "left" ? form.value.leftbar_widgets : form.value.rightbar_widgets;
    if (list && list[idx]) params = list[idx].params ?? {};
  }
  const keys = KNOWN_WIDGET_FIELDS[name]?.slice(0, 2).map((f) => f.key) ?? [];
  const out: string[] = [];
  for (const k of keys) {
    const v = params ? params[k] : undefined;
    if (v !== undefined && v !== null && v !== "" && v !== false) {
      out.push(`${k}: ${String(v).length > 24 ? String(v).slice(0, 24) + "…" : v}`);
    }
  }
  return out;
}

// 预览：先保存 → 新开标签打开前台 /{slug} 看真实效果
const previewing = ref(false);
async function handlePreview() {
  if (!form.value.slug.trim()) {
    Toast.error("请先填写 Slug（预览需要可访问的页面路径）");
    return;
  }
  previewing.value = true;
  try {
    const payload: any = { ...form.value };
    if (isNew.value) {
      await pageApi.create(payload);
    } else {
      await pageApi.update(slugParam.value, payload);
    }
    editorRef.value?.clearDraft();
    Toast.success("已保存，即将打开前台预览");
    window.open("/" + form.value.slug.trim(), "_blank", "noopener");
  } catch (e: any) {
    Toast.error(e.response?.data?.error || "保存失败");
  } finally {
    previewing.value = false;
  }
}

// ------- 本地：JSON 编辑态的文本/错误 -------
const jsonEditorText = ref<string>("");
const jsonEditorError = ref<string>("");
function openJsonEditor(p: Record<string, any>) {
  jsonEditorText.value = jsonOfParams(p);
  jsonEditorError.value = "";
}
function handleApplyJson(p: Record<string, any>) {
  jsonEditorError.value = applyParamsFromJson(p, jsonEditorText.value);
}

const inputCls =
  "w-full rounded-base border border-gray-200 px-3 py-2 text-sm outline-none transition-colors focus:border-primary dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500";
const smallInputCls =
  "w-full rounded-base border border-gray-200 px-2 py-1 text-xs outline-none transition-colors focus:border-primary dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100 dark:placeholder-gray-500";
</script>

<template>
  <div :class="{ 'fixed inset-0 z-[9999] overflow-y-auto bg-gray-50 dark:bg-gray-900': fullscreen }">
    <VPageHeader :title="isNew ? '新建页面' : '编辑页面'">
      <template #icon>
        <Icon icon="ri:pages-line" class="mr-2 self-center text-xl" />
      </template>
      <template #actions>
        <div class="flex items-center gap-2">
          <select
            :value="editorKind"
            class="rounded-base border border-gray-200 bg-white px-2 py-1.5 text-xs text-gray-700 outline-none transition-colors dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200"
            @change="onEditorKindSelect"
            title="编辑器模式"
          >
            <option value="rich">富文本</option>
            <option value="markdown">Markdown</option>
          </select>
          <select
            v-if="editorKind === 'markdown'"
            :value="editorMode"
            class="rounded-base border border-gray-200 bg-white px-2 py-1.5 text-xs text-gray-700 outline-none transition-colors dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200"
            @change="onEditorModeSelect"
            title="显示模式"
          >
            <option v-for="it in editorModeItems" :key="it.value" :value="it.value">{{ it.label }}</option>
          </select>
          <VButton size="sm" :title="fullscreen ? '退出全屏' : '全屏编辑'" @click="toggleFullscreen">
            <template #icon>
              <Icon :icon="fullscreen ? 'ri:fullscreen-exit-line' : 'ri:fullscreen-line'" class="h-full w-full" />
            </template>
          </VButton>
          <VButton @click="router.push('/pages')">取消</VButton>
          <VButton :loading="previewing" @click="handlePreview">
            <template #icon>
              <Icon icon="ri:external-link-line" class="h-full w-full" />
            </template>
            预览
          </VButton>
          <VButton type="secondary" :loading="saving" @click="handleSave">保存</VButton>
        </div>
      </template>
    </VPageHeader>

    <VLoading v-if="loading" />

    <div v-else class="m-0 space-y-4 md:m-4">
      <!-- 顶部全宽工具栏（Halo 式，与文章编辑器一致） -->
      <EditorToolbar
        :kind="editorKind"
        :mode="editorMode"
        :sidebar-open="sidebarOpen"
        :is-edit="!isNew"
        :fullscreen="fullscreen"
        :active-states="toolbarActive"
        @format="onFormat"
        @insert="onInsert"
        @upload="onUpload"
        @pick="onPick"
        @update:mode="setEditorMode"
        @toggle-sidebar="toggleSidebar"
        @open-ai="Toast.info('AI 功能仅在文章编辑器中可用')"
        @preview="handlePreview"
        @toggle-fullscreen="toggleFullscreen"
        @open-revision="Toast.info('版本历史仅在文章编辑器中可用')"
      />

      <!-- 主编辑区 + 右侧面板 -->
      <div class="flex flex-col gap-4 lg:flex-row lg:items-start">
        <div class="min-w-0 flex-1 space-y-4">
          <input
            v-model="form.title"
            placeholder="页面标题"
            class="w-full px-4 py-3 text-lg font-medium border border-gray-200 rounded-xl focus:outline-none focus:border-primary transition-colors dark:bg-gray-800 dark:border-gray-600 dark:text-gray-100 dark:placeholder-gray-500"
          />
          <!-- 富文本（默认）/ Markdown 分屏，绑定同一份 content_md，切换即同步 -->
          <RichTextEditor
            v-if="editorKind === 'rich'"
            ref="richRef"
            v-model="form.content_md"
            :storage-key="'page:' + (isNew ? 'new' : slugParam)"
            @save="handleSave"
          />
          <MarkdownEditor
            v-else
            ref="editorRef"
            v-model="form.content_md"
            :toolbar="false"
            :storage-key="'page:' + (isNew ? 'new' : slugParam)"
            @save="handleSave"
            @mode-change="(m: any) => (editorMode = m)"
          />
        </div>

        <!-- 右侧大纲/详情面板（工具栏按钮控制收起） -->
        <div
          v-show="sidebarOpen"
          class="w-full shrink-0 lg:sticky lg:top-4 lg:max-h-[calc(100vh-2rem)] lg:w-80 xl:w-96"
        >
          <PageEditorSidebar
            :form="form"
            :leftbar-widgets="form.leftbar_widgets"
            :rightbar-widgets="form.rightbar_widgets"
            :widget-library="WIDGET_LIBRARY"
            :known-widget-fields="KNOWN_WIDGET_FIELDS"
            @jump="onJump"
            @open-sidebar-config="scrollToConfig"
          />
        </div>
      </div>

      <!-- 基本信息 + 侧边栏完整配置（放在底部，右侧面板可跳过来） -->
      <div id="page-sidebar-config" class="space-y-4">
        <CollapsibleSection title="基本信息" subtitle="标题、Slug、发布状态" :default-open="!isNew" :key="'basic:' + (isNew ? 'new' : 'edit')">
          <div class="grid grid-cols-2 gap-4">
            <div>
              <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">标题</label>
              <input v-model="form.title" :class="inputCls" />
            </div>
            <div>
              <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">Slug</label>
              <input v-model="form.slug" :class="inputCls" />
            </div>
          </div>
          <div class="mt-4">
            <label class="mb-1 block text-xs text-gray-500 dark:text-gray-400">状态</label>
            <select
              v-model="form.status"
              class="w-40 rounded-base border border-gray-200 px-3 py-2 text-sm outline-none transition-colors focus:border-primary dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
            >
              <option value="draft">草稿</option>
              <option value="published">发布</option>
            </select>
          </div>
        </CollapsibleSection>

        <CollapsibleSection
          title="侧边栏配置"
          subtitle="组件序列 + 参数编辑（页面级覆盖，不影响全局 library）"
          :default-open="configSectionOpen"
          :key="'sidebar-config:' + (configSectionOpen ? 'open' : 'close')"
        >
          <div class="grid grid-cols-1 gap-6">
            <!-- 左栏 -->
            <div>
              <div class="mb-2 flex items-center justify-between">
                <label class="text-sm font-semibold text-gray-800 dark:text-gray-200">左栏</label>
                <select
                  v-model="leftMode"
                  class="rounded-base border border-gray-200 px-2 py-1 text-xs outline-none transition-colors focus:border-primary dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                >
                  <option value="inherit">继承全局</option>
                  <option value="custom">自定义</option>
                </select>
              </div>
              <div v-if="leftMode === 'custom'" class="space-y-2">
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  空列表 = 左栏为空（不渲染任何组件）；点击列表项展开参数编辑：
                </p>
                <div class="flex gap-2">
                  <select
                    v-model="newLeftWidget"
                    class="w-0 min-w-0 flex-1 rounded-base border border-gray-200 px-2 py-1.5 text-sm outline-none transition-colors focus:border-primary dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                  >
                    <option value="">+ 添加组件…</option>
                    <option
                      v-for="o in WIDGET_OPTIONS"
                      :key="o.value"
                      :value="o.value"
                      :disabled="widgetNames(form.leftbar_widgets).includes(o.value)"
                    >
                      {{ o.label }}
                    </option>
                  </select>
                  <VButton
                    size="sm"
                    :disabled="!newLeftWidget"
                    @click="addWidget('left', newLeftWidget); newLeftWidget = ''"
                  >
                    添加
                  </VButton>
                </div>
                <div class="overflow-hidden rounded-base border border-gray-200 dark:border-gray-700">
                  <div
                    v-if="form.leftbar_widgets!.length === 0"
                    class="px-3 py-4 text-center text-xs text-gray-400"
                  >
                    （无组件 — 将渲染空左栏）
                  </div>
                  <div
                    v-for="(w, idx) in form.leftbar_widgets"
                    :key="w.name"
                    class="border-b last:border-b-0 dark:border-gray-700"
                  >
                    <div
                      class="flex cursor-pointer items-center justify-between gap-2 px-2 py-1.5 text-sm hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-800/50"
                      @click="toggleExpand('left', idx)"
                    >
                      <span class="flex min-w-0 items-center gap-1 truncate">
                        <Icon
                          :icon="expandedWidget === 'left:' + idx ? 'ri:arrow-down-s-line' : 'ri:arrow-right-s-line'"
                          class="h-4 w-4 shrink-0 text-gray-400"
                        />
                        <span class="text-[10px] text-gray-400">#{{ idx + 1 }}</span>
                        <span class="truncate">
                          {{ widgetLabelOf(w.name) }}
                          <span class="text-xs text-gray-400">（{{ w.name }}）</span>
                        </span>
                        <span
                          v-if="w.params && Object.keys(w.params).length > 0"
                          class="shrink-0 rounded-full bg-primary/10 px-1.5 py-0.5 text-[10px] text-primary"
                        >
                          {{ Object.keys(w.params).length }} 参数
                        </span>
                      </span>
                      <div class="flex shrink-0 items-center gap-1" @click.stop>
                        <VButton
                          size="sm"
                          class="!px-2"
                          :disabled="idx === 0"
                          @click="moveWidget('left', idx, -1)"
                          title="上移"
                        >
                          <Icon icon="ri:arrow-up-line" class="h-4 w-4" />
                        </VButton>
                        <VButton
                          size="sm"
                          class="!px-2"
                          :disabled="idx === form.leftbar_widgets!.length - 1"
                          @click="moveWidget('left', idx, 1)"
                          title="下移"
                        >
                          <Icon icon="ri:arrow-down-line" class="h-4 w-4" />
                        </VButton>
                        <VButton size="sm" class="!px-2 !text-red-600" @click="removeWidget('left', idx)" title="移除">
                          <Icon icon="ri:delete-bin-line" class="h-4 w-4" />
                        </VButton>
                      </div>
                    </div>
                    <div
                      v-if="expandedWidget === 'left:' + idx"
                      class="space-y-2 border-t border-dashed bg-gray-50 px-3 py-2 text-xs dark:border-gray-700 dark:bg-gray-800/30"
                    >
                      <template v-if="KNOWN_WIDGET_FIELDS[w.name]">
                        <div
                          v-for="f in KNOWN_WIDGET_FIELDS[w.name]"
                          :key="f.key"
                          class="grid grid-cols-1 gap-1 sm:grid-cols-[120px_1fr] sm:items-start"
                        >
                          <label class="pt-1 text-[11px] font-medium text-gray-600 dark:text-gray-400">{{ f.label }}</label>
                          <template v-if="f.type === 'boolean'">
                            <label class="flex cursor-pointer items-center gap-1.5 py-0.5">
                              <input
                                type="checkbox"
                                :checked="getField(ensureParams('left', idx), f)"
                                @change="setField(ensureParams('left', idx), f, ($event.target as any).checked)"
                                class="h-3.5 w-3.5 cursor-pointer rounded border border-gray-300 accent-primary dark:border-gray-600"
                              />
                              <span class="text-[11px] text-gray-500 dark:text-gray-400">
                                启用（关闭=走全局 library 默认）
                              </span>
                            </label>
                          </template>
                          <template v-else-if="f.type === 'textarea'">
                            <textarea
                              rows="4"
                              :placeholder="f.placeholder || ''"
                              :value="getField(ensureParams('left', idx), f)"
                              @input="setField(ensureParams('left', idx), f, ($event.target as any).value)"
                              :class="smallInputCls + ' font-mono'"
                            ></textarea>
                          </template>
                          <template v-else>
                            <input
                              :type="f.type === 'number' ? 'number' : 'text'"
                              :placeholder="f.placeholder || ''"
                              :value="getField(ensureParams('left', idx), f)"
                              @input="setField(ensureParams('left', idx), f, ($event.target as any).value)"
                              :class="smallInputCls"
                            />
                          </template>
                        </div>
                      </template>
                      <template v-else>
                        <p class="text-[11px] text-gray-500 dark:text-gray-400">
                          该组件暂无结构化表单，可直接编辑 Params JSON：
                        </p>
                        <textarea
                          rows="6"
                          :class="smallInputCls + ' font-mono'"
                          placeholder='{"layout":"toc","title":"自定义标题"}'
                          @focus="openJsonEditor(ensureParams('left', idx))"
                          v-model="jsonEditorText"
                        ></textarea>
                        <div class="flex items-end justify-between gap-2">
                          <span class="text-[11px] text-red-500">{{ jsonEditorError }}</span>
                          <VButton size="sm" @click="handleApplyJson(ensureParams('left', idx))">应用 JSON</VButton>
                        </div>
                      </template>
                    </div>
                  </div>
                </div>
              </div>
              <div v-else class="rounded-base border border-dashed border-gray-300 px-3 py-4 text-center text-xs text-gray-500 dark:border-gray-700 dark:text-gray-400">
                使用全局 widgets.pages.page 左栏配置（related → recent → tagcloud → popular）
              </div>
            </div>

            <!-- 右栏 -->
            <div>
              <div class="mb-2 flex items-center justify-between">
                <label class="text-sm font-semibold text-gray-800 dark:text-gray-200">右栏</label>
                <select
                  v-model="rightMode"
                  class="rounded-base border border-gray-200 px-2 py-1 text-xs outline-none transition-colors focus:border-primary dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                >
                  <option value="inherit">继承全局</option>
                  <option value="custom">自定义</option>
                </select>
              </div>
              <div v-if="rightMode === 'custom'" class="space-y-2">
                <p class="text-xs text-gray-500 dark:text-gray-400">
                  空列表 = 右栏为空；点击列表项展开参数编辑：
                </p>
                <div class="flex gap-2">
                  <select
                    v-model="newRightWidget"
                    class="w-0 min-w-0 flex-1 rounded-base border border-gray-200 px-2 py-1.5 text-sm outline-none transition-colors focus:border-primary dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100"
                  >
                    <option value="">+ 添加组件…</option>
                    <option
                      v-for="o in WIDGET_OPTIONS"
                      :key="o.value"
                      :value="o.value"
                      :disabled="widgetNames(form.rightbar_widgets).includes(o.value)"
                    >
                      {{ o.label }}
                    </option>
                  </select>
                  <VButton
                    size="sm"
                    :disabled="!newRightWidget"
                    @click="addWidget('right', newRightWidget); newRightWidget = ''"
                  >
                    添加
                  </VButton>
                </div>
                <div class="overflow-hidden rounded-base border border-gray-200 dark:border-gray-700">
                  <div
                    v-if="form.rightbar_widgets!.length === 0"
                    class="px-3 py-4 text-center text-xs text-gray-400"
                  >
                    （无组件 — 将渲染空右栏）
                  </div>
                  <div
                    v-for="(w, idx) in form.rightbar_widgets"
                    :key="w.name"
                    class="border-b last:border-b-0 dark:border-gray-700"
                  >
                    <div
                      class="flex cursor-pointer items-center justify-between gap-2 px-2 py-1.5 text-sm hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-800/50"
                      @click="toggleExpand('right', idx)"
                    >
                      <span class="flex min-w-0 items-center gap-1 truncate">
                        <Icon
                          :icon="expandedWidget === 'right:' + idx ? 'ri:arrow-down-s-line' : 'ri:arrow-right-s-line'"
                          class="h-4 w-4 shrink-0 text-gray-400"
                        />
                        <span class="text-[10px] text-gray-400">#{{ idx + 1 }}</span>
                        <span class="truncate">
                          {{ widgetLabelOf(w.name) }}
                          <span class="text-xs text-gray-400">（{{ w.name }}）</span>
                        </span>
                        <span
                          v-if="w.params && Object.keys(w.params).length > 0"
                          class="shrink-0 rounded-full bg-primary/10 px-1.5 py-0.5 text-[10px] text-primary"
                        >
                          {{ Object.keys(w.params).length }} 参数
                        </span>
                      </span>
                      <div class="flex shrink-0 items-center gap-1" @click.stop>
                        <VButton
                          size="sm"
                          class="!px-2"
                          :disabled="idx === 0"
                          @click="moveWidget('right', idx, -1)"
                          title="上移"
                        >
                          <Icon icon="ri:arrow-up-line" class="h-4 w-4" />
                        </VButton>
                        <VButton
                          size="sm"
                          class="!px-2"
                          :disabled="idx === form.rightbar_widgets!.length - 1"
                          @click="moveWidget('right', idx, 1)"
                          title="下移"
                        >
                          <Icon icon="ri:arrow-down-line" class="h-4 w-4" />
                        </VButton>
                        <VButton size="sm" class="!px-2 !text-red-600" @click="removeWidget('right', idx)" title="移除">
                          <Icon icon="ri:delete-bin-line" class="h-4 w-4" />
                        </VButton>
                      </div>
                    </div>
                    <div
                      v-if="expandedWidget === 'right:' + idx"
                      class="space-y-2 border-t border-dashed bg-gray-50 px-3 py-2 text-xs dark:border-gray-700 dark:bg-gray-800/30"
                    >
                      <template v-if="KNOWN_WIDGET_FIELDS[w.name]">
                        <div
                          v-for="f in KNOWN_WIDGET_FIELDS[w.name]"
                          :key="f.key"
                          class="grid grid-cols-1 gap-1 sm:grid-cols-[120px_1fr] sm:items-start"
                        >
                          <label class="pt-1 text-[11px] font-medium text-gray-600 dark:text-gray-400">{{ f.label }}</label>
                          <template v-if="f.type === 'boolean'">
                            <label class="flex cursor-pointer items-center gap-1.5 py-0.5">
                              <input
                                type="checkbox"
                                :checked="getField(ensureParams('right', idx), f)"
                                @change="setField(ensureParams('right', idx), f, ($event.target as any).checked)"
                                class="h-3.5 w-3.5 cursor-pointer rounded border border-gray-300 accent-primary dark:border-gray-600"
                              />
                              <span class="text-[11px] text-gray-500 dark:text-gray-400">
                                启用（关闭=走全局 library 默认）
                              </span>
                            </label>
                          </template>
                          <template v-else-if="f.type === 'textarea'">
                            <textarea
                              rows="4"
                              :placeholder="f.placeholder || ''"
                              :value="getField(ensureParams('right', idx), f)"
                              @input="setField(ensureParams('right', idx), f, ($event.target as any).value)"
                              :class="smallInputCls + ' font-mono'"
                            ></textarea>
                          </template>
                          <template v-else>
                            <input
                              :type="f.type === 'number' ? 'number' : 'text'"
                              :placeholder="f.placeholder || ''"
                              :value="getField(ensureParams('right', idx), f)"
                              @input="setField(ensureParams('right', idx), f, ($event.target as any).value)"
                              :class="smallInputCls"
                            />
                          </template>
                        </div>
                      </template>
                      <template v-else>
                        <p class="text-[11px] text-gray-500 dark:text-gray-400">
                          该组件暂无结构化表单，可直接编辑 Params JSON：
                        </p>
                        <textarea
                          rows="6"
                          :class="smallInputCls + ' font-mono'"
                          placeholder='{"layout":"toc","title":"自定义标题"}'
                          @focus="openJsonEditor(ensureParams('right', idx))"
                          v-model="jsonEditorText"
                        ></textarea>
                        <div class="flex items-end justify-between gap-2">
                          <span class="text-[11px] text-red-500">{{ jsonEditorError }}</span>
                          <VButton size="sm" @click="handleApplyJson(ensureParams('right', idx))">应用 JSON</VButton>
                        </div>
                      </template>
                    </div>
                  </div>
                </div>
              </div>
              <div v-else class="rounded-base border border-dashed border-gray-300 px-3 py-4 text-center text-xs text-gray-500 dark:border-gray-700 dark:text-gray-400">
                使用全局 widgets.pages.page 右栏配置（welcome → author → siteinfo → latest_comment → toc）
              </div>
            </div>

            <!-- 实时缩略预览 -->
            <div>
              <div class="mb-2 flex items-baseline gap-2">
                <label class="text-sm font-semibold text-gray-800 dark:text-gray-200">实时缩略预览</label>
                <span class="text-[11px] text-gray-500 dark:text-gray-400">
                  左右栏宽度比例对齐前台；骨架区下会显示已设置的关键参数摘要
                </span>
              </div>
              <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 sm:justify-items-stretch md:grid-cols-[300px_280px] md:justify-start">
                <div class="w-full max-w-[320px] space-y-3 rounded-lg bg-gray-50 p-3 ring-1 ring-gray-200 dark:bg-gray-800/30 dark:ring-gray-700">
                  <div class="text-center text-[10px] font-medium uppercase tracking-[0.2em] text-gray-400 dark:text-gray-500">Leftbar</div>
                  <template v-if="resolvedLeftbar.length === 0">
                    <div class="rounded-base border border-dashed border-gray-300 p-4 text-center text-xs text-gray-400 dark:border-gray-700">
                      （空侧栏）
                    </div>
                  </template>
                  <div
                    v-for="(name, idx) in resolvedLeftbar"
                    :key="'L-' + name + '-' + idx"
                    class="overflow-hidden rounded-base ring-1 ring-gray-200 dark:ring-gray-700"
                  >
                    <div class="truncate bg-gray-200/70 px-2 py-1 text-xs text-gray-700 dark:bg-gray-700/60 dark:text-gray-200">
                      <span class="mr-1 inline-block h-2 w-2 rounded-full bg-primary/60 align-middle"></span>
                      {{ widgetLabelOf(name) }}
                      <span class="ml-1 text-[10px] font-normal opacity-60">({{ name }})</span>
                    </div>
                    <div class="space-y-1.5 bg-white p-2 dark:bg-gray-800">
                      <div class="h-2 animate-pulse rounded bg-gray-100 dark:bg-gray-700 w-full"></div>
                      <div class="h-2 animate-pulse rounded bg-gray-100 dark:bg-gray-700 w-4/5"></div>
                      <div class="h-2 animate-pulse rounded bg-gray-100 dark:bg-gray-700 w-3/5"></div>
                      <template v-if="leftMode === 'custom'">
                        <div
                          v-for="line in widgetSummaryLines(name, 'left', idx)"
                          :key="line"
                          class="mt-1 truncate border-t border-dashed border-gray-200 pt-1 text-[10px] font-mono text-primary/80 dark:border-gray-700 dark:text-primary/70"
                        >
                          {{ line }}
                        </div>
                      </template>
                    </div>
                  </div>
                </div>
                <div class="w-full max-w-[300px] space-y-3 rounded-lg bg-gray-50 p-3 ring-1 ring-gray-200 dark:bg-gray-800/30 dark:ring-gray-700">
                  <div class="text-center text-[10px] font-medium uppercase tracking-[0.2em] text-gray-400 dark:text-gray-500">Rightbar</div>
                  <template v-if="resolvedRightbar.length === 0">
                    <div class="rounded-base border border-dashed border-gray-300 p-4 text-center text-xs text-gray-400 dark:border-gray-700">
                      （空侧栏）
                    </div>
                  </template>
                  <div
                    v-for="(name, idx) in resolvedRightbar"
                    :key="'R-' + name + '-' + idx"
                    class="overflow-hidden rounded-base ring-1 ring-gray-200 dark:ring-gray-700"
                  >
                    <div class="truncate bg-gray-200/70 px-2 py-1 text-xs text-gray-700 dark:bg-gray-700/60 dark:text-gray-200">
                      <span class="mr-1 inline-block h-2 w-2 rounded-full bg-primary/60 align-middle"></span>
                      {{ widgetLabelOf(name) }}
                      <span class="ml-1 text-[10px] font-normal opacity-60">({{ name }})</span>
                    </div>
                    <div class="space-y-1.5 bg-white p-2 dark:bg-gray-800">
                      <div class="h-2 animate-pulse rounded bg-gray-100 dark:bg-gray-700 w-full"></div>
                      <div class="h-2 animate-pulse rounded bg-gray-100 dark:bg-gray-700 w-4/5"></div>
                      <div class="h-2 animate-pulse rounded bg-gray-100 dark:bg-gray-700 w-3/5"></div>
                      <template v-if="rightMode === 'custom'">
                        <div
                          v-for="line in widgetSummaryLines(name, 'right', idx)"
                          :key="line"
                          class="mt-1 truncate border-t border-dashed border-gray-200 pt-1 text-[10px] font-mono text-primary/80 dark:border-gray-700 dark:text-primary/70"
                        >
                          {{ line }}
                        </div>
                      </template>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </CollapsibleSection>
      </div>
    </div>
  </div>
</template>
