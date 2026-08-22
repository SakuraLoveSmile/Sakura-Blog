export type Breakpoint = "lg" | "md" | "sm" | "xs" | "xxs";

export const BREAKPOINTS: Record<"lg" | "md" | "sm" | "xs", number> = {
  lg: 1200,
  md: 996,
  sm: 768,
  xs: 480,
};

export const BREAKPOINT_ORDER: Breakpoint[] = ["lg", "md", "sm", "xs", "xxs"];
/** 用户可编辑/预览的三种设备布局；其余断点仅为 v10 数据兼容保留。 */
export const DESIGN_BREAKPOINTS = ["lg", "sm", "xxs"] as const;
export type DashboardLayoutBreakpoint = (typeof DESIGN_BREAKPOINTS)[number];
export const GRID_COLUMNS: Record<Breakpoint, number> = {
  lg: 12,
  md: 12,
  sm: 6,
  xs: 4,
  xxs: 4,
};
export const GRID_MAX_ROWS = 512;
export const GRID_MAX_WIDGET_ROWS = 64;
export const DASHBOARD_LAYOUT_VERSION = 10;
export const LEGACY_LAYOUT_STORAGE_KEY = "dashboard:layout:v9";

const LEGACY_V8_STORAGE_KEY = "dashboard:layout:v8";
const LEGACY_V7_STORAGE_KEY = "dashboard:layout:v7";
const LEGACY_V6_STORAGE_KEY = "dashboard:layout:v6";

export function getGridColumns(bp: Breakpoint): number {
  return GRID_COLUMNS[bp];
}

export function getBreakpoint(width: number): DashboardLayoutBreakpoint {
  if (width >= BREAKPOINTS.lg) return "lg";
  if (width >= BREAKPOINTS.sm) return "sm";
  return "xxs";
}

export interface DashboardWidget {
  i: string;
  id: string;
  x: number;
  y: number;
  w: number;
  h: number;
  minW?: number;
  minH?: number;
  maxW?: number;
  maxH?: number;
  config?: Record<string, unknown>;
  permissions?: string[];
}

export type GridWidget = DashboardWidget;
export type DashboardResponsiveLayout = Partial<Record<Breakpoint, DashboardWidget[]>>;
export type MultiBreakpointLayout = Record<Breakpoint, DashboardWidget[]>;

export interface GridSize {
  w: number;
  h: number;
}

export interface GridConstraints {
  minW: number;
  minH: number;
  maxW: number;
  maxH: number;
}

export type WidgetSizePreset = "small" | "medium" | "large";
export const WIDGET_SIZE_PRESET_ORDER: WidgetSizePreset[] = ["small", "medium", "large"];

interface WidgetSpec {
  defaults: Record<Breakpoint, GridSize>;
  minW: Record<Breakpoint, number>;
  minH: Record<Breakpoint, number>;
}

const statTypes = [
  "stat-posts",
  "stat-published",
  "stat-views",
  "stat-comments",
  "stat-attachments",
  "stat-users",
];

const widgetSpecs: Record<string, WidgetSpec> = Object.fromEntries(
  statTypes.map((type) => [type, {
    defaults: {
      lg: { w: 3, h: 19 }, md: { w: 6, h: 18 }, sm: { w: 3, h: 18 },
      xs: { w: 2, h: 18 }, xxs: { w: 2, h: 18 },
    },
    minW: { lg: 2, md: 2, sm: 2, xs: 2, xxs: 2 },
    minH: { lg: 10, md: 10, sm: 10, xs: 10, xxs: 10 },
  }]),
) as Record<string, WidgetSpec>;

function spec(type: string, defaults: Record<Breakpoint, GridSize>, minW: Record<Breakpoint, number>, minH: Record<Breakpoint, number>): void {
  widgetSpecs[type] = { defaults, minW, minH };
}

