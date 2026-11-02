import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import * as Vue from "../web/node_modules/vue/index.js";
import { parse, compileScript } from "../web/node_modules/vue/compiler-sfc/index.js";
import ts from "../web/node_modules/typescript/lib/typescript.js";
import { stellarEditorContextKey } from "../web/src/components/editor/stellar-context.ts";
import * as previewDocument from "../web/src/modules/plugins/preview/preview-document.ts";
import * as renderGate from "../web/src/modules/plugins/preview/render-gate.ts";

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
  const originalWindow = (globalThis as any).window;
  let app: Vue.App<any> | undefined;
  (globalThis as any).window = {
    addEventListener: (name: string, callback: (event: any) => void) => listeners.set(name, callback),
    removeEventListener: (name: string) => listeners.delete(name),
  };
  t.after(() => {
    app?.unmount();
    (globalThis as any).window = originalWindow;
  });

  const { descriptor } = parse(
    readFileSync(new URL("../web/src/components/editor/StellarTagView.vue", import.meta.url), "utf8"),
  );
  const compiled = ts.transpileModule(compileScript(descriptor, { id: "stellar-preview-height" }).content, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  const exports: any = {};
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
    "@/modules/plugins/preview/usePluginPreview": { usePluginPreview },
    "@/modules/plugins/preview/render-gate": { sharedPreviewQueue: renderGate.sharedPreviewQueue },
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
  const frame = {};
  state.iframeRef.value = Vue.markRaw({ contentWindow: frame });
  const message = listeners.get("message")!;
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
  assert.equal(listeners.has("message"), false);
});
