import { test } from "node:test";
import assert from "node:assert/strict";
import { http } from "../../../packages/api-client/src/transport.ts";
import {
  readStellarSettingsProblem,
  stellarSettingsApi,
} from "../../../packages/api-client/src/api/stellar_settings.ts";

test("Stellar settings transports raw JSON without numeric or parameter-bag conversion", async () => {
  const document =
    '{"version":2,"theme":{"comments":{"giscus":{"large":9007199254740993,"flag":false,"nil":null,"zero":0}}},"blog_extensions":{}}';
  const originalAdapter = http.defaults.adapter;
  const calls: Array<{ method: string | undefined; url: string | undefined; data: unknown }> = [];
  http.defaults.adapter = async (config) => {
    calls.push({ method: config.method, url: config.url, data: config.data });
    if (config.method !== "get") assert.equal(config.headers.get("Content-Type"), "application/json");
    return {
      config,
      status: 200,
      statusText: "OK",
      headers: {},
      data: config.url?.endsWith("/validate") ? '{"valid":true,"issues":[]}' : document,
    };
  };
  try {
    assert.equal((await stellarSettingsApi.get()).data, document);
    assert.equal((await stellarSettingsApi.save(document)).data, document);
    assert.deepEqual((await stellarSettingsApi.validate(document)).data, { valid: true, issues: [] });
    assert.deepEqual(calls, [
      { method: "get", url: "/settings/stellar", data: undefined },
      { method: "put", url: "/settings/stellar", data: document },
      { method: "post", url: "/settings/stellar/validate", data: document },
    ]);
  } finally {
    http.defaults.adapter = originalAdapter;
  }
});

test("Stellar settings decodes named diagnostics from raw and parsed error responses", () => {
  const problem = {
    type: "stellar_migration_required",
    title: "需要迁移",
    status: 409,
    detail: "先预览迁移",
    error: "先预览迁移",
    instance: "/api/v1/settings/stellar",
    issues: [],
    migration: { source: "settings", path: "stellar_v2", reason: "missing", sections: ["style"] },
  };
  assert.deepEqual(readStellarSettingsProblem({ response: { data: JSON.stringify(problem) } }), problem);
  assert.deepEqual(readStellarSettingsProblem({ response: { data: problem } }), problem);
  assert.equal(readStellarSettingsProblem({ response: { data: "unavailable" } }), undefined);
  assert.equal(readStellarSettingsProblem(undefined), undefined);
});
