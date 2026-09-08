import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const testsRoot = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(testsRoot, "../../..");

function read(relativePath: string): string {
  return readFileSync(join(repoRoot, relativePath), "utf8");
}

test("VModal 和 Dialog 具备规范的 dialog 语义、Esc 监听、焦点捕获与焦点恢复", () => {
  const vModal = read("packages/components/src/ui/VModal.vue");
  assert.match(vModal, /role="dialog"/);
  assert.match(vModal, /aria-modal="true"/);
  assert.match(vModal, /:aria-labelledby=/);
  assert.match(vModal, /e\.key === "Escape"/);
  assert.match(vModal, /previouslyFocusedElement/);
  assert.match(vModal, /e\.key === "Tab"/);

  const dialogTs = read("packages/components/src/ui/dialog.ts");
  assert.match(dialogTs, /role: type === "warning" \|\| type === "error" \? "alertdialog" : "dialog"/);
  assert.match(dialogTs, /"aria-modal": "true"/);
  assert.match(dialogTs, /"aria-labelledby": titleId/);
  assert.match(dialogTs, /e\.key === "Escape"/);
  assert.match(dialogTs, /previousActiveElement/);
  assert.match(dialogTs, /dialogCard\.querySelectorAll/);
});

test("FilterDropdown 使用原生 button 触发器并具备 WAI-ARIA listbox 语义与键盘操作", () => {
  const filterDropdown = read("packages/components/src/ui/FilterDropdown.vue");
  assert.match(filterDropdown, /<button[\s\S]*?ref="triggerBtn"[\s\S]*?aria-haspopup="listbox"/);
  assert.match(filterDropdown, /:aria-expanded="open"/);
  assert.match(filterDropdown, /role="listbox"/);
  assert.match(filterDropdown, /role="option"/);
  assert.match(filterDropdown, /:aria-selected=/);
  assert.match(filterDropdown, /e\.key === "ArrowDown"/);
  assert.match(filterDropdown, /e\.key === "ArrowUp"/);
  assert.match(filterDropdown, /e\.key === "Escape"/);
  assert.match(filterDropdown, /triggerBtn\.value\?\.focus\(\)/);
});

test("VSwitch 绑定原生 disabled、aria 属性并要求调用方全部提供可访问名称", () => {
  const vSwitch = read("packages/components/src/ui/VSwitch.vue");
  assert.match(vSwitch, /role="switch"/);
  assert.match(vSwitch, /:aria-checked="modelValue"/);
  assert.match(vSwitch, /:disabled="disabled"/);
  assert.match(vSwitch, /:aria-disabled="disabled"/);
  assert.match(vSwitch, /aria-label/);

  // 检查关键调用处均提供了 aria-label
  const seo = read("modules/console/web/src/modules/seo/SeoSettings.vue");
  assert.match(seo, /<VSwitch v-model="cfg\.push\.enabled" aria-label="站点推送" \/>/);
  const modules = read("modules/console/web/src/modules/modules/ModulesView.vue");
  assert.match(modules, /:aria-label="`\${m\.title \|\| m\.id} 开关`"/);
  const post = read("modules/console/web/src/modules/posts/EditorSidebar.vue");
  assert.match(post, /<VSwitch v-model="form\.pinned" aria-label="置顶" \/>/);
  const plugins = read("modules/console/web/src/modules/settings/PluginsTab.vue");
  assert.match(plugins, /aria-label="封面文字自适应颜色"/);
  assert.match(plugins, /aria-label="卡片 Hover 光斑与倾斜"/);
});

test("VTabbar 具备 tablist 语义、roving tabindex 和方向键焦点轮转", () => {
  const vTabbar = read("packages/components/src/ui/VTabbar.vue");
  assert.match(vTabbar, /role="tablist"/);
  assert.match(vTabbar, /role="tab"/);
  assert.match(vTabbar, /:aria-selected="item\.id === current"/);
  assert.match(vTabbar, /:aria-controls="`tabpanel-\${item\.id}`"/);
  assert.match(vTabbar, /:tabindex="item\.id === current \? 0 : -1"/);
  assert.match(vTabbar, /e\.key === "ArrowRight"/);
  assert.match(vTabbar, /e\.key === "ArrowLeft"/);
  assert.match(vTabbar, /buttonRefs\.value\[targetIndex\]\?\.focus\(\)/);
  assert.match(vTabbar, /\.tabbar-item:focus-visible/);
});

test("DefaultLayout 导航使用原生 router-link/button、动态移动列数与带关闭和焦点管理的抽屉", () => {
  const layout = read("modules/console/web/src/layouts/DefaultLayout.vue");
  // 原生链接与按钮
  assert.match(layout, /<router-link[\s\S]*?:to="item\.path"[\s\S]*?class="menu-item-title"/);
  assert.match(layout, /<button[\s\S]*?aria-label="搜索"[\s\S]*?@click="openSearch"/);
  assert.match(layout, /<button[\s\S]*?aria-label="未读通知"/);
  // 移动底栏动态列与安全区
  assert.match(layout, /gridTemplateColumns:\s*`repeat\(\$\{minimenus\.length \+ 1\}, minmax\(0, 1fr\)\)`/);
  assert.match(layout, /paddingBottom:\s*'env\(safe-area-inset-bottom, 0px\)'/);
  // 移动抽屉
  assert.match(layout, /role="dialog"/);
  assert.match(layout, /aria-modal="true"/);
  assert.match(layout, /aria-labelledby="mobile-more-drawer-title"/);
  assert.match(layout, /aria-label="关闭抽屉"/);
  assert.match(layout, /onMoreDrawerKeydown/);
  assert.match(layout, /drawerCloseButtonRef/);
  assert.match(layout, /moreButtonRef\.value\?\.focus\(\)/);
});

test("Dashboard 拖拽提供键盘等价移动按钮且图表提供可访问数据摘要", () => {
  const grid = read("modules/console/web/src/modules/dashboard/DashboardGrid.vue");
  assert.match(grid, /class="apple-edit-btn-move"/);
  assert.match(grid, /title="向前移动"/);
  assert.match(grid, /title="向后移动"/);
  assert.match(grid, /:disabled="index === 0"/);
  assert.match(grid, /:disabled="index === widgets\.length - 1"/);

  const trend = read("modules/console/web/src/modules/dashboard/widgets/TrendChartWidget.vue");
  assert.match(trend, /class="sr-only"/);
  assert.match(trend, /访问趋势数据明细/);

  const hourly = read("modules/console/web/src/modules/dashboard/widgets/HourlyChartWidget.vue");
  assert.match(hourly, /class="sr-only"/);
  assert.match(hourly, /24 小时访问时段分布明细/);

  const donut = read("modules/console/web/src/modules/dashboard/widgets/CategoryDonutWidget.vue");
  assert.match(donut, /class="sr-only"/);
  assert.match(donut, /文章分类分布明细/);
});
