<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { Icon } from "@iconify/vue";
import { VModal } from "@/components/ui";
import type { Breakpoint, WidgetSizePreset } from "./widgets/layout";
import { getWidgetPresetOptions } from "./widgets/size-preset-options";

interface Props {
  visible: boolean;
  widgetId: string;
  widgetLabel: string;
  widgetIcon: string;
  breakpoint: Breakpoint;
  currentPreset: WidgetSizePreset;
}

const props = defineProps<Props>();
const emit = defineEmits<{
  "update:visible": [value: boolean];
  save: [preset: WidgetSizePreset];
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

const presetMeta: Record<
  WidgetSizePreset,
  {
    label: string;
    title: string;
    description: string;
    icon: string;
  }
> = {
  small: {
    label: "小",
    title: "紧凑速览",
    description: "保留核心指标，适合并排查看。",
    icon: "ri:layout-column-line",
  },
  medium: {
    label: "中",
    title: "平衡标准",
    description: "标准内容密度，兼顾信息与呼吸感。",
    icon: "ri:layout-grid-line",
  },
  large: {
    label: "大",
    title: "深度洞察",
    description: "展开趋势、明细与更多操作入口。",
    icon: "ri:layout-right-line",
  },
};

const sizeOptions = computed(() =>
  getWidgetPresetOptions(props.widgetId, props.breakpoint).map(({ preset, size }) => ({
    key: preset,
    ...presetMeta[preset],
    size,
  })),
);

function save() {
  emit("save", selectedPreset.value);
  emit("update:visible", false);
}

function close() {
  emit("update:visible", false);
}
</script>

<template>
  <VModal v-model:visible="modalVisible" title="组件大小" :width="600">
    <div class="size-modal-header flex items-center gap-3 mb-5 p-3 rounded-xl">
      <div class="size-modal-icon w-10 h-10 rounded-lg flex items-center justify-center text-primary">
        <Icon :icon="widgetIcon" class="w-5 h-5" />
      </div>
      <div class="flex-1 min-w-0">
        <div class="text-sm font-medium text-foreground">{{ widgetLabel }}</div>
        <div class="text-xs text-muted-foreground">
          {{ breakpointLabel[breakpoint] }} · 选择预设大小
        </div>
      </div>
    </div>

    <div class="grid grid-cols-1 sm:grid-cols-3 gap-3 size-preset-options">
      <button
        v-for="option in sizeOptions"
        :key="option.key"
        type="button"
        :data-size-preset="option.key"
        :aria-pressed="selectedPreset === option.key"
        :class="[
          'size-preset-option flex min-h-40 flex-col items-center justify-start gap-2 rounded-xl border p-3 text-center transition-all',
          selectedPreset === option.key
            ? 'is-selected border-primary bg-primary/10 text-primary'
            : 'border-white/10 bg-white/[0.03] text-muted-foreground hover:border-primary/50 hover:bg-white/[0.06]',
        ]"
        @click="selectedPreset = option.key"
      >
        <div class="size-preset-preview" :class="`is-${option.key}`" aria-hidden="true">
          <span class="size-preset-preview-grid">
            <span class="size-preset-preview-block" />
          </span>
        </div>
        <div class="flex items-center gap-1.5">
          <Icon :icon="option.icon" class="h-4 w-4" />
          <span class="text-sm font-semibold">{{ option.label }} · {{ option.title }}</span>
        </div>
        <span class="text-xs opacity-70 leading-relaxed">{{ option.description }}</span>
        <span class="size-preset-grid-size">{{ option.size.w }} 列 × {{ option.size.h }} 行</span>
      </button>
    </div>

    <div class="size-modal-note mt-5 p-3 rounded-lg text-xs text-primary flex items-start gap-2">
      <Icon icon="ri:information-line" class="w-4 h-4 flex-shrink-0 mt-0.5" />
      <span>组件只能使用这三个预设大小；如果新尺寸放不下，组件会自动移动到当前布局的空位。</span>
    </div>

    <template #footer>
      <button
        class="size-modal-cancel px-4 py-2 rounded-lg text-sm mr-2"
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

<style scoped>
.size-preset-option {
  min-height: 176px;
  background: color-mix(in srgb, var(--color-card) 88%, transparent);
}

.size-preset-option.is-selected {
  transform: translateY(-1px);
}

.size-preset-option:focus-visible {
  outline: 2px solid var(--color-ring);
  outline-offset: 2px;
}

.size-preset-preview {
  width: 100%;
  height: 74px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 14px;
  background:
    var(--color-card);
  border: 1px solid color-mix(in srgb, var(--color-border) 72%, transparent);
}

.size-preset-preview-grid {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 86%;
  height: 70%;
  border: 1px dashed color-mix(in srgb, var(--color-muted-foreground) 55%, transparent);
  border-radius: 8px;
}

.size-preset-preview-block {
  display: block;
  width: 74%;
  height: 64%;
  border-radius: 6px;
  background: color-mix(in srgb, var(--color-primary) 78%, var(--color-card));
  box-shadow: 0 4px 12px -6px color-mix(in srgb, var(--color-primary) 60%, transparent);
}

.size-preset-preview.is-small .size-preset-preview-block {
  width: 34%;
  height: 42%;
}

.size-preset-preview.is-medium .size-preset-preview-block {
  width: 58%;
  height: 58%;
}

.size-preset-preview.is-large .size-preset-preview-block {
  width: 84%;
  height: 76%;
}

.size-preset-grid-size {
  margin-top: auto;
  color: var(--color-muted-foreground);
  font-family: var(--font-mono);
  font-size: 10px;
  letter-spacing: 0.02em;
}

.size-modal-header {
  background: color-mix(in srgb, var(--color-card) 82%, var(--color-primary));
  border: 1px solid color-mix(in srgb, #fff 7%, transparent);
}

.size-modal-icon {
  background: color-mix(in srgb, var(--color-primary) 14%, transparent);
  border: 1px solid color-mix(in srgb, var(--color-primary) 20%, transparent);
}

.size-modal-note {
  background: color-mix(in srgb, var(--color-primary) 10%, transparent);
  border: 1px solid color-mix(in srgb, var(--color-primary) 18%, transparent);
}

.size-modal-cancel {
  color: var(--color-muted-foreground);
  background: color-mix(in srgb, #fff 5%, transparent);
  border: 1px solid color-mix(in srgb, #fff 8%, transparent);
  transition: background var(--duration-fast) var(--ease-standard), color var(--duration-fast) var(--ease-standard);
}

.size-modal-cancel:hover {
  color: var(--color-foreground);
  background: color-mix(in srgb, #fff 9%, transparent);
}
</style>
