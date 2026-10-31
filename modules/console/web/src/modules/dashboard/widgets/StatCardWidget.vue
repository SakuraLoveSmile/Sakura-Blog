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
  <div class="apple-card dashboard-stat-card" :class="`stat-card-${sizePreset}`">
    <div class="dashboard-stat-heading">
      <Icon :icon="icon" class="h-4 w-4 shrink-0" aria-hidden="true" />
      <span>{{ label }}</span>
    </div>
    <div class="dashboard-stat-value">{{ value }}</div>
    <div v-if="sizePreset !== 'small'" class="dashboard-stat-detail">
      <span>{{ statContext }}</span>
      <span v-if="change" :class="['stat-change-label', `is-${change.tone}`]">
        <Icon
          :icon="change.tone === 'up' ? 'ri:arrow-up-line' : change.tone === 'down' ? 'ri:arrow-down-line' : 'ri:subtract-line'"
          class="h-3 w-3"
          aria-hidden="true"
        />
        {{ change.label }}
      </span>
      <span v-else>暂无对比数据</span>
    </div>
  </div>
</template>
