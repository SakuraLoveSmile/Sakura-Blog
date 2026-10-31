import { test } from "node:test";
import assert from "node:assert/strict";
import {
  changeSettingColorAlpha,
  parseSettingColor,
  replaceSettingColor,
} from "../web/src/modules/settings/setting-color.ts";

test("颜色编辑器解析 HEX 的短格式和透明通道", () => {
  assert.deepEqual(parseSettingColor(" #aBc "), { hex: "#aabbcc", alpha: 1 });
  assert.deepEqual(parseSettingColor("#1238"), { hex: "#112233", alpha: 136 / 255 });
  assert.deepEqual(parseSettingColor("#12345680"), { hex: "#123456", alpha: 128 / 255 });
  assert.deepEqual(parseSettingColor("transparent"), { hex: "#000000", alpha: 0 });
});

test("颜色编辑器解析逗号和现代 RGB 语法", () => {
  assert.deepEqual(parseSettingColor("rgb(255, 0, 128)"), { hex: "#ff0080", alpha: 1 });
  assert.deepEqual(parseSettingColor("rgba(0, 128, 255, .25)"), { hex: "#0080ff", alpha: 0.25 });
  assert.deepEqual(parseSettingColor("rgb(100% 0% 50% / 40%)"), { hex: "#ff0080", alpha: 0.4 });
  assert.deepEqual(parseSettingColor("rgb(300 -10 0 / 120%)"), { hex: "#ff0000", alpha: 1 });
});

test("颜色编辑器解析 HSL 单位和透明度", () => {
  assert.deepEqual(parseSettingColor("hsl(192 98% 55%)"), { hex: "#1cd0fd", alpha: 1 });
  for (const hue of ["120", "-240deg", "0.3333333333333333turn", "133.33333333333334grad", `${(Math.PI * 2) / 3}rad`]) {
    assert.deepEqual(parseSettingColor(`hsla(${hue}, 100%, 50%, 50%)`), { hex: "#00ff00", alpha: 0.5 });
  }
  assert.deepEqual(parseSettingColor("hsl(0 0% 25% / 0)"), { hex: "#404040", alpha: 0 });
});

test("取色和色板保留合法原值的透明度；透明度调整保留色相", () => {
  assert.equal(replaceSettingColor("rgba(1, 2, 3, .2)", "#007aff"), "rgba(0, 122, 255, 0.2)");
  assert.equal(replaceSettingColor("transparent", "#ff0000"), "rgba(255, 0, 0, 0)");
  assert.equal(replaceSettingColor("#12345680", "#ffffff"), "rgba(255, 255, 255, 0.502)");
  assert.equal(changeSettingColorAlpha("hsl(120 100% 50% / .2)", 75), "rgba(0, 255, 0, 0.75)");
  assert.equal(changeSettingColorAlpha("rgba(0, 255, 0, .2)", 100), "#00ff00");
  assert.equal(changeSettingColorAlpha("#123456", 0), "rgba(18, 52, 86, 0)");
});

test("无法转换的值不被改写或误判；用户选择色板后才开始新颜色", () => {
  const values = [
    "",
    "red",
    "var(--theme-color)",
    "color(display-p3 1 0 0)",
    "#12345",
    "rgb(1, 2, 3 / .5)",
    "rgb(1 2 3 /)",
    "rgb(1 2)",
    "rgb(1, 2%, 3)",
    "hsl(120 50 50%)",
    "hsl(foo 50% 50%)",
    "hsl(1e308turn 50% 50%)",
    "rgb(Infinity 0 0)",
  ];
  for (const value of values) {
    assert.equal(parseSettingColor(value), undefined, value);
    assert.equal(changeSettingColorAlpha(value, 50), value);
  }
  assert.equal(replaceSettingColor("var(--theme-color)", "#007aff"), "#007aff");
  assert.equal(replaceSettingColor(" #abc ", "invalid"), " #abc ");
  assert.equal(changeSettingColorAlpha(" #abc ", Number.NaN), " #abc ");
});
