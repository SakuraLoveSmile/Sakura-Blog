import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import * as Vue from "../web/node_modules/vue/index.js";
import { parse, compileScript } from "../web/node_modules/vue/compiler-sfc/index.js";
import ts from "../web/node_modules/typescript/lib/typescript.js";
import { useLocalDraft } from "../../../packages/shared/src/composables/useLocalDraft.ts";
import { stellarEditorContextKey } from "../web/src/components/editor/stellar-context.ts";

const compile = (source: string) =>
  ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText;
function component(path: string, imports: Record<string, any>) {
  const base = process.env.UC_LIFECYCLE_BASELINE;
  const source = base
    ? readFileSync(`${base}/modules/console/web/src/${path}`, "utf8")
    : readFileSync(new URL(`../web/src/${path}`, import.meta.url), "utf8");
  const { descriptor } = parse(source);
  const exports: any = {};
  new Function(
    "require",
    "exports",
    compile(
      compileScript(descriptor, {
        id: "uc-lifecycle",
        inlineTemplate: true,
      }).content,
    ),
  )((name: string) => {
    if (name === "vue") return { ...Vue, Teleport: Vue.Fragment };
    if (name === "@iconify/vue") return { Icon: { render: () => null } };
    if (name in imports) return imports[name];
    throw new Error(`Unexpected import ${name}`);
  }, exports);
  return exports.default;
}
const deferred = <T = any>() => {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((r) => {
    resolve = r;
  });
  return { promise, resolve };
};
type Host = { type: string; text: string; props: Record<string, any>; children: Host[]; parent?: Host };
const node = (type: string, text = ""): Host =>
  ({
    type,
    text,
    props: {},
    children: [],
    style: {},
    getRootNode: () => ({ activeElement: null }),
    focus() {},
    addEventListener() {},
    removeEventListener() {},
    setSelectionRange() {},
    selectionStart: 0,
    selectionEnd: 0,
  }) as any;
const renderer = Vue.createRenderer<Host, Host>({
  createElement: node,
  createText: (text) => node("text", text),
  createComment: (text) => node("comment", text),
  setText: (el, text) => {
    el.text = text;
  },
  setElementText: (el, text) => {
    el.text = text;
    el.children = [];
  },
  patchProp: (el, key, _old, value) => {
    el.props[key] = value;
  },
  insert(el, parent, anchor) {
    if (el.parent) el.parent.children.splice(el.parent.children.indexOf(el), 1);
    el.parent = parent;
    const i = anchor ? parent.children.indexOf(anchor) : -1;
    parent.children.splice(i < 0 ? parent.children.length : i, 0, el);
  },
  remove(el) {
    if (el.parent) el.parent.children.splice(el.parent.children.indexOf(el), 1);
  },
  parentNode: (el) => el.parent || null,
  nextSibling: (el) => el.parent?.children[el.parent.children.indexOf(el) + 1] || null,
});
const walk = (root: Host): Host[] => [root, ...root.children.flatMap(walk)];
const text = (root: Host): string => root.text + root.children.map(text).join("");
const flush = async () => {
  for (let i = 0; i < 8; i++) await Vue.nextTick();
};
const stub = { render: () => null };
const slots = { setup: (_props: any, ctx: any) => () => Vue.h("section", ctx.slots.default?.()) };
function profile(t: any, overrides: Record<string, any> = {}) {
  browserGlobals(t);
  const calls: string[] = [],
    signals: AbortSignal[] = [];
  const auth = Vue.reactive({
    user: { id: 7, role: "reader" },
    fetchMe: async (signal?: AbortSignal) => {
      calls.push("fetchMe");
      if (signal) signals.push(signal);
    },
    logout: async (signal?: AbortSignal) => {
      calls.push("logout");
      if (signal) signals.push(signal);
    },
  });
  const api = {
    get: async (signal?: AbortSignal) => {
      calls.push("get");
      if (signal) signals.push(signal);
      return {
        data: { username: "reader", display_name: "Reader", email: "reader@example.test", avatar: "", has_2fa: false },
      };
    },
    apiTokenStatus: async (signal?: AbortSignal) => {
      calls.push("tokenStatus");
      if (signal) signals.push(signal);
      return { data: { has_token: false } };
    },
    ...overrides,
  };
  const C = component("modules/profile/ProfileView.vue", {
    "@/api": {
      ucProfileApi: api,
      profileApi: { get: () => assert.fail("Console profile") },
      ucApi: overrides.ucApi || {},
      authApi: {},
    },
    "@/uc/stores/auth": { useUCAuthStore: () => auth },
    "@/stores/auth": { useAuthStore: () => assert.fail("Console store") },
    "vue-router": { useRouter: () => ({ push: (url: string) => calls.push(`navigate:${url}`) }) },
    "@/components/AttachmentPickerModal.vue": { __esModule: true, default: stub },
    "@/components/ui": {
      VPageHeader: slots,
      VCard: slots,
      VModal: stub,
      Toast: { success: () => calls.push("toast"), error: () => calls.push("error") },
    },
  });
  const root = node("root"),
    app = renderer.createApp(C, { userCenter: true });
  app.mount(root);
  t.after(() => app.unmount());
  const click = (label: string) => {
    const button = walk(root).find((el) => el.type === "button" && text(el).trim() === label);
    assert.ok(button, `missing button ${label}`);
    return button.props.onClick();
  };
  return { root, app, auth, calls, signals, click };
}

