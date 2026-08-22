// 统一网格布局的数据模型、迁移和边界行为。
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  defaultMultiLayout,
  findFreeGridPosition,
  getGridConstraints,
  hasGridOverlaps,
  migrateV6toV7,
  migrateV8toV9,
  nodesToGridWidgets,
  sanitizeGridLayout,
} from "../web/src/modules/dashboard/widgets/layout.ts";

test("默认三断点布局使用统一网格且没有重叠", () => {
  for (const bp of ["l", "m", "s"] as const) {
    assert.ok(defaultMultiLayout[bp].widgets.length > 0);
    assert.equal(hasGridOverlaps(defaultMultiLayout[bp].widgets), false, `${bp} 布局重叠`);
  }
});

test("v8 五区域布局迁移为单一 widgets 列表并保留组件顺序", () => {
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
  assert.deepEqual(migrated.l.widgets.map((item) => item.type), [
    "stat-posts",
    "trend-chart",
    "category-donut",
    "recent-posts",
    "stellar-version",
  ]);
  assert.equal(migrated.l.widgets[1].w, 8);
  assert.equal(hasGridOverlaps(migrated.l.widgets), false);
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

test("sanitizeGridLayout: 修正边界、去重并拒绝缺失断点", () => {
  const input = {
    l: {
      widgets: [
        { type: "stat-posts", x: -4, y: -2, w: 99, h: 999 },
        { type: "stat-posts", x: 4, y: 4, w: 4, h: 4 },
        { type: "trend-chart", x: 1, y: 2, w: 2, h: 2 },
      ],
    },
    m: { widgets: [] },
    s: { widgets: [] },
  };
  const output = sanitizeGridLayout(input);
  assert.ok(output);
  assert.equal(output.l.widgets.length, 2);
  assert.deepEqual(output.l.widgets[0], { type: "stat-posts", x: 0, y: 0, w: 12, h: 64 });
  assert.equal(sanitizeGridLayout({ l: { widgets: [] }, m: { widgets: [] } }), null);
});

test("findFreeGridPosition: 返回不重叠的首个位置", () => {
  const widgets = [{ type: "a", x: 0, y: 0, w: 6, h: 4 }];
  assert.deepEqual(findFreeGridPosition(widgets, { w: 6, h: 4 }), { x: 6, y: 0 });
});

test("节点序列化按 y/x 排序并使用 id 作为组件类型", () => {
  const output = nodesToGridWidgets([
    { id: "b", x: 3, y: 2, w: 2, h: 4 },
    { id: "a", x: 0, y: 1, w: 4, h: 3 },
  ] as any);
  assert.deepEqual(output, [
    { type: "a", x: 0, y: 1, w: 4, h: 3 },
    { type: "b", x: 3, y: 2, w: 2, h: 4 },
  ]);
});

test("组件最小尺寸在小屏固定为全宽，趋势图大屏至少四列", () => {
  assert.equal(getGridConstraints("stat-posts", "s").minW, 12);
  assert.equal(getGridConstraints("trend-chart", "l").minW, 4);
});
