<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { Icon } from "@iconify/vue";
import { getApiErrorMessage, settingsApi } from "@/api";
import { Toast, VButton, VCard, VPageHeader } from "@/components/ui";
import AppearanceTab from "@/modules/settings/AppearanceTab.vue";
import CoverTab from "@/modules/settings/CoverTab.vue";
import NavTabsTab from "@/modules/settings/NavTabsTab.vue";
import ThemeYamlTab from "@/modules/settings/ThemeYamlTab.vue";
import LayoutSettings from "./LayoutSettings.vue";
import EnhancementsSettings from "./EnhancementsSettings.vue";
import AssetsSettings from "./AssetsSettings.vue";
import { ensureSettingsDefaults } from "@/modules/settings/settings-state";
import "@/modules/settings/settings-form.css";

const tabs = [
  { key: "appearance", label: "外观", description: "主题色、字体、背景与组件视觉风格" },
  { key: "home", label: "首页", description: "首页 Hero 与功能入口" },
  { key: "layout", label: "布局", description: "右栏、页脚与组件编排" },
  { key: "navigation", label: "导航", description: "主题特有的导航标签" },
  { key: "enhancements", label: "内容增强", description: "前端增强与内容组件默认参数" },
  { key: "assets", label: "素材", description: "主题默认图片和占位资源" },
  { key: "advanced", label: "高级", description: "直接编辑主题 YAML 配置" },
] as const;
const tabKeys = new Set<string>(tabs.map((tab) => tab.key));
const route = useRoute();
const router = useRouter();
const active = ref("appearance");
const data = ref<Record<string, any>>({});
const loading = ref(true);
const loadError = ref("");
const savingKey = ref("");
const currentTab = computed(() => tabs.find((tab) => tab.key === active.value) || tabs[0]);

async function load() {
  loading.value = true;
  loadError.value = "";
  try {
    const { data: response } = await settingsApi.all();
    data.value = ensureSettingsDefaults({ ...response });
  } catch {
    loadError.value = "加载外观设置失败";
  } finally {
    loading.value = false;
  }
}

async function save(section: string) {
  savingKey.value = section;
  try {
    await settingsApi.updateSection(section, data.value[section]);
    Toast.success("设置已保存，前台已生效");
  } catch (error) {
    Toast.error(getApiErrorMessage(error, "保存失败，请检查参数或网络后重试"));
  } finally {
    savingKey.value = "";
  }
}

function applyRouteTab() {
  const queryTab = route.query.tab;
  active.value = typeof queryTab === "string" && tabKeys.has(queryTab) ? queryTab : "appearance";
}

watch(active, (tab) => {
  if (route.query.tab !== tab) void router.push({ query: { ...route.query, tab } });
});
watch(() => route.query.tab, applyRouteTab);
onMounted(() => {
  applyRouteTab();
  void load();
});
</script>

<template>
  <div class="appearance-page">
    <VPageHeader title="主题设置">
      <template #icon><Icon icon="ri:palette-line" class="text-primary" /></template>
      <template #subtitle>管理主题外观、首页、布局、导航与前端增强</template>
    </VPageHeader>
    <main class="appearance-container">
      <div class="appearance-heading"><div class="appearance-title">{{ currentTab.label }}</div><div class="appearance-desc">{{ currentTab.description }}</div></div>
      <nav class="appearance-tabs apple-segmented rect" aria-label="主题设置分类">
        <button v-for="tab in tabs" :key="tab.key" type="button" :class="{ active: active === tab.key }" @click="active = tab.key">{{ tab.label }}</button>
      </nav>
      <div v-if="loading" class="appearance-state"><Icon icon="ri:loader-4-line" class="appearance-spin" width="22" height="22" />加载中...</div>
      <div v-else-if="loadError" class="appearance-state appearance-error"><Icon icon="ri:error-warning-line" width="22" height="22" />{{ loadError }}<VButton size="sm" type="secondary" @click="load">重试</VButton></div>
      <template v-else>
        <VCard v-if="active === 'appearance'" title="外观">
          <AppearanceTab :data="data" :saving="savingKey" @save="save" />
        </VCard>
        <VCard v-else-if="active === 'home'" title="首页 Hero">
          <CoverTab :data="data.cover" :saving="savingKey" @save="save('cover')" />
        </VCard>
        <LayoutSettings v-else-if="active === 'layout'" :data="data" :saving="savingKey" @save="save" />
        <VCard v-else-if="active === 'navigation'" title="导航标签">
          <NavTabsTab :data="data.nav_tabs" :saving="savingKey" @save="save('nav_tabs')" />
        </VCard>
        <EnhancementsSettings v-else-if="active === 'enhancements'" :data="data" :saving="savingKey" @save="save" />
        <AssetsSettings v-else-if="active === 'assets'" :data="data" :saving="savingKey" @save="save" />
        <VCard v-else title="主题 YAML"><ThemeYamlTab /></VCard>
      </template>
    </main>
  </div>
</template>

<style scoped>
.appearance-page { min-height: 100%; background: var(--color-background); }
.appearance-container { display: flex; flex-direction: column; gap: 16px; width: 100%; max-width: 1100px; margin: 0 auto; padding: 24px; box-sizing: border-box; }
.appearance-heading { display: flex; flex-direction: column; gap: 4px; }
.appearance-title { color: var(--color-foreground); font-size: 22px; font-weight: 600; }
.appearance-desc { color: var(--color-text-tertiary); font-size: 13px; }
.appearance-tabs { display: flex; overflow-x: auto; flex-wrap: nowrap; scrollbar-width: none; }
.appearance-tabs::-webkit-scrollbar { display: none; }
.appearance-tabs button { min-height: 44px; white-space: nowrap; }
.appearance-state { display: flex; flex-direction: column; align-items: center; gap: 10px; padding: 60px 0; color: var(--color-text-tertiary); font-size: 13px; }
.appearance-error { color: var(--color-state-error); }
.appearance-spin { animation: appearance-spin 1s linear infinite; }
@keyframes appearance-spin { to { transform: rotate(360deg); } }
@media (max-width: 640px) { .appearance-container { padding: 12px; } .appearance-title { font-size: 18px; } }
</style>
