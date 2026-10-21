import { test } from "node:test";
import assert from "node:assert/strict";
import { http } from "../../../packages/api-client/src/transport.ts";
import { backupApi } from "../../../packages/api-client/src/api/settings.ts";

test("确认忽略参数时重传原文件，并保留已确认的专栏与精确字段位置", async () => {
  const source = "---\ntitle: 导入验证\npassword: fixture-only\n---\n原始正文。\n";
  const file = new File([source], "导入验证.md", { type: "text/markdown" });
  const originalAdapter = http.defaults.adapter;
  const forms: FormData[] = [];
  http.defaults.adapter = async (config) => {
    assert.equal(config.url, "/backup/import/markdown");
    assert.equal(config.method, "post");
    assert.equal(config.mute, true);
    assert.ok(config.data instanceof FormData);
    forms.push(config.data);
    return { config, status: 200, statusText: "OK", headers: {}, data: { imported: 1, slugs: [], skipped: [] } };
  };
  try {
    await backupApi.importMarkdown(file, true);
    await backupApi.importMarkdown(
      file,
      true,
      ["missing-topic"],
      [
        { source: file.name, path: "/password" },
        { source: file.name, path: "/custom~1field" },
      ],
    );
    assert.equal(forms[0].get("ignored_parameters"), null);
    assert.equal(forms[0].get("create_topics"), null);
    assert.equal(forms[1].get("file"), file);
    assert.equal(await (forms[1].get("file") as File).text(), source);
    assert.equal(forms[1].get("legacy"), "true");
    assert.deepEqual(JSON.parse(String(forms[1].get("create_topics"))), ["missing-topic"]);
    assert.deepEqual(JSON.parse(String(forms[1].get("ignored_parameters"))), [
      { source: file.name, path: "/password" },
      { source: file.name, path: "/custom~1field" },
    ]);
  } finally {
    http.defaults.adapter = originalAdapter;
  }
});
