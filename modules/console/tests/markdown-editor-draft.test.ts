import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import * as Vue from "../web/node_modules/vue/index.js";
import { parse, compileScript } from "../web/node_modules/vue/compiler-sfc/index.js";
import ts from "../web/node_modules/typescript/lib/typescript.js";
import { useLocalDraft } from "../../../packages/shared/src/composables/useLocalDraft.ts";
import { stellarEditorContextKey } from "../web/src/components/editor/stellar-context.ts";

// 编译并挂载真实 SFC，以实际 Vue props/watch/事件顺序验证；不复制草稿逻辑。
const { descriptor } = parse(
  readFileSync(new URL("../web/src/components/MarkdownEditor.vue", import.meta.url), "utf8"),
);
const script = compileScript(descriptor, { id: "draft-test", inlineTemplate: true });
const code = ts.transpileModule(script.content, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
}).outputText;
type HostNode = {
  type: string;
  text: string;
  props: Record<string, any>;
  style: Record<string, string>;
  children: HostNode[];
  parent?: HostNode;
};
const node = (type: string, text = ""): HostNode => ({ type, text, props: {}, style: {}, children: [] });
const renderer = Vue.createRenderer<HostNode, HostNode>({
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
  insert: (el, parent, anchor) => {
    if (el.parent) el.parent.children.splice(el.parent.children.indexOf(el), 1);
    el.parent = parent;
    const index = anchor ? parent.children.indexOf(anchor) : -1;
    parent.children.splice(index < 0 ? parent.children.length : index, 0, el);
  },
  remove: (el) => {
    if (el.parent) el.parent.children.splice(el.parent.children.indexOf(el), 1);
  },
  parentNode: (el) => el.parent || null,
  nextSibling: (el) => el.parent?.children[el.parent.children.indexOf(el) + 1] || null,
});
function walk(root: HostNode): HostNode[] {
  return [root, ...root.children.flatMap(walk)];
}
function textContent(root: HostNode): string {
  return root.text + root.children.map(textContent).join("");
}

function mountEditor(t: any, saved?: string, initial = "") {
  const store = new Map<string, string>();
  if (saved !== undefined) store.set("draft:post:test", saved);
  const storage = globalThis.localStorage,
    windowValue = (globalThis as any).window;
  (globalThis as any).localStorage = {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => store.set(key, value),
    removeItem: (key: string) => store.delete(key),
  };
  (globalThis as any).window = { addEventListener() {}, removeEventListener() {} };
  t.mock.timers.enable({ apis: ["setTimeout"] });
  const previews: any[] = [];
  const exports = {};
  const component =
    new Function("require", "exports", code)((name: string) => {
      if (name === "vue") return Vue;
      if (name === "./editor/stellar-context") return { stellarEditorContextKey };
      if (name === "@/composables/useLocalDraft") return { useLocalDraft };
      if (name === "@/api")
        return {
          previewApi: {
            render: async (...args: any[]) => {
              previews.push(args);
              return { data: { preview_token: "token", document: "preview" } };
            },
          },
          attachmentApi: { upload: async () => ({ data: { url: "/uploaded.png" } }) },
        };
      if (name === "@iconify/vue") return { Icon: { render: () => null } };
      if (name === "@/components/AttachmentPickerModal.vue")
        return { __esModule: true, default: { render: () => null } };
      throw new Error(`Unexpected SFC dependency: ${name}`);
    }, exports) || (exports as any).default;
  const value = Vue.ref(initial),
    context = Vue.ref<any>({ stellar_version: 2, stellar_config: "{}", blog_extensions: "{}" });
  const root = node("root"),
    editor = Vue.ref<any>();
  const app = renderer.createApp({
    setup() {
      Vue.provide(stellarEditorContextKey, context as any);
      return () =>
        Vue.h(component, {
          ref: editor,
          modelValue: value.value,
          storageKey: "post:test",
          "onUpdate:modelValue": (next: string) => {
            value.value = next;
          },
        });
    },
  });
  app.mount(root);
  t.after(() => {
    app.unmount();
    (globalThis as any).localStorage = storage;
    (globalThis as any).window = windowValue;
  });
  const flush = async (ms = 0) => {
    await Vue.nextTick();
    t.mock.timers.tick(ms);
    await Vue.nextTick();
  };
  const input = async (next: string) => {
    walk(root)
      .find((el) => el.type === "textarea")!
      .props.onInput({ target: { value: next } });
    await Vue.nextTick();
  };
  const click = async (text: string) => {
    const button = walk(root).find((el) => el.type === "button" && textContent(el).trim() === text);
    assert.ok(button, `missing ${text} button`);
    button.props.onClick();
    await Vue.nextTick();
  };
  return { store, value, context, previews, root, editor, flush, input, click, unmount: () => app.unmount() };
}

