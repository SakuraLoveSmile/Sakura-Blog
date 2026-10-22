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

const displayName = computed(() => authStore.user?.display_name || authStore.user?.username || "管理员");

const greetingInfo = computed(() => {
  const hour = new Date().getHours();
  if (hour < 6) return { text: "夜深了", icon: "ri:moon-clear-line" };
  if (hour < 12) return { text: "早上好", icon: "ri:sun-foggy-line" };
  if (hour < 14) return { text: "中午好", icon: "ri:sun-line" };
  if (hour < 18) return { text: "下午好", icon: "ri:sun-cloudy-line" };
  if (hour < 22) return { text: "晚上好", icon: "ri:moon-foggy-line" };
  return { text: "夜深了", icon: "ri:moon-clear-line" };
});

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
        <div class="flex flex-col sm:flex-row sm:items-end justify-between gap-4 apple-greeting">
          <div>
            <h1 class="flex items-center gap-2.5">
              <span class="greeting-icon" aria-hidden="true"><Icon :icon="greetingInfo.icon" class="w-6 h-6" /></span>
              <span>{{ greetingInfo.text }}，{{ displayName }}</span>
            </h1>
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

        <section
          v-if="hasPosts"
          aria-label="写作快捷入口"
          class="mb-5 flex flex-wrap items-center justify-between gap-4 rounded-2xl border border-border bg-card px-5 py-4"
        >
          <div>
            <h2 class="text-base font-semibold text-foreground">今天，写点什么？</h2>
            <p class="mt-1 text-sm text-text-tertiary">开始一篇新文章，或回到草稿接着写。</p>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <VButton :route="{ path: '/posts', query: { status: 'draft' } }" class="min-h-11">
              <template #icon><Icon icon="ri:draft-line" /></template>
              继续写草稿
            </VButton>
            <VButton v-if="canWritePost" type="primary" :route="{ path: '/posts/new' }" class="min-h-11">
              <template #icon><Icon icon="ri:quill-pen-line" /></template>
              撰写文章
            </VButton>
          </div>
        </section>

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
