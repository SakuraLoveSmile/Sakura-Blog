import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../web/src");
const API_ROOT = join(dirname(fileURLToPath(import.meta.url)), "../../../packages/api-client/src");
const read = (path: string) => readFileSync(join(ROOT, path), "utf8");

test("WikiProjectsView 编辑并校验 Stellar 1.44 Hero animation", () => {
  const view = read("modules/wiki/WikiProjectsView.vue");
  const api = readFileSync(join(API_ROOT, "api/wiki.ts"), "utf8");
  assert.match(api, /interface WikiHeroAnimation/);
  assert.match(api, /animation\?: WikiHeroAnimation/);
  assert.match(view, /animationType/);
  assert.match(view, /animationParamsText/);
  assert.match(view, /Galaxy 参数必须是合法 JSON/);
  assert.match(view, /Galaxy 参数必须是 JSON 对象/);
  assert.match(view, /type: "galaxy"/);
  assert.match(view, /背景图片与 Galaxy 动画可独立或叠加/);
});

test("Console 设置页暴露 Card Hover 配置并保持安全边界", () => {
  const settings = read("modules/settings/SettingsView.vue");
  const plugins = read("modules/settings/PluginsTab.vue");
  assert.match(settings, /card_hover\.enable = .*false/);
  assert.match(settings, /card_hover\.max_tilt = .*3/);
  assert.match(plugins, /卡片 Hover 光斑与倾斜/);
  assert.match(plugins, /min="0" max="8"/);
});
