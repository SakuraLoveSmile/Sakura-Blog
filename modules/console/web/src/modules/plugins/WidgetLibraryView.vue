<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from "vue";
import { Icon } from "@iconify/vue";
import { settingsApi, previewApi } from "@/api";
import { Toast, Dialog, VLoading, VPageHeader } from "@/components/ui";
import {
  WIDGET_META,
  BUILTIN_WIDGET_NAMES,
  metaOf,
  fieldsForLayout,
  NO_EXTRA_PARAM_LAYOUTS,
  builtinDefaults,
} from "../settings/widget-meta";
import { buildPreviewDoc } from "./preview/preview-document";

// 侧边栏组件库视图（Spec 58）：浏览 16 种内置侧边栏组件、编辑参数并保存为命名方案。
// 方案 = widgets.library 的命名条目（name → layout + 参数），零后端改动；
// 保存后经 PUT /settings/widgets 整段写回（含 pages，防止覆盖编排序列）。

interface PageBars {
  leftbar?: string[];
  rightbar?: string[];
}

const loading = ref(true);
const loadError = ref("");
const widgets = ref<{ library: Record<string, any>; pages: Record<string, PageBars> }>({
  library: {},
  pages: {},
});

async function load() {
  loading.value = true;
  loadError.value = "";
  try {
    const { data } = await settingsApi.all();
    const w = (data?.widgets || {}) as any;
    widgets.value = {
      library: w.library && typeof w.library === "object" ? w.library : {},
      pages: w.pages && typeof w.pages === "object" ? w.pages : {},
    };
    select("builtin", selName.value);
  } catch {
    loadError.value = "加载设置失败";
  } finally {
    loading.value = false;
  }
}

async function putWidgets() {
  await settingsApi.updateSection("widgets", widgets.value);
}

onMounted(load);

// ---------- 选择与草稿 ----------
type SelKind = "builtin" | "scheme";
const selKind = ref<SelKind>("builtin");
const selName = ref("tagcloud");
// draft = 当前编辑中的定义（含 layout），保存时写回 library
const draft = ref<Record<string, any>>({ layout: "tagcloud" });

function select(kind: SelKind, name: string) {
  selKind.value = kind;
  selName.value = name;
  if (kind === "builtin") {
    // 内置组件：缺省参数为基底，叠加用户已有同名覆盖（WidgetsTab 里自定义过的参数）
    const override = widgets.value.library[name];
    draft.value = {
      ...builtinDefaults(name),
      ...(override && typeof override === "object" ? override : {}),
      layout: builtinDefaults(name).layout,
    };
  } else {
    const entry = widgets.value.library[name];
    draft.value = entry && typeof entry === "object" ? JSON.parse(JSON.stringify(entry)) : {};
  }
  jsonText.value = JSON.stringify(draft.value, null, 2);
  schedulePreview(true);
}

const selMeta = computed(() => (selKind.value === "builtin" ? metaOf(selName.value) : undefined));
const draftLayout = computed(() => String(draft.value?.layout || ""));
const draftFields = computed(() => fieldsForLayout(draftLayout.value));
const noExtra = computed(() => NO_EXTRA_PARAM_LAYOUTS.has(draftLayout.value));

// ---------- 列表（搜索过滤） ----------
const search = ref("");
const filteredMeta = computed(() => {
  const kw = search.value.trim().toLowerCase();
  if (!kw) return WIDGET_META;
  return WIDGET_META.filter((m) => m.name.toLowerCase().includes(kw) || m.label.toLowerCase().includes(kw));
});
const schemeNames = computed(() =>
  Object.keys(widgets.value.library)
    .filter((n) => !BUILTIN_WIDGET_NAMES.has(n))
    .sort(),
);
const filteredSchemes = computed(() => {
  const kw = search.value.trim().toLowerCase();
  return kw ? schemeNames.value.filter((n) => n.toLowerCase().includes(kw)) : schemeNames.value;
});
function schemeLayout(name: string): string {
  return String(widgets.value.library[name]?.layout || "");
}
function isSelected(kind: SelKind, name: string): boolean {
  return selKind.value === kind && selName.value === name;
}

// ---------- 方案名校验 ----------
function validateSchemeName(name: string): string {
  const n = name.trim();
  if (!n) return "方案名不能为空";
  if (!/^[A-Za-z0-9_\u4e00-\u9fa5-]+$/.test(n)) return "方案名仅支持字母、数字、下划线、中划线与中文";
  if (n.startsWith("__")) return "方案名不能以双下划线开头（保留前缀）";
  if (BUILTIN_WIDGET_NAMES.has(n)) return `「${n}」是内置组件名，不能作为方案名`;
  return "";
}

