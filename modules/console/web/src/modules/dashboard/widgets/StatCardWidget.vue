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
  <div class="apple-card h-full flex flex-col justify-between transition-all duration-200 hover:-translate-y-0.5">
    <div class="flex items-start justify-between">
      <div
        class="flex h-10 w-10 items-center justify-center rounded-xl"
        :style="{ backgroundColor: color + '18', color: color }"
      >
        <Icon :icon="icon" class="w-5 h-5" />
      </div>
    </div>
    <div class="mt-3">
      <div class="text-[28px] sm:text-[32px] font-bold leading-none tracking-tight text-foreground" style="font-variant-numeric: tabular-nums;">
        {{ value }}
      </div>
      <div class="mt-1.5 text-xs font-medium text-text-tertiary">{{ label }}</div>
    </div>
  </div>
</template>
