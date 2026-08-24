import { test } from "node:test";
import assert from "node:assert/strict";
import { mergePluginCatalog, type PluginCatalogItem } from "../web/src/modules/plugins/plugin-catalog.ts";
import type { PluginMeta } from "../web/src/modules/plugins/plugins-data.ts";

const basePlugin: PluginMeta = {
  name: "note",
  title: "提示框 Note",
  category: "提示标注",
  type: "inline",
  desc: "旧展示文案",
  signature: "{% note ... %}",
  params: [],
  example: "{% note 示例 %}",
  form: [
    {
      key: "enabled",
      label: "启用",
      type: "flag",
      role: "flag",
      placeholder: "旧 placeholder",
    },
  ],
};

const catalog: PluginCatalogItem[] = [
  {
    name: "note",
    type: "inline",
    category: "提示标注",
    capability: "static",
    params: [
      { key: "enabled", type: "switch", role: "flag", default: true },
      { key: "limit", type: "number", role: "named", default: 3, options: ["1", "3"] },
    ],
  },
  {
    name: "table",
    type: "block",
    category: "布局容器",
    capability: "static",
    params: [{ key: "content", type: "textarea", role: "content" }],
  },
];

test("catalog merge uses backend order and preserves display metadata", () => {
  const merged = mergePluginCatalog([basePlugin], catalog);
  assert.deepEqual(merged.map((plugin) => plugin.name), ["note", "table"]);
  assert.equal(merged[0].title, "提示框 Note");
  assert.equal(merged[0].form?.[0].label, "启用");
  assert.equal(merged[0].form?.[0].type, "flag");
  assert.equal(merged[0].form?.[0].default, "true");
  assert.equal(merged[0].form?.[1].type, "number");
  assert.deepEqual(merged[0].form?.[1].options, ["1", "3"]);
  assert.equal(merged[1].type, "block");
  assert.equal(merged[1].title, "table");
});

test("empty catalog does not invent entries", () => {
  const merged = mergePluginCatalog([basePlugin], []);
  assert.deepEqual(merged, []);
});