// ---------- 保存 ----------
const saving = ref(false);

function cleanDraft(): Record<string, any> {
  const out: Record<string, any> = {};
  for (const [k, v] of Object.entries(draft.value)) {
    if (v === undefined || v === null || v === "") continue;
    out[k] = v;
  }
  if (!out.layout) out.layout = draftLayout.value || selName.value;
  return out;
}

// 「保存为方案」（内置组件详情）与「另存为」（方案详情）共用
const nameInputVisible = ref(false);
const nameInputValue = ref("");

function openNameInput() {
  nameInputValue.value = "";
  nameInputVisible.value = true;
}

async function confirmNameInput() {
  const err = validateSchemeName(nameInputValue.value);
  if (err) {
    Toast.error(err);
    return;
  }
  if (widgets.value.library[nameInputValue.value.trim()]) {
    Toast.error(`方案「${nameInputValue.value.trim()}」已存在，请换一个名字`);
    return;
  }
  const name = nameInputValue.value.trim();
  saving.value = true;
  try {
    widgets.value.library[name] = cleanDraft();
    await putWidgets();
    Toast.success("方案已保存", { description: `「${name}」可在设置 → 组件编排中选用` });
    nameInputVisible.value = false;
    select("scheme", name);
  } catch {
  } finally {
    saving.value = false;
  }
}

async function saveScheme() {
  if (selKind.value !== "scheme") return;
  saving.value = true;
  try {
    widgets.value.library[selName.value] = cleanDraft();
    await putWidgets();
    Toast.success("已保存，前台已生效");
  } catch {
  } finally {
    saving.value = false;
  }
}

// ---------- 删除（含 pages 引用检查） ----------
const PAGE_LABELS: Record<string, string> = {
  index: "首页",
  post: "文章页",
  page: "独立页",
  archive: "归档页",
  categories: "分类页",
  tags: "标签页",
  author: "作者页",
};

function referencedPages(name: string): string[] {
  const out: string[] = [];
  for (const [pageType, bars] of Object.entries(widgets.value.pages)) {
    const used = [...(bars?.leftbar || []), ...(bars?.rightbar || [])].includes(name);
    if (used) out.push(PAGE_LABELS[pageType] || pageType);
  }
  return out;
}

function removeScheme() {
  if (selKind.value !== "scheme") return;
  const name = selName.value;
  const refs = referencedPages(name);
  Dialog.warning({
    title: `删除方案「${name}」？`,
    description: refs.length
      ? `该方案正被页面类型「${refs.join("、")}」的左右栏序列引用，删除后对应位置将不再渲染此组件。`
      : "删除后不可恢复，可随时重新保存新方案。",
    confirmText: "删除",
    confirmType: "danger",
    onConfirm: async () => {
      try {
        delete widgets.value.library[name];
        await putWidgets();
        Toast.success("方案已删除");
        select("builtin", WIDGET_META[0].name);
      } catch {}
    },
  });
}

// ---------- JSON 兜底 ----------
const jsonText = ref("");
const jsonError = ref("");

function onJsonInput(val: string) {
  jsonText.value = val;
  try {
    const parsed = JSON.parse(val);
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
      draft.value = parsed;
      jsonError.value = "";
    } else {
      jsonError.value = "需为 JSON 对象";
    }
  } catch {
    jsonError.value = "JSON 格式错误";
  }
}

function setField(f: { key: string }, value: unknown) {
  draft.value[f.key] = value;
  jsonText.value = JSON.stringify(draft.value, null, 2);
}

const inputCls = "apple-input";

// ---------- 渲染预览（复用设置页草稿预览管线 POST /preview/site） ----------
// 把当前草稿以保留名 __preview__ 注入草稿 widgets（不落库），leftbar 仅挂该组件，
// 后端用真实前台管线渲染整页后，前端提取该 widget 片段塞入 iframe。
const PREVIEW_NAME = "__preview__";

/** 组件 layout → 预览载体页面类型（数据上下文匹配） */
function previewPageFor(layout: string): string {
  if (["toc", "author", "related"].includes(layout)) return "post";
  if (["tree", "wiki_related"].includes(layout)) return "wiki";
  return "index";
}

