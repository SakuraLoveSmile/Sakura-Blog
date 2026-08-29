// Stellar 1.44 模板/资源契约机械校验。
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..");
const read = (relative) => readFileSync(join(ROOT, relative), "utf8");

test("1.44 搜索控件使用语义化 search 输入框并加载共享快捷键", () => {
  const template = read("web/templates/partial/widgets/search.html");
  const layout = read("web/templates/layout/base.html");
  assert.match(template, /<div class="search-form" role="search">/);
  assert.match(template, /<label class="search-button" for="search-input" aria-hidden="true">/);
  assert.match(template, /<input type="search"[^>]*id="search-input"/);
  assert.match(layout, /assets\/js\/search\/shortcut\.js/);
  assert.doesNotMatch(template, /<form class="search-form"/);
  assert.doesNotMatch(template, /<a class="search-button"/);
});

test("1.44 Card Hover 组合类覆盖卡片、collection、shortcode 和 Hero 按钮", () => {
  const files = [
    "templates/partial/index.html",
    "templates/partial/wiki.html",
    "templates/partial/topic.html",
    "templates/partial/components/collection-item.html",
    "templates/partial/wiki_doc.html",
  ];
  for (const file of files) assert.match(read(`web/${file}`), /card-hover/);
  const shortcode = read("../../internal/post/shortcode_media.go");
  assert.match(shortcode, /link-card[^`]*card-hover/);
  assert.match(shortcode, /cell card-hover card-hover--spotlight card-hover--tilt/);
  assert.match(read("web/assets/css/main.css"), /\.card-hover__spotlight/);
});

test("Wiki Hero 同时支持图片和 Galaxy 参数，并保留 reduced-motion 降级", () => {
  const template = read("web/templates/partial/wiki_doc.html");
  const main = read("web/assets/js/main.js");
  assert.match(template, /wiki-cover-background\{\{if \$galaxy\}\} galaxy/);
  assert.match(template, /data-galaxy-params=/);
  assert.match(template, /data-text-adaptive="split"/);
  assert.match(main, /prefers-reduced-motion: reduce/);
  assert.match(main, /galaxyCanvases\.length === 0/);
});

test("版本标识和测试文章已切换到 1.44.0", () => {
  assert.match(read("version.go"), /StellarVersion = "1\.44\.0"/);
  const article = read("../../content/posts/stellar-144-acceptance.md");
  assert.match(article, /status: draft/);
  for (const keyword of ["Card Hover", "Galaxy", "Command", "移动端"]) assert.match(article, new RegExp(keyword));
});
