import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../web/src");
const read = (path: string) => readFileSync(join(ROOT, path), "utf8");

test("AttachmentPickerModal 只分页图片附件且不会提交外层表单", () => {
  const view = read("components/AttachmentPickerModal.vue");
  assert.match(view, /params\.kind = "image"/);
  assert.match(view, /从附件库选择图片/);
  assert.match(view, /<button[\s\S]*?type="button"[\s\S]*?emit\('select'/);
  assert.match(view, /<button[\s\S]*?type="button"[\s\S]*?page--/);
});

test("图片 URL 字段接入附件库选择入口", () => {
  const iconPicker = read("components/IconPicker.vue");
  assert.match(iconPicker, /import AttachmentPickerModal/);
  assert.match(iconPicker, /title="从附件库选择图片"/);

  for (const path of [
    "modules/friends/FriendList.vue",
    "modules/pages/PageEditorSidebar.vue",
    "modules/users/UserListView.vue",
    "modules/wiki/WikiProjectsView.vue",
  ]) {
    assert.match(read(path), /AttachmentField/);
  }

  assert.match(read("modules/posts/EditorSidebar.vue"), /从附件库选择/);
  assert.match(read("modules/profile/ProfileView.vue"), /从附件库选择/);
});
