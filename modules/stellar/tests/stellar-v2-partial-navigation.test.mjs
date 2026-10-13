// 上游browser事务断言移植，服务端eligibility由Go集成测试负责。
import { test } from 'node:test';
import assert from 'node:assert/strict';
const navigation = () => import('../web/assets/js/runtime/extensions/partial-navigation.js');
test("navigation requires matching collection and preserved-shell compatibility", async () => {
  const { navigationMatches } = await navigation();
  const current = { collection: 'one', signature: 'shell' };
  assert.equal(navigationMatches(current, { ...current }), true);
  assert.equal(navigationMatches(current, { ...current, collection: 'other' }), false);
  assert.equal(navigationMatches(current, { ...current, signature: 'changed' }), false);
  assert.equal(navigationMatches(current, null), false);
});

test("navigation transactions serialize commits and discard superseded pending work", async () => {
  const { createNavigationQueue } = await navigation();
  const queue = createNavigationQueue();
  const events = [];
  let finish;
  const first = queue.begin();
  const committing = queue.commit(first.signal, async () => {
    events.push('start');
    await new Promise(resolve => { finish = resolve; });
    events.push('finish');
  });
  await Promise.resolve();
  const second = queue.begin();
  const skipped = queue.commit(second.signal, () => events.push('stale'));
  const third = queue.begin();
  const latest = queue.commit(third.signal, () => events.push('latest'));
  assert.equal(first.signal.aborted, true);
  finish();
  await Promise.all([committing, skipped, latest]);
  assert.deepEqual(events, ['start', 'finish', 'latest']);
  queue.stop();
  assert.equal(third.signal.aborted, true);
});

test('运行时拒绝未声明的可执行script，同时接受标记后的核心脚本', async () => {
  const { readNavigation } = await navigation();
  const data = { collection: 'wiki:fixture', signature: 'shell', url: 'https://fixture.test/wiki/one/' };
  const config = { textContent: JSON.stringify(data) };
  const unmanaged = { getAttribute(name) { return name === 'type' ? '' : null; } };
  function documentWith(scripts) {
    return { getElementById() { return config; }, querySelectorAll(selector) { return selector === '#stellar-navigation-config' ? [config] : scripts; } };
  }
  assert.equal(readNavigation(documentWith([unmanaged])), null);
  assert.deepEqual(readNavigation(documentWith([])), data);
  const json = { getAttribute() { return 'application/json'; } };
  assert.deepEqual(readNavigation(documentWith([json])), data);
});
