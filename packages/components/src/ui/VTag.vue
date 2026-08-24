<script setup lang="ts">
import { computed } from "vue";

// Halo VTag 复刻（packages/components/src/components/tag/Tag.vue）
const props = withDefaults(
  defineProps<{
    theme?: "default" | "primary" | "secondary" | "danger";
    rounded?: boolean;
  }>(),
  { theme: "default", rounded: false }
);

const classes = computed(() => [`tag-${props.theme}`, { "tag-rounded": props.rounded }]);
</script>

<template>
  <div :class="classes" class="tag-wrapper">
    <div v-if="$slots.leftIcon" class="tag-left-icon">
      <slot name="leftIcon" />
    </div>
    <span class="tag-content">
      <slot />
    </span>
    <div v-if="$slots.rightIcon" class="tag-right-icon">
      <slot name="rightIcon" />
    </div>
  </div>
</template>

<style scoped>
.tag-wrapper {
  display: inline-flex;
  flex-shrink: 0;
  flex-wrap: wrap;
  box-sizing: border-box;
  cursor: default;
  text-align: center;
  align-items: center;
  justify-content: center;
  vertical-align: middle;
  height: 22px;
  padding: 0 8px;
  font-size: 12px;
  font-weight: 500;
  border-radius: var(--radius-sm);
  border: 1px solid transparent;
  transition: all var(--duration-fast) var(--ease-standard);
}

.tag-default {
  border-color: var(--color-border-strong);
  color: var(--color-text-secondary);
  background: var(--color-secondary);
}

.tag-primary {
  color: var(--color-foreground);
  background: var(--color-primary);
  border-color: var(--color-primary);
}

.tag-secondary {
  color: var(--color-foreground);
  background: var(--color-accent);
  border-color: var(--color-accent);
}

.tag-danger {
  color: var(--color-foreground);
  background: var(--color-state-error);
  border-color: var(--color-state-error);
}

.tag-rounded {
  border-radius: var(--radius-full);
}

.tag-content {
  padding: 0 2px;
}
.tag-left-icon {
  padding-left: 2px;
  display: inline-flex;
  align-items: center;
}
.tag-right-icon {
  padding-right: 2px;
  display: inline-flex;
  align-items: center;
}
</style>
