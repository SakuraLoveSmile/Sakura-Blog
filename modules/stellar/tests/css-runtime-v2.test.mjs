// 上游运行时的动态 CSS 入口及不改写产物的全量漂移检查。
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, writeFileSync, existsSync, mkdtempSync, mkdirSync, rmSync } from 'node:fs';
import { createRequire } from 'node:module';
import { dirname, join } from 'node:path';
import { tmpdir } from 'node:os';
import { fileURLToPath } from 'node:url';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const assets = join(root, 'web/assets/css');
const read = name => readFileSync(join(assets, name), 'utf8');
const dynamic = ['comments/beaudar.css', 'comments/utterances.css', 'comments/twikoo.css', 'comments/waline.css', 'comments/artalk.css', 'plugins/fancybox.css', 'plugins/swiper.css'];

test('全部上游动态入口包含各自真实组件规则，无整站布局或旧版本 core 替身', () => {
  const consumers = {
    'comments/beaudar.css': /\.cmt-body\.beaudar\s*\{[^}]*width: calc\(100% \+ 8px\)/,
    'comments/utterances.css': /\.cmt-body\.utterances\s*\{[^}]*width: calc\(100% \+ 8px\)/,
    'comments/twikoo.css': /\.cmt-body\.twikoo\s*\{[^}]*--twikoo-theme: var\(--theme\)/,
    'comments/waline.css': /\.cmt-body\.waline/,
    'comments/artalk.css': /\.cmt-body\.artalk \.artalk \.atk-main-editor/,
    'plugins/fancybox.css': /img\[data-fancybox\],\s*a\[data-fancybox\]\s*\{\s*cursor: zoom-in/,
    'plugins/swiper.css': /\.swiper:not\(\.swiper-initialized\)\s*\{\s*display: none/,
  };
  for (const name of dynamic) {
    const css = read(name);
    assert.ok(consumers[name].test(css), `${name} 未提供对应上游规则`);
    assert.doesNotMatch(css, /\.site-shell|\.site-main|LXGW WenKai Screen|\.l_body/);
    assert.doesNotMatch(css, /font-weight:\s*['"]var\(/);
  }
  assert.match(read('comments/waline.css'), /:root\[data-theme="dark"\] \.cmt-body\.waline/);
  assert.match(read('comments/waline.css'), /@media \(prefers-color-scheme: dark\)\s*\{\s*:root:not\(\[data-theme\]\) \.cmt-body\.waline/);
  assert.match(read('comments/twikoo.css'), /font-weight: var\(--font-weight-500, 500\)/);
  assert.match(read('plugins/swiper.css'), /--swiper-theme-color: var\(--theme\) !important/);
});

const require = createRequire(join(root, 'web/build/stylus/compile.js'));
let compiler;
try { compiler = require('./compile.js'); } catch (error) {
  if (error.code !== 'MODULE_NOT_FOUND') throw error;
}
const repositoryDir = join(root, '../../resource/hexo-theme-stellar');
test('漂移检查诊断非 main 入口和缺失文件，失败后仍保留原有产物', { skip: !compiler || !existsSync(repositoryDir) }, async () => {
  const outputDir = mkdtempSync(join(tmpdir(), 'stellar-css-drift-fixture-'));
  try {
    const generated = await compiler.compile({ repositoryDir, outputDir });
    mkdirSync(join(outputDir, 'local'), { recursive: true });
    const custom = join(outputDir, 'local/custom.css');
    writeFileSync(custom, 'body { margin: 1px; }');
    assert.equal(await compiler.check({ repositoryDir, outputDir }), 20);
    const damaged = join(outputDir, 'main-flat.css');
    const missing = join(outputDir, 'comments/waline.css');
    writeFileSync(damaged, '/* deliberate drift fixture */\n');
    rmSync(missing);
    const retained = Object.keys(generated).filter(name => name !== 'comments/waline.css');
    const before = Object.fromEntries(retained.map(name => [name, readFileSync(join(outputDir, name), 'utf8')]));
    await assert.rejects(compiler.check({ repositoryDir, outputDir }), error => {
      assert.match(error.message, /main-flat\.css: 与锁定源重编结果不同/);
      assert.match(error.message, /comments\/waline\.css: 缺少产物/);
      return true;
    });
    assert.equal(existsSync(missing), false, '检查不能重新生成被删文件');
    for (const name of retained) assert.equal(readFileSync(join(outputDir, name), 'utf8'), before[name], `${name} 被检查改写`);
    // 普通辅助 CSS 不在 generated 范围，不受主题漂移检查改写。
    assert.equal(readFileSync(custom, 'utf8'), 'body { margin: 1px; }');
  } finally {
    rmSync(outputDir, { recursive: true, force: true });
  }
});
