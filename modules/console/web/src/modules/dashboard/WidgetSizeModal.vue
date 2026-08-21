<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { Icon } from "@iconify/vue";
import { VModal } from "@/components/ui";
import {
  SPAN_LABELS,
  SIZE_PRESETS,
  type WidgetSpan,
  type WidgetSize,
} from "./widgets/registry";
import { uniquePresetOptions } from "./widgets/size-preset-options";
import type { Breakpoint } from "./widgets/layout";

interface Props {
  visible: boolean;
  widgetType: string;
  widgetLabel: string;
  widgetIcon: string;
  breakpoint: Breakpoint;
  currentSpan: WidgetSpan;
}

const props = defineProps<Props>();
const emit = defineEmits<{
  "update:visible": [value: boolean];
  "save": [span: WidgetSpan];
}>();

const modalVisible = computed({
  get: () => props.visible,
  set: (v: boolean) => emit("update:visible", v),
});

const selectedSpan = ref<WidgetSpan>(props.currentSpan);

// 打开 / 切换组件时同步当前断点跨度
watch(
  () => [props.visible, props.widgetType, props.currentSpan],
  () => {
    if (props.visible) selectedSpan.value = props.currentSpan;
  },
  { immediate: true },
);

const breakpointLabel: Record<Breakpoint, string> = {
  l: "大屏",
  m: "中屏",
  s: "小屏",
};

const spans: WidgetSpan[] = [3, 4, 6, 8, 12];
const presets = computed(() => uniquePresetOptions(SIZE_PRESETS, props.breakpoint));
const presetLabel: Record<WidgetSize, string> = {
  small: "小",
  medium: "中",
  large: "大",
};

/** 快捷预设仅改当前断点档：取预设在当前断点下的跨度 */
function presetSpan(preset: WidgetSize): WidgetSpan {
  return SIZE_PRESETS[preset][props.breakpoint];
}

function selectPreset(preset: WidgetSize) {
  selectedSpan.value = presetSpan(preset);
}

function close() {
  emit("update:visible", false);
}

function save() {
  emit("save", selectedSpan.value);
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
          当前断点：{{ breakpointLabel[breakpoint] }}
        </div>
      </div>
    </div>

    <!-- 5 档跨度 -->
    <div class="mb-5">
      <div class="text-xs font-medium text-text-quaternary dark:text-muted-foreground mb-2">列跨度（12 列网格）</div>
      <div class="flex gap-1.5">
        <button
          v-for="span in spans"
          :key="span"
          class="flex-1 min-w-[52px] px-2 py-2 rounded-md border text-sm transition-all"
          :class="selectedSpan === span
            ? 'border-primary bg-primary/10 text-primary font-medium'
            : 'border-border dark:border-border-strong text-text-quaternary dark:text-muted-foreground hover:border-border dark:hover:border-border-strong'"
          @click="selectedSpan = span"
        >
          {{ SPAN_LABELS[span] }}
        </button>
      </div>
    </div>

    <!-- 快捷预设（仅改当前断点档） -->
    <div class="mb-5">
      <div class="text-xs font-medium text-text-quaternary dark:text-muted-foreground mb-2">快捷预设（仅作用于当前断点）</div>
      <div class="flex gap-2">
        <button
          v-for="preset in presets"
          :key="preset"
          class="flex-1 px-3 py-2 rounded-lg border border-border dark:border-border-strong hover:border-primary/50 hover:bg-primary/5 text-sm text-text-quaternary dark:text-text-secondary transition-all"
          :class="{ 'border-primary bg-primary/10 text-primary dark:text-primary': selectedSpan === presetSpan(preset) }"
          @click="selectPreset(preset)"
        >
          <Icon
            :icon="preset === 'small' ? 'ri:layout-left-2-line' : preset === 'medium' ? 'ri:layout-masonry-line' : 'ri:layout-5-line'"
            class="w-4 h-4 inline-block mr-1.5 align-middle"
          />
          {{ presetLabel[preset] }}组件
        </button>
      </div>
    </div>

    <div class="p-3 rounded-lg bg-primary/10 dark:bg-primary/10 text-xs text-primary dark:text-primary flex items-start gap-2">
      <Icon icon="ri:information-line" class="w-4 h-4 flex-shrink-0 mt-0.5" />
      <span>尺寸调整仅作用于当前断点（{{ breakpointLabel[breakpoint] }}）。切换其它断点视角互不影响，需在各断点分别设置。</span>
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
