import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
import { backupApi, RestoreTaskHttpError } from "../../../packages/api-client/src/api/settings.ts";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../web/src");
const VIEW = readFileSync(join(ROOT, "modules/backup/BackupView.vue"), "utf8");

test("BackupView keeps restore controls busy for an active task and reuses its idempotency key", () => {
  assert.match(VIEW, /const restoreBusy = computed\([\s\S]*restoring\.value/);
  assert.match(VIEW, /if \(!p \|\| restoreBusy\.value/);
  assert.match(VIEW, /idempotencyKey: crypto\.randomUUID\(\)/);
  assert.match(VIEW, /p\.idempotencyKey/);
  assert.doesNotMatch(VIEW, /const idempotencyKey = crypto\.randomUUID\(\)/);
});

test("BackupView gates admin task polling behind public maintenance state", () => {
  const maintenanceRead = VIEW.indexOf("await readMaintenanceState()");
  const adminTaskRead = VIEW.indexOf("backupApi.getRestoreTask(taskID)");
  assert.ok(maintenanceRead >= 0);
  assert.ok(adminTaskRead > maintenanceRead);
  assert.match(VIEW, /componentUnmounted\.value/);
  assert.match(VIEW, /if \(componentUnmounted\.value \|\| !localStorage\.getItem/);
});

test("restore task fetch preserves a 401 for the backup-account recovery flow", async () => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async () => new Response(JSON.stringify({ error: "未认证" }), { status: 401 });
  try {
    await assert.rejects(backupApi.getRestoreTask("task-1"), (error: unknown) => {
      return error instanceof RestoreTaskHttpError && error.status === 401;
    });
  } finally {
    globalThis.fetch = originalFetch;
  }
});
