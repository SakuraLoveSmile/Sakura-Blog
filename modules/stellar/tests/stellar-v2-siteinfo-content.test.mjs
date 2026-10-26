import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

const flush = () => new Promise(resolve => setImmediate(resolve));
function fixture() {
  const requests = [], images = [];
  const sandbox = vm.createContext({ URL, utils: { request(_element, api, callback, _failure, options) {
    return new Promise((resolve, reject) => requests.push({ api, options, respond: data => Promise.resolve(callback({ json: async () => data })).then(resolve, reject) }));
  } }, Image: class { constructor() { images.push(this); } } });
  vm.runInContext(readFileSync(new URL('../web/assets/js/services/siteinfo.js', import.meta.url), 'utf8'), sandbox);
  const title = {}, desc = {}, image = { style: {}, setAttribute(key, value) { this[key] = value; } }, icon = { setAttribute(key, value) { this[key] = value; } };
  const attrs = new Map([['autofill', 'title,desc,icon'], ['cardlink', '']]);
  const target = { nodeType: 1, baseURI: 'https://example.test/post', href: 'https://friend.test', isConnected: true,
    dataset: { api: '/fixture/siteinfo', siteinfoApi: '/fixture/siteinfo' },
    classList: { contains: () => false }, getAttribute: key => attrs.get(key), removeAttribute: key => attrs.delete(key),
    querySelector: selector => ({ '.title': title, '.desc': desc, '.img': image, '.siteinfo-icon': icon, '.md-link-icon': { replaceChildren: value => { target.markdownImage = value; } } })[selector],
  };
  return { sandbox, requests, images, target, title, desc, image, icon, attrs };
}

test('siteinfo将标题描述保留为文本、拒绝危险图标，成功后同DOM不重复请求', async () => {
  const f = fixture(), controller = new AbortController();
  f.sandbox.setCardLink([f.target], controller.signal);
  await f.requests[0].respond({ title: '<img src=x onerror=alert(1)>', desc: '<script>alert(2)</script>', appicon: 'javascript:alert(3)' });
  assert.equal(f.title.textContent, '<img src=x onerror=alert(1)>');
  assert.equal(f.desc.textContent, '<script>alert(2)</script>');
  assert.equal(f.title.innerHTML, undefined); assert.equal(f.image.style.backgroundImage, undefined);
  assert.equal(f.attrs.has('cardlink'), false);
  controller.abort(); f.sandbox.setCardLink([f.target], new AbortController().signal);
  assert.equal(f.requests.length, 1);
});

test('siteinfo正文迟到不覆盖旧区域，取消后保留cardlink可重挂；图标URL只允许HTTP(S)', async () => {
  const f = fixture(), controller = new AbortController();
  f.sandbox.setCardLink([f.target], controller.signal); controller.abort();
  await f.requests[0].respond({ title: 'Late' });
  assert.equal(f.title.textContent, undefined); assert.equal(f.attrs.has('cardlink'), true);
  f.sandbox.setCardLink([f.target], new AbortController().signal);
  await f.requests[1].respond({ title: 'Current', appicon: '/icon.png' });
  assert.equal(f.title.textContent, 'Current'); assert.equal(f.image['data-bg'], 'https://example.test/icon.png');

  f.sandbox.setSiteCardIcon([f.target], new AbortController().signal);
  await f.requests[2].respond({ appicon: 'data:image/svg+xml,<svg onload=alert(1)>' });
  assert.equal(f.icon.src, undefined);
});

test('Markdown图标加载期间取消允许重新挂载，旧图片load不会覆盖新区域', async () => {
  const f = fixture(), controller = new AbortController();
  f.sandbox.setMdLinkIcon([f.target], controller.signal);
  const first = f.requests[0].respond({ favicon: '/old.png' }); await flush();
  const oldImage = f.images[0]; controller.abort(); await first;
  assert.equal(f.target.markdownImage, undefined);
  f.sandbox.setMdLinkIcon([f.target], new AbortController().signal);
  const second = f.requests[1].respond({ favicon: '/current.png' }); await flush();
  f.images[1].onload(); await second;
  assert.equal(f.target.markdownImage, f.images[1]); assert.equal(oldImage.onload, null);
});
