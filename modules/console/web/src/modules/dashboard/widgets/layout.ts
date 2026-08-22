export type Breakpoint = "l" | "m" | "s";

export const BREAKPOINTS = {
  l: 1280,
  m: 768,
} as const;

export const GRID_COLUMNS = 12;
/** 布局可使用的总行数，需覆盖默认布局和用户后续追加的组件。 */
export const GRID_MAX_ROWS = 512;
/** 单个组件的最大高度，避免尺寸编辑器产生不可控的大卡片。 */
export const GRID_MAX_WIDGET_ROWS = 64;

/** 根据视口宽度获取当前断点。 */
export function getBreakpoint(width: number): Breakpoint {
  if (width >= BREAKPOINTS.l) return "l";
  if (width >= BREAKPOINTS.m) return "m";
  return "s";
}

export interface GridWidget {
  type: string;
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface GridBreakpointLayout {
  widgets: GridWidget[];
}

/** 三个断点各自独立保存，组件不再属于固定区域。 */
export type MultiBreakpointLayout = Record<Breakpoint, GridBreakpointLayout>;

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

const GRID_DEFAULTS: Record<string, Record<Breakpoint, GridSize>> = {
  "stat-posts": { l: { w: 3, h: 19 }, m: { w: 6, h: 18 }, s: { w: 12, h: 18 } },
  "stat-published": { l: { w: 3, h: 19 }, m: { w: 6, h: 18 }, s: { w: 12, h: 18 } },
  "stat-views": { l: { w: 3, h: 19 }, m: { w: 6, h: 18 }, s: { w: 12, h: 18 } },
  "stat-comments": { l: { w: 3, h: 19 }, m: { w: 6, h: 18 }, s: { w: 12, h: 18 } },
  "stat-attachments": { l: { w: 3, h: 19 }, m: { w: 6, h: 18 }, s: { w: 12, h: 18 } },
  "stat-users": { l: { w: 3, h: 19 }, m: { w: 6, h: 18 }, s: { w: 12, h: 18 } },
  "trend-chart": { l: { w: 8, h: 52 }, m: { w: 12, h: 44 }, s: { w: 12, h: 42 } },
  "hourly-chart": { l: { w: 4, h: 40 }, m: { w: 6, h: 37 }, s: { w: 12, h: 35 } },
  "category-donut": { l: { w: 4, h: 34 }, m: { w: 6, h: 32 }, s: { w: 12, h: 29 } },
  "quick-actions": { l: { w: 4, h: 35 }, m: { w: 6, h: 31 }, s: { w: 12, h: 20 } },
  "recent-posts": { l: { w: 6, h: 36 }, m: { w: 12, h: 32 }, s: { w: 12, h: 27 } },
  "recent-comments": { l: { w: 6, h: 36 }, m: { w: 12, h: 32 }, s: { w: 12, h: 27 } },
  "stellar-version": { l: { w: 4, h: 25 }, m: { w: 6, h: 23 }, s: { w: 12, h: 22 } },
  "system-version": { l: { w: 4, h: 25 }, m: { w: 6, h: 23 }, s: { w: 12, h: 22 } },
};

/** 组件的默认网格尺寸，新增组件以当前断点的默认值落位。 */
export function getDefaultGridSize(type: string, bp: Breakpoint): GridSize {
  return { ...(GRID_DEFAULTS[type]?.[bp] ?? { w: bp === "s" ? 12 : 6, h: 12 }) };
}

/** 组件最小尺寸来自现有卡片内容，避免自由缩放时把内容裁掉。 */
export function getGridConstraints(type: string, bp: Breakpoint): GridConstraints {
  const minH = GRID_DEFAULTS[type]?.[bp]?.h ?? 12;
  const minW = bp === "s"
    ? 12
    : type === "trend-chart"
      ? 4
      : type.startsWith("stat-")
        ? 2
        : 3;
  return { minW, minH, maxW: GRID_COLUMNS, maxH: GRID_MAX_WIDGET_ROWS };
}

function cloneLayout(layout: MultiBreakpointLayout): MultiBreakpointLayout {
  return JSON.parse(JSON.stringify(layout)) as MultiBreakpointLayout;
}

function widget(type: string, x: number, y: number, w: number, h: number): GridWidget {
  return { type, x, y, w, h };
}

const defaultLayout_l: GridBreakpointLayout = {
  widgets: [
    widget("stat-posts", 0, 0, 3, 19),
    widget("stat-published", 3, 0, 3, 19),
    widget("stat-views", 6, 0, 3, 19),
    widget("stat-comments", 9, 0, 3, 19),
    widget("trend-chart", 0, 19, 8, 52),
    widget("category-donut", 8, 19, 4, 34),
    widget("quick-actions", 8, 53, 4, 35),
    widget("recent-posts", 0, 88, 6, 36),
    widget("recent-comments", 6, 88, 6, 36),
    widget("stellar-version", 0, 124, 4, 25),
  ],
};

const defaultLayout_m: GridBreakpointLayout = {
  widgets: [
    widget("stat-posts", 0, 0, 6, 18),
    widget("stat-published", 6, 0, 6, 18),
    widget("stat-views", 0, 18, 6, 18),
    widget("stat-comments", 6, 18, 6, 18),
    widget("trend-chart", 0, 36, 12, 44),
    widget("category-donut", 0, 80, 6, 32),
    widget("quick-actions", 6, 80, 6, 31),
    widget("recent-posts", 0, 112, 12, 32),
    widget("recent-comments", 0, 144, 12, 32),
    widget("stellar-version", 0, 176, 6, 23),
  ],
};

const defaultLayout_s: GridBreakpointLayout = {
  widgets: [
    widget("stat-posts", 0, 0, 12, 18),
    widget("stat-published", 0, 18, 12, 18),
    widget("stat-views", 0, 36, 12, 18),
    widget("stat-comments", 0, 54, 12, 18),
    widget("trend-chart", 0, 72, 12, 42),
    widget("category-donut", 0, 114, 12, 29),
    widget("quick-actions", 0, 143, 12, 20),
    widget("recent-posts", 0, 163, 12, 27),
    widget("recent-comments", 0, 190, 12, 27),
    widget("stellar-version", 0, 217, 12, 22),
  ],
};

export const defaultMultiLayout: MultiBreakpointLayout = {
  l: { widgets: defaultLayout_l.widgets.map((item) => ({ ...item })) },
  m: { widgets: defaultLayout_m.widgets.map((item) => ({ ...item })) },
  s: { widgets: defaultLayout_s.widgets.map((item) => ({ ...item })) },
};

/** 获取单个断点的默认网格布局。 */
export function getDefaultGridLayout(bp: Breakpoint): GridBreakpointLayout {
  return cloneLayout(defaultMultiLayout)[bp];
}

export const LAYOUT_STORAGE_KEY = "dashboard:layout:v9";
const LEGACY_V8_STORAGE_KEY = "dashboard:layout:v8";
const LEGACY_V7_STORAGE_KEY = "dashboard:layout:v7";
const LEGACY_V6_STORAGE_KEY = "dashboard:layout:v6";

interface LegacyResponsiveSize {
  l?: number;
  m?: number;
  s?: number;
}

interface LegacyWidget {
  type?: unknown;
  size?: LegacyResponsiveSize;
}

interface LegacySectionLayout {
  stats?: LegacyWidget[];
  chartWide?: LegacyWidget | null;
  chartNarrow?: LegacyWidget[];
  recents?: LegacyWidget[];
  others?: LegacyWidget[];
}

function clampInt(value: unknown, min: number, max: number, fallback: number): number {
  const n = typeof value === "number" && Number.isFinite(value) ? Math.round(value) : fallback;
  return Math.max(min, Math.min(max, n));
}

function normalizeGridWidget(item: Partial<GridWidget>, bp: Breakpoint): GridWidget | null {
  if (typeof item.type !== "string" || !item.type) return null;
  const constraints = getGridConstraints(item.type, bp);
  const defaults = getDefaultGridSize(item.type, bp);
  const w = clampInt(item.w, constraints.minW, constraints.maxW, defaults.w);
  const h = clampInt(item.h, constraints.minH, constraints.maxH, defaults.h);
  const x = clampInt(item.x, 0, GRID_COLUMNS - w, 0);
  const y = clampInt(item.y, 0, GRID_MAX_ROWS - h, 0);
  return { type: item.type, x, y, w, h };
}

function overlaps(a: GridWidget, b: GridWidget): boolean {
  return a.x < b.x + b.w && a.x + a.w > b.x && a.y < b.y + b.h && a.y + a.h > b.y;
}

export function hasGridOverlaps(widgets: GridWidget[]): boolean {
  for (let i = 0; i < widgets.length; i += 1) {
    for (let j = i + 1; j < widgets.length; j += 1) {
      if (overlaps(widgets[i], widgets[j])) return true;
    }
  }
  return false;
}

/** 按 y/x 顺序寻找第一个不与已有组件重叠的位置。 */
export function findFreeGridPosition(
  widgets: GridWidget[],
  size: GridSize,
): Pick<GridWidget, "x" | "y"> {
  const w = clampInt(size.w, 1, GRID_COLUMNS, size.w);
  const h = clampInt(size.h, 1, GRID_MAX_WIDGET_ROWS, size.h);
  for (let y = 0; y <= GRID_MAX_ROWS - h; y += 1) {
    for (let x = 0; x <= GRID_COLUMNS - w; x += 1) {
      const candidate = { type: "", x, y, w, h };
      if (!widgets.some((item) => overlaps(candidate, item))) return { x, y };
    }
  }
  const bottom = widgets.reduce((max, item) => Math.max(max, item.y + item.h), 0);
  return { x: 0, y: bottom };
}

export function createDefaultGridWidget(type: string, bp: Breakpoint, widgets: GridWidget[] = []): GridWidget {
  const size = getDefaultGridSize(type, bp);
  const position = findFreeGridPosition(widgets, size);
  return { type, ...position, ...size };
}

function normalizeWidgets(items: unknown, bp: Breakpoint): GridWidget[] {
  if (!Array.isArray(items)) return [];
  const result: GridWidget[] = [];
  const seen = new Set<string>();
  for (const raw of items) {
    const item = normalizeGridWidget((raw ?? {}) as Partial<GridWidget>, bp);
    if (!item || seen.has(item.type)) continue;
    seen.add(item.type);
    if (result.some((existing) => overlaps(existing, item))) {
      const position = findFreeGridPosition(result, { w: item.w, h: item.h });
      result.push({ ...item, ...position });
    } else {
      result.push(item);
    }
  }
  return result;
}

export function sanitizeGridLayout(input: unknown): MultiBreakpointLayout | null {
  if (!input || typeof input !== "object") return null;
  const source = input as Record<string, unknown>;
  const output = {} as MultiBreakpointLayout;
  for (const bp of ["l", "m", "s"] as const) {
    const section = source[bp];
    if (!section || typeof section !== "object") return null;
    const widgets = normalizeWidgets((section as { widgets?: unknown }).widgets, bp);
    output[bp] = { widgets };
  }
  return output;
}

/** v6 → v7：保留历史测试与旧布局迁移语义。 */
export function migrateV6toV7(v6: any): any | null {
  try {
    if (!v6 || !v6.l || !v6.m || !v6.s) return null;
    const layout = JSON.parse(JSON.stringify(v6));
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

function legacyItems(section: LegacySectionLayout): LegacyWidget[] {
  return [
    ...(Array.isArray(section.stats) ? section.stats : []),
    ...(section.chartWide ? [section.chartWide] : []),
    ...(Array.isArray(section.chartNarrow) ? section.chartNarrow : []),
    ...(Array.isArray(section.recents) ? section.recents : []),
    ...(Array.isArray(section.others) ? section.others : []),
  ];
}

function packLegacyItems(items: LegacyWidget[], bp: Breakpoint): GridWidget[] {
  const result: GridWidget[] = [];
  let cursorX = 0;
  let cursorY = 0;
  let rowHeight = 0;
  for (const legacy of items) {
    if (typeof legacy.type !== "string" || !legacy.type) continue;
    const defaults = getDefaultGridSize(legacy.type, bp);
    const width = clampInt(legacy.size?.[bp], 1, GRID_COLUMNS, defaults.w);
    if (cursorX + width > GRID_COLUMNS) {
      cursorX = 0;
      cursorY += rowHeight;
      rowHeight = 0;
    }
    const packed = normalizeGridWidget(
      { type: legacy.type, x: cursorX, y: cursorY, w: width, h: defaults.h },
      bp,
    );
    if (packed && !result.some((existing) => existing.type === packed.type)) {
      result.push(packed);
      cursorX = packed.x + packed.w;
      rowHeight = Math.max(rowHeight, packed.h);
    }
  }
  return result;
}

/** v8/v7 旧五区域布局 → v9 统一网格布局。 */
export function migrateV8toV9(input: unknown): MultiBreakpointLayout | null {
  if (!input || typeof input !== "object") return null;
  const source = input as Record<Breakpoint, LegacySectionLayout | undefined>;
  const output = {} as MultiBreakpointLayout;
  for (const bp of ["l", "m", "s"] as const) {
    const section = source[bp];
    if (!section || typeof section !== "object") return null;
    output[bp] = { widgets: packLegacyItems(legacyItems(section), bp) };
  }
  return output;
}

/** 从 localStorage 加载 v9，兼容 v8/v7/v6。 */
export function loadLayoutFromStorage(): MultiBreakpointLayout {
  try {
    const current = localStorage.getItem(LAYOUT_STORAGE_KEY);
    if (current) {
      const parsed = sanitizeGridLayout(JSON.parse(current));
      if (parsed) return parsed;
    }

    const legacyV8 = localStorage.getItem(LEGACY_V8_STORAGE_KEY);
    if (legacyV8) {
      const migrated = migrateV8toV9(JSON.parse(legacyV8));
      if (migrated) {
        saveLayoutToStorage(migrated);
        return migrated;
      }
    }

    const legacyV7 = localStorage.getItem(LEGACY_V7_STORAGE_KEY);
    if (legacyV7) {
      const migrated = migrateV8toV9(JSON.parse(legacyV7));
      if (migrated) {
        saveLayoutToStorage(migrated);
        return migrated;
      }
    }

    const legacyV6 = localStorage.getItem(LEGACY_V6_STORAGE_KEY);
    if (legacyV6) {
      const migratedV7 = migrateV6toV7(JSON.parse(legacyV6));
      const migrated = migrateV8toV9(migratedV7);
      if (migrated) {
        saveLayoutToStorage(migrated);
        return migrated;
      }
    }
  } catch {
    // 坏的 localStorage 不应阻断 Dashboard 启动，回退到默认布局。
  }
  return cloneLayout(defaultMultiLayout);
}

export function saveLayoutToStorage(layout: MultiBreakpointLayout) {
  localStorage.setItem(LAYOUT_STORAGE_KEY, JSON.stringify(layout));
}

/** 将网格引擎输出的节点转换成可持久化的纯数据。 */
export function nodesToGridWidgets(nodes: Array<Partial<GridWidget> & { id?: unknown }>): GridWidget[] {
  return nodes
    .map((node) => ({
      type: typeof node.type === "string" ? node.type : typeof node.id === "string" ? node.id : "",
      x: Math.max(0, Math.round(node.x ?? 0)),
      y: Math.max(0, Math.round(node.y ?? 0)),
      w: Math.max(1, Math.round(node.w ?? 1)),
      h: Math.max(1, Math.round(node.h ?? 1)),
    }))
    .filter((node) => Boolean(node.type))
    .sort((a, b) => a.y - b.y || a.x - b.x || a.type.localeCompare(b.type));
}
