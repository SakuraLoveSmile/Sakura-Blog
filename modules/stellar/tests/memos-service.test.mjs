import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import { createRequestClient } from '../web/assets/js/runtime/request-cache.js';
import { installLegacyRequestAdapter } from '../web/assets/js/runtime/legacy-request-adapter.js';

const source = path => readFileSync(new URL(path, import.meta.url), 'utf8');
const settled = () => new Promise(resolve => setImmediate(resolve));
const oldMemo = { content: 'Memos marker', createdTs: 1750000000, creatorName: 'Fixture', resourceList: [{ type: 'image/png', id: 7 }] };

function fixture(apis) {
  const requests = [], errors = [], failures = [], cells = [], observers = [];
  const elements = apis.map(api => ({ className: 'ds-memos', dataset: { api }, getAttribute: key => key === 'hide' ? 'user' : null }));
  const root = { getElementsByClassName: name => name === 'ds-memos' ? elements : [], querySelectorAll: () => [], contains: () => true };
  const controller = new AbortController();
  const policy = { retries: 0, timeoutMs: 5000, idleTimeoutMs: 100, maxCacheEntryBytes: 1024 };
  const client = createRequestClient({ policy, cache: { enabled: false, defaultTtl: 0, ttl: {}, maxEntries: 0 },
    fetch: (url, options) => new Promise(resolve => requests.push({ url, options, resolve })) });
  const utils = { onLoadFailure: element => { if (element) failures.push(element); }, dom: element => ({ append: html => cells.push({ element, html }) }) };
  installLegacyRequestAdapter(utils, client, policy);
  const document = Object.assign(new EventTarget(), { currentScript: {}, baseURI: 'https://blog.example.test/' });
  const sanitized = [];
  const window = Object.assign(new EventTarget(), { DOMPurify: () => ({ isSupported: true, addHook() {}, sanitize(html, config) { sanitized.push({ html, config }); return html; } }), utils });
  const sandbox = vm.createContext({ document, window, URL, def: { avatar: '' }, marked: { parse: content => `<p>${content}</p>` },
    console: { log() {} }, setTimeout, clearTimeout, setInterval, clearInterval,
    IntersectionObserver: class { constructor() { observers.push(this); } disconnect() { this.disconnected = true; } }
  });
  vm.runInContext(source('../web/assets/js/services/memos.js'), sandbox);
  const script = document.currentScript;
  vm.runInContext(source('../web/assets/js/runtime/extensions/services.js').replace('export async function mount', 'async function mount'), sandbox);
  const context = { signal: controller.signal, reportError: error => errors.push(error),
    extension: { config: { marked: '/marked.js' } },
    legacy: { ctx: { root: '/', services: { memos: { js: '/memos.js' } } } },
    assets: { script: async url => url === '/memos.js' ? script : {} }
  };
  const respond = (index, data, status = 200) => requests[index].resolve(new Response(JSON.stringify(data), { status, headers: { 'Content-Type': 'application/json' } }));
  return { requests, errors, failures, cells, controller, observers, respond, mount: () => sandbox.mount(root, context) };
}

test('Memos通过真实regional请求入口支持绝对HTTP(S)，保留参数并按实际origin渲染附件', async () => {
  for (const origin of ['http://127.0.0.1:8110', 'https://memos.example.test:8443']) {
    const api = origin + '/api/v1/memos?limit=2&visibility=PUBLIC';
    const f = fixture([api]), cleanup = await f.mount();
    assert.equal(f.requests.length, 1);
    assert.equal(f.requests[0].url, api);
    f.respond(0, [oldMemo]);
    await settled();
    assert.equal(f.cells.length, 1);
    assert.ok(f.cells[0].html.includes('Memos marker'));
    assert.ok(f.cells[0].html.includes(`src="${origin}/o/r/7"`));
    assert.equal(f.errors.length, 0);
    f.controller.abort(); cleanup();
    assert.ok(f.observers.every(observer => observer.disconnected));
  }
});

test('Memos无效或其他scheme明确失败，且不阻断同region的合法API', async () => {
  for (const api of ['broken', '//memos.example.test/api/v1/memos', 'https://', 'ftp://memos.example.test/list', 'javascript:alert(1)']) {
    const f = fixture([api, 'http://127.0.0.1:8110/memos']), cleanup = await f.mount();
    assert.equal(f.failures.length, 1, api);
    assert.equal(f.errors.length, 1, api);
    assert.match(f.errors[0].message, /Memos.*HTTP\(S\)/);
    assert.equal(f.requests.length, 1);
    f.respond(0, [oldMemo]);
    await settled();
    assert.equal(f.cells.length, 1);
    f.controller.abort(); cleanup();
  }
});

test('Memos各正式版本附件保留路径和外链优先级，不把HTTP强行改为HTTPS', async () => {
  const origin = 'http://127.0.0.1:8110';
  const common = { content: 'Version marker', createTime: '2026-10-13T00:00:00Z', creator: 'users/1' };
  const image = { type: 'image/png', id: 7, name: 'attachments/a', filename: 'a.png' };
  for (const [item, path] of [
    [{ ...common, resources: [image] }, '/o/r/7'],
    [{ ...common, attachments: [image] }, '/file/attachments/a/a.png'],
    [{ ...common, creator: 'users/alice', attachments: [image] }, '/file/attachments/a/a.png'],
    [{ ...common, attachments: [{ ...image, externalLink: 'https://cdn.example.test/image.png' }] }, null]
  ]) {
    const f = fixture([origin]), cleanup = await f.mount();
    f.respond(0, { memos: [item] });
    await settled();
    assert.equal(f.cells.length, 1);
    assert.ok(f.cells[0].html.includes(`src="${path ? origin + path : 'https://cdn.example.test/image.png'}"`));
    assert.equal(f.errors.length, 0);
    f.controller.abort(); cleanup();
  }
});

test('Memos503走正式failure；cleanup取消网络并阻止迟到内容回写', async () => {
  const failed = fixture(['http://127.0.0.1:8110/memos']), failedCleanup = await failed.mount();
  failed.respond(0, {}, 503);
  await settled();
  assert.equal(failed.failures.length, 1);
  assert.equal(failed.errors.length, 1);
  assert.match(failed.errors[0].message, /503/);
  assert.equal(failed.cells.length, 0);
  failed.controller.abort(); failedCleanup();
  const late = fixture(['https://memos.example.test/api/v1/memos']), cleanup = await late.mount();
  late.controller.abort(); cleanup();
  assert.equal(late.requests[0].options.signal.aborted, true);
  late.respond(0, [oldMemo]);
  await settled();
  assert.equal(late.cells.length, 0);
  assert.equal(late.errors.length, 0);
  assert.equal(late.failures.length, 0);
});
