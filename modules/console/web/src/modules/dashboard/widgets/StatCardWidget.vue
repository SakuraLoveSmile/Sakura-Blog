<script setup lang="ts">
import { computed } from "vue";
import { Icon } from "@iconify/vue";
import { useDashboardStats } from "./useDashboardStats";
import { useWidgetSize } from "../useWidgetSize";
import { themeColor } from "@/lib/theme-colors";
import type { WidgetSizePreset } from "./layout";

interface Props {
  label: string;
  statKey: string;
  icon: string;
  color?: string;
  sizePreset?: WidgetSizePreset;
}

const props = withDefaults(defineProps<Props>(), {
  color: themeColor("primary", "#2E8DFF"),
  sizePreset: "medium",
});
const sizePreset = useWidgetSize(props.sizePreset).preset;

// Spec 62：共享 useDashboardStats（模块级 promise 去重）
const { stats } = useDashboardStats();

const value = computed(() => {
  if (!stats.value) return "0";
  const v = (stats.value as Record<string, unknown>)[props.statKey];
  return typeof v === "number" ? v.toLocaleString() : "0";
});

const statContext = computed(() => {
  switch (props.statKey) {
    case "post_total": return "全部内容";
    case "post_published": return "公开内容";
    case "view_total": return "累计访问";
    case "comment_total": return "互动反馈";
    case "attachment_total": return "媒体资源";
    case "user_total": return "注册用户";
    default: return "当前统计";
  }
});

const changeKey: Record<string, "post_change" | "view_change" | "comment_change"> = {
  post_total: "post_change",
  post_published: "post_change",
  view_total: "view_change",
  comment_total: "comment_change",
};

const change = computed(() => {
  const key = changeKey[props.statKey];
  const raw = key ? stats.value?.[key] : undefined;
  if (typeof raw !== "number" || !Number.isFinite(raw)) return null;
  return {
    label: `${raw > 0 ? "+" : ""}${raw.toFixed(1)}%`,
    tone: raw > 0 ? "up" : raw < 0 ? "down" : "flat",
  } as const;
});
</script>

<template>
  <!-- Small 尺寸：横向紧凑速览 -->
  <div
    v-if="sizePreset === 'small'"
    class="apple-card stat-card-small h-full flex items-center gap-3 p-3 transition-all duration-200"
    :style="{
      background: `radial-gradient(circle at 100% 0%, ${color}10 0%, transparent 60%), color-mix(in srgb, var(--color-card) 78%, transparent)`
    }"
  >
    <div
      class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-xl border border-white/5 shadow-inner"
      :style="{ backgroundColor: color + '18', color: color }"
    >
      <Icon :icon="icon" class="w-4 h-4" />
    </div>
    <div class="min-w-0 flex-1">
      <div class="text-xl font-bold leading-tight tracking-tight text-foreground truncate tabular-nums">
        {{ value }}
      </div>
      <div class="text-[11px] font-medium text-text-tertiary truncate">{{ label }}</div>
    </div>
  </div>

  <!-- Large 尺寸：带辅助分析与优雅 SVG 微趋势走势的富指标卡 -->
  <div
    v-else-if="sizePreset === 'large'"
    class="apple-card stat-card-large h-full flex flex-col justify-between p-5 transition-all duration-200"
    :style="{
      background: `radial-gradient(circle at 95% 10%, ${color}14 0%, transparent 55%), color-mix(in srgb, var(--color-card) 78%, transparent)`
    }"
  >
    <div class="stat-card-large-header flex items-start justify-between">
      <div class="flex items-center gap-3">
        <div
          class="stat-card-large-icon flex h-11 w-11 items-center justify-center rounded-xl border border-white/5 shadow-sm"
          :style="{ backgroundColor: color + '1a', color: color }"
        >
          <Icon :icon="icon" class="w-6 h-6" />
        </div>
        <div>
          <div class="text-sm font-semibold text-foreground tracking-tight">{{ label }}</div>
          <div class="text-xs text-text-quaternary mt-0.5">实时核心统计指标</div>
        </div>
      </div>
      <span
        class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium border border-white/5"
        :style="{ backgroundColor: color + '14', color: color }"
      >
        <span class="w-1.5 h-1.5 rounded-full animate-pulse" :style="{ backgroundColor: color, boxShadow: `0 0 6px ${color}` }" />
        实时
      </span>
    </div>

    <div class="stat-card-large-body mt-4 flex items-end justify-between gap-4">
      <div>
        <div class="stat-card-large-value text-3xl sm:text-4xl font-bold leading-none tracking-tight text-foreground tabular-nums">
          {{ value }}
        </div>
        <div class="stat-card-large-meta mt-2 text-xs font-medium text-text-tertiary flex items-center gap-1.5">
          <Icon icon="ri:bar-chart-fill" class="w-3.5 h-3.5" :style="{ color }" />
          <span>{{ label }}数据统计</span>
        </div>
      </div>
      <div class="stat-card-signal" :style="{ '--stat-color': color }">
        <span class="stat-card-signal-orbit" aria-hidden="true">
          <Icon icon="ri:line-chart-line" class="w-4 h-4" />
        </span>
        <span v-if="change" :class="['stat-change-label', `is-${change.tone}`]">
          <Icon :icon="change.tone === 'up' ? 'ri:arrow-up-line' : change.tone === 'down' ? 'ri:arrow-down-line' : 'ri:subtract-line'" class="w-3 h-3" />
          {{ change.label }}
        </span>
        <span v-else>等待对比数据</span>
      </div>
    </div>
  </div>

  <!-- Medium 尺寸：经典标准 Apple 卡片，图标与标签同行、数值区自然排布 -->
  <div
    v-else
    class="apple-card stat-card-medium h-full flex flex-col justify-between transition-all duration-200"
  >
    <div class="flex items-center gap-2.5">
      <div
        class="stat-card-medium-icon flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-xl border border-white/5"
        :style="{ backgroundColor: color + '18', color: color }"
      >
        <Icon :icon="icon" class="w-4 h-4" />
      </div>
      <span class="min-w-0 truncate text-xs font-semibold text-text-tertiary">{{ label }}</span>
      <span class="stat-live-dot ml-auto" :style="{ '--stat-color': color }" title="实时同步" />
    </div>
    <div class="stat-card-medium-body mt-3">
      <div class="text-[28px] sm:text-[32px] font-bold leading-none tracking-tight text-foreground tabular-nums">
        {{ value }}
      </div>
    </div>
    <div class="stat-card-medium-footer">
      <span class="stat-card-medium-context">
        <Icon icon="ri:database-2-line" class="w-3.5 h-3.5" :style="{ color }" />
        {{ statContext }}
      </span>
      <span v-if="change" :class="['stat-card-medium-state', `is-${change.tone}`]">
        <span class="stat-card-medium-state-dot" />
        {{ change.label }}
      </span>
      <span v-else class="stat-card-medium-state">
        <span class="stat-card-medium-state-dot" :style="{ backgroundColor: color }" />
        实时
      </span>
    </div>
  </div>
</template>
