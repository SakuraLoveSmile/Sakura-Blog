import { test } from "node:test";
import assert from "node:assert/strict";
import { mergePluginCatalog, type PluginCatalogItem } from "../web/src/modules/plugins/plugin-catalog.ts";
import type { PluginMeta } from "../web/src/modules/plugins/plugin-types.ts";

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
    title: "后端 Note",
    description: "后端描述",
    signature: "{% note ... %}",
    example: "{% note %}",
    type: "inline",
    category: "提示标注",
    capability: "static",
    param_docs: [{ name: "color", desc: "配色", default: "blue" }],
    params: [
      { key: "enabled", type: "switch", role: "flag", default: true },
      { key: "limit", type: "number", role: "named", default: 3, options: ["1", "3"] },
    ],
  },
  {
    name: "table",
    title: "表格",
    description: "表格描述",
    signature: "{% table %}",
    example: "{% table %}\n{% endtable %}",
    type: "block",
    category: "布局容器",
    capability: "static",
    param_docs: [],
    params: [{ key: "content", type: "textarea", role: "content" }],
  },
];

test("catalog merge uses backend order and preserves display metadata", () => {
  const merged = mergePluginCatalog([basePlugin], catalog);
  assert.deepEqual(merged.map((plugin) => plugin.name), ["note", "table"]);
  assert.equal(merged[0].title, "后端 Note");
  assert.equal(merged[0].desc, "后端描述");
  assert.equal(merged[0].signature, "{% note ... %}");
  assert.equal(merged[0].example, "{% note %}");
  assert.deepEqual(merged[0].params, [{ name: "color", desc: "配色", default: "blue" }]);
  assert.equal(merged[0].form?.[0].label, "启用");
  assert.equal(merged[0].form?.[0].type, "flag");
  assert.equal(merged[0].form?.[0].default, "true");
  assert.equal(merged[0].form?.[1].type, "number");
  assert.deepEqual(merged[0].form?.[1].options, ["1", "3"]);
  assert.equal(merged[1].type, "block");
  assert.equal(merged[1].title, "表格");
});

test("empty catalog does not invent entries", () => {
  const merged = mergePluginCatalog([basePlugin], []);
  assert.deepEqual(merged, []);
});
