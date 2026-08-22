import { ref, computed, onMounted, onBeforeUnmount, provide, inject, type Ref } from "vue";
import { getBreakpoint, type DashboardLayoutBreakpoint } from "./widgets/layout";

export type ViewMode = "auto" | DashboardLayoutBreakpoint;

/** 每个视角模式对应的预览宽度（px） */
export const VIEW_MODE_WIDTH: Record<DashboardLayoutBreakpoint, number> = {
  lg: 1440,
  sm: 800,
  xxs: 390,
};

const BREAKPOINT_KEY = Symbol("dashboard-breakpoint");

/**
 * 响应式断点 + 视角预览控制
 * - auto模式：跟随窗口宽度自动切换断点
 * - 三种设备布局：手动锁定到电脑、平板或手机布局，用于预览
 *
 * 响应式样式统一通过 data-bp 属性驱动（替代CSS媒体查询），
 * 确保自动模式和预览模式样式完全一致。
 */
export function provideBreakpoint() {
  const viewportWidth = ref(window.innerWidth);
  const viewMode = ref<ViewMode>("auto");

  const breakpoint = computed<DashboardLayoutBreakpoint>(() => {
    if (viewMode.value !== "auto") return viewMode.value;
    return getBreakpoint(viewportWidth.value);
  });

  /** 预览容器宽度（auto模式为100%） */
  const previewWidth = computed(() => {
    if (viewMode.value === "auto") return "100%";
    return `${VIEW_MODE_WIDTH[viewMode.value]}px`;
  });

  /** 是否为预览模式（非auto） */
  const isPreview = computed(() => viewMode.value !== "auto");

  function updateWidth() {
    viewportWidth.value = window.innerWidth;
  }

  function setViewMode(mode: ViewMode) {
    viewMode.value = mode;
  }

  onMounted(() => {
    updateWidth();
    window.addEventListener("resize", updateWidth);
  });

  onBeforeUnmount(() => {
    window.removeEventListener("resize", updateWidth);
  });

  const api = {
    breakpoint,
    viewMode,
    viewportWidth,
    previewWidth,
    isPreview,
    setViewMode,
  };

  provide(BREAKPOINT_KEY, api);
  return api;
}

/** 子组件（widget）注入当前断点 */
export function useBreakpoint(): { breakpoint: Ref<DashboardLayoutBreakpoint> } {
  const bp = inject<{ breakpoint: Ref<DashboardLayoutBreakpoint> }>(BREAKPOINT_KEY);
  if (!bp) {
    // fallback：独立使用时跟随窗口
    const fallbackBp = ref<DashboardLayoutBreakpoint>(getBreakpoint(window.innerWidth));
    const update = () => (fallbackBp.value = getBreakpoint(window.innerWidth));
    onMounted(() => {
      update();
      window.addEventListener("resize", update);
    });
    onBeforeUnmount(() => window.removeEventListener("resize", update));
    return { breakpoint: fallbackBp };
  }
  return bp;
}
