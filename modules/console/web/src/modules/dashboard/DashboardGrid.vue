<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { GridStack } from "gridstack";
import "gridstack/dist/gridstack.min.css";
import { Icon } from "@iconify/vue";
import {
  getGridConstraints,
  getGridColumns,
  mergeGridWidgetMetadata,
  nodesToGridWidgets,
  type Breakpoint,
  type DashboardWidget,
} from "./widgets/layout";
import { getWidgetDefinition } from "./widgets/registry";
import UnknownWidget from "./widgets/UnknownWidget.vue";
import "./dashboard.css";

interface Props {
  widgets: DashboardWidget[];
  breakpoint: Breakpoint;
  editing?: boolean;
  refreshKey?: number;
}

const props = withDefaults(defineProps<Props>(), { editing: false, refreshKey: 0 });
const emit = defineEmits<{
  change: [widgets: DashboardWidget[]];
  remove: [instanceId: string];
  configure: [instanceId: string];
}>();

const gridEl = ref<HTMLElement | null>(null);
let grid: GridStack | null = null;

function readGridWidgets(): DashboardWidget[] {
  if (!gridEl.value) return props.widgets.map((item) => ({ ...item }));
  const source = new Map(props.widgets.map((item) => [item.i, item]));
  const nodes = Array.from(gridEl.value.querySelectorAll<HTMLElement>(".grid-stack-item"))
    .map((element) => ({
      i: element.getAttribute("gs-id") ?? "",
      widgetId: element.getAttribute("data-widget-id") ?? "",
      x: Number(element.getAttribute("gs-x")),
      y: Number(element.getAttribute("gs-y")),
      w: Number(element.getAttribute("gs-w")),
      h: Number(element.getAttribute("gs-h")),
    }));
  return mergeGridWidgetMetadata(nodesToGridWidgets(nodes, props.breakpoint), source);
}

function commitGrid() {
  emit("change", readGridWidgets());
}

function setEditing(enabled: boolean) {
  grid?.enableMove(enabled);
  grid?.enableResize(enabled);
}

function mountGrid() {
  if (!gridEl.value || grid) return;
  const instance = GridStack.init({
    column: getGridColumns(props.breakpoint),
    cellHeight: 8,
    margin: 10,
    float: false,
    animate: true,
    disableDrag: !props.editing,
    disableResize: !props.editing,
    alwaysShowResizeHandle: props.editing,
    draggable: { handle: ".apple-drag-handle" },
    resizable: { handles: "e,se,s,sw,w" },
  }, gridEl.value);
  if (!instance) return;
  grid = instance;
  instance.on("dragstop", commitGrid);
  instance.on("resizestop", commitGrid);
  setEditing(props.editing);
}

function destroyGrid() {
  grid?.offAll();
  grid?.destroy(false);
  grid = null;
}

async function rebuildGrid() {
  destroyGrid();
  await nextTick();
  mountGrid();
}

watch(() => props.breakpoint, () => void rebuildGrid());
watch(() => props.refreshKey, () => void rebuildGrid());
watch(() => props.editing, (enabled) => setEditing(enabled));

onMounted(mountGrid);
onBeforeUnmount(destroyGrid);
</script>

<template>
  <div
    ref="gridEl"
    class="grid-stack dashboard-grid-stack"
    :class="{ 'is-editing': editing }"
    :style="{ '--gs-column-width': `${100 / getGridColumns(breakpoint)}%` }"
  >
    <div
      v-for="item in widgets"
      :key="item.i"
      class="grid-stack-item"
      :gs-id="item.i"
      :data-widget-id="item.id"
      :gs-x="item.x"
      :gs-y="item.y"
      :gs-w="item.w"
      :gs-h="item.h"
      :gs-min-w="item.minW ?? getGridConstraints(item.id, breakpoint).minW"
      :gs-min-h="item.minH ?? getGridConstraints(item.id, breakpoint).minH"
      :gs-max-w="item.maxW ?? getGridConstraints(item.id, breakpoint).maxW"
      :gs-max-h="item.maxH ?? getGridConstraints(item.id, breakpoint).maxH"
    >
      <div class="grid-stack-item-content widget-cell" :class="{ editable: editing }">
        <component
          v-if="getWidgetDefinition(item.id)"
          :is="getWidgetDefinition(item.id)!.component"
          v-bind="getWidgetDefinition(item.id)!.props"
        />
        <UnknownWidget v-else :widget-id="item.id" />
        <div v-if="editing" class="apple-edit-overlay">
          <span class="apple-drag-handle" role="button" tabindex="0" title="拖拽排序" :aria-label="`拖拽${getWidgetDefinition(item.id)?.label ?? '组件'}`">
            <Icon icon="ri:drag-move-2-line" class="w-4 h-4" />
          </span>
          <button class="apple-edit-size-btn" type="button" title="组件设置" aria-label="组件设置" @click.stop="emit('configure', item.i)">
            <Icon icon="ri:settings-3-line" class="w-4 h-4" />
          </button>
          <button class="apple-remove-btn" type="button" title="移除" aria-label="移除组件" @click.stop="emit('remove', item.i)">
            <Icon icon="ri:close-line" class="w-4 h-4" />
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
