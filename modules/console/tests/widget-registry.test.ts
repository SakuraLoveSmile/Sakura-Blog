// 行为面：统一网格下 widget 注册表只负责组件身份，不再绑定宽图/窄图区域。
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const SRC = readFileSync(
  join(dirname(fileURLToPath(import.meta.url)), "../web/src/modules/dashboard/widgets/registry.ts"),
  "utf8",
);
const DASHBOARD_CSS = readFileSync(
  join(dirname(fileURLToPath(import.meta.url)), "../web/src/modules/dashboard/dashboard.css"),
  "utf8",
);
const DASHBOARD_GRID = readFileSync(
  join(dirname(fileURLToPath(import.meta.url)), "../web/src/modules/dashboard/DashboardGrid.vue"),
  "utf8",
);
const DASHBOARD_DESIGNER = readFileSync(
  join(dirname(fileURLToPath(import.meta.url)), "../web/src/modules/dashboard/DashboardDesignerView.vue"),
  "utf8",
);
const DASHBOARD_LAYOUT = readFileSync(
  join(dirname(fileURLToPath(import.meta.url)), "../web/src/modules/dashboard/widgets/layout.ts"),
  "utf8",
);
const DASHBOARD_STATS = readFileSync(
  join(dirname(fileURLToPath(import.meta.url)), "../web/src/modules/dashboard/widgets/useDashboardStats.ts"),
  "utf8",
);
const RECENT_COMMENTS = readFileSync(
  join(dirname(fileURLToPath(import.meta.url)), "../web/src/modules/dashboard/widgets/RecentCommentsWidget.vue"),
  "utf8",
);
const HOURLY = readFileSync(
  join(dirname(fileURLToPath(import.meta.url)), "../web/src/modules/dashboard/widgets/HourlyChartWidget.vue"),
  "utf8",
);
const TREND = readFileSync(
  join(dirname(fileURLToPath(import.meta.url)), "../web/src/modules/dashboard/widgets/TrendChartWidget.vue"),
  "utf8",
);

const blocks = SRC.match(/defineWidget\(\{[\s\S]*?\n\s+\}\),/g) || [];

test("registry: widget id 唯一且组件定义完整", () => {
  assert.ok(blocks.length >= 8, `registry blocks: ${blocks.length}`);
  const seen = new Set<string>();
  for (const block of blocks) {
    const id = block.match(/id: "([^"]+)"/)?.[1] ?? "";
    assert.ok(id, "id 非空");
    assert.ok(!seen.has(id), `重复 id: ${id}`);
    assert.ok(block.includes("component:"), `${id} 缺少 component`);
    assert.ok(block.includes("props:"), `${id} 缺少 props`);
    assert.ok(block.includes("defaultSize:"), `${id} 缺少 defaultSize`);
    seen.add(id);
  }
});

test("registry: 不再把组件绑定到 chartWide/chartNarrow 固定区域", () => {
  assert.doesNotMatch(SRC, /defaultSection/);
  assert.doesNotMatch(SRC, /chartWide|chartNarrow/);
});

test("registry: stat 卡 statKey 均为 /dashboard/stats 响应字段", () => {
  const validKeys = new Set([
    "post_total",
    "post_published",
    "view_total",
    "comment_total",
    "attachment_total",
    "user_total",
  ]);
  for (const match of SRC.matchAll(/statKey: "([^"]+)"/g)) {
    assert.ok(validKeys.has(match[1]), `非法 statKey: ${match[1]}`);
  }
});

test("dashboard grid: inset 间距不会被 100% 高度覆盖", () => {
  const contentRule = DASHBOARD_CSS.match(
    /\.dashboard-grid-stack > \.grid-stack-item > \.grid-stack-item-content \{([\s\S]*?)\}/,
  )?.[1] ?? "";
  const widgetCellRule = DASHBOARD_CSS.match(/\.widget-cell \{([\s\S]*?)\}/)?.[1] ?? "";
  assert.match(contentRule, /inset:\s*10px/);
  assert.match(contentRule, /height:\s*auto/);
  assert.doesNotMatch(widgetCellRule, /height:\s*100%/);
});

test("dashboard grid: 只允许拖动位置，不启用自由缩放", () => {
  assert.match(DASHBOARD_GRID, /disableResize:\s*true/);
  assert.doesNotMatch(DASHBOARD_GRID, /resizestop|resizable:\s*\{/);
  assert.doesNotMatch(DASHBOARD_GRID, /gs-min-|gs-max-/);
});

test("dashboard designer: 提供自适应和三种设备预览", () => {
  assert.match(DASHBOARD_DESIGNER, /key: "auto", label: "自适应"/);
  assert.match(DASHBOARD_DESIGNER, /key: "lg", label: "电脑"/);
  assert.match(DASHBOARD_DESIGNER, /key: "sm", label: "平板"/);
  assert.match(DASHBOARD_DESIGNER, /key: "xxs", label: "手机"/);
  assert.match(DASHBOARD_DESIGNER, /viewMode\.value === "auto" \? autoBreakpoint\.value : currentBreakpoint\.value/);
  assert.match(DASHBOARD_DESIGNER, /setViewMode\("auto"\)/);
});

test("dashboard designer: 保存成功后返回仪表盘", () => {
  assert.match(DASHBOARD_DESIGNER, /Toast\.success\("仪表盘布局已保存"\);\s*await router\.push\("\/dashboard"\);/);
});

test("dashboard phase 3: registers health/top-post widgets and three templates", () => {
  assert.match(SRC, /id: "top-posts"/);
  assert.match(SRC, /id: "system-health"/);
  for (const template of ["creator", "analyst", "monitor"]) {
    assert.match(DASHBOARD_LAYOUT, new RegExp(`id: "${template}"`));
  }
  assert.match(DASHBOARD_LAYOUT, /createDashboardTemplateLayout/);
  assert.match(DASHBOARD_LAYOUT, /"top-posts":/);
  assert.match(DASHBOARD_LAYOUT, /"system-health":/);
});

test("dashboard phase 1/2: refresh and non-blocking chart/comment states are wired", () => {
  assert.match(DASHBOARD_STATS, /DASHBOARD_REFRESH_EVENT/);
  assert.match(DASHBOARD_STATS, /refreshStats\(force = false\)/);
  assert.doesNotMatch(RECENT_COMMENTS, /window\.(prompt|confirm)/);
  assert.match(RECENT_COMMENTS, /comment-reply-popover/);
  assert.match(RECENT_COMMENTS, /handleAvatarError/);
  assert.match(HOURLY, /暂无访问数据/);
  assert.match(HOURLY, /trendDays === 30/);
  assert.match(TREND, /showUv/);
});
