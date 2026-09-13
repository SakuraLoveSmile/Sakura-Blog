import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import {
  ensureSettingsDefaults,
  deepCloneJsonSafe,
  ALL_SETTINGS_SECTIONS,
} from "../web/src/modules/settings/settings-state.ts";

const testsRoot = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(testsRoot, "../../..");

function read(relativePath: string): string {
  return readFileSync(join(repoRoot, relativePath), "utf8");
}

test("AppearanceView.save: 深复制提交内容与快照更新，杜绝在请求中继续编辑时的快照污染", () => {
  const code = read("modules/console/web/src/modules/appearance/AppearanceView.vue");

  // 请求前深拷贝提交内容
  assert.match(code, /payloadToSubmit\s*=\s*deepCloneJsonSafe/);
  // 请求接口使用深拷贝副本
  assert.match(code, /settingsApi\.updateSection\(section,\s*payloadToSubmit\)/);
  // 快照更新使用本次深拷贝副本，绝不读取请求结束时的可变表单对象
  assert.match(code, /updateSectionSnapshot\(section,\s*payloadToSubmit\)/);
});

test("SettingsView.save & saveJson: 维护各分区 JSON 文本基线与表单/草稿双重脏态判定", () => {
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

test("保存竞态模拟：提交 A 且响应前表单改为 B，响应后快照为 A、表单保持 B，脏状态依然保持", async () => {
  // 模拟初始状态
  const data: Record<string, any> = {
    site: { title: "Title A", subtitle: "Initial" },
  };
  let savedSnapshot = JSON.stringify(data);

  function isDirty() {
    const original = JSON.parse(savedSnapshot);
    return JSON.stringify(data.site) !== JSON.stringify(original.site);
  }

  assert.equal(isDirty(), false);

  // 用户点击保存，捕获提交时的快照副本
  const payloadToSubmit = deepCloneJsonSafe(data.site);

  // 模拟异步请求延迟中，用户将界面表单修改为 Title B
  data.site.title = "Title B";

  // 模拟 API 保存完成，只更新 payloadToSubmit 的快照
  const snap = JSON.parse(savedSnapshot);
  snap.site = deepCloneJsonSafe(payloadToSubmit);
  savedSnapshot = JSON.stringify(snap);

  // 验证：当前表单内容保留了用户在请求中编辑的 Title B，并没有被回滚为 Title A
  assert.equal(data.site.title, "Title B");
  // 验证：因为快照更新为 Title A，而表单是 Title B，脏状态保持为 true（未保存黄点不丢失）
  assert.equal(isDirty(), true);

  // 用户再次保存 B 成功
  const secondSubmit = deepCloneJsonSafe(data.site);
  const snap2 = JSON.parse(savedSnapshot);
  snap2.site = deepCloneJsonSafe(secondSubmit);
  savedSnapshot = JSON.stringify(snap2);

  // 此时未保存状态才消除
  assert.equal(isDirty(), false);
});

test("JSON 草稿未保存判定：文本格式修改、无效 JSON、以及独立草稿均能正确触发脏状态", () => {
  const baseline = JSON.stringify({ enable: true }, null, 2);
  let rawText = baseline;

  function isJsonDirty() {
    return rawText !== baseline;
  }

  assert.equal(isJsonDirty(), false);

  // 格式修改（哪怕语义相同，也是未保存文本修改）
  rawText = '{\n  "enable":  true\n}';
  assert.equal(isJsonDirty(), true);

  // 无效 JSON（语法错误）同样保持未保存脏状态
  rawText = '{\n  "enable": true,,\n}';
  assert.equal(isJsonDirty(), true);

  // 恢复一致
  rawText = baseline;
  assert.equal(isJsonDirty(), false);
});

test("JSON 保存期间表单发生变化的保护：不覆盖用户新修改的表单内容", () => {
  const section = "site";
  const form = { title: "Title 1", description: "Desc 1" };
  let savedSnapshot = JSON.stringify({ [section]: deepCloneJsonSafe(form) });

  let rawText = JSON.stringify(form, null, 2);
  let savedBaseline = rawText;

  // 用户在 JSON 编辑器里把 title 修改为 Title JSON
  rawText = JSON.stringify({ title: "Title JSON", description: "Desc 1" }, null, 2);

  // 用户点击保存 JSON：记录开始时表单状态
  const rawAtStart = rawText;
  const formAtStart = JSON.stringify(form);
  const payloadToSubmit = deepCloneJsonSafe(JSON.parse(rawAtStart));

  // 请求进行中，用户在结构化表单修改了 description 为 "Desc Form Edited"
  form.description = "Desc Form Edited";

  // 请求完成，根据规范更新：
  // 1. 快照更新为本次提交规范化副本
  const normalized = deepCloneJsonSafe(payloadToSubmit);
  const snap = JSON.parse(savedSnapshot);
  snap[section] = normalized;
  savedSnapshot = JSON.stringify(snap);

  // 2. 文本若未二次编辑，同步基线
  if (rawText === rawAtStart) {
    savedBaseline = rawText;
  }

  // 3. 表单检查：若与 formAtStart 不一致，绝对不覆盖！
  if (JSON.stringify(form) === formAtStart) {
    Object.assign(form, normalized);
  }

  // 断言：用户的表单编辑 "Desc Form Edited" 完好保留！没有被覆盖
  assert.equal(form.description, "Desc Form Edited");

  // 断言：由于表单是 { title: "Title 1", description: "Desc Form Edited" }，
  // 快照是 { title: "Title JSON", description: "Desc 1" }，脏状态准确为 true！
  const currentSnap = JSON.parse(savedSnapshot);
  const isFormDirty = JSON.stringify(form) !== JSON.stringify(currentSnap[section]);
  assert.equal(isFormDirty, true);
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
