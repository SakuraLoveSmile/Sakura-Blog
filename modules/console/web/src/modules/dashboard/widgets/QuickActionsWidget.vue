<script setup lang="ts">
import { Icon } from "@iconify/vue";
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useRouter } from "vue-router";
import { useBreakpoint } from "../useBreakpoint";
import { themeColors } from "@/lib/theme-colors";
import type { WidgetSizePreset } from "./layout";
import { useWidgetSize } from "../useWidgetSize";
import { DASHBOARD_REFRESH_EVENT } from "./useDashboardStats";
import { dashboardApi } from "@/api";

interface Props {
  sizePreset?: WidgetSizePreset;
}

const props = withDefaults(defineProps<Props>(), {
  sizePreset: "medium",
});
const sizePreset = useWidgetSize(props.sizePreset).preset;

const { breakpoint } = useBreakpoint();
const router = useRouter();
const colors = themeColors({
  primary: "#2E8DFF",
  success: "#30D158",
  warning: "#FF9F0A",
  chart5: "#BF5AF2",
  chart2: "#2e8dff",
  error: "#FF453A",
  primaryHover: "#66ABFF",
  primaryActive: "#007AFF",
});

interface QuickAction {
  key: string;
  label: string;
  desc?: string;
  icon: string;
  color: string;
  bgColor: string;
  path?: string;
  isExternal?: boolean;
}

const baseActions: QuickAction[] = [
  {
    key: "write_post",
    label: "写文章",
    desc: "发布新内容",
    icon: "ri:quill-pen-line",
    color: colors.primary,
    bgColor: "color-mix(in srgb, var(--color-primary) 14%, transparent)",
    path: "/posts/editor",
  },
  {
    key: "review_comments",
    label: "评论",
    desc: "互动审核",
    icon: "ri:chat-3-line",
    color: colors.success,
    bgColor: "color-mix(in srgb, var(--color-state-success) 14%, transparent)",
    path: "/comments",
  },
  {
    key: "manage_categories",
    label: "分类",
    desc: "组织结构",
    icon: "ri:folder-3-line",
    color: colors.warning,
    bgColor: "color-mix(in srgb, var(--color-state-warning) 14%, transparent)",
    path: "/categories",
  },
  {
    key: "settings",
    label: "设置",
    desc: "系统首选项",
    icon: "ri:settings-3-line",
    color: colors.chart5,
    bgColor: "color-mix(in srgb, var(--color-chart-5) 14%, transparent)",
    path: "/settings",
  },
];

const extendedActions: QuickAction[] = [
  ...baseActions,
  {
    key: "manage_posts",
    label: "文章列表",
    desc: "管理已有文章",
    icon: "ri:file-list-3-line",
    color: colors.primaryHover,
    bgColor: "color-mix(in srgb, var(--color-primary-hover) 14%, transparent)",
    path: "/posts",
  },
  {
    key: "upload_attachment",
    label: "媒体库",
    desc: "文件与图片",
    icon: "ri:attachment-2",
    color: colors.chart2,
    bgColor: "color-mix(in srgb, #2e8dff 14%, transparent)",
    path: "/attachments",
  },
  {
    key: "system_update",
    label: "系统更新",
    desc: "版本与升级",
    icon: "ri:rocket-2-line",
    color: colors.error,
    bgColor: "color-mix(in srgb, var(--color-state-error) 14%, transparent)",
    path: "/system-update",
  },
  {
    key: "open_frontend",
    label: "查看博客",
    desc: "打开前台页面",
    icon: "ri:external-link-line",
    color: colors.primaryActive,
    bgColor: "color-mix(in srgb, var(--color-primary-active) 14%, transparent)",
    path: "/",
    isExternal: true,
  },
];

const actionCatalog = [...extendedActions];
const remoteActions = ref<QuickAction[] | null>(null);

async function loadActions() {
  try {
    const response = await dashboardApi.quickActions();
    const keys = Array.isArray(response.data?.items) ? response.data.items : [];
    const resolved: QuickAction[] = keys.flatMap((key: unknown) => {
      const action = actionCatalog.find((candidate: QuickAction) => candidate.key === key);
      return action ? [action] : [];
    });
    remoteActions.value = resolved.length ? resolved : null;
  } catch {
    remoteActions.value = null;
  }
}

const displayedActions = computed(() => {
  const source = remoteActions.value ?? (sizePreset.value === "large" ? extendedActions : baseActions);
  if (sizePreset.value === "large") return source;
  return source.slice(0, 4);
});

function go(act: QuickAction) {
  if (!act.path) return;
  if (act.isExternal) {
    window.open(act.path, "_blank", "noopener,noreferrer");
  } else {
    router.push(act.path);
  }
}

const hoveredIdx = ref<number | null>(null);

onMounted(() => {
  loadActions();
  window.addEventListener(DASHBOARD_REFRESH_EVENT, loadActions);
});
onUnmounted(() => window.removeEventListener(DASHBOARD_REFRESH_EVENT, loadActions));
</script>

