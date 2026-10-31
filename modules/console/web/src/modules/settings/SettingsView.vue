<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { onBeforeRouteLeave, useRoute, useRouter } from "vue-router";
import { Icon } from "@iconify/vue";
import { getApiErrorMessage, settingsApi } from "@/api";
import { Toast, VButton, VCard, VPageHeader, VTabbar } from "@/components/ui";
import BasicSettings from "./tabs/BasicSettings.vue";
import SeoSettings from "./tabs/SeoSettings.vue";
import FeedbackSettings from "./tabs/FeedbackSettings.vue";
import Stellar2Tab from "./Stellar2Tab.vue";
import CanonicalSettingsForm from "./CanonicalSettingsForm.vue";
import StellarSettingsStatus from "./StellarSettingsStatus.vue";
import SettingsEffectGuide from "./SettingsEffectGuide.vue";
import { canonicalSettingsPaths, type CanonicalSettingsTab } from "./settings-canonical-fields";
import { useStellarSettings } from "./useStellarSettings";
import { readJSONFieldRaw } from "@/components/editor/stellar-form-json";
import { ALL_SETTINGS_SECTIONS, ensureSettingsDefaults, sortWikiProjects, deepCloneJsonSafe } from "./settings-state";
import { settingsPreviewPages, useSettingsPreview } from "./useSettingsPreview";
import "./settings-form.css";

// Shared defaults retain the existing frontend enhancement safety boundary: card_hover.enable = false; card_hover.max_tilt = 3.

const tabs = [
  { key: "basic", label: "基本", icon: "ri:global-line", description: "站点身份、标题与品牌资源" },
  { key: "appearance", label: "外观", icon: "ri:palette-line", description: "配色、字体、背景与圆角" },
  { key: "navigation", label: "导航", icon: "ri:menu-line", description: "顶栏、侧栏、菜单与页脚" },
  { key: "layout", label: "页面布局", icon: "ri:layout-line", description: "页面覆盖与侧栏组件" },
  { key: "post", label: "文章", icon: "ri:article-line", description: "文章排版、卡片、版权、分享与打赏" },
  { key: "comments", label: "评论", icon: "ri:chat-3-line", description: "评论开关、审核与邮件通知" },
  { key: "seo", label: "SEO", icon: "ri:search-eye-line", description: "站点来源、社交卡片与搜索引擎设置" },
  { key: "search", label: "搜索", icon: "ri:search-line", description: "搜索服务、索引范围与正文检索" },
  { key: "plugins", label: "插件", icon: "ri:puzzle-line", description: "标签组件与前台增强" },
  { key: "services", label: "服务", icon: "ri:cloud-line", description: "外部服务与默认图片" },
  { key: "community", label: "作者与友链", icon: "ri:team-line", description: "公开作者与友链设置" },
  { key: "feedback", label: "反馈", icon: "ri:feedback-line", description: "Feedback 反馈组件接入与入口外观" },
  { key: "advanced", label: "高级", icon: "ri:code-line", description: "页面代码注入与完整配置入口" },
] as const;

const tabSectionsMap: Record<string, string[]> = {
  basic: ["site"],
  post: ["article"],
  comments: ["comments", "site_tree"],
  seo: ["seo", "site"],
  search: ["search"],
  feedback: ["feedback"],
  advanced: ["inject", "data_cache"],
};

const tabKeys = new Set<string>(tabs.map((tab) => tab.key));
const legacyTabMap: Record<string, { path: string; tab?: string }> = {
  site: { path: "/settings", tab: "basic" },
  article: { path: "/settings", tab: "post" },
  comments: { path: "/settings", tab: "comments" },
  seo: { path: "/settings", tab: "seo" },
  search: { path: "/settings", tab: "search" },
  inject: { path: "/settings", tab: "advanced" },
  data_cache: { path: "/settings", tab: "advanced" },
  advanced: { path: "/settings", tab: "advanced" },
  appearance: { path: "/settings", tab: "appearance" },
  theme_yaml: { path: "/settings", tab: "advanced" },
  cover: { path: "/settings", tab: "layout" },
  nav_tabs: { path: "/settings", tab: "navigation" },
  widgets: { path: "/settings", tab: "layout" },
  plugins: { path: "/settings", tab: "plugins" },
  tag_plugins: { path: "/settings", tab: "plugins" },
  services: { path: "/settings", tab: "services" },
  feedback: { path: "/settings", tab: "feedback" },
  default: { path: "/settings", tab: "services" },
  menubar: { path: "/settings", tab: "navigation" },
  rightbar: { path: "/settings", tab: "navigation" },
  footer: { path: "/settings", tab: "navigation" },
  ai: { path: "/assistant", tab: "settings" },
  security: { path: "/profile" },
  authors: { path: "/settings", tab: "community" },
};

