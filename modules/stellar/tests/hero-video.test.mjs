import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const importSource = path => import('data:text/javascript;base64,' + Buffer.from(readFileSync(new URL(path, import.meta.url))).toString('base64'));
const { createExtensionRegistry } = await importSource('../web/assets/js/runtime/extension-registry.js');
const heroVideo = await importSource('../web/assets/js/runtime/extensions/hero-video.js');
const selector = '.wiki-cover-background-video';
const declaration = { id: 'hero-video', module: '/assets/js/runtime/extensions/hero-video.js', scope: 'region', when: { selector }, config: {} };
const settled = () => new Promise(resolve => setImmediate(resolve));

function video({ autoplay = true, pending = false, error, synchronous = false } = {}) {
  const node = {
    autoplay, paused: true, currentTime: 12, src: '/hero.mp4', loop: true, muted: true, playsInline: true,
    playCalls: 0, pauseCalls: 0,
    matches: value => value === selector,
    querySelectorAll: () => [],
    pause() { this.pauseCalls++; this.paused = true; },
    play() {
      this.playCalls++;
      if (synchronous && error) throw error;
      if (error) return Promise.reject(error);
      this.paused = false;
      if (pending) return new Promise((resolve, reject) => {
        this.resolvePlay = () => { this.paused = false; resolve(); };
        this.rejectPlay = reject;
      });
      return Promise.resolve();
    }
  };
  return node;
}
const region = videos => ({ matches: () => false, querySelector: value => value === selector ? videos[0] || null : null, querySelectorAll: value => value === selector ? videos : [] });
function registry(errors = [], imports = []) {
  const result = createExtensionRegistry({ importer: async module => { imports.push(module); return heroVideo; }, onError: detail => errors.push(detail) });
  result.register(declaration);
  return result;
}

test('Hero视频由实际registry的独立封面region持有；正文partial卸载不暂停封面', async () => {
  const hero = video(), cover = region([hero]), main = region([]), imports = [];
  const runtime = registry([], imports);
  assert.deepEqual(await runtime.mount(main), [{ id: 'hero-video', status: 'skipped' }]);
  assert.equal(imports.length, 0);
  assert.equal((await runtime.mount(cover))[0].status, 'mounted');
  assert.equal(hero.playCalls, 1);
  await runtime.unmount(main);
  assert.equal(hero.paused, false);
  const dispose = runtime.unmount(cover);
  assert.equal(hero.paused, true, 'abort必须同步pause，不能等待registry异步cleanup');
  assert.equal(hero.pauseCalls, 1);
  await dispose;
  await runtime.unmount(cover);
  assert.equal(hero.pauseCalls, 1);
  await runtime.mount(cover);
  assert.equal(hero.playCalls, 2);
  assert.equal(hero.paused, false);
  await runtime.unmount(cover);
  assert.equal(hero.pauseCalls, 2);
  assert.deepEqual([hero.src, hero.currentTime, hero.loop, hero.muted, hero.playsInline], ['/hero.mp4', 12, true, true, true]);
});

test('仅autoplay主动播放；精确video根与手动视频同样接受幂等取消', () => {
  for (const autoplay of [true, false]) {
    const hero = video({ autoplay }), controller = new AbortController(), cleanups = [];
    const cleanup = heroVideo.mount(hero, { signal: controller.signal, onCleanup: fn => cleanups.push(fn) });
    assert.equal(hero.playCalls, autoplay ? 1 : 0);
    if (!autoplay) hero.paused = false;
    controller.abort();
    assert.equal(hero.paused, true);
    cleanup(); cleanups.forEach(fn => fn());
    assert.equal(hero.pauseCalls, 1);
  }
});

test('预取消不play；pending play在取消后迟到完成必须再次暂停', async () => {
  const aborted = new AbortController(); aborted.abort();
  const unused = video();
  heroVideo.mount(region([unused]), { signal: aborted.signal });
  assert.equal(unused.playCalls, 0);
  assert.equal(unused.paused, true);
  for (const stop of ['abort', 'cleanup']) {
    const hero = video({ pending: true }), controller = new AbortController(), errors = [];
    const cleanup = heroVideo.mount(region([hero]), { signal: controller.signal, reportError: error => errors.push(error) });
    if (stop === 'abort') controller.abort(); else cleanup();
    assert.equal(hero.paused, true);
    assert.equal(hero.pauseCalls, 1);
    hero.resolvePlay(); await settled();
    assert.equal(hero.paused, true);
    assert.equal(hero.pauseCalls, 2);
    cleanup();
    assert.equal(hero.pauseCalls, 2);
    assert.deepEqual(errors, []);
  }
});

test('同一video重新挂载后旧play完成不能暂停新一代的播放', async () => {
  const hero = video({ pending: true }), runtime = registry(), cover = region([hero]);
  await runtime.mount(cover);
  const oldResolve = hero.resolvePlay;
  await runtime.unmount(cover);
  await runtime.mount(cover);
  hero.resolvePlay(); await settled();
  oldResolve(); await settled();
  assert.equal(hero.paused, false);
  assert.equal(hero.pauseCalls, 1);
  await runtime.unmount(cover);
});

test('浏览器播放策略/取消拒绝保持安静，其余同步与异步错误通过reportError报告', async () => {
  for (const synchronous of [true, false]) {
    for (const name of ['AbortError', 'NotAllowedError', 'NotSupportedError']) {
      const error = Object.assign(new Error('play failed'), { name });
      const hero = video({ error, synchronous }), controller = new AbortController(), errors = [];
      const cleanup = heroVideo.mount(region([hero]), { signal: controller.signal, reportError: value => errors.push(value) });
      await settled();
      assert.deepEqual(errors, name === 'NotSupportedError' ? [error] : []);
      cleanup();
    }
  }
});

test('离页后pending播放拒绝没有迟到错误；onCleanup无需abort也能同步暂停', async () => {
  const hero = video({ pending: true }), errors = [], cleanups = [];
  const cleanup = heroVideo.mount(region([hero]), { onCleanup: fn => cleanups.push(fn), reportError: error => errors.push(error) });
  cleanups[0]();
  assert.equal(hero.pauseCalls, 1);
  hero.rejectPlay(Object.assign(new Error('old playback failed'), { name: 'NotSupportedError' }));
  await settled();
  assert.deepEqual(errors, []);
  cleanup();
  assert.equal(hero.pauseCalls, 1);
});

test('普通正文video不符合精确Hero selector时不会import或获取播放所有权', async () => {
  const stock = video(); stock.paused = false;
  const imports = [], root = {
    matches: () => false,
    querySelector: value => value === '.video>video' ? stock : null,
    querySelectorAll: value => value === '.video>video' ? [stock] : []
  };
  const runtime = registry([], imports);
  assert.equal((await runtime.mount(root))[0].status, 'skipped');
  await runtime.unmount(root);
  assert.deepEqual(imports, []);
  assert.equal(stock.playCalls, 0);
  assert.equal(stock.pauseCalls, 0);
  assert.equal(stock.paused, false);
});
