# 仪表盘组件样式微调 — 基线

本目录为「仪表盘组件样式微调」任务的验收基线载体（入库，供回归对照）。

## 基线说明（before 状态）

- 采集时间：2026-08-22（dev 提交 `a0b4788`，即尺寸档位体系落库后的状态）
- 采集环境：dev worktree `./blog -dev`（:8080），admin 登录态，console 仪表盘
- `before-desktop.png`：仪表盘默认视图 @ 1440×900（电脑档）
- `before-tablet.png`：仪表盘默认视图 @ 768×1024（平板档）
- `before-mobile.png`：仪表盘默认视图 @ 390×844（手机档）
- `before-designer-versions.png`：设计器视图 @ 1440×900，临时加入「Blog 版本」「访问时段分布」
  组件（未保存布局）后的状态，用于补齐默认布局中不含的组件视觉对照

改动完成后同角度截图 `after-*.png` 逐项对照下方验收清单。

## 验收清单（固定）

约束：永久深色模式；只消费语义 token（`--color-*` / `--radius-*` / `--shadow-*` / `--duration-*` / `--ease-standard`），
不新造裸色值；不改组件布局结构与交互逻辑，只做密度、留白、对齐、配色层微调。

- [ ] A1 统计卡片 Medium 尺寸不再出现「图标独居顶部、下半截空旷」的失衡感，数值区视觉重心稳定
- [ ] A2 Large 尺寸装饰条（假走势柱）存在感降低或移除，留白干净
- [ ] B1 Stellar 版本卡与 Blog 版本卡样式代码不再各自重复约 130 行 scoped CSS，视觉保持一致
- [ ] B2 `.version-card-values` 上下间距收敛（约 16px），节奏协调
- [ ] C1 快捷操作三种尺寸表面色（底色/边框/hover）统一为同一套语义化写法，无 `bg-white/[0.03]` 类硬编码
- [ ] C2 Large 尺寸 hover 位移与全局 active 缩放不再叠加冲突（单一 transform 语义）
- [ ] D1 访问趋势卡无 `text-emerald-400` / `text-amber-400` / `bg-emerald-500` / `stroke="#30D158"`
      硬编码颜色，UV 相关颜色统一走 `--color-chart-1`
- [ ] D2 分类分布图 8 色序列无重复色，Hover 切换中心文案不抖动（y 坐标对齐）
- [ ] D3 访问时段卡高亮柱与实际数据峰值一致（不再硬编码 10-12/20-22 点）
- [ ] E2 三档断点（电脑 1440 / 平板 768 / 手机 390）下改动后组件无溢出、无裁切、无错位
- [ ] E3 `make test` 全绿（console 测试 + go vet + staticcheck + go test）
- [ ] E4 feature worktree 内 `make merge-check` 成功合回 dev（合并后 dev 组合态复验通过）