<template>
  <div class="apple-card h-full flex flex-col justify-between" :class="`quick-card-${sizePreset}`" :data-bp="breakpoint">
    <div class="flex items-center justify-between mb-3">
      <h3 class="apple-section-title" :class="{ 'text-sm': sizePreset === 'small' }">快捷操作</h3>
      <span class="text-xs text-muted-foreground">{{ displayedActions.length }} 个快捷入口</span>
    </div>

    <!-- Small 尺寸：极紧凑 4 小宫格 -->
    <div v-if="sizePreset === 'small'" class="grid grid-cols-2 gap-2 flex-1">
      <button
        v-for="act in displayedActions"
        :key="act.label"
        class="quick-action-item flex items-center gap-2 p-2 rounded-xl text-left"
        :class="{ 'quick-action-featured': act.label === '写文章' }"
        :aria-label="act.desc ? `${act.label}：${act.desc}` : act.label"
        @click="go(act)"
      >
        <div
          class="w-7 h-7 rounded-lg flex items-center justify-center flex-shrink-0"
          :style="{ backgroundColor: act.bgColor, color: act.color }"
        >
          <Icon :icon="act.icon" class="w-3.5 h-3.5" />
        </div>
        <span class="text-xs font-medium text-foreground truncate">{{ act.label }}</span>
      </button>
    </div>

    <!-- Medium 尺寸：经典 2x2 网格 -->
    <div v-else-if="sizePreset === 'medium'" class="quick-grid-compact flex-1">
      <button
        v-for="(act, i) in displayedActions"
        :key="act.label"
        class="quick-action-item flex flex-col items-center gap-1.5 p-3 rounded-[14px]"
        :class="{ 'quick-action-featured': act.label === '写文章' }"
        :aria-label="act.desc ? `${act.label}：${act.desc}` : act.label"
        @click="go(act)"
        @mouseenter="hoveredIdx = i"
        @mouseleave="hoveredIdx = null"
      >
        <div
          class="quick-icon-compact"
          :style="{
            backgroundColor: hoveredIdx === i ? act.bgColor.replace('0.14', '0.24') : act.bgColor,
            color: act.color,
          }"
        >
          <Icon :icon="act.icon" class="w-5 h-5" />
        </div>
        <span class="quick-label-compact">{{ act.label }}</span>
      </button>
    </div>

    <!-- Large 尺寸：8 宫格全功能扩展 -->
    <div v-else class="grid grid-cols-2 sm:grid-cols-4 gap-2.5 flex-1">
      <button
        v-for="act in displayedActions"
        :key="act.label"
        class="quick-action-item flex flex-col items-center justify-center p-3 rounded-2xl text-center group"
        :class="{ 'quick-action-featured': act.label === '写文章' }"
        :aria-label="act.desc ? `${act.label}：${act.desc}` : act.label"
        @click="go(act)"
      >
        <div
          class="w-10 h-10 rounded-xl flex items-center justify-center mb-2 group-hover:scale-105 transition-transform"
          :style="{ backgroundColor: act.bgColor, color: act.color }"
        >
          <Icon :icon="act.icon" class="w-5 h-5" />
        </div>
        <span class="text-xs font-semibold text-foreground">{{ act.label }}</span>
        <span class="text-[10px] text-muted-foreground mt-0.5">{{ act.desc }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
/* 三种尺寸共用表面：底色/边框/hover 单一定义（color-mix），布局差异走模板 tailwind */
.quick-action-item {
  background: color-mix(in srgb, #fff 3%, transparent);
  border: 1px solid color-mix(in srgb, #fff 5%, transparent);
  color: inherit;
  font: inherit;
  cursor: pointer;
  transition: background var(--duration-fast) var(--ease-standard),
    border-color var(--duration-fast) var(--ease-standard),
    transform var(--duration-fast) var(--ease-standard),
    box-shadow var(--duration-fast) var(--ease-standard);
}

.quick-action-item:hover {
  background: color-mix(in srgb, #fff 6%, transparent);
  border-color: color-mix(in srgb, #fff 8%, transparent);
  transform: scale(1.02);
  box-shadow: 0 10px 24px -18px #000;
}

.quick-action-item:active {
  transform: scale(0.98);
}

.quick-action-item.quick-action-featured {
  background: var(--color-card);
  border-color: color-mix(in srgb, var(--color-primary) 25%, transparent);
  box-shadow: inset 0 1px 0 color-mix(in srgb, #fff 10%, transparent), 0 12px 28px -22px var(--color-primary);
}

.quick-action-item.quick-action-featured:hover {
  border-color: color-mix(in srgb, var(--color-primary) 42%, transparent);
}

.quick-grid-compact {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 10px;
}

.quick-icon-compact {
  width: 38px;
  height: 38px;
  border-radius: 11px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background var(--duration-fast) var(--ease-standard);
}

.quick-label-compact {
  font-size: 12px;
  font-weight: 500;
  color: var(--color-foreground);
  letter-spacing: -0.1px;
}
</style>
