<script setup lang="ts">
// Halo VCard 复刻（packages/components/src/components/card/Card.vue）
defineProps<{
  title?: string;
  bodyClass?: string[];
}>();
</script>

<template>
  <div class="card-wrapper">
    <div v-if="title || $slots.header" class="card-header">
      <slot name="header">
        <div class="card-header-title">{{ title }}</div>
        <div class="card-header-actions">
          <slot name="actions" />
        </div>
      </slot>
    </div>
    <div :class="bodyClass" class="card-body">
      <slot />
    </div>
    <div v-if="$slots.footer" class="card-footer">
      <slot name="footer" />
    </div>
  </div>
</template>

<style scoped>
.card-wrapper {
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  background: #fff;
  box-shadow: 0 1px 2px 0 rgb(0 0 0 / 0.05);
  overflow: hidden;
  border-radius: 4px;
  border: 1px solid #eaecf0;
}
.dark .card-wrapper {
  background: #1f2937;
  border-color: #374151;
}

.card-header {
  display: flex;
  justify-content: space-between;
  border-bottom: 1px solid #eaecf0;
}
.dark .card-header {
  border-bottom-color: #374151;
}

.card-header-title {
  align-self: center;
  font-size: 1rem;
  font-weight: 700;
  padding: 12px 16px;
  color: #1f2937;
}
.dark .card-header-title {
  color: #f3f4f6;
}

.card-header-actions {
  align-self: center;
  padding-right: 16px;
}

.card-body {
  padding: 12px 16px;
}

.card-footer {
  border-top: 1px solid #eaecf0;
  padding: 12px 16px;
}
.dark .card-footer {
  border-top-color: #374151;
}
</style>
