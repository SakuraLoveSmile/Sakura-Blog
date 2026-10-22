import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { execFileSync } from "node:child_process";
import * as Vue from "../web/node_modules/vue/index.js";
import { parse, compileScript } from "../web/node_modules/vue/compiler-sfc/index.js";
import ts from "../web/node_modules/typescript/lib/typescript.js";

// 挂载真实 ContentEditor，阻塞子编辑器的 flush，验证切换不能先卸载并丢掉待发送正文。
const source = process.env.STELLAR_EDITOR_BASELINE === "1"
  ? execFileSync("git", ["show", "HEAD:modules/console/web/src/components/ContentEditor.vue"], { encoding: "utf8" })
  : readFileSync(new URL("../web/src/components/ContentEditor.vue", import.meta.url), "utf8");
const { descriptor } = parse(source);
const code = ts.transpileModule(compileScript(descriptor, { id: "switch-test", inlineTemplate: true }).content, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
}).outputText;
type Node = { type: string; props: Record<string, any>; children: Node[]; parent?: Node };
const node = (type: string): Node => ({ type, props: {}, children: [] });
const renderer = Vue.createRenderer<Node, Node>({
  createElement: node, createText: () => node("text"), createComment: () => node("comment"),
  setText() {}, setElementText() {}, patchProp: (el, key, _old, value) => { el.props[key] = value; },
  insert(el, parent, anchor) { if (el.parent) el.parent.children.splice(el.parent.children.indexOf(el), 1); el.parent = parent; const i = anchor ? parent.children.indexOf(anchor) : -1; parent.children.splice(i < 0 ? parent.children.length : i, 0, el); },
  remove(el) { if (el.parent) el.parent.children.splice(el.parent.children.indexOf(el), 1); },
  parentNode: el => el.parent || null, nextSibling: el => el.parent?.children[el.parent.children.indexOf(el) + 1] || null,
});
const walk = (n: Node): Node[] => [n, ...n.children.flatMap(walk)];

for (const pending of ["最后输入的普通文字", "{% note color:purple UpdatedTitle UPDATED_BODY %}"]) {
  test(`模式切换先提交待发送修改：${pending}`, async t => {
    const oldStorage = globalThis.localStorage;
    const storage = new Map<string, string>([["post-editor:kind", "rich"]]);
    (globalThis as any).localStorage = { getItem: (k: string) => storage.get(k), setItem: (k: string, v: string) => storage.set(k, v) };
    let release!: () => void, flushing = false, destroyed = false;
    const barrier = new Promise<void>(resolve => { release = resolve; });
    const Rich = Vue.defineComponent({
      props: ["modelValue"], emits: ["update:modelValue"],
      setup(_props, { emit, expose }) {
        expose({ flushPendingChanges: async () => { flushing = true; await barrier; emit("update:modelValue", pending); await Vue.nextTick(); } });
        Vue.onBeforeUnmount(() => { destroyed = true; });
        return () => Vue.h("rich-fixture");
      },
    });
    const Markdown = Vue.defineComponent({ props: ["modelValue"], setup: props => () => Vue.h("markdown-fixture", { value: props.modelValue }) });
    const exports: any = {};
    new Function("require", "exports", code)((name: string) => {
      if (name === "vue") return Vue;
      if (name === "@iconify/vue") return { Icon: { render: () => null } };
      if (name === "@/components/MarkdownEditor.vue") return { __esModule: true, default: Markdown };
      if (name === "@/components/RichTextEditor.vue") return { __esModule: true, default: Rich };
      throw new Error(`unexpected dependency ${name}`);
    }, exports);
    const value = Vue.ref("旧正文"), root = node("root");
    const app = renderer.createApp({ setup: () => () => Vue.h(exports.default, { modelValue: value.value, "onUpdate:modelValue": (v: string) => { value.value = v; } }) });
    app.mount(root);
    t.after(() => { app.unmount(); (globalThis as any).localStorage = oldStorage; });
    for (let i = 0; i < 8; i++) { await Promise.resolve(); await Vue.nextTick(); }
    assert.ok(walk(root).some(n => n.type === "rich-fixture"));
    const select = walk(root).find(n => n.type === "select" && n.props.title === "切换编辑器")!;
    const switching = select.props.onChange({ target: { value: "markdown" } });
    await Vue.nextTick();
    assert.equal(flushing, true);
    assert.equal(destroyed, false, "flush 完成之前不能卸载富文本");
    release(); await switching; await Vue.nextTick();
    assert.equal(value.value, pending);
    assert.equal(walk(root).find(n => n.type === "markdown-fixture")?.props.value, pending);
    assert.equal(storage.get("post-editor:kind"), "markdown");
  });
}
