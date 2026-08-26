import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../web/src");
const read = (path: string) => readFileSync(join(ROOT, path), "utf8");

test("编辑器插入菜单位于横向滚动容器之外", () => {
  const toolbar = read("modules/posts/EditorToolbar.vue");
  const insertStart = toolbar.indexOf('class="editor-toolbar-insert');
  const scrollStart = toolbar.indexOf('class="editor-toolbar-scroll');

  assert.ok(insertStart >= 0, "缺少独立的插入工具区域");
  assert.ok(scrollStart > insertStart, "横向滚动区域应位于插入工具区域之后");

  const insertRegion = toolbar.slice(insertStart, scrollStart);
  assert.match(insertRegion, /ref="dropdownRef"/);
  assert.match(insertRegion, /v-if="insertOpen" class="absolute left-0 top-full z-30/);
  assert.doesNotMatch(insertRegion, /overflow-x-auto/);
  assert.match(toolbar, /class="editor-toolbar-scroll[^"]*overflow-x-auto"/);
});

test("页面编辑器顶部导航栏启用滚动隐藏", () => {
  const pageEditor = read("modules/pages/PageEditor.vue");

  assert.match(pageEditor, /<VPageHeader[^>]*:hide-on-scroll="true"/);
  assert.doesNotMatch(pageEditor, /<VPageHeader[^>]*:hide-on-scroll="false"/);
});
