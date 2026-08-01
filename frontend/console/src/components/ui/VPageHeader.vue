<script setup lang="ts">
// Halo VPageHeader 复刻（packages/components/src/components/header/PageHeader.vue）
defineProps<{
  title?: string;
}>();
</script>

<template>
  <div class="flex min-h-14 items-center justify-between bg-white p-4 dark:bg-gray-800">
    <div class="min-w-0 flex-1 self-center">
      <h2 class="flex items-center gap-2 truncate text-xl font-bold text-gray-800 dark:text-gray-100">
        <slot name="icon" />
        <span>{{ title }}</span>
      </h2>
      <p v-if="$slots.subtitle" class="mt-0.5 truncate text-xs text-gray-500 dark:text-gray-400">
        <slot name="subtitle" />
      </p>
    </div>
    <div class="self-center">
      <slot name="actions" />
    </div>
  </div>
</template>
