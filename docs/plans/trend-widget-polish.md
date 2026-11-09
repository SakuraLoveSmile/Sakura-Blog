# BLG-20261109-06：访问趋势小组件展示样式优化

- 状态：待体验（本地实现与验证完成，未提交、未合并、未发布）
- 分支：`codex/feat/20261109-BLG-20261109-06-trend-widget-polish`
- 工作树：`/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261109-feat-BLG-20261109-06-trend-widget-polish`
- 合入目标：`dev`
- 基线依据：`docs/baselines/trend-widget-polish/README.md`（before/after 截图与逐项验收）

## 目标与范围

只改「访问趋势」小组件（`modules/console/web/src/modules/dashboard/widgets/TrendChartWidget.vue`）的展示层。
保留数据来源、`days` 切换语义、UV 图例在 medium 可切换 / large 静态的既有行为、读屏器数据表格与
`DASHBOARD_REFRESH_EVENT` 刷新链路。不涉及 API、布局规格、其他组件与 Stellar 前台。

## 已确认决定

- 指标不再各占卡内小卡，改为「小标签 + 数值 + 单位」的事实行；移动端回到行内紧凑排版。
- 横轴标签留在 HTML 层：图表 SVG 用 `preserveAspectRatio="none"` 撑满高度，内部 `<text>` 会被等比压扁。
- 绘图区两端内缩 5px，末点与峰谷描边不被卡片边缘裁切。
- 日均访问按「有访问的日子」为分母，并在标签上写明「活跃日」口径。
- 全零序列（接口对无数据日补零）视为空态，不再画平线。

## 改动

| 文件 | 改动 |
| --- | --- |
| `modules/console/web/src/modules/dashboard/widgets/TrendChartWidget.vue` | 重排标题/事实行/图表/结论行；横轴改 DOM 渲染与首尾锚点；绘图区内缩；新增 `hasTraffic` 空态判断；`trendShape` 走势结论；语义 token 收敛 |
| `modules/console/tests/widget-registry.test.ts` | 新增「访问趋势」回归断言：全零空态、横轴在 DOM 层、无 SVG `<text>`、首尾锚点、绘图区内缩 |

## 验证

- `make test`：EXIT=0（Console 类型检查与行为测试、assets、deploy、module-check、`go vet`、Staticcheck、`go test ./...`）。
- 定向门禁：`ui-restraint` + `components-a11y` + `widget-registry` 33/33。
- 新断言在旧组件源码上实测失败（1 fail），证明其有效，随后恢复新实现。
- 真实服务浏览器验收（隔离 root `.tmp/trend-baseline`，:8171，真实 dist）：
  1440/1280/768/390 四个视口内容溢出 0、页面横向溢出 0、横轴均在卡内；
  7 天与 30 天刻度抽稀正确；悬停浮层读数与数据一致；`console.error`/`pageerror` 为 0。
- 空态：清空 `page_views` 后进入「近 7 天暂无访问」，随后已恢复夹具数据。

## 未验证 / 遗留

- 仅 Chrome；未验证 Safari/Firefox 与真机触屏。
- small 档未在默认仪表盘出现（lg 默认 12×44 识别为 large），仅在设计器临时切换截图验证；未保存布局。
- 设计器编辑态 small 档的编辑工具条会遮住标题行右侧摘要，属既有浮层定位，未在本次范围内改。

## 下一步

`make merge-check` 合入 `dev` 并做组合态复验；随后按既有规则清理本任务 worktree 与已合并分支。
