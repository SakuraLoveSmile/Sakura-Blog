<script setup lang="ts">
import { Icon } from "@iconify/vue";
import { computed, ref } from "vue";
import { useRouter } from "vue-router";
import { useBreakpoint } from "../useBreakpoint";
import { themeColors } from "@/lib/theme-colors";
import type { WidgetSizePreset } from "./layout";
import { useWidgetSize } from "../useWidgetSize";

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
  chart2: "#00C7BE",
  error: "#FF453A",
  primaryHover: "#66ABFF",
  primaryActive: "#007AFF",
});

interface QuickAction {
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
    label: "写文章",
    desc: "发布新内容",
    icon: "ri:quill-pen-line",
    color: colors.primary,
    bgColor: "color-mix(in srgb, var(--color-primary) 14%, transparent)",
    path: "/posts/editor",
  },
  {
    label: "评论",
    desc: "互动审核",
    icon: "ri:chat-3-line",
    color: colors.success,
    bgColor: "color-mix(in srgb, var(--color-state-success) 14%, transparent)",
    path: "/comments",
  },
  {
    label: "分类",
    desc: "组织结构",
    icon: "ri:folder-3-line",
    color: colors.warning,
    bgColor: "color-mix(in srgb, var(--color-state-warning) 14%, transparent)",
    path: "/categories",
  },
  {
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
    label: "文章列表",
    desc: "管理已有文章",
    icon: "ri:file-list-3-line",
    color: colors.primaryHover,
    bgColor: "color-mix(in srgb, var(--color-primary-hover) 14%, transparent)",
    path: "/posts",
  },
  {
    label: "媒体附件",
    desc: "文件与图片",
    icon: "ri:attachment-2",
    color: colors.chart2,
    bgColor: "color-mix(in srgb, #00C7BE 14%, transparent)",
    path: "/attachments",
  },
  {
    label: "系统更新",
    desc: "版本与升级",
    icon: "ri:rocket-2-line",
    color: colors.error,
    bgColor: "color-mix(in srgb, var(--color-state-error) 14%, transparent)",
    path: "/system-update",
  },
  {
    label: "查看博客",
    desc: "打开前台页面",
    icon: "ri:external-link-line",
    color: colors.primaryActive,
    bgColor: "color-mix(in srgb, var(--color-primary-active) 14%, transparent)",
    path: "/",
    isExternal: true,
  },
];

const displayedActions = computed(() => {
  if (sizePreset.value === "large") return extendedActions;
  return baseActions;
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
</script>

<template>
  <div
    class="apple-card h-full flex flex-col justify-between"
    :class="`quick-card-${sizePreset}`"
    :data-bp="breakpoint"
  >
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
  transition:
    background var(--duration-fast) var(--ease-standard),
    border-color var(--duration-fast) var(--ease-standard);
}

.quick-action-item:hover {
  background: color-mix(in srgb, #fff 6%, transparent);
  border-color: color-mix(in srgb, #fff 8%, transparent);
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
