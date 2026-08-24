<script setup lang="ts">
import { computed } from "vue";

// Halo VStatusDot 复刻（packages/components/src/components/status）
const props = withDefaults(
  defineProps<{
    state?: "default" | "success" | "warning" | "error";
    animate?: boolean;
    text?: string;
  }>(),
  { state: "default", animate: false, text: undefined },
);

const dotColor = computed(() => {
  switch (props.state) {
    case "success":
      return "var(--color-state-success)";
    case "warning":
      return "var(--color-state-warning)";
    case "error":
      return "var(--color-state-error)";
    default:
      return "var(--color-text-tertiary)";
  }
});
</script>

<template>
  <div class="inline-flex items-center gap-1.5">
    <span class="relative flex h-2 w-2">
      <span
        v-if="animate"
        :style="{ background: dotColor }"
        class="absolute inline-flex h-full w-full animate-ping rounded-full opacity-75"
      ></span>
      <span :style="{ background: dotColor }" class="relative inline-flex h-2 w-2 rounded-full"></span>
    </span>
    <span v-if="text" class="text-xs" style="color: var(--color-text-tertiary)">{{ text }}</span>
  </div>
</template>
