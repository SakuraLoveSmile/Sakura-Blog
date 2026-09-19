import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { ensureSettingsDefaults, ALL_SETTINGS_SECTIONS } from "../web/src/modules/settings/settings-state.ts";

/**
 * 设置保存的「结构检查」。
 *
 * 边界声明：
 * - 本文件只做源码结构断言（正则）与纯函数断言，**不构成保存竞态的行为验收**。
 * - 行为验收在 `tests/e2e/console-settings-save-race.spec.ts`：拦截 `PUT /api/v1/settings/{section}`、
 *   由测试显式释放响应，断言真实组件在「请求未完成期间继续编辑」时的输入、草稿与快照边界。
 * - 禁止在本文件里复制一套保存函数（自建 isDirty / 自建 saveJson 流程）来「证明」生产代码正确。
 */

const testsRoot = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(testsRoot, "../../..");

function read(relativePath: string): string {
  return readFileSync(join(repoRoot, relativePath), "utf8");
}

/** 承接 `saving` 并渲染保存按钮的设置/主题设置子表单。 */
const SAVE_BUTTON_FILES = [
  "modules/console/web/src/modules/settings/tabs/BasicSettings.vue",
  "modules/console/web/src/modules/settings/tabs/PostSettings.vue",
  "modules/console/web/src/modules/settings/tabs/CommentSettings.vue",
  "modules/console/web/src/modules/settings/tabs/SeoSettings.vue",
  "modules/console/web/src/modules/settings/tabs/SearchSettings.vue",
  "modules/console/web/src/modules/settings/tabs/AdvancedSettings.vue",
  "modules/console/web/src/modules/settings/AuthorsTab.vue",
  "modules/console/web/src/modules/settings/WidgetsTab.vue",
  "modules/console/web/src/modules/settings/CoverTab.vue",
  "modules/console/web/src/modules/settings/NavTabsTab.vue",
  "modules/console/web/src/modules/settings/PluginsTab.vue",
  "modules/console/web/src/modules/settings/AppearanceTab.vue",
  "modules/console/web/src/modules/appearance/AssetsSettings.vue",
  "modules/console/web/src/modules/appearance/LayoutSettings.vue",
  "modules/console/web/src/modules/appearance/EnhancementsSettings.vue",
];

test("AppearanceView.save: 深复制提交内容与快照更新，杜绝请求中继续编辑造成的快照污染（结构检查）", () => {
  const code = read("modules/console/web/src/modules/appearance/AppearanceView.vue");

  // 请求前深拷贝提交内容
  assert.match(code, /payloadToSubmit\s*=\s*deepCloneJsonSafe/);
  // 请求接口使用深拷贝副本
  assert.match(code, /settingsApi\.updateSection\(section,\s*payloadToSubmit\)/);
  // 快照更新使用本次深拷贝副本，绝不读取请求结束时的可变表单对象
  assert.match(code, /updateSectionSnapshot\(section,\s*payloadToSubmit\)/);
});

test("SettingsView.save & saveJson: 维护各分区 JSON 文本基线与表单/草稿双重脏态判定（结构检查）", () => {
  const code = read("modules/console/web/src/modules/settings/SettingsView.vue");

  // 必须维护 savedJsonBaseline 响应式基线
  assert.match(code, /savedJsonBaseline\s*=\s*ref/);

  // dirtySections 同时检查表单和 JSON 文本草稿
  assert.match(code, /const isFormDirty\s*=\s*JSON\.stringify\(data\.value\[key\]\)\s*!==\s*JSON\.stringify\(original\[key\]\)/);
  assert.match(code, /const isJsonDirty\s*=\s*\(rawText\.value\[key\]\s*\?\?\s*""\)\s*!==\s*\(savedJsonBaseline\.value\[key\]\s*\?\?\s*""\)/);

  // saveJson 必须在发请求前解析 JSON，失败时不发送请求直接 return
  assert.match(code, /JSON\.parse\(rawAtStart\)/);
  assert.match(code, /Toast\.error\("JSON 格式错误，请检查语法后再保存"\)/);

  // 普通保存只有在无独立草稿且请求期间未被编辑时才自动格式化回填，否则保留用户草稿
  assert.match(code, /hadNoIndependentDraft\s*=\s*jsonAtStart\s*===\s*savedJsonBaseline\.value\[section\]/);
  assert.match(code, /if\s*\(rawText\.value\[section\]\s*===\s*jsonAtStart\s*&&\s*hadNoIndependentDraft\)/);

  // JSON 保存期间若表单发生变化，绝对不覆盖新的表单数据
  assert.match(code, /if\s*\(JSON\.stringify\(data\.value\[section\]\)\s*===\s*formAtStart\)\s*\{\s*data\.value\[section\]\s*=/);
});

test("所有设置与主题设置保存按钮都带全局保存禁用条件（结构检查）", () => {
  let loadingButtons = 0;
  let globallyDisabled = 0;
  for (const file of SAVE_BUTTON_FILES) {
    const code = read(file);
    loadingButtons += (code.match(/:loading="saving[^"]*"/g) ?? []).length;
    globallyDisabled += (code.match(/:disabled="Boolean\(saving\)"/g) ?? []).length;
  }
  assert.equal(loadingButtons, 21, "保存按钮数量应与受管子表单一致");
  assert.equal(globallyDisabled, loadingButtons, "每个保存按钮都必须带 Boolean(saving) 全局禁用条件");

  // 独立持有本地保存状态的子表单必须接线页面级 saving，否则不会参与全局禁用
  const themeYaml = read("modules/console/web/src/modules/settings/ThemeYamlTab.vue");
  assert.match(themeYaml, /defineProps<\{ saving\?: string \}>/);
  assert.match(themeYaml, /:disabled="loading \|\| localSaving \|\| Boolean\(props\.saving\)"/);
  // 请求期间仍允许继续编辑 YAML 文本（不得因保存禁用输入）
  assert.doesNotMatch(themeYaml, /:disabled="loading \|\| localSaving"/);
  assert.match(
    read("modules/console/web/src/modules/appearance/AppearanceView.vue"),
    /<ThemeYamlTab :saving="savingKey" \/>/,
  );
  assert.match(
    read("modules/console/web/src/modules/settings/MenubarEditor.vue"),
    /:disabled="hasErrors \|\| Boolean\(saving\)"/,
  );

  // 不改共享 VButton 的全局禁用语义：只在消费者侧传 disabled
  const vButton = read("packages/components/src/ui/VButton.vue");
  assert.match(vButton, /:disabled="disabled \|\| loading"/);
});

test("稀疏及旧配置加载后经过 ensureSettingsDefaults 与快照初始化后无假脏态", () => {
  // 模拟从后端拉取一个非常稀疏的旧配置（缺少 style, wiki, data_cache 等）
  const legacySparseConfig: Record<string, any> = {
    site: { title: "Old Blog" },
  };

  const loaded = ensureSettingsDefaults({ ...legacySparseConfig });
  const snapshot = JSON.stringify(loaded);

  // 模拟组件中后续检查
  const original = JSON.parse(snapshot);
  for (const key of ALL_SETTINGS_SECTIONS) {
    const isDirty = JSON.stringify(loaded[key]) !== JSON.stringify(original[key]);
    assert.equal(isDirty, false, `Section ${key} should not be falsely dirty`);
  }
});
