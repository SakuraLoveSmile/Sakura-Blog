import { test } from "node:test";
import assert from "node:assert/strict";
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
