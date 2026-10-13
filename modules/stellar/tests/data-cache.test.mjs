import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequestClient, REQUEST_CACHE_PREFIX } from '../web/assets/js/runtime/request-cache.js';
import { mountBlogService } from './blog-service-fixture.mjs';

const policy = { retries: 0, timeoutMs: 5000, idleTimeoutMs: 3000, maxCacheEntryBytes: 204800 };
function storageFixture() {
  const values = new Map([['unrelated', 'keep']]);
  return { values, get length() { return values.size; }, key: index => [...values.keys()][index],
    getItem: key => values.get(key) || null, setItem: (key, value) => values.set(key, value), removeItem: key => values.delete(key) };
}
test('runtime按服务TTL缓存并只淘汰自己的缓存；禁用缓存不写入', async () => {
  const storage = storageFixture(); let time = 0;
  const client = createRequestClient({ policy, storage, clock: () => ++time, scheduler: fn => fn(),
    cache: { enabled: true, defaultTtl: 60, ttl: { friends: 120 }, maxEntries: 1 }, fetch: async () => new Response('{}') });
  await client.request('/one', { service: 'friends' }); await new Promise(resolve => setImmediate(resolve));
  assert.equal(JSON.parse(storage.getItem(REQUEST_CACHE_PREFIX + '/one')).ttl, 120);
  await client.request('/two'); await new Promise(resolve => setImmediate(resolve));
  assert.equal(storage.getItem(REQUEST_CACHE_PREFIX + '/one'), null); assert.equal(storage.getItem('unrelated'), 'keep');
  const disabled = createRequestClient({ policy, storage, cache: { enabled: false, defaultTtl: 60, ttl: {}, maxEntries: 1 }, fetch: async () => new Response('{}') });
  await disabled.request('/disabled'); await new Promise(resolve => setImmediate(resolve));
  assert.equal(storage.getItem(REQUEST_CACHE_PREFIX + '/disabled'), null);
});
test('Blog动态服务经统一request client传递服务ID与可取消signal', async () => {
  for (const name of ['sites', 'friends', 'friends_and_posts', 'siteinfo']) {
    const calls = [];
    const element = { nodeType: 1, dataset: { api: '/fixture/' + name, siteinfoApi: '/fixture/siteinfo' }, querySelector() { return null; } };
    const document = { querySelectorAll: selector => selector.includes(name) || name === 'siteinfo' ? [element] : [],
      createElement() { return { innerHTML: '', querySelector() { return null; } }; }, getElementsByClassName() { return [element]; } };
    const window = { wrapLazyloadImages() {}, addEventListener() {}, removeEventListener() {}, dispatchEvent() {} };
    const sandbox = { window, document, def: { avatar: '/avatar', cover: '/cover' }, ctx: { services: { siteinfo: {} } },
      console, URL, Event, CustomEvent, utils: { dom() { return { find() { return { append() {} }; }, append() {} }; } } };
    const source = readFileSync(new URL('../web/assets/js/blog/services/' + name + '.js', import.meta.url), 'utf8');
    const { cleanup } = mountBlogService(source, sandbox, { request: async (url, options) => {
      calls.push({ url, options }); return { json: async () => ({ content: [], icon: '' }) };
    } });
    await new Promise(resolve => setImmediate(resolve));
    assert.equal(calls.length, 1, name); assert.equal(calls[0].options.service, name);
    cleanup(); assert.equal(calls[0].options.signal.aborted, true);
  }
});