const route = useRoute();
const router = useRouter();
const active = ref("basic");
const rawOpen = ref(false);
const data = ref<Record<string, any>>({});
const rawText = ref<Record<string, string>>({});
const loading = ref(true);
const loadError = ref("");
const hostSavingKey = ref("");
const stellar = useStellarSettings();
const {
  draft: stellarDraft,
  saved: stellarSaved,
  loading: stellarLoading,
  busy: stellarBusy,
  loadError: stellarLoadError,
  dirty: stellarDirty,
  parseError: stellarParseError,
  pendingInput,
  loadRevision,
} = stellar;
const savingKey = computed(() => hostSavingKey.value || (stellarBusy.value ? "stellar2" : ""));
const pendingTabs = ref<Record<string, boolean>>({});
const canonicalTabs = new Set<string>([
  "appearance",
  "navigation",
  "layout",
  "post",
  "comments",
  "seo",
  "search",
  "plugins",
  "services",
  "community",
  "advanced",
]);
function isCanonicalTab(tab: string): tab is CanonicalSettingsTab {
  return canonicalTabs.has(tab);
}
watch(
  pendingTabs,
  (tabs) => {
    pendingInput.value = Object.values(tabs).some(Boolean);
  },
  { deep: true, flush: "sync" },
);
watch(loadRevision, () => {
  pendingTabs.value = {};
});
const savedSnapshot = ref<string>("");
const savedJsonBaseline = ref<Record<string, string>>({});

const dirtySections = computed<Set<string>>(() => {
  if (loading.value || !savedSnapshot.value) return new Set();
  const set = new Set<string>();
  try {
    const original = JSON.parse(savedSnapshot.value);
    for (const key of ALL_SETTINGS_SECTIONS) {
      const isFormDirty = JSON.stringify(data.value[key]) !== JSON.stringify(original[key]);
      const isJsonDirty = (rawText.value[key] ?? "") !== (savedJsonBaseline.value[key] ?? "");
      if (isFormDirty || isJsonDirty) {
        set.add(key);
      }
    }
  } catch {}
  return set;
});

const isTabDirty = (tabKey: string) => {
  if (tabKey === "advanced") return stellarDirty.value;
  if (isCanonicalTab(tabKey)) {
    if (pendingTabs.value[tabKey]) return true;
    try {
      if (
        canonicalSettingsPaths(tabKey).some(
          (path) => readJSONFieldRaw(stellarDraft.value, path) !== readJSONFieldRaw(stellarSaved.value, path),
        )
      )
        return true;
    } catch {
      return stellarDirty.value;
    }
    return tabKey === "seo" && dirtySections.value.has("site");
  }
  const sections = tabSectionsMap[tabKey] || [];
  return sections.some((s) => dirtySections.value.has(s));
};

const isDirty = computed(() => {
  if (stellarDirty.value) return true;
  if (dirtySections.value.size > 0) return true;
  for (const key of ALL_SETTINGS_SECTIONS) {
    if ((rawText.value[key] ?? "") !== (savedJsonBaseline.value[key] ?? "")) {
      return true;
    }
  }
  return false;
});

const tabItems = computed(() =>
  tabs.map((tab) => ({
    id: tab.key,
    label: tab.label,
    icon: tab.icon,
    dirty: isTabDirty(tab.key),
  })),
);

function takeSnapshot() {
  savedSnapshot.value = JSON.stringify(data.value);
}

const previewBlocked = computed(
  () =>
    pendingInput.value || Boolean(stellarParseError.value) || stellarLoading.value || Boolean(stellarLoadError.value),
);
const {
  previewOpen,
  previewViewport,
  previewPage,
  previewHtml,
  previewLoading,
  previewError,
  previewFocus,
  refreshPreview,
} = useSettingsPreview(data, active, stellarDraft, previewBlocked);
const previewBody = ref<HTMLElement | null>(null);
const previewSize = ref({ width: 800, height: 400 });
let previewObserver: ResizeObserver | undefined;
watch(previewBody, (element) => {
  previewObserver?.disconnect();
  if (!element) return;
  previewObserver = new ResizeObserver(([entry]) => {
    previewSize.value = { width: entry.contentRect.width, height: entry.contentRect.height };
  });
  previewObserver.observe(element);
});
const previewFrameStyle = computed(() => {
  const width = previewViewport.value === "mobile" ? 390 : previewViewport.value === "tablet" ? 768 : 1200;
  const scale = Math.min(1, previewSize.value.width / width);
  return {
    width: `${width}px`,
    height: `${previewSize.value.height / Math.max(scale, 0.1)}px`,
    transform: `scale(${scale})`,
  };
});

