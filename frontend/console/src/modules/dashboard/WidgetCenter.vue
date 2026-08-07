<script setup lang="ts">
import { ref, computed } from "vue";
import { Icon } from "@iconify/vue";
import { VModal } from "@/components/ui";
import {
  widgetRegistry,
  SIZE_PRESETS,
  SPAN_LABELS,
  type WidgetDefinition,
  type WidgetSize,
  type ResponsiveSize,
  type WidgetSpan,
  type LayoutWidget,
} from "./widgets/registry";

interface Props {
  visible: boolean;
  existingTypes: string[];
}

const props = defineProps<Props>();
const emit = defineEmits<{
  "update:visible": [value: boolean];
  "add": [widget: LayoutWidget];
}>();

const modalVisible = computed({
  get: () => props.visible,
  set: (v: boolean) => emit("update:visible", v),
});

/** 当前选中的组件（用于尺寸配置） */
const selected = ref<WidgetDefinition | null>(null);
/** 当前配置的尺寸 */
const sizeConfig = ref<ResponsiveSize>({ l: 3, m: 6, s: 12 });

const sizeLabel: Record<WidgetSize, string> = {
  small: "小",
  medium: "中",
  large: "大",
};

const breakpointLabel = {
  l: "大屏 (≥1280px)",
  m: "中屏 (768-1279px)",
  s: "小屏 (<768px)",
} as const;

const availableSpans: WidgetSpan[] = [3, 4, 6, 8, 12];

const available = computed(() =>
  widgetRegistry.filter((w) => !props.existingTypes.includes(w.type))
);

function close() {
  emit("update:visible", false);
  selected.value = null;
}

/** 点击组件卡片，进入尺寸配置 */
function selectWidget(w: WidgetDefinition) {
  selected.value = w;
  sizeConfig.value = { ...w.defaultSize };
}

/** 使用预设快速填充 */
function applyPreset(preset: WidgetSize) {
  sizeConfig.value = { ...SIZE_PRESETS[preset] };
}

/** 确认添加 */
function confirmAdd() {
  if (!selected.value) return;
  emit("add", {
    type: selected.value.type,
    size: { ...sizeConfig.value },
  });
  close();
}

/** 返回组件选择 */
function backToSelect() {
  selected.value = null;
}
</script>

