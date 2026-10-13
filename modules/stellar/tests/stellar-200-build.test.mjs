// Stellar 2.0 构建契约；CI 直接检查已提交产物，不依赖被忽略的上游源。
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..");
const read = (relative) => readFileSync(join(ROOT, relative), "utf8");
const config = JSON.parse(read("web/build/stylus/config.json"));

test("2.0 构建锁定发布提交并采用上游 card / auto 默认外观", () => {
  const upstream = JSON.parse(read("web/build/stylus/upstream.json"));
  assert.equal(upstream.tag, "2.0.0");
  assert.equal(upstream.commit, "9d11978a24fe6610e3a7dbab741cf62634539d05");
  assert.match(read("version.go"), /StellarVersion = "2\.0\.0"/);
  assert.equal(config.appearance.preset, "card");
  assert.equal(config.appearance.color_scheme, "auto");
  assert.equal(config.features.color_scheme_switch.enabled, false);
  assert.equal(config.tags.checkbox.interactive, false);
  assert.equal(config.tags.note.border, true);
  assert.equal(config.article.listing.cover_ratio, 2);
});

test("2.0 产物含新布局与最终 card 视觉层，并保留中文和本地排版", () => {
  const css = read("web/assets/css/main.css");
  for (const selector of [".site-shell", ".site-region--leftbar", ".site-main", ".ui-surface", ".swiper", "[data-fancybox]", ".cmt-body.artalk"]) {
    assert.ok(css.includes(selector), `缺少 ${selector}`);
  }
  assert.match(css, /\.ui-surface\s*\{[^}]*background: var\(--card\)/);
  assert.match(css, /font-family: var\(--font-family-body, system-ui, "Microsoft Yahei", "Segoe UI", Arial, sans-serif\)/);
  assert.doesNotMatch(css, /LXGW WenKai Screen/);
  assert.match(css, /font-family: var\(--font-family-code, Menlo, Monaco, Consolas, system-ui, monospace, sans-serif\)/);
  assert.match(css, /--text-loading: ['"]正在加载['"]/);
  assert.match(css, /--text-loading-error: ['"]加载失败，请稍后重试。['"]/);
  assert.match(css, /--hue: 192/);
  assert.doesNotMatch(css, /(?:font-family|font-weight|border-radius): (?:null|undefined);/);
});

test("2.0 产物消费已有圆角和渐变设置，空设置保留各档原默认值", () => {
  const css = read("web/assets/css/main.css");
  for (const declaration of [
    "border-radius: var(--radius-card-l, var(--radius-card, 24px))",
    "border-radius: var(--radius-card, 16px)",
    "border-radius: var(--radius-card-s, var(--radius-block, 12px))",
    "border-radius: var(--radius-bar, var(--radius-navbar, 12px))",
    "border-radius: var(--radius-image-l, var(--radius-image, 24px))",
    "border-radius: var(--radius-image, 16px)",
    "border-radius: var(--radius-image-s, var(--radius-image, 8px))",
    // Stylus 将既有 HSL 默认渐变规范化为这些等值十六进制颜色。
    "background: var(--gradient-start, linear-gradient(to right, #4c95fa, #38c9fa, #24f9c4, #24f9c4, #38c9fa, #4c95fa))",
    `background: var(--gradient-searchbar, ${config.appearance.gradients.search_bar})`,
  ]) assert.ok(css.includes(declaration), `缺少 ${declaration}`);
  // 容器同心圆角与横幅裁剪也须接通，避免只覆盖 border-radius 的局部兼容。
  assert.ok(css.includes("calc(var(--radius-bar, var(--radius-navbar, 12px)) + 4px)"));
  assert.ok(css.includes("inset(0 round var(--radius-card-l, var(--radius-card, 24px)))"));
});

test("旧图库、About、下拉图标和 radio 内容保留实际 CSS 消费者", () => {
  const css = read("web/assets/css/main.css");
  // 本地模板和渲染器的既有 DOM 无需重新生成内容即可继续匹配样式。
  assert.match(read("web/templates/partial/moment_gallery.html"), /ratio="\{\{\.GridRatio\}\}"/);
  assert.match(read("../../internal/post/shortcode_display.go"), /class="tag-plugin about"/);
  assert.match(read("../../internal/post/shortcode_blocks.go"), /class="ui-collection__leading"/);
  assert.match(css, /\.tag-plugin\.gallery\.grid-box\[ratio='square'\] \.grid-cell\s*\{\s*aspect-ratio: 1;/);
  assert.match(css, /\.tag-plugin\.gallery\.grid-box\[ratio='portrait'\] \.grid-cell\s*\{\s*aspect-ratio: 2\s*\/\s*3;/);
  assert.match(css, /\.tag-plugin\.about \.about-header\s*\{[^}]*justify-content: center;/);
  assert.match(css, /\.tag-plugin\.about \.nav-back svg\s*\{\s*width: 1rem;\s*height: 1rem;/);
  assert.match(css, /\.dropdown-menu \.ui-collection__leading:not\(\.ui-icon\)\s*\{[^}]*width: 1\.5rem;\s*height: 1\.5rem;/);
  assert.match(css, /\.search-trigger \.ui-icon > svg:not\(\[fill\]\)\s*\{\s*fill: currentColor;/);
  assert.match(css, /\.md-text \.tag-plugin\.checkbox input\[type='radio'\]:not\(\.ui-radio\)\s*\{[^}]*--radio-ring: var\(--theme\);\s*--radio-dot-size: 0;/);
  assert.match(css, /\.md-text \.tag-plugin\.checkbox input\[type='radio'\]:not\(\.ui-radio\):checked\s*\{\s*--radio-dot-size: 8px;/);
  assert.match(css, /\.md-text \.tag-plugin\.checkbox input\s*\{[^}]*pointer-events: var\(--checkbox-pointer-events, none\);/);
});

test("搜索渐变匹配新版 dialog 的实际表单位置", () => {
  const template = read("web/templates/partial/widgets/search.html");
  const css = read("web/assets/css/main.css");
  assert.match(template, /class="search-dialog__header">\s*<div class="search-form"/);
  assert.match(css, /\.search-dialog__header \.search-form:after\s*\{[^}]*background: var\(--gradient-searchbar,/);
  assert.match(css, /\.search-dialog__header \.search-form:has\(input:focus\):after,[\s\S]*?\{\s*opacity: 1;/);
});
