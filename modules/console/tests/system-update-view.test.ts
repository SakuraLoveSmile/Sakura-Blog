import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../web/src");
const API_ROOT = join(dirname(fileURLToPath(import.meta.url)), "../../../packages/api-client/src");
const VIEW = readFileSync(join(ROOT, "modules/system/SystemUpdateView.vue"), "utf8");
const API = readFileSync(join(API_ROOT, "api/monitoring.ts"), "utf8");

test("SystemUpdateView: 一键更新链路已移除（无轮询 / apply / status 调用）", () => {
  // v1.7.3 起页面只负责检查更新，不再触发或跟踪执行。
  assert.doesNotMatch(VIEW, /pollTimer|setInterval|applyUpdate|systemApi\.apply|systemApi\.status/);
  assert.doesNotMatch(VIEW, /UpdateStatusResponse/);
});

test("SystemUpdateView: 版本三态文案存在（含「无法确定」）", () => {
  // current=unknown 或 latest 缺失时归「无法确定」，不再误显「已是最新版本」。
  for (const marker of ["发现新版本", "已是最新版本", "无法确定是否有更新"]) {
    assert.match(VIEW, new RegExp(marker));
  }
});

test("SystemUpdateView: 新版本横幅给出手动升级指引", () => {
  // 替代原「立即一键升级」按钮：告知 docker pull / update.sh 两种手动路径。
  assert.match(VIEW, /docker compose pull/);
  assert.match(VIEW, /update\.sh/);
});

test("api/monitoring.ts: systemApi 仅保留 version 与 check", () => {
  assert.match(API, /systemApi/);
  assert.doesNotMatch(API, /apply:\s*\(|status:\s*\(\)|UpdateApplyResponse|UpdateStatusResponse/);
});
