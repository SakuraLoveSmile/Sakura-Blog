import { ref, computed, onMounted, onBeforeUnmount, provide, inject, type Ref } from "vue";
import { getBreakpoint, type Breakpoint } from "./widgets/layout";

export type ViewMode = "auto" | Breakpoint;

/** 每个视角模式对应的预览宽度（px） */
export const VIEW_MODE_WIDTH: Record<Exclude<ViewMode, "auto">, number> = {
  l: 1440,
  m: 1024,
  s: 390,
};

const BREAKPOINT_KEY = Symbol("dashboard-breakpoint");

/**
 * 响应式断点 + 视角预览控制
 * - auto模式：跟随窗口宽度自动切换断点
 * - l/m/s模式：手动锁定到对应断点，用于预览
 *
 * 响应式样式统一通过 data-bp 属性驱动（替代CSS媒体查询），
 * 确保自动模式和预览模式样式完全一致。
 */
export function provideBreakpoint() {
  const viewportWidth = ref(window.innerWidth);
  const viewMode = ref<ViewMode>("auto");

  const breakpoint = computed<Breakpoint>(() => {
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
export function useBreakpoint(): { breakpoint: Ref<Breakpoint> } {
  const bp = inject<{ breakpoint: Ref<Breakpoint> }>(BREAKPOINT_KEY);
  if (!bp) {
    // fallback：独立使用时跟随窗口
    const fallbackBp = ref<Breakpoint>(getBreakpoint(window.innerWidth));
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
