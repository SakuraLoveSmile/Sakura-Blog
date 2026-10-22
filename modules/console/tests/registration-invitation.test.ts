import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import * as Vue from "../web/node_modules/vue/index.js";
import * as Pinia from "../web/node_modules/pinia/dist/pinia.js";
import { parse, compileScript } from "../web/node_modules/vue/compiler-sfc/index.js";
import ts from "../web/node_modules/typescript/lib/typescript.js";
import { QueryClient } from "../web/node_modules/@tanstack/vue-query/build/modern/index.js";

const compile = (source: string) =>
  ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText;
function authStore(queryClient = new QueryClient()) {
  const exports: any = {};
  new Function(
    "require",
    "exports",
    compile(readFileSync(new URL("../web/src/uc/stores/auth.ts", import.meta.url), "utf8")),
  )((name: string) => {
    if (name === "vue") return Vue;
    if (name === "pinia") return Pinia;
    if (name === "@/api/query-client") return { queryClient };
    throw new Error(name);
  }, exports);
  return exports.useUCAuthStore(Pinia.createPinia());
}

test("UC 身份切换及清理取消旧请求并清空上一账号通知缓存", async (t) => {
  const oldFetch = globalThis.fetch;
  t.after(() => {
    globalThis.fetch = oldFetch;
  });
  const queryClient = new QueryClient();
  const store = authStore(queryClient);
  store.user = { id: 1, role: "reader" };
  queryClient.setQueryData(["uc-notifications", 1], { items: ["A-private"] });
  let cancelled = false;
  const pending = queryClient
    .fetchQuery({
      queryKey: ["uc-pending", 1],
      queryFn: ({ signal }) =>
        new Promise((_resolve, reject) => {
          signal.addEventListener("abort", () => {
            cancelled = true;
            reject(new DOMException("Cancelled", "AbortError"));
          });
        }),
    })
    .catch(() => {});
  globalThis.fetch = (async () => ({
    ok: true,
    status: 200,
    json: async () => ({ user: { id: 2, role: "reader" } }),
  })) as typeof fetch;
  await store.login("fixture-B", "fixture-password");
  await pending;
  assert.equal(cancelled, true);
  assert.equal(queryClient.getQueryCache().getAll().length, 0);
  assert.equal(store.user.id, 2);
  queryClient.setQueryData(["uc-notifications", 2], { items: ["B-private"] });
  store.clearSession();
  assert.equal(queryClient.getQueryCache().getAll().length, 0);
  assert.equal(store.user, null);
});

test("UC 离页取消读取身份和注销，不清理当前账号；旧身份响应不覆盖新登录", async (t) => {
  const oldFetch = globalThis.fetch;
  t.after(() => {
    globalThis.fetch = oldFetch;
  });
  const queryClient = new QueryClient();
  const store = authStore(queryClient);
  store.user = { id: 1, role: "reader" };
  let release!: (data: any) => void;
  let body = new Promise((resolve) => {
    release = resolve;
  });
  globalThis.fetch = (async () => ({ ok: true, status: 200, json: () => body })) as typeof fetch;
  const controller = new AbortController();
  const reading = store.fetchMe(controller.signal);
  controller.abort();
  release({ id: 9, role: "reader", verified: true });
  await reading;
  assert.equal(store.user.id, 1);

  body = new Promise((resolve) => {
    release = resolve;
  });
  const logoutController = new AbortController();
  const logout = store.logout(logoutController.signal);
  logoutController.abort();
  release({});
  await assert.rejects(logout, { name: "AbortError" });
  assert.equal(store.user.id, 1);

  body = new Promise((resolve) => {
    release = resolve;
  });
  const stale = store.fetchMe();
  globalThis.fetch = (async () => ({
    ok: true,
    status: 200,
    json: async () => ({ user: { id: 2, role: "reader" } }),
  })) as typeof fetch;
  await store.login("fixture-B", "fixture-password");
  release({ id: 1, role: "reader", verified: true });
  await stale;
  assert.equal(store.user.id, 2);
});

