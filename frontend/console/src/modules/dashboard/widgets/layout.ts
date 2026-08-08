import type { LayoutWidget, ResponsiveSize } from "./registry";

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

/** 全断点布局存储结构（每个断点独立保存） */
export type MultiBreakpointLayout = Record<Breakpoint, SectionLayout>;

/** 每个断点的默认布局 */
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
    { type: "recent-posts",   size: { l: 6, m: 12, s: 12 } },
    { type: "recent-comments",size: { l: 6, m: 12, s: 12 } },
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

/** 布局存储key */
export const LAYOUT_STORAGE_KEY = "dashboard:layout:v7";

/** v6 旧布局存储 key（仅迁移用） */
const LEGACY_STORAGE_KEY = "dashboard:layout:v6";

/** 旧默认尺寸：v6 时代 m 断点 recents 的默认值 */
const OLD_DEFAULT_RECENT_SIZE: ResponsiveSize = { l: 6, m: 6, s: 12 };
/** 新默认尺寸：v7 时代 m 断点 recents 的默认值 */
const NEW_DEFAULT_RECENT_SIZE: ResponsiveSize = { l: 6, m: 12, s: 12 };

/**
 * v6 布局迁移到 v7：结构与 v7 相同（{l,m,s} 三段），仅将 m 断点 recents 中
 * recent-posts/recent-comments 仍为旧默认尺寸 {l:6,m:6,s:12} 的替换为新默认
 * {l:6,m:12,s:12}；用户自定义过的其他部件与尺寸全部保留
 */
function migrateV6toV7(v6: any): MultiBreakpointLayout | null {
  try {
    if (!v6 || !v6.l || !v6.m || !v6.s) return null;
    // 深拷贝，避免直接改动 localStorage 原始对象
    const layout = JSON.parse(JSON.stringify(v6)) as MultiBreakpointLayout;
    const recents = layout.m.recents;
    if (Array.isArray(recents)) {
      for (const w of recents) {
        const size = w.size;
        const isOldDefault =
          size &&
          size.l === OLD_DEFAULT_RECENT_SIZE.l &&
          size.m === OLD_DEFAULT_RECENT_SIZE.m &&
          size.s === OLD_DEFAULT_RECENT_SIZE.s;
        if (
          (w.type === "recent-posts" || w.type === "recent-comments") &&
          isOldDefault
        ) {
          w.size = { ...NEW_DEFAULT_RECENT_SIZE };
        }
      }
    }
    return layout;
  } catch { /* ignore */ }
  return null;
}

/** 从localStorage加载布局，支持v6迁移 */
export function loadLayoutFromStorage(): MultiBreakpointLayout {
  try {
    // 优先读 v7 key
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
    }
    // v7 缺失或格式无效时回退读 v6 key 并迁移
    const legacy = localStorage.getItem(LEGACY_STORAGE_KEY);
    if (legacy) {
      const migrated = migrateV6toV7(JSON.parse(legacy));
      if (migrated && migrated.l && migrated.m && migrated.s) {
        const layout = {
          l: { ...defaultMultiLayout.l, ...migrated.l },
          m: { ...defaultMultiLayout.m, ...migrated.m },
          s: { ...defaultMultiLayout.s, ...migrated.s },
        };
        // 迁移成功后写回 v7 key 并清理 v6 key
        saveLayoutToStorage(layout);
        localStorage.removeItem(LEGACY_STORAGE_KEY);
        return layout;
      }
    }
  } catch { /* ignore */ }
  return JSON.parse(JSON.stringify(defaultMultiLayout));
}

/** 保存布局到localStorage */
export function saveLayoutToStorage(layout: MultiBreakpointLayout) {
  localStorage.setItem(LAYOUT_STORAGE_KEY, JSON.stringify(layout));
}
