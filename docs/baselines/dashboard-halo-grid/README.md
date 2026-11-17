# 线上仪表盘「全组件」与 Halo 借鉴实现（BLG-20261110-05）验收说明

- 任务编号：`BLG-20261110-05`（plan：`docs/plans/BLG-20261110-05-dashboard-halo-grid.md`）
- 状态：待体验
- 上游参照：Halo `v2.27.2`（`4f9f661976bb2f989317035e05d217854108dcc5`），**只参照交互思路，未引入源码、依赖或样式**

## 唯一参照

- 布局与尺寸真值仍是本项目自己的统一足迹表（`WIDGET_PRESET_FOOTPRINTS`），本次未改动。
- 视觉规范：`resource/apple-design/` 的深色语义 token；字号/圆角/色彩受
  `modules/console/tests/ui-restraint.test.ts` 白名单约束，本次**未新增任何白名单豁免**。
- Halo 侧参照面：`ui/console-src/modules/dashboard/widgets/index.ts`（每实例 `config` + `defaultSize`）、
  `DashboardDesigner.vue`（设计器与 `availableWidgetDefinitions` provide 模式）、
  `widgets/presets/core/stack/StackWidget.vue`（叠放容器的翻页与轮播）。

## 本次借鉴的两条机制

| Halo 机制 | 本项目实现 |
| --- | --- |
| 每个 widget 实例带自己的 `config` | `WidgetConfigField` 描述 + `WidgetConfigModal.vue` 编辑 + `resolveWidgetConfig` 收敛；沿用已有的 `layout.ts` config 链路与后端 `dashboard_layout.go` 的 `config` 字段（后端本来就能存取） |
| `core:stack` 叠放容器 | `StackWidget.vue`：一块网格位置承载多个子组件，可手动翻页或按间隔轮播 |

## 文件

| 文件 | 说明 |
| --- | --- |
| `stack-first.png` | 叠放容器初始态（1/3，最近文章） |
| `stack-second.png` | 点击第二个页签后（2/3，最近评论），内容与高亮同步 |
| `config-dialog.png` | 「组件设置」弹窗：容器内组件多选 + 自动轮播间隔 |
| `browser-verification.json` | 真实 Chrome 量测：页签、切换结果、lg 断点溢出为 0、console 错误为 0 |

## 逐项验收

- [x] 组件设置入口只对声明了 `configFields` 的组件出现（统计卡没有该按钮，尺寸按钮仍在）。
- [x] 同类型两个实例的配置互相独立：实例 A 保存 `{interval:"8", stackWidgets:["recent-posts","recent-comments"]}`，
      实例 B 保存 `{interval:"0", stackWidgets:["top-posts"]}`，回读一致。
- [x] 叠放容器在只读页可切换子组件，切换后内容、计数器与页签高亮同步。
- [x] 只有一个子组件时不渲染页签栏与 `1 / 1` 计数器。
- [x] 叠放容器候选项不含自身，避免无限嵌套（单测断言）。
- [x] 三档尺寸体系未被破坏：既有 `dashboard-widget-size` 用例（144 组量测）通过。
- [x] Console UI 一致性 37 项通过，含尺寸弹窗三档断言与字号白名单。
- [x] `ui-restraint.test.ts` 全部守卫通过，无新增豁免。

## 验证

- `make test`：通过（Console 576 项、Stellar 229 项、Go 全部包，退出码 0）。
- `make build`：通过（真实 Console/UC 产物 + Go 二进制）。
- 新增 e2e `tests/e2e/dashboard-widget-config.spec.ts`：3 项通过。
- 既有 e2e：`dashboard-widget-size.spec.ts` 1 项、`console-ui-consistency.spec.ts` 37 项，全部通过。
- 新增单测 `modules/console/tests/widget-config.test.ts`：配置默认值/类型收敛/叠放容器约束等 12 项。

## 未验证 / 边界

- **本机未安装 Playwright 自带 Chromium**，e2e 用系统 Chrome（`channel: "chrome"`）经临时配置 `.tmp/pw-local.config.ts` 运行；
  仓库 `playwright.config.ts` 未修改，CI 仍走自带 Chromium。
- 叠放容器**按断点独立存储**：在设计器里添加组件只写入当前断点（v10 实例模型的既有行为，非本次引入），
  因此在 lg 添加的叠放容器在平板/手机断点看不到，需要切到对应断点再添加。
- 未在真实移动设备或真机视口验证；`sm`/`xxs` 是 Console 自带的设备预览。
- 未提交、未推送、未发版；生产环境未变更。
