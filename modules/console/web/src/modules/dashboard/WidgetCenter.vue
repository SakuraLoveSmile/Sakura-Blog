<script setup lang="ts">
import { computed } from "vue";
import { Icon } from "@iconify/vue";
import { VModal } from "@/components/ui";
import { widgetRegistry, type WidgetDefinition } from "./widgets/registry";

interface Props {
  visible: boolean;
  existingTypes: string[];
}

const props = defineProps<Props>();
const emit = defineEmits<{
  "update:visible": [value: boolean];
  "add": [type: string];
}>();

const modalVisible = computed({
  get: () => props.visible,
  set: (v: boolean) => emit("update:visible", v),
});

const available = computed(() =>
  widgetRegistry.filter((w) => !props.existingTypes.includes(w.type)),
);

function close() {
  emit("update:visible", false);
}

function addWidget(widget: WidgetDefinition) {
  emit("add", widget.type);
  close();
}
</script>

<template>
  <VModal v-model:visible="modalVisible" title="添加小组件" :width="560">
    <p class="mb-4 text-sm text-text-quaternary dark:text-muted-foreground">
      选择组件后会放入当前断点的第一个空位，之后可以直接拖拽或缩放。
    </p>
    <div v-if="available.length === 0" class="py-8 text-center text-sm text-muted-foreground">
      <Icon icon="ri:check-double-line" class="w-12 h-12 mx-auto mb-2 opacity-50" />
      所有可用组件都已添加
    </div>
    <div v-else class="grid grid-cols-1 sm:grid-cols-2 gap-3 max-h-96 overflow-y-auto pr-1">
      <button
        v-for="widget in available"
        :key="widget.type"
        class="flex items-center gap-3 p-3 rounded-xl border border-border dark:border-border-strong hover:border-primary/60 hover:bg-secondary dark:hover:bg-secondary/50 text-left transition-all group"
        @click="addWidget(widget)"
      >
        <div class="w-10 h-10 rounded-lg bg-secondary dark:bg-secondary flex items-center justify-center text-text-quaternary dark:text-muted-foreground group-hover:text-primary transition-colors">
          <Icon :icon="widget.icon" class="w-5 h-5" />
        </div>
        <div class="flex-1 min-w-0">
          <div class="text-sm font-medium text-text-quaternary dark:text-foreground">{{ widget.label }}</div>
          <div class="text-xs text-muted-foreground dark:text-text-quaternary mt-0.5">添加后可自由调整宽高</div>
        </div>
        <Icon icon="ri:add-line" class="w-5 h-5 text-text-secondary dark:text-text-quaternary group-hover:text-primary transition-colors" />
      </button>
    </div>

    <template #footer>
      <button
        class="px-4 py-2 rounded-lg bg-secondary dark:bg-secondary text-sm text-text-quaternary dark:text-text-secondary hover:bg-accent dark:hover:bg-accent transition-colors"
        @click="close"
      >
        关闭
      </button>
    </template>
  </VModal>
</template>
