import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import * as Vue from "../web/node_modules/vue/index.js";
import { parse, compileScript } from "../web/node_modules/vue/compiler-sfc/index.js";
import ts from "../web/node_modules/typescript/lib/typescript.js";
import { stellarEditorContextKey } from "../web/src/components/editor/stellar-context.ts";
import * as previewDocument from "../web/src/modules/plugins/preview/preview-document.ts";
import * as renderGate from "../web/src/modules/plugins/preview/render-gate.ts";
import * as TipTap from "../web/node_modules/@tiptap/core/dist/index.js";
import { NodeSelection } from "../web/node_modules/@tiptap/pm/state/dist/index.js";

// 加载真实 usePluginPreview（仅注入 @/api），验证 NodeView 已把高度校验交给 composable。
function loadUsePluginPreview(previewApi: any) {
  const source = readFileSync(
    new URL("../web/src/modules/plugins/preview/usePluginPreview.ts", import.meta.url),
    "utf8",
  );
  const code = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  const exports: any = {};
  new Function("require", "exports", code)((name: string) => {
    if (name === "vue") return Vue;
    if (name === "@/api") return { previewApi };
    if (name === "../plugin-types.ts") return {};
    if (name === "./preview-document.ts") return previewDocument;
    if (name === "./render-gate.ts") return renderGate;
    throw new Error(`Unexpected composable dependency: ${name}`);
  }, exports);
  return exports.usePluginPreview;
}

const previewApi = {
  render: async () => ({
    data: { html: "<p>Candidate</p>", document: "<p>Candidate</p>", preview_token: "current-token" },
  }),
};
const usePluginPreview = loadUsePluginPreview(previewApi);

