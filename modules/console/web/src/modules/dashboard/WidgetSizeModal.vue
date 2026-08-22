<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { Icon } from "@iconify/vue";
import { VModal } from "@/components/ui";
import type { Breakpoint, WidgetSizePreset } from "./widgets/layout";

interface Props {
  visible: boolean;
  widgetLabel: string;
  widgetIcon: string;
  breakpoint: Breakpoint;
  currentPreset: WidgetSizePreset;
}

const props = defineProps<Props>();
const emit = defineEmits<{
  "update:visible": [value: boolean];
  "save": [preset: WidgetSizePreset];
}>();

const modalVisible = computed({
  get: () => props.visible,
  set: (v: boolean) => emit("update:visible", v),
});

const selectedPreset = ref<WidgetSizePreset>(props.currentPreset);

watch(
  () => [props.visible, props.currentPreset],
  () => {
    if (props.visible) {
      selectedPreset.value = props.currentPreset;
    }
  },
  { immediate: true },
);

const breakpointLabel: Record<Breakpoint, string> = {
  lg: "大屏",
  md: "中屏",
  sm: "平板",
  xs: "小屏",
  xxs: "手机",
};

const sizeOptions: { key: WidgetSizePreset; label: string; description: string; icon: string }[] = [
  { key: "small", label: "小", description: "紧凑排列", icon: "ri:layout-column-line" },
  { key: "medium", label: "中", description: "平衡显示", icon: "ri:layout-grid-line" },
  { key: "large", label: "大", description: "突出展示", icon: "ri:layout-right-line" },
];

function save() {
  emit("save", selectedPreset.value);
  emit("update:visible", false);
}

function close() {
  emit("update:visible", false);
}
</script>

<template>
  <VModal v-model:visible="modalVisible" title="组件大小" :width="520">
    <div class="flex items-center gap-3 mb-5 p-3 rounded-xl bg-secondary dark:bg-secondary/50">
      <div class="w-10 h-10 rounded-lg bg-white dark:bg-accent flex items-center justify-center text-primary">
        <Icon :icon="widgetIcon" class="w-5 h-5" />
      </div>
      <div class="flex-1 min-w-0">
        <div class="text-sm font-medium text-text-quaternary dark:text-foreground">{{ widgetLabel }}</div>
        <div class="text-xs text-muted-foreground dark:text-text-quaternary">
          {{ breakpointLabel[breakpoint] }} · 选择预设大小
        </div>
      </div>
    </div>

    <div class="grid grid-cols-3 gap-3">
      <button
        v-for="option in sizeOptions"
        :key="option.key"
        type="button"
        :aria-pressed="selectedPreset === option.key"
        :class="[
          'flex min-h-28 flex-col items-center justify-center gap-2 rounded-xl border p-3 text-center transition-colors',
          selectedPreset === option.key
            ? 'border-primary bg-primary/10 text-primary'
            : 'border-border bg-secondary/40 text-text-quaternary hover:border-primary/50 hover:bg-secondary dark:border-border-strong dark:bg-secondary/30 dark:text-text-secondary',
        ]"
        @click="selectedPreset = option.key"
      >
        <Icon :icon="option.icon" class="h-6 w-6" />
        <span class="text-sm font-semibold">{{ option.label }}</span>
        <span class="text-xs opacity-70">{{ option.description }}</span>
      </button>
    </div>

    <div class="mt-5 p-3 rounded-lg bg-primary/10 text-xs text-primary flex items-start gap-2">
      <Icon icon="ri:information-line" class="w-4 h-4 flex-shrink-0 mt-0.5" />
      <span>组件只能使用这三个预设大小；如果新尺寸放不下，组件会自动移动到当前布局的空位。</span>
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
