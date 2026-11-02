import { test } from "node:test";
import assert from "node:assert/strict";
import * as Vue from "../web/node_modules/vue/index.js";
import { useStellarForm } from "../web/src/components/editor/useStellarForm.ts";
import { pageFields, postFields } from "../web/src/components/editor/stellar-form-fields.ts";

const json = (value: unknown) => JSON.parse(String(value));

function createForm(config: string, blog: string, extra: Record<string, unknown> = {}) {
  const form = Vue.reactive({
    stellar_version: 2,
    stellar_config: config,
    blog_extensions: blog,
    title: "页面标题",
    banner: "",
    banner_info: { title: "", subtitle: "", avatar: "", icon: "" },
    breadcrumb: true,
    author: "",
    date: "",
    leftbar_widgets: null,
    rightbar_widgets: null,
    ...extra,
  });
  useStellarForm(() => form, pageFields);
  return form;
}

test("页面横幅背景优先回填 Blog 扩展历史值，缺失时回退 front matter cover", () => {
  const withBlog = createForm('{"cover":"https://x.test/cover.webp"}', '{"content":{"banner_image":"https://x.test/blog.webp"}}');
  assert.equal(withBlog.banner, "https://x.test/blog.webp");
  const coverOnly = createForm('{"cover":"https://x.test/cover.webp"}', "{}");
  assert.equal(coverOnly.banner, "https://x.test/cover.webp");
  assert.equal(createForm("{}", "{}").banner, "");
});

test("侧栏品牌图片优先回填博客扩展旧 icon，缺失时回退 leftbar.brand.image.src", () => {
  const withBlog = createForm("{}", '{"content":{"banner_icon":"https://x.test/icon.webp"}}');
  assert.equal(withBlog.banner_info.icon, "https://x.test/icon.webp");
  const official = createForm('{"leftbar":{"brand":{"image":{"src":"https://x.test/brand.webp"}}}}', "{}");
  assert.equal(official.banner_info.icon, "https://x.test/brand.webp");
  assert.equal(createForm("{}", "{}").banner_info.icon, "");
});

test("编辑横幅背景图写入 cover、清理同义 Blog 字段并保留未知字段", () => {
  const form = createForm(
    '{"title":"友链","cover":"https://x.test/cover.webp","unknown":{"keep":1}}',
    '{"content":{"banner_image":"https://x.test/blog.webp","keep":true}}',
  );
  assert.equal(form.banner, "https://x.test/blog.webp");
  form.banner = "https://x.test/new.webp";
  const config = json(form.stellar_config);
  const blog = json(form.blog_extensions);
  assert.equal(config.cover, "https://x.test/new.webp");
  assert.deepEqual(config.unknown, { keep: 1 });
  assert.equal(config.title, "友链");
  assert.equal(blog.content.banner_image, undefined);
  assert.equal(blog.content.keep, true);
  assert.equal(form.banner, "https://x.test/new.webp");
});

test("编辑侧栏品牌图片写 leftbar.brand.image.src 并清理旧 content.banner_icon", () => {
  const form = createForm("{}", '{"content":{"banner_icon":"https://x.test/old-icon.webp","keep":1}}');
  assert.equal(form.banner_info.icon, "https://x.test/old-icon.webp");
  form.banner_info.icon = "https://x.test/new-icon.webp";
  const config = json(form.stellar_config);
  const blog = json(form.blog_extensions);
  assert.equal(config.leftbar.brand.image.src, "https://x.test/new-icon.webp");
  assert.equal(blog.content.banner_icon, undefined);
  assert.equal(blog.content.keep, 1);
});

test("清空背景图与品牌图删除对应键而不是写入空串", () => {
  const form = createForm('{"cover":"https://x.test/cover.webp"}', "{}");
  form.banner = "";
  assert.equal("cover" in json(form.stellar_config), false);
  const branded = createForm('{"leftbar":{"brand":{"image":{"src":"https://x.test/brand.webp"}}}}', "{}");
  branded.banner_info.icon = "";
  assert.equal("src" in json(branded.stellar_config).leftbar.brand.image, false);
});

test("横幅标题、面包屑与日期仍写 front matter，未编辑字段原文保留", () => {
  const form = createForm(
    '{"banner":{"headline":"原横幅"},"breadcrumb":true,"date":"2026-08-21","unknown":"x"}',
    "{}",
  );
  assert.equal(form.banner_info.title, "原横幅");
  assert.equal(form.breadcrumb, true);
  assert.equal(form.date, "2026-08-21");
  form.banner_info.title = "新横幅";
  form.breadcrumb = false;
  form.date = "2025-01-02";
  const config = json(form.stellar_config);
  assert.equal(config.banner.headline, "新横幅");
  assert.equal(config.breadcrumb, false);
  assert.equal(config.date, "2025-01-02");
  assert.equal(config.unknown, "x");
});

test("原文缺失 date 时保留宿主列日期，不被回填清空", () => {
  const form = createForm("{}", "{}", { date: "2019-01-01" });
  assert.equal(form.date, "2019-01-01");
});

test("文章编辑器横幅仍使用 Blog 扩展 banner_image", () => {
  const banner = postFields.find((field) => field.form === "banner");
  assert.equal(banner?.path, "content.banner_image");
  assert.equal(banner?.blog, true);
});