test("标签预览按内容增减高度，拒绝其他窗口、过期 token 和无效尺寸", async (t) => {
  const listeners = new Map<string, (event: any) => void>();
  const canvasListeners = new Map<string, (event: any) => void>();
  const canvasWindow = {
    addEventListener: (name: string, callback: (event: any) => void) => canvasListeners.set(name, callback),
    removeEventListener: (name: string) => canvasListeners.delete(name),
  };
  const originalWindow = (globalThis as any).window;
  const originalResizeObserver = (globalThis as any).ResizeObserver;
  let app: Vue.App<any> | undefined;
  (globalThis as any).ResizeObserver = class { observe() {} disconnect() {} };
  (globalThis as any).window = {
    innerWidth: 1280,
    addEventListener: (name: string, callback: (event: any) => void) => listeners.set(name, callback),
    removeEventListener: (name: string) => listeners.delete(name),
  };
  t.after(() => {
    app?.unmount();
    (globalThis as any).window = originalWindow;
    (globalThis as any).ResizeObserver = originalResizeObserver;
  });

  const { descriptor } = parse(
    readFileSync(new URL("../web/src/components/editor/StellarTagView.vue", import.meta.url), "utf8"),
  );
  const compiled = ts.transpileModule(compileScript(descriptor, { id: "stellar-preview-height" }).content, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  const exports: any = {};
  let nodeCache: unknown;
  const imports: Record<string, any> = {
    vue: Vue,
    "./stellar-context": { stellarEditorContextKey },
    "@tiptap/vue-3": { nodeViewProps: {} },
    "@iconify/vue": {},
    "@/api": { previewApi, pluginApi: { parseShortcode: async () => ({ data: {} }) } },
    "./StellarTagPropertyPopover.vue": {},
    "@/modules/plugins/usePluginCatalog": { loadPluginCatalog: async () => [] },
    "@/modules/plugins/shortcode": {},
    "./stellar-tag": {},
    "@/modules/plugins/preview/usePluginPreview": { usePluginPreview: (...args: any[]) => {
      nodeCache = args[2].cache;
      return usePluginPreview(...args);
    } },
    "@/modules/plugins/preview/render-gate": renderGate,
    "@/modules/plugins/preview/preview-document": previewDocument,
  };
  new Function("require", "exports", compiled)((name: string) => {
    assert.ok(name in imports, `Unexpected import: ${name}`);
    return imports[name];
  }, exports);

  // 在真实 Vue 生命周期内运行实际 SFC setup；不依赖浏览器布局来验证消息边界。
  let state: any;
  const renderer = Vue.createRenderer<any, any>({
    createElement: () => ({}),
    createText: () => ({}),
    createComment: () => ({}),
    setText() {},
    setElementText() {},
    patchProp() {},
    insert() {},
    remove() {},
    parentNode: () => null,
    nextSibling: () => null,
  });
  app = renderer.createApp({
    setup() {
      state = exports.default.setup({ node: { attrs: { raw: "{% copy candidate %}" } } }, { expose() {} });
      return () => null;
    },
  });
  app.mount({});
  // 确定性等待真实 composable 的 0ms 调度：轮询 token + 宏任务推进，不依赖墙钟延时。
  for (let i = 0; i < 100 && state.preview.previewToken.value !== "current-token"; i++) {
    await Vue.nextTick();
    await new Promise<void>((resolve) => setImmediate(resolve));
  }
  assert.equal(state.preview.previewToken.value, "current-token", "首次预览 token 未在确定轮询内就绪");
  assert.ok(nodeCache instanceof renderGate.PreviewCache);
  assert.notEqual(nodeCache, renderGate.sharedPreviewCache, "NodeView 重新挂载后应取得当前主题文档");
  const frame = {};
  state.iframeRef.value = Vue.markRaw({ contentWindow: frame, ownerDocument: { defaultView: canvasWindow } });
  await Vue.nextTick();
  assert.equal(listeners.has("message"), false, "高度消息不应监听 Console 祖父窗口");
  const message = canvasListeners.get("message")!;
  assert.ok(message);
  const resize = (height: any, overrides: Record<string, any> = {}) =>
    message({
      source: frame,
      data: { type: "component-preview-resize", token: "current-token", height },
      ...overrides,
    });
  const initial = state.preview.frameHeight.value;
  for (const height of ["900", NaN, Infinity, -1, 0]) resize(height);
  resize(900, { source: {} });
  resize(900, { data: { type: "component-preview-resize", token: "previous-token", height: 900 } });
  resize(900, { data: { type: "unrelated-event", token: "current-token", height: 900 } });
  assert.equal(state.preview.frameHeight.value, initial);

  resize(1200.4);
  assert.equal(state.preview.frameHeight.value, 1201, "长图应完整展开");
  resize(320);
  assert.equal(state.preview.frameHeight.value, 320, "内容缩短或重排后应随之缩短");
  resize(16);
  assert.equal(state.preview.frameHeight.value, 32, "空小组件保留最小点击区域");
  app.unmount();
  app = undefined;
  assert.equal(canvasListeners.has("message"), false);
  assert.equal(listeners.has("resize"), false);
});

function loadDragAdapter() {
  const source = readFileSync(new URL("../web/src/components/editor/stellar-tag.ts", import.meta.url), "utf8");
  const compiled = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  const exports: any = {};
  const imports: Record<string, any> = {
    "@tiptap/core": TipTap,
    "@tiptap/pm/state": { NodeSelection },
    "@tiptap/vue-3": {},
    "@/modules/plugins/usePluginCatalog": { runtimePlugins: { value: [] } },
    "./StellarTagView.vue": {},
    "./stellar-tag-syntax": {},
  };
  new Function("require", "exports", compiled)((name: string) => {
    assert.ok(name in imports, `Unexpected drag adapter import: ${name}`);
    return imports[name];
  }, exports);
  return exports.withOwnerDocumentDragEvents;
}

test("画布拖拽按实际 ownerDocument 结束，保留输入/选择事件规则并在销毁时清理", () => {
  const adapt = loadDragAdapter();
  const document = new EventTarget();
  const handle = { tagName: "SPAN", isContentEditable: false, closest: () => handle };
  const plain = { tagName: "SPAN", isContentEditable: false, closest: () => null };
  const input = { tagName: "INPUT", isContentEditable: false, closest: () => handle };
  const dom = { ownerDocument: document, contains: (target: unknown) => [handle, plain, input].includes(target as any) };
  let destroyed = false;
  const view = adapt({
    dom, contentDOM: null, editor: { isEditable: true }, node: { type: { spec: { draggable: true } } },
    isDragging: false, options: {}, stopEvent: TipTap.NodeView.prototype.stopEvent,
    destroy: () => { destroyed = true; },
  });
  const event = (type: string, target = handle) => ({ type, target, preventDefault() {} });
  for (const end of ["dragend", "drop", "mouseup"]) {
    assert.equal(view.stopEvent(event("mousedown")), false, "把手按下仍交给 ProseMirror 选择");
    assert.equal(view.isDragging, true);
    assert.equal(view.stopEvent(event("mousemove")), false, "拖拽过程中沿用上游事件放行");
    document.dispatchEvent(new Event(end));
    assert.equal(view.isDragging, false, `${end} 应结束画布中的拖拽状态`);
    assert.equal(view.stopEvent(event("mousemove")), true, "结束后恢复节点事件隔离");
  }
  assert.equal(view.stopEvent(event("mousedown", input)), true, "表单输入仍由节点自己处理");
  assert.equal(view.isDragging, false);
  assert.equal(view.stopEvent(event("mousedown", plain)), false, "普通节点点击仍可选中");
  assert.equal(view.isDragging, false, "非把手不能开始拖拽");
  view.stopEvent(event("mousedown"));
  view.destroy();
  assert.equal(destroyed, true);
  assert.equal(view.isDragging, false);
  view.isDragging = true;
  document.dispatchEvent(new Event("mouseup"));
  assert.equal(view.isDragging, true, "销毁后不应再收到文档恢复事件");
});

test("空白新页也请求正式样式，预览占位不回写正文", async (t) => {
  const originalWindow = (globalThis as any).window;
  const originalResizeObserver = (globalThis as any).ResizeObserver;
  (globalThis as any).window = { innerWidth: 1280, addEventListener() {}, removeEventListener() {} };
  (globalThis as any).ResizeObserver = class { observe() {} disconnect() {} };
  let app: Vue.App<any> | undefined;
  t.after(() => {
    app?.unmount();
    (globalThis as any).window = originalWindow;
    (globalThis as any).ResizeObserver = originalResizeObserver;
  });
  const { descriptor } = parse(readFileSync(new URL("../web/src/components/editor/StellarEditingCanvas.vue", import.meta.url), "utf8"));
  const compiled = ts.transpileModule(compileScript(descriptor, { id: "stellar-empty-canvas" }).content, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  const requests: Array<{ markdown: string; options: any }> = [];
  let canvasCache: unknown;
  const imports: Record<string, any> = {
    vue: Vue,
    "@/modules/plugins/preview/usePluginPreview": { usePluginPreview: (_info: unknown, _overrides: unknown, options: any) => {
      canvasCache = options.cache;
      return {
      status: Vue.ref("idle"), srcdoc: Vue.ref(""),
      scheduleRender: (markdown: string, options: any) => requests.push({ markdown, options }),
    }; } },
    "@/modules/plugins/preview/render-gate": renderGate,
    "./stellar-editor-decoration": {},
    "./stellar-editor-blocks": {},
  };
  const exports: any = {};
  new Function("require", "exports", compiled)((name: string) => {
    assert.ok(name in imports, `Unexpected canvas import: ${name}`);
    return imports[name];
  }, exports);
  const props = Vue.reactive({ markdown: "", metadata: { title: "新页面" } });
  let canvas: any;
  const renderer = Vue.createRenderer<any, any>({
    createElement: () => ({}), createText: () => ({}), createComment: () => ({}),
    setText() {}, setElementText() {}, patchProp() {}, insert() {}, remove() {},
    parentNode: () => null, nextSibling: () => null,
  });
  app = renderer.createApp({ setup() { canvas = exports.default.setup(props, { expose() {} }); return () => null; } });
  app.mount({});
  assert.ok(canvasCache instanceof renderGate.PreviewCache);
  assert.notEqual(canvasCache, renderGate.sharedPreviewCache, "完整正文预览不得沿用跨画布的旧站点设置缓存");
  assert.equal(requests.length, 1);
  assert.equal(requests[0].markdown, "<p></p>");
  assert.equal(requests[0].options.profile, null);
  assert.equal(requests[0].options.metadata.title, "新页面");
  assert.equal(props.markdown, "", "样式初始化不得写入新页面正文");
  props.markdown = "首次输入";
  await Vue.nextTick();
  assert.equal(requests.at(-1)?.markdown, "首次输入");
  props.markdown = "  \n";
  await Vue.nextTick();
  assert.equal(requests.at(-1)?.markdown, "<p></p>");
  assert.equal(props.markdown, "  \n", "清空正文后也保留用户原值");
  await t.test("只抽取编辑辅助规则，移除旧容器前缀并保留声明及条件", () => {
    const style = (selectorText: string, cssText: string, position = "") => ({
      selectorText, style: { cssText, length: 1, getPropertyValue: (name: string) => name === "position" ? position : "" },
    });
    const rules = [
      style("body, h3, ul", "margin: 0; font-size: 24px;"),
      style(".rte-search-match, body", "background: var(--color-primary);"),
      style(".rte-search-match-current", "box-shadow: inset 0 -2px 0 var(--color-primary);"),
      style(".rte-content .ProseMirror p.is-editor-empty:first-child::before", "content: attr(data-placeholder);"),
      style(".rte-content .ProseMirror .selectedCell::after", "position: absolute;"),
      style(".rte-content .ProseMirror td, .rte-content .ProseMirror th", "position: relative;", "relative"),
      style(".rte-content .ProseMirror td", "padding: 12px;"),
      style(".ProseMirror", "position: relative;", "relative"),
      style("img.ProseMirror-separator", "width: 0 !important;"),
      { cssText: "@media (min-width: 1px) {}", cssRules: [style(".ProseMirror-focused .ProseMirror-gapcursor", "display: block;")] },
      { name: "ProseMirror-cursor-blink", cssText: "@keyframes ProseMirror-cursor-blink {to {visibility:hidden}}" },
    ];
    const result = canvas.extractEditingRules(rules);
    assert.match(result, /\.rte-search-match \{background: var\(--color-primary\);\}/);
    assert.match(result, /\.rte-search-match-current \{box-shadow: inset 0 -2px 0 var\(--color-primary\);\}/);
    assert.match(result, /\.ProseMirror p\.is-editor-empty:first-child::before \{content: attr\(data-placeholder\);\}/);
    assert.match(result, /\.ProseMirror \.selectedCell::after \{position: absolute;\}/);
    assert.match(result, /\.ProseMirror td, \.ProseMirror th \{position: relative;\}/);
    assert.match(result, /\.ProseMirror \{position: relative;\}/);
    assert.match(result, /img\.ProseMirror-separator \{width: 0 !important;\}/);
    assert.match(result, /@media \(min-width: 1px\) \{\.ProseMirror-focused \.ProseMirror-gapcursor \{display: block;\}\}/);
    assert.match(result, /@keyframes ProseMirror-cursor-blink/);
    assert.doesNotMatch(result, /rte-content|body|h3|margin:|padding:|font-size:/);
  });
  await t.test("仅首次激活画布host时抑制原生居中，不反复抢占焦点", () => {
    const calls: unknown[] = [];
    const document: { activeElement: unknown } = { activeElement: null };
    const frameDocument: { body: object; documentElement: object; activeElement: unknown } = { body: {}, documentElement: {}, activeElement: null };
    const host = Vue.markRaw({
      ownerDocument: document,
      contentDocument: frameDocument,
      focus(options: unknown) { calls.push(options); document.activeElement = host; },
    });
    let prevented = 0;
    const pointer = (target: unknown) => ({ target, preventDefault() { prevented++; } });
    canvas.frame.value = host;
    canvas.activateCanvas(pointer({}));
    assert.deepEqual(calls, [{ preventScroll: true }]);
    canvas.activateCanvas(pointer({}));
    assert.equal(calls.length, 1, "host 已激活时不再 focus，保留内部控件焦点");
    assert.equal(prevented, 0, "编辑根和控件事件保持原生默认行为");
    canvas.activateCanvas(pointer(frameDocument.body));
    canvas.activateCanvas(pointer(frameDocument.documentElement));
    assert.equal(prevented, 2, "仅 body/html 空白背景阻止默认再次聚焦居中");
    assert.equal(calls.length, 1, "背景点击不重复抢占内部控件焦点");
    document.activeElement = {};
    canvas.activateCanvas(pointer({}));
    assert.equal(calls.length, 2, "离开画布后首次点击应重新无滚动激活");
    assert.deepEqual(calls[1], { preventScroll: true });
    const root = Vue.markRaw({ tabIndex: 0 });
    let editorFocus = 0;
    (props as any).editor = { isDestroyed: false, view: { dom: root, focus() { editorFocus++; frameDocument.activeElement = root; } } };
    try {
      const controlCalls: unknown[] = [];
      const button = {
        ownerDocument: frameDocument,
        matches: () => false,
        closest: () => null,
        focus(options: unknown) { controlCalls.push(options); frameDocument.activeElement = button; },
      };
      const svg = { closest: () => button };
      frameDocument.activeElement = root;
      canvas.activateCanvas(pointer(svg));
      assert.deepEqual(controlCalls, [{ preventScroll: true }], "点击按钮SVG时无滚动聚焦实际按钮");
      assert.equal(editorFocus, 0, "控件点击不抢回编辑根");
      canvas.activateCanvas(pointer(svg));
      assert.equal(controlCalls.length, 1, "已聚焦控件不重复focus，保留输入与光标状态");
      const input = {
        ownerDocument: frameDocument,
        matches: () => false,
        closest: () => null,
        focus(options: unknown) { controlCalls.push(options); frameDocument.activeElement = input; },
      };
      const labelChild = { closest: (selector: string) => {
        if (selector === "label") return { control: input };
        // 真实编辑根带 tabindex=0：混合查找或 tabindex 回退都会命中该祖先。
        if (selector.includes("[tabindex]")) return root;
        return null;
      } };
      canvas.activateCanvas(pointer(labelChild));
      assert.deepEqual(controlCalls[1], { preventScroll: true }, "标签子span/SVG点击提前聚焦label.control");
      assert.equal(frameDocument.activeElement, input);
      assert.equal(editorFocus, 0, "关联input优先于祖先编辑根的tabindex");
      canvas.activateCanvas(pointer(labelChild));
      assert.equal(controlCalls.length, 2, "关联input已持焦时不重复focus");
      const disabled = { ...button, matches: () => true };
      canvas.activateCanvas(pointer({ closest: () => disabled }));
      canvas.activateCanvas(pointer({ closest: (selector: string) => selector === "label" ? { control: disabled } : null }));
      assert.equal(controlCalls.length, 2, "直接控件或label关联的禁用控件均不执行focus");
      assert.equal(editorFocus, 0);
      assert.equal(prevented, 2, "控件事件不preventDefault，后续mousedown继续定位光标");
      frameDocument.activeElement = frameDocument.body;
      canvas.activateCanvas(pointer({}));
      assert.equal(editorFocus, 1, "编辑区或把手从body进入时使用PM自身无滚动focus");
      canvas.activateCanvas(pointer({}));
      assert.equal(editorFocus, 1, "编辑根已聚焦时不重复focus");
      frameDocument.activeElement = button;
      canvas.activateCanvas(pointer(frameDocument.body));
      assert.equal(editorFocus, 2, "空白关闭面板前将焦点安全交回编辑根");
      assert.equal(prevented, 3);
    } finally {
      delete (props as any).editor;
    }
  });
});
