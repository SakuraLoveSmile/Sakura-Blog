// 隔离 Chromium 运行时验收；仅使用本文件 HTTP fixture，不读取或写入 Blog 数据。
import { test, type Browser } from '@playwright/test';
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { readFile } from 'node:fs/promises';

const assetRoot = new URL('../../modules/stellar/web/assets/js/', import.meta.url);
let server: ReturnType<typeof createServer>, browser: Browser, base: string;
const manifest = {
  version: 1, root: '/',
  extensions: ['shell', 'search'].map(name => ({ id: 'blog-' + name,
    module: '/assets/js/runtime/extensions/' + name + '.js', scope: 'document', when: { always: true }, config: {} })),
  policy: { cache: { enabled: false, defaultTtl: 3600, ttl: {}, maxEntries: 200 },
    request: { retries: 2, timeoutMs: 5000, idleTimeoutMs: 3000, maxCacheEntryBytes: 204800 } },
};
const fixture = `<!doctype html><html data-theme="dark"><body>
<div class="site-shell" data-regions="leftbar rightbar"><div class="site-workspace">
<aside id="leftbar-region"><button id="search-open" data-search-action="open">搜索</button><a href="#main" id="left-link">导航</a></aside>
<main id="main"><textarea id="editor"></textarea></main><aside id="rightbar-region"><a id="toc-link" href="#main">目录</a></aside></div>
<button id="left-toggle" data-shell-action="toggle-leftbar-drawer" aria-controls="leftbar-region">左栏</button>
<button id="right-toggle" data-shell-action="toggle-rightbar-drawer" aria-controls="rightbar-region">右栏</button>
<button id="collapse" data-shell-action="toggle-leftbar">折叠</button><button id="dismiss" data-shell-action="dismiss-drawer">关闭</button>
<dialog id="site-search-dialog"><button id="search-close" data-search-action="close">关闭搜索</button>
<div class="search-wrapper" id="search-wrapper"><input type="search" id="search-input" data-filter="/wiki/demo/"><div id="search-result"></div></div></dialog></div>
<script>const ctx = {root:'/',search:{path:'/search.json',local_search:{lazy_load:true,cache_ttl:0}}};</script>
<script id="stellar-runtime-config" type="application/json">${JSON.stringify(manifest)}</script>
<script src="/assets/js/search/local-search.js"></script><script src="/assets/js/search/shortcut.js"></script>
<script type="module" src="/assets/js/runtime/index.js?v=fixture"></script></body></html>`;
test.beforeAll(async ({ browser: testBrowser }) => {
  browser = testBrowser;
  server = createServer(async (request, response) => {
    const url = new URL(request.url, 'http://127.0.0.1');
    if (url.pathname === '/') { response.setHeader('Content-Type', 'text/html'); response.end(fixture); return; }
    if (url.pathname === '/api/search') {
      if (url.searchParams.get('q') === 'fallback') { response.writeHead(503); response.end('{}'); return; }
      if (url.searchParams.get('q') === 'slow') await new Promise(resolve => setTimeout(resolve, 100));
      response.setHeader('Content-Type', 'application/json');
      response.end(JSON.stringify({ items: [
        { title: 'Blog post', type: 'post', path: '/post/test/', snippet: 'test' },
        { title: 'Wiki document', type: 'page', path: '/wiki/demo/doc/', snippet: url.searchParams.get('q') },
      ] })); return;
    }
    if (url.pathname === '/search.json') {
      response.setHeader('Content-Type', 'application/json');
      response.end(JSON.stringify([
        { title: 'Local wiki', path: '/wiki/demo/local/', content: 'fallback content 文库搜索独特词' },
        { title: 'Other wiki', path: '/wiki/other/local/', content: 'fallback content 文库搜索独特词' },
      ])); return;
    }
    if (url.pathname.startsWith('/assets/js/')) {
      try { response.setHeader('Content-Type', 'text/javascript'); response.end(await readFile(new URL(url.pathname.slice('/assets/js/'.length), assetRoot))); }
      catch { response.writeHead(404); response.end(); } return;
    }
    response.writeHead(404); response.end();
  });
  await new Promise(resolve => server.listen(0, '127.0.0.1', () => resolve(undefined)));
  base = 'http://127.0.0.1:' + (server.address() as AddressInfo).port;

});
test.afterAll(async () => { if (server) await new Promise<void>(resolve => server.close(() => resolve())); });
async function pageAt(width: number) {
  const page = await browser.newPage({ viewport: { width, height: 900 } });
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  page.on('console', message => { if (message.type() === 'error' && message.text().includes('stellar')) errors.push(message.text()); });
  await page.goto(base);
  await page.waitForFunction(() => window.blogShell && document.querySelector('#search-input')._localSearchBound);
  assert.deepEqual(errors, []);
  return page;
}
test('浏览器：2.0 ESM 启动传播缓存版本，移动抽屉互斥、隐藏状态与返回焦点正确', async () => {
  const page = await browser.newPage({ viewport: { width: 390, height: 900 } });
  const modules: string[] = [];
  page.on('request', request => { if (request.url().includes('/runtime/')) modules.push(request.url()); });
  await page.goto(base);
  await page.waitForFunction(() => window.blogShell);
  assert.ok(modules.length >= 6);
  assert.ok(modules.every(url => new URL(url).searchParams.get('v') === 'fixture'));
  assert.equal(await page.locator('#leftbar-region').evaluate(el => el.inert), true);
  await page.locator('#left-toggle').click();
  assert.equal(await page.locator('.site-shell').getAttribute('data-drawer'), 'leftbar');
  assert.equal(await page.locator('#left-toggle').getAttribute('aria-expanded'), 'true');
  assert.equal(await page.locator('#leftbar-region').evaluate(el => el.inert), false);
  assert.equal(await page.locator('#search-open').evaluate(el => document.activeElement === el), true);
  await page.locator('#right-toggle').click();
  assert.equal(await page.locator('.site-shell').getAttribute('data-drawer'), 'rightbar');
  assert.equal(await page.locator('#left-toggle').getAttribute('aria-expanded'), 'false');
  assert.equal(await page.locator('#leftbar-region').getAttribute('aria-hidden'), 'true');
  await page.keyboard.press('Escape');
  assert.equal(await page.locator('.site-shell').getAttribute('data-drawer'), null);
  assert.equal(await page.locator('#right-toggle').evaluate(el => document.activeElement === el), true);
  await page.close();
});
test('浏览器：断点 768/1180、桌面收起及 resize 的可访问状态同步', async () => {
  const page = await pageAt(1280);
  await page.locator('#collapse').click();
  assert.equal(await page.locator('html').getAttribute('data-leftbar-state'), 'collapsed');
  assert.equal(await page.locator('#collapse').getAttribute('aria-expanded'), 'false');
  assert.equal(await page.locator('#leftbar-region').evaluate(el => el.inert), false);
  for (const [width, leftHidden, rightHidden] of [[1181,false,false], [1180,false,true], [769,false,true], [768,true,true]]) {
    await page.setViewportSize({ width, height: 900 });
    await page.waitForFunction(({leftHidden,rightHidden}) => document.querySelector('#leftbar-region').inert === leftHidden && document.querySelector('#rightbar-region').inert === rightHidden, {leftHidden,rightHidden});
  }
  await page.locator('#left-toggle').click();
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.waitForFunction(() => !document.querySelector('.site-shell').dataset.drawer);
  assert.equal(await page.locator('#leftbar-region').getAttribute('aria-hidden'), null);
  await page.close();
});
test('浏览器：搜索使用真正 modal dialog，窄屏快捷键、编辑区和返回焦点正确', async () => {
  const page = await pageAt(390);
  await page.locator('#right-toggle').focus();
  await page.keyboard.press('Control+k');
  assert.equal(await page.locator('#site-search-dialog').evaluate(el => el.matches(':modal')), true);
  assert.equal(await page.locator('#search-input').evaluate(el => document.activeElement === el), true);
  await page.locator('#search-input').fill('Stellar');
  await page.keyboard.press('Escape');
  assert.equal(await page.locator('#site-search-dialog').evaluate(el => el.open), false);
  assert.equal(await page.locator('#right-toggle').evaluate(el => document.activeElement === el), true);
  await page.locator('#editor').focus();
  await page.keyboard.press('Meta+k');
  assert.equal(await page.locator('#site-search-dialog').evaluate(el => el.open), false);
  await page.locator('#left-toggle').click();
  await page.locator('#search-open').click();
  await page.locator('#search-close').click();
  assert.equal(await page.locator('#search-open').evaluate(el => document.activeElement === el), true);
  await page.close();
});
test('浏览器：保留 Wiki filter 与 Go FTS/本地索引，清空输入不被旧响应覆盖', async () => {
  const page = await pageAt(1280);
  await page.locator('#search-open').click();
  await page.locator('#search-input').fill('test');
  await page.waitForFunction(() => document.querySelector('#search-result').textContent.includes('Wiki document'));
  assert.equal(await page.locator('#search-result').textContent().then(text => text.includes('Blog post')), false);
  await page.locator('#search-input').fill('slow');
  await page.locator('#search-input').fill('');
  await new Promise(resolve => setTimeout(resolve, 180));
  assert.equal(await page.locator('#search-result').textContent(), '');
  await page.locator('#search-input').fill('fallback');
  await page.waitForFunction(() => document.querySelector('#search-result').textContent.includes('Local wiki'));
  assert.ok((await page.locator('#search-result a').getAttribute('href')).startsWith('/wiki/demo/local/?kw='));
  await page.close();
});
test('浏览器：pagehide 卸载与 BFCache pageshow 重挂载不叠加 shell/search 监听', async () => {
  const page = await pageAt(390);
  await page.evaluate(() => window.dispatchEvent(new PageTransitionEvent('pagehide', { persisted:true })));
  await page.waitForFunction(() => !window.blogShell && !document.querySelector('#search-input')._localSearchBound);
  await page.locator('#left-toggle').click();
  assert.equal(await page.locator('.site-shell').getAttribute('data-drawer'), null);
  await page.evaluate(() => window.dispatchEvent(new PageTransitionEvent('pageshow', { persisted:true })));
  await page.waitForFunction(() => window.blogShell && document.querySelector('#search-input')._localSearchBound);
  await page.locator('#left-toggle').click();
  assert.equal(await page.locator('.site-shell').getAttribute('data-drawer'), 'leftbar');
  await page.locator('#search-open').click();
  const calls: string[] = [];
  page.on('request', request => { if (request.url().includes('/api/search')) calls.push(request.url()); });
  await page.locator('#search-input').fill('test');
  await page.waitForFunction(() => document.querySelector('#search-result').textContent.includes('Wiki document'));
  assert.equal(calls.length, 1);
  await page.close();
});


