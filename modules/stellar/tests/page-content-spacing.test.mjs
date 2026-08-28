// 独立 Page 的 poetry → quot 间距契约：收紧首个标题与诗词卡片的外部留白。
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const CSS_PATH = join(dirname(fileURLToPath(import.meta.url)), '../web/assets/css/tag-plugins.css');
const css = readFileSync(CSS_PATH, 'utf8');

test('page poetry followed by quot uses compact 1rem margins', () => {
  assert.match(css, /\.l_body\[layout='page'\] \.article-banner-wrap \+ article\.md-text\.content > \.tag-plugin:first-child\s*\{[\s\S]*?margin-block-start: var\(--gap-p\);/);
  assert.match(css, /\.l_body\[layout='page'\] \.md-text\.content > \.tag-plugin\.poetry:has\(\+ \.tag-plugin\.quot\)/);
  assert.match(css, /\.l_body\[layout='page'\] \.md-text\.content > \.tag-plugin\.poetry \+ \.tag-plugin\.quot\s*\{[\s\S]*?margin-top: 1rem;/);
  assert.match(css, /\.l_body\[layout='page'\] \.md-text\.content > \.tag-plugin\.poetry:has\(\+ \.tag-plugin\.quot\)[\s\S]*?margin-bottom: 1rem;/);
});
