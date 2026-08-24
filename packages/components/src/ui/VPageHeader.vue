<script setup lang="ts">
// VPageHeader（Apple Dark Mode 增强型吸顶标题栏）
defineProps<{
  title?: string;
  subtitle?: string;
}>();
</script>

<template>
  <header
    class="sticky top-0 z-20 flex min-h-[58px] flex-wrap items-center justify-between gap-3 px-4 py-3.5 backdrop-blur-2xl transition-all sm:px-6 sm:py-4 md:px-8"
    style="
      background: color-mix(in srgb, #000 82%, transparent);
      border-bottom: 1px solid var(--color-border);
      box-shadow: 0 1px 3px 0 rgba(0, 0, 0, 0.35);
    "
  >
    <div class="min-w-0 flex-1 self-center">
      <div class="flex items-center gap-2.5">
        <slot name="icon" />
        <h1 class="truncate text-xl font-bold tracking-tight text-foreground sm:text-[22px] md:text-[24px]">
          <slot name="title">{{ title }}</slot>
        </h1>
        <slot name="badge" />
      </div>
      <p v-if="subtitle || $slots.subtitle" class="mt-0.5 truncate text-xs text-text-tertiary sm:text-[13px]">
        <slot name="subtitle">{{ subtitle }}</slot>
      </p>
    </div>
    <div v-if="$slots.actions" class="flex items-center gap-2 self-center sm:gap-2.5">
      <slot name="actions" />
    </div>
  </header>
</template>
