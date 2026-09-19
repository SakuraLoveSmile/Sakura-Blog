import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const testsRoot = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(testsRoot, "../../..");

function read(relativePath: string): string {
  return readFileSync(join(repoRoot, relativePath), "utf8");
}

test("Console scans shared component sources for Tailwind utilities", () => {
  const mainCss = read("modules/console/web/src/styles/main.css");
  assert.match(mainCss, /@source\s+"\.\.\/\.\.\/\.\.\/\.\.\/\.\.\/packages\/components\/src";/);
});

test("shared VSwitch keeps the rendered dimensions required by the settings UI", () => {
  const switchComponent = read("packages/components/src/ui/VSwitch.vue");
  assert.match(switchComponent, /h-\[31px\]/);
  assert.match(switchComponent, /w-\[51px\]/);
  assert.match(switchComponent, /h-\[27px\]/);
  assert.match(switchComponent, /w-\[27px\]/);
});
