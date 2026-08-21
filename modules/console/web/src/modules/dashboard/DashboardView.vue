<script setup lang="ts">
import { computed, onMounted, reactive, ref, toRef } from "vue";
import { Icon } from "@iconify/vue";
import { widgetRegistry, type WidgetDefinition, type LayoutWidget, type ResponsiveSize, type WidgetSpan } from "./widgets/registry";
import {
  type Breakpoint,
  type SectionLayout,
  type MultiBreakpointLayout,
  loadLayoutFromStorage,
  saveLayoutToStorage,
  getDefaultSectionLayout,
} from "./widgets/layout";
import { moveWidget, type MoveRef } from "./widgets/layout-move";
import { useSortableSection } from "./useSortableSection";
import { provideBreakpoint } from "./useBreakpoint";
import WidgetCenter from "./WidgetCenter.vue";
import WidgetSizeModal from "./WidgetSizeModal.vue";
import { useAuthStore } from "@/stores/auth";
import "./dashboard.css";

const authStore = useAuthStore();
const editor = reactive({ editing: false });
const showWidgetCenter = ref(false);

// 提供断点上下文（支持视角切换预览）
const { breakpoint, viewMode, previewWidth, isPreview, setViewMode } = provideBreakpoint();

/** 多断点布局存储：l/m/s 各自独立 */
const multiLayout = ref<MultiBreakpointLayout>(loadLayoutFromStorage());

/** 当前激活断点对应的布局（computed，随breakpoint切换自动切换） */
const sections = computed<SectionLayout>(() => multiLayout.value[breakpoint.value]);

// ===== 拖拽：五个区域容器各自挂载 SortableJS =====
const statsEl = ref<HTMLElement | null>(null);
const chartWideEl = ref<HTMLElement | null>(null);
const chartNarrowEl = ref<HTMLElement | null>(null);
const recentsEl = ref<HTMLElement | null>(null);
const othersEl = ref<HTMLElement | null>(null);

/** 数据层移动：调用纯函数更新当前断点布局并持久化 */
function handleMove(from: MoveRef, to: MoveRef) {
  const bp = breakpoint.value;
  const layoutCopy = { ...multiLayout.value };
  const curBp = { ...layoutCopy[bp] };
  layoutCopy[bp] = moveWidget(curBp, from, to);
  multiLayout.value = layoutCopy;
  persist();
}

const editingRef = toRef(editor, "editing");
useSortableSection(statsEl, { section: "stats", editing: editingRef, onMove: handleMove });
useSortableSection(chartWideEl, { section: "chartWide", editing: editingRef, onMove: handleMove });
useSortableSection(chartNarrowEl, { section: "chartNarrow", editing: editingRef, onMove: handleMove });
useSortableSection(recentsEl, { section: "recents", editing: editingRef, onMove: handleMove });
useSortableSection(othersEl, { section: "others", editing: editingRef, onMove: handleMove });

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
  editor.editing = !editor.editing;
  if (!editor.editing) persist();
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

/** 根据组件类型推断其归属区域（仅供添加流程默认落区） */
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

// ===== 组件尺寸编辑弹窗 =====
const sizeEditor = ref<{ visible: boolean; type: string }>({ visible: false, type: "" });

function openSizeEditor(type: string) {
  sizeEditor.value = { visible: true, type };
}

/** 取当前断点下该组件的列跨度 */
const sizeEditorCurrentSpan = computed<WidgetSpan>(() => {
  const cur = sections.value;
  const all: LayoutWidget[] = [
    ...cur.stats,
    ...cur.chartNarrow,
    ...cur.recents,
    ...cur.others,
  ];
  if (cur.chartWide) all.push(cur.chartWide);
  return (all.find((w) => w.type === sizeEditor.value.type)?.size[breakpoint.value]) ?? 6;
});

const sizeEditorDef = computed<WidgetDefinition | undefined>(() =>
  widgetRegistry.find((w) => w.type === sizeEditor.value.type)
);