const NO_OUTPUT_HINTS: Record<string, string> = {
  related: "专栏功能当前已禁用，前台不渲染此组件",
  tree: "文库组件仅文库文档页有数据，预览以首个项目模拟",
  wiki_related: "文库组件仅文库文档页有数据，预览以首个项目模拟",
  ghrepo: "需配置 repo/username/api 参数",
  ghuser: "需配置 repo/username/api 参数",
  ghissues: "需配置 repo/username/api 参数",
  timeline: "需配置 repo/username/api 参数",
};

const previewHtml = ref("");
const previewLoading = ref(false);
const previewError = ref("");
let previewTimer: ReturnType<typeof setTimeout> | undefined;
let previewSeq = 0;

async function refreshPreview() {
  const seq = ++previewSeq;
  previewLoading.value = true;
  previewError.value = "";
  try {
    const page = previewPageFor(draftLayout.value);
    const sections = {
      widgets: {
        library: { ...widgets.value.library, [PREVIEW_NAME]: cleanDraft() },
        pages: { [page]: { leftbar: [PREVIEW_NAME], rightbar: [] } },
      },
    };
    const { data } = await previewApi.site(sections, page);
    if (seq !== previewSeq) return;
    previewHtml.value = buildWidgetDoc(data?.html || "", draftLayout.value);
  } catch {
    if (seq === previewSeq) previewError.value = "预览渲染失败";
  } finally {
    if (seq === previewSeq) previewLoading.value = false;
  }
}

function schedulePreview(immediate = false) {
  clearTimeout(previewTimer);
  previewTimer = setTimeout(refreshPreview, immediate ? 0 : 500);
}

function buildWidgetDoc(fullHtml: string, layout = ""): string {
  const doc = new DOMParser().parseFromString(fullHtml, "text/html");
  const aside = doc.querySelector("aside.l_left");
  let bodyInner = "";
  if (aside) {
    aside.querySelector("header.header")?.remove();
    aside.querySelector("footer.footer")?.remove();
    aside.querySelector(".search-wrapper")?.remove();
    const widget = aside.querySelector(".widgets > widget:last-child");
    if (!widget) {
      const hint = NO_OUTPUT_HINTS[layout] || "该组件在当前参数与页面类型下无输出（如 GitHub 系需配置 repo/username）";
      bodyInner = `<p style="color:var(--text-p3,#999);font-size:13px;padding:12px">${hint}</p>`;
      aside.querySelector(".widgets")?.remove();
    }
    bodyInner = bodyInner || aside.outerHTML;
  } else {
    bodyInner = `<p style="color:var(--text-p3,#999);font-size:13px;padding:12px">预览渲染失败：未找到侧栏结构</p>`;
  }
  const bodyClass = doc.body.className || "";
  const scanScr = Array.from(doc.body.querySelectorAll("script")).find(
    (s) => !s.src && (s.textContent || "").includes("var services = ["),
  );
  const fx = `<style>
body{padding:12px !important}
aside.l_left{position:relative;width:300px;max-width:90%;margin:0 auto;max-height:none}
.l_left .leftbar-container{height:auto}
.l_left .leftbar-container >.widgets{overflow:visible;mask:none;-webkit-mask:none;flex-grow:0}
</style>`;
  return buildPreviewDoc({
    bodyHtml: bodyInner,
    scripts: ["/assets/js/main.js", "/assets/js/tagcloud.js"],
    bodyClass,
    wrapContent: false,
    extraHead: fx,
    inlineScripts: scanScr ? [scanScr.outerHTML] : [],
  });
}

// 草稿参数变化 → 防抖重渲染；切换选中项立即渲染
watch(draft, () => schedulePreview(), { deep: true });
onBeforeUnmount(() => clearTimeout(previewTimer));
</script>

