// 行为面：moveWidget 纯函数（src/modules/dashboard/widgets/layout-move.ts）。
// 覆盖同区重排、跨区域移动、落入已占用 chartWide 的占用者换回、chartWide 拖出、
// to.index 越界钳制、非法 from.index 原样返回、纯函数不修改入参、其余区域不变。
// 机械校验入口：make test → console-test → node --test ../tests/*.test.ts
import { test } from "node:test";
import assert from "node:assert/strict";
import { moveWidget } from "../web/src/modules/dashboard/widgets/layout-move.ts";
import type { SectionLayout, LayoutWidget } from "../web/src/modules/dashboard/widgets/layout.ts";

function w(type: string, l = 3, m = 6, s = 12): LayoutWidget {
  return { type, size: { l, m, s } };
}

function base(): SectionLayout {
  return {
    stats: [w("stat-posts"), w("stat-published"), w("stat-views"), w("stat-comments")],
    chartWide: w("trend-chart", 8, 12, 12),
    chartNarrow: [w("category-donut", 4, 6, 12), w("quick-actions", 4, 6, 12)],
    recents: [w("recent-posts", 6, 6, 12), w("recent-comments", 6, 6, 12)],
    others: [w("stellar-version", 4, 6, 12)],
  };
}

const types = (list: LayoutWidget[]) => list.map((x) => x.type);

test("moveWidget: 同区（stats）重排，index0→index1", () => {
  const r = moveWidget(base(), { key: "stats", index: 0 }, { key: "stats", index: 1 });
  assert.deepEqual(types(r.stats), [
    "stat-published",
    "stat-posts",
    "stat-views",
    "stat-comments",
  ]);
  assert.equal(r.chartWide?.type, "trend-chart", "chartWide 不变");
});

test("moveWidget: chartNarrow 内部重排", () => {
  const r = moveWidget(
    base(),
    { key: "chartNarrow", index: 1 },
    { key: "chartNarrow", index: 0 },
  );
  assert.deepEqual(types(r.chartNarrow), ["quick-actions", "category-donut"]);
});

test("moveWidget: 跨区域（recents→stats）移动", () => {
  const r = moveWidget(
    base(),
    { key: "recents", index: 0 },
    { key: "stats", index: 4 },
  );
  assert.deepEqual(types(r.recents), ["recent-comments"]);
  assert.deepEqual(types(r.stats), [
    "stat-posts",
    "stat-published",
    "stat-views",
    "stat-comments",
    "recent-posts",
  ]);
});

test("moveWidget: 拖入已占用的 chartWide，占用者换回来源区原位置", () => {
  const r = moveWidget(base(), { key: "stats", index: 1 }, { key: "chartWide", index: 0 });
  // 被拖组件占据 chartWide
  assert.equal(r.chartWide?.type, "stat-published");
  // 占用者 trend-chart 换回 stats 的原 index(1) 位置
  assert.deepEqual(types(r.stats), [
    "stat-posts",
    "trend-chart",
    "stat-views",
    "stat-comments",
  ]);
});

test("moveWidget: 从 chartWide 拖出到列表区，chartWide 置空", () => {
  const r = moveWidget(base(), { key: "chartWide", index: 0 }, { key: "others", index: 0 });
  assert.equal(r.chartWide, null);
  assert.deepEqual(types(r.others), ["trend-chart", "stellar-version"]);
});

test("moveWidget: to.index 越界时钳制到列表末尾", () => {
  const r = moveWidget(
    base(),
    { key: "recents", index: 0 },
    { key: "others", index: 999 },
  );
  assert.deepEqual(types(r.others), ["stellar-version", "recent-posts"]);
});

test("moveWidget: 非法 from.index 原样返回入参", () => {
  const input = base();
  const r = moveWidget(input, { key: "stats", index: 99 }, { key: "stats", index: 0 });
  assert.equal(r, input, "应直接返回原对象引用");
  assert.deepEqual(types(r.stats), types(input.stats));
});

test("moveWidget: 纯函数，不修改入参", () => {
  const input = base();
  const snapshot = JSON.stringify(input);
  moveWidget(input, { key: "stats", index: 0 }, { key: "others", index: 0 });
  assert.equal(JSON.stringify(input), snapshot, "入参未被修改");
});

test("moveWidget: 移动只影响相关区域，其余区域不变", () => {
  const input = base();
  const r = moveWidget(input, { key: "stats", index: 0 }, { key: "stats", index: 1 });
  assert.deepEqual(r.chartNarrow, input.chartNarrow, "chartNarrow 不变");
  assert.deepEqual(r.recents, input.recents, "recents 不变");
  assert.deepEqual(r.others, input.others, "others 不变");
  assert.equal(r.chartWide?.type, input.chartWide?.type, "chartWide 不变");
});
