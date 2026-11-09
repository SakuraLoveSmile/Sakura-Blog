import { test } from "node:test";
import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { ensureSettingsDefaults } from "../web/src/modules/settings/settings-state.ts";
import { resolveSettingsTab } from "../web/src/modules/settings/settings-pages.ts";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../web/src");
const read = (path: string) => readFileSync(join(ROOT, path), "utf8");

test("settings center exposes one registry-driven system settings surface of 5 groups and 12 pages", () => {
  const view = read("modules/settings/SettingsView.vue");
  const registry = read("modules/settings/settings-pages.ts");
  for (const label of ["站点信息", "SEO 与搜索", "主题外观", "导航与页脚", "页面与侧栏", "文章页", "列表与卡片", "作者与友链", "评论", "访客反馈", "扩展与服务", "代码与原文"])
    assert.match(registry, new RegExp(`label: "${label}",`));
  for (const label of ["站点", "外观与布局", "内容", "互动", "扩展与高级"])
    assert.match(registry, new RegExp(`label: "${label}",`));
  assert.doesNotMatch(view, /activeGroup|currentGroupTabs|安全设置/);
  assert.match(view, /<SettingsNav/);
  assert.doesNotMatch(view, /VTabbar|settings-tabs/);
  assert.match(view, /CanonicalSettingsForm/);
  assert.doesNotMatch(view, /CanonicalSettingsNotice/);
  assert.doesNotMatch(view, /<SeoSettings/);
  assert.match(view, /ALL_SETTINGS_SECTIONS/);
});

test("basic settings follows Halo identity and branding order and owns the site description and URL", () => {
  const basic = read("modules/settings/tabs/BasicSettings.vue");
  const markers = [
    "settings-site-title",
    "settings-site-subtitle",
    ">Logo<",
    ">Favicon<",
    ">Apple Touch Icon<",
    "settings-site-desc",
    "settings-site-url",
  ];
  let previous = -1;
  for (const marker of markers) {
    const index = basic.indexOf(marker);
    assert.ok(index > previous, `${marker} should follow the previous Halo basic field`);
    previous = index;
  }
  assert.match(basic, /data\.site\.description/);
  assert.match(basic, /data\.site\.url/);
  assert.match(basic, /emit\(["']save["'], ["']site["']\)/);
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
    ["BasicSettings.vue", /emit\(["']save["'], ["']site["']\)/],
    ["PostSettings.vue", /emit\(["']save["'], ["']article["']\)/],
    ["CommentSettings.vue", /settingsApi\.testNotify/],
    ["SearchSettings.vue", /emit\(["']save["'], ["']search["']\)/],
  ] as const;
  for (const [name, marker] of components) {
    assert.ok(existsSync(join(ROOT, "modules/settings/tabs", name)), `${name} should exist`);
    assert.match(read(`modules/settings/tabs/${name}`), new RegExp(marker));
  }
  assert.doesNotMatch(read("modules/settings/tabs/PostSettings.vue"), /friends\./);
  // 旧整段 JSON 保存页已退役：12 页结构化设置接管后没有入口，保存走统一保存栏。
  assert.ok(
    !existsSync(join(ROOT, "modules/settings/tabs/AdvancedSettings.vue")),
    "AdvancedSettings.vue 已退役，不应重新出现",
  );
  assert.ok(!existsSync(join(ROOT, "modules/settings/ThemeYamlTab.vue")), "ThemeYamlTab.vue 已退役，不应重新出现");
});

test("退役的旧分段页面只保留重定向，助手与友链设置仍走 canonical 表单", () => {
  const core = read("modules/core/module.ts");
  for (const page of ["appearance", "menus", "authors"]) {
    assert.match(core, new RegExp(`redirect: \\(to\\) => retiredConsoleRedirect\\("${page}", to\\.query\\.tab\\)`));
  }
  // 页面组件已删除：旧地址不再是可停留的空页面，避免再次变成死胡同。
  for (const retired of ["modules/appearance/AppearanceView.vue", "modules/appearance/MenuView.vue", "modules/authors/AuthorsView.vue"]) {
    assert.equal(existsSync(join(ROOT, retired)), false, `${retired} should be removed`);
  }
  assert.match(read("modules/assistant/AssistantView.vue"), /tab === "settings"/);
  assert.match(read("modules/assistant/AiSettings.vue"), /CanonicalSettingsForm/);
  assert.match(read("modules/friends/FriendList.vue"), /activeTab.*settings/);
  assert.match(read("modules/friends/FriendSettings.vue"), /CanonicalSettingsNotice/);
});

test("basic settings validates site title and supports native form submit", () => {
  const basic = read("modules/settings/tabs/BasicSettings.vue");
  assert.match(basic, /<form class="settings-basic-form" @submit\.prevent="handleSubmit">/);
  assert.match(basic, /站点标题不能为空/);
  // 保存按钮由统一保存栏经 actions 插槽放进表单，类型为原生 submit：回车与点击都先走 handleSubmit 的标题校验。
  assert.match(basic, /<slot name="actions" \/>/);
  assert.doesNotMatch(basic, /native-type="submit"/);
  assert.match(read("modules/settings/SettingsSaveBar.vue"), /native-type="submit"/);
  assert.match(
    read("modules/settings/SettingsView.vue"),
    /<BasicSettings[\s\S]*?<template #actions>[\s\S]*?<SettingsSaveBar[\s\S]*?scope="host"/,
  );
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
  assert.deepEqual(resolveSettingsTab("appearance"), { kind: "page", page: "appearance" });
  assert.deepEqual(resolveSettingsTab("menubar"), { kind: "page", page: "navigation" });
  assert.deepEqual(resolveSettingsTab("authors"), { kind: "page", page: "community" });
  assert.deepEqual(resolveSettingsTab("ai"), { kind: "route", path: "/assistant", query: { tab: "settings" } });
  assert.deepEqual(resolveSettingsTab("security"), { kind: "route", path: "/profile" });
  // 视图只通过注册表落页：旧键由 resolveSettingsTab 解析，路由目标改走 router.replace。
  const view = read("modules/settings/SettingsView.vue");
  assert.match(view, /resolveSettingsTab\(/);
  assert.match(view, /router\.replace\(\{ path: target\.path, query: target\.query \}\)/);
});

test("Stellar runtime settings are exposed by structured forms", () => {
  const search = read("modules/settings/tabs/SearchSettings.vue");
  assert.match(search, /首次聚焦再加载/);
  assert.match(search, /data\.search\.local_search\.lazy_load/);
  assert.match(search, /settings-search-cache-ttl/);
  assert.match(search, /data\.search\.local_search\.cache_ttl/);

  // 旧整段 JSON 页退役后，服务与缓存默认值仍由 settings-state 承载。
  const advanced = read("modules/settings/settings-state.ts");
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
