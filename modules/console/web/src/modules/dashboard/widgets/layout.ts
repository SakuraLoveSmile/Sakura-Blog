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

const statTypes = [
  "stat-posts",
  "stat-published",
  "stat-views",
  "stat-comments",
  "stat-attachments",
  "stat-users",
];

/** 已注册组件的类型全集；档位不再按组件定制，这里只用于默认档位与校验。 */
export const WIDGET_TYPES: string[] = [
  ...statTypes,
  "trend-chart",
  "hourly-chart",
  "category-donut",
  "recent-posts",
  "recent-comments",
  "quick-actions",
  "top-posts",
  "system-health",
  "stellar-version",
  "system-version",
];

/**
 * 档位足迹：同一断点下的同一档位只有一个尺寸，所有组件共用。
 *
 * 高度阶梯按 cellHeight=8 取值，覆盖图表、列表、版本卡与统计卡的共同可用区间；
 * 组件只决定内容密度，不再决定盒子大小。
 */
export const WIDGET_PRESET_FOOTPRINTS: Record<Breakpoint, Record<WidgetSizePreset, GridSize>> = {
  lg: {
    small: { w: 3, h: 16 },
    medium: { w: 6, h: 28 },
    large: { w: 12, h: 40 },
  },
  md: {
    small: { w: 3, h: 16 },
    medium: { w: 6, h: 28 },
    large: { w: 12, h: 40 },
  },
  sm: {
    small: { w: 3, h: 16 },
    medium: { w: 6, h: 28 },
    large: { w: 6, h: 40 },
  },
  xs: {
    small: { w: 2, h: 16 },
    medium: { w: 4, h: 26 },
    large: { w: 4, h: 36 },
  },
  xxs: {
    small: { w: 2, h: 16 },
    medium: { w: 4, h: 26 },
    large: { w: 4, h: 36 },
  },
};

/** 新组件默认落到的档位：统计卡是单行指标，其余按标准档插入。 */
export function getDefaultPreset(type: string): WidgetSizePreset {
  return statTypes.includes(type) ? "small" : "medium";
}

export function getDefaultGridSize(type: string, bp: Breakpoint): GridSize {
  return { ...WIDGET_PRESET_FOOTPRINTS[bp][getDefaultPreset(type)] };
}

/**
 * 约束只用于兜住历史数据与极端尺寸，不能把任何档位钳制到别的值。
 * 因此下界取 1，上界是断点列数与单卡最大行高。
 */
