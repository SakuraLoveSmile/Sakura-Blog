import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../web");
const read = (path: string) => readFileSync(join(ROOT, path), "utf8");
const readApi = (path: string) => readFileSync(join(ROOT, "../../../packages/api-client/src", path), "utf8");

test("User Center has a separate entry, history base, and UC API contract", () => {
  assert.match(read("uc.html"), /src="\/src\/uc\/main\.ts"/);
  assert.match(read("vite.uc.config.ts"), /base: "\/uc\/"/);
  assert.match(read("src/uc/router.ts"), /createWebHistory\("\/uc\/"\)/);
  assert.match(readApi("api/uc.ts"), /\/uc\/posts/);
  assert.match(readApi("api/uc.ts"), /\/uc\/attachments/);
});

test("User Center keeps authors in a restricted writing surface", () => {
  const router = read("src/uc/router.ts");
  const consoleRouter = read("src/router/index.ts");
  const editor = read("src/uc/UCPostEditorView.vue");
  const store = read("src/uc/stores/auth.ts");
  assert.match(router, /requiresAuthor/);
  assert.match(router, /\/upgrade/);
  assert.match(consoleRouter, /window\.location\.assign\("\/uc\/"\)/);
  assert.match(editor, /受限编辑器/);
  assert.doesNotMatch(editor, /settingsApi/);
  assert.match(store, /invitation_code/);
  assert.doesNotMatch(store, /captcha_|register\/challenge/);
  assert.match(store, /\/api\/auth\/upgrade/);
});
