<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { Icon } from "@iconify/vue";
import { GridStack } from "gridstack";
import "gridstack/dist/gridstack.min.css";
import {
  widgetRegistry,
  type WidgetDefinition,
} from "./widgets/registry";
import {
  type Breakpoint,
  type GridConstraints,
  type GridSize,
  type GridWidget,
  type MultiBreakpointLayout,
  createDefaultGridWidget,
  getDefaultGridLayout,
  getDefaultGridSize,
  getGridConstraints,
  loadLayoutFromStorage,
  nodesToGridWidgets,
  saveLayoutToStorage,
} from "./widgets/layout";
import { provideBreakpoint } from "./useBreakpoint";
import WidgetCenter from "./WidgetCenter.vue";
import WidgetSizeModal from "./WidgetSizeModal.vue";
import { useAuthStore } from "@/stores/auth";
import "./dashboard.css";

const authStore = useAuthStore();
const editor = reactive({ editing: false });
const showWidgetCenter = ref(false);
const gridEl = ref<HTMLElement | null>(null);
let grid: GridStack | null = null;

// 提供断点上下文（支持视角切换预览）
const { breakpoint, viewMode, previewWidth, isPreview, setViewMode } = provideBreakpoint();

/** 多断点布局存储：l/m/s 各自独立。 */
const multiLayout = ref<MultiBreakpointLayout>(loadLayoutFromStorage());
const widgets = computed<GridWidget[]>(() => multiLayout.value[breakpoint.value].widgets);

function greeting(): string {
  const h = new Date().getHours();
  if (h < 6) return "夜深了";
  if (h < 12) return "早上好";
  if (h < 14) return "中午好";
  if (h < 18) return "下午好";
  return "晚上好";
}

function getToday(): string {
  const d = new Date();
  const weekdays = ["周日", "周一", "周二", "周三", "周四", "周五", "周六"];
  return `${d.getFullYear()}年${d.getMonth() + 1}月${d.getDate()}日 ${weekdays[d.getDay()]}`;
}

const displayName = computed(() => authStore.user?.display_name || authStore.user?.username || "管理员");

function getWidgetDef(type: string): WidgetDefinition | undefined {
  return widgetRegistry.find((w) => w.type === type);
}

function persist() {
  saveLayoutToStorage(multiLayout.value);
}

function updateCurrentWidgets(nextWidgets: GridWidget[]) {
  const bp = breakpoint.value;
  multiLayout.value = {
    ...multiLayout.value,
    [bp]: { widgets: nextWidgets },
  };
  persist();
}

function existingTypes(): string[] {
  return widgets.value.map((widget) => widget.type);
}

function readGridWidgets(): GridWidget[] {
  if (!grid || !gridEl.value) return widgets.value.map((item) => ({ ...item }));
  // GridStack 13 在 Vue 自定义内容节点上 save() 的尺寸字段可能退化为 1。
  // dragstop/resizestop 时它已经把最终几何写回 gs-* 属性，DOM 是此刻最可靠的快照。
  const nodes = Array.from(gridEl.value.querySelectorAll<HTMLElement>(".grid-stack-item"))
    .map((element) => ({
      id: element.getAttribute("gs-id") ?? "",
      x: Number(element.getAttribute("gs-x")),
      y: Number(element.getAttribute("gs-y")),
      w: Number(element.getAttribute("gs-w")),
      h: Number(element.getAttribute("gs-h")),
    }));
  return nodesToGridWidgets(nodes);
}

/** 网格引擎完成拖动/缩放后，将所有受碰撞影响的组件一次性保存。 */
function commitGrid() {
  const nextWidgets = readGridWidgets();
  if (nextWidgets.length > 0 || widgets.value.length === 0) updateCurrentWidgets(nextWidgets);
}

function setGridEditing(enabled: boolean) {
  if (!grid) return;
  grid.enableMove(enabled);
  grid.enableResize(enabled);
}

