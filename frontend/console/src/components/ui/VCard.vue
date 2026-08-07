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
  background: var(--color-card);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  overflow: hidden;
  position: relative;
  box-shadow: var(--shadow-sm);
}
.card-wrapper::before {
  content: "";
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 1px;
  background: linear-gradient(90deg, transparent, color-mix(in srgb, #fff 6%, transparent), transparent);
  pointer-events: none;
  z-index: 1;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px 18px;
  border-bottom: 1px solid var(--color-border);
}

.card-header-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--color-foreground);
}

.card-header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.card-body {
  padding: 16px 18px;
}

.card-footer {
  border-top: 1px solid var(--color-border);
  padding: 14px 18px;
}
</style>