test("UC 注册读取响应期间取消，不回写当前身份", async (t) => {
  const oldFetch = globalThis.fetch;
  const store = authStore();
  const previous = { id: 7, nickname: "已有身份" };
  store.user = previous;
  let release!: (value: any) => void;
  const body = new Promise((resolve) => {
    release = resolve;
  });
  let options: RequestInit | undefined;
  globalThis.fetch = (async (_url, init) => {
    options = init;
    return { ok: true, status: 201, json: () => body } as any;
  }) as typeof fetch;
  t.after(() => {
    globalThis.fetch = oldFetch;
  });
  const controller = new AbortController();
  const pending = store.register(
    {
      username: "fixture",
      password: "fixture-password",
      display_name: "Fixture",
      email: "fixture@example.test",
      invitation_code: "fixture-code",
    },
    controller.signal,
  );
  await Promise.resolve();
  assert.equal(options?.signal, controller.signal);
  assert.equal(JSON.parse(String(options?.body)).invitation_code, "fixture-code");
  controller.abort();
  release({ user: { id: 9 } });
  await assert.rejects(pending, { name: "AbortError" });
  assert.equal(store.user.id, previous.id);
});

type Host = { type: string; props: Record<string, any>; children: Host[]; parent?: Host };
const node = (type: string): Host => ({ type, props: {}, children: [], addEventListener() {} });
const renderer = Vue.createRenderer<Host, Host>({
  createElement: node,
  createText: () => node("text"),
  createComment: () => node("comment"),
  setText() {},
  setElementText() {},
  patchProp: (el, key, _old, value) => {
    el.props[key] = value;
  },
  insert(el, parent, anchor) {
    el.parent = parent;
    const i = anchor ? parent.children.indexOf(anchor) : -1;
    parent.children.splice(i < 0 ? parent.children.length : i, 0, el);
  },
  remove(el) {
    if (el.parent) el.parent.children.splice(el.parent.children.indexOf(el), 1);
  },
  parentNode: (el) => el.parent || null,
  nextSibling: () => null,
});
const walk = (root: Host): Host[] => [root, ...root.children.flatMap(walk)];

test("UC 注册表单禁止重复提交，离页取消且迟到成功不导航", async () => {
  const { descriptor } = parse(readFileSync(new URL("../web/src/uc/UCRegisterView.vue", import.meta.url), "utf8"));
  const exports: any = {},
    navigations: string[] = [];
  let release!: () => void,
    signal: AbortSignal | undefined,
    calls = 0;
  const pending = new Promise<void>((resolve) => {
    release = resolve;
  });
  const auth = {
    register: async (_data: any, incoming: AbortSignal) => {
      signal = incoming;
      calls++;
      await pending;
    },
  };
  new Function(
    "require",
    "exports",
    compile(compileScript(descriptor, { id: "invitation", inlineTemplate: true }).content),
  )((name: string) => {
    if (name === "vue") return Vue;
    if (name === "vue-router")
      return {
        useRouter: () => ({
          replace: async (url: string) => {
            navigations.push(url);
          },
        }),
      };
    if (name === "./stores/auth") return { useUCAuthStore: () => auth };
    throw new Error(name);
  }, exports);
  const root = node("root"),
    app = renderer.createApp(exports.default);
  app.component("RouterLink", { render: () => null });
  app.mount(root);
  const inputs = walk(root).filter((el) => el.type === "input");
  for (const [i, value] of [
    "fixture",
    "Fixture",
    "fixture@example.test",
    "fixture-password",
    "fixture-code",
  ].entries()) {
    (inputs[i] as any).value = value;
    inputs[i].props["onUpdate:modelValue"](value);
  }
  await Vue.nextTick();
  const form = walk(root).find((el) => el.type === "form")!;
  const event = { preventDefault() {} };
  const first = form.props.onSubmit(event);
  await form.props.onSubmit(event);
  assert.equal(calls, 1);
  app.unmount();
  assert.equal(signal?.aborted, true);
  release();
  await first;
  assert.deepEqual(navigations, []);
});
