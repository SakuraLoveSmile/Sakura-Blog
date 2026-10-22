import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

const source = readFileSync(new URL('../web/assets/js/plugins/voice.js', import.meta.url), 'utf8');

function fixture(kind, duration = 4) {
  const classes = (...initial) => {
    const values = new Set(initial);
    return { contains: value => values.has(value), add: value => values.add(value), remove: value => values.delete(value) };
  };
  const document = new EventTarget();
  document.documentElement = { getAttribute: () => 'light' };
  const media = new EventTarget();
  media.matches = false;
  const frames = new Map();
  let frameID = 0, clears = 0, plays = 0, pauses = 0;
  const context = new Proxy({ clearRect() { clears++; } }, { get: (target, key) => target[key] || (() => {}) });
  const canvas = { classList: classes(), style: {}, getContext: () => context };
  const pauseButton = { style: { display: 'none' } }, playButton = { style: { display: 'flex' } };
  const seconds = { innerHTML: '' }, placeholder = { style: {} }, wechat = { classList: classes() };
  const nodes = new Map([['canvas', canvas], ['.pause-btn', pauseButton], ['.play-btn', playButton],
    ['.voice-seconds', seconds], ['.voice-placeholder', placeholder], ['.wechat-voice', wechat]]);
  const voice = new EventTarget();
  voice.querySelector = selector => nodes.get(selector);
  const audio = new EventTarget();
  Object.assign(audio, { classList: classes(kind), duration, currentTime: 0, parentElement: voice,
    play() { plays++; }, pause() { pauses++; } });
  const window = { devicePixelRatio: 1, matchMedia: () => media,
    requestAnimationFrame(callback) { const id = ++frameID; frames.set(id, callback); return id; },
    cancelAnimationFrame(id) { frames.delete(id); } };
  const runtime = vm.createContext({ window, document, console, utils: {
    requestAnimationFrame: callback => { window.requestAnimationFrame(callback); },
  } });
  vm.runInContext(source, runtime);
  return { audio, voice, document, media, frames, canvas, pauseButton, playButton, seconds, wechat,
    mount: () => runtime.createVoiceDom([audio]),
    click: () => voice.dispatchEvent(new Event('click')),
    counts: () => ({ plays, pauses, clears }) };
}

for (const kind of ['qq', 'wechat']) {
  test(`${kind} 同一 DOM 卸载重挂载后每次点击只播放一次`, () => {
    const f = fixture(kind);
    const cleanup = f.mount();
    cleanup(); cleanup();
    const cleanupAgain = f.mount();
    f.click();
    assert.equal(f.counts().plays, 1);
    f.click();
    assert.equal(f.counts().pauses, 1);
    cleanupAgain();
    f.click();
    f.audio.dispatchEvent(new Event('ended'));
    assert.deepEqual(f.counts(), { plays: 1, pauses: 1, clears: 0 });
  });

  test(`${kind} 未取得 duration 时卸载不会被迟到事件重新挂载`, () => {
    const f = fixture(kind, NaN);
    const cleanup = f.mount();
    cleanup();
    f.audio.duration = 4;
    f.audio.dispatchEvent(new Event('durationchange'));
    f.click();
    assert.equal(f.counts().plays, 0);
    assert.equal(f.seconds.innerHTML, '');
    const cleanupAgain = f.mount();
    f.click();
    assert.equal(f.counts().plays, 1);
    cleanupAgain();
  });

  test(`${kind} duration 重复变化不叠加播放与结束监听`, () => {
    const f = fixture(kind, NaN);
    const cleanup = f.mount();
    f.audio.duration = 4;
    f.audio.dispatchEvent(new Event('durationchange'));
    f.audio.dispatchEvent(new Event('durationchange'));
    f.click();
    assert.equal(f.counts().plays, 1);
    f.audio.dispatchEvent(new Event('ended'));
    assert.equal(f.counts().pauses, 1);
    assert.equal(f.audio.currentTime, 0);
    cleanup();
  });

  test(`${kind} 播放 UI 在卸载时恢复，同一 DOM 恢复后可播放`, () => {
    const f = fixture(kind);
    const cleanup = f.mount();
    f.click();
    f.audio.currentTime = 1;
    cleanup();
    assert.equal(f.pauseButton.style.display, 'none');
    assert.equal(f.wechat.classList.contains('play'), false);
    assert.equal(f.audio.currentTime, 1);
    const cleanupAgain = f.mount();
    f.click();
    assert.equal(f.counts().plays, 2);
    cleanupAgain();
  });
}

test('QQ 卸载取消绘制 RAF，已排队回调和旧媒体/配色事件不会写入恢复后的 canvas', () => {
  const f = fixture('qq');
  const cleanup = f.mount();
  f.audio.currentTime = 1;
  f.audio.dispatchEvent(new Event('timeupdate'));
  f.document.dispatchEvent(new Event('stellar:color-scheme-change'));
  f.media.dispatchEvent(new Event('change'));
  const staleFrames = [...f.frames.values()];
  assert.ok(staleFrames.length > 0);
  cleanup();
  assert.equal(f.frames.size, 0);
  const cleanupAgain = f.mount();
  for (const frame of staleFrames) frame();
  assert.equal(f.counts().clears, 0);
  cleanupAgain();
  f.audio.dispatchEvent(new Event('timeupdate'));
  f.audio.dispatchEvent(new Event('ended'));
  f.document.dispatchEvent(new Event('stellar:color-scheme-change'));
  f.media.dispatchEvent(new Event('change'));
  assert.equal(f.frames.size, 0);
  assert.equal(f.counts().clears, 0);
});

test('QQ 正常进度与结束仍绘制，清理保留外部所有者的监听与 RAF', () => {
  const f = fixture('qq');
  let outsideClicks = 0, outsideFrames = 0;
  f.voice.addEventListener('click', () => outsideClicks++);
  f.frames.set('outside', () => outsideFrames++);
  const cleanup = f.mount();
  f.click();
  f.audio.currentTime = 2;
  f.audio.dispatchEvent(new Event('timeupdate'));
  for (const [id, callback] of [...f.frames]) {
    if (id !== 'outside') { f.frames.delete(id); callback(); }
  }
  assert.equal(f.counts().clears, 1);
  f.audio.dispatchEvent(new Event('ended'));
  assert.equal(f.pauseButton.style.display, 'none');
  assert.equal(f.playButton.style.display, 'flex');
  assert.equal(f.audio.currentTime, 0);
  for (const [id, callback] of [...f.frames]) {
    if (id !== 'outside') { f.frames.delete(id); callback(); }
  }
  assert.equal(f.counts().clears, 2);
  cleanup();
  assert.equal(f.frames.size, 1);
  f.frames.get('outside')();
  f.click();
  assert.equal(outsideFrames, 1);
  assert.equal(outsideClicks, 2);
  assert.equal(f.counts().plays, 1);
});