test("UC 改密挂起后卸载并切换身份，晚成功不 logout 或导航", async (t) => {
  const pending = deferred();
  let signal: AbortSignal | undefined,
    count = 0;
  const p = profile(t, {
    ucApi: {
      changePassword: (_old: string, _next: string, incoming?: AbortSignal) => {
        signal = incoming;
        count++;
        return pending.promise;
      },
    },
  });
  await flush();
  for (const [placeholder, value] of [
    ["当前密码", "old"],
    ["新密码", "new"],
    ["确认新密码", "new"],
  ]) {
    const input = walk(p.root).find((el) => el.type === "input" && el.props.placeholder === placeholder)!;
    input.props["onUpdate:modelValue"](value);
  }
  await flush();
  const done = p.click("修改密码");
  p.app.unmount();
  p.auth.user = { id: 9, role: "reader" };
  pending.resolve({});
  await done;
  assert.equal(count, 1);
  assert.ok(!p.calls.includes("logout"));
  assert.equal(signal?.aborted, true);
  assert.ok(!p.calls.includes("toast"));
  assert.ok(!p.calls.some((value) => value.startsWith("navigate:")));
});

test("UC 初始资料迟到不会启动身份刷新或 token 请求", async (t) => {
  const pending = deferred();
  let signal: AbortSignal | undefined;
  const p = profile(t, {
    get: (incoming?: AbortSignal) => {
      signal = incoming;
      return pending.promise;
    },
  });
  p.app.unmount();
  pending.resolve({ data: { display_name: "Late", has_2fa: true } });
  await flush();
  assert.deepEqual(p.calls, []);
  assert.equal(signal?.aborted, true);
});

test("UC 保存资料迟到不再刷新或 toast；挂起 token 请求离页取消", async (t) => {
  const update = deferred(),
    token = deferred();
  let updateSignal: AbortSignal | undefined, tokenSignal: AbortSignal | undefined;
  const p = profile(t, {
    update: (_data: any, signal?: AbortSignal) => {
      updateSignal = signal;
      return update.promise;
    },
    generateApiToken: (signal?: AbortSignal) => {
      tokenSignal = signal;
      return token.promise;
    },
  });
  await flush();
  const before = p.calls.slice();
  const save = p.click("保存"),
    generate = p.click("生成 API Token");
  p.app.unmount();
  update.resolve({});
  token.resolve({ data: { token: "late-secret" } });
  await Promise.all([save, generate]);
  assert.deepEqual(p.calls, before);
  assert.equal(updateSignal?.aborted, true);
  assert.equal(tokenSignal?.aborted, true);
});

test("UC 两步验证确认迟到不再次刷新身份或提示成功", async (t) => {
  const pending = deferred();
  let signal: AbortSignal | undefined;
  const p = profile(t, {
    setup2FA: async () => ({ data: { secret: "fixture", auth_url: "otpauth://fixture", recovery_codes: ["fixture"] } }),
    confirm2FA: (_data: any, incoming?: AbortSignal) => {
      signal = incoming;
      return pending.promise;
    },
  });
  await flush();
  await p.click("开启两步验证");
  await flush();
  walk(p.root)
    .find((el) => el.type === "input" && el.props.placeholder === "6 位动态验证码")!
    .props["onUpdate:modelValue"]("123456");
  await flush();
  const before = p.calls.slice();
  const done = p.click("确认开启");
  p.app.unmount();
  pending.resolve({});
  await done;
  assert.deepEqual(p.calls, before);
  assert.equal(signal?.aborted, true);
});

function browserGlobals(t: any) {
  const saved = {
    ShadowRoot: (globalThis as any).ShadowRoot,
    Document: (globalThis as any).Document,
    window: (globalThis as any).window,
    document: (globalThis as any).document,
    localStorage: globalThis.localStorage,
  };
  (globalThis as any).Document = class {};
  (globalThis as any).ShadowRoot = class {};
  (globalThis as any).window = { addEventListener() {}, removeEventListener() {} };
  (globalThis as any).document = { activeElement: null };
  (globalThis as any).localStorage = { getItem: () => "edit", setItem() {}, removeItem() {} };
  t.after(() => Object.assign(globalThis, saved));
}

