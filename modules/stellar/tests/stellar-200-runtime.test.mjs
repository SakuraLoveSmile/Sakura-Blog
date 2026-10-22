// 零依赖行为检查：真正的上游 registry 与本地 shell extension。
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
const importSource = path => import('data:text/javascript;base64,' + Buffer.from(readFileSync(new URL(path, import.meta.url))).toString('base64'));
const { createExtensionRegistry } = await importSource('../web/assets/js/runtime/extension-registry.js');
const shellExtension = await importSource('../web/assets/js/runtime/extensions/shell.js');
const manifest = { extensions: [{ id: 'blog-shell', module: '/assets/js/runtime/extensions/shell.js', scope: 'document', when: { always:true }, config:{} }] };

// 默认 assets-test 不需要浏览器依赖；直接运行真实上游 registry 与 shell 扩展。
class Element extends EventTarget {
  constructor(id, documentRef) {
    super(); this.id = id; this.documentRef = documentRef; this.dataset = {};
    this.attributes = new Map(); this.inert = false; this.isConnected = true; this.child = null;
  }
  setAttribute(name, value) { this.attributes.set(name, value); }
  getAttribute(name) { return this.attributes.get(name) ?? null; }
  removeAttribute(name) { this.attributes.delete(name); }
  querySelectorAll() { return this.child ? [this.child] : []; }
  closest() { return null; }
  getClientRects() { return [{}]; }
  focus() { this.documentRef.activeElement = this; }
}
function shellFixture() {
  const documentRef = new EventTarget();
  const element = id => new Element(id, documentRef);
  const root = element('root'), shell = element('shell');
  const left = element('leftbar-region'), right = element('rightbar-region');
  left.child = element('left-focus'); right.child = element('right-focus');
  const leftToggle = element('left-toggle'), rightToggle = element('right-toggle'), collapse = element('collapse');
  const controls = new Map([['toggle-leftbar-drawer',leftToggle],['toggle-rightbar-drawer',rightToggle],['toggle-leftbar',collapse]]);
  documentRef.documentElement = root;
  documentRef.querySelector = () => shell;
  documentRef.querySelectorAll = selector => {
    const action = selector.match(/data-shell-action="([^"]+)"/)?.[1];
    return controls.has(action) ? [controls.get(action)] : [];
  };
  documentRef.getElementById = id => id === left.id ? left : id === right.id ? right : null;
  const media = { leftbar: new EventTarget(), rightbar: new EventTarget() };
  media.leftbar.matches = true; media.rightbar.matches = true;
  documentRef.defaultView = {
    matchMedia: query => query.includes('768') ? media.leftbar : media.rightbar,
    localStorage: { getItem() { return null; }, setItem() {} },
  };
  return { documentRef, root, shell, left, right, leftToggle, rightToggle, collapse, media };
}
test('上游 registry 挂载真实 shell 扩展，互斥抽屉与卸载状态正确', async () => {
  const fixture = shellFixture();
  const registry = createExtensionRegistry({ importer: async () => shellExtension });
  registry.register(manifest.extensions[0]);
  const result = await registry.mount(fixture.documentRef);
  assert.equal(result[0].status, 'mounted');
  const api = fixture.documentRef.defaultView.blogShell;
  assert.equal(fixture.left.inert, true);
  api.toggleDrawer('leftbar', fixture.leftToggle);
  assert.equal(fixture.shell.dataset.drawer, 'leftbar');
  assert.equal(fixture.left.inert, false);
  assert.equal(fixture.leftToggle.getAttribute('aria-expanded'), 'true');
  assert.equal(fixture.documentRef.activeElement, fixture.left.child);
  api.toggleDrawer('rightbar', fixture.rightToggle);
  assert.equal(fixture.shell.dataset.drawer, 'rightbar');
  assert.equal(fixture.left.inert, true);
  api.dismiss();
  assert.equal(fixture.documentRef.activeElement, fixture.rightToggle);
  fixture.media.leftbar.matches = false;
  fixture.media.rightbar.matches = false;
  fixture.media.leftbar.dispatchEvent(new Event('change'));
  assert.equal(fixture.left.inert, false);
  api.toggleLeftbar();
  assert.equal(fixture.root.dataset.leftbarState, 'collapsed');
  assert.equal(fixture.collapse.getAttribute('aria-expanded'), 'false');
  await registry.unmount(fixture.documentRef);
  assert.equal(fixture.documentRef.defaultView.blogShell, undefined);
  assert.equal(fixture.shell.dataset.drawer, undefined);
  await registry.mount(fixture.documentRef);
  assert.equal(typeof fixture.documentRef.defaultView.blogShell.toggleLeftbar, 'function');
  await registry.unmount(fixture.documentRef);
});
test('上游 registry 丢弃卸载期间未完成的 import，不挂载过期实例', async () => {
  let resolveImport;
  const importing = new Promise(resolve => { resolveImport = resolve; });
  let mounts = 0;
  const registry = createExtensionRegistry({ importer: () => importing });
  registry.register(manifest.extensions[0]);
  const root = {};
  const mounting = registry.mount(root);
  await new Promise(resolve => setImmediate(resolve));
  await registry.unmount(root);
  resolveImport({ mount() { mounts++; } });
  const result = await mounting;
  assert.equal(result[0].status, 'skipped');
  assert.equal(mounts, 0);
});
test('上游 registry 的挂载失败会释放已登记资源并报告失败', async () => {
  let cleanups = 0;
  const errors = [];
  const registry = createExtensionRegistry({ importer: async () => ({ mount(_root, context) {
    context.onCleanup(() => { cleanups++; });
    throw new Error('fixture mount failure');
  } }), onError: error => errors.push(error) });
  registry.register(manifest.extensions[0]);
  const root = {};
  const result = await registry.mount(root);
  assert.equal(result[0].status, 'failed');
  assert.equal(cleanups, 1);
  assert.equal(errors[0].phase, 'mount');
  await registry.unmount(root);
  assert.equal(cleanups, 1);
});
