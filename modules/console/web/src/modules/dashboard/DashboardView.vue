<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { VButton } from "@/components/ui";
import { useModulesStore } from "@/stores/modules";
import DashboardGrid from "./DashboardGrid.vue";
import { useDashboardLayout } from "./useDashboardLayout";
import { provideBreakpoint, type ViewMode } from "./useBreakpoint";
import { DASHBOARD_REFRESH_EVENT } from "./widgets/useDashboardStats";
import "./dashboard.css";

const router = useRouter();
const authStore = useAuthStore();
const modulesStore = useModulesStore();
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

const viewModes: { key: ViewMode; label: string; icon: string }[] = [
  { key: "auto", label: "自适应", icon: "ri:aspect-ratio-line" },
  { key: "lg", label: "电脑", icon: "ri:computer-line" },
  { key: "sm", label: "平板", icon: "ri:tablet-line" },
  { key: "xxs", label: "手机", icon: "ri:smartphone-line" },
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
          </div>
          <div class="apple-header-actions">
            <div v-if="hasPosts" class="dashboard-writing-actions" role="group" aria-label="写作快捷入口">
              <VButton :route="{ path: '/posts', query: { status: 'draft' } }">
                <template #icon><Icon icon="ri:draft-line" /></template>
                继续写草稿
              </VButton>
              <VButton v-if="canWritePost" type="primary" :route="{ path: '/posts/new' }">
                <template #icon><Icon icon="ri:add-line" /></template>
                撰写文章
              </VButton>
            </div>
            <div class="dashboard-display-actions" role="group" aria-label="仪表盘显示">
              <div class="apple-segmented view-switch">
                <button
                  v-for="mode in viewModes"
                  :key="mode.key"
                  :class="['seg-btn', mode.key === viewMode && 'active']"
                  :title="mode.label"
                  :aria-label="mode.label"
                  :aria-pressed="mode.key === viewMode"
                  @click="setViewMode(mode.key)"
                >
                  <Icon :icon="mode.icon" class="w-4 h-4" />
                </button>
              </div>
              <button
                class="apple-icon-btn dashboard-refresh-btn"
                type="button"
                title="刷新仪表盘数据"
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
              <button class="apple-edit-btn" @click="router.push('/dashboard/designer')">
                <Icon icon="ri:settings-3-line" class="w-4 h-4" />
                设置布局
              </button>
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
