<script setup lang="ts">
import { computed } from "vue";
import { Icon } from "@iconify/vue";
import { VCard } from "@/components/ui";
import { useDashboardStats, type DashboardStats } from "./useDashboardStats";

const props = defineProps<{
  statKey: keyof DashboardStats;
  label: string;
  icon: string;
}>();

const { stats } = useDashboardStats();
const value = computed(() => stats.value[props.statKey] ?? 0);
</script>

<template>
  <VCard>
    <div class="flex items-center gap-3">
      <span class="flex h-10 w-10 items-center justify-center rounded-base bg-gray-100 dark:bg-gray-700">
        <Icon :icon="icon" class="text-xl text-gray-600 dark:text-gray-300" />
      </span>
      <div>
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ label }}</p>
        <p class="text-xl font-bold text-gray-900 dark:text-gray-100">{{ value }}</p>
      </div>
    </div>
  </VCard>
</template>
