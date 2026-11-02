// BLG-20261031-03：标签预览共享调度器（防抖合并/缓存/迟到丢弃/失败保留旧文档/并发上限）。
// 纯 TS，不依赖 Vue；对请求次数做精确断言（mutation-check 思路）。
import { test, mock } from "node:test";
import assert from "node:assert/strict";
import {
  PreviewCache,
  PreviewQueue,
  PreviewScheduler,
  previewCacheKey,
  previewFingerprint,
  type PreviewResult,
} from "../web/src/modules/plugins/preview/render-gate.ts";
import { previewHeightFromMessage } from "../web/src/modules/plugins/preview/preview-document.ts";

function result(mark: string): PreviewResult {
  return { html: `<p>${mark}</p>`, document: `<!doctype html><p>${mark}</p>`, token: `token-${mark}` };
}

async function flush(): Promise<void> {
  for (let i = 0; i < 6; i++) await Promise.resolve();
}

test("连续输入 10 次只发一次请求，且应用最后一次输入", async () => {
  mock.timers.enable({ apis: ["setTimeout"] });
  try {
    const calls: string[] = [];
    let state = { status: "idle", result: null as PreviewResult | null, error: "" };
    const scheduler = new PreviewScheduler({
      delayMs: 400,
      cache: new PreviewCache(20),
      run: async (markdown) => {
        calls.push(markdown);
        return result(markdown);
      },
      onChange: (next) => (state = next),
    });
    for (let i = 0; i < 10; i++) scheduler.schedule({ markdown: `v${i}` });
    mock.timers.tick(399);
    await flush();
    assert.deepEqual(calls, [], "防抖窗口内不应发请求");
    assert.equal(state.status, "loading");
    mock.timers.tick(1);
    await flush();
    assert.deepEqual(calls, ["v9"], "10 次输入应合并为 1 次请求");
    assert.equal(state.status, "ready");
    assert.equal(state.result?.token, "token-v9");
  } finally {
    mock.timers.reset();
  }
});

test("相同 profile+markdown+context 命中缓存：跨实例 0 新请求", async () => {
  mock.timers.enable({ apis: ["setTimeout"] });
  try {
    const cache = new PreviewCache(20);
    const calls: string[] = [];
    const make = () =>
      new PreviewScheduler({
        delayMs: 0,
        cache,
        run: async (markdown) => {
          calls.push(markdown);
          return result(markdown);
        },
        onChange: () => {},
      });
    const a = make();
    a.schedule({ markdown: "{% note hi %}", immediate: true });
    mock.timers.tick(0);
    await flush();
    assert.deepEqual(calls, ["{% note hi %}"]);

    // 另一个实例（例如另一个标签节点）命中同一缓存，同步应用且不发请求。
    let state = { status: "idle", result: null as PreviewResult | null, error: "" };
    const b = new PreviewScheduler({
      delayMs: 0,
      cache,
      run: async (markdown) => {
        calls.push(markdown);
        return result(markdown);
      },
      onChange: (next) => (state = next),
    });
    b.schedule({ markdown: "{% note hi %}", immediate: true });
    assert.equal(state.status, "ready", "缓存命中应立即 ready，不进入 loading");
    assert.equal(state.result?.token, "token-{% note hi %}");
    assert.deepEqual(calls, ["{% note hi %}"], "缓存命中不得再发请求");

    // 上下文指纹不同（标题变化）→ 新 key，重新请求。
    b.schedule({
      markdown: "{% note hi %}",
      metadata: { title: "新标题" },
      immediate: true,
    });
    mock.timers.tick(0);
    await flush();
    assert.equal(calls.length, 2, "context 变化必须失效缓存");
  } finally {
    mock.timers.reset();
  }
});

test("迟到响应不覆盖新结果", async () => {
  mock.timers.enable({ apis: ["setTimeout"] });
  try {
    let resolveOld!: (value: PreviewResult) => void;
    let state = { status: "idle", result: null as PreviewResult | null, error: "" };
    const scheduler = new PreviewScheduler({
      delayMs: 0,
      cache: new PreviewCache(20),
      run: (markdown) =>
        markdown === "old"
          ? new Promise<PreviewResult>((resolve) => (resolveOld = resolve))
          : Promise.resolve(result("new")),
      onChange: (next) => (state = next),
    });
    scheduler.schedule({ markdown: "old", immediate: true });
    mock.timers.tick(0);
    await flush();
    scheduler.schedule({ markdown: "new", immediate: true });
    mock.timers.tick(0);
    await flush();
    assert.equal(state.result?.token, "token-new");

    // 旧请求现在才返回：序列号已过期，必须丢弃。
    resolveOld(result("old"));
    await flush();
    assert.equal(state.result?.token, "token-new", "迟到响应不得覆盖新预览");
    assert.equal(state.status, "ready");
  } finally {
    mock.timers.reset();
  }
});

