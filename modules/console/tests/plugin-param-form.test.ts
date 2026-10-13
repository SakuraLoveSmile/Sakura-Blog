// 行为面：PluginParamForm 的初始值策略（纯逻辑抽到 shortcode.ts，便于不经 Vue 组件断言）。
// 机械校验入口：make test → console-test → pnpm run test → tsx --test ../tests/*.test.ts
import { test } from "node:test";
import assert from "node:assert/strict";
import { editedFormPresence, initFormValues } from "../web/src/modules/plugins/shortcode.ts";
import type { PluginFormField } from "../web/src/modules/plugins/plugin-types.ts";

const SURFACE = "PluginParamForm 初始值";

const form: PluginFormField[] = [
  { key: "color", label: "配色", type: "select", role: "named", options: ["", "blue", "green"], default: "blue" },
  { key: "title", label: "标题", type: "text", role: "positional", default: "默认标题" },
  { key: "open", label: "默认展开", type: "flag", role: "flag", default: "true" },
  { key: "content", label: "正文", type: "textarea", role: "content", default: "默认正文" },
];

test(`${SURFACE} · 插入模式（initialValues 未传）使用 catalog default`, () => {
  assert.deepEqual(initFormValues(form), {
    color: "blue",
    title: "默认标题",
    open: true,
    content: "默认正文",
  });
});

test(`${SURFACE} · 编辑模式缺失字段取空值，不注入 default`, () => {
  // 对应 {% folding 标题 %}：open 没写，面板必须显示未勾选而不是 default true
  assert.deepEqual(initFormValues(form, { title: "真实标题", content: "真实正文" }), {
    color: "",
    title: "真实标题",
    open: false,
    content: "真实正文",
  });
});

test(`${SURFACE} · 编辑模式：flag 接受布尔与 "true" 两种表示`, () => {
  assert.equal(initFormValues(form, { open: true }).open, true);
  assert.equal(initFormValues(form, { open: "true" }).open, true);
  assert.equal(initFormValues(form, { open: false }).open, false);
  assert.equal(initFormValues(form, { open: "false" }).open, false);
  assert.equal(initFormValues(form, {}).open, false);
});

test(`${SURFACE} · 编辑模式：数值与布尔一律转成字符串，空值不回退 default`, () => {
  const numberForm: PluginFormField[] = [
    { key: "limit", label: "显示条数", type: "number", role: "named", default: 5 },
  ];
  assert.deepEqual(initFormValues(numberForm, { limit: 20 }), { limit: "20" });
  assert.deepEqual(initFormValues(numberForm, {}), { limit: "" });
});

test(`${SURFACE} · 零参数表单（links）两种模式都产出空对象`, () => {
  assert.deepEqual(initFormValues([]), {});
  assert.deepEqual(initFormValues([], {}), {});
});

const imageForm: PluginFormField[] = [
  { key: "src", label: "图片", type: "text", role: "positional" },
  { key: "width", label: "宽度", type: "text", role: "named", default: "100%" },
  { key: "height", label: "高度", type: "text", role: "named" },
  { key: "fancybox", label: "灯箱", type: "select", role: "named", options: ["", "true", "false"] },
];

test("属性编辑 · 新增高度会提交，原有宽度和显式 false 保留，未动字段不注入", () => {
  const present = ["src", "width", "fancybox"];
  const original = initFormValues(imageForm, { src: "/uploads/cover.png", width: "80%", fancybox: "false" });
  assert.deepEqual(editedFormPresence(imageForm, present, original, { ...original, height: "60px" }), [
    ...present,
    "height",
  ]);
  assert.deepEqual(present, ["src", "width", "fancybox"]);
  assert.deepEqual(editedFormPresence(imageForm, present, original, { ...original }), present);
});

test("属性编辑 · 累计新字段、清空、false 和零值均保留精确存在性", () => {
  const original = initFormValues(imageForm, { src: "/uploads/cover.png" });
  const sized = { ...original, width: "240px" };
  const widthPresent = editedFormPresence(imageForm, ["src"], original, sized);
  assert.deepEqual(widthPresent, ["src", "width"]);
  const cleared = { ...sized, width: "", fancybox: "false", height: "0" };
  assert.deepEqual(editedFormPresence(imageForm, widthPresent, sized, cleared), ["src", "width", "height", "fancybox"]);
});

test("属性编辑 · 插入模式不启用精确过滤，零参编辑与未知字段保持原契约", () => {
  const original = initFormValues(imageForm);
  assert.equal(editedFormPresence(imageForm, undefined, original, { ...original, height: "60px" }), undefined);
  assert.deepEqual(editedFormPresence([], [], {}, {}), []);
  assert.deepEqual(editedFormPresence(imageForm, ["src"], original, { ...original, unknown: "opaque" }), ["src"]);
});
