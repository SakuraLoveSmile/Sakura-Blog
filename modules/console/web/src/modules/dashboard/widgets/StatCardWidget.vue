<script setup lang="ts">
import { computed } from "vue";
import { useDashboardStats } from "./useDashboardStats";
import { useWidgetSize } from "../useWidgetSize";
import type { WidgetSizePreset } from "./layout";

interface Props {
  label: string;
  statKey: string;
  sizePreset?: WidgetSizePreset;
}

const props = withDefaults(defineProps<Props>(), {
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
  <div class="apple-card dashboard-stat-card" :class="`stat-card-${sizePreset}`">
    <div class="dashboard-stat-heading">
      <span>{{ label }}</span>
    </div>
    <div class="dashboard-stat-value">{{ value }}</div>
    <p v-if="sizePreset === 'large'" class="dashboard-stat-caption">累计</p>
  </div>
</template>
