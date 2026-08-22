import { ref } from "vue";
import { dashboardApi } from "@/api";
import {
  defaultMultiLayout,
  ensureCompleteLayout,
  sanitizeDashboardLayouts,
  type DashboardResponsiveLayout,
  type MultiBreakpointLayout,
} from "./widgets/layout";

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

export function useDashboardLayout() {
  const layout = ref<MultiBreakpointLayout>(clone(defaultMultiLayout));
  const loading = ref(true);
  const error = ref("");
  const hasServerLayout = ref(false);

  async function load() {
    loading.value = true;
    error.value = "";
    try {
      const response = await dashboardApi.layout();
      const serverLayout = sanitizeDashboardLayouts(response.data.layouts);
      hasServerLayout.value = Boolean(response.data.layouts);
      layout.value = ensureCompleteLayout(serverLayout ?? {});
    } catch (err) {
      error.value = err instanceof Error ? err.message : "Dashboard 布局加载失败";
      layout.value = clone(defaultMultiLayout);
    } finally {
      loading.value = false;
    }
  }

  function replace(next: DashboardResponsiveLayout) {
    layout.value = ensureCompleteLayout(next);
  }

  return { layout, loading, error, hasServerLayout, load, replace };
}
