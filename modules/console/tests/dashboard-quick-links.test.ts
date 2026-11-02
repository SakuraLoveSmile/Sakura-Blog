import { test } from "node:test";
import assert from "node:assert/strict";
import { buildDashboardQuickLinks } from "../web/src/modules/dashboard/dashboard-quick-links.ts";

const nav = [
  { path: "/posts" },
  { path: "/comments" },
  { path: "/taxonomy" },
  { path: "/settings", permissions: ["system:manage"] },
  { path: "/appearance", permissions: ["system:manage"] },
];

test("管理员快捷入口包含写文章、评论、分类与设置", () => {
  const links = buildDashboardQuickLinks(nav, (permission) => permission === "content:write" || permission === "system:manage");
  assert.deepEqual(links.map((link) => link.label), ["写文章", "评论", "分类", "设置"]);
});

test("非管理员不显示设置，且无 content:write 时不显示写文章", () => {
  const author = buildDashboardQuickLinks(nav, (permission) => permission === "content:write");
  assert.deepEqual(author.map((link) => link.label), ["写文章", "评论", "分类"]);
  assert.equal(author.some((link) => link.path === "/settings"), false);

  const reader = buildDashboardQuickLinks(nav, () => false);
  assert.deepEqual(reader.map((link) => link.label), ["评论", "分类"]);
});

test("模块未启用时对应快捷入口消失", () => {
  const hidden = buildDashboardQuickLinks(
    nav.filter((item) => item.path !== "/comments"),
    () => true,
  );
  assert.equal(hidden.some((link) => link.path === "/comments"), false);
  assert.equal(hidden.some((link) => link.path === "/taxonomy"), true);
});
