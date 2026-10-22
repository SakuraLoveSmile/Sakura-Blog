import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../web/src");
const read = (path: string) => readFileSync(join(ROOT, path), "utf8");

test("WikiProjectsView keeps Hero controls wired to strict Stellar 2 fields and all four effects", () => {
  const view = read("modules/wiki/WikiProjectsView.vue");
  const fields = read("components/editor/stellar-form-fields.ts");
  const json = read("components/editor/stellar-form-json.ts");
  assert.match(view, /useStellarForm\(\(\) => form.value, wikiFields\)/);
  assert.match(view, /animationParamsText/);
  for (const effect of ["strands", "galaxy", "light-rays", "ferrofluid"]) assert.ok(view.includes(`value="${effect}"`));
  assert.match(fields, /path: "hero.background.effect.options", raw: true/);
  assert.match(json, /if \(rawValue !== undefined\) JSON.parse\(rawValue\)/);
  assert.match(fields, /path: "hero.enabled"/);
});

test("Console 设置页暴露 Card Hover 配置并保持安全边界", () => {
  const settings = read("modules/settings/SettingsView.vue");
  const plugins = read("modules/settings/PluginsTab.vue");
  assert.match(settings, /card_hover\.enable = .*false/);
  assert.match(settings, /card_hover\.max_tilt = .*3/);
  assert.match(plugins, /卡片 Hover 光斑与倾斜/);
  assert.match(plugins, /min="0" max="8"/);
});
