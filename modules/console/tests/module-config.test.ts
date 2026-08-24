import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../web/src");
const WORKTREE = join(dirname(fileURLToPath(import.meta.url)), "../../..");
const read = (path: string) => readFileSync(join(ROOT, path), "utf8");

test("storage module configuration is manifest-driven and rendered with SchemaForm", () => {
  const module = readFileSync(join(dirname(fileURLToPath(import.meta.url)), "../../../internal/module/storage/storage.go"), "utf8");
  const view = read("modules/modules/ModulesView.vue");
  const form = read("components/forms/SchemaForm.vue");

  assert.match(module, /ConfigSection:\s*settingKey/);
  assert.match(module, /ConfigSchema:/);
  assert.match(module, /secret_key/);
  assert.match(view, /moduleApi\.getConfig/);
  assert.match(view, /moduleApi\.putConfig/);
  assert.match(view, /<SchemaForm/);
  assert.match(form, /field\.type === 'password'/);
});

test("module API exposes config-section reads and writes", () => {
  const api = readFileSync(join(WORKTREE, "packages/api-client/src/api/plugins.ts"), "utf8");
  assert.match(api, /getConfig:.*settings/);
  assert.match(api, /putConfig:[\s\S]*settings/);
});
