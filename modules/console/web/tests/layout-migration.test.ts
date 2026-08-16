// 行为面：仪表盘布局 v6→v7 迁移（src/modules/dashboard/widgets/layout.ts）。
// 机械校验入口：make test → console-test → node --test tests/*.test.ts
import { test } from "node:test";
import assert from "node:assert/strict";
import { migrateV6toV7 } from "../src/modules/dashboard/widgets/layout.ts";

const oldRecentSize = { l: 6, m: 6, s: 12 };
const newRecentSize = { l: 6, m: 12, s: 12 };

function v6Layout(recentItems: any[]): any {
  return {
    l: { stats: [{ type: "stat-posts", size: { l: 4, m: 4, s: 6 } }], recents: [] },
    m: { stats: [{ type: "stat-posts", size: { l: 4, m: 4, s: 6 } }], recents: recentItems },
    s: { stats: [{ type: "stat-posts", size: { l: 4, m: 4, s: 6 } }], recents: [] },
  };
}

test("migrateV6toV7: 旧默认尺寸的 recent-posts/recent-comments 替换为新默认", () => {
  const v6 = v6Layout([
    { type: "recent-posts", size: { ...oldRecentSize } },
    { type: "recent-comments", size: { ...oldRecentSize } },
  ]);
  const out = migrateV6toV7(v6);
  assert.ok(out, "should migrate");
  assert.deepEqual(out.m.recents[0].size, newRecentSize);
  assert.deepEqual(out.m.recents[1].size, newRecentSize);
});

test("migrateV6toV7: 用户自定义尺寸保留不动", () => {
  const custom = { l: 12, m: 12, s: 12 };
  const v6 = v6Layout([
    { type: "recent-posts", size: { ...custom } },
    { type: "recent-comments", size: { l: 6, m: 6, s: 6 } },
  ]);
  const out = migrateV6toV7(v6);
  assert.deepEqual(out.m.recents[0].size, custom, "自定义尺寸不得被覆盖");
  assert.deepEqual(out.m.recents[1].size, { l: 6, m: 6, s: 6 });
});

test("migrateV6toV7: 非 recents 部件与其它断点不受影响", () => {
  const v6 = v6Layout([]);
  const out = migrateV6toV7(v6);
  assert.deepEqual(out.l, v6.l, "l 断点原样保留");
  assert.deepEqual(out.s, v6.s, "s 断点原样保留");
});

test("migrateV6toV7: 缺段/损坏输入回退 null", () => {
  assert.equal(migrateV6toV7(null), null);
  assert.equal(migrateV6toV7(undefined), null);
  assert.equal(migrateV6toV7({ l: {}, m: {} }), null, "缺 s 段");
  assert.equal(migrateV6toV7("{broken json"), null, "非对象输入");
});

test("migrateV6toV7: 深拷贝——不修改输入对象", () => {
  const v6 = v6Layout([{ type: "recent-posts", size: { ...oldRecentSize } }]);
  const snapshot = JSON.stringify(v6);
  migrateV6toV7(v6);
  assert.equal(JSON.stringify(v6), snapshot, "输入对象不得被就地修改");
});
