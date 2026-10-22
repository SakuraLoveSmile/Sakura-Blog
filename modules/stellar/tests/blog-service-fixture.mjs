// 既有纯行为用例改从实际regional入口装配；测试导出只在VM中注入。
import { readFileSync } from 'node:fs';
import vm from 'node:vm';
export function mountBlogService(source, sandbox, { exports = [], request } = {}) {
  const doc = sandbox.document, view = sandbox.window;
  doc.currentScript = {};
  doc.defaultView = view;
  doc.querySelectorAll ||= selector => doc.getElementsByClassName(selector.slice(1));
  doc.addEventListener ||= () => {};
  doc.removeEventListener ||= () => {};
  view.utils = sandbox.utils || {};
  view.document = doc;
  view.fetch = sandbox.fetch || (() => Promise.reject(new Error('fixture request is missing')));
  view.setTimeout = setTimeout; view.clearTimeout = clearTimeout;
  view.setInterval = setInterval; view.clearInterval = clearInterval;
  Object.assign(sandbox, { AbortController, Proxy, Promise, WeakMap, Set });
  vm.createContext(sandbox);
  vm.runInContext(readFileSync(new URL('../web/assets/js/blog/lifecycle.js', import.meta.url), 'utf8'), sandbox);
  if (exports.length) source = source.replace('  })(scopedDocument);', '  Object.assign(window, { ' + exports.join(', ') + ' });\n  })(scopedDocument);');
  vm.runInContext(source, sandbox);
  const context = { signal: new AbortController().signal, onCleanup() {}, reportError(error) { throw error; }, ...(request ? { request: { request } } : {}) };
  const cleanup = doc.currentScript.stellarMount(doc, context);
  return { cleanup, context };
}
