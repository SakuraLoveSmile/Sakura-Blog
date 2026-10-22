import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import ts from "../web/node_modules/typescript/lib/typescript.js";
import { computed, ref, watch, effectScope, nextTick } from "../web/node_modules/vue/index.js";
import { patchJSONField, readJSONField } from "../web/src/components/editor/stellar-form-json.ts";

const sidebar = readFileSync(new URL("../web/src/modules/posts/EditorSidebar.vue", import.meta.url), "utf8");
const editor = readFileSync(new URL("../web/src/modules/posts/PostEditor.vue", import.meta.url), "utf8");

// 执行生产 SFC 的主题选择/快照片段，不复制另一套同步或保存实现。
function compile(source: string, names: string[], values: unknown[]) {
  const js = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText;
  return new Function(...names, js)(...values);
}

function topicHarness(form: any, list: () => Promise<unknown>) {
  const source = sidebar.slice(sidebar.indexOf("const topicOptions ="), sidebar.indexOf("// ---------- 大纲"));
  return compile(source + "\nreturn {selectedTopic, loadTopics, selectTopic, clearTopic};", [
    "props", "ref", "computed", "watch", "onMounted", "topicApi", "readJSONField", "patchJSONField", "Toast",
  ], [{ form }, ref, computed, watch, () => {}, { list }, readJSONField, patchJSONField, { error: assert.fail }]);
}

function payloadHarness(form: any) {
  const source = editor.slice(editor.indexOf("function snapshotForm("), editor.indexOf("async function flushPendingChanges("));
  return compile(source + "\nreturn snapshotForm;", ["form"], [{ value: form }]);
}

const raw = ' {"collection":{"profile":"topic","id":"port-journal"},"custom":{"integer":9007199254740993}} ';
const current = { id: 1, slug: "port-journal", name: "移植手记 · 已改名" };

test("strict topic display waits for a late catalog without mutating saved form or raw content", async () => {
  const scope = effectScope();
  try {
    const form = ref({ stellar_version: 2, stellar_config: raw, topic_id: null, topic: "", content_md: "原文\n{% unknown x %}" });
    const baseline = JSON.stringify(form.value);
    let release!: (data: unknown) => void;
    const response = new Promise((resolve) => { release = resolve; });
    const ui = scope.run(() => topicHarness(form.value, () => response));
    const pending = ui.loadTopics();
    assert.equal(ui.selectedTopic.value, null);
    assert.equal(JSON.stringify(form.value), baseline);
    release({ data: { items: [current] } });
    await pending;
    await nextTick();
    assert.equal(ui.selectedTopic.value?.name, current.name);
    assert.equal(JSON.stringify(form.value), baseline, "异步目录不得把派生 ID/名称写入保存基线");
    form.value.stellar_config = '{"collection":{"profile":"wiki","id":"port-journal"}}';
    await nextTick();
    assert.equal(ui.selectedTopic.value, null, "其他 profile 不能按相同 ID 显示为专栏");
    assert.equal(form.value.topic_id, null);
    assert.equal(form.value.topic, "");
  } finally { scope.stop(); }
});

test("only explicit topic select/clear patches canonical membership, preserving unrelated JSON and body", async () => {
  const scope = effectScope();
  try {
    const form = ref({ stellar_version: 2, stellar_config: raw, topic_id: 9, topic: "过期名称", content_md: "原文" });
    const ui = scope.run(() => topicHarness(form.value, async () => ({ data: { items: [current] } })));
    await ui.loadTopics();
    assert.equal(ui.selectedTopic.value?.id, current.id, "严格归属优先于过期列");
    const newTopic = { id: 2, slug: "second", name: "另一个专栏" };
    ui.selectTopic(newTopic);
    assert.equal(readJSONField(form.value.stellar_config, "collection.profile"), "topic");
    assert.equal(readJSONField(form.value.stellar_config, "collection.id"), "second");
    assert.match(form.value.stellar_config, /9007199254740993/);
    assert.equal(form.value.topic_id, 9);
    assert.equal(form.value.topic, "过期名称");
    ui.clearTopic();
    assert.equal(readJSONField(form.value.stellar_config, "collection"), undefined);
    assert.match(form.value.stellar_config, /9007199254740993/);
    assert.equal(form.value.content_md, "原文");
  } finally { scope.stop(); }
});

test("strict save/preview snapshot omits derived topic fields before catalog load and detects canonical edits", () => {
  const form = { stellar_version: 2, stellar_config: raw, topic_id: null, topic: "过期草稿名", title: "技术文章", content_md: "原文" };
  const snapshot = payloadHarness(form);
  const saved = snapshot();
  assert.equal("topic_id" in saved, false);
  assert.equal("topic" in saved, false);
  assert.equal(saved.stellar_config, raw);
  assert.equal(saved.content_md, form.content_md);
  form.topic_id = 1;
  form.topic = current.name;
  assert.deepEqual(snapshot(), saved, "派生列变化不得制造 dirty 或影响请求");
  form.stellar_config = patchJSONField(raw, "collection", { profile: "topic", id: "second" });
  assert.notDeepEqual(snapshot(), saved, "用户实际归属编辑仍必须产生 dirty");
  form.stellar_version = 0;
  assert.equal(snapshot().topic_id, 1, "旧模式字段仍按原契约保存");
  assert.equal(snapshot().topic, current.name);
});

test("post dirty baseline and acceptance use the same canonical snapshot as preview/save", () => {
  assert.match(editor, /richPending\.value \|\| JSON\.stringify\(snapshotForm\(\)\) !== savedSnapshot\.value/);
  assert.doesNotMatch(editor, /savedSnapshot\.value = JSON\.stringify\(form\.value\)/);
  assert.match(editor, /savedSnapshot\.value = JSON\.stringify\(snapshotForm\(\{ \.\.\.submitted, status \}\)\)/);
});
