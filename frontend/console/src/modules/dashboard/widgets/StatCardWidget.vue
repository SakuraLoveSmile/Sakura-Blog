<script setup lang="ts">
import { computed } from"vue";
import { Icon } from"@iconify/vue";
import { VCard } from"@/components/ui";
import { useDashboardStats, type DashboardStats } from"./useDashboardStats";

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
 <span class="flex h-10 w-10 items-center justify-center rounded-base bg-secondary">
 <Icon :icon="icon" class="text-xl text-text-secondary" />
 </span>
 <div>
 <p class="text-xs text-text-tertiary">{{ label }}</p>
 <p class="text-xl font-bold text-foreground">{{ value }}</p>
 </div>
 </div>
 </VCard>
</template>
