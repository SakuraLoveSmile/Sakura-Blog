import { test } from "node:test";
import assert from "node:assert/strict";
import {
  normalizeStyleSettings,
  ensureSettingsDefaults,
} from "../web/src/modules/settings/settings-state.ts";

test("normalizeStyleSettings: 处理空值、null 与非对象 primitive 初始输入", () => {
  const cases = [null, undefined, "", 123, true, []];
  for (const input of cases) {
    const res = normalizeStyleSettings(input);
    assert.ok(res && typeof res === "object" && !Array.isArray(res));
    assert.deepEqual(res.color, {});
    assert.deepEqual(res.font_size, {});
    assert.deepEqual(res.font_family, {});
    assert.equal(res.leftbar.ui_style, "card");
    assert.equal(res.leftbar.blur_px, "");
    assert.deepEqual(res.site, {});
    assert.equal(res.codeblock.highlightjs_theme, "");
    assert.equal(res.header_prefix.h2, "");
    assert.equal(res.header_prefix.h3, "");
    assert.equal(res.header_prefix.h4, "");
    assert.equal(res.header_prefix.h5, "");
    assert.equal(res.page_transition.enable, true);
    assert.deepEqual(res.border_radius, {});
    assert.deepEqual(res.gradient, {});
    assert.equal(res.corner_shape, "superellipse(1.25)");
    assert.deepEqual(res.animated_avatar, {});
    assert.deepEqual(res.loading, {});
    assert.equal(res.error_page, "");
  }
});

test("normalizeStyleSettings: 保留未知自定义扩展键与 header_prefix 扩展标签", () => {
  const input = {
    custom_theme_pack: "cyberpunk",
    extra_field: { foo: "bar", count: 42 },
    header_prefix: {
      h2: "## ",
      h6: "###### ",
      custom_tag: "@",
    },
    border_radius: {
      custom_pill: "999px",
    },
  };

  const res = normalizeStyleSettings(input);
  assert.equal(res.custom_theme_pack, "cyberpunk");
  assert.deepEqual(res.extra_field, { foo: "bar", count: 42 });
  assert.equal(res.header_prefix.h2, "## ");
  assert.equal(res.header_prefix.h6, "###### ");
  assert.equal(res.header_prefix.custom_tag, "@");
  assert.equal(res.border_radius.custom_pill, "999px");
});

test("normalizeStyleSettings: 保持显式 false 不被默认值覆写", () => {
  const input = {
    page_transition: {
      enable: false,
    },
  };

  const res = normalizeStyleSettings(input);
  assert.equal(res.page_transition.enable, false);
});

test("normalizeStyleSettings: blur_px 正确规范化数值与字符串", () => {
  // 数值 > 0 转为 px 字符串
  const r1 = normalizeStyleSettings({ leftbar: { blur_px: 16 } });
  assert.equal(r1.leftbar.blur_px, "16px");

  // 数值 <= 0 转为空字符串
  const r2 = normalizeStyleSettings({ leftbar: { blur_px: 0 } });
  assert.equal(r2.leftbar.blur_px, "");

  const r3 = normalizeStyleSettings({ leftbar: { blur_px: -5 } });
  assert.equal(r3.leftbar.blur_px, "");

  // 已有合法 px 字符串保留
  const r4 = normalizeStyleSettings({ leftbar: { blur_px: "24px" } });
  assert.equal(r4.leftbar.blur_px, "24px");

  // 空字符串保留
  const r5 = normalizeStyleSettings({ leftbar: { blur_px: "" } });
  assert.equal(r5.leftbar.blur_px, "");

  // null / undefined 规范化为空字符串
  const r6 = normalizeStyleSettings({ leftbar: { blur_px: null } });
  assert.equal(r6.leftbar.blur_px, "");
});

