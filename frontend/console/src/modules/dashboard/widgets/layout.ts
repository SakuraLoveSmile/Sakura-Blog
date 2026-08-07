import type { LayoutWidget } from "./registry";

export type Breakpoint = "l" | "m" | "s";

export const BREAKPOINTS = {
  l: 1280,
  m: 768,
} as const;

/** 根据视口宽度获取当前断点 */
export function getBreakpoint(width: number): Breakpoint {
  if (width >= BREAKPOINTS.l) return "l";
  if (width >= BREAKPOINTS.m) return "m";
  return "s";
}

/** 按区域分组的布局（每个断点独立） */
export interface SectionLayout {
  stats: LayoutWidget[];
  chartWide: LayoutWidget | null;
  chartNarrow: LayoutWidget[];
  recents: LayoutWidget[];
  others: LayoutWidget[];
}

/** 全断点布局存储结构（v6：每个断点独立保存） */
export type MultiBreakpointLayout = Record<Breakpoint, SectionLayout>;

/** 每个断点的默认布局 - 从v5的ResponsiveSize拆分为独立布局 */
const defaultLayout_l: SectionLayout = {
  stats: [
    { type: "stat-posts",     size: { l: 3, m: 6, s: 12 } },
    { type: "stat-published", size: { l: 3, m: 6, s: 12 } },
    { type: "stat-views",     size: { l: 3, m: 6, s: 12 } },
    { type: "stat-comments",  size: { l: 3, m: 6, s: 12 } },
  ],
  chartWide: { type: "trend-chart", size: { l: 8, m: 12, s: 12 } },
  chartNarrow: [
    { type: "category-donut", size: { l: 4, m: 6, s: 12 } },
    { type: "quick-actions",  size: { l: 4, m: 6, s: 12 } },
  ],
  recents: [
    { type: "recent-posts",   size: { l: 6, m: 6, s: 12 } },
    { type: "recent-comments",size: { l: 6, m: 6, s: 12 } },
  ],
  others: [],
};

const defaultLayout_m: SectionLayout = {
  stats: [
    { type: "stat-posts",     size: { l: 3, m: 6, s: 12 } },
    { type: "stat-published", size: { l: 3, m: 6, s: 12 } },
    { type: "stat-views",     size: { l: 3, m: 6, s: 12 } },
    { type: "stat-comments",  size: { l: 3, m: 6, s: 12 } },
  ],
  chartWide: { type: "trend-chart", size: { l: 8, m: 12, s: 12 } },
  chartNarrow: [
    { type: "category-donut", size: { l: 4, m: 6, s: 12 } },
    { type: "quick-actions",  size: { l: 4, m: 6, s: 12 } },
  ],
  recents: [
    { type: "recent-posts",   size: { l: 6, m: 6, s: 12 } },
    { type: "recent-comments",size: { l: 6, m: 6, s: 12 } },
  ],
  others: [],
};

const defaultLayout_s: SectionLayout = {
  stats: [
    { type: "stat-posts",     size: { l: 3, m: 6, s: 12 } },
    { type: "stat-published", size: { l: 3, m: 6, s: 12 } },
    { type: "stat-views",     size: { l: 3, m: 6, s: 12 } },
    { type: "stat-comments",  size: { l: 3, m: 6, s: 12 } },
  ],
  chartWide: { type: "trend-chart", size: { l: 8, m: 12, s: 12 } },
  chartNarrow: [
    { type: "category-donut", size: { l: 4, m: 6, s: 12 } },
    { type: "quick-actions",  size: { l: 4, m: 6, s: 12 } },
  ],
  recents: [
    { type: "recent-posts",   size: { l: 6, m: 6, s: 12 } },
    { type: "recent-comments",size: { l: 6, m: 6, s: 12 } },
  ],
  others: [],
};

export const defaultMultiLayout: MultiBreakpointLayout = {
  l: JSON.parse(JSON.stringify(defaultLayout_l)),
  m: JSON.parse(JSON.stringify(defaultLayout_m)),
  s: JSON.parse(JSON.stringify(defaultLayout_s)),
};

/** 获取单个断点的默认布局（深拷贝） */
export function getDefaultSectionLayout(bp: Breakpoint): SectionLayout {
  const base = bp === "l" ? defaultLayout_l : bp === "m" ? defaultLayout_m : defaultLayout_s;
  return JSON.parse(JSON.stringify(base));
}

/** 兼容v5格式转换为v6 */
function migrateV5toV6(v5: any): MultiBreakpointLayout | null {
  try {
    // v5格式是单个SectionLayout
    if (v5 && (v5.stats || v5.chartWide)) {
      // 把v5布局作为l的基础，m和s用默认，但widget的size保持
      const v5Layout = v5 as SectionLayout;
      return {
        l: JSON.parse(JSON.stringify(v5Layout)),
        m: JSON.parse(JSON.stringify(defaultLayout_m)),
        s: JSON.parse(JSON.stringify(defaultLayout_s)),
      };
    }
  } catch { /* ignore */ }
  return null;
}

/** 布局存储key */
export const LAYOUT_STORAGE_KEY = "dashboard:layout:v6";

/** 从localStorage加载布局，支持v5迁移 */
export function loadLayoutFromStorage(): MultiBreakpointLayout {
  try {
    const saved = localStorage.getItem(LAYOUT_STORAGE_KEY);
    if (saved) {
      const parsed = JSON.parse(saved);
      // 验证格式是否正确（包含l/m/s三个key）
      if (parsed.l && parsed.m && parsed.s) {
        return {
          l: { ...defaultMultiLayout.l, ...parsed.l },
          m: { ...defaultMultiLayout.m, ...parsed.m },
          s: { ...defaultMultiLayout.s, ...parsed.s },
        };
      }
      // 尝试v5迁移
      const migrated = migrateV5toV6(parsed);
      if (migrated) return migrated;
    }
  } catch { /* ignore */ }
  return JSON.parse(JSON.stringify(defaultMultiLayout));
}

/** 保存布局到localStorage */
export function saveLayoutToStorage(layout: MultiBreakpointLayout) {
  localStorage.setItem(LAYOUT_STORAGE_KEY, JSON.stringify(layout));
}
