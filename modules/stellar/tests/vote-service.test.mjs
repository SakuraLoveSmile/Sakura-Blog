// 行为面：文章投票（assets/js/services/vote.js）—— 乐观更新、防重复投票、同步失败回滚。
// 机械校验入口：make test → assets-test → node --test modules/stellar/tests/
// 加载真实 vote.js 源码到 vm 沙箱（最小浏览器桩），断言纯逻辑，不依赖真实 DOM/网络。
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { mountBlogService } from './blog-service-fixture.mjs';

const SRC_PATH = join(dirname(fileURLToPath(import.meta.url)), '../web/assets/js/blog/services/vote.js');
const SURFACE = '投票组件';

function makeClassList() {
  const set = new Set();
  return {
    add: (...names) => names.forEach((n) => set.add(n)),
    remove: (...names) => names.forEach((n) => set.delete(n)),
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

// 最小浏览器桩：readyState=complete 让顶层 initVotes() 立即空扫描；fetch 行为由 fetchImpl 注入。
function loadVote(fetchImpl) {
  const noop = () => {};
  const fetchCalls = [];
  const sandbox = {
    console: { log: noop, warn: noop, error: noop },
    localStorage: makeLocalStorage(),
    document: { readyState: 'complete', querySelectorAll: () => [], addEventListener: noop },
    fetch: (url, opts) => {
      fetchCalls.push({ url, opts });
      return fetchImpl(url, opts);
    },
    addEventListener: noop,
  };
  sandbox.window = sandbox;
  mountBlogService(readFileSync(SRC_PATH, 'utf8'), sandbox, { exports: ['submitVote', 'revertVote', 'hasVoted', 'storeVote', 'getVotedValue'] });
  assert.ok(
    sandbox.submitVote && sandbox.revertVote && sandbox.hasVoted,
    `[${SURFACE}] vote.js 顶层函数（submitVote/revertVote/hasVoted）未暴露，机械校验入口断裂`
  );
  return { sandbox, fetchCalls };
}

// 投票元素桩：submitVote/revertVote 只读 dataset/querySelector 与 .up/.down 计数节点
function makeVoteEl(id, up = 3, down = 1) {
  const upEl = { textContent: String(up) };
  const downEl = { textContent: String(down) };
  const upBtn = { classList: makeClassList() };
  const downBtn = { classList: makeClassList() };
  return {
    dataset: { id, api: 'https://api.example.com/vote' },
    classList: makeClassList(),
    querySelector: (sel) =>
      ({ '.up': upEl, '.down': downEl, '.vote-up': upBtn, '.vote-down': downBtn }[sel] ?? null),
    upEl,
    downEl,
    upBtn,
  };
}

test(`${SURFACE} · submitVote：乐观 +1、落 localStorage、发一次后台同步`, async () => {
  const { sandbox, fetchCalls } = loadVote(() => Promise.resolve({ json: async () => ({}) }));
  const el = makeVoteEl('post-1', 3, 1);
  sandbox.submitVote(el, 'up');
  // 乐观更新先于网络返回生效
  assert.equal(el.upEl.textContent, 4);
  assert.equal(el.downEl.textContent, '1');
  assert.equal(el.classList.contains('voted'), true);
  assert.equal(el.upBtn.classList.contains('active'), true);
  assert.equal(sandbox.getVotedValue('post-1'), 'up');
  await new Promise((r) => setTimeout(r, 0));
  const updates = fetchCalls.filter((c) => c.url.includes('/update'));
  assert.equal(updates.length, 1);
  assert.ok(updates[0].url.includes('id=post-1') && updates[0].url.includes('value=up'));
});

test(`${SURFACE} · submitVote：已投票的 id 再次提交被拒绝（无请求、计数不变）`, async () => {
  const { sandbox, fetchCalls } = loadVote(() => Promise.resolve({ json: async () => ({}) }));
  const el = makeVoteEl('post-2', 5, 2);
  sandbox.storeVote('post-2', 'up');
  sandbox.submitVote(el, 'down');
  await new Promise((r) => setTimeout(r, 0));
  assert.equal(el.downEl.textContent, '2');
  assert.equal(fetchCalls.filter((c) => c.url.includes('/update')).length, 0);
});

test(`${SURFACE} · 后台同步失败：计数回滚、样式清除、localStorage 撤销`, async () => {
  const { sandbox } = loadVote(() => Promise.reject(new Error('network down')));
  const el = makeVoteEl('post-3', 3, 1);
  sandbox.submitVote(el, 'up');
  assert.equal(el.upEl.textContent, 4); // 乐观值
  await new Promise((r) => setTimeout(r, 0)); // 等待 catch → revertVote
  assert.equal(el.upEl.textContent, 3);
  assert.equal(el.classList.contains('voted'), false);
  assert.equal(el.upBtn.classList.contains('active'), false);
  assert.equal(sandbox.hasVoted('post-3'), false);
});

test(`${SURFACE} · revertVote：计数回退不跌破 0`, () => {
  const { sandbox } = loadVote(() => Promise.resolve({ json: async () => ({}) }));
  const el = makeVoteEl('post-4', 0, 0);
  sandbox.storeVote('post-4', 'up');
  sandbox.revertVote(el, 'up');
  assert.equal(el.upEl.textContent, 0);
  assert.equal(sandbox.hasVoted('post-4'), false);
});
