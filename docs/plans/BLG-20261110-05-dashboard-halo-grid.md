# BLG-20261110-05：仪表盘借鉴 Halo「实例配置 + 叠放容器」

> 历史实现/验收记录：2026-11-17 提交、合入、发布和工作树收尾续接见 [BLG-20261117-01](BLG-20261117-01-dashboard-integration.md)，当前状态以该记录为准；以下保留原始验收细节。

- 状态：待体验（实现完成、门禁与真实浏览器验收通过，待用户确认观感）
- 日期：2026-11-10（Asia/Taipei）
- 分支：`codex/feat/20261110-BLG-20261110-05-dashboard-halo-grid`
- 工作树：`/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261110-feat-BLG-20261110-05-dashboard-halo-grid`
- 基点：`dev` @ `1af89ac8`
- 合入目标：`dev`
- 上游参照：Halo `v2.27.2`（`4f9f661976bb2f989317035e05d217854108dcc5`），只参照思路，不引入其源码、依赖或样式

## 背景

用户调研「能否直接用 Halo 博客的样式」。结论：不照抄。Halo 仪表盘（`ui/console-src/modules/dashboard/`，3473 行）
只有 6 类组件，没有趋势图/时段/分类环/热门/系统状态；视觉是白卡 + 灰字 + 彩色按钮的浅色形态，
与已验收的「深色朴素工具型」以及 `ui-restraint.test.ts` 守卫冲突；`vue-grid-layout@3.0.0-beta1` 是 2021 年
未转正的 beta，FormKit 要整套引入；`console-src` 沿用仓库 GPLv3，与其 MIT 的单包声明不同。

用户已确认：**只借鉴思路和网格布局机制，样式全部按本地深色规范**。具体落点为两条 Halo 机制。

## 已确认决定

- **保留统一足迹**（`BLG-20261110-01` 已验收、已发布）。不改成自由拖拽缩放，大中小三档语义不变。
- 引入 Halo 的**实例级 `config`**：同一组件类型的多个实例可以有不同配置。
- 引入 Halo 的 **`core:stack` 叠放容器**：一块网格位置轮播多个子组件，解决「组件铺太长」。
- 样式、token、字号、圆角全部沿用 `dashboard.css` 与 `ui-restraint.test.ts` 白名单，不新增裸色值。

## 范围

改：

- `modules/console/web/src/modules/dashboard/widgets/registry.ts`（`configFields` 落地为可用配置项）
- `modules/console/web/src/modules/dashboard/widgets/*.vue`（消费 config：统计卡动画、快捷操作入口、列表条数等）
- 新增 `modules/console/web/src/modules/dashboard/widgets/StackWidget.vue`（叠放容器）
- 新增实例配置弹窗 `WidgetConfigModal.vue`
- `DashboardGrid.vue`、`DashboardDesignerView.vue`、`WidgetSizeModal.vue`（配置入口）
- `dashboard.css`、`modules/console/tests/grid-layout.test.ts`

不改：后端 API/schema（`config` 字段早已存在且已持久化）、Stellar 前台、其他 Console 页面。

## 关键实现约定

- `config` 的读写沿用已有链路：`layout.ts` 的 `normalizeWidget` 会保留 `config`，`mergeGridWidgetMetadata` 在拖拽后
  保留 `config`，后端 `dashboardLayoutWidget.Config map[string]any` 已能存取。本次只补 UI 与消费端。
- 叠放容器用 `provide` 传递子组件定义列表（Halo 的 `provide("availableWidgetDefinitions")` 模式），
  避免 `registry.ts` ↔ `StackWidget.vue` 循环 import。
- 配置弹窗不引入 FormKit；用现有 `SchemaForm` 组件族与 `configFields` 描述，保持依赖不变。

## 验收场景

1. 给「快捷操作」实例配置只显示 2 个入口，刷新后仍生效；同类型的另一个实例不受影响。
2. 把「最近文章 / 最近评论 / 热门文章」放进一个叠放容器，容器内可切换，占用一个网格位置。
3. 三档断点下新组件尺寸仍来自统一足迹表，不破坏既有 e2e 尺寸断言。
4. `ui-restraint.test.ts` 全部守卫通过，无新增白名单豁免。

## 实施结果

- `registry.ts`：`WidgetConfigField` 扩展（选项对象、范围、说明），新增 `stack` 组件定义；
  快捷操作（入口数）、访问趋势（默认区间 / 默认 UV）、三个列表卡（显示条数）各带配置项。
- `widgets/widget-config.ts`（新增）：`configFieldOptions` / `resolveWidgetConfig` / `configInt` 纯函数。
  单独成文件是因为 `registry.ts` 会 import `.vue`，node:test 无法加载。
- `widgets/StackWidget.vue`（新增）：叠放容器，手动翻页 + 可选自动轮播；单子组件时隐藏页签栏。
- `WidgetConfigModal.vue`（新增）：按 `configFields` 渲染多选/开关/下拉/数字/文本，不引入 FormKit。
- `DashboardGrid.vue`：编辑浮层增加「组件设置」按钮（仅对有配置项的组件）；`componentProps` 按需绑定 `config`，
  避免未声明该 prop 的组件把它透传到 DOM（同一标签两个 `v-bind` 会被 Vue 编译器判为重复属性）。
- `layout.ts`：`WIDGET_TYPES` 增加 `stack`。
- `ri-subset.json`：重新生成（新增 `circle-line` 等图标用法）。

## 验证结果

- `make test` 通过（Console 576 / Stellar 229 / Go 全包）。
- `make build` 通过。
- 新增 `tests/e2e/dashboard-widget-config.spec.ts` 3 项通过；既有 `dashboard-widget-size.spec.ts` 1 项、
  `console-ui-consistency.spec.ts` 37 项通过。
- 真实 Chrome 证据见 `docs/baselines/dashboard-halo-grid/`。

## 实现中修掉的两个自身缺陷

1. **select 默认值类型不一致**：`interval` / `entryCount` / `itemLimit` 默认写成数字，而选项 value 是字符串，
   用户不改该项时保存下来的是数字，回读与选项比较失配（e2e 发现）。已统一为字符串，并新增单测锁定该约束。
2. **config 被透传到无该 prop 的组件**：`<component>` 上无条件绑定 `:config` 会让统计卡把它当作 attr 落到根元素。
   改为 `componentProps` 按需合并。

## 遗留与边界

- 叠放容器按断点独立存储，添加后只存在于当前断点（v10 既有行为）。
- 本机无 Playwright 自带 Chromium，e2e 用系统 Chrome 经 `.tmp/pw-local.config.ts` 运行；CI 配置未改。
- 未提交、未推送、未发版。
