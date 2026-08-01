<script setup lang="ts">
import { computed } from "vue";

// Halo VEntity 复刻（packages/components/src/components/entity/Entity.vue）
// 每行固定两个 td：entity-start-wrapper（主体信息）+ entity-end-wrapper（辅助信息 + 操作菜单）
const props = withDefaults(
  defineProps<{
    isSelected?: boolean;
  }>(),
  { isSelected: false }
);

const classes = computed(() => (props.isSelected ? ["entity-selected"] : []));
</script>

<template>
  <tr :class="classes" class="entity-wrapper group">
    <td class="entity-start-wrapper">
      <div v-show="isSelected" class="entity-selected-indicator"></div>
      <slot name="prepend" />
      <div class="entity-start">
        <div v-if="$slots.checkbox" class="entity-checkbox">
          <slot name="checkbox" />
        </div>
        <slot name="start" />
      </div>
    </td>
    <td class="entity-end-wrapper">
      <div class="entity-end">
        <slot name="end" />
        <div v-if="$slots.dropdownItems" class="entity-dropdown">
          <slot name="dropdownItems" />
        </div>
      </div>
    </td>
  </tr>
  <tr v-if="$slots.footer">
    <td colspan="2"><slot name="footer" /></td>
  </tr>
</template>

<style scoped>
.entity-wrapper {
  position: relative;
  transition: all 0.15s ease;
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

.entity-start-wrapper,
.entity-end-wrapper {
  position: relative;
  padding: 12px 16px;
  vertical-align: middle;
}

.entity-selected-indicator {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  width: 2px;
  background: #4ccba0;
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
  align-items: center;
  gap: 1rem;
  min-width: 0;
}

.entity-end {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 1.5rem;
}

.entity-dropdown {
  display: inline-flex;
  align-items: center;
}
</style>
