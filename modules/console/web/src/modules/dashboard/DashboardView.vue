<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import { useAuthStore } from "@/stores/auth";
import { VButton } from "@/components/ui";
import { useModulesStore } from "@/stores/modules";
import DashboardGrid from "./DashboardGrid.vue";
import { useDashboardLayout } from "./useDashboardLayout";
import { buildDashboardQuickLinks } from "./dashboard-quick-links";
import { provideBreakpoint, VIEW_MODE_WIDTH, type ViewMode } from "./useBreakpoint";
import { DASHBOARD_REFRESH_EVENT } from "./widgets/useDashboardStats";
import "./dashboard.css";

const authStore = useAuthStore();
const modulesStore = useModulesStore();
const quickLinks = computed(() =>
  buildDashboardQuickLinks(modulesStore.enabledNav(), (permission) => authStore.hasPermission(permission)),
);
const hasPosts = computed(() => modulesStore.enabledNav().some((item) => item.path === "/posts"));
const canWritePost = computed(() => hasPosts.value && authStore.hasPermission("content:write"));
const { breakpoint, viewMode, previewWidth, isPreview, setViewMode } = provideBreakpoint();
const { layout, loading, error, load } = useDashboardLayout();
const refreshing = ref(false);

function today(): string {
  const date = new Date();
  const weekdays = ["周日", "周一", "周二", "周三", "周四", "周五", "周六"];
  return `${date.getFullYear()}年${date.getMonth() + 1}月${date.getDate()}日 ${weekdays[date.getDay()]}`;
}

// hint 是悬停/聚焦提示的第二行：预览宽度直接取 VIEW_MODE_WIDTH，不另写一份数字。
const viewModes: { key: ViewMode; label: string; icon: string; hint: string }[] = [
  { key: "auto", label: "自适应", icon: "ri:aspect-ratio-line", hint: "跟随窗口宽度" },
  { key: "lg", label: "电脑", icon: "ri:computer-line", hint: `预览宽度 ${VIEW_MODE_WIDTH.lg}px` },
  { key: "sm", label: "平板", icon: "ri:tablet-line", hint: `预览宽度 ${VIEW_MODE_WIDTH.sm}px` },
  { key: "xxs", label: "手机", icon: "ri:smartphone-line", hint: `预览宽度 ${VIEW_MODE_WIDTH.xxs}px` },
];

function refreshDashboard() {
  if (refreshing.value) return;
  refreshing.value = true;
  window.dispatchEvent(new Event(DASHBOARD_REFRESH_EVENT));
  window.setTimeout(() => {
    refreshing.value = false;
  }, 850);
}

onMounted(load);
</script>

<template>
  <div class="dashboard-viewport" :class="{ 'is-preview': isPreview }">
    <div class="dashboard-canvas" :style="{ width: previewWidth }">
      <div class="dashboard-apple" :data-bp="breakpoint">
        <div class="apple-greeting dashboard-heading">
          <div>
            <h1>仪表盘</h1>
            <p>{{ today() }}</p>
            <nav class="dashboard-quick-links" aria-label="快捷操作">
              <RouterLink v-for="link in quickLinks" :key="link.path" :to="link.path">{{ link.label }}</RouterLink>
            </nav>
          </div>
          <div class="apple-header-actions">
            <div v-if="hasPosts" class="dashboard-writing-actions" role="group" aria-label="写作快捷入口">
              <VButton :route="{ path: '/posts', query: { status: 'draft' } }">
                <template #icon><Icon icon="ri:draft-line" class="w-3.5 h-3.5" /></template>
                继续写草稿
              </VButton>
              <VButton v-if="canWritePost" type="primary" :route="{ path: '/posts/new' }">
                <template #icon><Icon icon="ri:add-line" class="w-3.5 h-3.5" /></template>
                撰写文章
              </VButton>
            </div>
            <div class="dashboard-display-actions" role="group" aria-label="仪表盘显示">
              <div class="dashboard-view-switch" role="group" aria-label="视图模式">
                <span v-for="mode in viewModes" :key="mode.key" class="dashboard-tool">
                  <button
                    type="button"
                    :class="['dashboard-tool-btn', mode.key === viewMode && 'is-active']"
                    :aria-label="mode.label"
                    :aria-pressed="mode.key === viewMode"
                    :aria-describedby="`dashboard-tip-${mode.key}`"
                    @click="setViewMode(mode.key)"
                  >
                    <Icon :icon="mode.icon" class="w-4 h-4" />
                  </button>
                  <span class="dashboard-tip" role="tooltip">
                    {{ mode.label }}
                    <small :id="`dashboard-tip-${mode.key}`">{{ mode.hint }}</small>
                  </span>
                </span>
              </div>
              <span class="dashboard-tool">
                <button
                  type="button"
                  class="dashboard-tool-btn dashboard-refresh-btn"
                  aria-label="刷新仪表盘数据"
                  :aria-busy="refreshing"
                  @click="refreshDashboard"
                >
                  <Icon
                    :icon="refreshing ? 'ri:loader-4-line' : 'ri:refresh-line'"
                    class="w-4 h-4"
                    :class="{ 'animate-spin': refreshing }"
                  />
                </button>
                <span class="dashboard-tip" role="tooltip">刷新仪表盘数据</span>
              </span>
              <VButton type="ghost" :route="{ path: '/dashboard/designer' }">
                <template #icon><Icon icon="ri:settings-3-line" class="w-3.5 h-3.5" /></template>
                设置布局
              </VButton>
            </div>
          </div>
        </div>

        <div v-if="loading" class="apple-grid-empty">
          <Icon icon="ri:loader-4-line" class="w-5 h-5 animate-spin" />
          正在加载布局
        </div>
        <div v-else-if="error" class="apple-grid-empty text-state-error">
          {{ error }}
        </div>
        <DashboardGrid v-else :widgets="layout[breakpoint]" :breakpoint="breakpoint" :editing="false" />
      </div>
    </div>
  </div>
</template>
