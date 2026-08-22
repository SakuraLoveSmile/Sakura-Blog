import {
  getGridPresetSize,
  WIDGET_SIZE_PRESET_ORDER,
  type Breakpoint,
  type GridSize,
  type WidgetSizePreset,
} from "./layout";

export interface WidgetPresetOption {
  preset: WidgetSizePreset;
  size: GridSize;
}

/**
 * 返回当前断点下实际列跨度不重复的快捷预设。
 *
 * 三档名称是语义层级，不保证每个断点都有不同列数；同一列跨度只保留
 * 顺序靠前的代表项，避免两个按钮同时表达同一个可见网格宽度。
 */
export function uniquePresetOptions(type: string, breakpoint: Breakpoint): WidgetSizePreset[] {
  const seenWidths = new Set<number>();
  return WIDGET_SIZE_PRESET_ORDER.filter((preset) => {
    const span = getGridPresetSize(type, breakpoint, preset).w;
    if (seenWidths.has(span)) return false;
    seenWidths.add(span);
    return true;
  });
}

export function getWidgetPresetOptions(type: string, breakpoint: Breakpoint): WidgetPresetOption[] {
  return uniquePresetOptions(type, breakpoint).map((preset) => ({
    preset,
    size: getGridPresetSize(type, breakpoint, preset),
  }));
}
