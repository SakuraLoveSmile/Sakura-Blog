<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import { widgetRegistry, type WidgetDefinition, type LayoutWidget, type ResponsiveSize } from "./widgets/registry";
import {
  type Breakpoint,
  type SectionLayout,
  type MultiBreakpointLayout,
  loadLayoutFromStorage,
  saveLayoutToStorage,
  getDefaultSectionLayout,
} from "./widgets/layout";
import { provideBreakpoint } from "./useBreakpoint";
import WidgetCenter from "./WidgetCenter.vue";
import { useAuthStore } from "@/stores/auth";
import "./dashboard.css";

const authStore = useAuthStore();
const editor = ref({ editing: false });
const showWidgetCenter = ref(false);

// 提供断点上下文（支持视角切换预览）
const { breakpoint, viewMode, previewWidth, isPreview, setViewMode } = provideBreakpoint();

/** 多断点布局存储：l/m/s 各自独立 */
const multiLayout = ref<MultiBreakpointLayout>(loadLayoutFromStorage());

/** 当前激活断点对应的布局（computed，随breakpoint切换自动切换） */
const sections = computed<SectionLayout>(() => multiLayout.value[breakpoint.value]);

// ===== 拖拽排序状态 =====
type SectionKey = "stats" | "chartNarrow" | "recents" | "others";
const dragState = ref<{
  section: SectionKey;
  fromIndex: number;
  type: string;
} | null>(null);
const dragOverIndex = ref<{ section: SectionKey; index: number } | null>(null);

function makeDragHandlers(section: SectionKey, index: number) {
  return {
    draggable: editor.value.editing,
    onDragstart(e: DragEvent) {
      if (!editor.value.editing) return;
      dragState.value = { section, fromIndex: index, type: sections.value[section][index].type };
      if (e.dataTransfer) {
        e.dataTransfer.effectAllowed = "move";
        e.dataTransfer.setData("text/plain", sections.value[section][index].type);
      }
    },
    onDragover(e: DragEvent) {
      if (!dragState.value || dragState.value.section !== section) return;
      e.preventDefault();
      if (e.dataTransfer) e.dataTransfer.dropEffect = "move";
      dragOverIndex.value = { section, index };
    },
    onDragleave() {
      if (dragOverIndex.value?.section === section && dragOverIndex.value.index === index) {
        dragOverIndex.value = null;
      }
    },
    onDrop(e: DragEvent) {
      e.preventDefault();
      if (!dragState.value || dragState.value.section !== section) return;
      const from = dragState.value.fromIndex;
      const to = index;
      if (from !== to) {
        const bp = breakpoint.value;
        const layoutCopy = { ...multiLayout.value };
        const curBp = { ...layoutCopy[bp] };
        const list = [...(curBp[section] as LayoutWidget[])];
        const [moved] = list.splice(from, 1);
        list.splice(to, 0, moved);
        (curBp[section] as LayoutWidget[]) = list;
        layoutCopy[bp] = curBp;
        multiLayout.value = layoutCopy;
        persist();
      }
      dragState.value = null;
      dragOverIndex.value = null;
    },
    onDragend() {
      dragState.value = null;
      dragOverIndex.value = null;
    },
  };
}

function dragCellClass(section: SectionKey, index: number): Record<string, boolean> {
  return {
    editable: editor.value.editing,
    dragging: dragState.value?.section === section && dragState.value.fromIndex === index,
    "drag-over-top":
      dragOverIndex.value?.section === section &&
      dragOverIndex.value.index === index &&
      (dragState.value?.fromIndex ?? -1) > index,
    "drag-over-bottom":
      dragOverIndex.value?.section === section &&
      dragOverIndex.value.index === index &&
      (dragState.value?.fromIndex ?? -1) < index,
  };
}

onMounted(() => {
  // 初始化已通过 ref 默认值完成
});

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

/** 仅保存当前状态到 localStorage */
function persist() {
  saveLayoutToStorage(multiLayout.value);
}

function toggleEdit() {
  editor.value.editing = !editor.value.editing;
  if (!editor.value.editing) persist();
}

/** 当前断点已存在的组件类型列表 */
function existingTypes(): string[] {
  const cur = sections.value;
  return [
    ...cur.stats.map((w) => w.type),
    cur.chartWide?.type,
    ...cur.chartNarrow.map((w) => w.type),
    ...cur.recents.map((w) => w.type),
    ...cur.others.map((w) => w.type),
  ].filter(Boolean) as string[];
}

