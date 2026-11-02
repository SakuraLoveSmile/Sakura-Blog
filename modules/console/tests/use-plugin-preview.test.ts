// BLG-20261031-03 修复轮次：直接驱动真实 usePluginPreview（不 mock 调度器本身），
// 覆盖刷新保留旧文档、缓存 0 请求、失败+retry、frame 钳制、卸载无请求、迟到丢弃。
import { test, mock } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import * as Vue from "../web/node_modules/vue/index.js";
import ts from "../web/node_modules/typescript/lib/typescript.js";
import * as previewDocument from "../web/src/modules/plugins/preview/preview-document.ts";
import * as renderGate from "../web/src/modules/plugins/preview/render-gate.ts";

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
  return exports.usePluginPreview as (getInfo: () => any, getOverrides?: any, options?: any) => any;
}

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

async function flush(): Promise<void> {
  for (let i = 0; i < 8; i++) await Promise.resolve();
}

function result(mark: string) {
  return {
    data: {
      html: `<p>${mark}</p>`,
      document: `<!doctype html><html><head><meta charset="utf-8"></head><body>${mark}</body></html>`,
      preview_token: `token-${mark}`,
    },
  };
}

function mountComposable(render: (markdown: string) => Promise<any>, options: any = {}) {
  const calls: string[] = [];
  const api = {
    render: (markdown: string, profile: string | undefined, metadata: any) => {
      calls.push(markdown);
      void profile;
      void metadata;
      return render(markdown);
    },
  };
  const usePluginPreview = loadUsePluginPreview(api);
  const info = { height: 180 };
  let state: any;
  const app = renderer.createApp({
    setup() {
      state = usePluginPreview(() => ({ capability: "static", height: info.height }), undefined, {
        delayMs: 100,
        cache: new renderGate.PreviewCache(20),
        ...options,
      });
      return () => null;
    },
  });
  app.mount({});
  return { state, app, info, calls };
}

test("刷新期间保留旧 document，成功后替换", async () => {
  mock.timers.enable({ apis: ["setTimeout"] });
  try {
    let resolveB!: (value: any) => void;
    const { state, app } = mountComposable(
      (markdown) => (markdown === "B" ? new Promise((resolve) => (resolveB = resolve)) : Promise.resolve(result(markdown))),
    );
    state.scheduleRender("A", true);
    mock.timers.tick(0);
    await flush();
    assert.ok(state.document.value.includes("A"));
    assert.equal(state.status.value, "ready");

    state.scheduleRender("B", true);
    mock.timers.tick(0);
    await flush();
    assert.equal(state.status.value, "updating", "有旧文档时进入 updating");
    assert.ok(state.document.value.includes("A"), "刷新中必须保留旧文档");
    assert.equal(state.previewHtml.value, "<p>A</p>");

    resolveB(result("B"));
    await flush();
    assert.equal(state.status.value, "ready");
    assert.ok(state.document.value.includes("B"));
    app.unmount();
  } finally {
    mock.timers.reset();
  }
});

test("相同 profile+markdown+context 命中缓存：0 新请求", async () => {
  mock.timers.enable({ apis: ["setTimeout"] });
  try {
    const { state, app, calls } = mountComposable(async (markdown) => result(markdown));
    state.scheduleRender("A", true);
    mock.timers.tick(0);
    await flush();
    assert.equal(calls.length, 1);

    state.scheduleRender("A", { immediate: true });
    assert.equal(state.status.value, "ready", "缓存命中应立即 ready");
    assert.equal(calls.length, 1, "缓存命中不得再发请求");

    state.scheduleRender("A", { immediate: true, metadata: { title: "新标题" } });
    mock.timers.tick(0);
    await flush();
    assert.equal(calls.length, 2, "context 变化必须失效缓存");
    app.unmount();
  } finally {
    mock.timers.reset();
  }
});

test("失败保留旧文档，retry 复用上次请求并恢复", async () => {
  mock.timers.enable({ apis: ["setTimeout"] });
  try {
    let fail = false;
    const { state, app, calls } = mountComposable(async (markdown) => {
      if (fail) throw new Error("boom");
      return result(markdown);
    });
    state.scheduleRender("A", true);
    mock.timers.tick(0);
    await flush();

    fail = true;
    state.scheduleRender("B", true);
    mock.timers.tick(0);
    await flush();
    assert.equal(state.status.value, "error");
    assert.equal(state.previewError.value, "预览渲染失败");
    assert.ok(state.document.value.includes("A"), "失败必须保留上一次成功文档");

    fail = false;
    state.retry();
    mock.timers.tick(0);
    await flush();
    assert.equal(state.status.value, "ready");
    assert.ok(state.document.value.includes("B"));
    assert.equal(calls.length, 3, "retry 应跳过缓存重新请求");
    app.unmount();
  } finally {
    mock.timers.reset();
  }
});