function routeQuery(tab: string) {
  return { ...route.query, tab };
}

async function applyRouteQuery() {
  const queryTab = route.query.tab;
  if (typeof queryTab !== "string" || !queryTab) {
    active.value = "basic";
    return;
  }
  if (queryTab === "stellar2") {
    rawOpen.value = true;
    active.value = "advanced";
    await router.replace({ query: routeQuery("advanced") });
    return;
  }
  const legacy = legacyTabMap[queryTab];
  if (legacy && legacy.path !== "/settings") {
    await router.replace({ path: legacy.path, query: legacy.tab ? { tab: legacy.tab } : undefined });
    return;
  }
  const normalized = legacy?.tab || queryTab;
  active.value = tabKeys.has(normalized) ? normalized : "basic";
}

async function load() {
  loading.value = true;
  loadError.value = "";
  try {
    const { data: response } = await settingsApi.all();
    data.value = ensureSettingsDefaults({ ...response });
    data.value.wiki = sortWikiProjects(data.value.wiki);
    const initialBaselines: Record<string, string> = {};
    for (const section of ALL_SETTINGS_SECTIONS) {
      const jsonStr = JSON.stringify(response[section] ?? null, null, 2);
      rawText.value[section] = jsonStr;
      initialBaselines[section] = jsonStr;
    }
    savedJsonBaseline.value = initialBaselines;
    takeSnapshot();
  } catch {
    loadError.value = "加载设置失败";
  } finally {
    loading.value = false;
  }
}

function updateSectionSnapshot(section: string, savedData: any) {
  try {
    const snap = savedSnapshot.value ? JSON.parse(savedSnapshot.value) : {};
    snap[section] = JSON.parse(JSON.stringify(savedData));
    savedSnapshot.value = JSON.stringify(snap);
  } catch {}
}

async function save(section: string, payload?: unknown): Promise<boolean> {
  if (section !== "site" && section !== "feedback") {
    Toast.error("旧主题分段停止保存，请使用对应设置分类");
    return false;
  }
  if (savingKey.value) return false;
  hostSavingKey.value = section;
  try {
    const rawToSave = payload !== undefined ? payload : data.value[section];
    const body = deepCloneJsonSafe(rawToSave ?? {});
    const jsonAtStart = rawText.value[section];
    const hadNoIndependentDraft = jsonAtStart === savedJsonBaseline.value[section];

    await settingsApi.updateSection(section, body);

    const formatted = JSON.stringify(body, null, 2);
    // 普通保存前记录该分区 JSON 文本。成功后，只有文本仍等于请求开始时文本，且开始时没有独立 JSON 草稿，才自动同步为提交内容；否则保留用户草稿。
    if (rawText.value[section] === jsonAtStart && hadNoIndependentDraft) {
      rawText.value[section] = formatted;
      savedJsonBaseline.value[section] = formatted;
    } else {
      savedJsonBaseline.value[section] = formatted;
    }

    updateSectionSnapshot(section, body);
    Toast.success("设置已保存，前台已生效");
    return true;
  } catch (error) {
    Toast.error(getApiErrorMessage(error, "保存失败，请检查参数或网络后重试"));
    return false;
  } finally {
    hostSavingKey.value = "";
  }
}

async function saveStellar() {
  if (savingKey.value) return;
  if (await stellar.submit("save")) Toast.success("主题设置已保存，前台已生效");
}

function navigateToTab(tab: string) {
  active.value = tab;
}

watch(active, (tab) => {
  if (route.query.tab !== tab) void router.push({ query: routeQuery(tab) });
});
watch(
  () => route.query.tab,
  () => void applyRouteQuery(),
);

function handleBeforeUnload(e: BeforeUnloadEvent) {
  if (isDirty.value) {
    e.preventDefault();
    e.returnValue = "";
  }
}

onMounted(() => {
  void applyRouteQuery();
  void load();
  void stellar.load();
  window.addEventListener("beforeunload", handleBeforeUnload);
});

