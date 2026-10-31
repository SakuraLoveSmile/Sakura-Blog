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
 * 返回当前断点下实际宽高不重复的快捷预设。
 *
 * 识别与去重必须共用同一套键：只按列宽去重会丢掉同宽不同高的档位，
 * 导致当前识别出的 preset 不在弹窗里。只有宽高都相同的档位才视为重复。
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