<template>
  <div>
    <VPageHeader
      title="侧边栏组件"
      subtitle="浏览 16 种内置侧边栏组件、编辑参数并保存为命名方案（零后端改动）。"
    />
    <div class="mx-auto max-w-[1400px] px-4 py-5 sm:px-6 md:px-8">
      <VLoading v-if="loading" />
    <div v-else-if="loadError" class="p-6 text-sm text-state-error">{{ loadError }}</div>

    <div v-else class="widget-studio-container">
      <!-- 左侧：搜索 + 内置组件 + 已保存方案 -->
      <aside class="widget-sidebar">
        <div class="widget-sidebar-header">
          <div class="widget-search-wrap">
            <Icon icon="ri:search-line" class="widget-search-icon" />
            <input
              v-model="search"
              placeholder="搜索组件 / 方案..."
              class="widget-search-input"
            />
            <button
              v-if="search"
              type="button"
              class="widget-search-clear"
              title="清空"
              @click="search = ''"
            >
              <Icon icon="ri:close-circle-fill" />
            </button>
          </div>
        </div>

        <div class="widget-sidebar-list">
          <!-- 内置组件组 -->
          <div class="widget-group-label">
            <span>内置组件</span>
            <span class="group-count">{{ filteredMeta.length }}</span>
          </div>
          <button
            v-for="m in filteredMeta"
            :key="m.name"
            type="button"
            @click="select('builtin', m.name)"
            :class="['widget-list-item', { active: isSelected('builtin', m.name) }]"
          >
            <span class="widget-item-title">{{ m.label }}</span>
            <span class="widget-item-layout">{{ m.name }}</span>
          </button>

          <!-- 已保存方案组 -->
          <div class="widget-group-label widget-group-label-scheme">
            <span>已保存方案</span>
            <span class="group-count">{{ filteredSchemes.length }}</span>
          </div>
          <button
            v-for="n in filteredSchemes"
            :key="n"
            type="button"
            @click="select('scheme', n)"
            :class="['widget-list-item', 'scheme-item', { active: isSelected('scheme', n) }]"
          >
            <Icon icon="ri:bookmark-3-line" class="scheme-icon" />
            <span class="widget-item-title">{{ n }}</span>
            <span class="scheme-layout-tag">{{ schemeLayout(n) }}</span>
          </button>
          <p v-if="!filteredSchemes.length" class="widget-empty-schemes">
            暂无方案 — 选择内置组件编辑参数后「保存为方案」
          </p>
        </div>
      </aside>

      <!-- 右侧：详情 + 参数表单 + 预览 + 保存操作 -->
      <main class="widget-workbench">
        <!-- 1. 顶部 Header 概览卡片 -->
        <div class="widget-hero-card">
          <div class="widget-hero-title-row">
            <h3 class="widget-hero-title">
              {{ selKind === "builtin" ? selMeta?.label || selName : selName }}
            </h3>
            <span class="widget-layout-pill">
              <code>{{ draftLayout }}</code>
            </span>
            <span v-if="selKind === 'scheme'" class="widget-scheme-pill">
              <Icon icon="ri:bookmark-3-line" />
              <span>自定义方案</span>
            </span>
          </div>
          <p class="widget-hero-desc">
            {{ selKind === "builtin" ? selMeta?.desc : `基于 ${draftLayout} 布局的自定义方案，可直接插入左/右侧边栏` }}
          </p>
        </div>

        <!-- 2. 参数表单 -->
        <section class="widget-section">
          <div class="widget-section-heading">
            <h2>参数设置</h2>
            <p>调整该侧边栏组件的展示配置与选项。</p>
          </div>
          <div class="widget-panel widget-form-panel">
            <template v-if="draftFields && draftFields.length">
              <div class="widget-fields-grid">
                <div
                  v-for="f in draftFields"
                  :key="f.key"
                  :class="['widget-field-wrap', f.type === 'textarea' ? 'field-full' : '']"
                >
                  <label class="widget-field-label">
                    <span>{{ f.label || f.key }}</span>
                    <code class="field-key-tag">{{ f.key }}</code>
                  </label>
                  <input
                    v-if="f.type === 'checkbox'"
                    type="checkbox"
                    :checked="!!draft[f.key]"
                    @change="setField(f, ($event.target as HTMLInputElement).checked)"
                    class="widget-checkbox"
                  />
                  <select
                    v-else-if="f.type === 'select'"
                    :value="draft[f.key]"
                    @change="setField(f, ($event.target as HTMLSelectElement).value)"
                    :class="inputCls"
                  >
                    <option v-for="o in f.options" :key="o" :value="o">{{ o }}</option>
                  </select>
                  <textarea
                    v-else-if="f.type === 'textarea'"
                    rows="4"
                    :value="draft[f.key]"
                    @input="setField(f, ($event.target as HTMLTextAreaElement).value)"
                    :placeholder="f.hint || ''"
                    :class="[inputCls, 'widget-textarea']"
                  ></textarea>
                  <div v-else-if="f.type === 'color'" class="widget-color-wrap">
                    <input
                      type="color"
                      :value="draft[f.key] || '#000000'"
                      @input="setField(f, ($event.target as HTMLInputElement).value)"
                      class="widget-color-picker"
                    />
                    <input
                      type="text"
                      :value="draft[f.key]"
                      @input="setField(f, ($event.target as HTMLInputElement).value)"
                      :class="[inputCls, 'widget-color-text']"
                    />
                  </div>
                  <input
                    v-else
                    :type="f.type === 'number' ? 'number' : 'text'"
                    :value="draft[f.key]"
                    @input="
                      setField(
                        f,
                        f.type === 'number'
                          ? Number(($event.target as HTMLInputElement).value)
                          : ($event.target as HTMLInputElement).value,
                      )
                    "
                    :placeholder="f.hint || ''"
                    :class="[inputCls, f.type === 'number' ? '' : 'font-mono']"
                  />
                  <p v-if="f.hint && f.type !== 'textarea' && f.type !== 'text'" class="widget-field-hint">
                    {{ f.hint }}
                  </p>
                </div>
              </div>
            </template>

            <p v-else-if="noExtra" class="widget-no-extra">该组件无额外可配置参数。</p>

            <!-- 未知 layout：JSON 兜底 -->
            <div v-else class="widget-json-fallback">
              <label class="widget-field-label">参数 (JSON 对象)</label>
              <textarea
                :value="jsonText"
                @input="onJsonInput(($event.target as HTMLTextAreaElement).value)"
                rows="6"
                spellcheck="false"
                :class="[inputCls, 'widget-json-editor']"
              ></textarea>
              <p v-if="jsonError" class="widget-json-error">{{ jsonError }}</p>
            </div>
          </div>
        </section>

        <!-- 3. 渲染预览（草稿参数，走前台真实渲染管线） -->
        <section class="widget-section">
          <div class="widget-section-heading">
            <h2>渲染预览</h2>
            <p>使用前台真实渲染管线与侧栏玻璃质感样式进行模拟。</p>
          </div>
          <div class="widget-panel widget-canvas-panel">
            <!-- macOS Window Header -->
            <div class="widget-canvas-header">
              <div class="canvas-window-dots" aria-hidden="true">
                <span class="dot dot-red"></span>
                <span class="dot dot-yellow"></span>
                <span class="dot dot-green"></span>
              </div>
              <div class="canvas-header-meta">
                <span class="canvas-page-tag">
                  <Icon icon="ri:file-list-line" />
                  <span>载体：{{ previewPageFor(draftLayout) }} 页</span>
                </span>
                <span v-if="previewLoading" class="canvas-loading-tag">
                  <Icon icon="ri:loader-4-line" class="animate-spin" />
                  <span>渲染中...</span>
                </span>
              </div>
              <button
                type="button"
                class="canvas-refresh-btn"
                title="重新渲染预览"
                @click="schedulePreview(true)"
              >
                <Icon icon="ri:refresh-line" :class="{ 'animate-spin': previewLoading }" />
                <span>刷新预览</span>
              </button>
            </div>
            <div class="widget-canvas-body">
              <div v-if="previewError" class="canvas-state canvas-error">
                <Icon icon="ri:error-warning-line" width="28" height="28" />
                <p>{{ previewError }}</p>
              </div>
              <iframe
                v-else
                :srcdoc="previewHtml"
                class="widget-preview-iframe"
                sandbox="allow-same-origin allow-scripts"
              ></iframe>
            </div>
          </div>
        </section>

        <!-- 4. 操作区 -->
        <section class="widget-section">
          <div class="widget-section-heading">
            <h2>{{ selKind === "builtin" ? "保存为方案" : "方案操作" }}</h2>
            <p>{{ selKind === "builtin" ? "以当前参数创建命名方案，可在设置 → 组件编排中选用。" : "管理已保存方案的更新与页面引用。" }}</p>
          </div>
          <div class="widget-panel widget-actions-panel">
            <!-- 内置组件：保存为方案 -->
            <template v-if="selKind === 'builtin'">
              <div class="widget-action-row">
                <div v-if="nameInputVisible" class="widget-name-input-group">
                  <input
                    v-model="nameInputValue"
                    placeholder="输入方案名，如「彩色标签云」"
                    :class="inputCls"
                    class="scheme-name-input"
                    @keyup.enter="confirmNameInput"
                  />
                  <button
                    type="button"
                    :disabled="saving || !nameInputValue.trim()"
                    class="apple-btn apple-btn-primary"
                    @click="confirmNameInput"
                  >
                    <Icon :icon="saving ? 'ri:loader-4-line' : 'ri:check-line'" :class="{ 'animate-spin': saving }" />
                    <span>确定保存</span>
                  </button>
                  <button
                    type="button"
                    class="apple-btn apple-btn-secondary"
                    @click="nameInputVisible = false"
                  >
                    取消
                  </button>
                </div>
                <button
                  v-else
                  type="button"
                  class="apple-btn apple-btn-primary"
                  @click="openNameInput"
                >
                  <Icon icon="ri:bookmark-3-line" />
                  <span>保存为新方案</span>
                </button>
              </div>
              <p v-if="widgets.library[selName]" class="widget-override-hint">
                <Icon icon="ri:information-line" />
                <span>当前 library 已存在同名覆盖配置（来自组件编排的参数编辑），上方表单已包含该覆盖。</span>
              </p>
            </template>

            <!-- 已保存方案：保存 / 另存为 / 删除 -->
            <template v-else>
              <div class="widget-action-row flex-wrap">
                <button
                  type="button"
                  :disabled="saving"
                  class="apple-btn apple-btn-primary"
                  @click="saveScheme"
                >
                  <Icon :icon="saving ? 'ri:loader-4-line' : 'ri:save-line'" :class="{ 'animate-spin': saving }" />
                  <span>保存修改</span>
                </button>
                <button
                  type="button"
                  class="apple-btn apple-btn-secondary"
                  @click="openNameInput"
                >
                  <Icon icon="ri:file-copy-line" />
                  <span>另存为新方案</span>
                </button>
                <button
                  type="button"
                  class="apple-btn apple-btn-danger"
                  @click="removeScheme"
                >
                  <Icon icon="ri:delete-bin-line" />
                  <span>删除方案</span>
                </button>
                <span v-if="referencedPages(selName).length" class="widget-ref-badge">
                  <Icon icon="ri:links-line" />
                  <span>被页面类型「{{ referencedPages(selName).join("、") }}」引用中</span>
                </span>
              </div>

              <div v-if="nameInputVisible" class="widget-name-input-group mt-3">
                <input
                  v-model="nameInputValue"
                  placeholder="输入新方案名"
                  :class="inputCls"
                  class="scheme-name-input"
                  @keyup.enter="confirmNameInput"
                />
                <button
                  type="button"
                  :disabled="saving || !nameInputValue.trim()"
                  class="apple-btn apple-btn-primary"
                  @click="confirmNameInput"
                >
                  <Icon :icon="saving ? 'ri:loader-4-line' : 'ri:check-line'" :class="{ 'animate-spin': saving }" />
                  <span>保存为新方案</span>
                </button>
                <button
                  type="button"
                  class="apple-btn apple-btn-secondary"
                  @click="nameInputVisible = false"
                >
                  取消
                </button>
              </div>
            </template>
          </div>
        </section>
      </main>
    </div>
    </div>
  </div>
