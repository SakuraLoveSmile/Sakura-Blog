import { test } from "node:test";
import assert from "node:assert/strict";
import { uniquePresetOptions } from "../web/src/modules/dashboard/widgets/size-preset-options.ts";

const presets = {
  small: { l: 3, m: 6, s: 12 },
  medium: { l: 6, m: 6, s: 12 },
  large: { l: 8, m: 12, s: 12 },
} as const;

test("size presets: m 断点隐藏相同跨度的 medium", () => {
  assert.deepEqual(uniquePresetOptions(presets, "m"), ["small", "large"]);
});

test("size presets: s 断点只保留一个全宽代表项", () => {
  assert.deepEqual(uniquePresetOptions(presets, "s"), ["small"]);
});

test("size presets: l 断点保留三个不同跨度", () => {
  assert.deepEqual(uniquePresetOptions(presets, "l"), ["small", "medium", "large"]);
});
