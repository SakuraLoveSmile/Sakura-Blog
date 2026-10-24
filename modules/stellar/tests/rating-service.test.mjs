// 行为面：文章评分（assets/js/services/rating.js）—— localStorage 去重、星级预览、提交防重。
// 机械校验入口：make test → assets-test → node --test modules/stellar/tests/
// 加载真实 rating.js 源码到 vm 沙箱（最小浏览器桩），断言纯逻辑，不依赖真实 DOM/网络。
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { mountBlogService } from './blog-service-fixture.mjs';

const SRC_PATH = join(dirname(fileURLToPath(import.meta.url)), '../web/assets/js/blog/services/rating.js');
const SURFACE = '评分组件';

// classList 桩：Set 语义，覆盖 add/remove/toggle/contains
function makeClassList() {
  const set = new Set();
  return {
    add: (...names) => names.forEach((n) => set.add(n)),
    remove: (...names) => names.forEach((n) => set.delete(n)),
    toggle: (name, force) => {
      const on = force !== undefined ? !!force : !set.has(name);
      on ? set.add(name) : set.delete(name);
      return on;
    },
    contains: (name) => set.has(name),
  };
}

function makeLocalStorage() {
  const map = new Map();
  return {
    getItem: (k) => (map.has(k) ? map.get(k) : null),
    setItem: (k, v) => map.set(k, String(v)),
    removeItem: (k) => map.delete(k),
  };
}

// 最小浏览器桩：readyState=complete 让顶层 initRatings() 立即执行（页面无 .ds-rating 时为空扫描）。
function loadRating() {
  const noop = () => {};
  const fetchCalls = [];
  const sandbox = {
    console: { log: noop, warn: noop, error: noop },
    localStorage: makeLocalStorage(),
    document: {
      readyState: 'complete',
      querySelectorAll: () => [],
      addEventListener: noop,
      createElement: () => ({ className: '', textContent: '' }),
    },
    fetch: (url, opts) => {
      fetchCalls.push({ url, opts });
      return Promise.resolve({ json: async () => ({ avg: 4.0, count: 2 }) });
    },
    addEventListener: noop,
  };
  sandbox.window = sandbox;
  mountBlogService(readFileSync(SRC_PATH, 'utf8'), sandbox, { exports: ['storeRating', 'hasRated', 'getRatedValue', 'updatePreview', 'submitRating'] });
  assert.ok(
    sandbox.storeRating && sandbox.hasRated && sandbox.updatePreview && sandbox.submitRating,
    `[${SURFACE}] rating.js 顶层函数（storeRating/hasRated/updatePreview/submitRating）未暴露，机械校验入口断裂`
  );
  return { sandbox, fetchCalls };
}

function makeStars(count = 5) {
  return Array.from({ length: count }, (_, i) => ({
    dataset: { value: String(i + 1) },
    classList: makeClassList(),
  }));
}

// 评分元素桩：submitRating → loadRating 只读 dataset/querySelector(All)
function makeRatingEl(id, stars) {
  const avgEl = { className: 'avg', textContent: '' };
  const countEl = { className: 'count', textContent: '' };
  return {
    dataset: { id, api: 'https://api.example.com/rating' },
    classList: makeClassList(),
    querySelector: (sel) => ({ '.avg': avgEl, '.count': countEl }[sel] ?? null),
    querySelectorAll: (sel) => (sel === '.star' ? stars : []),
    appendChild: () => {},
  };
}

test(`${SURFACE} · localStorage 去重：storeRating 后 hasRated/getRatedValue 按 id 隔离`, () => {
  const { sandbox } = loadRating();
  assert.equal(sandbox.hasRated('post-1'), false);
  sandbox.storeRating('post-1', 4);
  assert.equal(sandbox.hasRated('post-1'), true);
  assert.equal(sandbox.getRatedValue('post-1'), 4);
  // 不同 id 互不影响
  assert.equal(sandbox.hasRated('post-2'), false);
  assert.equal(sandbox.getRatedValue('post-2'), 0);
});

test(`${SURFACE} · updatePreview：平均分向下取整，≤取整值的星星标记 preview`, () => {
  const { sandbox } = loadRating();
  const stars = makeStars();
  const el = makeRatingEl('post-preview', stars);
  sandbox.updatePreview(el, 3.7);
  assert.deepEqual(
    stars.map((s) => s.classList.contains('preview')),
    [true, true, true, false, false]
  );
  // 平均分下降后预览随之收缩
  sandbox.updatePreview(el, 1.2);
  assert.deepEqual(
    stars.map((s) => s.classList.contains('preview')),
    [true, false, false, false, false]
  );
});

test(`${SURFACE} · submitRating：首次提交 POST 一次并落 localStorage，重复提交不再发请求`, async () => {
  const { sandbox, fetchCalls } = loadRating();
  const el = makeRatingEl('post-submit', makeStars());
  await sandbox.submitRating(el, '5');
  await new Promise((r) => setTimeout(r, 0)); // 等待 submitRating 内部 loadRating 落定
  const posts = fetchCalls.filter((c) => c.opts?.method === 'POST');
  assert.equal(posts.length, 1);
  assert.deepEqual(JSON.parse(posts[0].opts.body), { id: 'post-submit', score: 5 });
  assert.equal(el.classList.contains('rated'), true);
  assert.equal(sandbox.getRatedValue('post-submit'), 5);
  // 已评分：再次提交（换一个分值）不产生新 POST、不覆盖已存分值
  await sandbox.submitRating(el, '1');
  await new Promise((r) => setTimeout(r, 0));
  assert.equal(fetchCalls.filter((c) => c.opts?.method === 'POST').length, 1);
  assert.equal(sandbox.getRatedValue('post-submit'), 5);
});
