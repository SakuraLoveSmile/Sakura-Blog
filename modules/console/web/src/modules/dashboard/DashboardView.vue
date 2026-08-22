<script setup lang="ts">
import { computed, onMounted } from "vue";
import { Icon } from "@iconify/vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import DashboardGrid from "./DashboardGrid.vue";
import { useDashboardLayout } from "./useDashboardLayout";
import { provideBreakpoint, type ViewMode } from "./useBreakpoint";
import "./dashboard.css";

const router = useRouter();
const authStore = useAuthStore();
const { breakpoint, viewMode, previewWidth, isPreview, setViewMode } = provideBreakpoint();
const { layout, loading, error, load } = useDashboardLayout();

const displayName = computed(() => authStore.user?.display_name || authStore.user?.username || "管理员");

function greeting(): string {
  const hour = new Date().getHours();
  if (hour < 6) return "夜深了";
  if (hour < 12) return "早上好";
  if (hour < 14) return "中午好";
  if (hour < 18) return "下午好";
  return "晚上好";
}

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

onMounted(load);
</script>

<template>
  <div class="dashboard-viewport" :class="{ 'is-preview': isPreview }">
    <div class="dashboard-canvas" :style="{ width: previewWidth }">
      <div class="dashboard-apple" :data-bp="breakpoint">
        <div class="flex flex-col sm:flex-row sm:items-end justify-between gap-4 apple-greeting">
          <div>
            <h1>{{ greeting() }}，{{ displayName }}</h1>
            <p>{{ today() }}，欢迎回到管理仪表盘</p>
          </div>
          <div class="apple-header-actions">
            <div class="apple-segmented view-switch">
              <button
                v-for="mode in viewModes"
                :key="mode.key"
                :class="['seg-btn', mode.key === viewMode && 'active']"
                :title="mode.label"
                :aria-label="mode.label"
                @click="setViewMode(mode.key)"
              >
                <Icon :icon="mode.icon" class="w-4 h-4" />
              </button>
            </div>
            <button class="apple-edit-btn" @click="router.push('/dashboard/designer')">
              <Icon icon="ri:settings-3-line" class="w-4 h-4" />
              设置布局
            </button>
          </div>
        </div>

        <div v-if="loading" class="apple-grid-empty">
          <Icon icon="ri:loader-4-line" class="w-5 h-5 animate-spin" />
          正在加载布局
        </div>
        <div v-else-if="error" class="apple-grid-empty text-state-error">
          {{ error }}
        </div>
        <DashboardGrid
          v-else
          :widgets="layout[breakpoint]"
          :breakpoint="breakpoint"
          :editing="false"
        />
      </div>
    </div>
  </div>
</template>
