<script setup lang="ts">
import { Icon } from "@iconify/vue";

// Halo FilterTag 复刻（可关闭筛选标签）
defineEmits<{
  (event: "close"): void;
}>();
</script>

<template>
  <div
    class="inline-flex items-center gap-1 rounded bg-gray-200 px-2 py-1 text-xs text-gray-600 dark:bg-gray-600 dark:text-gray-300"
    style="border-radius: 4px"
  >
    <slot />
    <button
      class="text-gray-400 transition-colors hover:text-gray-700 dark:hover:text-white"
      @click="$emit('close')"
    >
      <Icon icon="ri:close-line" class="text-sm" />
    </button>
  </div>
</template>
