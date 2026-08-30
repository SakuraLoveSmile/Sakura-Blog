import { test } from "node:test";
import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../web/src");
const read = (path: string) => readFileSync(join(ROOT, path), "utf8");

test("settings center exposes one six-tab system settings surface", () => {
  const view = read("modules/settings/SettingsView.vue");
  for (const label of ["基本", "文章", "评论", "SEO", "搜索", "高级"])
    assert.match(view, new RegExp(`label: "${label}"`));
  assert.doesNotMatch(view, /activeGroup|currentGroupTabs|安全设置/);
  assert.match(view, /settings-tabs/);
  assert.match(view, /settings-basic-panel/);
  assert.match(view, /VTabbar/);
  assert.match(view, /@save="saveComments"/);
  assert.match(view, /ALL_SETTINGS_SECTIONS/);
});

test("basic settings follows Halo identity and branding order", () => {
  const basic = read("modules/settings/tabs/BasicSettings.vue");
  const markers = ["settings-site-title", "settings-site-subtitle", ">Logo<", ">Favicon<", ">Apple Touch Icon<"];
  let previous = -1;
  for (const marker of markers) {
    const index = basic.indexOf(marker);
    assert.ok(index > previous, `${marker} should follow the previous Halo basic field`);
    previous = index;
  }
  assert.doesNotMatch(basic, /data\.site\.description/);
  assert.doesNotMatch(basic, /data\.site\.url/);
  const seo = read("modules/settings/tabs/SeoSettings.vue");
  assert.match(seo, /data\.site\.description/);
  assert.match(seo, /data\.site\.url/);
  assert.match(seo, /emit\('save', 'site'\)/);
});

test("settings center keeps structured forms and section save contracts split by tab", () => {
  const components: Array<[string, RegExp]> = [
    ["BasicSettings.vue", /emit\('save', 'site'\)/],
    ["PostSettings.vue", /emit\('save', 'article'\)/],
    ["CommentSettings.vue", /settingsApi\.testNotify/],
    ["SeoSettings.vue", /emit\('save', 'seo'\)/],
    ["SearchSettings.vue", /emit\('save', 'search'\)/],
    ["AdvancedSettings.vue", /saveJson/],
  ] as const;
  for (const [name, marker] of components) {
    assert.ok(existsSync(join(ROOT, "modules/settings/tabs", name)), `${name} should exist`);
    assert.match(read(`modules/settings/tabs/${name}`), new RegExp(marker));
  }
  assert.doesNotMatch(read("modules/settings/tabs/PostSettings.vue"), /friends\./);
});

test("appearance, menu, author, assistant and friend settings routes are wired", () => {
  const core = read("modules/core/module.ts");
  for (const marker of ['path: "/appearance"', 'path: "/menus"', 'path: "/authors"'])
    assert.match(core, new RegExp(marker));
  assert.match(read("modules/appearance/AppearanceView.vue"), /enhancements/);
  assert.match(read("modules/appearance/MenuView.vue"), /updateSection\("menubar"/);
  assert.match(read("modules/authors/AuthorsView.vue"), /updateSection\("authors"/);
  assert.match(read("modules/assistant/AssistantView.vue"), /tab === "settings"/);
  assert.match(read("modules/assistant/AiSettings.vue"), /updateSection\("ai"/);
  assert.match(read("modules/friends/FriendList.vue"), /activeTab.*settings/);
  assert.match(read("modules/friends/FriendSettings.vue"), /updateSection\("friends"/);
});

test("legacy settings tabs redirect to their new owners", () => {
  const view = read("modules/settings/SettingsView.vue");
  for (const marker of [
    'appearance: { path: "/appearance", tab: "appearance" }',
    'menubar: { path: "/menus" }',
    'ai: { path: "/assistant", tab: "settings" }',
    'security: { path: "/profile" }',
    'authors: { path: "/authors" }',
  ])
    assert.match(view, new RegExp(marker));
});