export function getGridConstraints(_type: string, bp: Breakpoint): GridConstraints {
  return {
    minW: 1,
    minH: 1,
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
  defaultWidget("stat-posts-1", 0, 0, 3, 16),
  defaultWidget("stat-published-1", 3, 0, 3, 16),
  defaultWidget("stat-views-1", 6, 0, 3, 16),
  defaultWidget("stat-comments-1", 9, 0, 3, 16),
  defaultWidget("trend-chart-1", 0, 16, 12, 40),
  defaultWidget("recent-posts-1", 0, 56, 6, 28),
  defaultWidget("recent-comments-1", 6, 56, 6, 28),
  defaultWidget("category-donut-1", 0, 84, 6, 28),
  defaultWidget("stellar-version-1", 6, 84, 6, 28),
];

const defaultLayoutMd: DashboardWidget[] = [
  defaultWidget("stat-posts-1", 0, 0, 3, 16),
  defaultWidget("stat-published-1", 3, 0, 3, 16),
  defaultWidget("stat-views-1", 6, 0, 3, 16),
  defaultWidget("stat-comments-1", 9, 0, 3, 16),
  defaultWidget("trend-chart-1", 0, 16, 12, 40),
  defaultWidget("recent-posts-1", 0, 56, 6, 28),
  defaultWidget("recent-comments-1", 6, 56, 6, 28),
  defaultWidget("category-donut-1", 0, 84, 6, 28),
  defaultWidget("stellar-version-1", 6, 84, 6, 28),
];

const defaultLayoutSm: DashboardWidget[] = [
  defaultWidget("stat-posts-1", 0, 0, 3, 16),
  defaultWidget("stat-published-1", 3, 0, 3, 16),
  defaultWidget("stat-views-1", 0, 16, 3, 16),
  defaultWidget("stat-comments-1", 3, 16, 3, 16),
  defaultWidget("trend-chart-1", 0, 32, 6, 40),
  defaultWidget("recent-posts-1", 0, 72, 6, 28),
  defaultWidget("recent-comments-1", 0, 100, 6, 28),
  defaultWidget("category-donut-1", 0, 128, 6, 28),
  defaultWidget("stellar-version-1", 0, 156, 6, 28),
];

const defaultLayoutXs: DashboardWidget[] = [
  defaultWidget("stat-posts-1", 0, 0, 2, 16),
  defaultWidget("stat-published-1", 2, 0, 2, 16),
  defaultWidget("stat-views-1", 0, 16, 2, 16),
  defaultWidget("stat-comments-1", 2, 16, 2, 16),
  defaultWidget("trend-chart-1", 0, 32, 4, 36),
  defaultWidget("recent-posts-1", 0, 68, 4, 26),
  defaultWidget("recent-comments-1", 0, 94, 4, 26),
  defaultWidget("category-donut-1", 0, 120, 4, 26),
  defaultWidget("stellar-version-1", 0, 146, 4, 26),
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

/** 根据档位足迹生成精确离散大小；同一断点同一档位对所有组件一致。 */
export function getGridPresetSize(type: string, bp: Breakpoint, preset: WidgetSizePreset): GridSize {
  const footprint = WIDGET_PRESET_FOOTPRINTS[bp][preset];
  const constraints = getGridConstraints(type, bp);
  return {
    w: clampInt(footprint.w, constraints.minW, constraints.maxW, footprint.w),
    h: clampInt(footprint.h, constraints.minH, constraints.maxH, footprint.h),
  };
}

/**
 * 将布局尺寸映射到最近的三个语义预设之一。
 *
 * 列跨度比行高更影响版面（一次列差通常是数百像素），因此宽度权重更高；
 * 但高度必须全局参与比较，不能只在同宽预设之间作为平手判据——否则
 * 12/44 的趋势卡会被 12/52 的“大”档覆盖，紧凑默认布局失去意义。
 */
export function getWidgetSizePreset(widget: Pick<DashboardWidget, "w" | "h">, bp: Breakpoint, type: string): WidgetSizePreset {
  let closest = WIDGET_SIZE_PRESET_ORDER[0];
  let closestDistance = Number.POSITIVE_INFINITY;
  for (const preset of WIDGET_SIZE_PRESET_ORDER) {
    const size = getGridPresetSize(type, bp, preset);
    const distance = Math.abs(widget.w - size.w) * 2 + Math.abs(widget.h - size.h);
    if (distance < closestDistance) {
      closest = preset;
      closestDistance = distance;
    }
  }
  return closest;
}

/** 默认布局或尺寸预设声明过的尺寸；加载时必须原样保留，不能再次归一化。 */
export function isDeclaredWidgetSize(type: string, bp: Breakpoint, size: Pick<GridSize, "w" | "h">): boolean {
  const fallback = getDefaultGridSize(type, bp);
  if (fallback.w === size.w && fallback.h === size.h) return true;
  return WIDGET_SIZE_PRESET_ORDER.some((preset) => {
    const candidate = getGridPresetSize(type, bp, preset);
    return candidate.w === size.w && candidate.h === size.h;
  });
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

export type DashboardTemplateId = "creator" | "analyst" | "monitor";
export const DASHBOARD_LAYOUT_TEMPLATES: { id: DashboardTemplateId; label: string; description: string }[] = [
  { id: "creator", label: "内容创作者", description: "写作、文章与评论审核" },
  { id: "analyst", label: "数据分析师", description: "趋势、时段、热门与分类" },
  { id: "monitor", label: "极简监控", description: "核心指标与运行状态" },
];

const dashboardTemplateWidgets: Record<DashboardTemplateId, { id: string; preset: WidgetSizePreset }[]> = {
  creator: [
    { id: "stat-posts", preset: "small" },
    { id: "stat-published", preset: "small" },
    { id: "stat-comments", preset: "small" },
    { id: "quick-actions", preset: "medium" },
    { id: "recent-posts", preset: "large" },
    { id: "recent-comments", preset: "large" },
  ],
  analyst: [
    { id: "stat-views", preset: "medium" },
    { id: "trend-chart", preset: "large" },
    { id: "hourly-chart", preset: "medium" },
    { id: "top-posts", preset: "medium" },
    { id: "category-donut", preset: "medium" },
  ],
  monitor: [
    { id: "stat-posts", preset: "small" },
    { id: "stat-published", preset: "small" },
    { id: "stat-views", preset: "small" },
    { id: "stat-comments", preset: "small" },
    { id: "quick-actions", preset: "medium" },
    { id: "system-health", preset: "large" },
  ],
};

/** 根据模板生成所有断点布局；每个断点独立套用离散尺寸并重新找空位。 */
export function createDashboardTemplateLayout(templateId: DashboardTemplateId): MultiBreakpointLayout {
  const entries = dashboardTemplateWidgets[templateId] ?? dashboardTemplateWidgets.creator;
  const result = {} as MultiBreakpointLayout;
  for (const bp of BREAKPOINT_ORDER) {
    const widgets: DashboardWidget[] = [];
    for (const entry of entries) {
      const widget = createDefaultGridWidget(entry.id, bp, widgets);
      widgets.push(applyGridWidgetSize(widget, entry.preset, bp, widgets));
    }
    result[bp] = widgets;
  }
  return result;
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

/** 保持尺寸不变，仅在位置冲突时寻找第一个空位。 */
function placeWidgetPreservingSize(
  widget: DashboardWidget,
  bp: Breakpoint,
  existing: DashboardWidget[],
): DashboardWidget {
  const x = Math.max(0, Math.min(GRID_COLUMNS[bp] - widget.w, widget.x));
  const y = Math.max(0, Math.min(GRID_MAX_ROWS - widget.h, widget.y));
  const position = existing.some((item) => overlaps({ ...widget, x, y }, item))
    ? findFreeGridPosition(existing, { w: widget.w, h: widget.h }, bp)
    : { x, y };
  return { ...widget, ...position };
}

function normalizeBreakpointWidgets(items: unknown, bp: Breakpoint): DashboardWidget[] {
  if (!Array.isArray(items)) return [];
  const result: DashboardWidget[] = [];
  const used = new Set<string>();
  for (const [index, raw] of items.entries()) {
    const item = normalizeWidget(raw, bp, index, used);
    if (!item) continue;
    if (isDeclaredWidgetSize(item.id, bp, item)) {
      // 新默认布局与尺寸弹窗产出的紧凑档位必须原样保留；加载时再归一化会把
      // 3×12 统计卡扩成 3×19、12×44 趋势卡扩成 12×52。
      result.push(placeWidgetPreservingSize(item, bp, result));
      continue;
    }
    // 历史 v9/v10 布局可能保存过任意宽高；非声明尺寸仍落到最近的离散档位。
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
