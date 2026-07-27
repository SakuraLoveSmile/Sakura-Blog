<script setup lang="ts">
import type { RouteLocationRaw } from "vue-router";

// Halo VEntityField 复刻（packages/components/src/components/entity/EntityField.vue）
defineProps<{
  title?: string;
  description?: string;
  route?: RouteLocationRaw;
  width?: string;
  maxWidth?: string;
}>();
</script>

<template>
  <div :style="{ width: width, maxWidth: maxWidth }" class="entity-field-wrapper">
    <div v-if="title || $slots.title" class="entity-field-title-body">
      <slot name="title">
        <router-link v-if="route" :to="route" class="entity-field-title hover:text-blue-600 dark:hover:text-blue-400">
          {{ title }}
        </router-link>
        <div v-else class="entity-field-title">{{ title }}</div>
      </slot>
      <slot name="extra" />
    </div>
    <div v-if="description || $slots.description" class="entity-field-description-body">
      <slot name="description">
        <span class="entity-field-description">{{ description }}</span>
      </slot>
    </div>
  </div>
</template>

<style scoped>
.entity-field-wrapper {
  display: inline-flex;
  flex-direction: column;
  gap: 0.25rem;
  min-width: 0;
  max-width: 20rem;
}

.entity-field-title-body {
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: 0.5rem;
}

.entity-field-title {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.875rem;
  font-weight: 500;
  color: #111827;
  text-decoration: none;
}
.dark .entity-field-title {
  color: #f3f4f6;
}

.entity-field-description-body {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.entity-field-description {
  font-size: 0.75rem;
  color: #6b7280;
}
.dark .entity-field-description {
  color: #9ca3af;
}
</style>