test("normalizeStyleSettings: 幂等性验证（连续多次规范化结构与值严格恒等）", () => {
  const rawInput = {
    color: { theme: "#2E8DFF" },
    leftbar: { blur_px: 12, ui_style: "dock" },
    page_transition: { enable: false },
    header_prefix: { h2: "§ ", h3: null },
    extra_field: "preserve_me",
  };

  const firstPass = normalizeStyleSettings(JSON.parse(JSON.stringify(rawInput)));
  const firstJson = JSON.stringify(firstPass);

  const secondPass = normalizeStyleSettings(JSON.parse(JSON.stringify(firstPass)));
  const secondJson = JSON.stringify(secondPass);

  const thirdPass = normalizeStyleSettings(secondPass);
  const thirdJson = JSON.stringify(thirdPass);

  assert.equal(firstJson, secondJson);
  assert.equal(secondJson, thirdJson);
  assert.deepEqual(firstPass, secondPass);
  assert.deepEqual(secondPass, thirdPass);
});

test("ensureSettingsDefaults: 包含完整的 style 规范化且保证无副作用脏变", () => {
  const mockSettings = {
    site: { title: "Test Blog" },
    style: {
      leftbar: { blur_px: 10 },
      page_transition: { enable: false },
    },
  };

  const out = ensureSettingsDefaults(mockSettings);
  assert.equal(out.style.leftbar.blur_px, "10px");
  assert.equal(out.style.leftbar.ui_style, "card");
  assert.equal(out.style.page_transition.enable, false);
  assert.equal(out.style.corner_shape, "superellipse(1.25)");

  // 快照比较：再次调用 ensureSettingsDefaults 不产生任何新变动
  const snap1 = JSON.stringify(out.style);
  ensureSettingsDefaults(out);
  const snap2 = JSON.stringify(out.style);
  assert.equal(snap1, snap2);
});

test("normalizeStyleSettings: 纯函数深度冻结测试（深冻结对象不抛错且原对象不被修改）", () => {
  function deepFreeze<T>(obj: T): T {
    if (!obj || typeof obj !== "object") return obj;
    Object.freeze(obj);
    for (const key of Object.keys(obj)) {
      deepFreeze((obj as any)[key]);
    }
    return obj;
  }

  const frozenInput = deepFreeze({
    color: { theme: "#2E8DFF" },
    leftbar: { blur_px: 20 },
    codeblock: { highlightjs_theme: "atom-one-dark" },
    page_transition: { enable: false },
    extra_field: { inner: "immutable" },
  });

  // 不抛出错误（即未对只读冻结对象进行就地属性赋值）
  assert.doesNotThrow(() => {
    const res = normalizeStyleSettings(frozenInput);
    assert.equal(res.leftbar.blur_px, "20px");
    assert.equal(res.leftbar.ui_style, "card");
    assert.equal(res.page_transition.enable, false);
    assert.deepEqual(res.extra_field, { inner: "immutable" });
  });

  // 原冻结对象内容严格不变且依然处于冻结态
  assert.ok(Object.isFrozen(frozenInput));
  assert.ok(Object.isFrozen(frozenInput.leftbar));
  assert.equal((frozenInput.leftbar as any).ui_style, undefined);
});

test("normalizeStyleSettings: 严格保留各种空值与假值（\"\"、0、false、未知键）", () => {
  const input = {
    leftbar: { ui_style: "", blur_px: 0 },
    corner_shape: "",
    error_page: "",
    codeblock: { highlightjs_theme: "" },
    page_transition: { enable: false },
    custom_zero: 0,
    custom_empty: "",
    custom_false: false,
    header_prefix: {
      h2: "",
      custom_tag: "",
    },
  };

  const res = normalizeStyleSettings(input);
  assert.equal(res.leftbar.ui_style, "");
  assert.equal(res.leftbar.blur_px, "");
  assert.equal(res.corner_shape, "");
  assert.equal(res.error_page, "");
  assert.equal(res.codeblock.highlightjs_theme, "");
  assert.equal(res.page_transition.enable, false);
  assert.equal(res.custom_zero, 0);
  assert.equal(res.custom_empty, "");
  assert.equal(res.custom_false, false);
  assert.equal(res.header_prefix.h2, "");
  assert.equal(res.header_prefix.custom_tag, "");
});