/** 根据组件类型推断其归属区域 */
function getSectionForType(type: string): keyof SectionLayout {
  if (type.startsWith("stat-")) return "stats";
  if (type === "trend-chart") return "chartWide";
  if (["category-donut", "quick-actions", "hourly-chart"].includes(type)) return "chartNarrow";
  if (["recent-posts", "recent-comments"].includes(type)) return "recents";
  return "others";
}

/** 向当前断点布局添加组件 */
function addWidget(widget: LayoutWidget) {
  const bp = breakpoint.value;
  if (existingTypes().includes(widget.type)) return;
  const section = getSectionForType(widget.type);
  const layoutCopy = { ...multiLayout.value };
  const curBp = { ...layoutCopy[bp] };
  if (section === "chartWide") {
    curBp.chartWide = widget;
  } else {
    curBp[section] = [...(curBp[section] as LayoutWidget[]), widget];
  }
  layoutCopy[bp] = curBp;
  multiLayout.value = layoutCopy;
  persist();
}

function removeFromList(list: LayoutWidget[], type: string): LayoutWidget[] {
  return list.filter((w) => w.type !== type);
}

/** 从当前断点布局移除组件 */
function removeWidget(type: string) {
  const bp = breakpoint.value;
  const layoutCopy = { ...multiLayout.value };
  const curBp = { ...layoutCopy[bp] };
  if (curBp.chartWide?.type === type) {
    curBp.chartWide = null;
  } else {
    curBp.stats = removeFromList(curBp.stats, type);
    curBp.chartNarrow = removeFromList(curBp.chartNarrow, type);
    curBp.recents = removeFromList(curBp.recents, type);
    curBp.others = removeFromList(curBp.others, type);
  }
  layoutCopy[bp] = curBp;
  multiLayout.value = layoutCopy;
  persist();
}

/** 仅重置当前断点布局为默认 */
function resetLayout() {
  const bp = breakpoint.value;
  const layoutCopy = { ...multiLayout.value };
  layoutCopy[bp] = getDefaultSectionLayout(bp);
  multiLayout.value = layoutCopy;
  persist();
}

/** 根据当前断点获取组件的列跨度style */
function spanStyle(size: ResponsiveSize): Record<string, string> {
  const bp = breakpoint.value;
  const span = size[bp];
  return { gridColumn: `span ${span} / span ${span}` };
}

const viewModes: { key: "auto" | Breakpoint; label: string; icon: string }[] = [
  { key: "auto", label: "自适应", icon: "ri:aspect-ratio-line" },
  { key: "l",    label: "大屏",   icon: "ri:computer-line" },
  { key: "m",    label: "中屏",   icon: "ri:tablet-line" },
  { key: "s",    label: "小屏",   icon: "ri:smartphone-line" },
];
</script>

