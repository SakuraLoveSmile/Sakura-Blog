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
 * 返回当前断点下可选的预设。
 *
 * 档位足迹统一后三档必然互不相同；保留去重是为了兜住历史上按组件定制尺寸
 * 的旧矩阵，避免识别出的 preset 在弹窗里找不到对应项。
 */
export function uniquePresetOptions(type: string, breakpoint: Breakpoint): WidgetSizePreset[] {
  const seen = new Set<string>();
  return WIDGET_SIZE_PRESET_ORDER.filter((preset) => {
    const size = getGridPresetSize(type, breakpoint, preset);
    const key = `${size.w}x${size.h}`;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

export function getWidgetPresetOptions(type: string, breakpoint: Breakpoint): WidgetPresetOption[] {
  return uniquePresetOptions(type, breakpoint).map((preset) => ({
    preset,
    size: getGridPresetSize(type, breakpoint, preset),
  }));
}
