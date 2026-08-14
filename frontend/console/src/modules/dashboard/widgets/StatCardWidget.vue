<script setup lang="ts">
import { computed } from "vue";
import { Icon } from "@iconify/vue";
import { useDashboardStats } from "./useDashboardStats";
import { themeColor } from "@/lib/theme-colors";

interface Props {
  label: string;
  statKey: string;
  icon: string;
  color?: string;
}

const props = withDefaults(defineProps<Props>(), {
  color: themeColor("primary", "#2E8DFF"),
});

// Spec 62：共享 useDashboardStats（模块级 promise 去重），
// 替代每个 StatCardWidget 各自裸 fetch 一次 /dashboard/stats 的 4-6 次重复请求。
const { stats } = useDashboardStats();

const value = computed(() => {
  if (!stats.value) return "0";
  const v = (stats.value as Record<string, unknown>)[props.statKey];
  return typeof v === "number" ? v.toLocaleString() : "0";
});

const sparklinePath = computed(() => {
  if (!stats.value?.sparkline || stats.value.sparkline.length === 0) return "";
  const data = stats.value.sparkline;
  const max = Math.max(...data);
  const min = Math.min(...data);
  const range = max - min || 1;
  const w = 100;
  const h = 30;
  const step = w / (data.length - 1);
  const points = data.map((v, i) => {
    const x = i * step;
    const y = h - ((v - min) / range) * h;
    return `${x},${y}`;
  });
  return `M${points.join(" L")}`;
});

const sparklineArea = computed(() => {
  const path = sparklinePath.value;
  if (!path) return "";
  return `${path} L100,30 L0,30 Z`;
});
</script>

<template>
  <div class="apple-card h-full flex flex-col">
    <div class="flex items-start justify-between">
      <div>
        <div class="apple-stat-icon" :style="{ backgroundColor: color + '20', color: color }">
          <Icon :icon="icon" class="w-5 h-5" />
        </div>
      </div>
    </div>
    <div class="apple-stat-value">{{ value }}</div>
    <div class="apple-stat-label">{{ label }}</div>
    <svg v-if="sparklinePath" class="apple-sparkline mt-auto" viewBox="0 0 100 30" preserveAspectRatio="none">
      <defs>
        <linearGradient :id="'grad-' + statKey" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" :stop-color="color" stop-opacity="0.3" />
          <stop offset="100%" :stop-color="color" stop-opacity="0" />
        </linearGradient>
      </defs>
      <path :d="sparklineArea" :fill="`url(#grad-${statKey})`" />
      <path :d="sparklinePath" fill="none" :stroke="color" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
    </svg>
  </div>
</template>
