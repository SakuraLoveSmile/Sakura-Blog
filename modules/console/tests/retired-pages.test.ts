import { test } from "node:test";
import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { retiredConsoleRedirect } from "../web/src/router/retired-pages.ts";
import { isSettingsPageKey } from "../web/src/modules/settings/settings-pages.ts";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../web/src");
const read = (path: string) => readFileSync(join(ROOT, path), "utf8");
const RETIRED_PAGES = ["appearance", "menus", "authors"] as const;

test("旧分段页面都落到已注册的设置页，并保留 feedback 旧改道", () => {
  assert.deepEqual(retiredConsoleRedirect("appearance", undefined), { path: "/settings", query: { tab: "appearance" } });
  assert.deepEqual(retiredConsoleRedirect("appearance", "feedback"), { path: "/settings", query: { tab: "feedback" } });
  assert.deepEqual(retiredConsoleRedirect("menus", undefined), { path: "/settings", query: { tab: "navigation" } });
  assert.deepEqual(retiredConsoleRedirect("authors", undefined), { path: "/settings", query: { tab: "community" } });
  // 未知 tab 与非字符串 query 不得把设置页带到注册表以外的页面。
  assert.equal(retiredConsoleRedirect("appearance", "theme_yaml").query.tab, "appearance");
  assert.equal(retiredConsoleRedirect("menus", ["navigation"]).query.tab, "navigation");
  for (const page of RETIRED_PAGES) {
    const tab = retiredConsoleRedirect(page, undefined).query.tab;
    assert.ok(isSettingsPageKey(tab), `${page} 的目标 ${tab} 必须是已注册的设置页`);
  }
});

test("旧页面组件已删除，路由只保留重定向，侧栏 manifest 不再挂死胡同入口", () => {
  const core = read("modules/core/module.ts");
  for (const page of RETIRED_PAGES) {
    assert.match(core, new RegExp(`redirect: \\(to\\) => retiredConsoleRedirect\\("${page}", to\\.query\\.tab\\)`));
    assert.doesNotMatch(core, new RegExp(`path: "/${page}"`), `侧栏不得再挂 /${page}`);
  }
  assert.doesNotMatch(core, /modules\/appearance\/(AppearanceView|MenuView)\.vue|modules\/authors\/AuthorsView\.vue/);
  for (const retired of ["modules/appearance/AppearanceView.vue", "modules/appearance/MenuView.vue", "modules/authors/AuthorsView.vue"]) {
    assert.equal(existsSync(join(ROOT, retired)), false, `${retired} 应已删除`);
  }
});
