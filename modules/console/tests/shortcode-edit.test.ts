import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "../web/src");
const read = (path: string) => readFileSync(join(ROOT, path), "utf8");

const apiSource = readFileSync(join(ROOT, "../../../../packages/api-client/src/api/plugins.ts"), "utf8");
const viewSource = read("components/editor/StellarTagView.vue");
const popoverSource = read("components/editor/StellarTagPropertyPopover.vue");
const formSource = read("modules/plugins/PluginParamForm.vue");
const catalogSource = read("modules/plugins/plugin-catalog.ts");
const typesSource = read("modules/plugins/plugin-types.ts");
const shortcodeSource = read("modules/plugins/shortcode.ts");

test("shortcode visual editor uses the API parser and existing catalog form", () => {
  assert.match(apiSource, /parseShortcode: \(raw: string\).*plugins\/shortcode\/parse/);
  assert.match(viewSource, /await pluginApi\.parseShortcode\(sourceRaw\)/);
  assert.match(viewSource, /data\.editable/);
  assert.match(viewSource, /props\.updateAttributes\(\{ raw: next \}\)/);
  assert.match(popoverSource, /<PluginParamForm/);
  assert.match(popoverSource, /:initial-values="initialValues"/);
  assert.match(popoverSource, /setTimeout\(\(\) =>/);
  assert.match(popoverSource, /}, 200\)/);
  assert.match(popoverSource, /pointerdown/);
  assert.match(popoverSource, /event\.key === "Escape"/);
  assert.match(popoverSource, /Shortcode 源码/);
  assert.match(formSource, /initialValues\?: Record<string, unknown>/);
});

test("shortcode visual editor supports hybrid variants (navbar / frame)", () => {
  // PluginType 必须保留 hybrid，否则 navbar/frame 的块级写法会被当成单行
  assert.match(typesSource, /"inline" \| "block" \| "hybrid"/);
  assert.match(catalogSource, /value === "block" \|\| value === "hybrid" \? value : "inline"/);
  // buildShortcode 接受可选 variant，只有 variant=inline 才走单行包装
  assert.match(shortcodeSource, /variant\?: PluginVariant/);
  assert.match(shortcodeSource, /plugin\.type === "hybrid" && variant !== "inline"/);
  // 单行 hybrid 正文换行必须折叠为空格，否则 splitArgs 无法切分位置参数
  assert.match(shortcodeSource, /replace\(\/\\s\*\\n\\s\*\/g, " "\)/);
  // frame 单行只做一处局部 form override，不引入 VariantFormRegistry
  assert.match(shortcodeSource, /const FRAME_INLINE_FORM/);
  assert.match(shortcodeSource, /export function formForVariant/);
  assert.match(viewSource, /formForVariant\(editingPlugin\.value, editingVariant\.value\)/);
  assert.match(viewSource, /:variant="editingVariant"/);
  assert.match(popoverSource, /:variant="variant"/);
  assert.match(formSource, /buildShortcode\(props\.plugin, shortcodeValues, props\.variant\)/);
  // variant 经 API 透传
  assert.match(apiSource, /variant\?: "inline" \| "block"/);
});

test("zero-param tags stay editable instead of falling back to source mode", () => {
  // StellarTagView 不能再因为 form 为空就判定不支持
  assert.doesNotMatch(viewSource, /!plugin\.form\?\.length/);
  assert.match(popoverSource, /该组件没有可编辑属性/);
  assert.match(popoverSource, /plugin && !sourceMode && !plugin\.form\?\.length/);
});

test("editing an existing shortcode never injects catalog defaults", () => {
  // initialValues 故意无默认值：undefined=插入模式用 default，有值=编辑模式用解析结果
  assert.doesNotMatch(popoverSource, /initialValues: \(\) => \(\{\}\)/);
  assert.match(formSource, /initFormValues\(props\.plugin\.form \|\| \[\], props\.initialValues\)/);
  // 编辑模式缺失字段取空值
  const initSource = read("modules/plugins/shortcode.ts");
  assert.match(initSource, /initial === undefined \|\| initial === null \? "" : String\(initial\)/);
});
