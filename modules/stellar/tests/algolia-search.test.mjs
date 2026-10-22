// 延迟真实 Promise 返回，验证输入顺序而非网络返回顺序决定搜索结果。
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

const source = readFileSync(new URL('../web/assets/js/search/algolia-search.js', import.meta.url), 'utf8');
const flush = () => new Promise(resolve => setImmediate(resolve));

class Element extends EventTarget {
  constructor() {
    super();
    this.children = [];
    this.attrs = new Map();
    const classes = new Set();
    this.classList = {
      add: (...names) => names.forEach(name => classes.add(name)),
      remove: (...names) => names.forEach(name => classes.delete(name)),
      contains: name => classes.has(name),
    };
  }
  appendChild(child) { this.children.push(child); }
  replaceChildren(...children) { this.children = children; }
  setAttribute(name, value) { this.attrs.set(name, value); }
  getAttribute(name) { return this.attrs.get(name) ?? null; }
}

function fixture(count = 1) {
  const requests = [], mounted = [], unmounted = [];
  const document = new EventTarget();
  document.createElement = () => new Element();
  const navigationListeners = new Set();
  const addEventListener = document.addEventListener.bind(document);
  const removeEventListener = document.removeEventListener.bind(document);
  document.addEventListener = (type, listener, ...options) => {
    if (type === 'stellar:navigation-complete') navigationListeners.add(listener);
    addEventListener(type, listener, ...options);
  };
  document.removeEventListener = (type, listener, ...options) => {
    if (type === 'stellar:navigation-complete') navigationListeners.delete(listener);
    removeEventListener(type, listener, ...options);
  };
  const wrappers = Array.from({ length: count }, () => {
    const wrapper = new Element(), input = new Element(), result = new Element();
    input.value = '';
    input.ownerDocument = document;
    wrapper.querySelector = selector => ({ '.search-input': input, '.search-result': result })[selector];
    return { wrapper, input, result };
  });
  document.querySelectorAll = () => wrappers.map(item => item.wrapper);
  const window = {
    location: new URL('https://fixture.test/first/'),
    searchConfig: { appId: 'fixture', apiKey: 'public-key', indexName: 'fixture', hitsPerPage: 10 },
  };
  document.defaultView = window;
  vm.runInNewContext(source, {
    window, document,
    ctx: { ui: { classes: { interactiveSpotlight: 'interactive-spotlight' } } },
    stellar: { cardHover: { mountAll: root => mounted.push(root), unmountAll: root => unmounted.push(root) } },
    MutationObserver: class { observe() {} disconnect() {} },
    algoliasearch: () => ({ initIndex: () => ({
      search(query, options) {
        return new Promise(resolve => requests.push({ query, options, resolve }));
      },
    }) }),
  });
  const cleanup = window.stellarAlgoliaSearch.mount(document);
  function input(value, index = 0) {
    wrappers[index].input.value = value;
    wrappers[index].input.dispatchEvent(new Event('input'));
  }
  async function reply(index, title) {
    requests[index].resolve({ hits: [{ url: '/result/' + title, hierarchy: { lvl1: title }, _snippetResult: { content: { value: 'snippet ' + title } } }] });
    await flush();
  }
  function title(index = 0) { return wrappers[index].result.children[0]?.children[0]?.children[0]?.children[0]?.textContent; }
  function navigate(url, complete = true) {
    window.location = new URL(url, window.location);
    if (complete) {
      const event = new Event('stellar:navigation-complete');
      event.detail = { url: window.location.href };
      document.dispatchEvent(event);
    }
  }
  return { requests, wrappers, mounted, unmounted, cleanup, input, reply, title, navigate, navigationListeners,
    mount: () => window.stellarAlgoliaSearch.mount(document) };
}

test('Algolia 新查询先返回后，旧查询迟到不会覆盖结果或重新挂载卡片', async () => {
  const f = fixture();
  f.input('first'); f.input('second');
  assert.equal(f.requests[0].query, 'first');
  assert.equal(f.requests[1].options.hitsPerPage, 10);
  await f.reply(1, 'second');
  assert.equal(f.title(), 'second');
  await f.reply(0, 'first');
  assert.equal(f.title(), 'second');
  assert.equal(f.mounted.length, 1);
});

test('Algolia 清空输入后，迟到结果不会重填或改变空搜索状态', async () => {
  const f = fixture();
  f.input('first');
  f.input('   ');
  assert.equal(f.wrappers[0].wrapper.getAttribute('searching'), 'false');
  await f.reply(0, 'first');
  assert.equal(f.wrappers[0].result.children.length, 0);
  assert.equal(f.wrappers[0].wrapper.getAttribute('searching'), 'false');
  assert.equal(f.mounted.length, 0);
});

