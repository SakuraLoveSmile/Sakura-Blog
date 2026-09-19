// 行为面：Mock Fixtures 数据定义与预览注入测试（Spec 63 + Spec 64）
import { test } from "node:test";
import assert from "node:assert/strict";
import { FIXTURE_ITEMS } from "../web/src/modules/plugins/fixtures/fixture-definitions.ts";
import { validateFixtureData } from "../web/src/modules/plugins/fixtures/fixture-validation.ts";
import { parseFixtureImport } from "../web/src/modules/plugins/fixtures/fixture-import.ts";
import { buildPreviewDoc } from "../web/src/modules/plugins/preview/preview-document.ts";

const SURFACE = "Mock Fixtures";
const persistedStorage = new Map<string, string>();
Object.defineProperty(globalThis, "localStorage", {
  configurable: true,
  value: {
    getItem: (key: string) => persistedStorage.get(key) ?? null,
    setItem: (key: string, value: string) => persistedStorage.set(key, value),
  },
});
const { useFixtureStore } = await import("../web/src/modules/plugins/fixtures/useFixtureStore.ts");

test(`${SURFACE} · Fixture 清单定义完整且端点唯一`, () => {
  assert.ok(FIXTURE_ITEMS.length >= 5, "至少包含 5 种标准 Fixture");
  const ids = new Set<string>();
  const endpoints = new Set<string>();

  for (const item of FIXTURE_ITEMS) {
    assert.ok(item.id, "必须包含 id");
    assert.ok(item.title, "必须包含 title");
    assert.ok(item.endpoint.startsWith("/api/v1/preview/"), "端点必须为 /api/v1/preview/ 前缀");
    assert.ok(item.format === "json" || item.format === "markdown", "格式必须为 json 或 markdown");
    assert.ok(item.previewShortcode.startsWith("{%"), "必须包含关联预览短代码");
    assert.ok(item.previewHeight > 0, "必须声明推荐预览高度");
    assert.ok(item.emptyData !== undefined, "必须声明空数据");
    assert.ok(item.schemaHint === undefined || item.schemaHint.length > 0, "schemaHint 非空时必须有内容");

    if (item.format === "json") {
      assert.equal(validateFixtureData(item, item.defaultData).valid, true, `Fixture ${item.id} 默认数据必须通过校验`);
      assert.equal(validateFixtureData(item, item.emptyData).valid, true, `Fixture ${item.id} 空数据必须通过校验`);
    } else {
      assert.ok(item.defaultData.length > 0, `Fixture ${item.id} 默认内容非空`);
      assert.equal(
        validateFixtureData(item, item.defaultData).valid,
        true,
        `Fixture ${item.id} 默认 Markdown 必须合法`,
      );
      assert.equal(validateFixtureData(item, item.emptyData).valid, true, `Fixture ${item.id} 空 Markdown 必须合法`);
    }

    assert.ok(!ids.has(item.id), `Fixture ID ${item.id} 不能重复`);
    assert.ok(!endpoints.has(item.endpoint), `Fixture Endpoint ${item.endpoint} 不能重复`);
    ids.add(item.id);
    endpoints.add(item.endpoint);
  }
});

test(`${SURFACE} · 修改状态包含非 normal 响应状态`, () => {
  const store = useFixtureStore();
  store.resetAll();

  assert.equal(store.isModified("rating"), false);
  store.updateState("rating", "error");
  assert.equal(store.isModified("rating"), true);
  assert.equal(store.customCount.value, 1);
  assert.equal(store.isCustom("rating"), false);
  assert.equal(store.overrideMap.value.rating.emptyData, '{\n  "avg": 0,\n  "count": 0\n}');

  store.resetFixture("rating");
  assert.equal(store.isModified("rating"), false);
  assert.equal(store.customCount.value, 0);
});

test(`${SURFACE} · Fixture validator 覆盖默认、空数据与非法结构`, () => {
  const fixture = (id: string) => FIXTURE_ITEMS.find((item) => item.id === id)!;
  const validCases: Array<[string, string]> = FIXTURE_ITEMS.map((item) => [item.id, item.defaultData]);
  for (const [id, raw] of validCases) {
    assert.equal(validateFixtureData(fixture(id), raw).valid, true, `${id} 默认数据应合法`);
  }

  assert.equal(validateFixtureData(fixture("friends"), '{"content":{}}').valid, false);
  assert.match(
    validateFixtureData(fixture("friends"), '{"content":[{"title":1,"url":"https://example.com"}]}').errors[0],
    /title.*string/,
  );
  assert.equal(validateFixtureData(fixture("sites"), '{"content":{}}').valid, false);
  assert.equal(validateFixtureData(fixture("rating"), '{"avg":"4.6","count":128}').valid, false);
  assert.equal(validateFixtureData(fixture("rating"), '{"avg":4.6,"count":[]}').valid, false);
  assert.equal(validateFixtureData(fixture("rating"), '{"avg":4.6,"count":-1}').valid, false);
  assert.equal(validateFixtureData(fixture("vote"), '{"votes":{"up":"10","down":2}}').valid, false);
  assert.equal(validateFixtureData(fixture("vote"), '{"votes":{"up":10,"down":-1}}').valid, false);
  assert.equal(validateFixtureData(fixture("rating"), '{"avg":').valid, false);
  assert.match(validateFixtureData(fixture("rating"), '{"avg":').errors[0], /JSON syntax error/);
  const parseJson = JSON.parse;
  JSON.parse = () => {
    throw new Error("Markdown should not be parsed as JSON");
  };
  try {
    assert.equal(validateFixtureData(fixture("md"), "not JSON at all").valid, true);
  } finally {
    JSON.parse = parseJson;
  }
});

