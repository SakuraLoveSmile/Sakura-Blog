<script setup lang="ts">
import { Icon } from "@iconify/vue";

// Halo FilterTag 复刻（可关闭筛选标签）
defineEmits<{
  (event: "close"): void;
}>();
</script>

<template>
  <div class="inline-flex items-center gap-1 rounded-base bg-secondary px-2 py-1 text-xs text-text-secondary">
    <slot />
    <button
      class="text-text-quaternary transition-colors hover:bg-accent hover:text-foreground"
      @click="$emit('close')"
    >
      <Icon icon="ri:close-line" class="text-sm" />
    </button>
  </div>
</template>
