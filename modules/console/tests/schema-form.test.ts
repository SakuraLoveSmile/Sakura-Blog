import { test } from "node:test";
import assert from "node:assert/strict";
import {
  colorInputValue,
  withSchemaDefaults,
  type SchemaField,
} from "../web/src/components/forms/schema.ts";

const SURFACE = "SchemaForm schema";

test(`${SURFACE} · 缺失值按字段类型填充且保留 false/0/空字符串`, () => {
  const fields: SchemaField[] = [
    { key: "title", type: "text", default: "默认标题" },
    { key: "limit", type: "number", default: 5 },
    { key: "enabled", type: "switch", default: true },
    { key: "empty", type: "text", default: "fallback" },
  ];

  assert.deepEqual(withSchemaDefaults(fields, { limit: 0, enabled: false, empty: "" }), {
    title: "默认标题",
    limit: 0,
    enabled: false,
    empty: "",
  });
});

test(`${SURFACE} · 保留 schema 未声明的扩展参数`, () => {
  const fields: SchemaField[] = [{ key: "title", type: "text", default: "标题" }];
  assert.deepEqual(withSchemaDefaults(fields, { custom: "value" }), {
    custom: "value",
    title: "标题",
  });
});

test(`${SURFACE} · color 输入规范化为原生控件可接受的六位 hex`, () => {
  assert.equal(colorInputValue("#abc"), "#aabbcc");
  assert.equal(colorInputValue("#12ABef"), "#12ABef");
  assert.equal(colorInputValue("red"), "#000000");
});
