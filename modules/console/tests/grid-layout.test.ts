// Halo 风格 Dashboard v10 数据模型、迁移和网格边界行为。
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  BREAKPOINT_ORDER,
  GRID_COLUMNS,
  copyLayoutToBreakpoint,
  defaultMultiLayout,
  ensureCompleteLayout,
  findFreeGridPosition,
  getBreakpoint,
  getGridConstraints,
  hasGridOverlaps,
  mergeGridWidgetMetadata,
  migrateV6toV7,
  migrateV8toV9,
  migrateV9toV10,
  nodesToGridWidgets,
  sanitizeDashboardLayouts,
} from "../web/src/modules/dashboard/widgets/layout.ts";

test("默认五断点布局使用统一网格且没有重叠", () => {
  for (const bp of BREAKPOINT_ORDER) {
    assert.ok(defaultMultiLayout[bp].length > 0);
    assert.equal(hasGridOverlaps(defaultMultiLayout[bp]), false, `${bp} 布局重叠`);
    assert.ok(defaultMultiLayout[bp].every((item) => item.x + item.w <= GRID_COLUMNS[bp]));
  }
});

test("v8 五区域布局迁移为 v9 组件列表并保留顺序", () => {
  const old = {
    l: {
      stats: [{ type: "stat-posts", size: { l: 3, m: 6, s: 12 } }],
      chartWide: { type: "trend-chart", size: { l: 8, m: 12, s: 12 } },
      chartNarrow: [{ type: "category-donut", size: { l: 4, m: 6, s: 12 } }],
      recents: [{ type: "recent-posts", size: { l: 6, m: 12, s: 12 } }],
      others: [{ type: "stellar-version", size: { l: 4, m: 6, s: 12 } }],
    },
    m: { stats: [], chartWide: null, chartNarrow: [], recents: [], others: [] },
    s: { stats: [], chartWide: null, chartNarrow: [], recents: [], others: [] },
  };
  const migrated = migrateV8toV9(old);
  assert.ok(migrated);
  assert.deepEqual(migrated.l.widgets.map((item) => item.id), [
    "stat-posts",
    "trend-chart",
    "category-donut",
    "recent-posts",
    "stellar-version",
  ]);
  assert.equal(migrated.l.widgets[1].w, 8);
  assert.equal(hasGridOverlaps(migrated.l.widgets), false);
});

test("v9 统一网格迁移为 v10 实例模型和五断点", () => {
  const old = {
    l: { widgets: [{ type: "trend-chart", x: 0, y: 0, w: 8, h: 30 }] },
    m: { widgets: [{ type: "quick-actions", x: 6, y: 0, w: 6, h: 20 }] },
    s: { widgets: [{ type: "quick-actions", x: 0, y: 0, w: 12, h: 20 }] },
  };
  const migrated = migrateV9toV10(old);
  assert.ok(migrated);
  assert.deepEqual(Object.keys(migrated).sort(), ["lg", "md", "sm", "xs", "xxs"]);
  assert.equal(migrated.lg?.[0].id, "trend-chart");
  assert.equal(typeof migrated.lg?.[0].i, "string");
  assert.equal(migrated.xs?.[0].w, 4);
  assert.equal(hasGridOverlaps(migrated.sm ?? []), false);
});

test("v6 迁移仍保留旧 recent 默认尺寸修复", () => {
  const old = {
    l: { stats: [], recents: [] },
    m: {
      stats: [],
      recents: [{ type: "recent-posts", size: { l: 6, m: 6, s: 12 } }],
    },
    s: { stats: [], recents: [] },
  };
  const migrated = migrateV6toV7(old);
  assert.deepEqual(migrated?.m.recents[0].size, { l: 6, m: 12, s: 12 });
  assert.deepEqual(old.m.recents[0].size, { l: 6, m: 6, s: 12 });
});

