import { test } from "node:test";
import assert from "node:assert/strict";
import { CanceledError } from "../../../packages/api-client/node_modules/axios/index.js";
import { QueryClient } from "../web/node_modules/@tanstack/vue-query/build/modern/index.js";
import { ucNotificationApi } from "../../../packages/api-client/src/api/monitoring.ts";
import { previewApi } from "../../../packages/api-client/src/api/editor.ts";
import { configureApiTransport, getApiErrorMessage, http } from "../../../packages/api-client/src/transport.ts";

function failingAdapter(status: number, data: unknown) {
  return async (config: any) => {
    throw { config, response: { data, status, statusText: "", headers: {}, config } };
  };
}

test("transport: backend error envelope is normalized for the root toast", async () => {
  const errors: string[] = [];
  configureApiTransport({ onError: (_error, message) => errors.push(message) });
  const originalAdapter = http.defaults.adapter;
  http.defaults.adapter = failingAdapter(422, { error: "标题已存在" }) as any;

  try {
    await assert.rejects(http.get("/posts"));
    assert.deepEqual(errors, ["标题已存在"]);
    assert.equal(getApiErrorMessage({ response: { data: { detail: "详细错误" } } }), "详细错误");
  } finally {
    http.defaults.adapter = originalAdapter;
    configureApiTransport({ onError: undefined, onUnauthorized: undefined });
  }
});

test("transport: mute suppresses global error toast for background requests", async () => {
  let calls = 0;
  configureApiTransport({ onError: () => calls++ });
  const originalAdapter = http.defaults.adapter;
  http.defaults.adapter = failingAdapter(503, { error: "服务暂不可用" }) as any;

  try {
    await assert.rejects(http.get("/notifications/unread-count", { mute: true }));
    assert.equal(calls, 0);
  } finally {
    http.defaults.adapter = originalAdapter;
    configureApiTransport({ onError: undefined, onUnauthorized: undefined });
  }
});

test("transport: 401 is delegated to the host once per response", async () => {
  let calls = 0;
  configureApiTransport({ onUnauthorized: () => calls++ });
  const originalAdapter = http.defaults.adapter;
  http.defaults.adapter = failingAdapter(401, { error: "未登录" }) as any;

  try {
    await assert.rejects(http.get("/posts"));
    assert.equal(calls, 1);
  } finally {
    http.defaults.adapter = originalAdapter;
    configureApiTransport({ onError: undefined, onUnauthorized: undefined });
  }
});

test("preview: failed inline tags reject locally without stacking global toasts", async () => {
  const errors: string[] = [];
  configureApiTransport({ onError: (_error, message) => errors.push(message) });
  const originalAdapter = http.defaults.adapter;
  http.defaults.adapter = failingAdapter(422, { error: "标签语法错误" }) as any;

  try {
    const results = await Promise.allSettled(
      Array.from({ length: 24 }, () => previewApi.render("{% copy example %}", undefined, { mode: "content" })),
    );
    assert.ok(results.every((result) => result.status === "rejected"));
    assert.deepEqual(errors, []);

    await assert.rejects(previewApi.render("document", undefined, { mode: "document" }));
    await assert.rejects(previewApi.render("default document"));
    assert.deepEqual(errors, ["标签语法错误", "标签语法错误"]);
  } finally {
    http.defaults.adapter = originalAdapter;
    configureApiTransport({ onError: undefined, onUnauthorized: undefined });
  }
});

test("preview: inline errors still delegate expired authentication to the host", async () => {
  let errors = 0,
    expired = 0;
  configureApiTransport({ onError: () => errors++, onUnauthorized: () => expired++ });
  const originalAdapter = http.defaults.adapter;
  http.defaults.adapter = failingAdapter(401, { error: "未登录" }) as any;

  try {
    await assert.rejects(previewApi.render("{% copy example %}", undefined, { mode: "content" }));
    assert.equal(expired, 1);
    assert.equal(errors, 0);
  } finally {
    http.defaults.adapter = originalAdapter;
    configureApiTransport({ onError: undefined, onUnauthorized: undefined });
  }
});

test("transport: cancelled requests do not toast or expire the current identity", async () => {
  let errors = 0,
    expired = 0;
  configureApiTransport({ onError: () => errors++, onUnauthorized: () => expired++ });
  const originalAdapter = http.defaults.adapter;
  http.defaults.adapter = (async (config: any) => {
    throw new CanceledError("cancelled", config);
  }) as any;
  try {
    await assert.rejects(http.get("/uc/profile"));
    assert.equal(errors, 0);
    assert.equal(expired, 0);
  } finally {
    http.defaults.adapter = originalAdapter;
    configureApiTransport({ onError: undefined, onUnauthorized: undefined });
  }
});

test("UC notifications: cancelled list and writes cannot expire a new session on late 401", async () => {
  let errors = 0,
    expired = 0;
  configureApiTransport({ onError: () => errors++, onUnauthorized: () => expired++ });
  const originalAdapter = http.defaults.adapter;
  const client = new QueryClient();
  let release!: () => void;
  let capturedSignal: AbortSignal | undefined;
  http.defaults.adapter = (async (config: any) => {
    capturedSignal = config.signal;
    await new Promise<void>((resolve) => {
      release = resolve;
    });
    throw { config, response: { data: { error: "旧身份已过期" }, status: 401, config } };
  }) as any;
  try {
    const pending = client
      .fetchQuery({
        queryKey: ["uc-notifications", 1],
        queryFn: ({ signal }) => ucNotificationApi.list({}, signal),
      })
      .catch(() => {});
    await Promise.resolve();
    await Promise.resolve();
    assert.ok(capturedSignal);
    client.clear();
    assert.equal(capturedSignal.aborted, true);
    release();
    await pending;
    for (const write of [
      (signal: AbortSignal) => ucNotificationApi.markRead(1, signal),
      (signal: AbortSignal) => ucNotificationApi.markAllRead(signal),
    ]) {
      const controller = new AbortController();
      const request = write(controller.signal);
      await Promise.resolve();
      await Promise.resolve();
      assert.equal(capturedSignal, controller.signal);
      controller.abort();
      release();
      await assert.rejects(request);
    }
    assert.equal(expired, 0);
    assert.equal(errors, 0);
  } finally {
    client.clear();
    http.defaults.adapter = originalAdapter;
    configureApiTransport({ onError: undefined, onUnauthorized: undefined });
  }
});
