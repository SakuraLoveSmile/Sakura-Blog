import { test } from "node:test";
import assert from "node:assert/strict";
import { themeColor, themeColorVariable, themeColors } from "../../../packages/shared/src/lib/theme-colors";

type ThemeTokens = Map<string, string>;
type MutableGlobal = typeof globalThis & {
  document?: unknown;
  getComputedStyle?: unknown;
};

function installTokenReader(tokens: ThemeTokens) {
  const globals = globalThis as MutableGlobal;
  const originalDocument = globals.document;
  const originalGetComputedStyle = globals.getComputedStyle;
  globals.document = { documentElement: {} };
  globals.getComputedStyle = () => ({
    getPropertyValue: (property: string) => tokens.get(property) ?? "",
  });
  return () => {
    globals.document = originalDocument;
    globals.getComputedStyle = originalGetComputedStyle;
  };
}

test("主题键同时兼容 camelCase、kebab-case 与完整变量名", () => {
  assert.equal(themeColorVariable("chart2"), "--color-chart-2");
  assert.equal(themeColorVariable("chart-2"), "--color-chart-2");
  assert.equal(themeColorVariable("primaryHover"), "--color-primary-hover");
  assert.equal(themeColorVariable("primary-hover"), "--color-primary-hover");
  assert.equal(themeColorVariable("stateSuccess"), "--color-state-success");
  assert.equal(themeColorVariable("--color-chart-5"), "--color-chart-5");
  assert.equal(themeColorVariable("--chart_5"), "--color-chart-5");
});

test("token 变化会反映到 themeColor/themeColors，而不是始终走 fallback", () => {
  const tokens: ThemeTokens = new Map([
    ["--color-chart-2", "#112233"],
    ["--color-primary-hover", "#445566"],
    ["--color-state-success", "#778899"],
  ]);
  const restore = installTokenReader(tokens);
  try {
    assert.equal(themeColor("chart2", "#fallback"), "#112233");
    assert.equal(themeColor("primaryHover", "#fallback"), "#445566");
    assert.deepEqual(themeColors({ chart2: "#fallback", stateSuccess: "#fallback", chart5: "#fallback" }), {
      chart2: "#112233",
      stateSuccess: "#778899",
      chart5: "#fallback",
    });

    tokens.set("--color-chart-2", "#aabbcc");
    assert.equal(themeColors({ chart2: "#fallback" }).chart2, "#aabbcc");
  } finally {
    restore();
  }
});

test("无 DOM 或 token 缺失时仍返回 fallback", () => {
  const globals = globalThis as MutableGlobal;
  const originalDocument = globals.document;
  try {
    delete globals.document;
    assert.equal(themeColor("chart2", "#fallback"), "#fallback");
  } finally {
    globals.document = originalDocument;
  }

  const restore = installTokenReader(new Map());
  try {
    assert.equal(themeColor("chart2", "#fallback"), "#fallback");
  } finally {
    restore();
  }
});
