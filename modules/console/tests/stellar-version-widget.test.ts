import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../web/src");
const API_ROOT = join(dirname(fileURLToPath(import.meta.url)), "../../../packages/api-client/src");
const WIDGET = readFileSync(join(ROOT, "modules/dashboard/widgets/StellarVersionWidget.vue"), "utf8");
const DASHBOARD_CSS = readFileSync(join(ROOT, "modules/dashboard/dashboard.css"), "utf8");
const API = readFileSync(join(API_ROOT, "api/dashboard.ts"), "utf8");

test("Stellar version widget: API and four visible states are wired", () => {
  assert.match(API, /stellarVersion:\s*\(\)\s*=>\s*http\.get<StellarVersionResponse>\("\/dashboard\/stellar-version"\)/);
  for (const marker of ["loading", "synced", "outdated", "unavailable"]) {
    assert.match(WIDGET, new RegExp(marker));
  }
  assert.match(WIDGET, /local_version/);
  assert.match(WIDGET, /upstream_version/);
  assert.match(WIDGET, /release_url/);
});

test("Stellar version widget: dark UI uses semantic tokens（共享 .version-card-* 定义在 dashboard.css）", () => {
  // 版本卡样式已提取为 dashboard.css 共享类（Spec：两组件 scoped 去重）
  assert.match(DASHBOARD_CSS, /\.version-card-state\.is-synced[\s\S]*var\(--color-state-success\)/);
  assert.match(DASHBOARD_CSS, /\.version-card-state\.is-outdated[\s\S]*var\(--color-state-warning\)/);
  assert.match(DASHBOARD_CSS, /var\(--color-muted-foreground\)/);
  assert.match(WIDGET, /version-card-/);
  assert.doesNotMatch(WIDGET, /#[0-9A-Fa-f]{6}/);
});
