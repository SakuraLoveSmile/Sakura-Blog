# Spec 08 · 阅读体验增强（进度条 + 返回顶部 + 图片灯箱）

- **分支**：`feature/reading-ux` · **Wave 1** · **隔离等级**：高
- **主模块（独占）**：新 `assets/js/reading.js` + `assets/css/reading.css`

## Why

文章页缺少阅读进度条、快捷返回顶部、图片点击放大（灯箱）三项通用阅读增强。全部为前端行为，落在新 js/css + 少量骨架，不碰后端。

## What Changes

- 新增 `assets/js/reading.js`：
  - **阅读进度条**：顶部细条，随滚动百分比更新宽度；仅文章页（`.md-text` 存在）启用。
  - **返回顶部**：滚动超一屏显示按钮，点击平滑回顶。**先确认 Stellar FAB 是否已有回顶**（`head.html` 有 `.fab`）；若已有则本项只做进度条+灯箱，不重复造回顶。
  - **图片灯箱**：点击 `.md-text img` 打开遮罩查看大图（支持 ESC/点击关闭）。**先 grep main.css 是否已含 fancybox/lightbox 类**；若 Stellar 已带则复用其结构，否则用极简自写遮罩。
- 新增 `assets/css/reading.css`：进度条 + 灯箱遮罩样式，**全部用主题变量**（`var(--theme)`/`var(--bg-a*)`），禁止写死色。
- `internal/template/partial/head.html`：`+` reading.css `<link>`。
- `internal/template/layout/base.html`：`+` 进度条元素（`<div id="reading-progress">`）与灯箱容器；`+` `reading.js` `<script>`。绑定走 `DOMContentLoaded` + `pjax:complete`，用 guard 防重复绑定（对齐评论点赞脚本模式）。

## 技术决策（无需再问）

- **零第三方库**：进度条/回顶/灯箱均原生实现，不引 fancybox 等（除非 main.css 已含 fancybox 样式则复用其 DOM 约定）。
- **PJAX 兼容**：`pjax:complete` 重新计算进度、重绑图片点击；window scroll 监听只绑一次（`window.__readingBound`）。
- **性能**：scroll 用 `requestAnimationFrame` 节流。
- **无障碍**：灯箱可 ESC 关闭、focus 管理；`prefers-reduced-motion` 时禁平滑动画。

## Impact

- **新增**：`assets/js/reading.js`、`assets/css/reading.css`
- **修改**：`head.html`（+1 link）、`base.html`（+进度条/灯箱容器 + script）
- **不影响**：Go 代码、store、admin、model

## ADDED Requirements

### Requirement: 阅读增强三件套
The system SHALL provide a scroll progress bar, back-to-top control, and image lightbox on article pages, PJAX-safe.

#### Scenario: 阅读进度
- **WHEN** 在文章页向下滚动
- **THEN** 顶部进度条宽度随阅读百分比增长

#### Scenario: 图片放大
- **WHEN** 点击正文图片
- **THEN** 弹出灯箱查看大图，ESC / 点击遮罩关闭

#### Scenario: PJAX 导航
- **WHEN** PJAX 切换到另一篇文章
- **THEN** 进度条重置、图片灯箱在新内容上生效，无重复绑定

## 验证

1. `go build ./... && go vet ./...` 通过（本 spec 不改 Go）
2. 文章页滚动进度条正确；回顶可用（或确认复用 FAB）
3. 点击正文图片弹灯箱、ESC 关闭
4. 亮/暗模式样式正常；PJAX 后仍生效、控制台零错误
5. 首页/归档等非文章页不出现进度条

## Out of Scope

- 图片相册轮播（见 Spec 06 gallery）、评论区图片灯箱
- 阅读时间估算（可另立）
