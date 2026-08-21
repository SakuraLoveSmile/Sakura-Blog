import type { ResponsiveSize, WidgetSize } from "./registry";

const PRESET_ORDER: WidgetSize[] = ["small", "medium", "large"];

/**
 * 返回当前断点下实际跨度不重复的快捷预设。
 *
 * 响应式预设允许两个语义尺寸在某个断点合并为同一跨度（例如 m 断点
 * 的 small/medium 都是 6 列）。这种情况下只保留顺序靠前的代表项，
 * 避免两个按钮同时显示为选中状态，造成用户以为存在两个不同尺寸。
 */
export function uniquePresetOptions(
  presets: Record<WidgetSize, ResponsiveSize>,
  breakpoint: keyof ResponsiveSize,
): WidgetSize[] {
  const seenSpans = new Set<number>();
  return PRESET_ORDER.filter((preset) => {
    const span = presets[preset][breakpoint];
    if (seenSpans.has(span)) return false;
    seenSpans.add(span);
    return true;
  });
}
