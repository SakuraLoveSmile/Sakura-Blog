<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { Icon } from "@iconify/vue";

// Halo VTabbar 复刻（packages/components/src/components/tabbar/Tabbar.vue）
// 支持 default / outline / segmented 风格，用于设置页、备份页等分标签场景。
export interface TabbarItem {
  id: string;
  label: string;
  icon?: string;
  dirty?: boolean;
}

const props = withDefaults(
  defineProps<{
    /** 标签项列表 */
    items: TabbarItem[];
    /** 当前激活的标签 id（v-model:active-id） */
    activeId?: string;
    /** 风格：default 底部高亮条 / outline 胶囊填充 / segmented 悬浮分段胶囊 */
    type?: "default" | "outline" | "segmented";
    /**
     * 激活标签时自动水平显露（适合移动端/窄屏横向滚动场景）。
     * 开启后根元素自身成为唯一横向滚动区域：width:100%、min-width:0、overflow-x:auto、不换行；
     * 外层容器只负责宽度与裁切，不再承担横向滚动。
     */
    autoReveal?: boolean;
  }>(),
  {
    activeId: undefined,
    type: "default",
    autoReveal: false,
  },
);

const emit = defineEmits<{
  (event: "update:activeId", id: string): void;
  (event: "change", id: string): void;
}>();

const containerRef = ref<HTMLElement | null>(null);
const buttonRefs = ref<HTMLButtonElement[]>([]);
let resizeObserver: ResizeObserver | null = null;

const current = computed(() => props.activeId ?? props.items[0]?.id);

function prefersReducedMotion(): boolean {
  return (
    typeof window !== "undefined" &&
    typeof window.matchMedia === "function" &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches
  );
}

/**
 * 计算活动标签所需的最小水平位移。
 * 统一使用容器与标签的 getBoundingClientRect() 结合当前 scrollLeft，不使用元素相对偏移属性；
 * 标签已完整可见时不移动；结果限制在 0..scrollWidth-clientWidth。
 */
function scrollActiveIntoView() {
  if (!props.autoReveal) return;
  const container = containerRef.value;
  if (!container) return;

  const index = props.items.findIndex((item) => item.id === current.value);
  const target = buttonRefs.value[index];
  if (!target) return;

  const maxScrollLeft = container.scrollWidth - container.clientWidth;
  if (maxScrollLeft <= 0) return;

  const containerRect = container.getBoundingClientRect();
  const targetRect = target.getBoundingClientRect();

  let next = container.scrollLeft;
  if (targetRect.right > containerRect.right) {
    // 右侧超出：向右移动超出量
    next += targetRect.right - containerRect.right;
  } else if (targetRect.left < containerRect.left) {
    // 左侧超出：向左移动超出量
    next -= containerRect.left - targetRect.left;
  } else {
    // 完整可见：不移动
    return;
  }

  next = Math.max(0, Math.min(next, maxScrollLeft));
  if (Math.abs(next - container.scrollLeft) < 1) return;
  container.scrollTo({ left: next, behavior: prefersReducedMotion() ? "auto" : "smooth" });
}

function scheduleReveal() {
  void nextTick(scrollActiveIntoView);
}

// 点击、方向键/Home/End 切换与 query 驱动的 activeId 变化统一走 current
watch(current, scheduleReveal);
// 标签集合变化（query 切换导致标签增减）后也需要重新检查
watch(
  () => props.items.map((item) => item.id).join("|"),
  scheduleReveal,
);

onMounted(() => {
  scheduleReveal();
  // 容器尺寸变化时重新检查活动标签
  if (props.autoReveal && containerRef.value && typeof ResizeObserver !== "undefined") {
    resizeObserver = new ResizeObserver(scheduleReveal);
    resizeObserver.observe(containerRef.value);
  }
});

onBeforeUnmount(() => {
  resizeObserver?.disconnect();
  resizeObserver = null;
});

function select(id: string) {
  if (id === current.value) return;
  emit("update:activeId", id);
  emit("change", id);
}

function onKeydown(e: KeyboardEvent, index: number) {
  let targetIndex = -1;
  if (e.key === "ArrowRight" || e.key === "ArrowDown") {
    e.preventDefault();
    targetIndex = (index + 1) % props.items.length;
  } else if (e.key === "ArrowLeft" || e.key === "ArrowUp") {
    e.preventDefault();
    targetIndex = (index - 1 + props.items.length) % props.items.length;
  } else if (e.key === "Home") {
    e.preventDefault();
    targetIndex = 0;
  } else if (e.key === "End") {
    e.preventDefault();
    targetIndex = props.items.length - 1;
  }

  if (targetIndex >= 0 && targetIndex < props.items.length) {
    const nextItem = props.items[targetIndex];
    select(nextItem.id);
    buttonRefs.value[targetIndex]?.focus({ preventScroll: true });
  }
}
</script>

