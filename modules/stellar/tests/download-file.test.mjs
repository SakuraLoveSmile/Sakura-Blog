import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

function fixture() {
  const warnings = [], downloads = [], requests = [];
  let responseData, resolveData;
  const context = vm.createContext({
    AbortController, Blob,
    ctx: { tag_plugins: { chat: { api: 'https://fixture.test/file' } } },
    console: { warn: error => warnings.push(error) },
    fetch: async (url, options) => {
      requests.push({ url, options });
      return { ok: true, json: () => responseData };
    },
    URL: { createObjectURL: blob => { downloads.push(blob); return 'blob:fixture'; }, revokeObjectURL() {} },
    document: {
      createElement: () => ({ click() {} }),
      body: { appendChild() {}, removeChild() {} }
    }
  });
  vm.runInContext(readFileSync(new URL('../web/assets/js/plugins/download-file.js', import.meta.url), 'utf8'), context);
  function element() {
    const target = new EventTarget(), span = { innerHTML: '待加载' };
    target.getAttribute = () => '/uploads/fixture.txt';
    target.querySelector = () => span;
    let listeners = 0;
    const add = target.addEventListener.bind(target), remove = target.removeEventListener.bind(target);
    target.addEventListener = (...args) => { listeners++; add(...args); };
    target.removeEventListener = (...args) => { listeners--; remove(...args); };
    return { target, span, listeners: () => listeners };
  }
  const mount = (data, item = element()) => {
    responseData = data;
    const cleanup = context.downloadFileEvent([item.target]);
    return { ...item, cleanup, click: () => item.target.dispatchEvent(new Event('click')) };
  };
  const deferred = () => { responseData = new Promise(resolve => { resolveData = resolve; }); return responseData; };
  return { mount, deferred, resolve: data => resolveData(data), warnings, downloads, requests };
}

const settled = () => new Promise(resolve => setImmediate(resolve));

test('HTTP200缺少或畸形file.data进入失败路径，不绑定下载或生成伪空文件', async () => {
  const malformed = [
    null, {}, { file: null }, { file: {} }, { file: { data: null } },
    { file: { data: '' } }, { file: { data: { 0: 1, length: 1 } } },
    ...[null, true, '1', -1, 256, 1.5, NaN, Infinity, undefined].map(byte => ({ file: { data: [byte] } })),
    { file: { data: new Array(1) } }
  ];
  for (const data of malformed) {
    const f = fixture(), item = f.mount(data);
    await settled();
    item.click();
    assert.equal(item.listeners(), 0, JSON.stringify(data));
    assert.equal(item.span.innerHTML, '待加载');
    assert.equal(f.downloads.length, 0);
    assert.equal(f.warnings.length, 1);
    item.cleanup();
  }
});

test('正式Buffer字节协议保留精确内容，真实空数组仍可下载', async () => {
  for (const bytes of [[], [0, 1, 127, 128, 254, 255]]) {
    const f = fixture(), item = f.mount({ file: { type: 'Buffer', data: bytes } });
    await settled();
    assert.equal(item.listeners(), 1);
    assert.equal(item.span.innerHTML, `${bytes.length}B`);
    item.click();
    assert.equal(f.downloads.length, 1);
    assert.equal(f.downloads[0].type, 'text/plain');
    assert.deepEqual([...new Uint8Array(await f.downloads[0].arrayBuffer())], bytes);
    assert.equal(f.warnings.length, 0);
    assert.equal(f.requests[0].url, 'https://fixture.test/file?type=file&url=%2Fuploads%2Ffixture.txt');
    item.cleanup();
    item.click();
    assert.equal(f.downloads.length, 1);
  }
});

test('同URL重新挂载的畸形或迟到响应不能复用旧文件下载监听', async () => {
  const f = fixture(), first = f.mount({ file: { data: [65] } });
  await settled();
  first.cleanup();
  const next = f.mount({});
  await settled();
  next.click(); first.click();
  assert.equal(f.downloads.length, 0);
  assert.equal(next.listeners(), 0);
  next.cleanup();
  const pending = f.mount(f.deferred());
  await settled();
  pending.cleanup();
  f.resolve({ file: { data: [66] } });
  await settled();
  pending.click();
  assert.equal(f.downloads.length, 0);
  assert.equal(pending.listeners(), 0);
  assert.equal(pending.span.innerHTML, '待加载');
});
