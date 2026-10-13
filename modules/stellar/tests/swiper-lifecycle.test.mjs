import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

const source = readFileSync(new URL('../web/assets/js/runtime/extensions/swiper.js', import.meta.url), 'utf8');

function carrier(effect) {
  const controls = new Map(['.swiper-pagination', '.swiper-button-next', '.swiper-button-prev'].map(selector => [selector, { selector, effect }]));
  return {
    controls,
    matches: selector => selector === '#swiper-api',
    getAttribute: name => name === 'effect' ? effect : null,
    querySelector: selector => controls.get(selector),
    querySelectorAll: () => [],
  };
}

function fixture({ rootCarrier = false, failAt = 0 } = {}) {
  const elements = [carrier('fade'), carrier('coverflow')];
  const instances = [], loads = [];
  const runtime = vm.createContext({ window: { Swiper: class {
    constructor(element, options) {
      if (failAt === instances.length + 1) throw new Error('Swiper initialization failed');
      this.element = element;
      this.options = options;
      this.destroyCalls = [];
      instances.push(this);
    }
    destroy(...args) { this.destroyCalls.push(args); }
  } } });
  vm.runInContext(source.replace('export async function mount', 'async function mount'), runtime);
  const root = rootCarrier ? elements[0] : {
    querySelector: () => elements[0],
    querySelectorAll: selector => selector === '#swiper-api' ? elements : [],
  };
  const assets = {
    style: async path => { loads.push(path); },
    script: async path => { loads.push(path); },
  };
  const mount = signal => runtime.mount(root, { assets, signal, extension: { config: { assets: {
    localCss: '/local.css', css: '/swiper.css', js: '/swiper.js',
  } } } });
  return { root, elements, instances, loads, assets, mount };
}

test('同页 fade 与 coverflow 全部挂载，分页和前后按钮只引用各自载体', async () => {
  const f = fixture();
  const cleanup = await f.mount();
  assert.equal(f.instances.length, 2);
  for (const [index, instance] of f.instances.entries()) {
    const element = f.elements[index];
    assert.equal(instance.element, element);
    assert.equal(instance.options.effect, element.getAttribute('effect'));
    assert.equal(instance.options.pagination.el, element.controls.get('.swiper-pagination'));
    assert.equal(instance.options.navigation.nextEl, element.controls.get('.swiper-button-next'));
    assert.equal(instance.options.navigation.prevEl, element.controls.get('.swiper-button-prev'));
    assert.equal(instance.options.pagination.clickable, true);
    assert.equal(instance.options.rewind, true);
  }
  cleanup();
  cleanup();
  for (const instance of f.instances) {
    assert.equal(instance.destroyCalls.length, 1);
    assert.equal(instance.destroyCalls[0][0], true);
    assert.equal(instance.destroyCalls[0][1], true);
  }
  const cleanupAgain = await f.mount();
  assert.equal(f.instances.length, 4);
  cleanup();
  assert.equal(f.instances[2].destroyCalls.length, 0);
  cleanupAgain();
  assert.equal(f.instances[2].destroyCalls.length, 1);
  assert.equal(f.instances[3].destroyCalls.length, 1);
});

test('区域根本身是 Swiper 载体时也挂载且清理', async () => {
  const f = fixture({ rootCarrier: true });
  const cleanup = await f.mount();
  assert.equal(f.instances.length, 1);
  assert.equal(f.instances[0].element, f.root);
  cleanup();
  assert.equal(f.instances[0].destroyCalls.length, 1);
});

test('资源加载期间取消，迟到的脚本完成不会实例化旧页', async () => {
  const f = fixture();
  let resolveLoad;
  f.assets.script = () => new Promise(resolve => { resolveLoad = resolve; });
  const controller = new AbortController();
  const pending = f.mount(controller.signal);
  controller.abort();
  resolveLoad();
  await assert.rejects(pending, { name: 'AbortError' });
  assert.equal(f.instances.length, 0);
  f.assets.script = async () => {};
  const cleanup = await f.mount(new AbortController().signal);
  assert.equal(f.instances.length, 2);
  cleanup();
  assert.ok(f.instances.every(instance => instance.destroyCalls.length === 1));
});

test('资源失败不实例化，后一个构造失败会清理此前成功的实例', async () => {
  const failedLoad = fixture();
  failedLoad.assets.style = async () => { throw new Error('CSS failed'); };
  await assert.rejects(failedLoad.mount(), /CSS failed/);
  assert.equal(failedLoad.instances.length, 0);
  const partial = fixture({ failAt: 2 });
  await assert.rejects(partial.mount(), /Swiper initialization failed/);
  assert.equal(partial.instances.length, 1);
  assert.equal(partial.instances[0].destroyCalls.length, 1);
});

test('没有载体的区域不创建空实例', async () => {
  const f = fixture();
  f.root.querySelectorAll = () => [];
  const cleanup = await f.mount();
  assert.equal(f.instances.length, 0);
  cleanup();
});
