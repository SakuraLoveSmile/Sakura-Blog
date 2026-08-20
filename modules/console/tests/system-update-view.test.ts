import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../web/src");
const VIEW = readFileSync(join(ROOT, "modules/system/SystemUpdateView.vue"), "utf8");
const API = readFileSync(join(ROOT, "api/index.ts"), "utf8");

test("SystemUpdateView: poll timer lifecycle is owned (no leaked/const interval)", () => {
  // 防止回退为 const null / 丢弃 setInterval 返回值——那会导致 Toast 每 3 秒无限弹、离开页面后仍请求泄漏。
  assert.match(VIEW, /let pollTimer/);
  assert.match(VIEW, /pollTimer = setInterval/);
  assert.match(VIEW, /clearInterval\(pollTimer\)/);
  assert.match(VIEW, /pollTimer = null/);
});

test("SystemUpdateView: error 语义分离后消费 apply_note", () => {
  // 不支持一键更新的原因只走 apply_note，不再污染 error。
  assert.match(VIEW, /checkData\.apply_note/);
});

test("SystemUpdateView: 版本三态文案存在（含「无法确定」）", () => {
  // current=unknown 或 latest 缺失时归「无法确定」，不再误显「已是最新版本」。
  for (const marker of ["发现新版本", "已是最新版本", "无法确定是否有更新"]) {
    assert.match(VIEW, new RegExp(marker));
  }
});

test("api/index.ts: UpdateCheckResponse 含 apply_note 字段", () => {
  assert.match(API, /apply_note\?:\s*string/);
});
