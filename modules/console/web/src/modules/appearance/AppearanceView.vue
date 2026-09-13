<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref, watch } from "vue";
import { onBeforeRouteLeave, useRoute, useRouter } from "vue-router";
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
  { key: "advanced", label: "高级", description: "直接编辑主题样式 YAML 配置" },
] as const;

const tabSectionsMap: Record<string, string[]> = {
  appearance: ["style"],
  home: ["cover"],
  layout: ["rightbar", "footer", "widgets"],
  navigation: ["nav_tabs"],
  enhancements: ["plugins", "tag_plugins", "services"],
  assets: ["default"],
  advanced: [],
};

const ALL_APPEARANCE_SECTIONS = [
  "style",
  "cover",
  "rightbar",
  "footer",
  "widgets",
  "nav_tabs",
  "plugins",
  "tag_plugins",
  "services",
  "default",
];

const tabKeys = new Set<string>(tabs.map((tab) => tab.key));
const route = useRoute();
const router = useRouter();
const active = ref("appearance");
const data = ref<Record<string, any>>({});
const loading = ref(true);
const loadError = ref("");
const savingKey = ref("");
const savedSnapshot = ref<string>("");
const currentTab = computed(() => tabs.find((tab) => tab.key === active.value) || tabs[0]);

function takeSnapshot() {
  savedSnapshot.value = JSON.stringify(data.value);
}

function updateSectionSnapshot(section: string, savedData: any) {
  try {
    const snap = savedSnapshot.value ? JSON.parse(savedSnapshot.value) : {};
    snap[section] = JSON.parse(JSON.stringify(savedData));
    savedSnapshot.value = JSON.stringify(snap);
  } catch {}
}

const dirtySections = computed<Set<string>>(() => {
  if (loading.value || !savedSnapshot.value) return new Set();
  const set = new Set<string>();
  try {
    const original = JSON.parse(savedSnapshot.value);
    for (const key of ALL_APPEARANCE_SECTIONS) {
      if (JSON.stringify(data.value[key]) !== JSON.stringify(original[key])) {
        set.add(key);
      }
    }
  } catch {}
  return set;
});

const isTabDirty = (tabKey: string) => {
  const sections = tabSectionsMap[tabKey] || [];
  return sections.some((s) => dirtySections.value.has(s));
};

const isDirty = computed(() => dirtySections.value.size > 0);

async function load() {
  loading.value = true;
  loadError.value = "";
  try {
    const { data: response } = await settingsApi.all();
    data.value = ensureSettingsDefaults({ ...response });
    takeSnapshot();
  } catch {
    loadError.value = "加载外观设置失败";
  } finally {
    loading.value = false;
  }
}

async function save(section: string) {
  if (savingKey.value) return;
  savingKey.value = section;
  const currentPayload = data.value[section];
  try {
    await settingsApi.updateSection(section, currentPayload);
    updateSectionSnapshot(section, currentPayload);
    Toast.success("设置已保存，前台已生效");
  } catch (error) {
    Toast.error(getApiErrorMessage(error, "保存失败，请检查参数或网络后重试"));
  } finally {
    savingKey.value = "";
  }
}

function handleBeforeUnload(e: BeforeUnloadEvent) {
  if (isDirty.value) {
    e.preventDefault();
    e.returnValue = "";
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
  window.addEventListener("beforeunload", handleBeforeUnload);
});

onBeforeUnmount(() => {
  window.removeEventListener("beforeunload", handleBeforeUnload);
});

onBeforeRouteLeave((to, from, next) => {
  if (to.path === from.path) {
    next();
    return;
  }
  if (isDirty.value) {
    const confirmed = window.confirm("您有未保存的主题设置更改，离开后修改将丢失，是否确认离开？");
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
  <div class="appearance-page">
    <VPageHeader title="主题设置">
      <template #icon><Icon icon="ri:palette-line" class="text-primary" /></template>
      <template #subtitle>
        <span>管理主题外观、首页、布局、导航与前端增强</span>
        <span v-if="isDirty" class="settings-dirty-badge">（有未保存修改）</span>
      </template>
    </VPageHeader>
    <main class="appearance-container">
      <div class="appearance-heading"><div class="appearance-title">{{ currentTab.label }}</div><div class="appearance-desc">{{ currentTab.description }}</div></div>
      <nav class="appearance-tabs apple-segmented rect" aria-label="主题设置分类">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          type="button"
          :class="{ active: active === tab.key }"
          @click="active = tab.key"
        >
          <span>{{ tab.label }}</span>
          <span v-if="isTabDirty(tab.key)" class="settings-tab-dirty-dot" title="有未保存修改" aria-label="有未保存修改" />
        </button>
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
        <VCard v-else title="样式 YAML"><ThemeYamlTab /></VCard>
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
.appearance-tabs button { min-height: 44px; white-space: nowrap; outline: none; }
.appearance-tabs button:focus-visible {
  outline: 2px solid var(--color-ring, var(--color-primary, #2e8dff));
  outline-offset: 2px;
}
.appearance-state { display: flex; flex-direction: column; align-items: center; gap: 10px; padding: 60px 0; color: var(--color-text-tertiary); font-size: 13px; }
.appearance-error { color: var(--color-state-error); }
.appearance-spin { animation: appearance-spin 1s linear infinite; }
@keyframes appearance-spin { to { transform: rotate(360deg); } }
@media (max-width: 640px) { .appearance-container { padding: 12px; } .appearance-title { font-size: 18px; } }
</style>