test(`${SURFACE} · Fixture Import 完整校验并支持合法 round trip`, () => {
  const raw = JSON.stringify({
    version: 1,
    exportedAt: "not required",
    customData: { friends: FIXTURE_ITEMS[0].defaultData },
    states: { rating: "empty" },
  });
  const result = parseFixtureImport(raw, FIXTURE_ITEMS);
  assert.equal(result.success, true);
  if (result.success) {
    assert.equal(result.state.customData.friends, FIXTURE_ITEMS[0].defaultData);
    assert.equal(result.state.states.rating, "empty");
  }
});

test(`${SURFACE} · Fixture Import 拒绝版本、ID、类型、结构、状态与 JSON 错误`, () => {
  const base = {
    version: 1,
    customData: {},
    states: {},
  };
  const parse = (value: unknown) => parseFixtureImport(JSON.stringify(value), FIXTURE_ITEMS);
  assert.equal(parse({ ...base, version: 2 }).success, false);
  assert.equal(parse({ ...base, customData: { unknown: "{}" } }).success, false);
  assert.equal(parse({ ...base, customData: { rating: 4 } }).success, false);
  assert.equal(parse({ ...base, customData: { rating: '{"avg":"4.6","count":128}' } }).success, false);
  assert.equal(parse({ ...base, states: { rating: "loading" } }).success, false);
  assert.equal(parse("not json").success, false);
  assert.equal(parse({ ...base, customData: [] }).success, false);
});

test(`${SURFACE} · 非法 Import 不会部分覆盖 Store 或 localStorage`, () => {
  const store = useFixtureStore();
  store.resetAll();
  store.updateData("friends", FIXTURE_ITEMS[0].defaultData);
  store.updateState("friends", "error");
  const before = store.overrideMap.value.friends;
  const beforeStorage = persistedStorage.get("blog:console:mock_fixtures_v1");

  const result = store.importJson(
    JSON.stringify({
      version: 1,
      customData: {
        friends: FIXTURE_ITEMS[0].defaultData,
        rating: '{"avg":"invalid","count":1}',
      },
      states: { friends: "normal" },
    }),
  );

  assert.equal(result.success, false);
  assert.deepEqual(store.overrideMap.value.friends, before);
  assert.equal(persistedStorage.get("blog:console:mock_fixtures_v1"), beforeStorage);
});

test(`${SURFACE} · buildPreviewDoc 注入自定义 mockFixtures 拦截脚本`, () => {
  const doc = buildPreviewDoc({
    bodyHtml: '<div class="data-service ds-friends" data-api="/api/v1/preview/friends"></div>',
    mockFixtures: {
      friends: {
        endpoint: "/api/v1/preview/friends",
        format: "json",
        state: "normal",
        customData: '{"content":[{"title":"自定义好友","url":"https://custom.com"}]}',
        emptyData: '{"content":[]}',
      },
    },
  });

  assert.ok(doc.includes("window.__MOCK_FIXTURES__"), "必须注入 window.__MOCK_FIXTURES__");
  assert.ok(doc.includes("自定义好友"), "必须包含传入的自定义数据");
  assert.ok(doc.includes("simulated fixture error"), "必须包含 fetch 异常模拟逻辑");
});

test(`${SURFACE} · Preview Runtime 使用 Fixture 自己的 emptyData 并精确匹配端点`, () => {
  const doc = buildPreviewDoc({
    bodyHtml: '<div data-api="/api/v1/preview/friends"></div>',
    mockFixtures: {
      friends: {
        endpoint: "/api/v1/preview/friends",
        format: "json",
        state: "empty",
        emptyData: '{"content":[]}',
      },
    },
  });
  assert.ok(doc.includes('"emptyData":"{\\"content\\":[]}"'), "必须注入 Fixture 声明的空数据");
  assert.ok(doc.includes("var emptyText = f.emptyData != null ? f.emptyData : ''"));
  assert.ok(doc.includes("path === f.endpoint"));
  assert.ok(doc.includes("split(/[?#]/)[0]"), "srcdoc URL 异常时也必须去掉 query/hash");
  assert.ok(!doc.includes("key === 'rating'") && !doc.includes("key === 'vote'"));
});

test(`${SURFACE} · buildPreviewDoc 模拟 error 状态正确`, () => {
  const errorDoc = buildPreviewDoc({
    bodyHtml: "<div>test</div>",
    mockFixtures: {
      rating: {
        endpoint: "/api/v1/preview/rating",
        format: "json",
        state: "error",
      },
    },
  });
  assert.ok(errorDoc.includes('"state":"error"'), "必须正确序列化 error 状态");
  assert.ok(errorDoc.includes("status: 500"), "error Fixture 必须模拟 HTTP 500");
});