test("resetHeight 应用 frame min/max 钳制，高度消息同一规则", async () => {
  mock.timers.enable({ apis: ["setTimeout"] });
  try {
    const { state, app, info } = mountComposable(async (markdown) => result(markdown), {
      frame: { minHeight: 180, maxHeight: 360, margin: 0 },
    });
    assert.equal(state.frameHeight.value, 180);

    info.height = 20;
    state.resetHeight();
    assert.equal(state.frameHeight.value, 180, "推荐高度低于 min 时钳制到 min");
    info.height = 5000;
    state.resetHeight();
    assert.equal(state.frameHeight.value, 360, "推荐高度高于 max 时钳制到 max");

    state.scheduleRender("A", true);
    mock.timers.tick(0);
    await flush();
    const frame: any = { contentWindow: {} };
    state.onMessage(
      { source: frame.contentWindow, data: { type: "component-preview-resize", token: "token-A", height: 100 } },
      frame,
    );
    assert.equal(state.frameHeight.value, 180, "内容高度低于 min 时保持 min");
    state.onMessage(
      { source: frame.contentWindow, data: { type: "component-preview-resize", token: "token-A", height: 5000 } },
      frame,
    );
    assert.equal(state.frameHeight.value, 360, "内容高度高于 max 时保持 max");
    app.unmount();
  } finally {
    mock.timers.reset();
  }
});

test("默认 frame 下限沿用插件推荐高度，显式 minHeight 仍优先", async () => {
  mock.timers.enable({ apis: ["setTimeout"] });
  try {
    const { state, app, info } = mountComposable(async (markdown) => result(markdown));
    assert.equal(state.frameHeight.value, 180, "初始高度使用插件推荐高度");
    state.scheduleRender("A", true);
    mock.timers.tick(0);
    await flush();
    const frame: any = { contentWindow: {} };
    state.onMessage(
      { source: frame.contentWindow, data: { type: "component-preview-resize", token: "token-A", height: 20 } },
      frame,
    );
    assert.equal(state.frameHeight.value, 180, "默认下限是推荐高度而不是 60");

    info.height = 320;
    state.resetHeight();
    assert.equal(state.frameHeight.value, 320);
    app.unmount();

    const explicit = mountComposable(async (markdown) => result(markdown), {
      frame: { minHeight: 60, maxHeight: 900, margin: 0 },
    });
    assert.equal(explicit.state.frameHeight.value, 180, "显式 min 不改变初始推荐高度");
    explicit.state.scheduleRender("A", true);
    mock.timers.tick(0);
    await flush();
    explicit.state.onMessage(
      { source: frame.contentWindow, data: { type: "component-preview-resize", token: "token-A", height: 20 } },
      frame,
    );
    assert.equal(explicit.state.frameHeight.value, 60, "显式 frame.minHeight 仍优先");
    explicit.app.unmount();
  } finally {
    mock.timers.reset();
  }
});

test("失败时释放 holdHeight，保留旧文档但允许恢复正常收缩", async () => {
  mock.timers.enable({ apis: ["setTimeout"] });
  try {
    let fail = false;
    const { state, app } = mountComposable(
      async (markdown) => {
        if (fail) throw new Error("boom");
        return result(markdown);
      },
      { frame: { minHeight: 60, maxHeight: 900, margin: 0 } },
    );
    state.scheduleRender("A", true);
    mock.timers.tick(0);
    await flush();
    const frame: any = { contentWindow: {} };
    state.onMessage(
      { source: frame.contentWindow, data: { type: "component-preview-resize", token: "token-A", height: 240 } },
      frame,
    );
    assert.equal(state.frameHeight.value, 240);

    state.holdHeight();
    fail = true;
    state.scheduleRender("B", true);
    mock.timers.tick(0);
    await flush();
    assert.equal(state.status.value, "error");
    assert.ok(state.document.value.includes("A"), "失败仍保留旧文档");
    state.onMessage(
      { source: frame.contentWindow, data: { type: "component-preview-resize", token: "token-A", height: 139 } },
      frame,
    );
    assert.equal(state.frameHeight.value, 139, "失败后不得继续冻结高度");
    app.unmount();
  } finally {
    mock.timers.reset();
  }
});

test("刷新期间旧文档迟到的收缩高度被忽略，ready 后恢复接受", async () => {
  mock.timers.enable({ apis: ["setTimeout"] });
  try {
    let resolveB!: (value: any) => void;
    const { state, app } = mountComposable(
      (markdown) => (markdown === "B" ? new Promise((resolve) => (resolveB = resolve)) : Promise.resolve(result(markdown))),
      { frame: { minHeight: 60, maxHeight: 900, margin: 0 } },
    );
    state.scheduleRender("A", true);
    mock.timers.tick(0);
    await flush();
    const frame: any = { contentWindow: {} };
    state.onMessage(
      { source: frame.contentWindow, data: { type: "component-preview-resize", token: "token-A", height: 240 } },
      frame,
    );
    assert.equal(state.frameHeight.value, 240);

    state.scheduleRender("B", true);
    mock.timers.tick(0);
    await flush();
    assert.equal(state.status.value, "updating");
    // 旧文档兜底回调可能报告更小高度；刷新期间必须保持旧高度。
    state.onMessage(
      { source: frame.contentWindow, data: { type: "component-preview-resize", token: "token-A", height: 139 } },
      frame,
    );
    assert.equal(state.frameHeight.value, 240, "更新中不得回退高度");

    resolveB(result("B"));
    await flush();
    assert.equal(state.status.value, "ready");
    state.onMessage(
      { source: frame.contentWindow, data: { type: "component-preview-resize", token: "token-B", height: 139 } },
      frame,
    );
    assert.equal(state.frameHeight.value, 139, "新文档 ready 后允许收缩");
    app.unmount();
  } finally {
    mock.timers.reset();
  }
});