<template>
  <div
    ref="containerRef"
    :class="['tabbar', `tabbar-${type}`, { 'tabbar-autoreveal': autoReveal }]"
    role="tablist"
  >
    <button
      v-for="(item, index) in items"
      :id="`tab-${item.id}`"
      :key="item.id"
      :ref="(el) => { if (el) buttonRefs[index] = el as HTMLButtonElement; }"
      :class="['tabbar-item', { active: item.id === current, 'is-dirty': item.dirty }]"
      type="button"
      role="tab"
      :aria-selected="item.id === current"
      :aria-controls="`tabpanel-${item.id}`"
      :tabindex="item.id === current ? 0 : -1"
      @click="select(item.id)"
      @keydown="onKeydown($event, index)"
    >
      <slot name="item" :item="item" :active="item.id === current">
        <Icon v-if="item.icon" :icon="item.icon" class="tabbar-item-icon" />
        <span class="tabbar-item-label">{{ item.label }}</span>
        <span v-if="item.dirty" class="tabbar-item-dot settings-tab-dirty-dot" title="有未保存修改" aria-label="有未保存修改" />
      </slot>
    </button>
  </div>
</template>

<style scoped>
.tabbar {
  display: flex;
  gap: 0;
}

/* autoReveal：根元素自身是唯一横向滚动区域，外层只负责宽度与裁切 */
.tabbar-autoreveal {
  width: 100%;
  min-width: 0;
  max-width: 100%;
  overflow-x: auto;
  overflow-y: hidden;
  flex-wrap: nowrap;
  scrollbar-width: none;
  -webkit-overflow-scrolling: touch;
}
.tabbar-autoreveal::-webkit-scrollbar {
  display: none;
}
.tabbar-autoreveal .tabbar-item {
  flex: 0 0 auto;
}
.tabbar-autoreveal .tabbar-item-label {
  white-space: nowrap;
  overflow: visible;
  text-overflow: clip;
}

/* default 风格：底部主色高亮条（Apple Dark） */
.tabbar-default {
  border-bottom: 1px solid var(--color-border);
  flex-wrap: wrap;
}

.tabbar-default .tabbar-item {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: 6px;
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
.tabbar-default .tabbar-item.active .tabbar-item-icon {
  color: var(--color-primary);
}

/* outline 风格：Apple 胶囊分段控件 */
.tabbar-outline {
  gap: 2px;
  padding: 2px;
  background: var(--color-secondary);
  border-radius: var(--radius-full);
}

.tabbar-outline .tabbar-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
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

/* segmented 风格：Apple Segmented 悬浮分段胶囊（方案 A） */
.tabbar-segmented {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 5px;
  background: color-mix(in srgb, var(--color-background) 55%, var(--color-card));
  border: 1px solid var(--color-border);
  border-radius: var(--radius-xl, 14px);
  box-shadow: inset 0 1px 3px rgba(0, 0, 0, 0.2);
}

.tabbar-segmented .tabbar-item {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: 7px;
  min-height: 38px;
  padding: 0 16px;
  font-size: 13.5px;
  font-weight: 550;
  color: var(--color-text-secondary);
  background: transparent;
  border: 1px solid transparent;
  border-radius: calc(var(--radius-xl, 14px) - 3px);
  cursor: pointer;
  user-select: none;
  transition: all var(--duration-fast) var(--ease-standard);
}

.tabbar-segmented .tabbar-item:hover {
  color: var(--color-foreground);
  background: color-mix(in srgb, var(--color-foreground) 4%, transparent);
}

.tabbar-segmented .tabbar-item.active {
  color: var(--color-foreground);
  background: var(--color-card);
  border-color: color-mix(in srgb, var(--color-border) 80%, transparent);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.28), 0 1px 2px rgba(0, 0, 0, 0.16);
}

.tabbar-item-icon {
  font-size: 15px;
  flex-shrink: 0;
  transition: transform var(--duration-fast) var(--ease-standard), color var(--duration-fast) var(--ease-standard);
}

.tabbar-segmented .tabbar-item.active .tabbar-item-icon {
  color: var(--color-primary);
  transform: scale(1.06);
}

.tabbar-item-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--color-state-warning, #ff9f0a);
  box-shadow: 0 0 6px color-mix(in srgb, var(--color-state-warning) 80%, transparent);
  flex-shrink: 0;
  margin-left: 2px;
}
.tabbar-item:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}
</style>
