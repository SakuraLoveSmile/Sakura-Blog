<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { Icon } from "@iconify/vue";
import { getApiErrorMessage, settingsApi } from "@/api";
import { Toast, VButton, VCard, VPageHeader, VTabbar } from "@/components/ui";
import BasicSettings from "./tabs/BasicSettings.vue";
import PostSettings from "./tabs/PostSettings.vue";
import CommentSettings from "./tabs/CommentSettings.vue";
import SeoSettings from "./tabs/SeoSettings.vue";
import SearchSettings from "./tabs/SearchSettings.vue";
import AdvancedSettings from "./tabs/AdvancedSettings.vue";
import { ALL_SETTINGS_SECTIONS, ensureSettingsDefaults, sortWikiProjects } from "./settings-state";
import { useSettingsPreview } from "./useSettingsPreview";
import "./settings-form.css";

// Shared defaults retain the existing frontend enhancement safety boundary: card_hover.enable = false; card_hover.max_tilt = 3.

const tabs = [
  { key: "basic", label: "基本", description: "站点身份、标题与品牌资源" },
  { key: "post", label: "文章", description: "文章排版、卡片、版权、分享与打赏" },
  { key: "comments", label: "评论", description: "评论开关、审核与邮件通知" },
  { key: "seo", label: "SEO", description: "站点来源、社交卡片与搜索引擎设置" },
  { key: "search", label: "搜索", description: "本地索引范围与排除路径" },
  { key: "advanced", label: "高级", description: "代码注入、缓存与原始 JSON 兜底" },
] as const;
const tabItems = tabs.map((tab) => ({ id: tab.key, label: tab.label }));
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
  appearance: { path: "/appearance", tab: "appearance" },
  theme_yaml: { path: "/appearance", tab: "advanced" },
  cover: { path: "/appearance", tab: "home" },
  nav_tabs: { path: "/appearance", tab: "navigation" },
  widgets: { path: "/appearance", tab: "layout" },
  plugins: { path: "/appearance", tab: "enhancements" },
  tag_plugins: { path: "/appearance", tab: "enhancements" },
  default: { path: "/appearance", tab: "assets" },
  menubar: { path: "/menus" },
  rightbar: { path: "/appearance", tab: "layout" },
  footer: { path: "/appearance", tab: "layout" },
  ai: { path: "/assistant", tab: "settings" },
  security: { path: "/profile" },
  authors: { path: "/authors" },
};

const route = useRoute();
const router = useRouter();
const active = ref("basic");
const data = ref<Record<string, any>>({});
const rawText = ref<Record<string, string>>({});
const loading = ref(true);
const loadError = ref("");
const savingKey = ref("");
const currentTab = computed(() => tabs.find((tab) => tab.key === active.value) || tabs[0]);
const { previewOpen, previewViewport, previewHtml, previewLoading, previewFocus, refreshPreview } = useSettingsPreview(
  data,
  active,
  ALL_SETTINGS_SECTIONS,
);

function routeQuery(tab: string) {
  return { ...route.query, tab };
}

async function applyRouteQuery() {
  const queryTab = route.query.tab;
  if (typeof queryTab !== "string" || !queryTab) {
    active.value = "basic";
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
    for (const section of ALL_SETTINGS_SECTIONS)
      rawText.value[section] = JSON.stringify(response[section] ?? null, null, 2);
  } catch {
    loadError.value = "加载设置失败";
  } finally {
    loading.value = false;
  }
}

async function save(section: string, payload?: unknown): Promise<boolean> {
  savingKey.value = section;
  try {
    const body = payload !== undefined ? payload : data.value[section];
    await settingsApi.updateSection(section, body);
    rawText.value[section] = JSON.stringify(body, null, 2);
    Toast.success("设置已保存，前台已生效");
    return true;
  } catch (error) {
    Toast.error(getApiErrorMessage(error, "保存失败，请检查参数或网络后重试"));
    return false;
  } finally {
    savingKey.value = "";
  }
}

async function saveComments() {
  const commentsSaved = await save("comments");
  if (commentsSaved) await save("site_tree");
}

