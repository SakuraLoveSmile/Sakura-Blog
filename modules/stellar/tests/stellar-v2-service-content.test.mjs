import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import { createRequestClient } from '../web/assets/js/runtime/request-cache.js';
import { installLegacyRequestAdapter } from '../web/assets/js/runtime/legacy-request-adapter.js';

const source = path => readFileSync(new URL('../web/assets/js/' + path, import.meta.url), 'utf8');
const flush = () => new Promise(resolve => setImmediate(resolve));

function fixture(service = 'friends', api = '/fixture/friends', baseURI = 'https://example.test/blog/post/demo', browserGlobals = {}) {
  const requests = [], errors = [], cells = [], failures = [];
  const attributes = new Map();
  const element = { nodeType: 1, className: 'ds-' + service, dataset: { api }, baseURI,
    getAttribute: key => attributes.get(key) ?? null,
    querySelectorAll: () => [],
    classList: { contains: () => false, add() {}, remove() {} },
  };
  const root = { getElementsByClassName: name => name === 'ds-' + service ? [element] : [], querySelectorAll: () => [], contains: node => node === element, querySelector: selector => selector === '.ds-' + service ? element : null };
  const policy = { retries: 0, timeoutMs: 5000, idleTimeoutMs: 100, maxCacheEntryBytes: 1024 };
  const client = createRequestClient({ policy, cache: { enabled: false, defaultTtl: 0, ttl: {}, maxEntries: 0 },
    fetch: (url, options) => new Promise(resolve => requests.push({ url, options, resolve })) });
  const dom = target => ({
    find: () => dom(target),
    append(html) { cells.push({ target, html }); },
    text() {}, attr() {},
  });
  const utils = { dom, onLoadFailure: node => { if (node) failures.push(node); } };
  installLegacyRequestAdapter(utils, client, policy);
  const document = Object.assign(new EventTarget(), { currentScript: {}, baseURI });
  const sanitized = [];
  const window = Object.assign(new EventTarget(), { DOMPurify: () => ({ isSupported: true, addHook() {}, sanitize(html, config) { sanitized.push({ html, config }); return html; } }), utils, wrapLazyloadImages() {} });
  const sandbox = vm.createContext({ window, document, URL, AbortController, CustomEvent: class extends Event { constructor(type, options) { super(type); this.detail = options?.detail; } },
    def: { avatar: '/avatar.png', cover: '/cover.png' }, ctx: { services: { siteinfo: {} }, icons: { 'default:to-comment': '<svg></svg>' } },
    marked: { parse: content => '<p>' + content + '</p>' }, console, setTimeout, clearTimeout, setInterval, clearInterval,
    IntersectionObserver: class { disconnect() {} },
    ...browserGlobals,
  });
  vm.runInContext(source('services/' + (['giscus', 'waline', 'twikoo', 'artalk'].includes(service) ? service + '_latest_comment' : service) + '.js'), sandbox);
  const script = document.currentScript;
  vm.runInContext(source('runtime/extensions/services.js').replace('export async function mount', 'async function mount'), sandbox);
  let controller;
  async function mount() {
    controller = new AbortController();
    const cleanup = await sandbox.mount(root, { signal: controller.signal, reportError: error => errors.push(error),
      extension: { config: { marked: '/marked.js' } }, legacy: { ctx: { root: '/blog/', services: { siteinfo: { js: '/siteinfo.js' }, [service]: { js: '/' + service + '.js' } } } },
      assets: { script: async url => url === '/' + service + '.js' ? script : {} },
    });
    return () => { controller.abort(); cleanup(); };
  }
  function respond(index, data, status = 200) {
    requests[index].resolve(new Response(JSON.stringify(data), { status, headers: { 'Content-Type': 'application/json' } }));
  }
  return { requests, errors, cells, failures, element, attributes, sanitized, mount, respond };
}

