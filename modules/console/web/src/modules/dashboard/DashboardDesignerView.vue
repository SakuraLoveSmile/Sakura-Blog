<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { Icon } from "@iconify/vue";
import { useRouter, onBeforeRouteLeave } from "vue-router";
import { Toast } from "@/components/ui";
import { dashboardApi } from "@/api";
import DashboardGrid from "./DashboardGrid.vue";
import WidgetCenter from "./WidgetCenter.vue";
import WidgetSizeModal from "./WidgetSizeModal.vue";
import { provideBreakpoint, type ViewMode } from "./useBreakpoint";
import { useDashboardLayout } from "./useDashboardLayout";
import {
  copyLayoutToBreakpoint,
  applyGridWidgetSize,
  createDefaultGridWidget,
  ensureCompleteLayout,
  getDefaultGridLayout,
  getWidgetSizePreset,
  loadLegacyLayoutFromStorage,
  type DashboardLayoutBreakpoint,
  type DashboardResponsiveLayout,
  type DashboardWidget,
  type MultiBreakpointLayout,
  type WidgetSizePreset,
} from "./widgets/layout";
import { getWidgetDefinition } from "./widgets/registry";
import "./dashboard.css";

const router = useRouter();
const { breakpoint: autoBreakpoint, previewWidth, viewMode, setViewMode } = provideBreakpoint();
const { layout, loading, error, hasServerLayout, load, replace } = useDashboardLayout();

const currentBreakpoint = ref<DashboardLayoutBreakpoint>("lg");
const activeBreakpoint = computed<DashboardLayoutBreakpoint>(() =>
  viewMode.value === "auto" ? autoBreakpoint.value : currentBreakpoint.value,
);
const originalLayout = ref<MultiBreakpointLayout | null>(null);
const showWidgetCenter = ref(false);
const legacyLayout = ref<DashboardResponsiveLayout | null>(null);
const copySource = ref<DashboardLayoutBreakpoint>("lg");
const refreshKey = ref(0);
const saving = ref(false);
const sizingWidgetId = ref<string | null>(null);

const currentWidgets = computed(() => layout.value[activeBreakpoint.value] ?? []);
const sizingWidget = computed(() => currentWidgets.value.find((item) => item.i === sizingWidgetId.value) ?? null);
const sizingDefinition = computed(() => {
  const widget = sizingWidget.value;
  if (!widget) return undefined;
  return (
    getWidgetDefinition(widget.id) ?? {
      label: `未知组件：${widget.id}`,
      icon: "ri:question-line",
    }
  );
});
const sizeModalVisible = computed({
  get: () => Boolean(sizingWidget.value && sizingDefinition.value),
  set: (visible: boolean) => {
    if (!visible) sizingWidgetId.value = null;
  },
});
const sizingPreset = computed<WidgetSizePreset>(() =>
  sizingWidget.value
    ? getWidgetSizePreset(sizingWidget.value, activeBreakpoint.value, sizingWidget.value.id)
    : "medium",
);
const dirty = computed(
  () => originalLayout.value !== null && JSON.stringify(layout.value) !== JSON.stringify(originalLayout.value),
);
const canImportLegacy = computed(() => !hasServerLayout.value && Boolean(legacyLayout.value));

const viewOptions: { key: ViewMode; label: string; icon: string }[] = [
  { key: "auto", label: "自适应", icon: "ri:aspect-ratio-line" },
  { key: "lg", label: "电脑", icon: "ri:computer-line" },
  { key: "sm", label: "平板", icon: "ri:tablet-line" },
  { key: "xxs", label: "手机", icon: "ri:smartphone-line" },
];
const deviceOptions: { key: DashboardLayoutBreakpoint; label: string }[] = [
  { key: "lg", label: "电脑" },
  { key: "sm", label: "平板" },
  { key: "xxs", label: "手机" },
];

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