</template>

<style scoped>
.widget-studio-container {
  display: flex;
  align-items: flex-start;
}

/* 左侧边栏 */
.widget-sidebar {
  width: 290px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  background: var(--color-card);
  border-right: 1px solid var(--color-border);
  position: sticky;
  top: 1.25rem;
  align-self: flex-start;
  max-height: calc(100vh - 2.5rem);
}

.widget-sidebar-header {
  padding: 14px;
  border-bottom: 1px solid var(--color-border);
}

.widget-search-wrap {
  position: relative;
  display: flex;
  align-items: center;
}

.widget-search-icon {
  position: absolute;
  left: 10px;
  color: var(--color-text-quaternary);
  font-size: 15px;
  pointer-events: none;
}

.widget-search-input {
  width: 100%;
  box-sizing: border-box;
  padding: 7px 28px 7px 32px;
  font-size: 13px;
  border-radius: var(--radius-md, 8px);
  border: 1px solid var(--color-border);
  background: color-mix(in srgb, var(--color-background) 50%, var(--color-card));
  color: var(--color-foreground);
  transition: all 0.2s ease;
}

.widget-search-input:focus {
  outline: none;
  border-color: var(--color-primary);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--color-primary) 20%, transparent);
}

.widget-search-clear {
  position: absolute;
  right: 8px;
  background: transparent;
  border: none;
  color: var(--color-text-quaternary);
  cursor: pointer;
  padding: 2px;
  display: flex;
  align-items: center;
}

