// 分页器必须保持 Stellar 原版的直接子元素结构。
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..");
const scrollReveal = readFileSync(
  join(ROOT, "web/assets/js/plugins/scrollreveal-init.js"),
  "utf8",
);

test("ScrollReveal initializes every Stellar reveal target", () => {
  assert.match(
    scrollReveal,
    /document\.querySelector\('\.post-card, \.slide-up'\)/,
    "ScrollReveal should initialize when a page has only a slide-up target",
  );
  assert.match(
    scrollReveal,
    /sr\.reveal\('\.post-card, \.slide-up'/,
    "ScrollReveal should reveal paginator slide-up targets",
  );
  assert.match(
    scrollReveal,
    /document\.documentElement\.classList\.add\('sr-fallback'\)/,
    "reduced-motion or loader failure should keep slide-up content visible",
  );
});

for (const file of [
  "web/templates/partial/index.html",
  "web/templates/partial/moments.html",
  "web/templates/partial/author.html",
]) {
  test(`${file} uses Stellar paginator markup`, () => {
    const template = readFileSync(join(ROOT, file), "utf8");

    assert.match(
      template,
      /<div class="paginator-wrap dis-select\{\{if and \.Site\.Plugins\.ScrollReveal \.Site\.Plugins\.ScrollReveal\.Enable\}\} slide-up\{\{end\}\}">/,
      "paginator wrapper should add Stellar's scroll-reveal class only when enabled",
    );
    assert.match(
      template,
      /<a class="extend prev" rel="prev"[\s\S]*?<span class="extend prev" rel="prev">/,
      "paginator should render a linked previous arrow and a disabled first-page arrow",
    );
    assert.match(
      template,
      /<span class="page-number current">\{\{\.\}\}<\/span>[\s\S]*?<a class="page-number" href=/,
      "the current page should be an unlinked span while other pages remain links",
    );
    assert.match(
      template,
      /<a class="extend next" rel="next"[\s\S]*?<span class="extend next" rel="next">/,
      "paginator should render a linked next arrow and a disabled last-page arrow",
    );
    assert.doesNotMatch(template, /<nav class="paginator">/, "paginator must not add a wrapper nav");
  });
}
