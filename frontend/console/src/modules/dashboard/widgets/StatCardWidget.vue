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
  </div>
</template>