function mountGrid() {
  if (!gridEl.value || grid) return;
  grid = GridStack.init(
    {
      column: 12,
      cellHeight: 8,
      margin: 10,
      float: false,
      animate: true,
      alwaysShowResizeHandle: true,
      draggable: { handle: ".apple-drag-handle" },
      resizable: { handles: "e,se,s,sw,w" },
    },
    gridEl.value,
  );
  if (!grid) return;
  grid.on("dragstop", commitGrid);
  grid.on("resizestop", commitGrid);
  setGridEditing(editor.editing);
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

function toggleEdit() {
  editor.editing = !editor.editing;
  setGridEditing(editor.editing);
  if (!editor.editing) persist();
}

function addWidget(type: string) {
  if (existingTypes().includes(type)) return;
  const item = createDefaultGridWidget(type, breakpoint.value, widgets.value);
  updateCurrentWidgets([...widgets.value, item]);
  void rebuildGrid();
}

function removeWidget(type: string) {
  updateCurrentWidgets(widgets.value.filter((item) => item.type !== type));
  void rebuildGrid();
}

function resetLayout() {
  updateCurrentWidgets(getDefaultGridLayout(breakpoint.value).widgets);
  void rebuildGrid();
}

// ===== 组件尺寸编辑弹窗 =====
const sizeEditor = ref<{ visible: boolean; type: string }>({ visible: false, type: "" });

function openSizeEditor(type: string) {
  sizeEditor.value = { visible: true, type };
}

const sizeEditorWidget = computed<GridWidget | undefined>(() =>
  widgets.value.find((item) => item.type === sizeEditor.value.type),
);

const sizeEditorDef = computed<WidgetDefinition | undefined>(() =>
  getWidgetDef(sizeEditor.value.type),
);

const sizeEditorCurrentSize = computed<GridSize>(() => {
  const item = sizeEditorWidget.value;
  return item ? { w: item.w, h: item.h } : getDefaultGridSize(sizeEditor.value.type, breakpoint.value);
});

const sizeEditorConstraints = computed<GridConstraints>(() =>
  getGridConstraints(sizeEditor.value.type, breakpoint.value),
);

function findGridElement(type: string): HTMLElement | null {
  return Array.from(gridEl.value?.querySelectorAll<HTMLElement>(".grid-stack-item") ?? [])
    .find((element) => element.getAttribute("gs-id") === type) ?? null;
}

function applySize(size: GridSize) {
  const type = sizeEditor.value.type;
  const constraints = getGridConstraints(type, breakpoint.value);
  const nextSize = {
    w: Math.max(constraints.minW, Math.min(constraints.maxW, Math.round(size.w))),
    h: Math.max(constraints.minH, Math.min(constraints.maxH, Math.round(size.h))),
  };
  const element = findGridElement(type);
  if (grid && element) {
    grid.update(element, nextSize);
    commitGrid();
    return;
  }
  updateCurrentWidgets(widgets.value.map((item) =>
    item.type === type ? { ...item, ...nextSize } : item,
  ));
}

const viewModes: { key: "auto" | Breakpoint; label: string; icon: string }[] = [
  { key: "auto", label: "自适应", icon: "ri:aspect-ratio-line" },
  { key: "l", label: "大屏", icon: "ri:computer-line" },
  { key: "m", label: "中屏", icon: "ri:tablet-line" },
  { key: "s", label: "小屏", icon: "ri:smartphone-line" },
];

onMounted(() => {
  mountGrid();
});

watch(() => breakpoint.value, () => {
  void rebuildGrid();
});

watch(() => editor.editing, (enabled) => {
  setGridEditing(enabled);
});

onBeforeUnmount(() => {
  destroyGrid();
});
</script>

<template>
  <div class="dashboard-viewport" :class="{ 'is-preview': isPreview }">
    <div class="dashboard-canvas" :style="{ width: previewWidth }">
      <div class="dashboard-apple" :data-bp="breakpoint">
        <div class="flex flex-col sm:flex-row sm:items-end justify-between gap-4 apple-greeting">
          <div>
            <h1>{{ greeting() }}，{{ displayName }}</h1>
            <p>{{ getToday() }}，欢迎回到管理仪表盘</p>
          </div>
          <div class="apple-header-actions">
            <div class="apple-segmented view-switch">
              <button
                v-for="vm in viewModes"
                :key="vm.key"
                :class="['seg-btn', vm.key === viewMode && 'active']"
                :title="vm.label"
                :aria-label="vm.label"
                @click="setViewMode(vm.key)"
              >
                <Icon :icon="vm.icon" class="w-4 h-4" />
              </button>
            </div>
            <button class="apple-edit-btn" @click="toggleEdit">
              <Icon :icon="editor.editing ? 'ri:check-line' : 'ri:edit-line'" class="w-4 h-4" />
              {{ editor.editing ? "完成" : "编辑布局" }}
            </button>
            <button
              v-if="editor.editing"
              class="apple-icon-btn"
              title="添加组件"
              aria-label="添加组件"
              @click="showWidgetCenter = true"
            >
              <Icon icon="ri:add-line" class="w-5 h-5" />
            </button>
            <button
              v-if="editor.editing"
              class="apple-icon-btn"
              title="重置当前布局"
              aria-label="重置当前布局"
              @click="resetLayout"
            >
              <Icon icon="ri:refresh-line" class="w-4 h-4" />
            </button>
          </div>
        </div>

        <div v-if="editor.editing" class="apple-edit-hint">
          <Icon icon="ri:drag-move-2-line" class="w-4 h-4" />
          正在编辑「{{ viewModes.find((v) => v.key === breakpoint)?.label }}」布局，可拖拽组件并缩放宽高，修改只对当前尺寸生效
        </div>

        <div v-if="editor.editing && widgets.length === 0" class="apple-grid-empty">
          <Icon icon="ri:drag-drop-line" class="w-5 h-5" />
          拖入组件
        </div>

        <div ref="gridEl" class="grid-stack dashboard-grid-stack" :class="{ 'is-editing': editor.editing }">
          <div
            v-for="item in widgets"
            :key="item.type"
            class="grid-stack-item"
            :gs-id="item.type"
            :gs-x="item.x"
            :gs-y="item.y"
            :gs-w="item.w"
            :gs-h="item.h"
            :gs-min-w="getGridConstraints(item.type, breakpoint).minW"
            :gs-min-h="getGridConstraints(item.type, breakpoint).minH"
            :gs-max-w="getGridConstraints(item.type, breakpoint).maxW"
            :gs-max-h="getGridConstraints(item.type, breakpoint).maxH"
          >
            <div class="grid-stack-item-content widget-cell" :class="{ editable: editor.editing }">
              <component
                v-if="getWidgetDef(item.type)"
                :is="getWidgetDef(item.type)!.component"
                v-bind="getWidgetDef(item.type)!.props"
              />
              <div v-if="editor.editing" class="apple-edit-overlay">
                <span class="apple-drag-handle" role="button" tabindex="0" title="拖拽排序" :aria-label="`拖拽${getWidgetDef(item.type)?.label ?? '组件'}`">
                  <Icon icon="ri:drag-move-2-line" class="w-4 h-4" />
                </span>
                <button class="apple-edit-size-btn" type="button" title="调整宽高" aria-label="调整宽高" @click.stop="openSizeEditor(item.type)">
                  <Icon icon="ri:settings-3-line" class="w-4 h-4" />
                </button>
                <button class="apple-remove-btn" type="button" title="移除" aria-label="移除组件" @click.stop="removeWidget(item.type)">
                  <Icon icon="ri:close-line" class="w-4 h-4" />
                </button>
              </div>
            </div>
          </div>
        </div>

        <WidgetCenter
          v-model:visible="showWidgetCenter"
          :existing-types="existingTypes()"
          @add="addWidget"
        />

        <WidgetSizeModal
          v-model:visible="sizeEditor.visible"
          :widget-label="sizeEditorDef?.label ?? ''"
          :widget-icon="sizeEditorDef?.icon ?? ''"
          :breakpoint="breakpoint"
          :current-size="sizeEditorCurrentSize"
          :constraints="sizeEditorConstraints"
          @save="applySize"
        />
      </div>
    </div>
  </div>
</template>