test('官方friends成功后保留DOM重挂不重复，错误正文解析后仍允许重新挂载重试', async () => {
  const loaded = fixture();
  let cleanup = await loaded.mount();
  loaded.respond(0, [{ title: 'Friend', url: 'https://friend.test' }]); await flush(); await flush();
  assert.equal(loaded.cells.length, 1); cleanup();
  cleanup = await loaded.mount(); await flush();
  assert.equal(loaded.requests.length, 1);
  assert.equal(loaded.cells.length, 1); cleanup();

  const retry = fixture(); cleanup = await retry.mount();
  retry.requests[0].resolve(new Response('{broken', { headers: { 'Content-Type': 'application/json' } }));
  await flush(); await flush(); cleanup();
  assert.equal(retry.errors.length, 1);
  cleanup = await retry.mount(); await flush();
  assert.equal(retry.requests.length, 2, '失败不能被成功标记阻止重试');
  retry.respond(1, [{ title: 'Recovered', url: 'https://friend.test' }]); await flush(); await flush();
  assert.equal(retry.cells.length, 1); cleanup();
});

test('官方friends正文迟到且区域已卸载时不追加，新区域可重试同一个保留节点', async () => {
  const f = fixture(); let cleanup = await f.mount();
  let release;
  f.requests[0].resolve({ ok: true, status: 200, headers: new Headers(), clone() { return this; }, text: async () => '[]',
    json: () => new Promise(resolve => { release = resolve; }) });
  await flush(); cleanup(); release([{ title: 'Late', url: 'https://friend.test' }]); await flush(); await flush();
  assert.equal(f.cells.length, 0);
  assert.equal(f.errors.length, 0);
  cleanup = await f.mount(); await flush();
  assert.equal(f.requests.length, 2, '取消解码不能留下成功标记');
  f.respond(1, [{ title: 'Current', url: 'https://friend.test' }]); await flush(); await flush();
  assert.equal(f.cells.length, 1); cleanup();
});

test('官方timeline相对API按页面base解析，保留查询标签过滤并可渲染返回数据', async () => {
  for (const api of ['/fixture/timeline?labels=Release', '../api/timeline?labels=Release']) {
    const f = fixture('timeline', api); const cleanup = await f.mount();
    f.respond(0, [{ title: 'Release', body: 'Timeline body', html_url: 'https://example.test/release', created_at: '2026-10-26T00:00:00Z', labels: [{ name: 'Release', color: '123456' }] }]);
    await flush(); await flush();
    assert.equal(f.errors.length, 0, api);
    assert.equal(f.cells.length, 1, api);
    assert.ok(f.cells[0].html.includes('Timeline body'));
    assert.ok(!f.cells[0].html.includes('class="item label Release"'), 'API查询已经包含的标签不重复显示');
    cleanup();
  }
});

