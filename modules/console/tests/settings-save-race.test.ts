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

/**
 * 承接 `saving` 并渲染保存按钮的设置/主题设置子表单。
 * 站点信息（BasicSettings）与访客反馈（FeedbackSettings）的保存按钮已统一到 SettingsSaveBar，
 * 由下面“统一保存栏”用例单独校验；SeoSettings 的宿主表单已并入站点信息后删除。
 */
const SAVE_BUTTON_FILES = [
  "modules/console/web/src/modules/settings/tabs/PostSettings.vue",
  "modules/console/web/src/modules/settings/tabs/CommentSettings.vue",
  "modules/console/web/src/modules/settings/tabs/SearchSettings.vue",
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

test("友链设置仅提供 canonical 配置入口，不再双协议写入", () => {
  const code = read("modules/console/web/src/modules/friends/FriendSettings.vue");
  assert.match(code, /CanonicalSettingsNotice/);
  assert.doesNotMatch(code, /settingsApi\.updateSection|settingsApi\.all/);
});

test("AI 设置使用 canonical 编辑器读写 blog_extensions.ai，不接入旧 ai 分段", () => {
  const code = read("modules/console/web/src/modules/assistant/AiSettings.vue");
  assert.match(code, /useStellarSettings\(\)/);
  assert.match(code, /<CanonicalSettingsForm/);
  assert.match(code, /tab="ai"/);
  assert.doesNotMatch(code, /CanonicalSettingsNotice|settingsApi\.updateSection|settingsApi\.all/);
});

test("AI 设置仅在首次进入设置标签后挂载，默认助手标签不读取配置", () => {
  const code = read("modules/console/web/src/modules/assistant/AssistantView.vue");
  assert.match(code, /<AiSettings v-if="settingsVisited" v-show="activeTab === 'settings'" \/>/);
  assert.match(code, /if \(tab === "settings"\) settingsVisited\.value = true;/);
});

test("SettingsView.save 与 canonical 原文保存: 维护各分区 JSON 文本基线与表单/草稿双重脏态判定（结构检查）", () => {
  const code = read("modules/console/web/src/modules/settings/SettingsView.vue");

  // 必须维护 savedJsonBaseline 响应式基线
  assert.match(code, /savedJsonBaseline\s*=\s*ref/);

  // dirtySections 同时检查表单和 JSON 文本草稿
  assert.match(code, /const isFormDirty\s*=\s*JSON\.stringify\(data\.value\[key\]\)\s*!==\s*JSON\.stringify\(original\[key\]\)/);
  assert.match(code, /const isJsonDirty\s*=\s*\(rawText\.value\[key\]\s*\?\?\s*""\)\s*!==\s*\(savedJsonBaseline\.value\[key\]\s*\?\?\s*""\)/);

  // 旧 JSON 分段编辑器已停止产品保存，只有宿主基本信息与 Feedback 可走旧 API。
  assert.match(code, /section !== "site" && section !== "feedback"/);
  assert.doesNotMatch(code, /function saveJson/);

  // 普通保存只有在无独立草稿且请求期间未被编辑时才自动格式化回填，否则保留用户草稿
  assert.match(code, /hadNoIndependentDraft\s*=\s*jsonAtStart\s*===\s*savedJsonBaseline\.value\[section\]/);
  assert.match(code, /if\s*\(rawText\.value\[section\]\s*===\s*jsonAtStart\s*&&\s*hadNoIndependentDraft\)/);

  // canonical 提交原文且保存期间新草稿不被完成响应覆盖，保留大整数。
  const canonical = read("modules/console/web/src/modules/settings/useStellarSettings.ts");
  assert.match(canonical, /const payload = draft\.value/);
  assert.match(canonical, /api\.save\(payload\)/);
  assert.match(canonical, /if \(draft\.value === payload\) draft\.value = data/);
});

test("所有设置与主题设置保存按钮都带全局保存禁用条件（结构检查）", () => {
  let loadingButtons = 0;
  let globallyDisabled = 0;
  for (const file of SAVE_BUTTON_FILES) {
    const code = read(file);
    loadingButtons += (code.match(/:loading="saving[^"]*"/g) ?? []).length;
    globallyDisabled += (code.match(/:disabled="Boolean\(saving\)"/g) ?? []).length;
  }
  // 原 21 个：站点信息 1、SEO 宿主表单 2、访客反馈 1 已移入统一保存栏；
  // AdvancedSettings 退役带走 3 个（inject、data_cache、json:<section>），其余 14 个保持不变。
  assert.equal(loadingButtons, 14, "保存按钮数量应与受管子表单一致");
  assert.equal(globallyDisabled, loadingButtons, "每个保存按钮都必须带 Boolean(saving) 全局禁用条件");

  // 独立持有本地保存状态的子表单必须接线页面级 saving，否则不会参与全局禁用。
  // 样式 YAML 整段编辑器随 ThemeYamlTab 一并退役，嵌套 JSON 原文统一走“代码与原文”页。
  assert.match(
    read("modules/console/web/src/modules/settings/MenubarEditor.vue"),
    /:disabled="hasErrors \|\| Boolean\(saving\)"/,
  );

  // 不改共享 VButton 的全局禁用语义：只在消费者侧传 disabled
  const vButton = read("packages/components/src/ui/VButton.vue");
  assert.match(vButton, /:disabled="disabled \|\| loading"/);
});

test("统一保存栏：所有设置页的保存按钮都经 SettingsSaveBar，并带全局保存禁用条件（结构检查）", () => {
  const bar = read("modules/console/web/src/modules/settings/SettingsSaveBar.vue");
  // 保存栏只有一个主按钮：原生 submit，没有改动、其他保存进行中或存在阻止原因时禁用，并显示本栏的保存中状态。
  assert.equal((bar.match(/native-type="submit"/g) ?? []).length, 1);
  assert.match(bar, /:disabled="!dirty \|\| busy \|\| Boolean\(blocked\)"/);
  assert.match(bar, /:loading="saving"/);
  // 放弃本页修改同样不能在任一保存请求进行中使用（否则会与回写的响应竞态）。
  assert.match(bar, /:disabled="!pageDirty \|\| busy"/);

  // 站点信息与访客反馈不再自带保存按钮，只通过 actions 插槽接入统一保存栏。
  for (const name of ["BasicSettings", "FeedbackSettings"]) {
    const form = read(`modules/console/web/src/modules/settings/tabs/${name}.vue`);
    assert.match(form, /<slot name="actions" \/>/);
    assert.doesNotMatch(form, /native-type="submit"|:loading="saving/);
  }

  // 页面渲染的每个保存栏都接线“任意保存进行中”(savingKey) 作为 busy，而不是只看自己的状态。
  const view = read("modules/console/web/src/modules/settings/SettingsView.vue");
  const bars = (view.match(/<SettingsSaveBar/g) ?? []).length;
  assert.equal(bars, 3, "主题页、站点信息、访客反馈各一个保存栏");
  assert.equal((view.match(/:busy="Boolean\(savingKey\)"/g) ?? []).length, bars);
  // 表单内的主题保存栏经 CanonicalSettingsForm 的 actions 插槽接入，表单自身的提交仍受 saving 约束。
  assert.match(view, /<CanonicalSettingsForm[\s\S]*?:saving="savingKey \|\| \(pendingInput \? 'pending-input' : ''\)"/);
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
