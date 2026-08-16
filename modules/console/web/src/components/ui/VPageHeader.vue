<script setup lang="ts">
// Halo VPageHeader 复刻（Apple Dark Mode 视觉调整）
defineProps<{
  title?: string;
}>();
</script>

<template>
  <div
    class="sticky top-0 z-20 flex min-h-[56px] items-center justify-between px-5 py-3 backdrop-blur-xl"
    style="background: color-mix(in srgb, #000 72%, transparent); border-bottom: 1px solid var(--color-border);"
  >
    <div class="min-w-0 flex-1 self-center">
      <h2 class="flex items-center gap-2 truncate text-[22px] font-bold" style="color: var(--color-foreground);">
        <slot name="icon" />
        <span>{{ title }}</span>
      </h2>
      <p v-if="$slots.subtitle" class="mt-0.5 truncate text-[13px]" style="color: var(--color-text-tertiary);">
        <slot name="subtitle" />
      </p>
    </div>
    <div class="flex items-center gap-2 self-center">
      <slot name="actions" />
    </div>
  </div>
</template>