test("失败保留上一次成功文档，重试成功后替换", async () => {
  mock.timers.enable({ apis: ["setTimeout"] });
  try {
    let fail = false;
    let calls = 0;
    let state = { status: "idle", result: null as PreviewResult | null, error: "" };
    const scheduler = new PreviewScheduler({
      delayMs: 0,
      cache: new PreviewCache(20),
      run: async (markdown) => {
        calls++;
        if (fail) throw new Error("boom");
        return result(markdown);
      },
      onChange: (next) => (state = next),
    });
    scheduler.schedule({ markdown: "first", immediate: true });
    mock.timers.tick(0);
    await flush();
    assert.equal(state.result?.token, "token-first");

    fail = true;
    scheduler.schedule({ markdown: "second", immediate: true });
    mock.timers.tick(0);
    await flush();
    assert.equal(state.status, "error");
    assert.equal(state.error, "预览渲染失败");
    assert.equal(state.result?.token, "token-first", "失败时不得清空上一次成功文档");

    fail = false;
    scheduler.retry();
    mock.timers.tick(0);
    await flush();
    assert.equal(state.status, "ready");
    assert.equal(state.result?.token, "token-second");
    assert.equal(calls, 3, "重试应跳过缓存再次请求");
  } finally {
    mock.timers.reset();
  }
});

test("清空/卸载后不再发请求，迟到响应被丢弃", async () => {
  mock.timers.enable({ apis: ["setTimeout"] });
  try {
    let calls = 0;
    const pending: Array<(value: PreviewResult) => void> = [];
    let state = { status: "idle", result: null as PreviewResult | null, error: "" };
    const scheduler = new PreviewScheduler({
      delayMs: 200,
      cache: new PreviewCache(20),
      run: (markdown) => {
        calls++;
        return new Promise<PreviewResult>((resolve) => pending.push(resolve));
      },
      onChange: (next) => (state = next),
    });

    // 防抖窗口内 clear：不应发请求。
    scheduler.schedule({ markdown: "a" });
    scheduler.clear();
    mock.timers.tick(1000);
    await flush();
    assert.equal(calls, 0, "clear 后待执行请求必须取消");
    assert.equal(state.status, "idle");

    // 已发出请求后 dispose：响应到达也不应用。
    scheduler.schedule({ markdown: "b", immediate: true });
    mock.timers.tick(0);
    await flush();
    assert.equal(calls, 1);
    scheduler.dispose();
    pending[0](result("b"));
    await flush();
    assert.equal(state.status, "loading", "卸载后不得应用迟到结果");

    scheduler.schedule({ markdown: "c", immediate: true });
    mock.timers.tick(0);
    await flush();
    assert.equal(calls, 1, "卸载后不得再发请求");
  } finally {
    mock.timers.reset();
  }
});

test("有界缓存按 LRU 淘汰，容量 20", () => {
  const cache = new PreviewCache(20);
  for (let i = 0; i < 25; i++) cache.set(`k${i}`, result(String(i)));
  assert.equal(cache.size, 20);
  assert.equal(cache.get("k0"), undefined, "最旧条目应被淘汰");
  assert.equal(cache.get("k24")?.token, "token-24");
  // 命中后刷新 LRU 次序
  cache.get("k5");
  cache.set("k25", result("25"));
  assert.ok(cache.get("k5"), "刚命中过的条目应保留");
  assert.equal(cache.get("k6"), undefined, "命中最少的条目先淘汰");
});

test("上下文指纹稳定：键顺序不同但内容相同得到同一缓存键", () => {
  const a = previewCacheKey(undefined, "{% note x %}", { title: "t", config: "{}", slug: "s" });
  const b = previewCacheKey(undefined, "{% note x %}", { slug: "s", config: "{}", title: "t" });
  const c = previewCacheKey(undefined, "{% note x %}", { title: "t2", config: "{}", slug: "s" });
  assert.equal(a, b);
  assert.notEqual(a, c);
  assert.equal(previewFingerprint({ a: 1, b: 2 }), previewFingerprint({ b: 2, a: 1 }));
  assert.notEqual(previewCacheKey("component-library", "x"), previewCacheKey(null, "x"));
});

test("并发闸上限 2，且过期的排队请求被跳过", async () => {
  const queue = new PreviewQueue(2);
  let settled = false;
  const first = queue.acquire(() => true);
  const second = queue.acquire(() => true);
  const stale = queue.acquire(() => false).then((granted) => {
    settled = true;
    return granted;
  });
  assert.deepEqual(await Promise.all([first, second]), [true, true]);
  await flush();
  assert.equal(settled, false, "第三个请求应排队等待执行位");
  queue.release();
  assert.equal(await stale, false, "取消的排队请求应被跳过而不是占位");
  assert.equal(settled, true);
  // 被跳过的请求不占用并发位：仍可直接拿到一个执行位
  assert.equal(await queue.acquire(() => true), true);
  queue.release();
  queue.release();
});

test("高度消息只接受当前 token 的合法数值", () => {
  assert.equal(previewHeightFromMessage({ type: "component-preview-resize", token: "t", height: 1200.4 }, "t"), 1200.4);
  assert.equal(previewHeightFromMessage({ type: "component-preview-resize", token: "t", height: "900" }, "t"), null);
  assert.equal(previewHeightFromMessage({ type: "component-preview-resize", token: "old", height: 900 }, "t"), null);
  assert.equal(previewHeightFromMessage({ type: "other", token: "t", height: 900 }, "t"), null);
  assert.equal(previewHeightFromMessage({ type: "component-preview-resize", token: "t", height: -1 }, "t"), null);
  assert.equal(previewHeightFromMessage({ type: "component-preview-resize", token: "t", height: Infinity }, "t"), null);
});
