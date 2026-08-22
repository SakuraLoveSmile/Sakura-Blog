# 仪表盘组件样式微调 — 基线

本目录为「仪表盘组件样式微调」任务的验收基线载体（入库，供回归对照）。

## 基线说明

### before 状态

- 采集时间：2026-08-22（dev 提交 `a0b4788`，即尺寸档位体系落库后的状态）
- 采集环境：dev worktree `./blog -dev`（:8080），admin 登录态，console 仪表盘
- `before-desktop.png`：仪表盘默认视图 @ 1440×900（电脑档）
- `before-tablet.png`：仪表盘默认视图 @ 768×1024（平板档）
- `before-mobile.png`：仪表盘默认视图 @ 390×844（手机档）
- `before-designer-versions.png`：设计器视图 @ 1440×900，临时加入「Blog 版本」「访问时段分布」
  组件（未保存布局）后的状态，用于补齐默认布局中不含的组件视觉对照

### after 状态

- 采集时间：2026-08-22，feature 分支 vite dev（:5173，代理 :8080 API），admin 登录态
- `after-desktop.png` / `after-tablet.png` / `after-mobile.png`：同角度三档断点
- `after-designer-versions.png`：设计器同流程（临时加入 Blog 版本 + 访问时段分布，未保存）

## 验收清单（固定，after 核对结果）

约束：永久深色模式；只消费语义 token（`--color-*` / `--radius-*` / `--shadow-*` / `--duration-*` / `--ease-standard`），
不新造裸色值；不改组件布局结构与交互逻辑，只做密度、留白、对齐、配色层微调。

- [x] A1 统计卡片 Medium 重心修复：图标+标签同行（icon 36px 盒 y≈159、label 同排 y≈169），
      数值区 y=211 固定 gap 16px 自然排布，不再 justify-between 撑开（DOM 几何实测，feature 4821b9d）
- [x] A2 Large 装饰条（5 根假走势柱 + opacity-40）已移除，留白干净
- [x] B1 版本卡样式去重：两组件约 150 行同构 scoped CSS 删除，共享 `.version-card-*` 定义于
      dashboard.css；Small 状态徽标同步迁移（git diff：-376/+233 行）
- [x] B2 `.version-card-values` margin 22px 0 18px → 16px 0（getComputedStyle 实测 16px）
- [x] C1 快捷操作三尺寸统一 `.quick-action-item`（color-mix #fff 3%/5%，hover 6%/8%），
      全文件无 `bg-white/[` 硬编码（grep 实测；getComputedStyle = color(srgb 1 1 1 / 0.03)）
- [x] C2 Large 移除 `hover:-translate-y-0.5`，hover 不再与全局 `button:active scale(0.97)` 叠加
      （transform 单一语义：仅 active 缩放）
- [x] D1 访问趋势卡无 emerald/amber/#30D158 硬编码（grep 实测），UV 统一 `--color-chart-1`
      （数值/图例点/虚线/ tooltip UV 点），峰值 `--color-chart-3`
- [x] D2 分类分布 8 色无重复（COLORS 第 7 项 primaryHover → chart-4 #5E5CE6，grep 实测）；
      小尺寸中心文案 y 82/102 → 85/105 与默认态对齐，hover 切换不抖动
- [x] D3 访问时段高亮改为真实数据最高 4 个非零时段（`topValues` 计算，grep 无硬编码时段）；
      顶部峰值胶囊 16:00-17:00 与 API 数据一致；Large 分析胶囊改 `bg-secondary/40 border-white/10`
      与 apple-card 边框语言协调
- [x] E2 三档断点（1440/768/390）横向溢出 0px、组件位置无错位（DOM 几何实测 scrollWidth == innerWidth）
- [x] E3 `make test` 全绿：console-test 86/86 + assets-test + deploy-test + module-check +
      go vet + staticcheck + go test（EXIT=0；stellar-version-widget.test.ts 断言随去重迁移到 dashboard.css）
- [ ] E4 `make merge-check`（feature worktree 内执行，测试门槛 + 合入 dev + 组合态复验）——待执行

## 备注

- 本改动附带的最小测试更新：`modules/console/tests/stellar-version-widget.test.ts` 语义 token 断言
  从组件 scoped（已删）迁至 dashboard.css 共享类（断言意图不变：版本卡暗色 UI 走语义 token、无 6 位十六进制裸色）。
- 基线/验收图中「设计器临时加入组件」均未点击保存布局，服务端布局未改动。