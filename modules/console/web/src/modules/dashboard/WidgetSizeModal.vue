<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { Icon } from "@iconify/vue";
import { VModal } from "@/components/ui";
import type { Breakpoint, GridConstraints, GridSize } from "./widgets/layout";

interface Props {
  visible: boolean;
  widgetLabel: string;
  widgetIcon: string;
  breakpoint: Breakpoint;
  currentSize: GridSize;
  constraints: GridConstraints;
}

const props = defineProps<Props>();
const emit = defineEmits<{
  "update:visible": [value: boolean];
  "save": [size: GridSize];
}>();

const modalVisible = computed({
  get: () => props.visible,
  set: (v: boolean) => emit("update:visible", v),
});

const width = ref(props.currentSize.w);
const height = ref(props.currentSize.h);

watch(
  () => [props.visible, props.currentSize.w, props.currentSize.h],
  () => {
    if (props.visible) {
      width.value = props.currentSize.w;
      height.value = props.currentSize.h;
    }
  },
  { immediate: true },
);

const breakpointLabel: Record<Breakpoint, string> = {
  l: "大屏",
  m: "中屏",
  s: "小屏",
};

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, Math.round(value || min)));
}

function save() {
  emit("save", {
    w: clamp(width.value, props.constraints.minW, props.constraints.maxW),
    h: clamp(height.value, props.constraints.minH, props.constraints.maxH),
  });
  emit("update:visible", false);
}

function close() {
  emit("update:visible", false);
}
</script>

<template>
  <VModal v-model:visible="modalVisible" title="组件尺寸" :width="460">
    <div class="flex items-center gap-3 mb-5 p-3 rounded-xl bg-secondary dark:bg-secondary/50">
      <div class="w-10 h-10 rounded-lg bg-white dark:bg-accent flex items-center justify-center text-primary">
        <Icon :icon="widgetIcon" class="w-5 h-5" />
      </div>
      <div class="flex-1 min-w-0">
        <div class="text-sm font-medium text-text-quaternary dark:text-foreground">{{ widgetLabel }}</div>
        <div class="text-xs text-muted-foreground dark:text-text-quaternary">
          {{ breakpointLabel[breakpoint] }} · 拖拽手柄也可以直接调整
        </div>
      </div>
    </div>

    <div class="grid grid-cols-2 gap-4">
      <label class="flex flex-col gap-2 text-sm text-text-quaternary dark:text-text-secondary">
        <span>宽度（列）</span>
        <input
          v-model.number="width"
          type="number"
          :min="constraints.minW"
          :max="constraints.maxW"
          step="1"
          class="apple-grid-number"
        />
        <span class="text-xs text-muted-foreground">{{ constraints.minW }}–{{ constraints.maxW }} 列</span>
      </label>
      <label class="flex flex-col gap-2 text-sm text-text-quaternary dark:text-text-secondary">
        <span>高度（网格行）</span>
        <input
          v-model.number="height"
          type="number"
          :min="constraints.minH"
          :max="constraints.maxH"
          step="1"
          class="apple-grid-number"
        />
        <span class="text-xs text-muted-foreground">{{ constraints.minH }}–{{ constraints.maxH }} 行</span>
      </label>
    </div>

    <div class="mt-5 p-3 rounded-lg bg-primary/10 text-xs text-primary flex items-start gap-2">
      <Icon icon="ri:information-line" class="w-4 h-4 flex-shrink-0 mt-0.5" />
      <span>宽度按 12 列网格吸附，高度按网格行吸附；最小尺寸用于保护组件内容不被裁切。</span>
    </div>

    <template #footer>
      <button
        class="px-4 py-2 rounded-lg bg-secondary dark:bg-secondary text-sm text-text-quaternary dark:text-text-secondary hover:bg-accent dark:hover:bg-accent transition-colors mr-2"
        @click="close"
      >
        取消
      </button>
      <button
        class="px-4 py-2 rounded-full bg-primary text-sm font-semibold text-primary-foreground transition-colors hover:bg-primary-hover"
        @click="save"
      >
        保存
      </button>
    </template>
  </VModal>
</template>
