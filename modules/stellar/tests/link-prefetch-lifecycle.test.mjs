import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

const source = path => readFileSync(new URL(path, import.meta.url), 'utf8');
const importSource = path => import('data:text/javascript;base64,' + Buffer.from(source(path)).toString('base64'));
const { createAssetLoader } = await importSource('../web/assets/js/runtime/asset-loader.js');
const { createExtensionRegistry } = await importSource('../web/assets/js/runtime/extension-registry.js');
function fixture({ native = true, connection } = {}) {
  let next = 1;
  const timers = new Map(), intervals = new Map(), idle = new Map(), observers = [], links = [], scripts = [], requests = [];
  const listeners = new Map();
  const anchor = { href: 'https://example.test/next/', closest() { return this; } };
  const document = {
    querySelectorAll() { return [anchor]; },
    createElement(tag) {
      const element = new EventTarget();
      Object.assign(element, { tag, relList: { supports: () => native }, setAttribute() {}, remove() { this.removed = true; } });
      return element;
    },
    head: { appendChild(element) { (element.tag === 'script' ? scripts : links).push(element); } },
    addEventListener(type, callback) { const group = listeners.get(type) || new Set(); group.add(callback); listeners.set(type, group); },
    removeEventListener(type, callback) { listeners.get(type)?.delete(callback); }
  };
  const context = vm.createContext({ document, navigator: { connection }, Promise, Set,
    setTimeout(fn) { const id = next++; timers.set(id, fn); return id; },
    clearTimeout(id) { timers.delete(id); },
    setInterval(fn) { const id = next++; intervals.set(id, fn); return id; },
    clearInterval(id) { intervals.delete(id); },
    IntersectionObserver: class {
      constructor(callback) { this.callback = callback; this.targets = new Set(); observers.push(this); }
      observe(target) { this.targets.add(target); }
      unobserve(target) { this.targets.delete(target); }
      disconnect() { this.targets.clear(); this.disconnected = true; }
    },
    XMLHttpRequest: class {
      constructor() { requests.push(this); }
      open(method, url) { this.method = method; this.url = url; }
      send() { this.sent = true; }
      abort() { this.aborted = true; this.onabort?.(); }
    }
  });
  context.window = context;
  context.location = { origin: 'https://example.test', href: 'https://example.test/current/' };
  context.requestIdleCallback = fn => { const id = next++; idle.set(id, fn); return id; };
  context.cancelIdleCallback = id => idle.delete(id);
  vm.runInContext(source('../web/assets/vendor/flying-pages.min.js'), context);
  vm.runInContext(source('../web/assets/js/runtime/extensions/link-prefetch.js').replace('export async function mount', 'async function mount'), context);
  const flush = map => { const batch = [...map]; map.clear(); batch.forEach(([, fn]) => fn()); };
  const fire = (type, target = anchor) => [...(listeners.get(type) || [])].forEach(fn => fn({ target }));
  const count = () => ({ intervals: intervals.size, timers: timers.size, idle: idle.size,
    listeners: [...listeners.values()].reduce((sum, group) => sum + group.size, 0),
    observers: observers.filter(observer => !observer.disconnected).length });
  const assets = { script: async () => {} };
  const mount = signal => context.mount(document, { assets, signal, extension: { config: { asset: '/flying-pages.js' } } });
  return { context, document, anchor, assets, mount, count, timers, intervals, idle, observers, links, scripts, requests, flush, fire };
}
const empty = { intervals: 0, timers: 0, idle: 0, listeners: 0, observers: 0 };

test('预取 mount→cleanup→remount 清理全部句柄且恢复真实队列预取', async () => {
  const f = fixture();
  let cleanup = await f.mount();
  assert.equal(f.count().intervals, 1);
  f.flush(f.idle); f.flush(f.timers);
  f.observers.at(-1).callback([{ target: f.anchor, isIntersecting: true }]);
  [...f.intervals.values()][0]();
  assert.equal(f.links.length, 1);
  assert.equal(f.links[0].href, f.anchor.href);
  f.links[0].onload(); await Promise.resolve();
  cleanup(); cleanup();
  assert.deepEqual(f.count(), empty);
  cleanup = await f.mount();
  f.fire('mouseover', { href: 'https://example.test/second/', closest() { return this; } });
  f.flush(f.timers);
  assert.equal(f.links.length, 2);
  cleanup();
  assert.deepEqual(f.count(), empty);
  assert.ok(f.links.every(link => link.removed));
});

test('资源 pending 时 abort 不启动过期预取', async () => {
  const f = fixture();
  let resolveLoad, calls = 0, loaded;
  f.assets.script = () => { if (!loaded) { calls++; loaded = new Promise(resolve => { resolveLoad = resolve; }); } return loaded; };
  const controller = new AbortController();
  const pending = f.mount(controller.signal);
  controller.abort(); resolveLoad();
  await assert.rejects(pending, { name: 'AbortError' });
  assert.deepEqual(f.count(), empty);
  const cleanup = await f.mount(new AbortController().signal);
  assert.equal(calls, 1);
  cleanup(); assert.deepEqual(f.count(), empty);
});