/** 仅更新当前断点下该组件的 size[bp]，其它断点不变 */
function applySize(span: WidgetSpan) {
  const bp = breakpoint.value;
  const type = sizeEditor.value.type;
  const layoutCopy = { ...multiLayout.value };
  const curBp = { ...layoutCopy[bp] };
  const updateIn = (list: LayoutWidget[]): LayoutWidget[] =>
    list.map((w) =>
      w.type === type ? { ...w, size: { ...w.size, [bp]: span } } : w
    );
  curBp.stats = updateIn(curBp.stats);
  curBp.chartNarrow = updateIn(curBp.chartNarrow);
  curBp.recents = updateIn(curBp.recents);
  curBp.others = updateIn(curBp.others);
  if (curBp.chartWide?.type === type) {
    curBp.chartWide = { ...curBp.chartWide, size: { ...curBp.chartWide.size, [bp]: span } };
  }
  layoutCopy[bp] = curBp;
  multiLayout.value = layoutCopy;
  persist();
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
        <div
          ref="statsEl"
          data-section="stats"
          v-if="sections.stats.length || editor.editing"
          class="dashboard-grid dashboard-grid-unified"
        >
          <div
            v-for="(w) in sections.stats"
            :key="w.type"
            :class="['widget-cell', editor.editing && 'editable']"
            :style="spanStyle(w.size)"
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
              <button class="apple-edit-size-btn" title="调整尺寸" @click.stop="openSizeEditor(w.type)">
                <Icon icon="ri:settings-3-line" class="w-4 h-4" />
              </button>
              <button class="apple-remove-btn" title="移除" @click.stop="removeWidget(w.type)">
                <Icon icon="ri:close-line" class="w-4 h-4" />
              </button>
            </div>
          </div>
          <div v-if="editor.editing && sections.stats.length === 0" class="apple-drop-placeholder">
            <Icon icon="ri:drag-drop-line" class="w-5 h-5" />
            拖入统计组件
          </div>
        </div>

        <!-- Section 2: Charts -->
        <div class="dashboard-grid dashboard-grid-charts mt-6">
          <!-- chartWide 专用可拖拽容器 -->
          <div ref="chartWideEl" data-section="chartWide" class="chart-wide-sortable">
            <div
              v-if="sections.chartWide && getWidgetDef(sections.chartWide.type)"
              :class="['widget-cell chart-wide-cell', editor.editing && 'editable']"
              :key="sections.chartWide.type"
            >
              <component
                :is="getWidgetDef(sections.chartWide.type)!.component"
                v-bind="getWidgetDef(sections.chartWide.type)!.props"
              />
              <div v-if="editor.editing" class="apple-edit-overlay">
                <span class="apple-drag-handle" title="拖拽排序">
                  <Icon icon="ri:drag-move-2-line" class="w-4 h-4" />
                </span>
                <button class="apple-edit-size-btn" title="调整尺寸" @click.stop="openSizeEditor(sections.chartWide!.type)">
                  <Icon icon="ri:settings-3-line" class="w-4 h-4" />
                </button>
                <button class="apple-remove-btn" title="移除" @click.stop="removeWidget(sections.chartWide!.type)">
                  <Icon icon="ri:close-line" class="w-4 h-4" />
                </button>
              </div>
            </div>
            <div v-else-if="editor.editing" class="apple-drop-placeholder">
              <Icon icon="ri:image-2-line" class="w-5 h-5" />
              拖入宽图组件
            </div>
          </div>

          <!-- chartNarrow 可拖拽容器 -->
          <div
            ref="chartNarrowEl"
            data-section="chartNarrow"
            v-if="sections.chartNarrow.length || editor.editing"
            class="narrow-stack"
          >
            <div
              v-for="(w) in sections.chartNarrow"
              :key="w.type"
              :class="['widget-cell', editor.editing && 'editable']"
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
                <button class="apple-edit-size-btn" title="调整尺寸" @click.stop="openSizeEditor(w.type)">
                  <Icon icon="ri:settings-3-line" class="w-4 h-4" />
                </button>
                <button class="apple-remove-btn" title="移除" @click.stop="removeWidget(w.type)">
                  <Icon icon="ri:close-line" class="w-4 h-4" />
                </button>
              </div>
            </div>
            <div v-if="editor.editing && sections.chartNarrow.length === 0" class="apple-drop-placeholder">
              <Icon icon="ri:drag-drop-line" class="w-5 h-5" />
              拖入窄图组件
            </div>
          </div>
        </div>

        <!-- Section 3: Recents -->
        <div
          ref="recentsEl"
          data-section="recents"
          v-if="sections.recents.length || editor.editing"
          class="dashboard-grid dashboard-grid-unified mt-6"
        >
          <div
            v-for="(w) in sections.recents"
            :key="w.type"
            :class="['widget-cell', editor.editing && 'editable']"
            :style="spanStyle(w.size)"
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
              <button class="apple-edit-size-btn" title="调整尺寸" @click.stop="openSizeEditor(w.type)">
                <Icon icon="ri:settings-3-line" class="w-4 h-4" />
              </button>
              <button class="apple-remove-btn" title="移除" @click.stop="removeWidget(w.type)">
                <Icon icon="ri:close-line" class="w-4 h-4" />
              </button>
            </div>
          </div>
          <div v-if="editor.editing && sections.recents.length === 0" class="apple-drop-placeholder">
            <Icon icon="ri:drag-drop-line" class="w-5 h-4" />
            拖入列表组件
          </div>
        </div>

        <!-- Section 4: Others -->
        <div
          ref="othersEl"
          data-section="others"
          v-if="sections.others.length || editor.editing"
          class="dashboard-grid dashboard-grid-unified mt-6"
        >
          <div
            v-for="(w) in sections.others"
            :key="w.type"
            :class="['widget-cell', editor.editing && 'editable']"
            :style="spanStyle(w.size)"
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
              <button class="apple-edit-size-btn" title="调整尺寸" @click.stop="openSizeEditor(w.type)">
                <Icon icon="ri:settings-3-line" class="w-4 h-4" />
              </button>
              <button class="apple-remove-btn" title="移除" @click.stop="removeWidget(w.type)">
                <Icon icon="ri:close-line" class="w-4 h-4" />
              </button>
            </div>
          </div>
          <div v-if="editor.editing && sections.others.length === 0" class="apple-drop-placeholder">
            <Icon icon="ri:drag-drop-line" class="w-5 h-4" />
            拖入其它组件
          </div>
        </div>

        <WidgetCenter
          v-model:visible="showWidgetCenter"
          :existing-types="existingTypes()"
          @add="addWidget"
        />

        <WidgetSizeModal
          v-model:visible="sizeEditor.visible"
          :widget-type="sizeEditor.type"
          :widget-label="sizeEditorDef?.label ?? ''"
          :widget-icon="sizeEditorDef?.icon ?? ''"
          :breakpoint="breakpoint"
          :current-span="sizeEditorCurrentSpan"
          @save="applySize"
        />
      </div>
    </div>
  </div>
</template>