onBeforeUnmount(() => {
  previewObserver?.disconnect();
  window.removeEventListener("beforeunload", handleBeforeUnload);
});

onBeforeRouteLeave((to, from, next) => {
  if (to.path === from.path) {
    next();
    return;
  }
  if (isDirty.value) {
    const confirmed = window.confirm("您有未保存的设置更改，离开后修改将丢失，是否确认离开？");
    if (confirmed) {
      next();
    } else {
      next(false);
    }
    return;
  }
  next();
});
</script>

<template>
  <div class="settings-view" :class="{ 'preview-docked': previewOpen }">
    <VPageHeader title="系统设置">
      <template #icon><Icon icon="ri:settings-3-line" class="text-primary" /></template>
      <template #subtitle>
        <span>站点信息、主题外观、页面布局与各项功能配置</span>
        <span v-if="isDirty" class="settings-dirty-badge">（有未保存修改）</span>
      </template>
      <template #actions>
        <VButton
          v-if="previewFocus"
          size="sm"
          :type="previewOpen ? 'primary' : 'secondary'"
          @click="previewOpen = !previewOpen"
        >
          <template #icon><Icon icon="ri:eye-line" /></template><span>前端预览</span>
        </VButton>
      </template>
    </VPageHeader>

    <div class="settings-container">
      <main class="settings-content">
        <VCard class="settings-basic-panel">
          <template #header>
            <div class="settings-card-tabs settings-tabs">
              <VTabbar
                v-model:active-id="active"
                :items="tabItems"
                type="default"
                :auto-reveal="true"
                class="settings-card-tabbar"
                @change="navigateToTab"
              />
            </div>
          </template>

          <div v-if="loading" class="settings-state">
            <Icon icon="ri:loader-4-line" class="settings-spin" width="22" height="22" /><span>加载中...</span>
          </div>
          <div v-else-if="loadError" class="settings-state settings-state-error">
            <Icon icon="ri:error-warning-line" width="22" height="22" /><span>设置加载失败：{{ loadError }}</span
            ><VButton type="secondary" size="sm" @click="load">重试</VButton>
          </div>
          <template v-else>
            <div
              v-for="tab in tabs"
              :id="`tabpanel-${tab.key}`"
              :key="tab.key"
              role="tabpanel"
              :aria-labelledby="`tab-${tab.key}`"
              tabindex="0"
              class="settings-tab-panel outline-none"
              :hidden="active !== tab.key"
            >
              <SettingsEffectGuide
                v-if="active === tab.key"
                :tab="tab.key"
                :preview-available="Boolean(previewFocus)"
                :dirty="isTabDirty(tab.key)"
                @preview="previewOpen = true"
              />
              <div v-if="isCanonicalTab(tab.key)" class="settings-tab-content">
                <SeoSettings v-if="tab.key === 'seo'" :data="data" :saving="savingKey" host-only @save="save" />
                <div class="canonical-settings-status">
                  <StellarSettingsStatus :editor="stellar" :saving="savingKey" />
                  <p v-if="stellarParseError" class="text-sm text-state-error" role="alert">
                    完整配置暂时无法用于表单编辑：{{ stellarParseError }}。请展开高级中的完整配置修正后继续。
                  </p>
                </div>
                <CanonicalSettingsForm
                  v-if="!stellarLoading && !stellarLoadError && !stellarParseError"
                  :key="`${tab.key}-${loadRevision}`"
                  :tab="tab.key"
                  :document="stellarDraft"
                  :saving="savingKey || (pendingInput ? 'pending-input' : '')"
                  @change="stellar.updateField"
                  @pending="pendingTabs[tab.key] = $event"
                  @save="saveStellar"
                />
                <details
                  v-if="tab.key === 'advanced'"
                  class="settings-raw-editor"
                  :open="rawOpen"
                  @toggle="rawOpen = ($event.target as HTMLDetailsElement).open"
                >
                  <summary>完整配置原文（高级）</summary>
                  <Stellar2Tab :editor="stellar" :saving="savingKey" />
                </details>
              </div>
              <template v-else-if="active === tab.key">
                <BasicSettings v-if="tab.key === 'basic'" :data="data" :saving="savingKey" @save="save" />
                <div v-else-if="tab.key === 'feedback'" class="settings-tab-content">
                  <FeedbackSettings :data="data" :saving="savingKey" @save="save" />
                </div>
              </template>
            </div>
          </template>
        </VCard>
      </main>

      <aside v-if="previewOpen" class="preview-panel">
        <div class="preview-panel-header">
          <span class="preview-focus-label"
            ><Icon icon="ri:eye-line" />{{ previewFocus ? `预览：${previewFocus.label}` : "当前设置无固定预览" }}</span
          >
          <div class="preview-panel-actions">
            <button
              type="button"
              :aria-pressed="previewViewport === 'desktop'"
              :class="{ active: previewViewport === 'desktop' }"
              @click="previewViewport = 'desktop'"
            >
              桌面
            </button>
            <button
              type="button"
              :aria-pressed="previewViewport === 'tablet'"
              :class="{ active: previewViewport === 'tablet' }"
              @click="previewViewport = 'tablet'"
            >
              平板
            </button>
            <button
              type="button"
              :aria-pressed="previewViewport === 'mobile'"
              :class="{ active: previewViewport === 'mobile' }"
              @click="previewViewport = 'mobile'"
            >
              手机
            </button>
            <button type="button" aria-label="刷新预览" title="刷新预览" @click="refreshPreview">
              <Icon icon="ri:refresh-line" />
            </button>
            <button type="button" aria-label="关闭预览" title="关闭预览" @click="previewOpen = false">
              <Icon icon="ri:close-line" />
            </button>
          </div>
        </div>
        <div class="preview-context">
          <select v-model="previewPage" class="apple-select" aria-label="预览页面">
            <option v-for="page in settingsPreviewPages" :key="page.value" :value="page.value">{{ page.label }}</option>
          </select>
          <p>当前草稿 · 尚未保存到前台。静态预览不执行脚本，交互功能请保存后到前台检查；没有对应内容时显示列表页。</p>
        </div>
        <div ref="previewBody" class="preview-panel-body" :aria-busy="previewLoading">
          <div v-if="!previewFocus" class="preview-loading">
            <Icon icon="ri:information-line" width="20" height="20" /><span
              >当前设置项没有固定视觉预览，保存后请在前台查看效果。</span
            >
          </div>
          <div v-else-if="previewLoading" class="preview-loading" role="status">
            <Icon icon="ri:loader-4-line" class="settings-spin" width="22" height="22" /><span>生成预览中...</span>
          </div>
          <div v-else-if="previewError" class="preview-loading preview-error" role="alert">
            <Icon icon="ri:error-warning-line" width="22" height="22" />
            <span>{{ previewError }}</span>
            <VButton size="sm" type="secondary" @click="refreshPreview">重试预览</VButton>
          </div>
          <iframe
            v-else-if="previewHtml"
            :srcdoc="previewHtml"
            title="前台设置预览"
            class="preview-frame"
            sandbox=""
            :style="previewFrameStyle"
          />
          <div v-else class="preview-loading">暂无预览</div>
        </div>
      </aside>
    </div>
  </div>
