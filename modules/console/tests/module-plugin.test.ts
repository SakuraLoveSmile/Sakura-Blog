import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../web/src");
const read = (path: string) => readFileSync(join(ROOT, path), "utf8");

test("Console plugins: compile-time glob collects modules and widgets", () => {
  const registry = read("plugin-registry.ts");
  const moments = read("modules/moments/module.ts");
  assert.match(registry, /import\.meta\.glob\("\.\/modules\/\*\*\/module\.ts"/);
  assert.match(registry, /syncPluginWidgets/);
  assert.match(moments, /id: "moments"/);
  assert.match(moments, /path: "moments"/);
  assert.match(moments, /id: "recent-moments"/);
});

test("Console plugins: router permissions/modules and manifest-driven navigation are wired", () => {
  const router = read("router/index.ts");
  const layout = read("layouts/DefaultLayout.vue");
  const store = read("stores/modules.ts");
  assert.doesNotMatch(router, /adminOnly/);
  assert.match(router, /moduleId/);
  assert.match(router, /permissionsFor/);
  assert.match(router, /hasPermission/);
  assert.match(layout, /enabledNav\(\)/);
  assert.doesNotMatch(layout, /const navItems/);
  assert.match(store, /moduleApi\.list/);
  assert.match(store, /syncPluginWidgets/);
});

test("moments disable path: dashboard keeps UnknownWidget fallback", () => {
  const grid = read("modules/dashboard/DashboardGrid.vue");
  const widgets = read("modules/dashboard/widgets/UnknownWidget.vue");
  assert.match(grid, /<UnknownWidget v-else/);
  assert.match(widgets, /widgetId/);
});

test("runtime bundle loading stays same-origin and externalizes host dependencies", () => {
  const runtime = read("../../../../packages/plugin-runtime/src/index.ts");
  const vite = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "../web/vite.plugin.config.ts"), "utf8");
  assert.match(runtime, /startsWith\("\/plugins\/"\)/);
  assert.match(runtime, /identity mismatch/);
  assert.match(vite, /external/);
  assert.match(vite, /@blog\/components/);
});

test("permission directive and lazy i18n are registered by both entries", () => {
  const main = read("main.ts");
  const ucMain = read("uc/main.ts");
  const directive = read("directives/permission.ts");
  const i18n = read("i18n/index.ts");
  assert.match(main, /app\.directive\("permission"/);
  assert.match(ucMain, /app\.directive\("permission"/);
  assert.match(directive, /hasPermission/);
  assert.match(i18n, /import\("\.\/locales\/en-US"\)/);
});