test("首次 API 水合不会覆盖等待恢复的真实草稿，恢复后真实输入继续备份", async (t) => {
  const e = mountEditor(t, "未保存的真实草稿");
  e.value.value = "服务器正文";
  await e.flush(2500);
  assert.equal(e.store.get("draft:post:test"), "未保存的真实草稿");
  assert.ok(textContent(e.root).includes("检测到本地未保存草稿"));
  await e.click("恢复");
  assert.equal(e.value.value, "未保存的真实草稿");
  await e.input("恢复后的继续编辑");
  await e.flush(2000);
  assert.equal(e.store.get("draft:post:test"), "恢复后的继续编辑");
});

test("没有本地草稿时，API 水合与只改变预览 context 均不制造草稿，预览仍更新", async (t) => {
  const e = mountEditor(t);
  e.value.value = "服务器正文";
  await e.flush(2500);
  assert.equal(e.store.has("draft:post:test"), false);
  e.context.value = { ...e.context.value, title: "只改标题" };
  await e.flush(2500);
  assert.equal(e.store.has("draft:post:test"), false);
  assert.equal(e.previews.at(-1)[2].title, "只改标题");
});

test("已有待恢复草稿时，只改变预览 context 也不会覆盖草稿", async (t) => {
  const e = mountEditor(t, "待恢复草稿", "服务器正文");
  e.context.value = { ...e.context.value, title: "只改标题" };
  await e.flush(2500);
  assert.equal(e.store.get("draft:post:test"), "待恢复草稿");
  assert.equal(e.previews.at(-1)[2].title, "只改标题");
});

test("未决草稿不被用户输入覆盖，丢弃后真实输入按两秒防抖备份，clearDraft取消待写", async (t) => {
  const e = mountEditor(t, "待恢复草稿", "服务器正文");
  await e.input("暂时输入");
  await e.flush(2500);
  assert.equal(e.store.get("draft:post:test"), "待恢复草稿");
  await e.click("丢弃");
  await e.input("第一次输入");
  await e.flush(1500);
  assert.equal(e.store.has("draft:post:test"), false);
  await e.input("第二次输入");
  await e.flush(1999);
  assert.equal(e.store.has("draft:post:test"), false);
  await e.flush(1);
  assert.equal(e.store.get("draft:post:test"), "第二次输入");
  await e.input("已保存的新正文");
  e.editor.value.clearDraft();
  await e.flush(2500);
  assert.equal(e.store.has("draft:post:test"), false);
});

test("外置工具栏插入也是用户修改并备份，卸载取消待写", async (t) => {
  const e = mountEditor(t, undefined, "正文");
  const textarea = walk(e.root).find((el) => el.type === "textarea")! as any;
  textarea.selectionStart = 2;
  textarea.selectionEnd = 2;
  textarea.focus = () => {};
  textarea.setSelectionRange = () => {};
  e.editor.value.insertBlock("工具栏插入");
  await e.flush(2000);
  assert.equal(e.store.get("draft:post:test"), "正文工具栏插入");
  await e.input("未落盘编辑");
  e.unmount();
  await e.flush(2500);
  assert.equal(e.store.get("draft:post:test"), "正文工具栏插入");
});

test("用户上传图片后备份最终链接，不把上传占位符留为草稿", async (t) => {
  const e = mountEditor(t, undefined, "正文");
  const textarea = walk(e.root).find((el) => el.type === "textarea")! as any;
  textarea.selectionStart = 2;
  textarea.selectionEnd = 2;
  textarea.focus = () => {};
  textarea.setSelectionRange = () => {};
  const fileInput = walk(e.root).find((el) => el.type === "input" && el.props.type === "file")!;
  fileInput.props.onChange({ target: { files: [{ name: "fixture.png", type: "image/png" }], value: "" } });
  await e.flush();
  await e.flush(2000);
  assert.equal(e.store.get("draft:post:test"), "正文![fixture.png](/uploaded.png)");
});
