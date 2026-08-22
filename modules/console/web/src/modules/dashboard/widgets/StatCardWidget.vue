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
</script>

<template>
  <!-- Small 尺寸：横向紧凑速览 -->
  <div
    v-if="sizePreset === 'small'"
    class="apple-card stat-card-small h-full flex items-center gap-3 p-3 transition-all duration-200 hover:-translate-y-0.5"
  >
    <div
      class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-xl"
      :style="{ backgroundColor: color + '18', color: color }"
    >
      <Icon :icon="icon" class="w-4 h-4" />
    </div>
    <div class="min-w-0 flex-1">
      <div class="text-xl font-bold leading-tight tracking-tight text-foreground truncate" style="font-variant-numeric: tabular-nums;">
        {{ value }}
      </div>
      <div class="text-[11px] font-medium text-text-tertiary truncate">{{ label }}</div>
    </div>
  </div>

  <!-- Large 尺寸：带辅助分析与微型走势装饰的富指标卡 -->
  <div
    v-else-if="sizePreset === 'large'"
    class="apple-card stat-card-large h-full flex flex-col justify-between p-5 transition-all duration-200 hover:-translate-y-0.5"
  >
    <div class="flex items-start justify-between">
      <div class="flex items-center gap-3">
        <div
          class="flex h-11 w-11 items-center justify-center rounded-xl"
          :style="{ backgroundColor: color + '18', color: color }"
        >
          <Icon :icon="icon" class="w-6 h-6" />
        </div>
        <div>
          <div class="text-sm font-semibold text-foreground">{{ label }}</div>
          <div class="text-xs text-text-quaternary">实时核心统计指标</div>
        </div>
      </div>
      <span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium" :style="{ backgroundColor: color + '14', color: color }">
        <Icon icon="ri:pulse-line" class="w-3 h-3" />
        活跃
      </span>
    </div>

    <div class="mt-4 flex items-end justify-between">
      <div>
        <div class="text-3xl sm:text-4xl font-bold leading-none tracking-tight text-foreground" style="font-variant-numeric: tabular-nums;">
          {{ value }}
        </div>
        <div class="mt-2 text-xs font-medium text-text-tertiary flex items-center gap-1.5">
          <Icon icon="ri:bar-chart-fill" class="w-3.5 h-3.5" :style="{ color }" />
          <span>{{ label }}总计</span>
        </div>
      </div>
    </div>
  </div>

  <!-- Medium 尺寸：经典标准 Apple 卡片，图标与标签同行、数值区自然排布 -->
  <div
    v-else
    class="apple-card stat-card-medium h-full flex flex-col transition-all duration-200 hover:-translate-y-0.5"
  >
    <div class="flex items-center gap-2.5">
      <div
        class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-xl"
        :style="{ backgroundColor: color + '18', color: color }"
      >
        <Icon :icon="icon" class="w-4 h-4" />
      </div>
      <span class="min-w-0 truncate text-xs font-semibold text-text-tertiary">{{ label }}</span>
    </div>
    <div class="mt-4">
      <div class="text-[28px] sm:text-[32px] font-bold leading-none tracking-tight text-foreground" style="font-variant-numeric: tabular-nums;">
        {{ value }}
      </div>
    </div>
  </div>
</template>