test('浏览器：Wiki 限定搜索在 FTS 空或全部跨项目时等待 JSON 索引并严格限定项目', async () => {
  for (const items of [[], [{ title:'Other FTS wiki', type:'page', path:'/wiki/other/doc/', snippet:'文库搜索独特词' }]]) {
    const page = await pageAt(1280);
    await page.route('**/api/search?**', route => route.fulfill({json:{items}}));
    await page.locator('#search-open').click();
    await page.locator('#search-input').fill('文库搜索独特词');
    await page.waitForFunction(() => document.querySelector('#search-result').textContent.includes('Local wiki'), undefined, {timeout:1000});
    assert.equal(await page.locator('#search-result a').count(), 1);
    assert.ok((await page.locator('#search-result a').getAttribute('href')).startsWith('/wiki/demo/local/?kw='));
    assert.equal((await page.locator('#search-result').textContent()).includes('Other'), false);
    await page.close();
  }
});
test('浏览器：普通搜索 FTS 空结果不自动转为 Wiki 本地结果', async () => {
  const page = await pageAt(1280);
  const indexRequests: string[] = [];
  page.on('request', request => { if(request.url().endsWith('/search.json')) indexRequests.push(request.url()); });
  await page.locator('#search-input').evaluate(input => input.removeAttribute('data-filter'));
  await page.route('**/api/search?**', route => route.fulfill({json:{items:[]}}));
  await page.locator('#search-open').click();
  const response = page.waitForResponse(response => response.url().includes('/api/search'));
  await page.locator('#search-input').fill('文库搜索独特词');
  await response;
  assert.equal(await page.locator('#search-result').textContent(), '');
  assert.equal(indexRequests.length, 0);
  await page.close();
});
test('浏览器：等待 Wiki 本地索引期间清空、改词或卸载会丢弃旧结果', async () => {
  for (const action of ['clear', 'replace', 'unmount']) {
    const page = await pageAt(1280);
    await page.route('**/api/search?**', route => route.fulfill({json:{items:[]}}));
    let releaseIndex: () => void;
    const indexReady = new Promise<void>(resolve => { releaseIndex = resolve; });
    let indexRequested: () => void;
    let indexRequests = 0;
    const requested = new Promise<void>(resolve => { indexRequested = resolve; });
    await page.route('**/search.json', async route => {
      indexRequests++;
      indexRequested();
      await indexReady;
      await route.fulfill({json:[
        {title:'Late old wiki',path:'/wiki/demo/late-old/',content:'文库搜索独特词'},
        {title:'Late new wiki',path:'/wiki/demo/late-new/',content:'更新的词'},
      ]});
    });
    await page.locator('#search-open').click();
    await page.locator('#search-input').fill('文库搜索独特词');
    await requested;
    if (action === 'clear') await page.locator('#search-input').fill('');
    else if (action === 'replace') await page.locator('#search-input').fill('更新的词');
    else {
      await page.evaluate(() => window.dispatchEvent(new PageTransitionEvent('pagehide', {persisted:true})));
      await page.waitForFunction(() => !document.querySelector('#search-input')._localSearchBound);
    }
    const received = page.waitForResponse(response => response.url().endsWith('/search.json'));
    releaseIndex();
    await received;
    await page.waitForFunction(() => searchFetchPromise === null);
    if (action === 'replace') {
      await page.waitForFunction(() => document.querySelector('#search-result').textContent.includes('Late new wiki'));
      assert.equal(await page.locator('#search-result a').count(), 1);
      assert.equal((await page.locator('#search-result').textContent()).includes('Late old wiki'), false);
    } else assert.equal(await page.locator('#search-result').textContent(), '');
    assert.equal(indexRequests, 1);
    await page.close();
  }
});

