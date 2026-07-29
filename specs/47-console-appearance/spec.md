# Spec 47 · Console 外观设置（style/widgets 可视化编辑）

- **分支**：`feature/console-appearance` · **Wave 9**（依赖 34/33 先合）· **隔离等级**：中
- **主模块**：`console/src/modules/settings/`（+外观 Tab 组件）+ `internal/api/settings.go`（+appearance 分段）
- **前置**：**Spec 34（style 配置段）与 Spec 33（widgets 配置段）必须先合**——本 spec 是它们的 Console 可视化层

## Why

Halo「主题设置」的本项目等价物：34/33 落地后外观与组件编排能力存在但只能改 site.yml 文件。把高频项做进 Console 设置页，即达到 Halo「后台改外观即时生效」的体验闭环；同时这是差距清单「主题管理」项的正式处置——**不做多主题安装/切换**（单主题内置是本项目架构前提），只做主题设置面板。

## 功能需求

### A. 外观 Tab（消费 Spec 34 的 style 段）
| 区 | 控件 |
|---|---|
| 主题色 | theme/accent/link 取色器 + 实时预览色块 |
| 深浅模式 | prefers_theme 三选（auto/light/dark） |
| 字体字号 | root/body 字号滑杆；font-family 文本框（高级） |
| 背景 | leftbar 背景（纯色/渐变/图片三态 + blur 滑杆）、全站背景图（附件选择器） |
| 圆角 | 五档滑杆组 |
| 代码块 | 高亮主题下拉（内置候选 URL 列表）+ 自定义 URL |
| 杂项 | 标题前缀开关、404 插图（附件选择器）、loading 文案 |

### B. 组件编排 Tab（消费 Spec 33 的 widgets 段）
- 页面类型下拉 → 左右栏组件序列的拖拽排序编辑（交互复用 MenubarEditor 拖拽模式）+ 从组件库添加/移除 + 组件参数表单（按 layout 类型渲染字段）。
- 「恢复默认」按钮（清除该页面类型自定义序列，回退缺省布局）。

## What Changes

- `internal/config/reload.go` + `internal/api/settings.go`：style/widgets 段入 settings 读写路径（34/33 落地时若仅文件配置，此处补 DB 化 + 热更新；已 DB 化则只加分段端点）。
- `console/src/modules/settings/`：+`AppearanceTab.vue`、`WidgetsTab.vue`（拖拽复用 MenubarEditor 模式）；SettingsView 注册 2 Tab。
- 预览：外观改动保存后 toast 附「查看前台」链接（不做 iframe 实时预览，见技术决策）。

## 实现步骤

1. settings 分段端点 + 热更新验证（改主题色 → 前台刷新即变）。
2. AppearanceTab 分区逐个落（主题色/模式先行）。
3. WidgetsTab 拖拽编排 + 参数表单。
4. 恢复默认与空态。

## 技术决策（无需再问）

- **多主题系统不做**：主题即 Stellar 单内置（模板/CSS 走 go:embed，运行时换主题与单二进制架构冲突）。此为差距清单「主题管理」项的正式处置。
- **不做 iframe 实时预览**：外观项保存即热更新，前台开新标签即见；实时预览的双向状态同步复杂度不值。
- **高亮主题内置候选**：chroma 支持的主题列出 6-8 个常用项，自定义 URL 兜底。
- **widgets 参数表单按 layout 白名单渲染**：未知 layout 显示 JSON 编辑兜底（对齐 settings 现有「高级 JSON」模式）。

## Impact

- **新增**：AppearanceTab.vue、WidgetsTab.vue、settings appearance/widgets 分段
- **修改**：SettingsView.vue、api/settings.go、reload.go
- **不影响**：前台渲染逻辑（消费 34/33 已有机制）、其他模块

## 验证

- [ ] 改主题色/模式/背景保存后前台刷新生效（亮暗双轨）
- [ ] 组件编排拖拽 + 参数编辑 + 恢复默认全流程，前台布局随动
- [ ] 全部留空时前台与 34/33 缺省一致
- [ ] settings 其他 Tab 不回归
- [ ] `make test` 全绿
