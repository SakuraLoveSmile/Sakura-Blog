import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

const source = path => readFileSync(new URL(path, import.meta.url), 'utf8');

function fixture({ clipboard = true } = {}) {
  const buttons = [], toasts = [], copied = [];
  let resolveCopy, rejectCopy;
  const code = {
    querySelector: selector => selector === 'pre' ? { innerText: 'go test ./...' } : buttons[0],
    appendChild: button => buttons.push(button)
  };
  const ctx = { icons: { 'copy:copy': '<svg></svg>' } };
  const context = vm.createContext({
    ctx,
    navigator: clipboard ? { clipboard: { writeText: value => {
      copied.push(value);
      return new Promise((resolve, reject) => { resolveCopy = resolve; rejectCopy = reject; });
    } } } : {},
    hud: { toast: (...values) => toasts.push(values) },
    document: { createElement: () => {
      const button = new EventTarget();
      button.setAttribute = () => {};
      button.remove = () => { const index = buttons.indexOf(button); if (index !== -1) buttons.splice(index, 1); };
      return button;
    } }
  });
  context.window = context;
  vm.runInContext(source('../web/assets/js/runtime/extensions/code-copy.js').replace('export async function mount', 'async function mount'), context);
  const controller = new AbortController();
  const root = { querySelectorAll: () => [code] };
  let loaded = false;
  const mount = () => context.mount(root, {
    extension: { config: { messages: { label: 'Copy', copied: 'Copied', denied: 'Denied', unsupported: 'Unsupported' }, assets: { js: '/copycode.js' } } },
    manifest: { policy: { features: { codeCopyToastMs: 2500 } } },
    legacy: { ctx }, signal: controller.signal,
    assets: { script: async () => {
      if (!loaded) { loaded = true; vm.runInContext(source('../web/assets/js/plugins/copycode.js'), context); }
    } }
  });
  return { buttons, toasts, copied, controller, mount, resolve: () => resolveCopy(), reject: () => rejectCopy(new Error('Denied')) };
}

const settled = () => new Promise(resolve => setImmediate(resolve));

test('复制保留copied/denied/unsupported正式提示与精确代码内容', async () => {
  for (const result of ['copied', 'denied', 'unsupported']) {
    const f = fixture({ clipboard: result !== 'unsupported' });
    const cleanup = await f.mount();
    f.buttons[0].dispatchEvent(new Event('click'));
    if (result === 'copied') f.resolve();
    if (result === 'denied') f.reject();
    await settled();
    assert.deepEqual(f.toasts, [[{ copied: 'Copied', denied: 'Denied', unsupported: 'Unsupported' }[result], 2500]]);
    assert.deepEqual(f.copied, result === 'unsupported' ? [] : ['go test ./...']);
    cleanup();
    assert.equal(f.buttons.length, 0);
  }
});

test('pending clipboard在cleanup或signal取消后resolve/reject均不得显示迟到toast', async () => {
  for (const stop of ['cleanup', 'abort']) {
    for (const result of ['resolve', 'reject']) {
      const f = fixture(), cleanup = await f.mount(), button = f.buttons[0];
      button.dispatchEvent(new Event('click'));
      if (stop === 'cleanup') cleanup(); else f.controller.abort();
      f[result]();
      await settled();
      assert.deepEqual(f.toasts, [], `${stop}/${result}`);
      assert.equal(f.buttons.length, 0);
      button.dispatchEvent(new Event('click'));
      assert.equal(f.copied.length, 1);
      cleanup();
    }
  }
});
