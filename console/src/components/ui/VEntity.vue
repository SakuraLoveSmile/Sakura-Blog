<script setup lang="ts">
import { computed } from "vue";

// Halo VEntity 复刻（packages/components/src/components/entity/Entity.vue）
const props = withDefaults(
  defineProps<{
    isSelected?: boolean;
  }>(),
  { isSelected: false }
);

const classes = computed(() => (props.isSelected ? ["entity-selected"] : []));
</script>

<template>
  <div :class="classes" class="entity-wrapper group">
    <div v-show="isSelected" class="entity-selected-indicator"></div>
    <slot name="prepend" />
    <div class="entity-body">
      <div v-if="$slots.checkbox" class="entity-checkbox">
        <slot name="checkbox" />
      </div>
      <div class="entity-start">
        <slot name="start" />
      </div>
      <div class="entity-end">
        <slot name="end" />
      </div>
      <div v-if="$slots.dropdownItems" class="entity-dropdown">
        <slot name="dropdownItems" />
      </div>
    </div>
    <div v-if="$slots.footer">
      <slot name="footer" />
    </div>
  </div>
</template>

<style scoped>
.entity-wrapper {
  position: relative;
  display: block;
  cursor: pointer;
  padding: 12px 16px;
  transition: all 0.15s ease;
  width: 100%;
}
.entity-wrapper:hover {
  background: #f9fafb;
}
.dark .entity-wrapper:hover {
  background: rgb(55 65 81 / 0.3);
}
.entity-wrapper.entity-selected {
  background: #f3f4f6;
}
.dark .entity-wrapper.entity-selected {
  background: rgb(55 65 81 / 0.5);
}

.entity-selected-indicator {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  width: 2px;
  background: #4ccba0;
}

.entity-body {
  position: relative;
  display: flex;
  flex-direction: row;
  align-items: center;
  width: 100%;
}

.entity-checkbox {
  margin-right: 1rem;
  display: none;
  align-items: center;
}
@media (min-width: 640px) {
  .entity-checkbox {
    display: flex;
  }
}

.entity-start {
  display: flex;
  flex: 1;
  align-items: center;
  gap: 1rem;
  min-width: 0;
}

.entity-end {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 1rem;
}
@media (min-width: 640px) {
  .entity-end {
    flex-direction: row;
    align-items: center;
    gap: 1.5rem;
  }
}

.entity-dropdown {
  margin-left: 1rem;
  display: inline-flex;
  align-items: center;
}
</style>
