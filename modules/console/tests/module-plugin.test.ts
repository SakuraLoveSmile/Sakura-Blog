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

test("Console plugins: plugin library exposes five admin child routes and legacy redirect", () => {
  const moduleSource = read("modules/core/module.ts");
  for (const route of ["shortcodes", "assets", "widgets", "fixtures", "runtime"]) {
    assert.match(moduleSource, new RegExp(`path: "/plugins/${route}"`));
    assert.match(moduleSource, new RegExp(`path: "plugins/${route}"`));
    assert.match(moduleSource, /permissions: admin/);
  }
  assert.match(moduleSource, /path: "plugins", redirect: "\/plugins\/shortcodes"/);
  assert.match(moduleSource, /ShortcodeStudioView\.vue/);
  assert.match(moduleSource, /fixtures\/FixtureStudioView\.vue/);
  assert.match(moduleSource, /ScriptPluginListView\.vue/);
});

test("Console plugins: wiki and projects merged into one projects entry with docs under the project scope", () => {
  const moduleSource = read("modules/core/module.ts");
  // 侧栏只保留一个「项目管理」入口，且不再挂子项。
  assert.match(moduleSource, /path: "\/wiki", label: "项目管理", icon: "ri:stack-line"/);
  assert.doesNotMatch(moduleSource, /label: "文库"/);
  assert.doesNotMatch(moduleSource, /\/wiki\/projects", label: "项目管理"/);
  // 项目列表是落地页；文档工作台在项目作用域下。
  assert.match(moduleSource, /path: "wiki", redirect: "\/wiki\/projects"/);
  assert.match(moduleSource, /path: "wiki\/projects", name: "WikiProjects"/);
  assert.match(moduleSource, /path: "wiki\/:project\/docs",\s+name: "WikiDocs"/);
  // 文档页不再用下拉框选项目，项目只来自路由。
  const docs = read("modules/wiki/WikiDocsView.vue");
  assert.match(docs, /const activeProject = computed\(\(\) => String\(route\.params\.project \|\| ""\)\)/);
  assert.doesNotMatch(docs, /title="选择文库项目"/);
});

test("Console plugins: catalog and runtime pages expose recoverable loading states", () => {  const catalog = read("modules/plugins/usePluginCatalog.ts");
  const shortcode = read("modules/plugins/ShortcodeStudioView.vue");
  const runtime = read("modules/plugins/ScriptPluginListView.vue");
  assert.match(catalog, /catalogStatus/);
  assert.match(catalog, /retryLoadCatalog/);
  assert.match(shortcode, /无法加载标签插件/);
  assert.match(shortcode, /暂无标签插件/);
  assert.match(runtime, /pluginApi\.runtime\(\)/);
  assert.match(runtime, /network_allowlist/);
  assert.match(runtime, /重试/);
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
