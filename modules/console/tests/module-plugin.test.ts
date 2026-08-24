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

test("Console plugins: router roles/modules and manifest-driven navigation are wired", () => {
  const router = read("router/index.ts");
  const layout = read("layouts/DefaultLayout.vue");
  const store = read("stores/modules.ts");
  assert.doesNotMatch(router, /adminOnly/);
  assert.match(router, /moduleId/);
  assert.match(router, /rolesFor/);
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