spec("trend-chart", {
  lg: { w: 8, h: 52 }, md: { w: 12, h: 44 }, sm: { w: 6, h: 44 },
  xs: { w: 4, h: 42 }, xxs: { w: 4, h: 42 },
}, { lg: 4, md: 4, sm: 3, xs: 2, xxs: 2 }, { lg: 24, md: 24, sm: 24, xs: 22, xxs: 22 });
spec("hourly-chart", {
  lg: { w: 4, h: 40 }, md: { w: 6, h: 37 }, sm: { w: 3, h: 37 },
  xs: { w: 4, h: 35 }, xxs: { w: 4, h: 35 },
}, { lg: 2, md: 2, sm: 2, xs: 2, xxs: 2 }, { lg: 20, md: 20, sm: 20, xs: 18, xxs: 18 });
spec("category-donut", {
  lg: { w: 4, h: 34 }, md: { w: 6, h: 32 }, sm: { w: 3, h: 32 },
  xs: { w: 4, h: 29 }, xxs: { w: 4, h: 29 },
}, { lg: 2, md: 2, sm: 2, xs: 2, xxs: 2 }, { lg: 18, md: 18, sm: 18, xs: 16, xxs: 16 });
spec("quick-actions", {
  lg: { w: 4, h: 35 }, md: { w: 6, h: 31 }, sm: { w: 3, h: 31 },
  xs: { w: 4, h: 20 }, xxs: { w: 4, h: 20 },
}, { lg: 2, md: 2, sm: 2, xs: 2, xxs: 2 }, { lg: 16, md: 16, sm: 16, xs: 14, xxs: 14 });
for (const type of ["recent-posts", "recent-comments"]) {
  spec(type, {
    lg: { w: 6, h: 36 }, md: { w: 12, h: 32 }, sm: { w: 6, h: 32 },
    xs: { w: 4, h: 27 }, xxs: { w: 4, h: 27 },
  }, { lg: 3, md: 3, sm: 3, xs: 2, xxs: 2 }, { lg: 18, md: 18, sm: 18, xs: 16, xxs: 16 });
}
for (const type of ["stellar-version", "system-version"]) {
  spec(type, {
    lg: { w: 4, h: 25 }, md: { w: 6, h: 23 }, sm: { w: 3, h: 23 },
    xs: { w: 4, h: 22 }, xxs: { w: 4, h: 22 },
  }, { lg: 2, md: 2, sm: 2, xs: 2, xxs: 2 }, { lg: 12, md: 12, sm: 12, xs: 12, xxs: 12 });
}

function fallbackSpec(): WidgetSpec {
  const defaults = Object.fromEntries(BREAKPOINT_ORDER.map((bp) => [
    bp,
    { w: GRID_COLUMNS[bp] === 4 ? 4 : GRID_COLUMNS[bp] === 6 ? 6 : 6, h: 12 },
  ])) as Record<Breakpoint, GridSize>;
  return {
    defaults,
    minW: Object.fromEntries(BREAKPOINT_ORDER.map((bp) => [bp, 1])) as Record<Breakpoint, number>,
    minH: Object.fromEntries(BREAKPOINT_ORDER.map((bp) => [bp, 6])) as Record<Breakpoint, number>,
  };
}

function getSpec(type: string): WidgetSpec {
  return widgetSpecs[type] ?? fallbackSpec();
}

export function getDefaultGridSize(type: string, bp: Breakpoint): GridSize {
  return { ...getSpec(type).defaults[bp] };
}

export function getGridConstraints(type: string, bp: Breakpoint): GridConstraints {
  const widgetSpec = getSpec(type);
  return {
    minW: Math.min(GRID_COLUMNS[bp], widgetSpec.minW[bp]),
    minH: Math.min(GRID_MAX_WIDGET_ROWS, widgetSpec.minH[bp]),
    maxW: GRID_COLUMNS[bp],
    maxH: GRID_MAX_WIDGET_ROWS,
  };
}

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T;
}

