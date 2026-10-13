// 搜索快捷键：所有宽度打开原生 dialog，编辑区域保持原有按键行为。
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';
const SOURCE = readFileSync(new URL('../web/assets/js/search/shortcut.js', import.meta.url), 'utf8');
function createRuntime({ hasInput = true } = {}) {
  const listeners = new Map();
  const input = hasInput ? {} : null;
  const opened = [];
  const trigger = hasInput ? { click() { opened.push(document.activeElement); } } : null;
  const document = {
    readyState: 'complete', activeElement: { id: 'opener' },
    querySelector() { return trigger; },
    addEventListener(type, handler) { listeners.set(type, handler); },
    removeEventListener(type, handler) { if (listeners.get(type) === handler) listeners.delete(type); },
  };
  const window = {};
  vm.runInNewContext(SOURCE, { document, window });
  const cleanup = window.stellarSearchShortcut.mount(document);
  function dispatch(overrides = {}) {
    let prevented = false;
    const event = {
      key: 'k', metaKey: false, ctrlKey: false, altKey: false, shiftKey: false,
      isComposing: false, defaultPrevented: false,
      target: { closest() { return null; } },
      preventDefault() { prevented = true; }, ...overrides,
    };
    listeners.get('keydown')?.(event);
    return prevented;
  }
  return { input, document, opened, cleanup, dispatch };
}
test('Command+K 与 Ctrl+K 调用 dialog opener 并保存返回焦点', () => {
  for (const modifiers of [{ metaKey: true }, { ctrlKey: true }]) {
    const rt = createRuntime();
    assert.equal(rt.dispatch(modifiers), true);
    assert.deepEqual(rt.opened, [rt.document.activeElement]);
  }
});
test('编辑区域、输入法组合和错误按键不接管快捷键', () => {
  for (const event of [
    { ctrlKey: true, target: { closest() { return {}; } } },
    { ctrlKey: true, isComposing: true }, { key: 'j', ctrlKey: true }, { key: 'k' },
    { ctrlKey: true, altKey: true }, { ctrlKey: true, shiftKey: true },
    { ctrlKey: true, defaultPrevented: true },
  ]) {
    const rt = createRuntime();
    assert.equal(rt.dispatch(event), false);
    assert.equal(rt.opened.length, 0);
  }
});
test('卸载后没有遗留快捷键监听', () => {
  const rt = createRuntime();
  assert.equal(rt.dispatch({ ctrlKey: true, target: rt.input }), true);
  rt.cleanup();
  assert.equal(rt.dispatch({ ctrlKey: true }), false);
  assert.equal(rt.opened.length, 1);
});
test('搜索框不存在时保持浏览器默认行为', () => {
  const rt = createRuntime({ hasInput: false });
  assert.equal(rt.dispatch({ ctrlKey: true }), false);
});
