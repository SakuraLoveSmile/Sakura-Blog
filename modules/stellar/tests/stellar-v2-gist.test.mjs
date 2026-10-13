import { test } from 'node:test';
import assert from 'node:assert/strict';
import { webcrypto } from 'node:crypto';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import { mount } from '../web/assets/js/runtime/extensions/blog-gist.js';

test('Gist尺寸只接受本region对应opaque frame；卸载及abort禁止迟到写回', () => {
  const listeners = new Set();
  const view = { crypto: webcrypto, addEventListener(name, listener) { assert.equal(name, 'message'); listeners.add(listener); }, removeEventListener(_name, listener) { listeners.delete(listener); } };
  const frame = () => Object.assign(new EventTarget(), { contentWindow: { postMessage() {} }, style: {}, isConnected: true, src: '/stellar/embed/gist?gist=test%2Fab', getAttribute() { return this.src; }, setAttribute(_key, value) { this.src = value; } });
  const first = frame();
  const second = frame();
  const root = frame => ({ ownerDocument: { defaultView: view, baseURI: 'https://fixture.test/' }, querySelectorAll: () => [frame] });
  const controller = new AbortController();
  const cleanup = mount(root(first), { signal: controller.signal });
  const emit = event => [...listeners].forEach(fn => fn(event));
  const valid = { origin: 'null', source: first.contentWindow, data: { type: 'stellar:gist:resize', height: 300, token: new URLSearchParams(new URL(first.src).hash.slice(1)).get('stellar-gist-token') } };
  for (const event of [ { ...valid, source: {} }, { ...valid, origin: 'https://evil.test' }, { ...valid, data: { ...valid.data, height: Infinity } }, { ...valid, data: { ...valid.data, height: '300' } }, { ...valid, data: { ...valid.data, height: -1 } } ]) emit(event);
  assert.equal(first.style.height, undefined);
  emit(valid); assert.equal(first.style.height, '300px');
  first.isConnected = false;
  emit({ ...valid, data: { ...valid.data, height: 400 } }); assert.equal(first.style.height, '300px');
  first.isConnected = true;
  controller.abort(); assert.equal(listeners.size, 0);
  cleanup();
  const next = mount(root(second), { signal: new AbortController().signal });
  emit(valid); assert.equal(second.style.height, undefined);
  emit({ ...valid, source: second.contentWindow, data: { ...valid.data, height: 450, token: new URLSearchParams(new URL(second.src).hash.slice(1)).get('stellar-gist-token') } });
  assert.equal(second.style.height, '450px'); next(); assert.equal(listeners.size, 0);
});

function message(target, source, data, origin = 'null') {
  const event = new Event('message');
  for (const [key, value] of Object.entries({ source, data, origin })) Object.defineProperty(event, key, { value });
  target.dispatchEvent(event);
}

test('正式父子脚本在初次尺寸丢失后通过load握手恢复同高度，卸载拒绝迟到请求', () => {
  const parent = new EventTarget();
  parent.crypto = webcrypto;
  const child = new EventTarget();
  child.parent = parent;
  child.location = { hash: '' };
  const callbacks = new Map();
  let next = 0, dropped = false, disconnected = false;
  child.requestAnimationFrame = fn => { callbacks.set(++next, fn); return next; };
  child.cancelAnimationFrame = id => callbacks.delete(id);
  const flush = () => { const pending = [...callbacks.values()]; callbacks.clear(); pending.forEach(fn => fn()); };
  parent.postMessage = data => { if (!dropped) { dropped = true; return; } message(parent, child, data); };
  const requests = [];
  child.postMessage = (data, origin) => { requests.push({ data, origin }); message(child, parent, data, 'https://fixture.test'); };
  const frame = Object.assign(new EventTarget(), {
    contentWindow: child, style: {}, isConnected: true, src: '/stellar/embed/gist?gist=test%2Fab',
    getAttribute() { return this.src; }, setAttribute(_key, value) { this.src = value; child.location.hash = new URL(value, 'https://fixture.test').hash; }
  });
  const root = { ownerDocument: { defaultView: parent, baseURI: 'https://fixture.test/' }, querySelectorAll: () => [frame] };
  const controller = new AbortController();
  const cleanup = mount(root, { signal: controller.signal });
  class ResizeObserver { observe() {} disconnect() { disconnected = true; } }
  vm.runInNewContext(readFileSync(new URL('../web/assets/js/blog-gist.js', import.meta.url), 'utf8'), {
    window: child, document: { body: { getBoundingClientRect: () => ({ height: 850 }) } }, URLSearchParams, ResizeObserver
  });
  flush();
  assert.equal(dropped, true);
  assert.equal(frame.style.height, undefined);
  frame.dispatchEvent(new Event('load'));
  flush();
  assert.equal(frame.style.height, '850px');
  assert.equal(requests.length, 1);
  assert.equal(requests[0].origin, '*');
  assert.equal(requests[0].data.token, new URLSearchParams(child.location.hash.slice(1)).get('stellar-gist-token'));
  message(child, {}, requests[0].data);
  message(child, parent, { ...requests[0].data, token: 'old' });
  assert.equal(callbacks.size, 0);
  controller.abort();
  cleanup();
  frame.dispatchEvent(new Event('load'));
  assert.equal(requests.length, 1);
  child.dispatchEvent(new Event('pagehide'));
  message(child, parent, requests[0].data);
  assert.equal(callbacks.size, 0);
  assert.equal(disconnected, true);
});
