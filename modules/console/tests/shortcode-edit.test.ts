import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../web/src");
const read = (path: string) => readFileSync(join(ROOT, path), "utf8");

test("shortcode visual editor uses the API parser and existing catalog form", () => {
  const apiSource = readFileSync(join(ROOT, "../../../../packages/api-client/src/api/plugins.ts"), "utf8");
  const viewSource = read("components/editor/StellarTagView.vue");
  const popoverSource = read("components/editor/StellarTagPropertyPopover.vue");
  const formSource = read("modules/plugins/PluginParamForm.vue");

  assert.match(apiSource, /parseShortcode: \(raw: string\).*plugins\/shortcode\/parse/);
  assert.match(viewSource, /await pluginApi\.parseShortcode\(sourceRaw\)/);
  assert.match(viewSource, /data\.editable/);
  assert.match(viewSource, /props\.updateAttributes\(\{ raw: next \}\)/);
  assert.match(popoverSource, /<PluginParamForm/);
  assert.match(popoverSource, /:initial-values="initialValues"/);
  assert.match(popoverSource, /setTimeout\(\(\) =>/);
  assert.match(popoverSource, /}, 200\)/);
  assert.match(popoverSource, /pointerdown/);
  assert.match(popoverSource, /event\.key === "Escape"/);
  assert.match(popoverSource, /Shortcode 源码/);
  assert.match(formSource, /initialValues\?: Record<string, unknown>/);
  assert.match(formSource, /initial !== undefined/);
});
