import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../web/src");
const read = (path: string) => readFileSync(join(ROOT, path), "utf8");

test("AI 助手把助手与知识库合并为单一对话视图", () => {
  const code = read("modules/assistant/AssistantView.vue");
  // 标签栏只剩「对话」「设置」；知识库状态作为对话视图右侧面板，不再单独成页。
  assert.match(code, /aria-label="AI 助手分类"/);
  assert.match(code, />对话<\/button>/);
  assert.match(code, />设置<\/button>/);
  assert.doesNotMatch(code, /activeTab = 'knowledge'/);
  assert.doesNotMatch(code, />知识库<\/button>/);
  assert.match(code, /activeTab === 'chat'/);
  assert.match(code, /class="assistant-layout"/);
  assert.match(code, /知识库状态/);
});

test("旧的 knowledge / assistant 标签链接落到对话视图", () => {
  const code = read("modules/assistant/AssistantView.vue");
  assert.match(code, /route\.query\.tab === "settings" \? "settings" : "chat"/);
  assert.match(code, /activeTab\.value = tab === "settings" \? "settings" : "chat"/);
});
