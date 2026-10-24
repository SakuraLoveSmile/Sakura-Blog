// 行为面：本地草稿 composable（src/composables/useLocalDraft.ts）。
// node:test 直跑纯函数：stub localStorage + 假定时器（防抖立即执行）。
// 机械校验入口：make test → console-test → node --test ../tests/*.test.ts
import { test, beforeEach } from "node:test";
import assert from "node:assert/strict";
import { useLocalDraft } from "../web/src/composables/useLocalDraft.ts";

// --- localStorage stub ---
const store = new Map<string, string>();
beforeEach(() => store.clear());
(globalThis as any).localStorage = {
  getItem: (k: string) => (store.has(k) ? store.get(k)! : null),
  setItem: (k: string, v: string) => store.set(k, v),
  removeItem: (k: string) => store.delete(k),
};

// --- 假定时器：setTimeout 立即执行（防抖 2s 无需等待） ---
const realSetTimeout = globalThis.setTimeout;
(globalThis as any).setTimeout = (fn: () => void) => {
  fn();
  return 0;
};

function makeDraft(initial: string, baseline?: string) {
  let value = initial;
  return {
    ...useLocalDraft({
      storageKey: "post:test",
      getValue: () => value,
      setValue: (v: string) => (value = v),
      baseline: baseline ? () => baseline : undefined,
    }),
    readValue: () => value,
  };
}

test("scheduleDraft: 2s 防抖后落 localStorage（draft: 前缀 key）", () => {
  const d = makeDraft("hello");
  d.scheduleDraft();
  assert.equal(store.get("draft:post:test"), "hello");
});

test("scheduleDraft: 空 storageKey 为 no-op", () => {
  let value = "x";
  const { scheduleDraft } = useLocalDraft({
    storageKey: "",
    getValue: () => value,
    setValue: (v: string) => (value = v),
  });
  scheduleDraft();
  assert.equal(store.size, 0);
});

test("checkDraft: 草稿等于当前值 → 清理且不弹条幅", () => {
  store.set("draft:post:test", "hello");
  const d = makeDraft("hello");
  d.checkDraft();
  assert.equal(d.draftBanner.value, false);
  assert.equal(store.has("draft:post:test"), false, "幻影草稿被清除");
});

test("checkDraft: 草稿等于编辑器基线 → 清理且不弹条幅（round-trip 幻影防护）", () => {
  store.set("draft:post:test", "normalized");
  const d = makeDraft("normalized", "normalized");
  d.checkDraft();
  assert.equal(d.draftBanner.value, false);
  assert.equal(store.has("draft:post:test"), false);
});

test("checkDraft: 草稿与当前值不同 → 弹条幅", () => {
  store.set("draft:post:test", "old-draft");
  const d = makeDraft("newer");
  d.checkDraft();
  assert.equal(d.draftBanner.value, true);
});

test("restoreDraft: 回填 setValue 并关闭条幅", () => {
  store.set("draft:post:test", "saved-draft");
  const d = makeDraft("current");
  d.draftBanner.value = true;
  d.restoreDraft();
  assert.equal(d.readValue(), "saved-draft");
  assert.equal(d.draftBanner.value, false);
});

test("clearDraft / discardDraft: 清理存储与条幅", () => {
  store.set("draft:post:test", "x");
  const d = makeDraft("y");
  d.draftBanner.value = true;
  d.discardDraft();
  assert.equal(store.has("draft:post:test"), false);
  assert.equal(d.draftBanner.value, false);
});

test("clearTimeout 恢复真实定时器", () => {
  globalThis.setTimeout = realSetTimeout;
});

test("storageKey getter: 创建后待执行防抖写入新文章命名空间，恢复与清理沿用新key", () => {
  const pending: Array<() => void> = [];
  const originalSetTimeout = globalThis.setTimeout;
  (globalThis as any).setTimeout = (fn: () => void) => {
    pending.push(fn);
    return 0;
  };
  try {
    let key = "post:new";
    let value = "继续输入的正文";
    const draft = useLocalDraft({
      storageKey: () => key,
      getValue: () => value,
      setValue: (v) => {
        value = v;
      },
    });
    draft.scheduleDraft();
    key = "post:created";
    pending[0]();
    assert.equal(store.has("draft:post:new"), false);
    assert.equal(store.get("draft:post:created"), value);
    value = "服务器正文";
    draft.checkDraft();
    assert.equal(draft.draftBanner.value, true);
    draft.restoreDraft();
    assert.equal(value, "继续输入的正文");
    draft.clearDraft();
    assert.equal(store.has("draft:post:created"), false);
    assert.equal(draft.draftBanner.value, false);
  } finally {
    globalThis.setTimeout = originalSetTimeout;
  }
});
