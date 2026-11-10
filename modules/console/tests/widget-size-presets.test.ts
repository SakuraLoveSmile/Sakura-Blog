// 仪表盘尺寸档位：同一断点下的同一档位对所有组件只有一个足迹。
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import {
  BREAKPOINT_ORDER,
  GRID_COLUMNS,
  WIDGET_PRESET_FOOTPRINTS,
  WIDGET_TYPES,
  defaultMultiLayout,
  getGridPresetSize,
  getWidgetSizePreset,
} from "../web/src/modules/dashboard/widgets/layout.ts";
import { getWidgetPresetOptions, uniquePresetOptions } from "../web/src/modules/dashboard/widgets/size-preset-options.ts";

const PRESETS = ["small", "medium", "large"] as const;

test("每个断点每个组件都使用同一组足迹，不被组件钳制", () => {
  for (const bp of BREAKPOINT_ORDER) {
    for (const widgetId of WIDGET_TYPES) {
      for (const preset of PRESETS) {
        assert.deepEqual(
          getGridPresetSize(widgetId, bp, preset),
          WIDGET_PRESET_FOOTPRINTS[bp][preset],
          `${widgetId}/${bp}/${preset} 与足迹表不一致`,
        );
      }
    }
  }
});

test("三档在断点内宽高与面积严格递增", () => {
  for (const bp of BREAKPOINT_ORDER) {
    const sizes = PRESETS.map((preset) => WIDGET_PRESET_FOOTPRINTS[bp][preset]);
    assert.ok(sizes[0].w < sizes[1].w && sizes[1].w <= sizes[2].w, `${bp} 宽度未递增`);
    assert.ok(sizes[0].h < sizes[1].h && sizes[1].h < sizes[2].h, `${bp} 高度未递增`);
    for (let i = 1; i < sizes.length; i += 1) {
      assert.ok(
        sizes[i].w * sizes[i].h > sizes[i - 1].w * sizes[i - 1].h,
        `${bp} 第 ${i + 1} 档面积没有变大`,
      );
    }
    assert.ok(sizes[0].w >= 1 && sizes[2].w <= GRID_COLUMNS[bp], `${bp} 列宽越界`);
  }
});

test("尺寸弹窗在任一断点都列出三档，展示尺寸与足迹一致", () => {
  for (const bp of BREAKPOINT_ORDER) {
    for (const widgetId of WIDGET_TYPES) {
      assert.deepEqual(uniquePresetOptions(widgetId, bp), ["small", "medium", "large"]);
      assert.deepEqual(
        getWidgetPresetOptions(widgetId, bp).map((option) => option.size),
        PRESETS.map((preset) => WIDGET_PRESET_FOOTPRINTS[bp][preset]),
      );
    }
  }
});

test("默认布局只使用足迹表里的三种尺寸", () => {
  for (const bp of BREAKPOINT_ORDER) {
    const allowed = new Set(
      PRESETS.map((preset) => {
        const size = WIDGET_PRESET_FOOTPRINTS[bp][preset];
        return `${size.w}x${size.h}`;
      }),
    );
    for (const widget of defaultMultiLayout[bp]) {
      assert.ok(
        allowed.has(`${widget.w}x${widget.h}`),
        `${widget.i}/${bp} 使用了档位外尺寸 ${widget.w}x${widget.h}`,
      );
    }
  }
});

test("任意历史尺寸都会被识别成仍可选的档位", () => {
  for (const bp of BREAKPOINT_ORDER) {
    for (const widgetId of WIDGET_TYPES) {
      const options = uniquePresetOptions(widgetId, bp);
      for (let w = 1; w <= GRID_COLUMNS[bp]; w += 1) {
        for (let h = 1; h <= 64; h += 1) {
          const preset = getWidgetSizePreset({ w, h }, bp, widgetId);
          assert.ok(
            options.includes(preset),
            `${widgetId}/${bp} ${w}×${h} 识别为 ${preset}，但选项 ${options.join(",")} 不含该档`,
          );
        }
      }
    }
  }
});

test("旧缺陷回归：三档不再重复、不再反向", () => {
  // md「访问趋势」的「大」曾经与「中」完全相同，弹窗因此丢掉大档。
  assert.notDeepEqual(
    getGridPresetSize("trend-chart", "md", "large"),
    getGridPresetSize("trend-chart", "md", "medium"),
  );
  // lg「分类分布」的「大」曾经比「中」还矮。
  assert.ok(
    getGridPresetSize("category-donut", "lg", "large").h >
      getGridPresetSize("category-donut", "lg", "medium").h,
  );
  // 手机「快捷操作」的「中」曾经比「小」还矮。
  for (const bp of ["xs", "xxs"] as const) {
    assert.ok(
      getGridPresetSize("quick-actions", bp, "medium").h >
        getGridPresetSize("quick-actions", bp, "small").h,
    );
  }
  // 统计卡不再出现同一尺寸跨断点换标签。
  assert.deepEqual(getGridPresetSize("stat-posts", "lg", "large"), { w: 12, h: 40 });
  assert.deepEqual(getGridPresetSize("stat-posts", "xxs", "medium"), { w: 4, h: 26 });
});

test("组件清单与 registry 保持一致", () => {
  const source = readFileSync(
    join(dirname(fileURLToPath(import.meta.url)), "../web/src/modules/dashboard/widgets/registry.ts"),
    "utf8",
  );
  const ids = [...source.matchAll(/^ {4}id: "([^"]+)",$/gm)].map((match) => match[1]);
  assert.equal(ids.length, WIDGET_TYPES.length, "registry 与 WIDGET_TYPES 数量不同");
  assert.deepEqual([...ids].sort(), [...WIDGET_TYPES].sort());
});
