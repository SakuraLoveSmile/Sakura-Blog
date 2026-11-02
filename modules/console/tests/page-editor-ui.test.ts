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
  assert.match(insertRegion, /v-if="insertOpen"/);
  assert.match(insertRegion, /class="absolute left-0 top-full z-30/);
  assert.doesNotMatch(insertRegion, /overflow-x-auto/);
  assert.match(toolbar, /class="editor-toolbar-scroll[^"]*overflow-x-auto"/);
});

test("文章专用右侧插槽位于横向滚动区之外，且默认侧栏开关保留", () => {
  const toolbar = read("modules/posts/EditorToolbar.vue");
  const scrollStart = toolbar.indexOf('class="editor-toolbar-scroll');
  const endStart = toolbar.indexOf('class="editor-toolbar-end');

  assert.ok(scrollStart >= 0, "缺少工具栏横向滚动区");
  assert.ok(endStart > scrollStart, "文章专用右侧插槽应位于滚动区之后");

  const scrollRegion = toolbar.slice(scrollStart, endStart);
  assert.match(scrollRegion, /v-if="!slots\.end"/, "未提供 #end 时应保留默认侧栏开关");
  assert.doesNotMatch(scrollRegion, /editor-toolbar-end/);
  assert.match(toolbar.slice(endStart), /<slot name="end" \/>/);

  const postEditor = read("modules/posts/PostEditor.vue");
  const slotStart = postEditor.indexOf("<template #end>");
  assert.ok(slotStart > 0, "文章页应注入工具栏 #end");
  const slot = postEditor.slice(slotStart, postEditor.indexOf("</template>", slotStart));
  assert.match(slot, /editor-kind-picker/);
  assert.match(slot, /editor-sidebar-toggle/);
  assert.match(slot, /ref="mobileSidebarTrigger"/);
  assert.doesNotMatch(slot, /editor-toolbar-scroll/);
});

test("文章页只保留一个并入页头的保存状态 live region", () => {
  const postEditor = read("modules/posts/PostEditor.vue");

  assert.equal(postEditor.match(/class="editor-save-status/g)?.length, 1);
  assert.equal(postEditor.match(/role="status"/g)?.length, 1);
  assert.match(postEditor, /role="status"[\s\S]{0,200}aria-live="polite"/);
  assert.doesNotMatch(postEditor, /editor-status-bar/);
});

test("页面编辑器不注入文章专用工具栏插槽", () => {
  const pageEditor = read("modules/pages/PageEditor.vue");

  assert.match(pageEditor, /<EditorToolbar/);
  assert.doesNotMatch(pageEditor, /<template #end>/);
  assert.doesNotMatch(pageEditor, /#end/);
});

test("文章编辑页将设置字段分组改为单层结构", () => {
  const sidebar = read("modules/posts/EditorSidebar.vue");
  const theme = read("components/editor/PostThemeSettings.vue");

  assert.match(sidebar, /const groupCls = "border-b border-border py-5 last:border-b-0"/);
  assert.doesNotMatch(sidebar, /rounded-\[14px\] border border-border bg-card/);
  assert.match(theme, /class="min-w-0 border-b border-border py-5 last:border-b-0"/);
  assert.doesNotMatch(theme, /rounded-\[14px\] border border-border bg-card/);
});

test("文章页局部去掉编辑器实例的卡片背景与圆角", () => {
  const postEditor = read("modules/posts/PostEditor.vue");

  assert.equal(postEditor.match(/class="post-editor-instance !border-0"/g)?.length, 2);
  assert.match(postEditor, /\.post-editor-body :deep\(\.post-editor-instance\)/);
  assert.match(postEditor, /background: transparent;/);
  assert.match(postEditor, /border-radius: 0;/);
});

test("页面编辑器顶部导航栏启用滚动隐藏", () => {
  const pageEditor = read("modules/pages/PageEditor.vue");

  assert.match(pageEditor, /<VPageHeader[^>]*:hide-on-scroll="true"/);
  assert.doesNotMatch(pageEditor, /<VPageHeader[^>]*:hide-on-scroll="false"/);
});

test("文章与页面编辑器共享 Markdown 导入按钮和确认链路", () => {
  const toolbar = read("modules/posts/EditorToolbar.vue");
  const postEditor = read("modules/posts/PostEditor.vue");
  const pageEditor = read("modules/pages/PageEditor.vue");
  const preview = read("components/editor/MarkdownImportPreview.vue");
  const api = read("../../../../packages/api-client/src/api/editor.ts");

  assert.match(toolbar, /accept="\.md,\.markdown,text\/markdown,text\/plain"/);
  assert.match(toolbar, /emit\("import", file, legacyImport\.value\)/);
  assert.match(toolbar, /chooseImportFile\(false\)/);
  assert.match(toolbar, /chooseImportFile\(true\)/);
  assert.match(postEditor, /kind: "post"/);
  assert.match(pageEditor, /kind:\s*"page"/);
  for (const editor of [postEditor, pageEditor]) {
    assert.match(editor, /<MarkdownImportPreview :data="pendingImport"/);
    assert.match(editor, /@confirm="confirmImport"/);
    assert.match(editor, /pendingImport\.value = data/);
  }
  assert.match(preview, /审查 1\.x 转换预览/);
  assert.match(preview, /不会自动保存或发布/);
  assert.match(api, /\/editor\/import\/stellar/);
});
