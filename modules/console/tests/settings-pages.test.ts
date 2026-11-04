import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import {
  canonicalFieldByPath,
  canonicalSettingsGroups,
  canonicalSettingsPaths,
} from "../web/src/modules/settings/settings-canonical-fields.ts";
import {
  DEFAULT_SETTINGS_PAGE,
  resolveSettingsTab,
  settingsGroups,
  settingsPageFields,
  settingsPageGroups,
  settingsPageKeys,
  settingsPagePaths,
  settingsPages,
  settingsThemePages,
} from "../web/src/modules/settings/settings-pages.ts";

const schema = JSON.parse(readFileSync(new URL("../../../internal/stellarv2/schema.json", import.meta.url), "utf8"));

test("设置导航是 5 组 12 页，每页恰好出现一次且顺序稳定", () => {
  assert.equal(settingsPageKeys.length, 12);
  assert.deepEqual(
    settingsGroups.map((group) => [group.label, group.pages.map((key) => settingsPages[key].label)]),
    [
      ["站点", ["站点信息", "SEO 与搜索"]],
      ["外观与布局", ["主题外观", "导航与页脚", "页面与侧栏"]],
      ["内容", ["文章页", "列表与卡片", "作者与友链"]],
      ["互动", ["评论", "访客反馈"]],
      ["扩展与高级", ["扩展与服务", "代码与原文"]],
    ],
  );
  const listed = settingsGroups.flatMap((group) => group.pages);
  assert.deepEqual([...listed].sort(), [...settingsPageKeys].sort());
  assert.equal(new Set(listed).size, listed.length);
});

test("只有站点信息与访客反馈独立保存，其余 10 页共享主题草稿", () => {
  const host = settingsPageKeys.filter((key) => settingsPages[key].host);
  assert.deepEqual(host.sort(), ["feedback", "site"]);
  assert.equal(settingsPages.site.host, "site");
  assert.equal(settingsPages.feedback.host, "feedback");
  assert.equal(settingsThemePages.length, 10);
  for (const key of host) assert.deepEqual(settingsPagePaths(key), []);
});

test("每个字段路径只属于一页，且与字段目录（不含 AI 页）逐项对应", () => {
  const owners = new Map<string, string>();
  for (const key of settingsThemePages)
    for (const item of settingsPageFields(key)) {
      assert.ok(canonicalFieldByPath.has(item.path), `${key} 引用了目录之外的 ${item.path}`);
      assert.ok(!owners.has(item.path), `${item.path} 同时属于 ${owners.get(item.path)} 与 ${key}`);
      owners.set(item.path, key);
    }
  const catalog = Object.entries(canonicalSettingsGroups)
    .filter(([tab]) => tab !== "ai")
    .flatMap(([, groups]) => groups.flatMap((group) => group.fields.map((item) => item.path)));
  assert.deepEqual([...owners.keys()].sort(), [...catalog].sort(), "目录里的字段必须全部有归属，页面不得凭空新增");
});

test("Go 搜索覆盖路径随搜索服务一起归属 SEO 与搜索页", () => {
  assert.ok(settingsPagePaths("seo").includes("blog_extensions.search.provider"));
  for (const key of settingsThemePages.filter((key) => key !== "seo"))
    assert.ok(!settingsPagePaths(key).includes("blog_extensions.search.provider"), key);
  // 旧「搜索」页的全部路径（含 Go 覆盖）都应并入新的 SEO 与搜索页。
  assert.ok(canonicalSettingsPaths("search").every((path) => settingsPagePaths("seo").includes(path)));
});

test("外观页按 schema 拆成分区后仍覆盖 theme.appearance 的全部顶层字段", () => {
  const keys = Object.keys(schema.theme.properties.appearance.properties).sort();
  const covered = settingsPageFields("appearance")
    .map((item) => item.path.replace(/^theme\.appearance\./, ""))
    .sort();
  assert.deepEqual(covered, keys);
});