test('直接重挂载去重，hover/idle/delay/pending request 都可取消', async () => {
  const f = fixture({ native: false });
  const oldCleanup = await f.mount();
  const cleanup = await f.mount();
  assert.equal(f.count().intervals, 1);
  assert.equal(f.count().listeners, 3);
  oldCleanup();
  assert.equal(f.count().intervals, 1);
  f.fire('touchstart');
  assert.equal(f.requests.length, 1);
  assert.equal(f.requests[0].withCredentials, true);
  f.fire('mouseover', { href: 'https://example.test/hover/', closest() { return this; } });
  cleanup();
  assert.equal(f.requests[0].aborted, true);
  assert.deepEqual(f.count(), empty);
  f.flush(f.idle); f.flush(f.timers);
  assert.equal(f.requests.length, 1);
});

test('慢网络/saveData 与官方同源、关键字、当前位置和去重规则保持', async () => {
  for (const connection of [{ saveData: true }, { effectiveType: '2g' }, { effectiveType: 'slow-2g' }]) {
    const f = fixture({ connection });
    const cleanup = await f.mount(); cleanup();
    assert.deepEqual(f.count(), empty);
  }
  const f = fixture();
  const cleanup = f.context.flyingPages({ ignoreKeywords: ['/ignore/'], maxRPS: 5, hoverDelay: 25 });
  for (const href of ['https://other.test/a/', f.context.location.href, 'https://example.test/ignore/a/', f.anchor.href, f.anchor.href]) {
    f.fire('touchstart', { href, closest() { return this; } });
  }
  assert.equal(f.links.length, 1);
  f.links[0].onload(); await Promise.resolve();
  cleanup();
  const nextCleanup = await f.mount(); f.fire('touchstart');
  assert.equal(f.links.length, 1);
  nextCleanup(); assert.deepEqual(f.count(), empty);
});

test('网络失败停止并清理完整实例，之后重挂载可重新预取取消项', async () => {
  const f = fixture();
  const cleanup = await f.mount();
  f.fire('touchstart'); f.links[0].onerror();
  await Promise.resolve(); await Promise.resolve();
  assert.deepEqual(f.count(), empty);
  cleanup();
  const nextCleanup = await f.mount(); f.fire('touchstart');
  assert.equal(f.links.length, 2);
  nextCleanup(); assert.deepEqual(f.count(), empty);
});

// 使用生产 registry + asset loader，证明缓存去重与取消并非测试替身的行为。
test('实际 registry 卸载 pending script 后重挂载复用同一资源且无旧实例', async () => {
  const f = fixture();
  const assets = createAssetLoader({ document: f.document });
  const registry = createExtensionRegistry({ importer: async () => ({ mount: f.context.mount }) });
  registry.register({ id: 'link-prefetch', module: '/link-prefetch.js', scope: 'document', when: { always: true }, config: { asset: '/flying-pages.js' } });
  const oldMount = registry.mount(f.document, { assets });
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(f.scripts.length, 1);
  await registry.unmount(f.document);
  const oldResult = await oldMount;
  assert.equal(oldResult[0].status, 'skipped');
  assert.deepEqual(f.count(), empty);
  const nextMount = registry.mount(f.document, { assets });
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(f.scripts.length, 1);
  f.scripts[0].dispatchEvent(new Event('load'));
  assert.equal((await nextMount)[0].status, 'mounted');
  assert.equal(f.count().intervals, 1);
  f.fire('touchstart');
  assert.equal(f.links.length, 1);
  await registry.unmount(f.document);
  assert.deepEqual(f.count(), empty);
  assert.equal((await registry.mount(f.document, { assets }))[0].status, 'mounted');
  assert.equal(f.scripts.length, 1);
  assert.equal(f.count().listeners, 3);
  await registry.unmount(f.document);
  assert.deepEqual(f.count(), empty);
});

test('idle fallback、延迟观察、hover 离开及超时的回调不会离页后执行', async () => {
  const f = fixture();
  delete f.context.requestIdleCallback; delete f.context.cancelIdleCallback;
  const cleanup = await f.mount();
  f.flush(f.timers);
  assert.equal(f.count().timers, 1);
  cleanup();
  assert.deepEqual(f.count(), empty);
  const nextCleanup = await f.mount();
  f.fire('mouseover'); f.fire('mouseout');
  f.flush(f.timers);
  assert.equal(f.links.length, 0);
  f.fire('touchstart');
  f.flush(f.timers);
  assert.deepEqual(f.count(), empty);
  nextCleanup();
});
