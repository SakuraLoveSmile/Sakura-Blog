# 访问趋势小组件展示优化 — 基线

任务 BLG-20261109-06。本目录为「访问趋势」（`TrendChartWidget.vue`）展示样式优化的验收基线，入库供回归对照。

## 采集环境

- 隔离服务 `127.0.0.1:8171`，独立 root `.tmp/trend-baseline`（DB/config/content 全隔离，符号链接 `modules`/`internal`）。
- 数据库为 e2e 隔离夹具 + 自建 `page_views` 形状数据：13 个有访问日、前 17 天补零，
  峰值 133 PV（10/29）、有周末抬升与两次尖峰，用于同时验证 7 天与 30 天窗口、补零段与峰值形态。
- 采集于 feature 工作树，`VITE_E2E=1 pnpm --filter console run build` 的真实 dist，
  服务在该产物上启动；未连接生产库、未触碰根工作树用户改动。

## 截图对照

| 场景 | before | after |
| --- | --- | --- |
| 1280 默认（7 天） | `before-trend-1280.png` | `after-trend-1280.png` |
| 768 默认（7 天） | `before-trend-768.png` | `after-trend-768.png` |
| 390 默认（7 天） | `before-trend-390.png` | `after-trend-390.png` |
| 390 卡片完整可见 | — | `after-trend-390-view.png` |
| 1280 30 天 | `before-trend-1280-30d.png` | `after-trend-1280-30d.png` |
| 390 30 天 | `before-trend-390-30d.png` | `after-trend-390-30d.png` |
| 1280 悬停提示 | `before-trend-1280-hover.png` | `after-trend-1280-hover.png` |
| 390 悬停提示 | — | `after-trend-390-hover.png` |
| 全零空态 | （旧版画平线 + 全 0 数字） | `empty-trend.png` |
| large 档位 | — | `size-large.png` |
| small 档位 | — | `size-small.png` |

`before-trend-390*.png` 中卡片被移动端底栏覆盖的部分属于截图取景，不是布局缺陷；
`after-trend-390-view.png` 为隐藏底栏后的卡片全貌。

## 改动前进查出的问题

1. **指标与图表抢空间**：3–4 个指标各自「标签 + 20px 数字」，在 390 下占掉三行，
   图表只剩约 105px，横轴标签 `11/3`、`11/9` 被卡片边缘裁掉。
2. **实时刻度错位**：横轴标签是绝对定位的 `<span>`，首尾未处理锚点，30 天视图首尾日期被切半个字。
3. **折线端点贴边**：绘图区无内缩，末点记号与峰谷描边压在卡片边界上被裁。
4. **全零不算空态**：接口对无数据日补零，`data.length` 恒大于 0，全零时仍画一条平线并显示一排 0。
5. **日均口径失真**：`pv / 天数` 把无访问的空档也算进分母，停更站点读数偏低。
6. **视觉同质**：卡内以留白与细线分层不足，30 天密集数据没有参照基线，读不出「现在算高还是低」。

## 验收清单

- [x] A1 桌面事实行改为「小标签 + 数值 + 单位」，一行 5 组（large）/ 4 组（medium），不再各占独立小卡。
- [x] A2 移动端事实行 `2×2` 紧凑行内排版，标签回到数值左侧；`xxs` 隐藏与标题重复的副标题。
- [x] A3 横轴标签移回 HTML 层（SVG 用 `preserveAspectRatio="none"` 会压扁内部文字），首尾标签分别用
      `is-first`/`is-last` 改锚点；30 天按 6 个刻度抽稀，7 天全显。
- [x] A4 绘图区内缩 5px，末点记号与峰谷描边不再被卡片边缘裁切。
- [x] A5 峰值日 + 「活跃天数 / 起止变化百分比」替换原先孤立的峰值数字，给出窗口内的走势结论。
- [x] A6 日均访问改按「有访问的日子」为分母，并在事实行标注为「活跃日」口径。
- [x] A7 全零序列走空态，显示「近 N 天暂无访问」，不再画平线。
- [x] A8 只消费语义 token（`--color-primary*` / `--color-chart-1` / `--color-chart-grid` /
      `--color-text-*` / `--color-border` / `--radius-*` / `--duration-*` / `--ease-standard`），无新增裸色值与字号档。

## 实测结果

浏览器（Playwright，真实 dist，`pageerror`/`console.error` 为 0）：

| 视口 | 断点 | 卡片高 | 图表高 | 横轴在卡内 | 内容溢出 | 页面横向溢出 |
| --- | --- | --- | --- | --- | --- | --- |
| 1440×900 | lg | 332 | 150 | 是 | 0 | 0 |
| 1280×900 | lg | 332 | 150 | 是 | 0 | 0 |
| 768×1024 | sm | 332 | 139 | 是 | 0 | 0 |
| 390×844 | xxs | 316 | 128 | 是 | 0 | 0 |

- 7 天刻度：`11/3 … 11/9` 全显；30 天刻度：`10/11, 10/17, 10/23, 10/28, 11/3, 11/9`。
- 悬停：浮层出现在指针上方，读数与该日 PV/UV 一致（`10/31 PV 93 / UV 80`）。
- 图例：medium 为可切换按钮（`aria-pressed` 跟随），large 为静态 UV 标签（保持既有语义）。
- 全零数据：进入空态，无残留数字与平线。

机械门禁：`make test` EXIT=0（含 console 类型检查与行为测试、assets、deploy、module-check、
`go vet`、Staticcheck、`go test ./...`）；定向 `ui-restraint` + `components-a11y` +
`widget-registry` 33/33 通过。

## 已知边界

- 仅验证 Chrome；未验证 Safari/Firefox 与真机触屏。
- small 档位截图取自设计器临时切换（未保存布局）；默认 lg 布局本身就是 12×44，识别为 large 档，
  这是既有布局语义，本次未改。
- 设计器编辑态下，small 档右上角的编辑工具条会盖住标题行右侧的 PV/UV 摘要；
  该工具条是设计器既有浮层，默认视图不出现，本次未改其定位。
