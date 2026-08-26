import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../web/src");
const read = (path: string) => readFileSync(join(ROOT, path), "utf8");

const tabs = [
  ["modules/settings/MenubarEditor.vue", "menubar", "添加菜单项"],
  ["modules/settings/CoverTab.vue", "cover", "添加按钮"],
  ["modules/settings/NavTabsTab.vue", "nav_tabs", "添加标签"],
  ["modules/settings/PluginsTab.vue", "plugins", null],
  ["modules/settings/AuthorsTab.vue", "authors", "添加作者"],
  ["modules/settings/WidgetsTab.vue", "widgets", null],
] as const;

test("设置子页的保存按钮使用共享高对比 VButton", () => {
  for (const [path, section] of tabs) {
    const view = read(path);
    assert.match(view, /import \{[^}]*VButton[^}]*\} from "@\/components\/ui"/);
    assert.match(view, new RegExp(`<VButton type="primary" size="sm"[\\s\\S]*saving === '${section}'`));
    assert.doesNotMatch(view, /const btnCls\s*=/);
  }
});

test("设置子页的添加类操作使用可见的次级 VButton", () => {
  for (const [path, , addLabel] of tabs) {
    if (!addLabel) continue;
    const view = read(path);
    assert.match(view, new RegExp(`<VButton type="secondary" size="sm"[\\s\\S]*${addLabel}`));
    assert.doesNotMatch(view, /const addBtnCls\s*=/);
  }
});

test("设置子页保留的原生图标按钮明确声明 button 类型", () => {
  for (const path of tabs.map(([file]) => file)) {
    const view = read(path);
    for (const match of view.matchAll(/<button([\s\S]*?)>/g)) {
      assert.match(match[1], /type="button"/, `${path} contains a native button without type=button`);
    }
  }
});