test("sanitizeDashboardLayouts: 修正边界、去重并保留实例", () => {
  const output = sanitizeDashboardLayouts({
    lg: [
      { id: "stat-posts", i: "same", x: -4, y: -2, w: 99, h: 999 },
      { id: "stat-posts", i: "same", x: 4, y: 4, w: 4, h: 8 },
      { id: "future:widget", i: "future-1", x: 1, y: 80, w: 2, h: 8 },
    ],
    xs: [],
  });
  assert.ok(output);
  assert.equal(output.lg?.length, 3);
  assert.equal(output.lg?.[0].w, 12);
  assert.equal(output.lg?.[0].h, 64);
  assert.notEqual(output.lg?.[0].i, output.lg?.[1].i);
  assert.equal(output.lg?.[2].id, "future:widget");
  assert.equal(hasGridOverlaps(output.lg ?? []), false);
  assert.equal(sanitizeDashboardLayouts({ lg: "invalid" }), null);
});

test("ensureCompleteLayout 为缺失断点提供独立默认布局", () => {
  const output = ensureCompleteLayout({ lg: [] });
  assert.deepEqual(Object.keys(output), BREAKPOINT_ORDER);
  assert.deepEqual(output.lg, []);
  assert.ok(output.xs.length > 0);
});

test("findFreeGridPosition: 返回不重叠的首个位置", () => {
  const widgets = [{ i: "a", id: "a", x: 0, y: 0, w: 3, h: 4 }];
  assert.deepEqual(findFreeGridPosition(widgets, { w: 3, h: 4 }, "sm"), { x: 3, y: 0 });
});

test("节点序列化保留实例 ID、定义 ID 和最终尺寸", () => {
  const output = nodesToGridWidgets([
    { i: "instance-b", widgetId: "future:b", x: 3, y: 2, w: 2, h: 8 },
    { i: "instance-a", widgetId: "future:a", x: 0, y: 1, w: 4, h: 8 },
  ], "lg");
  assert.deepEqual(output.map((item) => ({ i: item.i, id: item.id, x: item.x, y: item.y, w: item.w, h: item.h })), [
    { i: "instance-a", id: "future:a", x: 0, y: 1, w: 4, h: 8 },
    { i: "instance-b", id: "future:b", x: 3, y: 2, w: 2, h: 8 },
  ]);
});

test("网格几何更新不会丢失实例配置和权限", () => {
  const original = {
    i: "quick-1",
    id: "quick-actions",
    x: 1,
    y: 2,
    w: 4,
    h: 20,
    config: { title: "常用入口" },
    permissions: ["dashboard:read"],
  };
  const next = nodesToGridWidgets([
    { i: original.i, widgetId: original.id, x: 3, y: 4, w: 5, h: 22 },
  ], "lg");
  const merged = mergeGridWidgetMetadata(next, new Map([[original.i, original]]));
  assert.deepEqual(merged[0].config, { title: "常用入口" });
  assert.deepEqual(merged[0].permissions, ["dashboard:read"]);
  assert.equal(merged[0].x, 3);
  assert.equal(merged[0].w, 5);
});

test("Halo 断点、组件最小尺寸和布局复制", () => {
  assert.equal(getBreakpoint(1440), "lg");
  assert.equal(getBreakpoint(800), "sm");
  assert.equal(getBreakpoint(390), "xxs");
  assert.equal(getGridConstraints("stat-posts", "xs").minW, 2);
  assert.equal(getGridConstraints("trend-chart", "lg").minW, 4);

  const copied = copyLayoutToBreakpoint({
    lg: [{ i: "trend-1", id: "trend-chart", x: 0, y: 0, w: 8, h: 30 }],
    xs: [],
  }, "lg", "xs");
  assert.equal(copied.xs?.[0].w, 3);
  assert.notEqual(copied.xs?.[0].i, "trend-1");
  assert.equal(hasGridOverlaps(copied.xs ?? []), false);
});
