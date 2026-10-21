import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import * as Vue from "../web/node_modules/vue/index.js";
import { parse, compileScript } from "../web/node_modules/vue/compiler-sfc/index.js";
import ts from "../web/node_modules/typescript/lib/typescript.js";
import * as json from "../web/src/components/editor/stellar-form-json.ts";

const { descriptor } = parse(
  readFileSync(new URL("../web/src/components/editor/PostThemeSettings.vue", import.meta.url), "utf8"),
);
const compiled = ts.transpileModule(compileScript(descriptor, { id: "post-theme-settings" }).content, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
}).outputText;
const component: any = {};
new Function("require", "exports", compiled)((name: string) => {
  if (name === "vue") return Vue;
  if (name === "./stellar-form-json") return json;
  throw new Error("Unexpected import: " + name);
}, component);

function fixture(config: string, t: { after: (fn: () => void) => void }) {
  const props = Vue.reactive({ config, version: 2, syncError: "" });
  const emitted: string[] = [];
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
      state = component.default.setup(props, {
        expose() {},
        emit(event: string, value: string) {
          assert.equal(event, "update:config");
          emitted.push(value);
          props.config = value;
        },
      });
      return () => null;
    },
  });
  app.mount({});
  t.after(() => app.unmount());
  return { props, state, emitted };
}
const original =
  ' { "article": {"style":"tech"}, "comments":{"enabled":null,"options":{"large":9007199254740993,"off":false,"zero":0,"empty":[]}}, "render":{"diagrams":{"provider":"mermaid","options":{"theme":"dark"}}}, "footer":{"license":true,"share":[],"references":[{"title":"Existing","url":"/existing"}]} } ';

test("打开文章设置只回显，单项修改保留原文参数、显式空值和大整数", (t) => {
  const { props, state, emitted } = fixture(original, t);
  assert.equal(emitted.length, 0);
  assert.equal(props.config, original);
  assert.equal(state.selected(state.groups[0].fields[0]), "tech");
  assert.equal(state.selected(state.groups[2].fields[1]), "custom");
  assert.equal(state.complexReferences.value, true);
  assert.equal(state.licenseMode.value, "site");
  assert.equal(state.shareMode.value, "custom");
  state.selectField(state.groups[0].fields[0], "story");
  assert.equal(props.config, original.replace('"style":"tech"', '"style":"story"'));
  assert.equal(emitted.length, 1);
  assert.equal(state.complexReferences.value, true);
});

test("继承、关闭和清空保持不同语义，分享平台与文本可从控件修改", (t) => {
  const { props, state } = fixture("{}", t);
  const math = state.groups[2].fields[0];
  state.selectField(math, "false");
  assert.equal(json.readJSONField(props.config, "render.math"), false);
  state.selectField(math, "");
  assert.equal(json.readJSONField(props.config, "render.math"), undefined);
  state.setReferences("");
  assert.deepEqual(JSON.parse(props.config).footer.references, []);
  state.setField("footer.references", undefined);
  assert.equal(json.readJSONField(props.config, "footer.references"), undefined);
  state.setShareMode("custom");
  assert.deepEqual(JSON.parse(props.config).footer.share, []);
  state.toggleShare("email", true);
  state.toggleShare("qrcode", true);
  assert.deepEqual(JSON.parse(props.config).footer.share, ["email", "qrcode"]);
  state.toggleShare("email", false);
  assert.deepEqual(JSON.parse(props.config).footer.share, ["qrcode"]);
  state.setShareMode("off");
  assert.equal(json.readJSONField(props.config, "footer.share"), false);
  state.setText("footer.license", "My license");
  assert.equal(state.licenseMode.value, "custom");
  assert.equal(json.readJSONField(props.config, "footer.license"), "My license");
  state.setText("footer.license", "");
  assert.equal(json.readJSONField(props.config, "footer.license"), undefined);
});

test("原文编辑同步回显，非法原文及旧版本阻止结构化控件反写", (t) => {
  const { props, state, emitted } = fixture(original, t);
  props.config = '{"article":{"style":"story"},"render":{"math":false}}';
  assert.equal(state.selected(state.groups[0].fields[0]), "story");
  assert.equal(state.selected(state.groups[2].fields[0]), "false");
  assert.equal(emitted.length, 0);
  props.config = "{";
  assert.ok(state.configError.value);
  state.setText("comments.title", "New");
  assert.equal(props.config, "{");
  assert.equal(emitted.length, 0);
  props.config = "{}";
  props.syncError = "Blog JSON is invalid";
  state.setField("comments.enabled", false);
  assert.equal(emitted.length, 0);
  props.syncError = "";
  props.version = 1;
  state.setField("comments.enabled", false);
  assert.equal(emitted.length, 0);
  props.version = 2;
  state.setField("comments.enabled", false);
  assert.equal(json.readJSONField(props.config, "comments.enabled"), false);
});