test("每页的分区 id 唯一，有字段的分区可交给表单渲染", () => {
  for (const key of settingsPageKeys) {
    const ids = settingsPages[key].sections.map((section) => section.id);
    assert.equal(new Set(ids).size, ids.length, `${key} 分区 id 重复`);
    assert.ok(settingsPages[key].sections.length > 0, key);
    for (const group of settingsPageGroups(key)) {
      assert.ok(group.id && ids.includes(group.id), `${key}/${group.title}`);
      assert.ok(group.fields.length > 0);
    }
  }
  assert.deepEqual(
    settingsPageGroups("advanced").map((group) => group.title),
    ["代码注入"],
    "仅作锚点的“完整配置原文”分区不由表单渲染",
  );
});

test("列表与卡片、文章页的字段拆分符合归属对照", () => {
  const post = settingsPageFields("post").map((item) => item.path);
  const listing = settingsPageFields("listing").map((item) => item.path);
  for (const path of ["theme.article.style", "theme.article.paragraph_indent", "theme.article.show_reading_time"])
    assert.ok(post.includes(path), path);
  for (const path of [
    "theme.article.listing.card_layout",
    "theme.article.category_colors",
    "blog_extensions.content.category_icons",
    "blog_extensions.content.tag_icons",
  ])
    assert.ok(listing.includes(path), path);
  assert.ok(!post.some((path) => path.includes(".listing.")));
  assert.ok(settingsPageFields("extensions").some((item) => item.path === "theme.preconnect"));
  assert.ok(settingsPageFields("layout").some((item) => item.path === "theme.rightbar"));
  assert.ok(!settingsPageFields("navigation").some((item) => item.path === "theme.rightbar"));
});

test("第三方评论、Algolia 与站点信息/评分/投票不再是设置页字段，外部服务只保留 5 个子项", () => {
  const owned = settingsThemePages.flatMap((key) => settingsPageFields(key).map((item) => item.path));
  for (const removed of ["theme.comments", "theme.search.algolia", "theme.services.site_info", "theme.services.rating", "theme.services.vote"])
    assert.ok(!owned.some((path) => path === removed || path.startsWith(`${removed}.`)), removed);
  // 整个 theme.services 节点不再作为字段渲染，只保留下面列出的 5 个子项。
  assert.ok(!owned.includes("theme.services"));
  // 外部服务改为 5 个独立 structured 字段，不再整体渲染 theme.services。
  assert.deepEqual(
    settingsPageFields("extensions")
      .map((item) => item.path)
      .filter((path) => path.startsWith("theme.services")),
    [
      "theme.services.gravatar",
      "theme.services.markdown",
      "theme.services.contributors",
      "theme.services.github_card",
      "theme.services.github",
    ],
  );
  assert.deepEqual(
    settingsPages.comments.sections.map((section) => section.title),
    ["原生评论", "评论邮件通知"],
    "评论页只保留原生评论与邮件通知",
  );
  assert.deepEqual(
    settingsPageGroups("comments").map((group) => group.title),
    ["原生评论", "评论邮件通知"],
  );
});

test("旧版 Tab 与分段键落到新页面或原跳转目标", () => {
  const page = (tab: unknown) => {
    const target = resolveSettingsTab(tab);
    assert.equal(target.kind, "page", String(tab));
    return target.kind === "page" ? target.page : undefined;
  };
  for (const key of settingsPageKeys) assert.equal(page(key), key);
  const expected: Record<string, string> = {
    basic: "site",
    search: "seo",
    plugins: "extensions",
    services: "extensions",
    tag_plugins: "extensions",
    default: "extensions",
    article: "post",
    inject: "advanced",
    data_cache: "advanced",
    theme_yaml: "advanced",
    cover: "layout",
    widgets: "layout",
    rightbar: "layout",
    nav_tabs: "navigation",
    menubar: "navigation",
    footer: "navigation",
    authors: "community",
  };
  for (const [legacy, target] of Object.entries(expected)) assert.equal(page(legacy), target, legacy);
  assert.deepEqual(resolveSettingsTab("stellar2"), { kind: "page", page: "advanced", raw: true });
  assert.deepEqual(resolveSettingsTab("ai"), { kind: "route", path: "/assistant", query: { tab: "settings" } });
  assert.deepEqual(resolveSettingsTab("security"), { kind: "route", path: "/profile" });
  for (const value of [undefined, "", "nope", "__proto__", "constructor", 7, null])
    assert.deepEqual(resolveSettingsTab(value), { kind: "page", page: DEFAULT_SETTINGS_PAGE });
});
