// BLG-20261031-03 修复轮次：顶栏气泡切换不串台、focus 与 hover 共用防抖、点击插入不多打请求。
import { test, mock } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import * as Vue from "../web/node_modules/vue/index.js";
import { parse, compileScript } from "../web/node_modules/vue/compiler-sfc/index.js";
import ts from "../web/node_modules/typescript/lib/typescript.js";
import { sharedPreviewQueue } from "../web/src/modules/plugins/preview/render-gate.ts";

const { descriptor } = parse(
  readFileSync(new URL("../web/src/modules/posts/EditorToolbar.vue", import.meta.url), "utf8"),
);
const compiled = ts.transpileModule(compileScript(descriptor, { id: "toolbar-preview-test" }).content, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
}).outputText;

function mountToolbar() {
  const calls = {
    schedule: [] as { markdown: string; options: any }[],
    clear: 0,
    reset: 0,
  };
  let capturedOptions: any;
  const engine = {
    status: Vue.ref("ready"),
    srcdoc: Vue.ref(""),
    previewError: Vue.ref(""),
    frameHeight: Vue.ref(180),
    scheduleRender(markdown: string, options: any) {
      calls.schedule.push({ markdown, options });
      engine.srcdoc.value = `<p>${markdown}</p>`;
      engine.status.value = "ready";
    },
    clearPreview() {
      calls.clear++;
      engine.srcdoc.value = "";
      engine.status.value = "idle";
    },
    resetHeight() {
      calls.reset++;
    },
    retry() {},
    onMessage() {},
  };
  const usePluginPreview = (_getInfo: any, _overrides: any, options: any) => {
    capturedOptions = options;
    return engine;
  };
  const exports: any = {};
  const imports: Record<string, any> = {
    vue: Vue,
    "@iconify/vue": { Icon: { render: () => null } },
    "@/components/ui": { VModal: { render: () => null }, VButton: { render: () => null } },
    "@/modules/plugins/plugin-types": {
      PLUGIN_CATEGORIES: ["提示标注", "布局容器", "文艺装饰", "媒体播放", "数据图表", "GitHub"],
      previewInfoFor: () => ({ capability: "static", height: 180 }),
    },
    "@/modules/plugins/usePluginCatalog": {
      loadPluginCatalog: async () => [],
      runtimePlugins: Vue.ref([]),
    },
    "@/modules/plugins/preview/usePluginPreview": { usePluginPreview },
    "@/modules/plugins/preview/render-gate": { sharedPreviewQueue },
  };
  new Function("require", "exports", compiled)((name: string) => {
    assert.ok(name in imports, `Unexpected toolbar dependency: ${name}`);
    return imports[name];
  }, exports);

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
  const app = renderer.createApp({
    setup() {
      state = exports.default.setup(
        { kind: "rich", mode: "edit", sidebarOpen: false, isEdit: false, fullscreen: false },
        { expose() {}, emit() {} },
      );
      return () => null;
    },
  });
  app.mount({});
  return { state, app, calls, capturedOptions: () => capturedOptions };
}

const originalDocument = (globalThis as any).document;
const originalWindow = (globalThis as any).window;

function installDomStubs() {
  (globalThis as any).document = { addEventListener() {}, removeEventListener() {} };
  (globalThis as any).window = { addEventListener() {}, removeEventListener() {} };
}

test("气泡切换到不同插件先清空旧文档，同插件重复进入不清空", async (t) => {
  installDomStubs();
  t.after(() => {
    (globalThis as any).document = originalDocument;
    (globalThis as any).window = originalWindow;
  });
  mock.timers.enable({ apis: ["setTimeout"] });
  try {
    const { state, app, calls } = mountToolbar();
    const a = { name: "a", title: "A", example: "{% a %}" } as any;
    const b = { name: "b", title: "B", example: "{% b %}" } as any;
    const item = { offsetTop: 12 } as HTMLElement;

    state.onItemFocus(a, { currentTarget: item });
    mock.timers.tick(300);
    assert.deepEqual(calls.schedule.map((c) => c.markdown), ["{% a %}"]);
    assert.equal(calls.clear, 0, "首个插件无需清空");

    state.onItemFocus(b, { currentTarget: item });
    mock.timers.tick(300);
    assert.equal(calls.clear, 1, "A→B 必须先清掉旧文档，避免标题 B 配内容 A");
    assert.equal(calls.schedule.at(-1)?.markdown, "{% b %}");

    state.onItemFocus(b, { currentTarget: item });
    mock.timers.tick(300);
    assert.equal(calls.clear, 1, "同一插件重复进入不得清空");
    app.unmount();
  } finally {
    mock.timers.reset();
  }
});

test("focus 与 hover 共用 300ms 防抖：Tab 连击只渲染最后一项", async (t) => {
  installDomStubs();
  t.after(() => {
    (globalThis as any).document = originalDocument;
    (globalThis as any).window = originalWindow;
  });
  mock.timers.enable({ apis: ["setTimeout"] });
  try {
    const { state, app, calls } = mountToolbar();
    const item = { offsetTop: 0 } as HTMLElement;
    const plugins = ["a", "b", "c"].map((name) => ({ name, title: name, example: `{% ${name} %}` }));

    for (const plugin of plugins) {
      state.onItemFocus(plugin, { currentTarget: item });
      mock.timers.tick(100);
    }
    assert.deepEqual(calls.schedule, [], "连续 focus 不应立即渲染");
    mock.timers.tick(200);
    assert.deepEqual(calls.schedule.map((c) => c.markdown), ["{% c %}"], "只渲染最后一次 focus");
    app.unmount();
  } finally {
    mock.timers.reset();
  }
});

test("focus 后点击插入：取消待执行预览，不多打请求", async (t) => {
  installDomStubs();
  t.after(() => {
    (globalThis as any).document = originalDocument;
    (globalThis as any).window = originalWindow;
  });
  mock.timers.enable({ apis: ["setTimeout"] });
  try {
    const { state, app, calls } = mountToolbar();
    const plugin = { name: "a", title: "A", example: "{% a %}" } as any;
    const item = { offsetTop: 0 } as HTMLElement;

    state.onItemFocus(plugin, { currentTarget: item });
    mock.timers.tick(100);
    state.insertPlugin(plugin);
    assert.equal(calls.clear, 1, "插入时清空气泡");
    mock.timers.tick(1000);
    assert.deepEqual(calls.schedule, [], "点击插入后不得再补一次预览请求");
    app.unmount();
  } finally {
    mock.timers.reset();
  }
});

test("顶栏预览接入 sharedPreviewQueue", async (t) => {
  installDomStubs();
  t.after(() => {
    (globalThis as any).document = originalDocument;
    (globalThis as any).window = originalWindow;
  });
  const { state, app, capturedOptions } = mountToolbar();
  assert.equal(capturedOptions().queue, sharedPreviewQueue);
  state.onItemFocus({ name: "a", title: "A", example: "{% a %}" } as any, {
    currentTarget: { offsetTop: 0 } as HTMLElement,
  });
  app.unmount();
});
