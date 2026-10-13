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
  const document = { createElement: () => new Element() };
  const wrappers = Array.from({ length: count }, () => {
    const wrapper = new Element(), input = new Element(), result = new Element();
    input.value = '';
    input.ownerDocument = document;
    wrapper.querySelector = selector => ({ '.search-input': input, '.search-result': result })[selector];
    return { wrapper, input, result };
  });
  document.querySelectorAll = () => wrappers.map(item => item.wrapper);
  const window = { searchConfig: { appId: 'fixture', apiKey: 'public-key', indexName: 'fixture', hitsPerPage: 10 } };
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
  return { requests, wrappers, mounted, unmounted, cleanup, input, reply, title };
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