function switchBreakpoint(next: ViewMode) {
  if (next === viewMode.value) return;
  if (dirty.value && !window.confirm("当前布局尚未保存，切换设备会保留修改但不会丢失。是否继续？")) return;
  if (next !== "auto") currentBreakpoint.value = next;
  setViewMode(next);
}

function updateCurrentWidgets(next: DashboardWidget[]) {
  layout.value = { ...layout.value, [activeBreakpoint.value]: next };
}

function addWidget(id: string) {
  updateCurrentWidgets([
    ...currentWidgets.value,
    createDefaultGridWidget(id, activeBreakpoint.value, currentWidgets.value),
  ]);
  refreshKey.value += 1;
}

function removeWidget(instanceId: string) {
  updateCurrentWidgets(currentWidgets.value.filter((widget) => widget.i !== instanceId));
  refreshKey.value += 1;
}

function resetCurrentLayout() {
  updateCurrentWidgets(getDefaultGridLayout(activeBreakpoint.value));
  refreshKey.value += 1;
}

function copyLayout() {
  const target = activeBreakpoint.value;
  if (copySource.value === target) {
    Toast.info("请选择其他设备布局作为来源");
    return;
  }
  layout.value = copyLayoutToBreakpoint(layout.value, copySource.value, target) as MultiBreakpointLayout;
  refreshKey.value += 1;
  Toast.success("布局已复制到当前设备");
}

function importLegacy() {
  if (!legacyLayout.value) return;
  replace(legacyLayout.value);
  legacyLayout.value = null;
  refreshKey.value += 1;
  Toast.success("旧版浏览器布局已导入，请点击保存");
}

async function save() {
  saving.value = true;
  try {
    const complete = ensureCompleteLayout(layout.value);
    await dashboardApi.saveLayout(complete);
    replace(complete);
    originalLayout.value = clone(complete);
    hasServerLayout.value = true;
    Toast.success("仪表盘布局已保存");
  } catch (err: any) {
    Toast.error(err?.response?.data?.error || "仪表盘布局保存失败");
  } finally {
    saving.value = false;
  }
}

function configureWidget(instanceId: string) {
  if (currentWidgets.value.some((item) => item.i === instanceId)) sizingWidgetId.value = instanceId;
}

function saveWidgetSize(preset: WidgetSizePreset) {
  if (!sizingWidget.value) return;
  const updated = currentWidgets.value.map((item) =>
    item.i === sizingWidget.value?.i
      ? applyGridWidgetSize(item, preset, activeBreakpoint.value, currentWidgets.value)
      : item,
  );
  updateCurrentWidgets(updated);
  refreshKey.value += 1;
  sizingWidgetId.value = null;
  Toast.success(`已切换为${preset === "small" ? "小" : preset === "large" ? "大" : "中"}尺寸`);
}

function leave() {
  router.push("/dashboard");
}

function beforeUnload(event: BeforeUnloadEvent) {
  if (!dirty.value) return;
  event.preventDefault();
  event.returnValue = "布局尚未保存";
}

onMounted(async () => {
  setViewMode("auto");
  await load();
  originalLayout.value = clone(layout.value);
  if (!hasServerLayout.value) legacyLayout.value = loadLegacyLayoutFromStorage();
});

watch(activeBreakpoint, () => {
  // 自适应模式跨越断点时关闭当前尺寸弹窗，避免继续操作旧断点的实例。
  sizingWidgetId.value = null;
});

window.addEventListener("beforeunload", beforeUnload);
onBeforeUnmount(() => window.removeEventListener("beforeunload", beforeUnload));

onBeforeRouteLeave(() => {
  if (!dirty.value) return true;
  return window.confirm("布局尚未保存，确定要离开吗？");
});
</script>

