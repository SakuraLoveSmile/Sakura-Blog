import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import {
  BREAKPOINT_ORDER,
  defaultMultiLayout,
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
  assert.deepEqual(getGridPresetSize("stat-posts", "lg", "small"), { w: 2, h: 12 });
  assert.deepEqual(getGridPresetSize("stat-posts", "lg", "medium"), { w: 3, h: 12 });
  assert.deepEqual(getGridPresetSize("stat-posts", "lg", "large"), { w: 6, h: 12 });
  assert.deepEqual(getGridPresetSize("trend-chart", "lg", "large"), { w: 12, h: 44 });

  const layoutSource = readFileSync(
    join(dirname(fileURLToPath(import.meta.url)), "../web/src/modules/dashboard/widgets/layout.ts"),
    "utf8",
  );
  assert.equal(layoutSource.includes("WIDGET_SIZE_SCALE"), false);
});

test("弹窗选项与尺寸识别共用宽×高去重键，同宽不同高必须全部可选", () => {
  assert.deepEqual(uniquePresetOptions("stat-posts", "lg"), ["small", "medium", "large"]);
  assert.deepEqual(uniquePresetOptions("recent-comments", "lg"), ["small", "medium", "large"]);
  // sm 趋势三档同宽 6 列但高度 36/44/52 不同，不能只留 small。
  assert.deepEqual(uniquePresetOptions("trend-chart", "sm"), ["small", "medium", "large"]);
  assert.deepEqual(uniquePresetOptions("trend-chart", "xxs"), ["small", "medium", "large"]);
  assert.deepEqual(
    getWidgetPresetOptions("trend-chart", "sm").map((option) => option.size),
    [{ w: 6, h: 36 }, { w: 6, h: 44 }, { w: 6, h: 52 }],
  );
  assert.equal(getWidgetSizePreset({ w: 4, h: 50 }, "xxs", "trend-chart"), "large");
  assert.equal(getWidgetSizePreset({ w: 4, h: 42 }, "xxs", "trend-chart"), "medium");
  assert.equal(getWidgetSizePreset({ w: 2, h: 18 }, "xxs", "stat-posts"), "small");
});

test("所有默认布局实例识别出的 preset 必须出现在对应断点选项里", () => {
  for (const breakpoint of BREAKPOINT_ORDER) {
    for (const widget of defaultMultiLayout[breakpoint]) {
      const preset = getWidgetSizePreset(widget, breakpoint, widget.id);
      assert.ok(
        uniquePresetOptions(widget.id, breakpoint).includes(preset),
        `${widget.i}/${breakpoint} 识别为 ${preset}，但弹窗选项 ${uniquePresetOptions(widget.id, breakpoint).join(",")} 不含该档`,
      );
    }
  }
});

test("尺寸被钳制成相同时，识别出的 preset 仍必须保留在选项中", () => {
  // stat-posts 在 xxs 的 medium/large 都会被钳制为 4×12；识别按预设顺序返回 medium，
  // 弹窗必须保留 medium 而不是把它去重掉，否则当前档位无处可选中。
  assert.deepEqual(uniquePresetOptions("stat-posts", "xxs"), ["small", "medium"]);
  assert.equal(getWidgetSizePreset({ w: 4, h: 12 }, "xxs", "stat-posts"), "medium");
  assert.ok(
    uniquePresetOptions("stat-posts", "xxs").includes(
      getWidgetSizePreset({ w: 4, h: 12 }, "xxs", "stat-posts"),
    ),
  );

  // 任意宽高组合识别出的 preset 都必须出现在选项里；去重只允许丢掉识别永远不会返回的档位。
  for (const widgetId of Object.keys(WIDGET_PRESET_SPECS)) {
    for (const breakpoint of BREAKPOINT_ORDER) {
      const options = uniquePresetOptions(widgetId, breakpoint);
      for (let w = 1; w <= 12; w += 1) {
        for (let h = 1; h <= 64; h += 1) {
          const preset = getWidgetSizePreset({ w, h }, breakpoint, widgetId);
          assert.ok(
            options.includes(preset),
            `${widgetId}/${breakpoint} ${w}×${h} 识别为 ${preset}，但选项 ${options.join(",")} 不含该档`,
          );
        }
      }
    }
  }
});

test("sm/xs/xxs 趋势与默认中档组件必须提供三档并选中 medium", () => {
  for (const breakpoint of ["sm", "xs", "xxs"] as const) {
    for (const widget of defaultMultiLayout[breakpoint]) {
      const preset = getWidgetSizePreset(widget, breakpoint, widget.id);
      if (!["trend-chart", "recent-posts", "recent-comments", "category-donut", "stellar-version"].includes(widget.id)) {
        continue;
      }
      assert.equal(preset, "medium", `${widget.i}/${breakpoint} 默认应为中档`);
      assert.deepEqual(
        uniquePresetOptions(widget.id, breakpoint),
        ["small", "medium", "large"],
        `${widget.id}/${breakpoint} 应列出三档`,
      );
    }
  }
});
