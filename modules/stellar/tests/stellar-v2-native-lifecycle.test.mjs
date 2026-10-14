import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import vm from 'node:vm';

const flush = () => new Promise(resolve => setImmediate(resolve));
function source(path) {
  if (process.env.BLOG_RUNTIME_BASELINE === '1') {
    try { return execFileSync('git', ['show', '379adf9:modules/stellar/web/assets/js/' + path], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'ignore'] }); }
    catch { return ''; }
  }
  return readFileSync(new URL('../web/assets/js/' + path, import.meta.url), 'utf8');
}
class Element extends EventTarget {
  constructor() { super(); this.map = {}; this.attrs = new Map(); this.style = {}; this.dataset = {}; this.textContent = ''; this.classList = { add() {}, remove() {} }; }
  querySelector(selector) { return this.map[selector] || null; }
  querySelectorAll(selector) { return this.map[selector] ? [this.map[selector]] : []; }
  getElementsByClassName() { return []; }
  getAttribute(name) { return this.attrs.get(name) || null; }
  setAttribute(name, value) { this.attrs.set(name, value); }
  removeAttribute(name) { this.attrs.delete(name); }
  contains(element) { return Object.values(this.map).includes(element); }
  focus() {}
  closest(selector) { return this.closestMap?.[selector] || null; }
}
function fixture() {
  const document = new Element(); document.readyState = 'complete'; document.body = new Element();
  document.documentElement = new Element(); document.getElementById = id => document.map['#' + id] || null;
  document.createElement = () => new Element();
  const window = new Element(); window.document = document; document.defaultView = window;
  const requests = [], timers = new Map(), frames = new Map(); let id = 0;
  window.fetch = (url, options) => new Promise(resolve => requests.push({ url, options, resolve }));
  window.setTimeout = window.setInterval = fn => { timers.set(++id, fn); return id; };
  window.clearTimeout = window.clearInterval = id => timers.delete(id);
  window.requestAnimationFrame = fn => { frames.set(++id, fn); return id; };
  window.cancelAnimationFrame = id => frames.delete(id);
  const localStorage = { getItem() { return null; }, setItem() {} };
  const sandbox = { window, document, Event, AbortController, Proxy, Promise, WeakMap, Set, console, localStorage,
    fetch: window.fetch, setTimeout: window.setTimeout, clearTimeout: window.clearTimeout, setInterval: window.setInterval, clearInterval: window.clearInterval,
    requestAnimationFrame: window.requestAnimationFrame };
  vm.createContext(sandbox);
  const load = path => vm.runInContext(source(path), sandbox, { filename: path });
  load('blog/lifecycle.js');
  function region(map = {}) { const root = new Element(); root.ownerDocument = document; root.map = map; Object.assign(document.map, map); return root; }
  const context = () => ({ signal: new AbortController().signal, onCleanup() {} });
  return { window, document, requests, timers, frames, load, region, context, sandbox };
}
test('Blog region卸载取消请求、事件、计时器及动画帧，重新挂载只有一个处理器', async () => {
  const f = fixture(), root = f.region(); const scope = f.window.blogRuntime.createScope(root, f.context());
  let clicks = 0, writes = 0;
  scope.listen(root, 'click', () => clicks++); scope.setTimeout(() => writes++, 100); scope.setInterval(() => writes++, 100); scope.requestAnimationFrame(() => writes++);
  const pending = scope.fetch('/fixture').then(response => response.json()).then(() => writes++).catch(() => writes++).finally(() => writes++);
  root.dispatchEvent(new Event('click')); assert.equal(clicks, 1);
  scope.cleanup(); scope.cleanup();
  assert.equal(f.requests[0].options.signal.aborted, true); assert.equal(f.timers.size, 0); assert.equal(f.frames.size, 0);
  f.requests[0].resolve({ json: async () => ({}) }); await pending;
  root.dispatchEvent(new Event('click')); assert.equal(clicks, 1); assert.equal(writes, 0);
  const replacement = f.window.blogRuntime.createScope(root, f.context()); replacement.listen(root, 'click', () => clicks++);
  root.dispatchEvent(new Event('click')); assert.equal(clicks, 2); replacement.cleanup();
});
test('AI摘要生成离页后不渲染迟到成功、不报网络错、不回写按钮', async () => {
  const f = fixture(); f.load('ai-summary.js');
  const panel = new Element(), button = new Element(), endpoint = new Element(), model = new Element(), key = new Element();
  endpoint.value = 'https://fixture.invalid'; model.value = 'fixture'; key.value = 'secret'; button.textContent = '生成';
  panel.map = { '.ai-summary-generate': button, '.ai-summary-endpoint': endpoint, '.ai-summary-model': model, '.ai-summary-key': key };
  button.closestMap = { '.ai-summary-generate': button, '#ai-summary': panel };
  const root = f.region({ '#ai-summary': panel }); const cleanup = f.window.blogAISummary.mount(root, f.context());
  const event = new Event('click'); Object.defineProperty(event, 'target', { value: button }); root.dispatchEvent(event);
  assert.equal(f.requests.length, 1); assert.equal(button.disabled, true);
  cleanup(); assert.equal(f.requests[0].options.signal.aborted, true);
  f.requests[0].resolve({ ok: true, json: async () => ({ summary: '迟到摘要' }) }); await flush(); await flush();
  assert.equal(button.disabled, true); assert.equal(f.timers.size, 0);
});
test('原生评论旧身份请求取消，新页身份只更新自己的region；卸载移除初始化标记', async () => {
  const f = fixture(); f.load('comments.js');
  function commentRegion() {
    const body = new Element(), anon = new Element(), logged = new Element(), nick = new Element();
    return { body, nick, root: f.region({ '#cmt-body': body, '#cmt-id-anon': anon, '#cmt-id-logged': logged, '#cmt-id-nick': nick }) };
  }
  const previous = commentRegion(); const cleanup = f.window.stellar.comments.mount(previous.root, f.context());
  assert.equal(f.requests.length, 1); cleanup(); assert.equal(f.requests[0].options.signal.aborted, true);
  assert.equal(previous.body.getAttribute('data-cmt-init'), null);
  const next = commentRegion(); const cleanupNext = f.window.stellar.comments.mount(next.root, f.context());
  f.requests[0].resolve({ json: async () => ({ id: 1, nickname: '旧身份' }) });
  f.requests[1].resolve({ json: async () => ({ id: 2, nickname: '新身份' }) }); await flush(); await flush();
  assert.equal(previous.nick.textContent, ''); assert.equal(next.nick.textContent, '新身份');
  cleanupNext(); assert.equal(next.body.getAttribute('data-cmt-init'), null);
});
test('原生评论身份请求未完成时，进入视口仍加载公开列表，卸载取消两个请求', async () => {
  const f = fixture(), observers = [];
  f.window.IntersectionObserver = class {
    constructor(callback) { this.callback = callback; observers.push(this); }
    observe(element) { this.element = element; }
    disconnect() { this.disconnected = true; }
  };
  f.load('comments.js');
  const body = new Element(), list = new Element(), head = new Element(), empty = new Element();
  body.setAttribute('data-page-key', '/post/native');
  head.map['.cmt-list-title'] = new Element();
  list.innerHTML = '加载评论中…';
  const root = f.region({ '#cmt-body': body, '#cmt-list': list, '#cmt-list-head': head, '#cmt-empty': empty });
  const cleanup = f.window.stellar.comments.mount(root, f.context());
  assert.equal(f.requests[0].url, '/api/auth/me');
  assert.equal(observers.length, 1, '列表观察不应等待身份响应');
  assert.equal(f.requests.length, 1, '进入视口前保持懒加载');
  observers[0].callback([{ isIntersecting: true }]);
  assert.equal(f.requests[1].url, '/api/comments?page_key=%2Fpost%2Fnative');
  f.requests[1].resolve({ ok: true, json: async () => ({ ok: true, total: 0, comments: [] }) });
  await flush(); await flush();
  assert.equal(list.innerHTML, '');
  assert.equal(empty.style.display, 'flex');
  cleanup();
  assert.equal(f.requests[0].options.signal.aborted, true);
  assert.equal(f.requests[1].options.signal.aborted, true);
  assert.equal(observers[0].disconnected, true);
  f.requests[0].resolve({ json: async () => ({ id: 1 }) });
  await flush();
  assert.equal(body.getAttribute('data-cmt-init'), null);
});
test('原生评论区域登录按钮打开本组件弹窗，卸载解绑表单，重挂只发一次登录请求', async () => {
  const f = fixture(); f.load('comments.js');
  const body = new Element(), modal = new Element(), chooser = new Element(), auth = new Element();
  const button = new Element(), email = new Element(), close = new Element(), form = new Element();
  const username = new Element(), password = new Element(), submit = new Element();
  username.value = 'fixture'; password.value = 'fixture-password'; modal.style.display = 'none';
  button.closestMap = { '#cmt-submit': button };
  const root = f.region({ '#cmt-body': body, '#cmt-modal': modal, '#cmt-modal-chooser': chooser,
    '#cmt-modal-auth': auth, '#cmt-submit': button, '#cmt-choose-email': email, '#cmt-modal-close': close,
    '#cmt-form-login': form, '#cmt-login-username': username, '#cmt-login-password': password, '#cmt-do-login': submit });
  const cleanup = f.window.stellar.comments.mount(root, f.context());
  f.requests[0].resolve({ json: async () => ({ id: null }) }); await flush(); await flush();
  const click = new Event('click'); Object.defineProperty(click, 'target', { value: button }); root.dispatchEvent(click);
  assert.equal(modal.style.display, 'flex');
  email.dispatchEvent(new Event('click')); assert.equal(auth.style.display, 'flex'); assert.equal(chooser.style.display, 'none');
  close.dispatchEvent(new Event('click')); assert.equal(modal.style.display, 'none');
  root.dispatchEvent(click); const escape = new Event('keydown'); Object.defineProperty(escape, 'key', { value: 'Escape' });
  root.dispatchEvent(escape); assert.equal(modal.style.display, 'none');
  form.dispatchEvent(new Event('submit', { cancelable: true }));
  assert.equal(f.requests.filter(request => request.url === '/api/auth/login').length, 1);
  cleanup(); root.dispatchEvent(click); form.dispatchEvent(new Event('submit', { cancelable: true }));
  assert.equal(modal.style.display, 'none');
  assert.equal(f.requests.filter(request => request.url === '/api/auth/login').length, 1);
  assert.equal(f.requests.find(request => request.url === '/api/auth/login').options.signal.aborted, true);
  const cleanupNext = f.window.stellar.comments.mount(root, f.context());
  f.requests.filter(request => request.url === '/api/auth/me').at(-1).resolve({ json: async () => ({ id: null }) }); await flush(); await flush();
  root.dispatchEvent(click);
  assert.equal(modal.style.display, 'flex'); form.dispatchEvent(new Event('submit', { cancelable: true }));
  assert.equal(f.requests.filter(request => request.url === '/api/auth/login').length, 2, '重挂后不能重复提交');
  cleanupNext();
});
test('原生注册验证码离页取消，迟到题目不写旧区域，重新挂载获取新题', async () => {
  const f = fixture(); f.load('comments.js');
  const body = new Element(), register = new Element(), question = new Element(), answer = new Element(), submit = new Element();
  const root = f.region({ '#cmt-body': body, '#cmt-goto-register': register, '#cmt-reg-question': question,
    '#cmt-reg-captcha': answer, '#cmt-do-register': submit });
  const cleanup = f.window.stellar.comments.mount(root, f.context());
  register.dispatchEvent(new Event('click'));
  const previous = f.requests.find(request => request.url === '/api/auth/register/challenge');
  assert.ok(previous); assert.equal(submit.disabled, true); assert.equal(question.textContent, '加载中…');
  cleanup(); assert.equal(previous.options.signal.aborted, true);
  previous.resolve({ ok: true, json: async () => ({ token: 'old', question: '旧题' }) }); await flush(); await flush();
  assert.equal(question.textContent, '加载中…'); assert.equal(submit.disabled, true);
  register.dispatchEvent(new Event('click'));
  assert.equal(f.requests.filter(request => request.url === '/api/auth/register/challenge').length, 1);
  const cleanupNext = f.window.stellar.comments.mount(root, f.context()); register.dispatchEvent(new Event('click'));
  const next = f.requests.filter(request => request.url === '/api/auth/register/challenge').at(-1);
  next.resolve({ ok: true, json: async () => ({ token: 'new', question: '新题' }) }); await flush(); await flush();
  assert.equal(question.textContent, '新题'); assert.equal(submit.disabled, false); cleanupNext();
});
test('RAG问答离页取消，不回写输出、按钮或密钥输入', async () => {
  const f = fixture(); f.load('search/ai-ask.js');
  const panel = new Element(), form = new Element(), input = new Element(), button = new Element(), key = new Element(), output = new Element();
  input.value = '问题'; key.value = 'fixture-key'; button.textContent = '发送';
  panel.map = { '[data-rag-ai-form]': form, '[data-rag-ai-input]': input, '[data-rag-ai-key]': key, '.search-ai-ask-submit': button, '[data-rag-ai-result]': output };
  form.closestMap = { '[data-rag-ai-form]': form, '#rag-ai-ask': panel };
  const root = f.region({ '#rag-ai-ask': panel }); const cleanup = f.window.blogAIAsk.mount(root, f.context());
  const event = new Event('submit', { cancelable: true }); Object.defineProperty(event, 'target', { value: form }); root.dispatchEvent(event);
  assert.equal(f.requests.length, 1); cleanup(); assert.equal(f.requests[0].options.signal.aborted, true);
  f.requests[0].resolve({ ok: true, json: async () => ({ answer: '迟到答案' }) }); await flush(); await flush();
  assert.equal(output.textContent, ''); assert.equal(button.disabled, true); assert.equal(key.value, 'fixture-key'); assert.equal(f.timers.size, 0);
});
test('Blog friends旧正文解码取消不追加卡片，成功后再挂载不重复追加', async () => {
  const f = fixture();
  f.document.currentScript = {};
  const appended = [];
  f.window.utils = { dom() { return { find() { return { append(html) { appended.push(html); } }; } }; } };
  f.window.wrapLazyloadImages = () => {};
  // 服务读取既有ctx/def，与真实页面的defines一致。
  const serviceSource = 'var def = {avatar:"/avatar"};\n' + source('blog/services/friends.js');
  const serviceSandbox = { window: f.window, document: f.document, Promise, WeakSet, console };
  vm.runInNewContext(serviceSource, serviceSandbox);
  const service = f.document.currentScript.stellarMount;
  const element = new Element(); element.ownerDocument = f.document; element.dataset.api = '/fixture/friends';
  element.matches = selector => selector === '.ds-friends';
  let resolveBody;
  const context = { ...f.context(), reportError(error) { throw error; }, request: { request: async () => ({ json: () => new Promise(resolve => { resolveBody = resolve; }) }) } };
  const cleanup = service(element, context);
  await flush(); cleanup(); resolveBody([{ title: '迟到友链', url: 'https://fixture.test' }]); await flush();
  assert.equal(appended.length, 0);
  const loaded = { ...f.context(), reportError(error) { throw error; }, request: { request: async () => ({ json: async () => [{ title: '友链', url: 'https://fixture.test' }] }) } };
  const first = service(element, loaded); await flush(); assert.equal(appended.length, 1); first();
  const second = service(element, loaded); await flush(); assert.equal(appended.length, 1); second();
});