test('浏览器：Wiki 已有索引在后台刷新延迟 503 后仍可显示缓存命中', async () => {
  const page = await pageAt(1280);
  await page.route('**/api/search?**', route => route.fulfill({json:{items:[]}}));
  await page.locator('#search-open').click();
  await page.locator('#search-input').fill('文库搜索独特词');
  await page.waitForFunction(() => document.querySelector('#search-result').textContent.includes('Local wiki'));
  await page.locator('#search-close').click();

  let releaseRefresh: () => void;
  const refreshReady = new Promise<void>(resolve => { releaseRefresh = resolve; });
  let markRefreshRequested: () => void;
  const requested = new Promise<void>(resolve => { markRefreshRequested = resolve; });
  let refreshRequests = 0;
  await page.route('**/search.json', async route => {
    refreshRequests++;
    markRefreshRequested();
    await refreshReady;
    await route.fulfill({status:503,body:'index unavailable'});
  });
  // fixture 的 cache_ttl=0：重新聚焦时会后台刷新，旧索引仍然可用。
  await page.locator('#search-open').click();
  await requested;
  await page.locator('#search-input').fill('');
  const searchReceived = page.waitForResponse(response => response.url().includes('/api/search'));
  await page.locator('#search-input').fill('文库搜索独特词');
  await searchReceived;
  assert.equal(await page.locator('#search-result').textContent(), '');
  const refreshed = page.waitForResponse(response => response.url().endsWith('/search.json'));
  releaseRefresh();
  await refreshed;
  await page.waitForFunction(() => document.querySelector('#search-result').textContent.includes('Local wiki'), undefined, {timeout:1000});
  assert.equal(await page.locator('#search-result a').count(), 1);
  assert.ok((await page.locator('#search-result a').getAttribute('href')).startsWith('/wiki/demo/local/?kw='));
  assert.equal((await page.locator('#search-result').textContent()).includes('Other wiki'), false);
  assert.equal(refreshRequests, 1);
  await page.close();
});
