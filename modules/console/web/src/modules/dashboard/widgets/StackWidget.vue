<script setup lang="ts">
/**
 * 叠放容器（参照 Halo `core:stack` 的交互思路，样式与结构按本项目深色规范重写）。
 *
 * 一个网格位置里轮播多个子组件：子组件由 `config.stackWidgets` 指定，
 * 渲染时直接复用 `widgetRegistry` 的定义与 props，因此统计口径、数据源与
 * 独立摆放时完全一致。子组件仍渲染各自的 `apple-card`，这里用
 * `.stack-widget-body :deep(.apple-card)` 把内层卡片表面抹平，避免卡片套卡片
 * （`ui-restraint.test.ts` 的「禁止卡片套卡片」守卫扫描模板，运行时嵌套由这条样式收敛）。
 */
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { Icon } from "@iconify/vue";
import { useBreakpoint } from "../useBreakpoint";
import { useWidgetSize } from "../useWidgetSize";
import { getWidgetDefinition } from "./registry";
import type { WidgetSizePreset } from "./layout";

interface Props {
  sizePreset?: WidgetSizePreset;
  config?: Record<string, unknown>;
}

const props = withDefaults(defineProps<Props>(), {
  sizePreset: "medium",
  config: () => ({}),
});
const sizePreset = useWidgetSize(props.sizePreset).preset;
const { breakpoint } = useBreakpoint();
const container = ref<HTMLElement | null>(null);

/** 容器内组件 id 列表：只保留已注册的定义，未知 id 静默忽略。 */
const entries = computed(() => {
  const raw = props.config.stackWidgets;
  const ids = Array.isArray(raw) ? raw.filter((item): item is string => typeof item === "string") : [];
  const seen = new Set<string>();
  return ids.flatMap((id) => {
    if (id === "stack" || seen.has(id)) return [];
    seen.add(id);
    const definition = getWidgetDefinition(id);
    return definition ? [definition] : [];
  });
});

const index = ref(0);
const activeIndex = computed(() => (entries.value.length ? Math.min(index.value, entries.value.length - 1) : 0));
const active = computed(() => entries.value[activeIndex.value]);

watch(
  () => entries.value.length,
  (length) => {
    if (index.value >= length) index.value = 0;
  },
);

function step(direction: -1 | 1) {
  const total = entries.value.length;
  if (total === 0) return;
  index.value = (activeIndex.value + direction + total) % total;
}

function select(target: number) {
  index.value = target;
}

/** 自动轮播：间隔 0 表示关闭；组件卸载或配置变化时清理定时器。 */
const intervalSeconds = computed(() => {
  const raw = Number(props.config.interval);
  return Number.isFinite(raw) && raw > 0 ? raw : 0;
});

let timer: ReturnType<typeof window.setInterval> | null = null;

function stopTimer() {
  if (timer !== null) {
    window.clearInterval(timer);
    timer = null;
  }
}

function restartTimer() {
  stopTimer();
  if (!intervalSeconds.value || entries.value.length < 2) return;
  timer = window.setInterval(() => {
    // 输入、回复及键盘操作期间保留当前卡片，避免轮播夺走焦点。
    if (!container.value?.contains(document.activeElement) && !document.querySelector('[role="dialog"]')) step(1);
  }, intervalSeconds.value * 1000);
}

watch([intervalSeconds, () => entries.value.length], restartTimer, { immediate: true });
onBeforeUnmount(stopTimer);

const positionLabel = computed(() =>
  entries.value.length ? `${activeIndex.value + 1} / ${entries.value.length}` : "0 / 0",
);
</script>

