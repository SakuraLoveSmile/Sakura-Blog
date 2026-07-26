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
  cursor: pointer;
  text-align: center;
  align-items: center;
  justify-content: center;
  width: auto;
  vertical-align: middle;
  height: 1.25rem;
  font-size: 0.75rem;
  border-radius: 4px;
  border: 1px solid transparent;
}

.tag-default {
  border-color: #d9d9d9;
  color: #4b5563;
  background: #fff;
}
.dark .tag-default {
  border-color: #4b5563;
  color: #d1d5db;
  background: transparent;
}

.tag-primary {
  color: #fff;
  background: #4ccba0;
  border-color: #4ccba0;
}
.tag-secondary {
  color: #fff;
  background: #0e1731;
  border-color: #0e1731;
}
.tag-danger {
  color: #fff;
  background: #d71d1d;
  border-color: #d71d1d;
}

.tag-rounded {
  border-radius: 9999px;
}

.tag-content {
  padding: 0 0.25rem;
}
.tag-left-icon {
  padding-left: 0.25rem;
  display: inline-flex;
  align-items: center;
}
.tag-right-icon {
  padding-right: 0.25rem;
  display: inline-flex;
  align-items: center;
}
</style>
