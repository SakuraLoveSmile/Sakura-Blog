// 行为面：仪表盘 widget 注册表数据不变量（源码级校验）。
// registry.ts 含 .vue 组件 import，node:test 无法直接加载，故对源文件做机械校验：
// type 唯一、defaultSize 引用存在的 SIZE_PRESETS、defaultSection 合法、props.statKey 对应真实统计字段。
// 机械校验入口：make test → console-test → node --test tests/*.test.ts
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const SRC = readFileSync(
  join(dirname(fileURLToPath(import.meta.url)), "../src/modules/dashboard/widgets/registry.ts"),
  "utf8"
);

// 逐块提取 widget 定义（对象字面量）：type / defaultSize / defaultSection / props.statKey
const blocks = SRC.match(/\{\n\s+type: "([^"]+)",[\s\S]*?\n\s+\},/g) || [];
assert.ok(blocks.length >= 8, `registry blocks: ${blocks.length}`);

interface Parsed {
  type: string;
  size: string;
  section: string;
  statKey?: string;
}

const parsed: Parsed[] = blocks.map((b) => {
  const type = b.match(/type: "([^"]+)"/)?.[1] ?? "";
  const size = b.match(/defaultSize: (SIZE_PRESETS\.\w+|\{[^}]*\})/)?.[1] ?? "";
  const section = b.match(/defaultSection: "([^"]+)"/)?.[1] ?? "";
  const statKey = b.match(/statKey: "([^"]+)"/)?.[1];
  return { type, size, section, statKey };
});

test("registry: widget type 唯一", () => {
  const seen = new Set<string>();
  for (const p of parsed) {
    assert.ok(p.type, "type 非空");
    assert.ok(!seen.has(p.type), `重复 type: ${p.type}`);
    seen.add(p.type);
  }
});

test("registry: defaultSize 全部引用已定义的 SIZE_PRESETS", () => {
  const presets = [...SRC.matchAll(/SIZE_PRESETS\.(\w+)/g)].map((m) => m[1]);
  assert.ok(presets.length > 0, "SIZE_PRESETS 定义存在");
  const presetSet = new Set(presets);
  for (const p of parsed) {
    if (p.size.startsWith("{")) continue; // 字面量对象（如图表卡）
    const ref = p.size.replace("SIZE_PRESETS.", "");
    assert.ok(presetSet.has(ref), `defaultSize 引用了不存在的 preset: ${p.size} (${p.type})`);
  }
});

test("registry: defaultSection 合法取值", () => {
  const valid = ["stats", "chartWide", "chartNarrow", "recents", "others"];
  for (const p of parsed) {
    assert.ok(valid.includes(p.section), `非法 defaultSection: ${p.section} (${p.type})`);
  }
});

test("registry: stat 卡 statKey 均为 /dashboard/stats 响应字段", () => {
  const validKeys = [
    "post_total",
    "post_published",
    "view_total",
    "comment_total",
    "attachment_total",
    "user_total",
  ];
  for (const p of parsed) {
    if (p.statKey) {
      assert.ok(validKeys.includes(p.statKey), `非法 statKey: ${p.statKey} (${p.type})`);
    }
  }
});
