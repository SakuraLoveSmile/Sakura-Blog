<script setup lang="ts">
import { computed } from "vue";

// Halo VTabbar 复刻（packages/components/src/components/tabbar/Tabbar.vue）
// 支持 default / outline 两种风格，用于设置页、备份页等分标签场景。
export interface TabbarItem {
  id: string;
  label: string;
}

const props = withDefaults(
  defineProps<{
    /** 标签项列表 */
    items: TabbarItem[];
    /** 当前激活的标签 id（v-model:active-id） */
    activeId?: string;
    /** 风格：default 底部高亮条 / outline 胶囊填充 */
    type?: "default" | "outline";
  }>(),
  {
    activeId: undefined,
    type: "default",
  }
);

const emit = defineEmits<{
  (event: "update:activeId", id: string): void;
  (event: "change", id: string): void;
}>();

const current = computed(() => props.activeId ?? props.items[0]?.id);

function select(id: string) {
  if (id === current.value) return;
  emit("update:activeId", id);
  emit("change", id);
}
</script>

<template>
  <div :class="['tabbar', `tabbar-${type}`]">
    <button
      v-for="item in items"
      :key="item.id"
      :class="['tabbar-item', { active: item.id === current }]"
      type="button"
      @click="select(item.id)"
    >
      {{ item.label }}
    </button>
  </div>
</template>

<style scoped>
.tabbar {
  display: flex;
  gap: 0;
}

/* default 风格：底部主色高亮条（Apple Dark） */
.tabbar-default {
  border-bottom: 1px solid var(--color-border);
  flex-wrap: wrap;
}

.tabbar-default .tabbar-item {
  position: relative;
  padding: 0.5rem 1rem;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--color-text-tertiary);
  background: transparent;
  border: none;
  cursor: pointer;
  transition: color var(--duration-fast) var(--ease-standard);
  margin-bottom: -1px;
  border-bottom: 2px solid transparent;
}
.tabbar-default .tabbar-item:hover {
  color: var(--color-foreground);
}
.tabbar-default .tabbar-item.active {
  color: var(--color-foreground);
  border-bottom-color: var(--color-primary);
}

/* outline 风格：Apple 胶囊分段控件 */
.tabbar-outline {
  gap: 2px;
  padding: 2px;
  background: var(--color-secondary);
  border-radius: var(--radius-full);
}

.tabbar-outline .tabbar-item {
  padding: 0.375rem 0.875rem;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--color-text-tertiary);
  background: transparent;
  border: none;
  cursor: pointer;
  border-radius: var(--radius-full);
  transition: all var(--duration-fast) var(--ease-standard);
}
.tabbar-outline .tabbar-item:hover {
  color: var(--color-foreground);
}
.tabbar-outline .tabbar-item.active {
  color: var(--color-foreground);
  background: var(--color-accent);
  box-shadow: var(--shadow-sm);
}
</style>
