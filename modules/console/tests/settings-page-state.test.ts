import { test } from "node:test";
import assert from "node:assert/strict";
import { canonicalFieldByPath } from "../web/src/modules/settings/settings-canonical-fields.ts";
import {
  changeCount,
  hasOverride,
  offCatalogChanged,
  pageChangeCount,
  parseDocument,
  restorePage,
  restorePaths,
  themeChangeSummary,
} from "../web/src/modules/settings/settings-page-state.ts";
import { settingsPagePaths } from "../web/src/modules/settings/settings-pages.ts";
import { canonicalDocument, patchJSONField } from "../web/src/components/editor/stellar-form-json.ts";

const saved = JSON.stringify(
  {
    version: 2,
    theme: { article: { style: "story" }, topbar: { brand: { name: "Demo" } }, appearance: { colors: { primary: "#111" } } },
    blog_extensions: { content: { enabled: true } },
  },
  null,
  2,
);
const edit = (doc: string, path: string, value: unknown) => patchJSONField(doc, path, value);
const field = (path: string) => canonicalFieldByPath.get(path)!;

test("changeCount 把缺失与空对象视为相同，并按叶子计数", () => {
  assert.equal(changeCount(undefined, undefined), 0);
  assert.equal(changeCount({}, undefined), 0);
  assert.equal(changeCount({ a: {} }, {}), 0);
  assert.equal(changeCount({ a: 1, b: { c: 2 } }, { a: 1, b: { c: 3 } }), 1);
  assert.equal(changeCount({ a: 1, b: 2 }, undefined), 2);
  assert.equal(changeCount([1, 2], [1, 2]), 0);
  assert.equal(changeCount([1, 2], [2, 1]), 1);
  assert.equal(changeCount(undefined, "x"), 1);
  assert.equal(changeCount({ a: 1 }, "x"), 1);
});

test("页面计数：普通控件算一项，结构化字段按内部叶子计数", () => {
  const draft = edit(
    edit(
      edit(saved, "theme.article.show_reading_time", true),
      "theme.appearance.colors",
      { primary: "#222", accent: "#333" },
    ),
    "theme.article.style",
    "tech",
  );
  const d = parseDocument(draft)!;
  const s = parseDocument(saved)!;
  assert.equal(pageChangeCount(d, s, "post"), 2);
  assert.equal(pageChangeCount(d, s, "appearance"), 2);
  assert.equal(pageChangeCount(d, s, "comments"), 0);
  const summary = themeChangeSummary(draft, saved);
  assert.equal(summary.pages.post, 2);
  assert.equal(summary.pages.appearance, 2);
  assert.equal(summary.other, 0);
  assert.equal(summary.total, 4);
});

test("搜索服务组合控件任一覆盖路径变化都只算一项", () => {
  const draft = edit(edit(saved, "theme.search.provider", "local"), "blog_extensions.search.provider", "go");
  assert.equal(pageChangeCount(parseDocument(draft)!, parseDocument(saved)!, "seo"), 1);
});

test("只在完整配置原文里改动目录之外的参数时计入“其他”，且父级空对象残留不算改动", () => {
  const raw = edit(saved, "theme.custom_extra", 1);
  assert.equal(themeChangeSummary(raw, saved).other, 1);
  assert.equal(themeChangeSummary(raw, saved).total, 1);
  assert.ok(offCatalogChanged(parseDocument(raw), parseDocument(saved)));
  const residue = edit(saved, "theme.article.show_reading_time", true);
  const cleared = patchJSONField(residue, "theme.article.show_reading_time", undefined);
  assert.equal(themeChangeSummary(cleared, saved).total, 0);
  const emptyParent = patchJSONField(patchJSONField(saved, "theme.listing_x", {}), "theme.appearance.shape", {});
  assert.equal(themeChangeSummary(emptyParent, saved).total, 0, "空对象与缺失等价");
});

test("hasOverride 以草稿里是否存在显式覆盖为准，空结构化对象不算覆盖", () => {
  const draft = parseDocument(edit(saved, "theme.appearance.shape", {}))!;
  assert.equal(hasOverride(draft, field("theme.article.style")), true);
  assert.equal(hasOverride(draft, field("theme.article.show_reading_time")), false);
  assert.equal(hasOverride(draft, field("theme.appearance.shape")), false);
  assert.equal(hasOverride(draft, field("theme.appearance.colors")), true);
  const provider = parseDocument(edit(saved, "blog_extensions.search.provider", "go"))!;
  assert.equal(hasOverride(provider, field("theme.search.provider")), true);
});

test("canonicalDocument 忽略空白与键顺序，取最后一个重复键，并保留超精度整数", () => {
  assert.equal(canonicalDocument('{ "b": 1, "a": {"y":2, "x":[1, 2]} }'), canonicalDocument('{"a":{"x":[1,2],"y":2},"b":1}'));
  assert.equal(canonicalDocument('{"a":1,"a":2}'), canonicalDocument('{"a":2}'));
  assert.notEqual(canonicalDocument('{"n":9007199254740993}'), canonicalDocument('{"n":9007199254740992}'));
  assert.notEqual(canonicalDocument('{"a":[1,2]}'), canonicalDocument('{"a":[2,1]}'));
  assert.equal(canonicalDocument('{"s":"\\u4e2d"}'), canonicalDocument('{"s":"中"}'));
});

