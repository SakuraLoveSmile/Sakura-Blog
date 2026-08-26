import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const DIALOG = readFileSync(
  join(dirname(fileURLToPath(import.meta.url)), "../../../packages/components/src/ui/dialog.ts"),
  "utf8",
);

test("命令式确认弹窗始终有紧凑的最大宽度", () => {
  assert.match(DIALOG, /class: "relative w-full overflow-hidden p-5"/);
  assert.match(DIALOG, /maxWidth: "24rem"/);
});
