// 分页器必须保持 Stellar 原版的直接子元素结构。
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..");

for (const file of [
  "web/templates/partial/index.html",
  "web/templates/partial/moments.html",
  "web/templates/partial/author.html",
]) {
  test(`${file} uses direct Stellar paginator links`, () => {
    const template = readFileSync(join(ROOT, file), "utf8");

    assert.match(
      template,
      /<div class="paginator-wrap dis-select">[\s\S]*?<a class="(?:extend prev|page-number)/,
      "paginator links should be direct children of the wrapper",
    );
    assert.doesNotMatch(template, /<nav class="paginator">/, "paginator must not add a wrapper nav");
  });
}