test("放弃本页修改：只改本页字段后还原，草稿与已保存原文逐字一致", () => {
  const draft = edit(edit(saved, "theme.article.show_reading_time", true), "theme.article.style", "tech");
  const restored = restorePaths(draft, saved, settingsPagePaths("post"));
  assert.equal(restored, saved);
});

test("放弃本页修改：新增叶子被移除并清理空父对象，不留下 {} 残留", () => {
  const base = JSON.stringify({ version: 2, theme: { topbar: { brand: { name: "Demo" } } }, blog_extensions: {} }, null, 2);
  const draft = edit(edit(base, "theme.article.listing.card_layout", "classic"), "theme.article.style", "story");
  const restored = restorePaths(draft, base, settingsPagePaths("post").concat(settingsPagePaths("listing")));
  assert.equal(restored, base);
  assert.equal(restored.includes('"article"'), false);
});

test("放弃本页修改保留其他页的草稿，大整数原文不被改写", () => {
  const withBig = '{"version":2,"theme":{"article":{"style":"story"},"x":9007199254740993},"blog_extensions":{}}';
  const draft = edit(edit(withBig, "theme.article.style", "tech"), "theme.topbar", { brand: { name: "New" } });
  const restored = restorePaths(draft, withBig, settingsPagePaths("post"));
  assert.notEqual(restored, withBig);
  assert.ok(restored.includes("9007199254740993"));
  assert.equal(JSON.parse(restored).theme.article.style, "story");
  assert.deepEqual(JSON.parse(restored).theme.topbar, { brand: { name: "New" } });
  assert.equal(themeChangeSummary(restored, withBig).pages.post, 0);
  assert.equal(themeChangeSummary(restored, withBig).pages.navigation, 1);
});

test("放弃代码与原文页：目录之外的改动与注入代码回到已保存值，其余页的草稿保留", () => {
  const base = JSON.stringify(
    { version: 2, theme: { article: { style: "story" }, inject: { head_end: "<!-- a -->" }, keep: 1 }, blog_extensions: {} },
    null,
    2,
  );
  const draft = edit(
    edit(edit(edit(base, "theme.article.style", "tech"), "theme.inject.head_end", "<!-- b -->"), "theme.keep", 2),
    "theme.custom_extra",
    { a: 1 },
  );
  const restored = restorePage(draft, base, "advanced");
  const doc = JSON.parse(restored);
  assert.equal(doc.theme.article.style, "tech", "其他页的草稿不应丢失");
  assert.equal(doc.theme.inject.head_end, "<!-- a -->");
  assert.equal(doc.theme.keep, 1, "目录之外的改动回到已保存值");
  assert.equal(doc.theme.custom_extra, undefined);
  assert.equal(themeChangeSummary(restored, base).other, 0);
  assert.equal(themeChangeSummary(restored, base).pages.post, 1);
  // 本页没有改动时原样返回草稿，不重写格式。
  const onlyPost = edit(base, "theme.article.style", "tech");
  assert.equal(restorePage(onlyPost, base, "advanced"), onlyPost);
  // 普通主题页仍只还原自己的路径。
  assert.equal(restorePage(onlyPost, base, "post"), base);
});

test("放弃本页修改：已保存值缺失的显式覆盖被移除，已存在值逐字恢复", () => {
  const draft = edit(edit(saved, "theme.article.style", "tech"), "theme.article.paragraph_indent", "never");
  const restored = restorePaths(draft, saved, ["theme.article.style", "theme.article.paragraph_indent"]);
  assert.equal(restored, saved);
  assert.equal(JSON.parse(restored).theme.article.paragraph_indent, undefined);
});

test("已从设置页移除的第三方评论与站点信息节点只算完整原文改动，保留的服务子项仍随扩展页计数", () => {
  const base = JSON.stringify(
    {
      version: 2,
      theme: {
        services: { gravatar: { base_url: "https://a.example/avatar/" }, site_info: { provider: "site_info_api" } },
        comments: { giscus: { theme: "dark" } },
      },
      blog_extensions: {},
    },
    null,
    2,
  );
  // 仍在“扩展与服务”页的 Gravatar 字段按页计数，不落入“其他”。
  const kept = edit(base, "theme.services.gravatar.base_url", "https://b.example/avatar/");
  assert.equal(pageChangeCount(parseDocument(kept)!, parseDocument(base)!, "extensions"), 1);
  assert.equal(themeChangeSummary(kept, base).other, 0);
  // 站点信息、第三方评论已无界面，只能从完整原文修改，因此只计入“其他”。
  const siteInfo = edit(base, "theme.services.site_info.provider", "star_vote");
  assert.equal(pageChangeCount(parseDocument(siteInfo)!, parseDocument(base)!, "extensions"), 0);
  assert.equal(themeChangeSummary(siteInfo, base).other, 1);
  const comments = edit(base, "theme.comments.giscus.theme", "light");
  assert.equal(pageChangeCount(parseDocument(comments)!, parseDocument(base)!, "comments"), 0);
  assert.equal(themeChangeSummary(comments, base).other, 1);
});
