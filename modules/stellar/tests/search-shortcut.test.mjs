// Stellar 1.44 搜索快捷键：桌面端接管 Command/Ctrl+K，移动端和编辑区降级。
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import vm from "node:vm";

const SOURCE_PATH = join(dirname(fileURLToPath(import.meta.url)), "../web/assets/js/search/shortcut.js");
const SOURCE = readFileSync(SOURCE_PATH, "utf8");

function createRuntime({ narrow = false, hasInput = true } = {}) {
  const listeners = {};
  const input = hasInput ? { focusCount: 0, focus() { this.focusCount += 1; } } : null;
  const document = {
    readyState: "complete",
    getElementById(id) { return id === "search-input" ? input : null; },
    querySelector(selector) { return selector === ".mobile-only.leftbar-toggle" ? {} : null; },
    addEventListener(type, handler) { listeners[type] = handler; },
  };
  const window = { getComputedStyle() { return { display: narrow ? "block" : "none" }; } };
  vm.runInNewContext(SOURCE, { document, window });
  function dispatch(overrides = {}) {
    let prevented = false;
    const event = {
      key: "k", metaKey: false, ctrlKey: false, altKey: false, shiftKey: false,
      isComposing: false, defaultPrevented: false,
      target: { closest() { return null; } },
      preventDefault() { prevented = true; },
      ...overrides,
    };
    listeners.keydown(event);
    return prevented;
  }
  return { input, listeners, dispatch };
}

test("桌面端 Command+K 与 Ctrl+K 聚焦搜索框", () => {
  for (const modifiers of [{ metaKey: true }, { ctrlKey: true }]) {
    const rt = createRuntime();
    assert.equal(rt.dispatch(modifiers), true);
    assert.equal(rt.input.focusCount, 1);
  }
});

test("窄屏、编辑区域、输入法组合和错误按键不接管快捷键", () => {
  const narrow = createRuntime({ narrow: true });
  assert.equal(narrow.dispatch({ ctrlKey: true }), false);
  assert.equal(narrow.input.focusCount, 0);

  const editable = createRuntime();
  assert.equal(editable.dispatch({ ctrlKey: true, target: { closest() { return {}; } } }), false);
  assert.equal(editable.input.focusCount, 0);

  const composing = createRuntime();
  assert.equal(composing.dispatch({ ctrlKey: true, isComposing: true }), false);

  for (const event of [{ key: "j", ctrlKey: true }, { key: "k" }, { key: "k", ctrlKey: true, altKey: true }, { key: "k", ctrlKey: true, defaultPrevented: true }]) {
    const rt = createRuntime();
    assert.equal(rt.dispatch(event), false);
    assert.equal(rt.input.focusCount, 0);
  }
});

test("搜索框不存在时保持浏览器默认行为", () => {
  const rt = createRuntime({ hasInput: false });
  assert.equal(rt.dispatch({ ctrlKey: true }), false);
});
