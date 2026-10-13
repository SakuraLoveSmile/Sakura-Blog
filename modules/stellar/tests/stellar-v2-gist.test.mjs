import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mount } from '../web/assets/js/runtime/extensions/blog-gist.js';

test('Gist尺寸只接受本region对应opaque frame；卸载及abort禁止迟到写回', () => {
  const listeners = new Set();
  const view = { addEventListener(name, listener) { assert.equal(name, 'message'); listeners.add(listener); }, removeEventListener(_name, listener) { listeners.delete(listener); } };
  const first = { contentWindow: {}, style: {}, isConnected: true };
  const second = { contentWindow: {}, style: {}, isConnected: true };
  const root = frame => ({ ownerDocument: { defaultView: view }, querySelectorAll: () => [frame] });
  const controller = new AbortController();
  const cleanup = mount(root(first), { signal: controller.signal });
  const emit = event => [...listeners].forEach(fn => fn(event));
  const valid = { origin: 'null', source: first.contentWindow, data: { type: 'stellar:gist:resize', height: 300 } };
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
  emit({ ...valid, source: second.contentWindow, data: { ...valid.data, height: 450 } });
  assert.equal(second.style.height, '450px'); next(); assert.equal(listeners.size, 0);
});
