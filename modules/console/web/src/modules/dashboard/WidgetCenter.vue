<script setup lang="ts">
import { computed } from "vue";
import { Icon } from "@iconify/vue";
import { VModal } from "@/components/ui";
import { widgetRegistry, type WidgetDefinition } from "./widgets/registry";

interface Props {
  visible: boolean;
}

const props = defineProps<Props>();
const emit = defineEmits<{
  "update:visible": [value: boolean];
  add: [id: string];
}>();

const modalVisible = computed({
  get: () => props.visible,
  set: (v: boolean) => emit("update:visible", v),
});

const grouped = computed(() => {
  const groups = new Map<string, WidgetDefinition[]>();
  for (const widget of widgetRegistry) {
    const items = groups.get(widget.group) ?? [];
    items.push(widget);
    groups.set(widget.group, items);
  }
  return [...groups.entries()];
});

const groupLabels: Record<string, string> = {
  statistics: "数据统计",
  content: "内容",
  tools: "工具",
  system: "系统",
  general: "其他",
};

function close() {
  emit("update:visible", false);
}

function addWidget(widget: WidgetDefinition) {
  emit("add", widget.id);
  close();
}
</script>

<template>
  <VModal v-model:visible="modalVisible" title="添加组件" :width="600">
    <p class="mb-4 text-sm text-text-quaternary dark:text-muted-foreground">
      选择组件后会放入当前设备布局的第一个空位，之后可以拖拽位置或选择大、中、小尺寸。
    </p>
    <div class="max-h-96 overflow-y-auto pr-1 space-y-5">
      <section v-for="[group, widgets] in grouped" :key="group">
        <h3 class="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          {{ groupLabels[group] ?? group }}
        </h3>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <button
            v-for="widget in widgets"
            :key="widget.id"
            class="flex items-center gap-3 p-3 rounded-xl border border-border dark:border-border-strong hover:border-primary/60 hover:bg-secondary dark:hover:bg-secondary/50 text-left transition-all group"
            @click="addWidget(widget)"
          >
            <div
              class="w-10 h-10 rounded-lg bg-secondary dark:bg-secondary flex items-center justify-center text-text-quaternary dark:text-muted-foreground group-hover:text-primary transition-colors"
            >
              <Icon :icon="widget.icon" class="w-5 h-5" />
            </div>
            <div class="flex-1 min-w-0">
              <div class="text-sm font-medium text-text-quaternary dark:text-foreground">{{ widget.label }}</div>
              <div class="text-xs text-muted-foreground dark:text-text-quaternary mt-0.5">
                默认大小可在组件设置中切换
              </div>
            </div>
            <Icon
              icon="ri:add-line"
              class="w-5 h-5 text-text-secondary dark:text-text-quaternary group-hover:text-primary transition-colors"
            />
          </button>
        </div>
      </section>
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
