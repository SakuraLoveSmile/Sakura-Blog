// 行为面：统一网格下 widget 注册表只负责组件身份，不再绑定宽图/窄图区域。
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const SRC = readFileSync(
  join(dirname(fileURLToPath(import.meta.url)), "../web/src/modules/dashboard/widgets/registry.ts"),
  "utf8",
);

const blocks = SRC.match(/defineWidget\(\{[\s\S]*?\n\s+\}\),/g) || [];

test("registry: widget id 唯一且组件定义完整", () => {
  assert.ok(blocks.length >= 8, `registry blocks: ${blocks.length}`);
  const seen = new Set<string>();
  for (const block of blocks) {
    const id = block.match(/id: "([^"]+)"/)?.[1] ?? "";
    assert.ok(id, "id 非空");
    assert.ok(!seen.has(id), `重复 id: ${id}`);
    assert.ok(block.includes("component:"), `${id} 缺少 component`);
    assert.ok(block.includes("props:"), `${id} 缺少 props`);
    assert.ok(block.includes("defaultSize:"), `${id} 缺少 defaultSize`);
    seen.add(id);
  }
});

test("registry: 不再把组件绑定到 chartWide/chartNarrow 固定区域", () => {
  assert.doesNotMatch(SRC, /defaultSection/);
  assert.doesNotMatch(SRC, /chartWide|chartNarrow/);
});

test("registry: stat 卡 statKey 均为 /dashboard/stats 响应字段", () => {
  const validKeys = new Set([
    "post_total",
    "post_published",
    "view_total",
    "comment_total",
    "attachment_total",
    "user_total",
  ]);
  for (const match of SRC.matchAll(/statKey: "([^"]+)"/g)) {
    assert.ok(validKeys.has(match[1]), `非法 statKey: ${match[1]}`);
  }
});
