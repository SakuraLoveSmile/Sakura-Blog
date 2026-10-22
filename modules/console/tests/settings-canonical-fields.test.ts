import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import {
  canonicalSettingsGroups,
  canonicalSettingsPaths,
  effectiveSearchProvider,
  searchProviderChanges,
  type CanonicalSettingsTab,
} from "../web/src/modules/settings/settings-canonical-fields.ts";
import { patchJSONField, readJSONFieldRaw } from "../web/src/components/editor/stellar-form-json.ts";

const schema = JSON.parse(readFileSync(new URL("../../../internal/stellarv2/schema.json", import.meta.url), "utf8"));
const tabs = Object.keys(canonicalSettingsGroups) as CanonicalSettingsTab[];

test("AI 设置只编辑独立 Blog 扩展 blog_extensions.ai，开关默认关闭且与运行时缺省一致", () => {
  assert.deepEqual(canonicalSettingsPaths("ai"), [
    "blog_extensions.ai.base_url",
    "blog_extensions.ai.api_key",
    "blog_extensions.ai.model",
    "blog_extensions.ai.embedding_model",
    "blog_extensions.ai.summary.enabled",
    "blog_extensions.ai.summary.allow_user_api",
    "blog_extensions.ai.ask.enabled",
    "blog_extensions.ai.ask.allow_user_llm",
  ]);
  const fields = canonicalSettingsGroups.ai.flatMap((group) => group.fields);
  assert.equal(fields.find((field) => field.path === "blog_extensions.ai.api_key")?.kind, "password");
  for (const field of fields) if (field.kind === "switch") assert.equal(field.default, false, field.path);
});

test("普通设置字段遵守当前严格主题 schema，显示默认值与上游一致", () => {
  for (const tab of tabs) {
    for (const group of canonicalSettingsGroups[tab]) {
      for (const field of group.fields) {
        if (!field.path.startsWith("theme.")) continue;
        const keys = field.path.split(".").slice(1);
        let node = schema.theme;
        let value = schema.theme.default.value;
        for (const key of keys) {
          node = node?.properties?.[key];
          value = value?.[key];
        }
        assert.ok(node, `${field.path} 必须是当前主题字段`);
        if (field.default !== undefined) assert.deepEqual(field.default, value, `${field.path} 默认值`);
      }
    }
  }
});

test("标签脏标记包含所有表单路径与 Go 搜索覆盖，不包含停止使用的旧分段", () => {
  for (const tab of tabs) {
    const paths = canonicalSettingsPaths(tab);
    assert.equal(new Set(paths).size, paths.length);
    for (const group of canonicalSettingsGroups[tab])
      for (const field of group.fields) assert.ok(paths.includes(field.path));
    assert.ok(paths.every((path) => path.startsWith("theme.") || path.startsWith("blog_extensions.")));
  }
  assert.ok(canonicalSettingsPaths("search").includes("blog_extensions.search.provider"));
  assert.ok(!canonicalSettingsPaths("search").some((path) => /lazy_load|skip_search|\.path$/.test(path)));
  assert.deepEqual(canonicalSettingsPaths("advanced"), [
    "theme.inject.head_begin",
    "theme.inject.head_end",
    "theme.inject.body_begin",
    "theme.inject.body_end",
  ]);
});

test("组合搜索控件尊重 Go 覆盖及显式 null，切换官方服务只清除 provider 覆盖", () => {
  assert.equal(effectiveSearchProvider("algolia", "go"), "go");
  assert.equal(effectiveSearchProvider(null, undefined), "none");
  assert.equal(effectiveSearchProvider(undefined, undefined), "local");
  assert.equal(effectiveSearchProvider("future", undefined), "future");
  const raw =
    '{"version":2,"theme":{"search":{"provider":"algolia","algolia":{"indexId":9007199254740993,"optional":null}}},"blog_extensions":{"search":{"provider":"go","option":9007199254740995}}}';
  for (const provider of ["local", "algolia", "none", "go"]) {
    const next = searchProviderChanges(provider).reduce(
      (document, [path, value]) => patchJSONField(document, path, value),
      raw,
    );
    const parsed = JSON.parse(next);
    assert.equal(
      effectiveSearchProvider(parsed.theme.search.provider, parsed.blog_extensions.search.provider),
      provider,
    );
    assert.equal(readJSONFieldRaw(next, "theme.search.algolia"), '{"indexId":9007199254740993,"optional":null}');
    assert.equal(readJSONFieldRaw(next, "blog_extensions.search.option"), "9007199254740995");
  }
});