<template>
  <div class="dashboard-viewport is-preview">
    <div class="dashboard-canvas" :style="{ width: previewWidth }">
      <div class="dashboard-apple dashboard-designer" :data-bp="activeBreakpoint">
        <div class="flex flex-col gap-4 apple-greeting">
          <div class="flex flex-col lg:flex-row lg:items-end justify-between gap-4">
            <div>
              <h1>设置仪表盘</h1>
              <p>可自适应预览，也可按设备尺寸分别调整组件布局，保存后对当前账号生效</p>
            </div>
            <div class="apple-header-actions">
              <button class="apple-secondary-btn" @click="leave">
                <Icon icon="ri:arrow-left-line" class="w-4 h-4" />
                返回
              </button>
              <button class="apple-edit-btn" :disabled="saving || loading" @click="save">
                <Icon
                  :icon="saving ? 'ri:loader-4-line' : 'ri:save-3-line'"
                  :class="['w-4 h-4', saving && 'animate-spin']"
                />
                {{ saving ? "保存中" : "保存布局" }}
              </button>
            </div>
          </div>

          <div class="dashboard-device-tabs" role="tablist" aria-label="设备断点">
            <button
              v-for="option in viewOptions"
              :key="option.key"
              :class="['dashboard-device-tab', option.key === viewMode && 'active']"
              role="tab"
              :aria-selected="option.key === viewMode"
              @click="switchBreakpoint(option.key)"
            >
              <Icon :icon="option.icon" class="w-4 h-4" />
              {{ option.label }}
            </button>
          </div>
        </div>

        <div v-if="canImportLegacy" class="dashboard-import-hint">
          <Icon icon="ri:history-line" class="w-5 h-5" />
          <span>检测到旧版浏览器布局。服务端布局为空，不会自动覆盖；可以导入后检查，再手动保存。</span>
          <button class="apple-link" @click="importLegacy">导入当前浏览器布局</button>
        </div>

        <div class="dashboard-designer-toolbar">
          <div class="flex items-center gap-2 flex-wrap">
            <button class="apple-primary-btn" @click="showWidgetCenter = true">
              <Icon icon="ri:add-line" class="w-4 h-4" />
              添加组件
            </button>
            <button class="apple-secondary-btn" @click="resetCurrentLayout">
              <Icon icon="ri:refresh-line" class="w-4 h-4" />
              重置当前布局
            </button>
          </div>
          <div class="flex items-center gap-2 flex-wrap text-sm text-muted-foreground">
            <label for="dashboard-copy-source">复制来源</label>
            <select id="dashboard-copy-source" v-model="copySource" class="dashboard-select">
              <option v-for="option in deviceOptions" :key="option.key" :value="option.key">{{ option.label }}</option>
            </select>
            <button class="apple-secondary-btn" @click="copyLayout">
              <Icon icon="ri:file-copy-line" class="w-4 h-4" />
              复制到当前断点
            </button>
          </div>
        </div>

        <div v-if="loading" class="apple-grid-empty">
          <Icon icon="ri:loader-4-line" class="w-5 h-5 animate-spin" />
          正在加载布局
        </div>
        <div v-else-if="error" class="apple-grid-empty text-state-error">{{ error }}</div>
        <template v-else>
          <div class="apple-edit-hint">
            <Icon icon="ri:drag-move-2-line" class="w-4 h-4" />
            拖动组件标题区域调整位置，点击设置选择大、中、小尺寸；修改只在点击“保存布局”后写入服务端
          </div>
          <DashboardGrid
            :widgets="currentWidgets"
            :breakpoint="activeBreakpoint"
            :editing="true"
            :refresh-key="refreshKey"
            @change="updateCurrentWidgets"
            @remove="removeWidget"
            @configure="configureWidget"
          />
          <WidgetSizeModal
            v-if="sizingWidget && sizingDefinition"
            v-model:visible="sizeModalVisible"
            :widget-id="sizingWidget.id"
            :widget-label="sizingDefinition.label"
            :widget-icon="sizingDefinition.icon"
            :breakpoint="activeBreakpoint"
            :current-preset="sizingPreset"
            @save="saveWidgetSize"
          />
        </template>

        <WidgetCenter v-model:visible="showWidgetCenter" @add="addWidget" />
      </div>
    </div>
  </div>
</template>
