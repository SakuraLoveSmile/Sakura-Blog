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

/* default 风格：底部高亮条，对齐 Halo 原版 */
.tabbar-default {
  border-bottom: 1px solid #e5e7eb;
  flex-wrap: wrap;
}
.dark .tabbar-default {
  border-bottom-color: #374151;
}

.tabbar-default .tabbar-item {
  position: relative;
  padding: 0.5rem 1rem;
  font-size: 0.875rem;
  font-weight: 500;
  color: #6b7280;
  background: transparent;
  border: none;
  cursor: pointer;
  transition: color 0.15s ease;
  margin-bottom: -1px;
  border-bottom: 2px solid transparent;
}
.dark .tabbar-default .tabbar-item {
  color: #9ca3af;
}
.tabbar-default .tabbar-item:hover {
  color: #1f2937;
}
.dark .tabbar-default .tabbar-item:hover {
  color: #f3f4f6;
}
.tabbar-default .tabbar-item.active {
  color: #0e1731;
  border-bottom-color: #0e1731;
}
.dark .tabbar-default .tabbar-item.active {
  color: #4ccba0;
  border-bottom-color: #4ccba0;
}

/* outline 风格：胶囊填充，用于 VCard header 内 */
.tabbar-outline {
  gap: 0.25rem;
  padding: 0.25rem;
  background: #f3f4f6;
  border-radius: 4px;
}
.dark .tabbar-outline {
  background: #374151;
}

.tabbar-outline .tabbar-item {
  padding: 0.375rem 0.75rem;
  font-size: 0.875rem;
  font-weight: 500;
  color: #6b7280;
  background: transparent;
  border: none;
  cursor: pointer;
  border-radius: 4px;
  transition: all 0.15s ease;
}
.dark .tabbar-outline .tabbar-item {
  color: #9ca3af;
}
.tabbar-outline .tabbar-item:hover {
  color: #1f2937;
}
.dark .tabbar-outline .tabbar-item:hover {
  color: #f3f4f6;
}
.tabbar-outline .tabbar-item.active {
  color: #1f2937;
  background: #fff;
  box-shadow: 0 1px 2px 0 rgb(0 0 0 / 0.05);
}
.dark .tabbar-outline .tabbar-item.active {
  color: #f3f4f6;
  background: #1f2937;
}
</style>
