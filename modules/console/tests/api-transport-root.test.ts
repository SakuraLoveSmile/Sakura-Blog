import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../web/src");
const MAIN = readFileSync(join(ROOT, "main.ts"), "utf8");

test("root transport policy: deduplicates expired-login dialog before redirect", () => {
  assert.match(MAIN, /let unauthorizedDialogOpen = false/);
  assert.match(MAIN, /if \(router\.currentRoute\.value\.path === "\/login" \|\| unauthorizedDialogOpen\)/);
  assert.match(MAIN, /Dialog\.warning\(/);
  assert.match(MAIN, /title: "登录已过期"/);
  assert.match(MAIN, /onConfirm: async/);
  assert.match(MAIN, /router\.replace\(\{ path: "\/login"/);
  assert.match(MAIN, /onCancel: \(\) =>/);
});

test("root transport policy: installs a global normalized error toast callback", () => {
  assert.match(MAIN, /onError: \(_error, message\) => Toast\.error\(message\)/);
});
