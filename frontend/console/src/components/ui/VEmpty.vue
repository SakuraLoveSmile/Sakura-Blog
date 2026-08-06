<script setup lang="ts">
// Halo VEmpty 复刻（packages/components/src/components/empty/Empty.vue）
defineProps<{
  title?: string;
  message?: string;
}>();
</script>

<template>
  <div class="flex h-full w-full flex-col items-center justify-center py-16">
    <slot name="image">
      <svg class="mb-4 h-16 w-16" fill="none" stroke="currentColor" stroke-width="1" viewBox="0 0 24 24" style="color: var(--color-text-tertiary)">
        <path
          d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"
          stroke-linecap="round"
          stroke-linejoin="round"
        />
      </svg>
    </slot>
    <h3 v-if="title" class="text-sm font-medium text-foreground">{{ title }}</h3>
    <p v-if="message" class="mt-1 text-xs text-text-tertiary">{{ message }}</p>
    <div v-if="$slots.actions" class="mt-4">
      <slot name="actions" />
    </div>
  </div>
</template>