test('官方friends把远端标题和标签当文本，拒绝事件属性与危险URL', async () => {
  const f = fixture(); const cleanup = await f.mount();
  f.respond(0, [{ title: '<img src=x onerror=alert(1)>', url: 'javascript:alert(2)', avatar: 'x" onload="alert(3)', labels: [{ color: 'fff;background:url(javascript:alert(4))', name: '<svg onload=alert(5)>' }] }]);
  await flush(); await flush();
  assert.equal(f.errors.length, 0);
  assert.equal(f.cells.length, 1);
  const html = f.cells[0].html;
  assert.ok(html.includes('&lt;img src=x onerror=alert(1)&gt;'));
  assert.ok(html.includes('&lt;svg onload=alert(5)&gt;'));
  assert.ok(!/href="javascript:|onload="|background:url\(javascript:/i.test(html));
  cleanup();
});

test('空响应也属于成功；畸形列表失败时没有半个响应残留，重新挂载可以重试', async () => {
  const empty = fixture(); let cleanup = await empty.mount();
  empty.respond(0, []); await flush(); await flush(); cleanup();
  cleanup = await empty.mount();
  assert.equal(empty.requests.length, 1); assert.equal(empty.cells.length, 0); cleanup();

  const retry = fixture(); cleanup = await retry.mount();
  retry.respond(0, [{ title: 'Partial', url: '/partial' }, null]); await flush(); await flush();
  assert.equal(retry.errors.length, 1); assert.equal(retry.cells.length, 0, '失败响应必须完整保留到生成结束后才追加'); cleanup();
  cleanup = await retry.mount();
  retry.respond(1, [{ title: 'Recovered', url: '/recovered' }]); await flush(); await flush();
  assert.equal(retry.cells.length, 1); assert.ok(retry.cells[0].html.includes('Recovered')); cleanup();
});

test('sites缺少labels仍展示，并按文本、属性和颜色上下文处理远端内容', async () => {
  const f = fixture('sites', '/fixture/sites'); const cleanup = await f.mount();
  f.respond(0, { content: [{ title: '<b>Site</b>', description: '<img onerror=alert(1)>', url: 'javascript:alert(2)', cover: 'x" onerror="alert(3)' }] });
  await flush(); await flush();
  assert.equal(f.errors.length, 0); assert.equal(f.cells.length, 1);
  assert.ok(f.cells[0].html.includes('&lt;b&gt;Site&lt;/b&gt;'));
  assert.ok(f.cells[0].html.includes('&lt;img onerror=alert(1)&gt;'));
  assert.ok(!/href="javascript:|onerror="/.test(f.cells[0].html)); cleanup();
});

test('动态Markdown把最终完整HTML交给局部净化边界，普通字段仍作为文本', async () => {
  const f = fixture('timeline'); const cleanup = await f.mount();
  f.respond(0, [{ title: '<script>title</script>', body: '<svg onload="alert(1)"></svg>', html_url: 'javascript:alert(2)', created_at: '2026-10-26', comments: '<img onerror=alert(3)>' }]);
  await flush(); await flush();
  assert.equal(f.errors.length, 0); assert.equal(f.sanitized.length, 1);
  const { html, config } = f.sanitized[0];
  assert.ok(html.includes('&lt;script&gt;title&lt;/script&gt;'));
  assert.ok(html.includes('<svg onload="alert(1)"></svg>'), '此VM仅检查真实净化器的调用契约；HTML安全由真实浏览器核验');
  assert.ok(config.FORBID_TAGS.includes('iframe'));
  assert.equal(config.ALLOW_DATA_ATTR, false); cleanup();
});

test('第三方最新评论与微博保留默认图机制，昵称按文本处理；成功重挂不追加', async () => {
  for (const [service, data] of [
    ['giscus', [{ author: { login: '<b>Author</b>', avatarUrl: 'javascript:bad' }, body: 'Text', createdAt: '2026-10-26', url: '/comment' }]],
    ['waline', [{ nick: '<b>Author</b>', comment: '<p>Comment</p>', avatar: 'javascript:bad', time: '2026-10-26', url: '/comment', objectId: '1' }]],
    ['weibo', { user: { nick_name: '<b>Author</b>', avatar_hd: 'javascript:bad' }, tweets: [{ created_at: '2026-10-26', content: 'Tweet', url: '/tweet', reposts_count: 1, comments_count: 2, attitudes_count: 3 }] }],
  ]) {
    const f = fixture(service); let cleanup = await f.mount();
    f.respond(0, data); await flush(); await flush();
    assert.equal(f.errors.length, 0, service); assert.equal(f.cells.length, 1, service);
    assert.ok(f.cells[0].html.includes('&lt;b&gt;Author&lt;/b&gt;'), service);
    assert.ok(f.cells[0].html.includes('data-fallback-src="/avatar.png"'), service);
    assert.ok(!/onerror=|src="javascript:/i.test(f.cells[0].html));
    cleanup(); cleanup = await f.mount(); assert.equal(f.requests.length, 1, service); cleanup();
  }
});

test('Twikoo最新评论保持只读POST协议并获得取消、成功重挂幂等行为', async () => {
  const f = fixture('twikoo', 'https://twikoo.example.test'); let cleanup = await f.mount();
  assert.equal(f.requests[0].options.method, 'POST');
  assert.deepEqual(JSON.parse(f.requests[0].options.body), { event: 'GET_RECENT_COMMENTS', envId: 'https://twikoo.example.test', pageSize: 10, includeReply: true });
  f.respond(0, { data: [{ commentText: '<img onerror=alert(1)>', nick: '<b>Author</b>', url: 'javascript:bad', id: '1', created: '2026-10-26' }] });
  await flush(); await flush(); assert.equal(f.errors.length, 0); assert.equal(f.cells.length, 1);
  assert.ok(f.cells[0].html.includes('&lt;b&gt;Author&lt;/b&gt;'));
  assert.ok(!/href="javascript:/i.test(f.cells[0].html));
  cleanup(); cleanup = await f.mount(); assert.equal(f.requests.length, 1); cleanup();
  const late = fixture('twikoo', 'https://twikoo.example.test'); cleanup = await late.mount(); cleanup();
  late.respond(0, { data: [{ commentText: 'Late', created: '2026-10-26' }] }); await flush(); await flush();
  assert.equal(late.cells.length, 0); assert.equal(late.errors.length, 0);
});

test('JSON Feed与Artalk保留富正文，普通作者标题和解码后的预览文本不变成HTML', async () => {
  const feed = fixture('rss'); let cleanup = await feed.mount();
  feed.respond(0, { version: 'https://jsonfeed.org/version/1.1', authors: [{ name: '<b>Author</b>' }], items: [{ id: '1" onload="alert(1)', title: '<img onerror=alert(2)>', url: 'javascript:bad', content_html: '<strong>Rich body</strong>', date_published: '2026-10-26' }] });
  await flush(); await flush(); assert.equal(feed.errors.length, 0); assert.equal(feed.cells.length, 1);
  assert.ok(feed.cells[0].html.includes('&lt;b&gt;Author&lt;/b&gt;'));
  assert.ok(feed.cells[0].html.includes('&lt;img onerror=alert(2)&gt;'));
  assert.ok(feed.sanitized[0].html.includes('<strong>Rich body</strong>'));
  assert.ok(!/href="javascript:|onload="/.test(feed.cells[0].html)); cleanup();

  const artalk = fixture('artalk'); cleanup = await artalk.mount();
  artalk.respond(0, { data: [{ nick: '<b>Author</b>', content_marked: '<p>&lt;img src=x onerror=alert(1)&gt;</p>', page_url: '/comment', id: '1', date: '2026-10-26' }] });
  await flush(); await flush(); assert.equal(artalk.errors.length, 0); assert.equal(artalk.cells.length, 1);
  assert.ok(artalk.cells[0].html.includes('&lt;b&gt;Author&lt;/b&gt;'));
  assert.ok(artalk.cells[0].html.includes('&lt;img src=x onerror=alert(1)&gt;')); cleanup();
});


function feedDocument(rootName, namespaceURI = '', malformed = false) {
  return { documentElement: { nodeName: rootName, namespaceURI },
    querySelector: selector => selector === 'parsererror' && malformed ? {} : null,
    querySelectorAll: () => [],
  };
}

test('RSS解析失败和未知XML根进入失败路径，同一DOM重新挂载可以重试', async () => {
  for (const [badDocument, message] of [
    [feedDocument('parsererror', '', true), /Failed to parse feed XML/],
    [feedDocument('html'), /Unsupported feed XML root: html/],
  ]) {
    const documents = [badDocument, feedDocument('rss')];
    const f = fixture('rss', '/fixture/feed', undefined, { DOMParser: class {
      parseFromString(_xml, type) { assert.equal(type, 'text/xml'); return documents.shift(); }
    } });
    let cleanup = await f.mount();
    f.requests[0].resolve(new Response('<invalid/>', { headers: { 'Content-Type': 'application/xml' } }));
    await flush(); await flush();
    assert.equal(f.failures.length, 1); assert.equal(f.errors.length, 1);
    assert.match(f.errors[0].message, message); assert.equal(f.cells.length, 0); cleanup();
    cleanup = await f.mount();
    assert.equal(f.requests.length, 2, '失败不能被标记为complete而阻止同一DOM再次请求');
    f.requests[1].resolve(new Response('<rss><channel/></rss>', { headers: { 'Content-Type': 'application/xml' } }));
    await flush(); await flush();
    assert.equal(f.failures.length, 1); assert.equal(f.errors.length, 1);
    assert.ok(f.cells.every(cell => cell.html === ''), '恢复后的合法空feed不生成条目');
    cleanup(); cleanup = await f.mount();
    assert.equal(f.requests.length, 2, '合法空feed成功后重挂不重复请求'); cleanup();
  }
});

test('RSS 2.0、RSS 1.0和Atom合法空feed均成功，不以无条目误判失败', async () => {
  for (const document of [feedDocument('rss'), feedDocument('rdf:RDF'), feedDocument('feed', 'http://www.w3.org/2005/Atom')]) {
    const f = fixture('rss', '/fixture/feed', undefined, { DOMParser: class { parseFromString() { return document; } } });
    let cleanup = await f.mount();
    f.requests[0].resolve(new Response('<empty-feed/>', { headers: { 'Content-Type': 'application/xml' } }));
    await flush(); await flush();
    assert.equal(f.failures.length, 0); assert.equal(f.errors.length, 0);
    assert.ok(f.cells.every(cell => cell.html === '')); cleanup();
    cleanup = await f.mount(); assert.equal(f.requests.length, 1); cleanup();
  }
});