</template>

<style scoped>
.settings-raw-editor > summary {
  cursor: pointer;
  padding: 24px 28px;
  color: var(--color-text-secondary);
  font-size: 14px;
}
@media (max-width: 767px) {
  .settings-raw-editor > summary {
    padding-left: 16px;
    padding-right: 16px;
  }
}

.settings-view {
  min-height: 100%;
  background: var(--color-background);
}
.settings-container {
  display: flex;
  flex-direction: column;
  gap: 18px;
  width: 100%;
  max-width: 1100px;
  margin: 0 auto;
  padding: 24px;
  box-sizing: border-box;
}
.settings-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}
.settings-basic-panel {
  width: 100%;
  overflow: visible;
}
.settings-basic-panel :deep(.card-wrapper) {
  border-radius: var(--radius-md);
  overflow: visible;
}
.settings-basic-panel :deep(.card-header) {
  padding: 0 16px;
  background: transparent;
  border-bottom: 1px solid var(--color-border);
  border-top-left-radius: calc(var(--radius-md) - 1px);
  border-top-right-radius: calc(var(--radius-md) - 1px);
}
.settings-basic-panel :deep(.card-body) {
  padding: 0;
  overflow: visible;
  border-bottom-left-radius: calc(var(--radius-md) - 1px);
  border-bottom-right-radius: calc(var(--radius-md) - 1px);
}
.settings-basic-panel :deep(.settings-basic-form),
.settings-basic-panel :deep(.settings-post-form),
.settings-basic-panel :deep(.settings-comments-form),
.settings-basic-panel :deep(.settings-seo-form),
.settings-basic-panel :deep(.settings-search-form),
.settings-basic-panel :deep(.settings-advanced-form) {
  border-bottom-left-radius: calc(var(--radius-md) - 1px);
  border-bottom-right-radius: calc(var(--radius-md) - 1px);
}
.settings-basic-panel :deep(.settings-basic-actions),
.settings-basic-panel :deep(.settings-post-actions),
.settings-basic-panel :deep(.settings-comments-actions),
.settings-basic-panel :deep(.settings-seo-actions),
.settings-basic-panel :deep(.settings-search-actions) {
  border-bottom-left-radius: calc(var(--radius-md) - 1px);
  border-bottom-right-radius: calc(var(--radius-md) - 1px);
}
.settings-dirty-badge {
  display: inline-flex;
  align-items: center;
  margin-left: 8px;
  color: var(--color-state-warning, #ff9f0a);
  font-size: 12px;
  font-weight: 500;
}
.settings-tab-content :deep(.card-wrapper) {
  border: none;
  border-radius: 0;
  box-shadow: none;
  background: transparent;
}
.settings-tab-content :deep(.card-wrapper::before) {
  display: none;
}
.settings-tab-content :deep(.card-header) {
  padding: 24px 28px 16px;
}
.settings-tab-content :deep(.card-body) {
  padding: 0 28px 28px;
}
.canonical-settings-status {
  padding: 0 28px;
}
.canonical-settings-status:has([role="status"]),
.canonical-settings-status:has([role="alert"]) {
  padding-top: 24px;
  padding-bottom: 12px;
}
@media (max-width: 767px) {
  .canonical-settings-status {
    padding-left: 16px;
    padding-right: 16px;
  }
}
.settings-card-tabs {
  width: 100%;
  min-width: 0;
  overflow: hidden;
}
.settings-card-tabbar :deep(.tabbar-item) {
  min-height: 44px;
  padding: 10px 12px;
  font-size: 13px;
}
.settings-card-tabbar {
  border-bottom: none;
}
.settings-card-tabs :deep(.tabbar-autoreveal) {
  flex-wrap: nowrap;
}
.settings-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 60px 0;
  color: var(--color-text-tertiary);
  font-size: 13px;
}
.settings-state-error {
  color: var(--color-state-error);
}
.settings-spin {
  animation: settings-spin 1s linear infinite;
}
@keyframes settings-spin {
  to {
    transform: rotate(360deg);
  }
}
.preview-panel {
  position: sticky;
  bottom: 0;
  z-index: 30;
  display: flex;
  flex-direction: column;
  height: 56vh;
  min-height: 320px;
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-bottom: none;
  border-radius: 12px 12px 0 0;
  background: var(--color-card);
  box-shadow: var(--shadow-lg);
}
.preview-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--color-border);
}
.preview-focus-label {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--color-text-secondary);
  font-size: 13px;
  font-weight: 500;
}
.preview-panel-actions {
  display: flex;
  align-items: center;
  gap: 4px;
}
.preview-panel-actions button {
  min-height: 44px;
  padding: 0 8px;
  border: 0;
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--color-text-tertiary);
  cursor: pointer;
}
.preview-panel-actions button.active,
.preview-panel-actions button:hover {
  background: var(--color-accent);
  color: var(--color-foreground);
}
.preview-panel-body {
  position: relative;
  flex: 1;
  min-height: 0;
  overflow: hidden;
  background: var(--color-background);
}
.preview-frame {
  display: block;
  max-width: none;
  transform-origin: top left;
  border: 0;
  background: var(--color-background);
}
.preview-loading {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--color-text-tertiary);
  font-size: 13px;
  text-align: center;
  padding: 20px;
}
.preview-context {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--color-border);
}
.preview-context select {
  width: 180px;
  min-height: 40px;
  flex-shrink: 0;
}
.preview-context p {
  margin: 0;
  color: var(--color-text-tertiary);
  font-size: 11px;
  line-height: 1.5;
}
.preview-error {
  color: var(--color-state-error);
}
.preview-panel-actions button:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}
@media (max-width: 640px) {
  .settings-container {
    padding: 12px;
    gap: 12px;
  }
  .settings-section-title {
    font-size: 18px;
  }
  .settings-basic-panel :deep(.tabbar-default .tabbar-item) {
    padding: 0 16px;
  }
  .preview-panel {
    height: 65vh;
  }
  .preview-panel-header {
    align-items: flex-start;
    flex-direction: column;
  }
  .preview-context {
    align-items: stretch;
    flex-direction: column;
    gap: 6px;
  }
  .preview-context select {
    width: 100%;
  }
}
</style>
