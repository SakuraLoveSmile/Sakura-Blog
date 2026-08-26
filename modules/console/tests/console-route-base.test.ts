import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../web/src");
const read = (path: string) => readFileSync(join(ROOT, path), "utf8");

test("评论页面内部跳转使用 Console 路由基准下的相对路径", () => {
  const comments = read("modules/comments/CommentList.vue");
  const users = read("modules/comments/CommentUsersView.vue");

  assert.match(comments, /<VButton :route="\{ path: '\/comment-users' \}">/);
  assert.match(users, /<VButton :route="\{ path: '\/comments' \}">/);
  assert.doesNotMatch(comments, /path: '\/console\//);
  assert.doesNotMatch(users, /path: '\/console\//);
});

test("评论用户页兼容历史重复 Console 前缀地址", () => {
  const core = read("modules/core/module.ts");
  assert.match(core, /path: "console\/comment-users", redirect: "\/comment-users"/);
});