<template>
  <div class="dashboard-viewport" :class="{ 'is-preview': isPreview }">
    <div
      class="dashboard-canvas"
      :style="{ width: previewWidth }"
    >
      <div class="dashboard-apple" :data-bp="breakpoint">
        <!-- Header -->
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
              @click="showWidgetCenter = true"
            >
              <Icon icon="ri:add-line" class="w-5 h-5" />
            </button>
            <button
              v-if="editor.editing"
              class="apple-icon-btn"
              title="重置当前尺寸布局"
              @click="resetLayout"
            >
              <Icon icon="ri:refresh-line" class="w-4 h-4" />
            </button>
          </div>
        </div>

        <!-- 编辑模式提示 -->
        <div v-if="editor.editing" class="apple-edit-hint">
          <Icon icon="ri:drag-move-2-line" class="w-4 h-4" />
          正在编辑「{{ viewModes.find(v => v.key === breakpoint)?.label }}」布局，可拖拽调整顺序，修改只对当前尺寸生效
        </div>

        <!-- Section 1: Stats -->
        <div v-if="sections.stats.length" class="dashboard-grid dashboard-grid-unified">
          <div
            v-for="(w, i) in sections.stats"
            :key="w.type"
            :class="['widget-cell', dragCellClass('stats', i)]"
            :style="spanStyle(w.size)"
            v-bind="makeDragHandlers('stats', i)"
          >
            <component
              v-if="getWidgetDef(w.type)"
              :is="getWidgetDef(w.type)!.component"
              v-bind="getWidgetDef(w.type)!.props"
            />
            <div v-if="editor.editing" class="apple-edit-overlay">
              <span class="apple-drag-handle" title="拖拽排序">
                <Icon icon="ri:drag-move-2-line" class="w-4 h-4" />
              </span>
              <button class="apple-remove-btn" title="移除" @click.stop="removeWidget(w.type)">
                <Icon icon="ri:close-line" class="w-4 h-4" />
              </button>
            </div>
          </div>
        </div>

        <!-- Section 2: Charts -->
        <div class="dashboard-grid dashboard-grid-charts mt-6">
          <div
            v-if="sections.chartWide && getWidgetDef(sections.chartWide.type)"
            :class="['widget-cell chart-wide-cell', editor.editing && 'editable']"
          >
            <component
              :is="getWidgetDef(sections.chartWide.type)!.component"
              v-bind="getWidgetDef(sections.chartWide.type)!.props"
            />
            <div v-if="editor.editing" class="apple-edit-overlay">
              <button class="apple-remove-btn" title="移除" @click="removeWidget(sections.chartWide!.type)">
                <Icon icon="ri:close-line" class="w-4 h-4" />
              </button>
            </div>
          </div>
          <div v-if="sections.chartNarrow.length" class="narrow-stack">
            <div
              v-for="(w, i) in sections.chartNarrow"
              :key="w.type"
              :class="['widget-cell', dragCellClass('chartNarrow', i)]"
              v-bind="makeDragHandlers('chartNarrow', i)"
            >
              <component
                v-if="getWidgetDef(w.type)"
                :is="getWidgetDef(w.type)!.component"
                v-bind="getWidgetDef(w.type)!.props"
              />
              <div v-if="editor.editing" class="apple-edit-overlay">
                <span class="apple-drag-handle" title="拖拽排序">
                  <Icon icon="ri:drag-move-2-line" class="w-4 h-4" />
                </span>
                <button class="apple-remove-btn" title="移除" @click.stop="removeWidget(w.type)">
                  <Icon icon="ri:close-line" class="w-4 h-4" />
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Section 3: Recents -->
        <div v-if="sections.recents.length" class="dashboard-grid dashboard-grid-unified mt-6">
          <div
            v-for="(w, i) in sections.recents"
            :key="w.type"
            :class="['widget-cell', dragCellClass('recents', i)]"
            :style="spanStyle(w.size)"
            v-bind="makeDragHandlers('recents', i)"
          >
            <component
              v-if="getWidgetDef(w.type)"
              :is="getWidgetDef(w.type)!.component"
              v-bind="getWidgetDef(w.type)!.props"
            />
            <div v-if="editor.editing" class="apple-edit-overlay">
              <span class="apple-drag-handle" title="拖拽排序">
                <Icon icon="ri:drag-move-2-line" class="w-4 h-4" />
              </span>
              <button class="apple-remove-btn" title="移除" @click.stop="removeWidget(w.type)">
                <Icon icon="ri:close-line" class="w-4 h-4" />
              </button>
            </div>
          </div>
        </div>

        <!-- Section 4: Others -->
        <div v-if="sections.others.length" class="dashboard-grid dashboard-grid-unified mt-6">
          <div
            v-for="(w, i) in sections.others"
            :key="w.type"
            :class="['widget-cell', dragCellClass('others', i)]"
            :style="spanStyle(w.size)"
            v-bind="makeDragHandlers('others', i)"
          >
            <component
              v-if="getWidgetDef(w.type)"
              :is="getWidgetDef(w.type)!.component"
              v-bind="getWidgetDef(w.type)!.props"
            />
            <div v-if="editor.editing" class="apple-edit-overlay">
              <span class="apple-drag-handle" title="拖拽排序">
                <Icon icon="ri:drag-move-2-line" class="w-4 h-4" />
              </span>
              <button class="apple-remove-btn" title="移除" @click.stop="removeWidget(w.type)">
                <Icon icon="ri:close-line" class="w-4 h-4" />
              </button>
            </div>
          </div>
        </div>

        <WidgetCenter
          v-model:visible="showWidgetCenter"
          :existing-types="existingTypes()"
          @add="addWidget"
        />
      </div>
    </div>
  </div>
</template>