test('Algolia A→B→A 输入也仅接受最后一轮 A，不能只比较查询文本', async () => {
  const f = fixture();
  f.input('same'); f.input('other'); f.input('same');
  await f.reply(2, 'latest');
  await f.reply(0, 'outdated');
  await f.reply(1, 'other');
  assert.equal(f.title(), 'latest');
  assert.equal(f.mounted.length, 1);
});

test('Algolia 每个搜索框独立接受响应，卸载后迟到响应不写 DOM', async () => {
  const f = fixture(2);
  f.input('first', 0); f.input('second', 1);
  await f.reply(0, 'first'); await f.reply(1, 'second');
  assert.equal(f.title(0), 'first'); assert.equal(f.title(1), 'second');
  f.input('pending'); f.cleanup();
  await f.reply(2, 'late');
  assert.equal(f.title(0), 'first');
  f.input('unmounted');
  assert.equal(f.requests.length, 3);
});

test('Algolia 响应抵达时再次检查当前输入值', async () => {
  const f = fixture();
  f.input('first');
  f.wrappers[0].input.value = '';
  await f.reply(0, 'first');
  assert.equal(f.wrappers[0].result.children.length, 0);
});

test('Algolia 正式 partial 完成事件使离页请求失效，保留实例与已有结果', async () => {
  const f = fixture();
  f.input('shown');
  await f.reply(0, 'shown');
  f.input('pending');
  f.navigate('/second/');
  await f.reply(1, 'late');
  assert.equal(f.title(), 'shown');
  assert.equal(f.mounted.length, 1);
  f.input('new page');
  await f.reply(2, 'new page');
  assert.equal(f.title(), 'new page');
  assert.equal(f.mounted.length, 2);
});

test('Algolia pushState 已离页但 partial 挂载未完成时，迟到响应也不写 DOM', async () => {
  for (const url of ['/second/', '/first/?page=2', 'https://other.test/first/']) {
    const f = fixture();
    f.input('pending');
    f.navigate(url, false);
    await f.reply(0, 'late');
    assert.equal(f.wrappers[0].result.children.length, 0, url);
    assert.equal(f.mounted.length, 0, url);
  }
});

test('Algolia 新页挂载期间发起的查询跨过 partial 完成事件仍可显示', async () => {
  const f = fixture();
  f.input('same');
  f.navigate('/second/', false);
  f.input('same');
  f.navigate('/second/');
  await f.reply(1, 'new page');
  assert.equal(f.title(), 'new page');
  await f.reply(0, 'old page');
  assert.equal(f.title(), 'new page');
  assert.equal(f.mounted.length, 1);
});

test('Algolia 新页挂载期间清空输入，完成事件后旧结果不能重填', async () => {
  const f = fixture();
  f.input('old page');
  f.navigate('/second/', false);
  f.input('pending new page');
  f.input('   ');
  f.navigate('/second/');
  await f.reply(0, 'old page');
  await f.reply(1, 'pending new page');
  assert.equal(f.wrappers[0].result.children.length, 0);
  assert.equal(f.wrappers[0].wrapper.getAttribute('searching'), 'false');
  assert.equal(f.mounted.length, 0);
  f.input('after clear');
  await f.reply(2, 'after clear');
  assert.equal(f.title(), 'after clear');
});

test('Algolia partial A→B→A 后不能接受离页前同 URL 的旧响应', async () => {
  const f = fixture();
  f.input('pending');
  f.navigate('/second/');
  f.navigate('/first/');
  await f.reply(0, 'late');
  assert.equal(f.wrappers[0].result.children.length, 0);
  assert.equal(f.mounted.length, 0);
});

test('Algolia 仅 hash 滚动或未提交的导航尝试保留当前查询', async () => {
  const f = fixture();
  f.input('pending');
  f.navigate('#section', false);
  await f.reply(0, 'current page');
  assert.equal(f.title(), 'current page');
});

test('Algolia cleanup 移除正式导航监听，重新挂载不会累积监听', async () => {
  const f = fixture(2);
  assert.equal(f.navigationListeners.size, 2);
  assert.equal(f.mount(), f.cleanup);
  assert.equal(f.navigationListeners.size, 2);
  f.input('pending');
  f.cleanup();
  assert.equal(f.navigationListeners.size, 0);
  f.navigate('/second/');
  await f.reply(0, 'late');
  assert.equal(f.mounted.length, 0);
  const dispose = f.mount();
  assert.equal(f.navigationListeners.size, 2);
  f.input('remounted');
  await f.reply(1, 'remounted');
  assert.equal(f.title(), 'remounted');
  dispose();
  assert.equal(f.navigationListeners.size, 0);
});