function createInstanceId(type: string): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return `${type}-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

function makeWidget(i: string, id: string, x: number, y: number, w: number, h: number, bp: Breakpoint): DashboardWidget {
  const constraints = getGridConstraints(id, bp);
  return {
    i,
    id,
    x,
    y,
    w,
    h,
    minW: constraints.minW,
    minH: constraints.minH,
    maxW: constraints.maxW,
    maxH: constraints.maxH,
    config: {},
  };
}

function defaultWidget(id: string, x: number, y: number, w: number, h: number): DashboardWidget {
  return makeWidget(id, id.replace(/-\d+$/, ""), x, y, w, h, "lg");
}

const defaultLayoutLg: DashboardWidget[] = [
  defaultWidget("stat-posts-1", 0, 0, 3, 19),
  defaultWidget("stat-published-1", 3, 0, 3, 19),
  defaultWidget("stat-views-1", 6, 0, 3, 19),
  defaultWidget("stat-comments-1", 9, 0, 3, 19),
  defaultWidget("trend-chart-1", 0, 19, 8, 52),
  defaultWidget("category-donut-1", 8, 19, 4, 34),
  defaultWidget("quick-actions-1", 8, 53, 4, 35),
  defaultWidget("recent-posts-1", 0, 88, 6, 36),
  defaultWidget("recent-comments-1", 6, 88, 6, 36),
  defaultWidget("stellar-version-1", 0, 124, 4, 25),
];

const defaultLayoutMd: DashboardWidget[] = [
  defaultWidget("stat-posts-1", 0, 0, 6, 18),
  defaultWidget("stat-published-1", 6, 0, 6, 18),
  defaultWidget("stat-views-1", 0, 18, 6, 18),
  defaultWidget("stat-comments-1", 6, 18, 6, 18),
  defaultWidget("trend-chart-1", 0, 36, 12, 44),
  defaultWidget("category-donut-1", 0, 80, 6, 32),
  defaultWidget("quick-actions-1", 6, 80, 6, 31),
  defaultWidget("recent-posts-1", 0, 112, 12, 32),
  defaultWidget("recent-comments-1", 0, 144, 12, 32),
  defaultWidget("stellar-version-1", 0, 176, 6, 23),
];

const defaultLayoutSm: DashboardWidget[] = [
  defaultWidget("stat-posts-1", 0, 0, 3, 18),
  defaultWidget("stat-published-1", 3, 0, 3, 18),
  defaultWidget("stat-views-1", 0, 18, 3, 18),
  defaultWidget("stat-comments-1", 3, 18, 3, 18),
  defaultWidget("trend-chart-1", 0, 36, 6, 44),
  defaultWidget("category-donut-1", 0, 80, 3, 32),
  defaultWidget("quick-actions-1", 3, 80, 3, 31),
  defaultWidget("recent-posts-1", 0, 112, 6, 32),
  defaultWidget("recent-comments-1", 0, 144, 6, 32),
  defaultWidget("stellar-version-1", 0, 176, 3, 23),
];

const defaultLayoutXs: DashboardWidget[] = [
  defaultWidget("stat-posts-1", 0, 0, 2, 18),
  defaultWidget("stat-published-1", 2, 0, 2, 18),
  defaultWidget("stat-views-1", 0, 18, 2, 18),
  defaultWidget("stat-comments-1", 2, 18, 2, 18),
  defaultWidget("trend-chart-1", 0, 36, 4, 42),
  defaultWidget("category-donut-1", 0, 78, 4, 29),
  defaultWidget("quick-actions-1", 0, 107, 4, 20),
  defaultWidget("recent-posts-1", 0, 127, 4, 27),
  defaultWidget("recent-comments-1", 0, 154, 4, 27),
  defaultWidget("stellar-version-1", 0, 181, 4, 22),
];

export const defaultMultiLayout: MultiBreakpointLayout = {
  lg: clone(defaultLayoutLg),
  md: clone(defaultLayoutMd),
  sm: clone(defaultLayoutSm),
  xs: clone(defaultLayoutXs),
  xxs: clone(defaultLayoutXs),
};

export function getDefaultGridLayout(bp: Breakpoint): DashboardWidget[] {
  return clone(defaultMultiLayout[bp]);
}

function clampInt(value: unknown, min: number, max: number, fallback: number): number {
  const n = typeof value === "number" && Number.isFinite(value) ? Math.round(value) : fallback;
  return Math.max(min, Math.min(max, n));
}

function overlaps(a: Pick<DashboardWidget, "x" | "y" | "w" | "h">, b: Pick<DashboardWidget, "x" | "y" | "w" | "h">): boolean {
  return a.x < b.x + b.w && a.x + a.w > b.x && a.y < b.y + b.h && a.y + a.h > b.y;
}

export function hasGridOverlaps(widgets: DashboardWidget[]): boolean {
  for (let i = 0; i < widgets.length; i += 1) {
    for (let j = i + 1; j < widgets.length; j += 1) {
      if (overlaps(widgets[i], widgets[j])) return true;
    }
  }
  return false;
}

export function findFreeGridPosition(
  widgets: DashboardWidget[],
  size: GridSize,
  bp: Breakpoint = "lg",
): Pick<DashboardWidget, "x" | "y"> {
  const columns = GRID_COLUMNS[bp];
  const w = clampInt(size.w, 1, columns, size.w);
  const h = clampInt(size.h, 1, GRID_MAX_WIDGET_ROWS, size.h);
  for (let y = 0; y <= GRID_MAX_ROWS - h; y += 1) {
    for (let x = 0; x <= columns - w; x += 1) {
      const candidate = { x, y, w, h };
      if (!widgets.some((item) => overlaps(candidate, item))) return { x, y };
    }
  }
  const bottom = widgets.reduce((max, item) => Math.max(max, item.y + item.h), 0);
  return { x: 0, y: bottom };
}

export type WidgetPresetMatrix = Record<WidgetSizePreset, Record<Breakpoint, GridSize>>;

export const WIDGET_PRESET_SPECS: Record<string, WidgetPresetMatrix> = {
  ...Object.fromEntries(
    statTypes.map((type) => [
      type,
      {
        small: {
          lg: { w: 2, h: 14 }, md: { w: 3, h: 14 }, sm: { w: 2, h: 14 },
          xs: { w: 2, h: 14 }, xxs: { w: 2, h: 14 },
        },
        medium: {
          lg: { w: 3, h: 19 }, md: { w: 6, h: 18 }, sm: { w: 3, h: 18 },
          xs: { w: 2, h: 18 }, xxs: { w: 2, h: 18 },
        },
        large: {
          lg: { w: 6, h: 19 }, md: { w: 12, h: 18 }, sm: { w: 6, h: 18 },
          xs: { w: 4, h: 18 }, xxs: { w: 4, h: 18 },
        },
      },
    ]),
  ),
  "trend-chart": {
    small: {
      lg: { w: 6, h: 36 }, md: { w: 6, h: 36 }, sm: { w: 6, h: 36 },
      xs: { w: 4, h: 32 }, xxs: { w: 4, h: 32 },
    },
    medium: {
      lg: { w: 8, h: 52 }, md: { w: 12, h: 44 }, sm: { w: 6, h: 44 },
      xs: { w: 4, h: 42 }, xxs: { w: 4, h: 42 },
    },
    large: {
      lg: { w: 12, h: 52 }, md: { w: 12, h: 52 }, sm: { w: 6, h: 52 },
      xs: { w: 4, h: 50 }, xxs: { w: 4, h: 50 },
    },
  },
  "hourly-chart": {
    small: {
      lg: { w: 4, h: 28 }, md: { w: 6, h: 28 }, sm: { w: 3, h: 28 },
      xs: { w: 4, h: 26 }, xxs: { w: 4, h: 26 },
    },
    medium: {
      lg: { w: 4, h: 40 }, md: { w: 6, h: 37 }, sm: { w: 3, h: 37 },
      xs: { w: 4, h: 35 }, xxs: { w: 4, h: 35 },
    },
    large: {
      lg: { w: 6, h: 40 }, md: { w: 12, h: 37 }, sm: { w: 6, h: 37 },
      xs: { w: 4, h: 42 }, xxs: { w: 4, h: 42 },
    },
  },
  "category-donut": {
    small: {
      lg: { w: 3, h: 28 }, md: { w: 4, h: 28 }, sm: { w: 3, h: 28 },
      xs: { w: 4, h: 22 }, xxs: { w: 4, h: 22 },
    },
    medium: {
      lg: { w: 4, h: 34 }, md: { w: 6, h: 32 }, sm: { w: 3, h: 32 },
      xs: { w: 4, h: 29 }, xxs: { w: 4, h: 29 },
    },
    large: {
      lg: { w: 6, h: 34 }, md: { w: 12, h: 32 }, sm: { w: 6, h: 32 },
      xs: { w: 4, h: 36 }, xxs: { w: 4, h: 36 },
    },
  },
  "recent-posts": {
    small: {
      lg: { w: 4, h: 28 }, md: { w: 6, h: 26 }, sm: { w: 6, h: 26 },
      xs: { w: 4, h: 22 }, xxs: { w: 4, h: 22 },
    },
    medium: {
      lg: { w: 6, h: 36 }, md: { w: 12, h: 32 }, sm: { w: 6, h: 32 },
      xs: { w: 4, h: 27 }, xxs: { w: 4, h: 27 },
    },
    large: {
      lg: { w: 12, h: 36 }, md: { w: 12, h: 44 }, sm: { w: 6, h: 44 },
      xs: { w: 4, h: 36 }, xxs: { w: 4, h: 36 },
    },
  },
  "recent-comments": {
    small: {
      lg: { w: 4, h: 28 }, md: { w: 6, h: 26 }, sm: { w: 6, h: 26 },
      xs: { w: 4, h: 22 }, xxs: { w: 4, h: 22 },
    },
    medium: {
      lg: { w: 6, h: 36 }, md: { w: 12, h: 32 }, sm: { w: 6, h: 32 },
      xs: { w: 4, h: 27 }, xxs: { w: 4, h: 27 },
    },
    large: {
      lg: { w: 12, h: 36 }, md: { w: 12, h: 44 }, sm: { w: 6, h: 44 },
      xs: { w: 4, h: 36 }, xxs: { w: 4, h: 36 },
    },
  },
  "quick-actions": {
    small: {
      lg: { w: 3, h: 24 }, md: { w: 4, h: 22 }, sm: { w: 3, h: 22 },
      xs: { w: 4, h: 16 }, xxs: { w: 4, h: 16 },
    },
    medium: {
      lg: { w: 4, h: 35 }, md: { w: 6, h: 31 }, sm: { w: 3, h: 31 },
      xs: { w: 4, h: 20 }, xxs: { w: 4, h: 20 },
    },
    large: {
      lg: { w: 6, h: 35 }, md: { w: 12, h: 31 }, sm: { w: 6, h: 31 },
      xs: { w: 4, h: 32 }, xxs: { w: 4, h: 32 },
    },
  },
  "stellar-version": {
    small: {
      lg: { w: 3, h: 18 }, md: { w: 4, h: 18 }, sm: { w: 3, h: 18 },
      xs: { w: 4, h: 16 }, xxs: { w: 4, h: 16 },
    },
    medium: {
      lg: { w: 4, h: 25 }, md: { w: 6, h: 23 }, sm: { w: 3, h: 23 },
      xs: { w: 4, h: 22 }, xxs: { w: 4, h: 22 },
    },
    large: {
      lg: { w: 6, h: 25 }, md: { w: 12, h: 23 }, sm: { w: 6, h: 23 },
      xs: { w: 4, h: 28 }, xxs: { w: 4, h: 28 },
    },
  },
  "system-version": {
    small: {
      lg: { w: 3, h: 18 }, md: { w: 4, h: 18 }, sm: { w: 3, h: 18 },
      xs: { w: 4, h: 16 }, xxs: { w: 4, h: 16 },
    },
    medium: {
      lg: { w: 4, h: 25 }, md: { w: 6, h: 23 }, sm: { w: 3, h: 23 },
      xs: { w: 4, h: 22 }, xxs: { w: 4, h: 22 },
    },
    large: {
      lg: { w: 6, h: 25 }, md: { w: 12, h: 23 }, sm: { w: 6, h: 23 },
      xs: { w: 4, h: 28 }, xxs: { w: 4, h: 28 },
    },
  },
};

/** 未知组件的显式兜底规格；已注册组件必须使用 WIDGET_PRESET_SPECS。 */
const FALLBACK_PRESET_SPECS: WidgetPresetMatrix = {
  small: {
    lg: { w: 4, h: 14 }, md: { w: 4, h: 14 }, sm: { w: 2, h: 14 },
    xs: { w: 2, h: 14 }, xxs: { w: 2, h: 14 },
  },
  medium: {
    lg: { w: 6, h: 20 }, md: { w: 6, h: 20 }, sm: { w: 4, h: 20 },
    xs: { w: 4, h: 20 }, xxs: { w: 4, h: 20 },
  },
  large: {
    lg: { w: 12, h: 32 }, md: { w: 12, h: 32 }, sm: { w: 6, h: 32 },
    xs: { w: 4, h: 32 }, xxs: { w: 4, h: 32 },
  },
};

/** 根据组件预设规格矩阵生成精确离散大小，避免破环网格整除与对齐。 */
export function getGridPresetSize(type: string, bp: Breakpoint, preset: WidgetSizePreset): GridSize {
  const desired = (WIDGET_PRESET_SPECS[type] ?? FALLBACK_PRESET_SPECS)[preset][bp];
  const constraints = getGridConstraints(type, bp);
  return {
    w: clampInt(desired.w, constraints.minW, constraints.maxW, desired.w),
    h: clampInt(desired.h, constraints.minH, constraints.maxH, desired.h),
  };
}

/** 将历史布局的任意尺寸映射到最近的三个预设之一。 */
export function getWidgetSizePreset(widget: Pick<DashboardWidget, "w" | "h">, bp: Breakpoint, type: string): WidgetSizePreset {
  let closest = WIDGET_SIZE_PRESET_ORDER[0];
  let closestWidthDistance = Number.POSITIVE_INFINITY;
  let closestHeightDistance = Number.POSITIVE_INFINITY;
  const seenWidths = new Set<number>();
  for (const preset of WIDGET_SIZE_PRESET_ORDER) {
    const size = getGridPresetSize(type, bp, preset);
    // 同一断点下相同列跨度只保留第一个语义代表，避免弹窗和卡片状态重复。
    if (seenWidths.has(size.w)) continue;
    seenWidths.add(size.w);
    const widthDistance = Math.abs(widget.w - size.w);
    const heightDistance = Math.abs(widget.h - size.h);
    if (
      widthDistance < closestWidthDistance
      || (widthDistance === closestWidthDistance && heightDistance < closestHeightDistance)
    ) {
      closest = preset;
      closestWidthDistance = widthDistance;
      closestHeightDistance = heightDistance;
    }
  }
  return closest;
}

/** 应用离散尺寸；放不下时把组件移到当前断点的第一个空位。 */
export function applyGridWidgetSize(
  widget: DashboardWidget,
  preset: WidgetSizePreset,
  bp: Breakpoint,
  widgets: DashboardWidget[],
): DashboardWidget {
  const size = getGridPresetSize(widget.id, bp, preset);
  const others = widgets.filter((item) => item.i !== widget.i);
  const x = Math.max(0, Math.min(GRID_COLUMNS[bp] - size.w, widget.x));
  const y = Math.max(0, Math.min(GRID_MAX_ROWS - size.h, widget.y));
  const position = others.some((item) => overlaps({ x, y, ...size }, item))
    ? findFreeGridPosition(others, size, bp)
    : { x, y };
  const constraints = getGridConstraints(widget.id, bp);
  return {
    ...widget,
    ...position,
    ...size,
    minW: constraints.minW,
    minH: constraints.minH,
    maxW: constraints.maxW,
    maxH: constraints.maxH,
  };
}

export function createDefaultGridWidget(
  type: string,
  bp: Breakpoint,
  widgets: DashboardWidget[] = [],
): DashboardWidget {
  const size = getDefaultGridSize(type, bp);
  const position = findFreeGridPosition(widgets, size, bp);
  return makeWidget(createInstanceId(type), type, position.x, position.y, size.w, size.h, bp);
}

function normalizeWidget(raw: unknown, bp: Breakpoint, index: number, used: Set<string>): DashboardWidget | null {
  if (!raw || typeof raw !== "object") return null;
  const source = raw as Partial<DashboardWidget> & { type?: unknown };
  const id = typeof source.id === "string" && source.id ? source.id : typeof source.type === "string" ? source.type : "";
  if (!id) return null;
  let instance = typeof source.i === "string" && source.i ? source.i : `${id}-${index + 1}`;
  while (used.has(instance)) instance = `${instance}-copy`;
  used.add(instance);
  const defaults = getDefaultGridSize(id, bp);
  const constraints = getGridConstraints(id, bp);
  const w = clampInt(source.w, constraints.minW, constraints.maxW, defaults.w);
  const h = clampInt(source.h, constraints.minH, constraints.maxH, defaults.h);
  const widget: DashboardWidget = {
    i: instance,
    id,
    x: clampInt(source.x, 0, GRID_COLUMNS[bp] - w, 0),
    y: clampInt(source.y, 0, GRID_MAX_ROWS - h, 0),
    w,
    h,
    minW: clampInt(source.minW, 1, constraints.maxW, constraints.minW),
    minH: clampInt(source.minH, 1, constraints.maxH, constraints.minH),
    maxW: clampInt(source.maxW, constraints.minW, constraints.maxW, constraints.maxW),
    maxH: clampInt(source.maxH, constraints.minH, constraints.maxH, constraints.maxH),
    config: source.config && typeof source.config === "object" ? clone(source.config) : {},
  };
  if (Array.isArray(source.permissions)) widget.permissions = source.permissions.filter((item): item is string => typeof item === "string");
  return widget;
}

function normalizeBreakpointWidgets(items: unknown, bp: Breakpoint): DashboardWidget[] {
  if (!Array.isArray(items)) return [];
  const result: DashboardWidget[] = [];
  const used = new Set<string>();
  for (const [index, raw] of items.entries()) {
    const item = normalizeWidget(raw, bp, index, used);
    if (!item) continue;
    // 历史 v9/v10 布局可能保存过任意宽高；加载时统一落到最近的离散档位。
    const preset = getWidgetSizePreset(item, bp, item.id);
    result.push(applyGridWidgetSize(item, preset, bp, result));
  }
  return result;
}

export function sanitizeResponsiveLayout(input: unknown): DashboardResponsiveLayout | null {
  if (!input || typeof input !== "object") return null;
  const source = input as Record<string, unknown>;
  const output: DashboardResponsiveLayout = {};
  for (const bp of BREAKPOINT_ORDER) {
    if (!(bp in source)) continue;
    const value = source[bp];
    if (!Array.isArray(value)) return null;
    output[bp] = normalizeBreakpointWidgets(value, bp);
  }
  return output;
}

/** Backwards-compatible v6 → v7 migration retained for existing local data. */
export function migrateV6toV7(v6: any): any | null {
  try {
    if (!v6 || !v6.l || !v6.m || !v6.s) return null;
    const layout = clone(v6);
    const recents = layout.m.recents;
    if (Array.isArray(recents)) {
      for (const item of recents) {
        const size = item?.size;
        const isOldDefault = size && size.l === 6 && size.m === 6 && size.s === 12;
        if ((item.type === "recent-posts" || item.type === "recent-comments") && isOldDefault) {
          item.size = { l: 6, m: 12, s: 12 };
        }
      }
    }
    return layout;
  } catch {
    return null;
  }
}

interface LegacyResponsiveSize { l?: number; m?: number; s?: number }
interface LegacyWidget { type?: unknown; size?: LegacyResponsiveSize; x?: number; y?: number; w?: number; h?: number }
interface LegacySectionLayout { stats?: LegacyWidget[]; chartWide?: LegacyWidget | null; chartNarrow?: LegacyWidget[]; recents?: LegacyWidget[]; others?: LegacyWidget[] }

function legacyItems(section: LegacySectionLayout): LegacyWidget[] {
  return [
    ...(Array.isArray(section.stats) ? section.stats : []),
    ...(section.chartWide ? [section.chartWide] : []),
    ...(Array.isArray(section.chartNarrow) ? section.chartNarrow : []),
    ...(Array.isArray(section.recents) ? section.recents : []),
    ...(Array.isArray(section.others) ? section.others : []),
  ];
}

function packLegacyItems(items: LegacyWidget[], bp: "l" | "m" | "s"): DashboardWidget[] {
  const target: Breakpoint = bp === "l" ? "lg" : "md";
  const result: DashboardWidget[] = [];
  let cursorX = 0;
  let cursorY = 0;
  let rowHeight = 0;
  for (const [index, legacy] of items.entries()) {
    if (typeof legacy.type !== "string" || !legacy.type) continue;
    const defaults = getDefaultGridSize(legacy.type, target);
    const width = clampInt(legacy.size?.[bp], 1, GRID_COLUMNS[target], defaults.w);
    if (cursorX + width > GRID_COLUMNS[target]) {
      cursorX = 0;
      cursorY += rowHeight;
      rowHeight = 0;
    }
    const item = normalizeWidget({
      i: `${legacy.type}-${index + 1}`,
      id: legacy.type,
      x: cursorX,
      y: cursorY,
      w: width,
      h: defaults.h,
    }, target, index, new Set(result.map((widget) => widget.i)));
    if (item) {
      result.push(item);
      cursorX = item.x + item.w;
      rowHeight = Math.max(rowHeight, item.h);
    }
  }
  return result;
}

/** v8/v7 旧五区域布局 → v9 统一网格布局。 */
export function migrateV8toV9(input: unknown): Record<"l" | "m" | "s", { widgets: DashboardWidget[] }> | null {
  if (!input || typeof input !== "object") return null;
  const source = input as Record<"l" | "m" | "s", LegacySectionLayout | undefined>;
  const output = {} as Record<"l" | "m" | "s", { widgets: DashboardWidget[] }>;
  for (const bp of ["l", "m", "s"] as const) {
    const section = source[bp];
    if (!section || typeof section !== "object") return null;
    output[bp] = { widgets: packLegacyItems(legacyItems(section), bp) };
  }
  return output;
}

function scaleWidget(
  widget: DashboardWidget,
  from: number,
  to: number,
  sourceBp: Breakpoint,
  bp: Breakpoint,
  index: number,
): DashboardWidget {
  const scaledX = Math.round((widget.x * to) / from);
  const preset = getWidgetSizePreset(widget, sourceBp, widget.id);
  const size = getGridPresetSize(widget.id, bp, preset);
  const constraints = getGridConstraints(widget.id, bp);
  return {
    ...widget,
    i: `${widget.i}-${bp}-${index + 1}`,
    x: Math.max(0, Math.min(to - size.w, scaledX)),
    y: widget.y,
    w: size.w,
    h: size.h,
    minW: constraints.minW,
    minH: constraints.minH,
    maxW: constraints.maxW,
    maxH: constraints.maxH,
    config: widget.config ? clone(widget.config) : {},
  };
}

function convertWidgets(items: DashboardWidget[], from: number, sourceBp: Breakpoint, bp: Breakpoint): DashboardWidget[] {
  const result: DashboardWidget[] = [];
  for (const [index, item] of items.entries()) {
    const scaled = scaleWidget(item, from, GRID_COLUMNS[bp], sourceBp, bp, index);
    const position = findFreeGridPosition(result, scaled, bp);
    result.push({ ...scaled, ...position });
  }
  return result;
}

/** Convert the current v9 local layout into Halo-style responsive instances. */
export function migrateV9toV10(input: unknown): DashboardResponsiveLayout | null {
  const source = input as Record<string, { widgets?: unknown }> | null;
  if (!source || typeof source !== "object") return null;
  const v9: Record<"l" | "m" | "s", DashboardWidget[]> = { l: [], m: [], s: [] };
  for (const oldBp of ["l", "m", "s"] as const) {
    const widgets = source[oldBp]?.widgets;
    if (!Array.isArray(widgets)) return null;
    v9[oldBp] = normalizeBreakpointWidgets(widgets, oldBp === "l" ? "lg" : "md");
  }
  return {
    lg: convertWidgets(v9.l, 12, "lg", "lg"),
    md: convertWidgets(v9.m, 12, "md", "md"),
    sm: convertWidgets(v9.m, 12, "md", "sm"),
    xs: convertWidgets(v9.s, 12, "md", "xs"),
    xxs: convertWidgets(v9.s, 12, "md", "xxs"),
  };
}

/** Read old browser data without writing it back; the Designer offers explicit import. */
export function loadLegacyLayoutFromStorage(): DashboardResponsiveLayout | null {
  try {
    const current = localStorage.getItem(LEGACY_LAYOUT_STORAGE_KEY);
    if (current) {
      const migrated = migrateV9toV10(JSON.parse(current));
      if (migrated) return migrated;
    }
    const legacyV8 = localStorage.getItem(LEGACY_V8_STORAGE_KEY);
    if (legacyV8) {
      const migrated = migrateV8toV9(JSON.parse(legacyV8));
      const v10 = migrated && migrateV9toV10(migrated);
      if (v10) return v10;
    }
    const legacyV7 = localStorage.getItem(LEGACY_V7_STORAGE_KEY);
    if (legacyV7) {
      const migrated = migrateV8toV9(JSON.parse(legacyV7));
      const v10 = migrated && migrateV9toV10(migrated);
      if (v10) return v10;
    }
    const legacyV6 = localStorage.getItem(LEGACY_V6_STORAGE_KEY);
    if (legacyV6) {
      const migratedV7 = migrateV6toV7(JSON.parse(legacyV6));
      const migrated = migratedV7 && migrateV8toV9(migratedV7);
      const v10 = migrated && migrateV9toV10(migrated);
      if (v10) return v10;
    }
  } catch {
    // 浏览器中的坏数据只影响导入提示，不应阻断 Dashboard。
  }
  return null;
}

export function sanitizeDashboardLayouts(input: unknown): DashboardResponsiveLayout | null {
  if (!input || typeof input !== "object") return null;
  const source = input as Record<string, unknown>;
  const result: DashboardResponsiveLayout = {};
  for (const bp of BREAKPOINT_ORDER) {
    if (!(bp in source)) continue;
    const items = source[bp];
    if (!Array.isArray(items)) return null;
    result[bp] = normalizeBreakpointWidgets(items, bp);
  }
  return result;
}

export function nodesToGridWidgets(
  nodes: Array<Partial<DashboardWidget> & { id?: unknown; widgetId?: unknown }>,
  bp: Breakpoint = "lg",
): DashboardWidget[] {
  return nodes
    .map((node, index) => {
      const instance = typeof node.i === "string" ? node.i : typeof node.id === "string" ? node.id : `widget-${index + 1}`;
      const definition = typeof node.widgetId === "string" ? node.widgetId : typeof node.id === "string" ? node.id : instance;
      return normalizeWidget({
        ...node,
        i: instance,
        id: definition,
        x: Math.max(0, Math.round(node.x ?? 0)),
        y: Math.max(0, Math.round(node.y ?? 0)),
        w: Math.max(1, Math.round(node.w ?? 1)),
        h: Math.max(1, Math.round(node.h ?? 1)),
      }, bp, index, new Set());
    })
    .filter((node): node is DashboardWidget => Boolean(node))
    .sort((a, b) => a.y - b.y || a.x - b.x || a.i.localeCompare(b.i));
}

/**
 * Merge GridStack geometry with instance metadata that is not represented in
 * DOM attributes. This keeps future widget config and permission data intact
 * after a drag.
 */
export function mergeGridWidgetMetadata(
  items: DashboardWidget[],
  previous: ReadonlyMap<string, DashboardWidget>,
): DashboardWidget[] {
  return items.map((item) => {
    const prior = previous.get(item.i);
    if (!prior) return item;
    return {
      ...item,
      ...(prior.config !== undefined ? { config: clone(prior.config) } : {}),
      ...(prior.permissions !== undefined ? { permissions: [...prior.permissions] } : {}),
    };
  });
}

export function copyLayoutToBreakpoint(
  layout: DashboardResponsiveLayout,
  source: Breakpoint,
  target: Breakpoint,
): DashboardResponsiveLayout {
  const sourceWidgets = layout[source] ?? [];
  const copied: DashboardWidget[] = [];
  for (const [index, sourceWidget] of sourceWidgets.entries()) {
    const scaled = scaleWidget(
      sourceWidget,
      GRID_COLUMNS[source],
      GRID_COLUMNS[target],
      source,
      target,
      index,
    );
    const position = findFreeGridPosition(copied, scaled, target);
    copied.push({ ...scaled, ...position, i: `${sourceWidget.i}-copy-${target}-${index + 1}` });
  }
  return { ...clone(layout), [target]: copied };
}

export function ensureCompleteLayout(layout: DashboardResponsiveLayout): MultiBreakpointLayout {
  const result = {} as MultiBreakpointLayout;
  for (const bp of BREAKPOINT_ORDER) {
    // 统一入口保证只向只读页、Designer 和服务端输出三档尺寸。
    result[bp] = normalizeBreakpointWidgets(layout[bp] ?? defaultMultiLayout[bp], bp);
  }
  return result;
}
