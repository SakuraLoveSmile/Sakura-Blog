// 四外观最终视觉与三配色选择器；CI 不依赖被忽略的上游源。
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, existsSync, mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { execFileSync } from 'node:child_process';
import { createRequire } from 'node:module';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const read = name => readFileSync(join(root, 'web/assets/css', name), 'utf8');
const config = JSON.parse(readFileSync(join(root, 'web/build/stylus/config.json'), 'utf8'));
const presets = ['card', 'glass', 'minimal', 'flat'];
const css = Object.fromEntries(presets.map(preset => [preset, read(`main-${preset}.css`)]));

// 只提取最终层独立文件里的 UI Surface 规则，避免侧栏/断点的特化误判为全局规则。
function surface(preset) {
  const tail = read(`main-appearance-${preset}.css`);
  return tail.match(/(?:^|\n)([^{}]*\.ui-surface)\s*\{([^}]+)\}/)?.[2] || '';
}

test('四种外观使用各自真实材质和布局，card 兼容入口逐字一致', () => {
  assert.equal(read('main.css'), css.card);
  assert.match(surface('card'), /background: var\(--card\)/);
  assert.match(surface('card'), /box-shadow: 0 4px 8px/);
  assert.match(surface('glass'), /background: transparent/);
  assert.match(surface('glass'), /isolation: isolate/);
  assert.match(css.glass, /\.ui-surface:before\s*\{[^}]*backdrop-filter: blur\(8px\)/);
  assert.match(css.glass, /\.site-region--leftbar \.site-region__decoration\s*\{[^}]*background-image: radial-gradient/);
  assert.match(surface('minimal'), /background: transparent/);
  assert.match(surface('minimal'), /box-shadow: none/);
  assert.match(css.minimal, /\.site-region--topbar,\s*\.listing-nav\s*\{\s*position: static/);
  assert.match(surface('flat'), /border: 1px solid var\(--block-border\)/);
  assert.match(surface('flat'), /border-radius: 0/);
  assert.match(surface('flat'), /backdrop-filter: saturate\(140%\) blur\(24px\)/);
  assert.equal(new Set(Object.values(css)).size, 4);
});

test('每种外观同时支持显式浅色、显式深色和无 data-theme 的系统深色', () => {
  for (const preset of presets) {
    assert.match(css[preset], /:root\s*\{[^}]*--background: hsla\(210deg 20% 98% \/ 1\)/, `${preset} 基础浅色`);
    assert.match(css[preset], /:root\[data-theme="light"\]\s*\{[^}]*--background: hsla\(210deg 20% 98% \/ 1\)/, `${preset} 显式浅色`);
    assert.match(css[preset], /:root\[data-theme="dark"\]\s*\{[^}]*--background: hsla\(210deg 8% 12% \/ 1\)/, `${preset} 显式深色`);
    assert.match(css[preset], /@media \(prefers-color-scheme: dark\)\s*\{\s*:root:not\(\[data-theme\]\)\s*\{[^}]*--background: hsla\(210deg 8% 12% \/ 1\)/, `${preset} 自动深色`);
    assert.doesNotMatch(css[preset], /(?:font-family|font-weight|border-radius): (?:null|undefined);/);
  }
  // Minimal 在基础色之后用暖纸色覆盖三种状态，不能只保留全局冷色 tokens。
  assert.match(css.minimal, /:root\[data-theme='light'\]\s*\{\s*--background: hsla\(44 50% 96% \/ 1\)/);
  assert.match(css.minimal, /:root\[data-theme='dark'\]\s*\{\s*--background: hsla\(44 12% 12% \/ 1\)/);
  assert.match(css.minimal, /@media \(prefers-color-scheme: dark\)\s*\{\s*:root:not\(\[data-theme\]\)\s*\{\s*--background: hsla\(44 12% 12% \/ 1\)/);
});

test('构建默认值包含完整上游玻璃背景及系统字体，热更新变量进入实际声明', () => {
  assert.equal(config.appearance.color_scheme, 'auto');
  assert.equal(config.appearance.backgrounds.page.image, null);
  assert.equal(config.appearance.backgrounds.leftbar.type, 'gradient');
  assert.equal(config.appearance.backgrounds.leftbar.gradient.light.length, 4);
  assert.equal(config.appearance.backgrounds.leftbar.gradient.dark.length, 4);
  assert.deepEqual(config.features.card_hover, { spotlight: false, tilt: false });
  assert.equal(config.features.lazy_loading.transition, 'fade');
  for (const preset of presets) {
    for (const variable of ['--font-family-body', '--font-family-code', '--font-size-root', '--font-size-inline-code', '--font-size-code-block', '--font-smoothing', '--content-align', '--corner-shape', '--code-scrollbar-width', '--radius-card-l', '--radius-card', '--radius-card-s', '--radius-bar', '--radius-image-l', '--radius-image', '--radius-image-s', '--gradient-start', '--gradient-searchbar', '--checkbox-pointer-events', '--note-border']) {
      assert.ok(css[preset].includes(`var(${variable},`), `${preset}: ${variable}`);
    }
    for (const weight of [400, 500, 600, 700, 900]) {
      assert.ok(css[preset].includes(`var(--font-weight-${weight}, ${weight})`));
    }
    assert.doesNotMatch(css[preset], /LXGW WenKai Screen/);
    assert.match(css[preset], /input\[type='radio'\]:not\(\.ui-radio\)/);
    assert.match(css[preset], /input\.ui-radio\s*\{[^}]*--radio-ring: var\(--text-p3\)/);
  }
});

test('选配资源独立启用 spotlight、tilt、KaTeX、blur，最终外观层不重置基础颜色', () => {
  const spotlight = read('main-card-hover-spotlight.css');
  const tilt = read('main-card-hover-tilt.css');
  assert.match(spotlight, /\.card-hover__spotlight/);
  assert.doesNotMatch(spotlight, /\.post-list:not\(\.topic\) \.post-card-wrap/);
  assert.doesNotMatch(spotlight, /transform: perspective/);
  assert.match(tilt, /transform: perspective/);
  assert.match(tilt, /\.post-list:not\(\.topic\) \.post-card-wrap/);
  assert.doesNotMatch(tilt, /(?:^|\n)\.card-hover__spotlight\s*\{/);
  assert.match(read('main-katex.css'), /\.md-text p\.katex-block\s*\{[^}]*overflow: scroll/);
  assert.match(read('main-lazyload-blur.css'), /\.lazy\s*\{[^}]*filter: blur\(20px\)/);
  assert.match(read('main-lazyload-blur.css'), /animation: ll-blur-in/);
  assert.doesNotMatch(read('main-lazyload-blur.css'), /animation: ll-fade-in/);
  assert.doesNotMatch(css.card, /(?:^|\n)\.card-hover__spotlight\s*\{|transform: perspective|animation: ll-blur-in/);
  for (const preset of presets) {
    const tail = read(`main-appearance-${preset}.css`);
    assert.doesNotMatch(tail, /stellar-appearance-boundary|--hue: 192|--fs-root:/);
  }
  // 加载 tilt 后的最终层必须可再次移除 Minimal/Flat 的列表阴影。
  for (const preset of ['minimal', 'flat']) {
    assert.match(read(`main-appearance-${preset}.css`), /post-card[^}]*\{\s*box-shadow: none;/);
  }
});

// 本地编译型验证使用已有 Stylus 依赖；CI 仍只要求上述提交产物检查。
const require = createRequire(join(root, 'web/build/stylus/compile.js'));
let renderSource, compile;
try { ({ renderSource, compile } = require('./compile.js')); } catch (error) {
  if (error.code !== 'MODULE_NOT_FOUND') throw error;
}
test('独立编译时布尔 false、null、数组与不同 renderer 配置没有串扰', { skip: !renderSource }, async () => {
  const source = `
.probe
  if hexo-config('flag') == true
    display: block
  else
    display: none
  if hexo-config('missing')
    color: red
  count: length(hexo-config('values'))
`;
  const [enabled, disabled] = await Promise.all([
    renderSource(source, 'probe.styl', '.', { flag: true, missing: null, values: ['one', 'two'] }),
    renderSource(source, 'probe.styl', '.', { flag: false, values: ['one', 'two', 'three'] }),
  ]);
  assert.match(enabled, /display: block/);
  assert.match(enabled, /count: 2/);
  assert.match(disabled, /display: none/);
  assert.match(disabled, /count: 3/);
  assert.doesNotMatch(enabled + disabled, /color: red/);
});

const repositoryDir = join(root, '../../resource/hexo-theme-stellar');
test('全套 CSS 再编译逐字一致，包含所有独立选配和最终层', { skip: !compile || !existsSync(repositoryDir) }, async () => {
  const outputDir = mkdtempSync(join(tmpdir(), 'stellar-appearance-test-'));
  try {
    // 模拟提交 hook 的 Git 环境，必须继续读取 upstream 而非当前 Blog 仓库。
    const script = `require(process.argv[1]).compile({ outputDir: process.argv[2] }).catch(error => { console.error(error); process.exitCode = 1; });`;
    execFileSync(process.execPath, ['-e', script, join(root, 'web/build/stylus/compile.js'), outputDir], {
      env: { ...process.env, GIT_DIR: join(outputDir, 'foreign-git-dir'), GIT_INDEX_FILE: join(outputDir, 'foreign-index') },
    });
    const outputs = await compile({ repositoryDir, outputDir });
    assert.equal(Object.keys(outputs).length, 13);
    for (const [name, rebuilt] of Object.entries(outputs)) {
      assert.ok(rebuilt === read(name), `${name} 与锁定源重新编译不同`);
    }
  } finally {
    rmSync(outputDir, { recursive: true, force: true });
  }
});