.widget-sidebar-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px 0;
}

.widget-group-label {
  padding: 6px 14px;
  font-size: 11px;
  font-weight: 600;
  color: var(--color-text-quaternary);
  display: flex;
  align-items: center;
  justify-content: space-between;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}

.widget-group-label-scheme {
  margin-top: 14px;
}

.group-count {
  font-size: 10px;
  opacity: 0.8;
}

.widget-list-item {
  width: calc(100% - 12px);
  margin: 2px 6px;
  padding: 8px 10px;
  border-radius: var(--radius-md, 8px);
  border: 1px solid transparent;
  background: transparent;
  display: flex;
  align-items: center;
  gap: 8px;
  text-align: left;
  cursor: pointer;
  transition: all 0.15s ease;
}

.widget-list-item:hover {
  background: color-mix(in srgb, var(--color-foreground) 4%, transparent);
}

.widget-list-item.active {
  background: color-mix(in srgb, var(--color-primary) 12%, var(--color-card));
  border-color: color-mix(in srgb, var(--color-primary) 30%, transparent);
}

.widget-item-title {
  font-size: 13px;
  color: var(--color-foreground);
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.widget-list-item.active .widget-item-title {
  color: var(--color-primary);
  font-weight: 600;
}

.widget-item-layout {
  font-family: var(--font-mono, monospace);
  font-size: 11px;
  color: var(--color-text-quaternary);
  flex-shrink: 0;
}

.scheme-icon {
  color: var(--color-primary);
  font-size: 14px;
  flex-shrink: 0;
}

.scheme-layout-tag {
  font-family: var(--font-mono, monospace);
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 4px;
  background: color-mix(in srgb, var(--color-primary) 15%, transparent);
  color: var(--color-primary);
  flex-shrink: 0;
}

.widget-empty-schemes {
  margin: 8px 14px;
  font-size: 11.5px;
  color: var(--color-text-quaternary);
  line-height: 1.5;
}

/* ================= 右侧工作台 ================= */
.widget-workbench {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  padding: 24px 32px;
  display: flex;
  flex-direction: column;
  gap: 24px;
  box-sizing: border-box;
}

/* Hero 概览卡片 */
.widget-hero-card {
  background: linear-gradient(135deg, color-mix(in srgb, var(--color-background) 40%, var(--color-card)), var(--color-card));
  border: 1px solid var(--color-border);
  border-radius: var(--radius-xl);
  padding: 20px 24px;
  box-shadow: var(--shadow-sm);
}

.widget-hero-title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}

