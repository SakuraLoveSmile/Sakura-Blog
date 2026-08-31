// 行为面：Mock Fixtures 数据定义与预览注入测试（Spec 63 + Spec 64）
import { test } from "node:test";
import assert from "node:assert/strict";
import { FIXTURE_ITEMS } from "../web/src/modules/plugins/fixtures/fixture-definitions.ts";
import { buildPreviewDoc } from "../web/src/modules/plugins/preview/preview-document.ts";

const SURFACE = "Mock Fixtures";

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

    if (item.format === "json") {
      assert.doesNotThrow(() => JSON.parse(item.defaultData), `Fixture ${item.id} 默认 JSON 必须合法`);
    } else {
      assert.ok(item.defaultData.length > 0, `Fixture ${item.id} 默认内容非空`);
    }

    assert.ok(!ids.has(item.id), `Fixture ID ${item.id} 不能重复`);
    assert.ok(!endpoints.has(item.endpoint), `Fixture Endpoint ${item.endpoint} 不能重复`);
    ids.add(item.id);
    endpoints.add(item.endpoint);
  }
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
      },
    },
  });

  assert.ok(doc.includes("window.__MOCK_FIXTURES__"), "必须注入 window.__MOCK_FIXTURES__");
  assert.ok(doc.includes("自定义好友"), "必须包含传入的自定义数据");
  assert.ok(doc.includes("simulated fixture error"), "必须包含 fetch 异常模拟逻辑");
});

test(`${SURFACE} · buildPreviewDoc 模拟 empty / error 状态正确`, () => {
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
});