test('Blog Go搜索快速换词与卸载取消服务端及索引请求，迟到响应不写结果', async () => {
  const f = fixture(); const input = new Element(), result = new Element(), wrapper = new Element();
  input.value = ''; result.innerHTML = '';
  wrapper.map = { '.search-input': input, '.search-result': result };
  Object.assign(f.document.map, { '.search-wrapper': wrapper });
  f.sandbox.URLSearchParams = URLSearchParams;
  f.sandbox.ctx = { root: '/', search: { path: 'search.json', local_search: { lazy_load: true, cache_ttl: 0 } } };
  f.sandbox.MutationObserver = class { observe() {} disconnect() {} };
  f.load('blog/search/local-search.js'); f.window.blogLocalSearch.mount(); input.dispatchEvent(new Event('focus'));
  input.value = 'first'; input.dispatchEvent(new Event('input'));
  input.value = 'second'; input.dispatchEvent(new Event('input'));
  assert.equal(f.requests.length, 2); assert.equal(f.requests[0].options.signal.aborted, true);
  f.window.blogLocalSearch.unmount(); assert.equal(f.requests[1].options.signal.aborted, true);
  f.requests.forEach(request => request.resolve({ ok: true, json: async () => ({ items: [{ title: 'late', path: '/late' }] }) }));
  await flush(); await flush(); assert.equal(result.innerHTML, '');
  f.window.blogLocalSearch.mount(); input.dispatchEvent(new Event('focus'));
  input.dispatchEvent(new Event('focus')); // 已初始化后按需请求后台索引。
  const index = f.requests.find(request => request.url === '/search.json'); assert.ok(index);
  f.window.blogLocalSearch.unmount(); assert.equal(index.options.signal.aborted, true);
  index.resolve({ ok: true, json: async () => [] }); await flush();
});
test('Blog分享和打赏使用当前region数据；离页后剪贴板结果没有迟到toast或事件', async t => {
  const f = fixture(); const footer = new Element(), share = new Element(), reward = new Element(), codes = new Element();
  footer.setAttribute('data-url', 'https://fixture.test/post'); footer.setAttribute('data-title', '标题'); share.setAttribute('data-platform', 'link');
  share.closestMap = { '#share .share-item': share }; reward.closestMap = { '#reward-toggle': reward };
  const classes = new Set(); codes.classList = { contains: name => classes.has(name), toggle(name, enabled) { enabled ? classes.add(name) : classes.delete(name); } };
  const root = f.region({ '#post-footer, #share': footer, '#reward-codes': codes });
  const descriptors = new Map(['window', 'document', 'location', 'navigator'].map(key => [key, Object.getOwnPropertyDescriptor(globalThis, key)]));
  let completeCopy, copied = '', toasts = 0;
  f.window.stellar = { toast() { toasts++; } };
  for (const [key, value] of Object.entries({ window: f.window, document: f.document, location: { href: 'https://fixture.test/' }, navigator: { clipboard: { writeText(value) { copied = value; return new Promise(resolve => { completeCopy = resolve; }); } } } })) {
    Object.defineProperty(globalThis, key, { configurable: true, value });
  }
  t.after(() => { for (const [key, descriptor] of descriptors) { if (descriptor) Object.defineProperty(globalThis, key, descriptor); else delete globalThis[key]; } });
  const { mount } = await import('../web/assets/js/runtime/extensions/blog-content.js');
  const cleanup = await mount(root, { ...f.context(), extension: { config: {} }, assets: { script: async () => {} } });
  function click(target) { const event = new Event('click', { cancelable: true }); Object.defineProperty(event, 'target', { value: target }); root.dispatchEvent(event); }
  click(reward); assert.equal(reward.getAttribute('aria-expanded'), 'true'); assert.equal(codes.inert, false);
  click(reward); assert.equal(reward.getAttribute('aria-expanded'), 'false'); assert.equal(codes.inert, true);
  click(share); assert.equal(copied, 'https://fixture.test/post'); cleanup(); completeCopy(); await flush();
  assert.equal(toasts, 0); click(reward); assert.equal(reward.getAttribute('aria-expanded'), 'false');
});