.widget-hero-title {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
  color: var(--color-foreground);
  letter-spacing: -0.01em;
  display: flex;
  align-items: center;
  gap: 8px;
}

.widget-hero-title::before {
  content: "";
  display: inline-block;
  width: 3.5px;
  height: 18px;
  border-radius: 2px;
  background: var(--color-primary);
}

.widget-layout-pill {
  font-family: var(--font-mono, monospace);
  font-size: 12px;
  padding: 2px 8px;
  border-radius: 6px;
  border: 1px solid var(--color-border);
  background: color-mix(in srgb, var(--color-background) 50%, var(--color-card));
  color: var(--color-text-secondary);
}

.widget-scheme-pill {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 9999px;
  background: color-mix(in srgb, var(--color-primary) 15%, transparent);
  color: var(--color-primary);
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-weight: 550;
}

.widget-hero-desc {
  margin: 0;
  color: var(--color-text-secondary);
  font-size: 13.5px;
  line-height: 1.6;
}

/* 区域分块（Option 1 Inset Group Panel） */
.widget-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.widget-section-heading {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.widget-section-heading h2 {
  margin: 0;
  font-size: 15.5px;
  font-weight: 650;
  color: var(--color-foreground);
  display: flex;
  align-items: center;
  gap: 8px;
}

.widget-section-heading h2::before {
  content: "";
  display: inline-block;
  width: 3.5px;
  height: 15px;
  border-radius: 2px;
  background: var(--color-primary);
}

.widget-section-heading p {
  margin: 0;
  font-size: 12px;
  color: var(--color-text-tertiary);
  padding-left: 11.5px;
}

.widget-panel {
  background: color-mix(in srgb, var(--color-background) 38%, var(--color-card));
  border: 1px solid var(--color-border);
  border-radius: var(--radius-xl);
  overflow: hidden;
}

/* 参数表单面板 */
.widget-form-panel {
  padding: 20px 24px;
}

.widget-fields-grid {
  display: grid;
  grid-template-columns: repeat(1, minmax(0, 1fr));
  gap: 16px;
}

@media (min-width: 768px) {
  .widget-fields-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

.field-full {
  grid-column: 1 / -1;
}

.widget-field-wrap {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.widget-field-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 550;
  color: var(--color-foreground);
}

.field-key-tag {
  font-family: var(--font-mono, monospace);
  font-size: 11px;
  color: var(--color-text-quaternary);
  font-weight: normal;
}

.widget-checkbox {
  width: 18px;
  height: 18px;
  accent-color: var(--color-primary);
  cursor: pointer;
  margin-top: 4px;
}

.widget-textarea {
  font-family: var(--font-mono, monospace);
  font-size: 12.5px;
  line-height: 1.5;
  resize: vertical;
}

.widget-color-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
}

.widget-color-picker {
  width: 36px;
  height: 36px;
  padding: 2px;
  border-radius: var(--radius-md, 8px);
  border: 1px solid var(--color-border);
  background: transparent;
  cursor: pointer;
}

.widget-color-text {
  font-family: var(--font-mono, monospace);
}

.widget-field-hint {
  margin: 0;
  font-size: 11.5px;
  color: var(--color-text-quaternary);
  line-height: 1.4;
}

.widget-no-extra {
  margin: 0;
  font-size: 13px;
  color: var(--color-text-quaternary);
}

.widget-json-fallback {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.widget-json-editor {
  font-family: var(--font-mono, monospace);
  font-size: 12px;
  line-height: 1.5;
}

.widget-json-error {
  margin: 0;
  font-size: 11.5px;
  color: var(--color-state-error);
}

/* 画布预览面板 */
.widget-canvas-panel {
  background: var(--color-card);
}

.widget-canvas-header {
  padding: 10px 16px;
  background: color-mix(in srgb, var(--color-background) 35%, var(--color-card));
  border-bottom: 1px solid var(--color-border);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.canvas-header-meta {
  display: flex;
  align-items: center;
  gap: 8px;
}

.canvas-page-tag {
  font-size: 11.5px;
  color: var(--color-text-tertiary);
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.canvas-loading-tag {
  font-size: 11px;
  color: var(--color-primary);
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.widget-canvas-body {
  position: relative;
  background: #121316;
  min-height: 260px;
}

.widget-preview-iframe {
  display: block;
  width: 100%;
  border: none;
  min-height: 260px;
  max-height: 520px;
  resize: vertical;
  overflow: auto;
}

/* 操作面板 */
.widget-actions-panel {
  padding: 18px 22px;
  background: var(--color-card);
}

.widget-action-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.widget-name-input-group {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.scheme-name-input {
  width: 240px;
}

.apple-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  border-radius: 9999px;
  font-size: 13px;
  font-weight: 550;
  cursor: pointer;
  transition: all 0.15s ease;
}

.apple-btn-primary {
  background: var(--color-primary);
  color: #fff;
  border: 1px solid var(--color-primary);
}
.apple-btn-primary:hover:not(:disabled) {
  opacity: 0.9;
}
.apple-btn-primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.apple-btn-secondary {
  background: color-mix(in srgb, var(--color-foreground) 6%, transparent);
  color: var(--color-text-secondary);
  border: 1px solid var(--color-border);
}
.apple-btn-secondary:hover {
  background: color-mix(in srgb, var(--color-foreground) 10%, transparent);
  color: var(--color-foreground);
}

.apple-btn-danger {
  background: rgba(255, 69, 58, 0.12);
  color: #ff453a;
  border: 1px solid rgba(255, 69, 58, 0.3);
}
.apple-btn-danger:hover {
  background: rgba(255, 69, 58, 0.2);
}

.widget-ref-badge {
  font-size: 11.5px;
  color: var(--color-text-quaternary);
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-left: 6px;
}

.widget-override-hint {
  margin: 10px 0 0;
  font-size: 11.5px;
  color: var(--color-text-quaternary);
  display: flex;
  align-items: center;
  gap: 5px;
}

@media (max-width: 768px) {
  .widget-studio-container {
    flex-direction: column;
    overflow-y: auto;
  }
  .widget-sidebar {
    width: 100%;
    border-right: none;
    border-bottom: 1px solid var(--color-border);
    max-height: 300px;
    position: static;
  }
  .widget-workbench {
    padding: 16px;
    gap: 16px;
  }
}
</style>