async function saveJson(section: string) {
  savingKey.value = "json:" + section;
  try {
    const payload = JSON.parse(rawText.value[section]);
    await settingsApi.updateSection(section, payload);
    data.value[section] = payload;
    ensureSettingsDefaults(data.value);
    Toast.success("设置已保存，前台已生效");
  } catch (error) {
    Toast.error(getApiErrorMessage(error, "JSON 解析或保存失败"));
  } finally {
    savingKey.value = "";
  }
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

onMounted(() => {
  void applyRouteQuery();
  void load();
});
</script>

<template>
  <div class="settings-view" :class="{ 'preview-docked': previewOpen }">
    <VPageHeader title="系统设置">
      <template #icon><Icon icon="ri:settings-3-line" class="text-primary" /></template>
      <template #subtitle>基本、文章、评论、SEO、搜索与高级系统配置</template>
      <template #actions>
        <VButton size="sm" :type="previewOpen ? 'primary' : 'secondary'" @click="previewOpen = !previewOpen">
          <template #icon><Icon icon="ri:eye-line" /></template><span>前端预览</span>
        </VButton>
      </template>
    </VPageHeader>

    <div class="settings-container">
      <main class="settings-content">
        <template v-if="active === 'basic'">
          <VCard class="settings-basic-panel">
            <template #header>
              <div class="settings-card-tabs">
                <VTabbar
                  v-model:active-id="active"
                  :items="tabItems"
                  type="default"
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
            <BasicSettings v-else :data="data" :saving="savingKey" @save="save" />
          </VCard>
        </template>

        <template v-else>
          <div class="settings-heading">
            <div class="settings-section-title">{{ currentTab.label }}</div>
            <div class="settings-section-desc">{{ currentTab.description }}</div>
          </div>
          <nav class="settings-tabs apple-segmented rect" aria-label="系统设置分类">
            <button
              v-for="tab in tabs"
              :key="tab.key"
              type="button"
              :class="{ active: active === tab.key }"
              @click="navigateToTab(tab.key)"
            >
              {{ tab.label }}
            </button>
          </nav>

          <div v-if="loading" class="settings-state">
            <Icon icon="ri:loader-4-line" class="settings-spin" width="22" height="22" /><span>加载中...</span>
          </div>
          <div v-else-if="loadError" class="settings-state settings-state-error">
            <Icon icon="ri:error-warning-line" width="22" height="22" /><span>设置加载失败：{{ loadError }}</span
            ><VButton type="secondary" size="sm" @click="load">重试</VButton>
          </div>
          <template v-else>
            <PostSettings v-if="active === 'post'" :data="data" :saving="savingKey" @save="save" />
            <CommentSettings v-else-if="active === 'comments'" :data="data" :saving="savingKey" @save="saveComments" />
            <SeoSettings v-else-if="active === 'seo'" :data="data" :saving="savingKey" @save="save" />
            <SearchSettings v-else-if="active === 'search'" :data="data" :saving="savingKey" @save="save" />
            <AdvancedSettings
              v-else
              :data="data"
              :raw-text="rawText"
              :saving="savingKey"
              @save="save"
              @save-json="saveJson"
            />
          </template>
        </template>
      </main>

      <aside v-if="previewOpen" class="preview-panel">
        <div class="preview-panel-header">
          <span class="preview-focus-label"
            ><Icon icon="ri:eye-line" />{{ previewFocus ? `预览：${previewFocus.label}` : "当前设置无固定预览" }}</span
          >
          <div class="preview-panel-actions">
            <button
              type="button"
              :class="{ active: previewViewport === 'desktop' }"
              @click="previewViewport = 'desktop'"
            >
              桌面
            </button>
            <button type="button" :class="{ active: previewViewport === 'tablet' }" @click="previewViewport = 'tablet'">
              平板
            </button>
            <button type="button" :class="{ active: previewViewport === 'mobile' }" @click="previewViewport = 'mobile'">
              手机
            </button>
            <button type="button" title="刷新预览" @click="refreshPreview"><Icon icon="ri:refresh-line" /></button>
            <button type="button" title="关闭预览" @click="previewOpen = false"><Icon icon="ri:close-line" /></button>
          </div>
        </div>
        <div class="preview-panel-body">
          <div v-if="!previewFocus" class="preview-loading">
            <Icon icon="ri:information-line" width="20" height="20" /><span
              >当前设置项没有固定视觉预览，保存后请在前台查看效果。</span
            >
          </div>
          <div v-else-if="previewLoading" class="preview-loading">
            <Icon icon="ri:loader-4-line" class="settings-spin" width="22" height="22" /><span>生成预览中...</span>
          </div>
          <iframe
            v-else-if="previewHtml"
            :srcdoc="previewHtml"
            title="前台设置预览"
            class="preview-frame"
            :style="{
              maxWidth: previewViewport === 'mobile' ? '390px' : previewViewport === 'tablet' ? '768px' : '100%',
              margin: '0 auto',
              display: 'block',
            }"
          />
          <div v-else class="preview-loading">暂无预览</div>
        </div>
      </aside>
    </div>
  </div>
</template>

<style scoped>
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
.settings-basic-panel :deep(.card-header) {
  padding: 0;
}
.settings-basic-panel :deep(.card-body) {
  padding: 0;
  overflow: visible;
}
.settings-card-tabs {
  width: 100%;
  min-width: 0;
  overflow: hidden;
}
.settings-basic-panel :deep(.tabbar-default) {
  flex-wrap: nowrap;
  overflow-x: auto;
  padding: 4px 6px;
  border-bottom: 0;
  scrollbar-color: var(--color-border) transparent;
  scrollbar-width: thin;
}
.settings-basic-panel :deep(.tabbar-default .tabbar-item) {
  flex: 0 0 auto;
  min-height: 48px;
  padding: 0 20px;
  border-bottom: 0;
  border-radius: var(--radius-md);
  font-size: 14px;
}
.settings-basic-panel :deep(.tabbar-default .tabbar-item:hover) {
  background: var(--color-secondary);
}
.settings-basic-panel :deep(.tabbar-default .tabbar-item.active) {
  background: var(--color-accent);
  box-shadow: var(--shadow-sm);
}
.settings-basic-panel :deep(.tabbar-default .tabbar-item:focus-visible) {
  outline: 2px solid var(--color-primary);
  outline-offset: -2px;
}
.settings-heading {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.settings-section-title {
  color: var(--color-foreground);
  font-size: 22px;
  font-weight: 600;
  letter-spacing: -0.3px;
}
.settings-section-desc {
  color: var(--color-text-tertiary);
  font-size: 13px;
}
.settings-tabs {
  align-self: stretch;
  display: flex;
  overflow-x: auto;
  flex-wrap: nowrap;
  scrollbar-width: none;
}
.settings-tabs::-webkit-scrollbar {
  display: none;
}
.settings-tabs button {
  min-height: 44px;
  white-space: nowrap;
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
  height: 45vh;
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-bottom: none;
  border-radius: 12px 12px 0 0;
  background: var(--color-card);
  box-shadow: 0 -8px 24px rgba(0, 0, 0, 0.35);
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
  min-height: 32px;
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
}
.preview-frame {
  width: 100%;
  height: 100%;
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
    height: 55vh;
  }
  .preview-panel-header {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
