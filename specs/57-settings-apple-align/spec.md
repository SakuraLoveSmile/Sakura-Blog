# Spec 57 · 设置页 Apple 设计对齐（P0 批）

- **分支**：`dev`（直接工作；提交走 `make pre-commit-check` 门禁）
- **主模块**：`frontend/console/src/modules/settings/SettingsView.vue`、`frontend/console/src/styles/apple-base.css`、`frontend/console/src/components/ui/VCard.vue`
- **前置**：无

## Why

设置页多处样式偏离 `.design_library/苹果/`（Pinguo Apple）规范：一级分组切换栏激活态在窄屏渲染成断裂的蓝色「括号」下边框、桌面竖排为 Halo 风左竖条；输入框/嵌套组与卡片底同色（均 `#1C1C1E`）导致框层次粘连；可编辑行间距仅 8px、大量 off-scale 间距值（14/18/28）、分隔线过密。用户反馈「切换栏位丑」「框之间没有间距」。本 spec 只修 P0 批，P1/P2 另开子任务。

## What Changes

### 前端

- `SettingsView.vue` 一级导航：移除 `border-left` 左竖条与窄屏 `border-bottom` + 负边距 hack，激活态统一为圆角填充块（桌面竖排 rounded-10 填充、窄屏横排 pill 填充）；窄屏 item 触达高度 ≥44px。
- `SettingsView.vue` 间距归位到设计库 scale（4/8/12/16/24/32）：容器 28→24、内容列 18→16、field/form-grid 14→16/12、subsection 18→24 且去分隔线、card-actions 去分隔线 margin 24、editable 行距/行内距 8→12、nested-group 底色加深与卡片拉开层次、friends-subsection 22→24。
- `apple-base.css`：`.apple-input/.apple-textarea/.apple-select/.apple-search` 背景由 `var(--color-secondary)`（与卡片同色）改为更深一档混色，建立 tonal 层次。
- `VCard.vue`：header/body 水平 padding 18→16（归位 scale）。

## UI 验收边界

> 载体约定见根目录 [`AGENTS.md`](../../AGENTS.md)「UI 类任务开工约定」。

- **参照（唯一）**：`.design_library/苹果/preview/component-navigation.html`（导航形态：pill/圆角填充、无下划线指示器）、`.design_library/苹果/colors_and_type.css` 与 `frontend/console/src/styles/main.css`（token/scale）。中途不得更换参照。
- **基线截图**：`baselines/before-mobile-site.png`、`baselines/before-mobile-footer.png`、`baselines/before-desktop-site.png`、`baselines/before-desktop-footer.png`。
- **验收清单**（每项一个可观察判据，完成时逐项勾选）：
  - [x] 窄屏（≤860px）一级切换栏 active 为 pill 填充块，无蓝色弧形/括号残影，无负边距错位（实测 radius 9999px、border 0、高 44px）
  - [x] 桌面一级导航 active 无 3px 左竖条，仅圆角填充 + 主色文字（实测 radius 10px、border-left 0）
  - [x] 窄屏一级导航 item 高度 ≥44px（实测 44px）
  - [x] 输入框/文本域底色明显深于卡片底 `#1C1C1E`，嵌套组与卡片有 tonal 层次（实测 input ≈ rgb(20,20,21) < nested ≈ rgb(22,22,24) < card rgb(28,28,30)）
  - [x] 社交图标/右栏链接可编辑行行距 ≥12px、行内框距 ≥12px，框不相贴（实测 12/12）
  - [x] 设置页间距值落在 scale：容器 24、内容列 16（桌面）/12（移动）、field 16/12、subsection 24、card-actions 24（实测一致）
  - [x] 卡片内 subsection 与 card-actions 无横向分隔线（switch-row 分隔线保留）（实测 border-top 0）
  - [x] 验收完成后 after 截图存入 `baselines/` 并更新本节勾选状态

## ADDED Requirements

### Requirement: 设置页 P0 样式对齐 Pinguo Apple

The system SHALL 使设置页一级导航激活态、嵌套面层次与间距节奏符合 Pinguo Apple 设计库（pill/圆角填充指示器、tonal 分层、4/8/12/16/24/32 scale、留白优先于分隔线）。

#### Scenario: 窄屏一级切换栏
- **WHEN** 视口 ≤860px 打开 `/console/settings`
- **THEN** 激活分组显示为 pill 填充块，无下边框弧形残影；item 高度 ≥44px

#### Scenario: 桌面一级导航
- **WHEN** 视口 >860px 打开 `/console/settings`
- **THEN** 激活分组为圆角填充块 + 主色文字，无左竖条

#### Scenario: 表单面层次与间距
- **WHEN** 打开「页脚」tab 查看社交图标编辑区
- **THEN** 输入框底色深于卡片底；行距/行内距 ≥12px；subsection/card-actions 无分隔线

## 验证

1. `make test` 全绿（console-test 含 vue-tsc）
2. 「UI 验收边界」清单逐项对照 baselines/ 勾选全绿
3. 提交前 `make pre-commit-check` 全绿

## Out of Scope

- P1：VCard 静置阴影/顶部高光线、输入框 40/44 高、row-btn/icon-btn 触达、segmented active 对比
- P2：AppearanceTab 自造按钮类、code-style 裸色值、移动端 !important 覆盖、页头 padding 冲突