test("UC Picker 逆序分页只保留最新页，关闭/卸载请求不能回写或读取后台分组", async (t) => {
  browserGlobals(t);
  const requests: { params: any; signal?: AbortSignal; pending: ReturnType<typeof deferred> }[] = [];
  const C = component("components/AttachmentPickerModal.vue", {
    "@/api": {
      ucApi: {
        attachments: {
          list: (params: any, signal?: AbortSignal) => {
            const pending = deferred();
            requests.push({ params, signal, pending });
            return pending.promise;
          },
        },
      },
      attachmentApi: { list: () => assert.fail("Console attachments") },
      attachmentGroupApi: { list: () => assert.fail("Console groups") },
    },
  });
  const open = Vue.ref(true),
    root = node("root");
  const app = renderer.createApp({ render: () => Vue.h(C, { open: open.value, userCenter: true }) });
  app.mount(root);
  t.after(() => app.unmount());
  const result = (filename: string) => ({
    data: { total: 100, items: [{ id: 1, filename, url: `/${filename}`, mime: "image/png" }] },
  });
  requests[0].pending.resolve(result("first.png"));
  await flush();
  const next = () =>
    walk(root)
      .find((el) => el.type === "button" && text(el).trim() === "下一页")!
      .props.onClick();
  next();
  await flush();
  next();
  await flush();
  assert.deepEqual(
    requests.map((r) => r.params.page),
    [1, 2, 3],
  );
  requests[2].pending.resolve(result("new.png"));
  await flush();
  requests[1].pending.resolve(result("stale.png"));
  await flush();
  assert.deepEqual(
    walk(root)
      .filter((el) => el.type === "img")
      .map((el) => el.props.src),
    ["/new.png"],
  );
  assert.equal(requests[1].signal?.aborted, true);
  next();
  await flush();
  const closing = requests[3];
  open.value = false;
  await flush();
  assert.equal(closing.signal?.aborted, true);
  closing.pending.resolve(result("closed.png"));
  await flush();
  open.value = true;
  await flush();
  assert.equal(requests.at(-1)!.params.page, 1);
  assert.ok(!walk(root).some((el) => el.type === "img" && el.props.src === "/closed.png"));
  app.unmount();
  assert.equal(requests.at(-1)!.signal?.aborted, true);
  requests.at(-1)!.pending.resolve(result("unmounted.png"));
  await flush();
});

test("UC Markdown 上传实际调用 /uc/attachments/upload 并透传 Picker；离页不回填", async (t) => {
  browserGlobals(t);
  const requests: { path: string; options: any; pending: ReturnType<typeof deferred> }[] = [];
  const apiExports: any = {};
  new Function(
    "require",
    "exports",
    compile(readFileSync(new URL("../../../packages/api-client/src/api/uc.ts", import.meta.url), "utf8")),
  )((name: string) => {
    if (name === "../transport")
      return {
        http: {
          post: (path: string, _form: any, options: any) => {
            const pending = deferred();
            requests.push({ path, options, pending });
            return pending.promise;
          },
        },
      };
    throw new Error(name);
  }, apiExports);
  let pickerUC: boolean | undefined;
  const C = component("components/MarkdownEditor.vue", {
    "./editor/stellar-context": { stellarEditorContextKey },
    "@/composables/useLocalDraft": { useLocalDraft },
    "@/api": {
      ucApi: apiExports.ucApi,
      attachmentApi: { upload: () => assert.fail("Console upload") },
      previewApi: {},
    },
    "@/components/AttachmentPickerModal.vue": {
      __esModule: true,
      default: {
        props: ["userCenter"],
        setup: (props: any) => {
          pickerUC = props.userCenter;
          return () => null;
        },
      },
    },
  });
  const value = Vue.ref("body"),
    root = node("root");
  const app = renderer.createApp({
    render: () =>
      Vue.h(C, {
        modelValue: value.value,
        userCenter: true,
        "onUpdate:modelValue": (v: string) => {
          value.value = v;
        },
      }),
  });
  app.mount(root);
  t.after(() => app.unmount());
  const file = new File(["fixture"], "image.png", { type: "image/png" });
  const upload = () =>
    walk(root)
      .find((el) => el.type === "input" && el.props.type === "file")!
      .props.onChange({ target: { files: [file], value: "" } });
  upload();
  await flush();
  assert.equal(pickerUC, true);
  assert.equal(requests[0].path, "/uc/attachments/upload");
  requests[0].pending.resolve({ data: { url: "/own/image.png" } });
  await flush();
  assert.ok(value.value.includes("![image.png](/own/image.png)"));
  upload();
  await flush();
  const before = value.value;
  app.unmount();
  assert.equal(requests[1].options.signal.aborted, true);
  requests[1].pending.resolve({ data: { url: "/late.png" } });
  await flush();
  assert.equal(value.value, before);
});