<template>
  <div ref="container" class="apple-card stack-widget" :class="`stack-card-${sizePreset}`" :data-bp="breakpoint">
    <div class="stack-widget-heading">
      <div class="stack-widget-title">
        <h3 class="apple-section-title">叠放</h3>
        <span v-if="entries.length > 1" class="stack-widget-position">{{ positionLabel }}</span>
      </div>
      <div v-if="entries.length > 1" class="stack-widget-controls">
        <button
          type="button"
          class="stack-widget-btn"
          aria-label="上一个组件"
          title="上一个组件"
          @click="step(-1)"
        >
          <Icon icon="ri:arrow-left-s-line" class="h-4 w-4" />
        </button>
        <button
          type="button"
          class="stack-widget-btn"
          aria-label="下一个组件"
          title="下一个组件"
          @click="step(1)"
        >
          <Icon icon="ri:arrow-right-s-line" class="h-4 w-4" />
        </button>
      </div>
    </div>

    <div v-if="!active" class="stack-widget-empty">
      <Icon icon="ri:stack-line" class="h-4 w-4" />
      还没有选择容器内组件，可在组件设置里添加。
    </div>
    <div v-else class="stack-widget-body">
      <KeepAlive>
        <component
          :is="active.component"
          :key="active.id"
          v-bind="active.props"
          :config="props.config"
          :size-preset="sizePreset"
        />
      </KeepAlive>
    </div>

    <div v-if="entries.length > 1" class="stack-widget-tabs" role="tablist" aria-label="叠放容器内组件">
      <button
        v-for="(entry, i) in entries"
        :key="entry.id"
        type="button"
        role="tab"
        class="stack-tab"
        :class="{ 'is-active': i === activeIndex }"
        :aria-selected="i === activeIndex"
        :title="entry.label"
        @click="select(i)"
      >
        <Icon :icon="entry.icon" class="h-3.5 w-3.5" />
        <span class="stack-tab-label">{{ entry.label }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.stack-widget {
  display: flex;
  flex-direction: column;
  min-height: 0;
  gap: 12px;
}

.stack-widget-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-shrink: 0;
}

.stack-widget-title {
  display: flex;
  align-items: baseline;
  gap: 8px;
  min-width: 0;
}

.stack-widget-position {
  font-size: 12px;
  color: var(--color-text-tertiary);
  font-variant-numeric: tabular-nums;
}

.stack-widget-controls {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.stack-widget-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--color-border);
  color: var(--color-text-tertiary);
  background: transparent;
  cursor: pointer;
  transition: color 150ms, border-color 150ms;
}

.stack-widget-btn:hover {
  color: var(--color-foreground);
  border-color: var(--color-border-strong);
}

.stack-widget-btn:focus-visible {
  outline: 2px solid var(--color-ring);
  outline-offset: 2px;
}

.stack-widget-body {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* 子组件自带 apple-card，这里抹平内层表面，只保留容器一层卡片。 */
.stack-widget-body :deep(.apple-card) {
  flex: 1;
  min-height: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
  padding: 0;
}

.stack-widget-empty {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  font-size: 12px;
  color: var(--color-text-tertiary);
  text-align: center;
}

.stack-widget-tabs {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
  overflow-x: auto;
  scrollbar-width: none;
}

.stack-widget-tabs::-webkit-scrollbar {
  display: none;
}

.stack-tab {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 4px 8px;
  border-radius: var(--radius-sm);
  border: 1px solid transparent;
  color: var(--color-text-tertiary);
  background: transparent;
  cursor: pointer;
  font-size: 12px;
  white-space: nowrap;
  transition: color 150ms, background-color 150ms, border-color 150ms;
}

.stack-tab:hover {
  color: var(--color-text-secondary);
}

.stack-tab.is-active {
  color: var(--color-foreground);
  border-color: var(--color-border);
  background: var(--color-accent);
}

.stack-tab:focus-visible {
  outline: 2px solid var(--color-ring);
  outline-offset: 2px;
}

.stack-tab-label {
  overflow: hidden;
  text-overflow: ellipsis;
}

.stack-card-small {
  gap: 8px;
}

.stack-card-small .stack-tab-label {
  display: none;
}

.stack-card-small .stack-widget-tabs {
  gap: 4px;
}
</style>
