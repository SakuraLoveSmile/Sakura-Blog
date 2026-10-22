import { test } from "node:test";
import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { ensureSettingsDefaults } from "../web/src/modules/settings/settings-state.ts";

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
  assert.match(view, /CanonicalSettingsForm/);
  assert.doesNotMatch(view, /CanonicalSettingsNotice/);
  assert.match(view, /<SeoSettings[^>]*host-only/);
  assert.match(read("modules/settings/tabs/SeoSettings.vue"), /<template v-if="!hostOnly">/);
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

test("post settings follows Halo vertical form order and keeps the article save contract", () => {
  const post = read("modules/settings/tabs/PostSettings.vue");
  assert.match(post, /<form class="settings-post-form" @submit\.prevent="handleSubmit">/);
  const markers = ["文章展示", "文章布局类型", "阅读体验", "分类与标签映射", "版权与分享", "打赏"];
  let previous = -1;
  for (const marker of markers) {
    const index = post.indexOf(marker);
    assert.ok(index > previous, `${marker} should follow the previous post form group`);
    previous = index;
  }
  assert.match(post, /settings-post-actions/);
  assert.match(post, /native-type="submit"/);
  assert.match(post, /emit\(["']save["'], ["']article["']\)/);
  assert.match(post, /category_color/);
  assert.match(post, /AttachmentField/);
  assert.match(post, /IconPicker/);
  assert.doesNotMatch(post, /<VCard title="文章设置">/);
});

test("settings center keeps structured forms and section save contracts split by tab", () => {
  const components: Array<[string, RegExp]> = [
    ["BasicSettings.vue", /emit\('save', 'site'\)/],
    ["PostSettings.vue", /emit\(["']save["'], ["']article["']\)/],
    ["CommentSettings.vue", /settingsApi\.testNotify/],
    ["SeoSettings.vue", /emit\('save', 'seo'\)/],
    ["SearchSettings.vue", /emit\(["']save["'], ["']search["']\)/],
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
  assert.match(read("modules/appearance/AppearanceView.vue"), /CanonicalSettingsNotice/);
  assert.match(read("modules/appearance/MenuView.vue"), /CanonicalSettingsNotice/);
  assert.match(read("modules/authors/AuthorsView.vue"), /CanonicalSettingsNotice/);
  assert.match(read("modules/assistant/AssistantView.vue"), /tab === "settings"/);
  assert.match(read("modules/assistant/AiSettings.vue"), /CanonicalSettingsForm/);
  assert.match(read("modules/friends/FriendList.vue"), /activeTab.*settings/);
  assert.match(read("modules/friends/FriendSettings.vue"), /CanonicalSettingsNotice/);
});

test("basic settings validates site title and supports native form submit", () => {
  const basic = read("modules/settings/tabs/BasicSettings.vue");
  assert.match(basic, /<form class="settings-basic-form" @submit\.prevent="handleSubmit">/);
  assert.match(basic, /站点标题不能为空/);
  assert.match(basic, /native-type="submit"/);
});

test("settings view tracks dirty state and guards unsaved changes on leave", () => {
  const view = read("modules/settings/SettingsView.vue");
  assert.match(view, /isDirty/);
  assert.match(view, /onBeforeRouteLeave/);
  assert.match(view, /beforeunload/);
  assert.match(view, /settings-dirty-badge/);
  assert.match(view, /settings-tab-content/);
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

test("Stellar runtime settings are exposed by structured forms", () => {
  const search = read("modules/settings/tabs/SearchSettings.vue");
  assert.match(search, /首次聚焦再加载/);
  assert.match(search, /data\.search\.local_search\.lazy_load/);
  assert.match(search, /settings-search-cache-ttl/);
  assert.match(search, /data\.search\.local_search\.cache_ttl/);

  const advanced = read("modules/settings/tabs/AdvancedSettings.vue");
  assert.match(advanced, /settings-cache-max-entries/);
  for (const service of ["giscus", "waline", "artalk", "memos-user", "sites", "friends_and_posts", "siteinfo"])
    assert.match(advanced, new RegExp(service));

  const enhancements = read("modules/appearance/EnhancementsSettings.vue");
  for (const marker of [
    "settings-siteinfo-api",
    "note.border",
    "checkbox.interactive",
    "okr.border",
    "emojiSources",
    "qq",
    "addStatus",
  ])
    assert.match(enhancements, new RegExp(marker.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
  // feedback 段专属设置 tab（v1.7.1 自主题设置迁出）
  const feedback = read("modules/settings/tabs/FeedbackSettings.vue");
  for (const marker of [
    "settings-feedback-api-base",
    "settings-feedback-app-id",
    "settings-feedback-launcher-mode",
    "data.feedback.enabled",
  ])
    assert.match(feedback, new RegExp(marker.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
  const state = read("modules/settings/settings-state.ts");
  assert.match(state, /"services"/);
  assert.match(state, /"feedback"/);
});

test("ensureSettingsDefaults safely initializes article ai_label and edit_this_page", () => {
  const defaulted = ensureSettingsDefaults({ article: null });
  assert.ok(defaulted.article, "article must be an object");
  assert.ok(defaulted.article.ai_label, "article.ai_label must be an object");
  assert.strictEqual(defaulted.article.ai_label.default, "");
  for (const tier of ["manual", "reviewed", "polished", "generated"] as const) {
    assert.ok(defaulted.article.ai_label[tier], `article.ai_label.${tier} must exist`);
    assert.strictEqual(defaulted.article.ai_label[tier].color, "");
    assert.strictEqual(defaulted.article.ai_label[tier].icon, "");
  }
  assert.ok(defaulted.article.edit_this_page, "article.edit_this_page must be an object");
  assert.deepStrictEqual(defaulted.article.edit_this_page, {});
});