test("holdHeight 在输入解析等待期间冻结收缩，新文档 ready 后解除", async () => {
  mock.timers.enable({ apis: ["setTimeout"] });
  try {
    const { state, app } = mountComposable(async (markdown) => result(markdown), {
      frame: { minHeight: 60, maxHeight: 900, margin: 0 },
    });
    state.scheduleRender("A", true);
    mock.timers.tick(0);
    await flush();
    const frame: any = { contentWindow: {} };
    state.onMessage(
      { source: frame.contentWindow, data: { type: "component-preview-resize", token: "token-A", height: 240 } },
      frame,
    );
    state.holdHeight();
    state.onMessage(
      { source: frame.contentWindow, data: { type: "component-preview-resize", token: "token-A", height: 139 } },
      frame,
    );
    assert.equal(state.frameHeight.value, 240, "解析等待期间不得收缩");

    // 命中缓存立即 ready 也应解除冻结。
    state.scheduleRender("A", { immediate: true });
    state.onMessage(
      { source: frame.contentWindow, data: { type: "component-preview-resize", token: "token-A", height: 139 } },
      frame,
    );
    assert.equal(state.frameHeight.value, 139);
    app.unmount();
  } finally {
    mock.timers.reset();
  }
});

test("卸载后取消待执行请求，在途响应被丢弃且不再发请求", async () => {
  mock.timers.enable({ apis: ["setTimeout"] });
  try {
    let resolveInflight!: (value: any) => void;
    const { state, app, calls } = mountComposable(
      (markdown) =>
        markdown === "inflight"
          ? new Promise((resolve) => (resolveInflight = resolve))
          : Promise.resolve(result(markdown)),
    );
    state.scheduleRender("pending");
    app.unmount();
    mock.timers.tick(1000);
    await flush();
    assert.deepEqual(calls, [], "卸载后防抖中的请求不得发出");

    // 新实例测试在途响应：卸载后 resolve 不得写入状态。
    const second = mountComposable(
      (markdown) => new Promise((resolve, reject) => (markdown === "x" ? (resolveInflight = resolve) : reject())),
    );
    second.state.scheduleRender("x", true);
    mock.timers.tick(0);
    await flush();
    assert.equal(second.calls.length, 1);
    second.app.unmount();
    resolveInflight(result("x"));
    await flush();
    assert.equal(second.state.document.value, "", "卸载后迟到响应不得应用");
    second.state.scheduleRender("y", true);
    mock.timers.tick(0);
    await flush();
    assert.equal(second.calls.length, 1, "卸载后不得再发请求");
  } finally {
    mock.timers.reset();
  }
});

test("迟到响应不覆盖新结果", async () => {
  mock.timers.enable({ apis: ["setTimeout"] });
  try {
    let resolveOld!: (value: any) => void;
    const { state, app } = mountComposable(
      (markdown) =>
        markdown === "old" ? new Promise((resolve) => (resolveOld = resolve)) : Promise.resolve(result(markdown)),
    );
    state.scheduleRender("old", true);
    mock.timers.tick(0);
    await flush();
    state.scheduleRender("new", true);
    mock.timers.tick(0);
    await flush();
    assert.ok(state.document.value.includes("new"));

    resolveOld(result("old"));
    await flush();
    assert.ok(state.document.value.includes("new"), "迟到响应不得覆盖新结果");
    app.unmount();
  } finally {
    mock.timers.reset();
  }
});

test("存储回退按 profile 收窄：component-library 注入，真实上下文不注入", async () => {
  mock.timers.enable({ apis: ["setTimeout"] });
  try {
    const { state, app } = mountComposable(async (markdown) => result(markdown));
    state.scheduleRender("real", { immediate: true, profile: null });
    mock.timers.tick(0);
    await flush();
    assert.ok(state.document.value.includes('data-preview-write-guard="1"'));
    assert.ok(!state.document.value.includes('data-preview-storage="1"'), "真实上下文不得注入内存 Storage");

    state.scheduleRender("fixture", { immediate: true });
    mock.timers.tick(0);
    await flush();
    assert.ok(state.document.value.includes('data-preview-storage="1"'), "component-library 应注入内存 Storage");
    app.unmount();
  } finally {
    mock.timers.reset();
  }
});
