# Console UI 一致性设计参照基准 (Design Reference)

本基线记录《Console UI 一致性修复计划》的固定设计裁决、视觉参照图与验收边界。

## 1. 唯一参照图

- **设计基线路径**：`docs/baselines/console-ui-consistency/design-reference.png`
- **涵盖区域**：
  1. **桌面列表与卡片**：1400px 内容容器、标题 24px/600、副标题 13px、胶囊主按钮（黑色文字高对比无辉光）、紧凑 80px 概览卡片（蓝/中性身份色区分）。
  2. **手机操作区与底栏**：390px 视口、44px 最小触控区域、搜索框与按钮自适应换行流式布局、底栏仅在窄屏出现。
  3. **设置分区与 Tabs**：1100px 容器、`VTabbar(type="segmented")` 顶部单行滑动、单一卡片边界、无内外重复卡片与标题。
  4. **弹窗与控件阶梯**：标准模态弹窗、2px 焦点环 (`:focus-visible`)、按钮尺寸阶梯 (xs 24 / sm 28 / md 36 / lg 40px)。

## 2. 固定核心设计规范

| 项目 | 固定规则 |
|---|---|
| 页面背景 | `--color-background`（黑色 `#000000`） |
| 卡片 | `--color-card`（深灰 `#1c1c1e`）；边框 `1px solid var(--color-border)` (`#3a3a3c`) |
| 主操作 | 背景 `--color-primary`（`#2e8dff`）；文字和图标使用 `--color-primary-foreground`（`#000000`） |
| 主操作交互 | 悬停 primary-hover，按下 primary-active；严禁添加蓝色外发光 |
| 状态色 | 成功绿 `#30d158`、警告橙 `#ff9f0a`、错误红 `#ff453a`；管理员蓝色、作者中性色，不用红色表达身份 |
| 按钮 | 文字按钮胶囊形状，圆角在尺寸之后生效；桌面高度 xs 24 / sm 28 / md 36 / lg 40px；手机点击区域 ≥44px |
| 焦点 | `:focus-visible` 2px ring，2px offset |
| 概览卡片 | 桌面 80px / 手机 88px 紧凑最小高度，桌面 4 列 / 手机 2 列 |
| 设置 Tabs | 统一复用 `VTabbar(type="segmented")`，单行横向滚动，深灰背景激活项配合蓝色图标 |
