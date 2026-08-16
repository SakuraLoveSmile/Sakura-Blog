# Spec 69 · 浏览器批注清理（前台 + 控制台 10 项）

- **分支**：`feature/annotation-fixes` · **Wave 1**
- **主模块**：
  - 前台主题：`backend/internal/template/partial/`、`backend/internal/template/model_style.go`、`frontend/assets/js/`、`frontend/assets/css/`
  - 控制台：`frontend/console/src/modules/dashboard/`、`frontend/console/src/modules/posts/`
  - 后端（仅 C3 恢复专栏涉及）：`backend/internal/store/topic.go`、`backend/internal/api/topic.go`、前台 topic 路由/模板
- **前置**：评论 Boost、设置页复刻 Stellar 配置（用户已另行完成；本 spec 与两者文件面不交叉，合并顺序无依赖）

## Why

用户在浏览器审查时留下 10 条批注，覆盖前台主题观感（左栏模式、轮播箭头、专栏页、已验证徽章）与控制台体验（仪表盘、文章列表、编辑器侧栏）。逐项落地为可独立验收的子任务。

## What Changes

### 前台主题组

- C1 左栏纯色/光晕开关：前台加访客可切换的持久化开关（localStorage + head 内联脚本防闪烁，参照主题持久化既有模式），切换 `ui-style: card | glass`；设置页 AppearanceTab 下拉保留为默认值来源，消费点 `leftbarCSS()`
- C2 置顶轮播箭头消失：对照 `resource/hexo-theme-stellar/layout/` 轮播 ejs 与 main.css `.pin-slider` 选择器定位箭头缺失根因（class 缺失或模板未输出），按「对照原版先行」修复，不自创样式
- C3 恢复专栏页：参照 https://xaoxuu.com/topic/ 恢复 `/topic` 专栏列表——路由、模板 partial（对齐 Stellar topic 结构）、topics 数据源（优先复用 `store/topic.go`/`api/topic.go` 现状）、菜单项走 menubar 设置添加；参照截图待开工时截取存入 `baselines/`
- C4 已验证徽章：参照 Artalk `.atk-verified-icon`（小图标、无文字）替换 `comments.js` renderBadges 与 comments.html 编辑器内 verified 徽章（保留博主徽章），bump 版本戳

### 控制台组

- C5 状态灯含义：RecentPostsWidget.vue `apple-status-dot` 加 title（已发布/草稿）或改文字徽章
- C6 假曲线：StatCardWidget.vue sparkline 先核实数据源（`store/stats.go`/`store/pageview.go` 可否按日聚合真数据），二选一：接真数据 或 移除 sparkline 换真实可得指标，不伪造
- C7 视图切换去文字：DashboardView.vue `apple-segmented.view-switch` 隐藏 `.seg-label`，保留图标 + title
- C8 窄屏头像比例：PostList.vue 头像容器固定宽高 + `flex-shrink:0` + `object-fit:cover`，375/480/768 断点验证
- C9 作者选择头像式：EditorSidebar.vue 下拉改为头像行（已选作者头像 +「+」按钮），hover 头像 tooltip 显示基本信息，点头像可移除，数据源复用 authors API
- C10 标签可取消：EditorSidebar.vue 候选标签按钮改 toggle（再次点击取消选中态），与逗号分隔 input 双向同步

## UI 验收边界

- **参照图路径**：C3 为 `baselines/xaoxuu-topic.png`（唯一参照：https://xaoxuu.com/topic/；参照截图待开工时截取存入 `baselines/`；入库前 C3 不动工）；其余项以批注原文描述为判据
- **验收清单**：
  - [ ] 左栏有模式开关，切换纯色/光晕即时生效，刷新后保持
  - [ ] 置顶轮播切换按钮箭头正常显示，深浅色下可见
  - [ ] `/topic` 专栏列表页可访问，卡片结构对齐参照截图
  - [ ] 评论已验证徽章为小图标样式，博主徽章不受影响，深浅色正常
  - [ ] 仪表盘状态灯有明确含义（title 或文字），无歧义
  - [ ] 统计卡不再显示伪造曲线（真实数据或新样式）
  - [ ] 视图切换按钮仅图标，悬浮有提示
  - [ ] 375/480/768 三档文章列表头像不拉伸不变形
  - [ ] 作者选择为头像式：头像 + 加号，hover 气泡，可添加/移除
  - [ ] 标签候选可再次点击取消，与输入框双向同步
  - [ ] 验收完成后截图存入 `baselines/` 并更新本节勾选状态

## ADDED Requirements

### Requirement: 左栏显示模式访客可切换

The system SHALL 提供前台左栏纯色/光晕模式的运行时开关并持久化选择。

#### Scenario: 访客切换模式

- **WHEN** 访客点击左栏模式开关
- **THEN** 左栏在 card/glass 间切换，选择写入 localStorage，刷新后保持

### Requirement: 专栏列表页恢复

The system SHALL 提供 `/topic` 专栏列表页，结构与 Stellar topic 页对齐。

#### Scenario: 访问专栏页

- **WHEN** 访客访问 `/topic`
- **THEN** 渲染专栏卡片列表，数据来自 topics 数据源，无数据时空态友好

### Requirement: 编辑器标签可取消

The system SHALL 支持在文章编辑器中通过再次点击候选标签取消其选中。

#### Scenario: 取消已选标签

- **WHEN** 用户点击已选中（高亮）的候选标签
- **THEN** 该标签从文章 tags 移除，输入框同步更新

### Requirement: 仪表盘不展示伪造数据

The system SHALL 保证仪表盘统计卡曲线来自真实聚合数据或不展示曲线。

#### Scenario: 无真实序列时

- **WHEN** 统计指标无按日真实序列
- **THEN** 不渲染 sparkline，改为真实可得的简洁指标

## MODIFIED Requirements

（无）

## 验证

1. `make test`（console tests + `go vet` + `go test`）全绿
2. 「UI 验收边界」清单逐项对照截图勾选全绿（C3 对照 baselines/xaoxuu-topic.png）
3. 前台改动浏览器验收（8081）：深浅色 + 375/480/768 视口截图，控制台零错误
4. `make merge-check` 全绿后合入 dev

## Out of Scope

- 评论 Boost（linux.do 简短回应）——用户已完成
- 设置页复刻 Stellar `_config.yml` 缺口补齐——用户已完成
- 评论区平铺/树形切换、举报功能
- 评论系统外部服务（artalk/waline 等）接入
