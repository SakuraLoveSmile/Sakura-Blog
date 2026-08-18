import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../web/src");
const WIDGET = readFileSync(join(ROOT, "modules/dashboard/widgets/StellarVersionWidget.vue"), "utf8");
const API = readFileSync(join(ROOT, "api/index.ts"), "utf8");

test("Stellar version widget: API and four visible states are wired", () => {
  assert.match(API, /stellarVersion:\s*\(\)\s*=>\s*http\.get<StellarVersionResponse>\("\/dashboard\/stellar-version"\)/);
  for (const marker of ["loading", "synced", "outdated", "unavailable"]) {
    assert.match(WIDGET, new RegExp(marker));
  }
  assert.match(WIDGET, /local_version/);
  assert.match(WIDGET, /upstream_version/);
  assert.match(WIDGET, /release_url/);
});

test("Stellar version widget: dark UI uses semantic tokens", () => {
  assert.match(WIDGET, /var\(--color-state-success\)/);
  assert.match(WIDGET, /var\(--color-state-warning\)/);
  assert.match(WIDGET, /var\(--color-muted-foreground\)/);
  assert.doesNotMatch(WIDGET, /#[0-9A-Fa-f]{6}/);
});