<template>
  <VModal v-model:visible="modalVisible" title="添加小组件" :width="560">
    <!-- 组件选择页 -->
    <template v-if="!selected">
      <p class="mb-4 text-sm text-gray-500 dark:text-gray-400">
        选择要添加到仪表盘的小组件
      </p>
      <div v-if="available.length === 0" class="py-8 text-center text-sm text-gray-400">
        <Icon icon="ri:check-double-line" class="w-12 h-12 mx-auto mb-2 opacity-50" />
        所有可用组件都已添加
      </div>
      <div v-else class="grid grid-cols-1 sm:grid-cols-2 gap-3 max-h-96 overflow-y-auto pr-1">
        <button
          v-for="w in available"
          :key="w.type"
          class="flex items-center gap-3 p-3 rounded-xl border border-gray-100 dark:border-gray-700 hover:border-gray-200 dark:hover:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-800/50 text-left transition-all group"
          @click="selectWidget(w)"
        >
          <div class="w-10 h-10 rounded-lg bg-gray-100 dark:bg-gray-800 flex items-center justify-center text-gray-500 dark:text-gray-400 group-hover:text-primary transition-colors">
            <Icon :icon="w.icon" class="w-5 h-5" />
          </div>
          <div class="flex-1 min-w-0">
            <div class="text-sm font-medium text-gray-800 dark:text-gray-200">{{ w.label }}</div>
            <div class="text-xs text-gray-400 dark:text-gray-500 mt-0.5">点击自定义尺寸</div>
          </div>
          <Icon icon="ri:arrow-right-s-line" class="w-5 h-5 text-gray-300 dark:text-gray-600 group-hover:text-primary transition-colors" />
        </button>
      </div>
    </template>

    <!-- 尺寸配置页 -->
    <template v-else>
      <div class="flex items-center gap-3 mb-5 p-3 rounded-xl bg-gray-50 dark:bg-gray-800/50">
        <div class="w-10 h-10 rounded-lg bg-white dark:bg-gray-700 flex items-center justify-center text-primary">
          <Icon :icon="selected.icon" class="w-5 h-5" />
        </div>
        <div class="flex-1 min-w-0">
          <div class="text-sm font-medium text-gray-800 dark:text-gray-200">{{ selected.label }}</div>
          <div class="text-xs text-gray-400 dark:text-gray-500">配置在不同屏幕下的尺寸</div>
        </div>
        <button
          class="text-xs px-2 py-1 rounded-md text-gray-500 dark:text-gray-400 hover:text-primary hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
          @click="backToSelect"
        >
          <Icon icon="ri:arrow-left-line" class="w-4 h-4 inline align-middle mr-0.5" />
          返回
        </button>
      </div>

      <!-- 快捷预设 -->
      <div class="mb-5">
        <div class="text-xs font-medium text-gray-500 dark:text-gray-400 mb-2">快捷预设</div>
        <div class="flex gap-2">
          <button
            v-for="preset in (['small', 'medium', 'large'] as WidgetSize[])"
            :key="preset"
            class="flex-1 px-3 py-2 rounded-lg border border-gray-200 dark:border-gray-700 hover:border-primary/50 hover:bg-primary/5 text-sm text-gray-700 dark:text-gray-300 transition-all"
            :class="{ 'border-primary bg-primary/10 text-primary dark:text-primary': JSON.stringify(sizeConfig) === JSON.stringify(SIZE_PRESETS[preset]) }"
            @click="applyPreset(preset)"
          >
            <Icon :icon="preset === 'small' ? 'ri:layout-left-2-line' : preset === 'medium' ? 'ri:layout-masonry-line' : 'ri:layout-5-line'" class="w-4 h-4 inline-block mr-1.5 align-middle" />
            {{ sizeLabel[preset] }}组件
          </button>
        </div>
      </div>

      <!-- 三断点尺寸选择 -->
      <div class="space-y-4">
        <div
          v-for="bp in (['l', 'm', 's'] as const)"
          :key="bp"
          class="flex items-center gap-4"
        >
          <div class="w-32 flex-shrink-0">
            <div class="flex items-center gap-1.5 text-sm text-gray-700 dark:text-gray-300">
              <Icon
                :icon="bp === 'l' ? 'ri:computer-line' : bp === 'm' ? 'ri:tablet-line' : 'ri:smartphone-line'"
                class="w-4 h-4 text-gray-400"
              />
              {{ bp === 'l' ? '大屏' : bp === 'm' ? '中屏' : '小屏' }}
            </div>
            <div class="text-xs text-gray-400 dark:text-gray-500 mt-0.5">{{ breakpointLabel[bp] }}</div>
          </div>
          <div class="flex gap-1.5 flex-1">
            <button
              v-for="span in availableSpans"
              :key="span"
              class="flex-1 min-w-[52px] px-2 py-2 rounded-md border text-sm transition-all"
              :class="sizeConfig[bp] === span
                ? 'border-primary bg-primary/10 text-primary font-medium'
                : 'border-gray-200 dark:border-gray-700 text-gray-600 dark:text-gray-400 hover:border-gray-300 dark:hover:border-gray-600'"
              @click="sizeConfig[bp] = span"
            >
              {{ SPAN_LABELS[span] }}
            </button>
          </div>
        </div>
      </div>

      <!-- 预览提示 -->
      <div class="mt-5 p-3 rounded-lg bg-blue-50 dark:bg-blue-500/10 text-xs text-blue-600 dark:text-blue-400 flex items-start gap-2">
        <Icon icon="ri:information-line" class="w-4 h-4 flex-shrink-0 mt-0.5" />
        <span>尺寸以12列网格为基准：1/4=占3列、1/2=占6列、全宽=占12列。小屏下建议选全宽以保证可读性。</span>
      </div>
    </template>

    <template #footer>
      <template v-if="selected">
        <button
          class="px-4 py-2 rounded-lg bg-gray-100 dark:bg-gray-800 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors mr-2"
          @click="backToSelect"
        >
          上一步
        </button>
        <button
          class="px-4 py-2 rounded-lg bg-primary text-sm text-white hover:opacity-90 transition-opacity"
          @click="confirmAdd"
        >
          <Icon icon="ri:add-line" class="w-4 h-4 inline align-middle mr-1" />
          添加组件
        </button>
      </template>
      <template v-else>
        <button
          class="px-4 py-2 rounded-lg bg-gray-100 dark:bg-gray-800 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors"
          @click="close"
        >
          关闭
        </button>
      </template>
    </template>
  </VModal>
</template>
