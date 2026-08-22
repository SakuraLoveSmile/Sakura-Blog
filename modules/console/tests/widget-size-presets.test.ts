import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import {
  getGridPresetSize,
  getWidgetSizePreset,
  WIDGET_PRESET_SPECS,
} from "../web/src/modules/dashboard/widgets/layout.ts";
import { getWidgetPresetOptions, uniquePresetOptions } from "../web/src/modules/dashboard/widgets/size-preset-options.ts";

test("每个已注册组件都拥有三档显式规格", () => {
  for (const widgetId of Object.keys(WIDGET_PRESET_SPECS)) {
    assert.ok(WIDGET_PRESET_SPECS[widgetId], `${widgetId} 缺少尺寸矩阵`);
    for (const breakpoint of ["lg", "sm", "xxs"] as const) {
      const sizes = getWidgetPresetOptions(widgetId, breakpoint);
      assert.ok(sizes.length >= 1, `${widgetId}/${breakpoint} 没有可用预设`);
      for (const option of sizes) {
        assert.ok([2, 3, 4, 6, 8, 12].includes(option.size.w), `${widgetId}/${breakpoint} 使用了非规范列宽`);
        assert.ok(option.size.w >= 1);
        assert.ok(option.size.h >= 1);
      }
    }
  }
});

test("规格矩阵使用整洁的三档网格宽度而不是线性缩放", () => {
  assert.deepEqual(getGridPresetSize("stat-posts", "lg", "small"), { w: 2, h: 14 });
  assert.deepEqual(getGridPresetSize("stat-posts", "lg", "medium"), { w: 3, h: 19 });
  assert.deepEqual(getGridPresetSize("stat-posts", "lg", "large"), { w: 6, h: 19 });
  assert.deepEqual(getGridPresetSize("trend-chart", "lg", "large"), { w: 12, h: 52 });

  const layoutSource = readFileSync(
    join(dirname(fileURLToPath(import.meta.url)), "../web/src/modules/dashboard/widgets/layout.ts"),
    "utf8",
  );
  assert.equal(layoutSource.includes("WIDGET_SIZE_SCALE"), false);
});

test("相同断点列跨度只保留一个语义代表", () => {
  assert.deepEqual(uniquePresetOptions("stat-posts", "lg"), ["small", "medium", "large"]);
  assert.deepEqual(uniquePresetOptions("recent-comments", "lg"), ["small", "medium", "large"]);
  assert.deepEqual(uniquePresetOptions("trend-chart", "sm"), ["small"]);
  assert.deepEqual(uniquePresetOptions("trend-chart", "xxs"), ["small"]);
  assert.equal(getWidgetSizePreset({ w: 4, h: 50 }, "xxs", "trend-chart"), "small");
  assert.equal(getWidgetSizePreset({ w: 2, h: 18 }, "xxs", "stat-posts"), "small");
});
