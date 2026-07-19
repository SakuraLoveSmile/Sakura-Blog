# 并行开发 Spec 集（夜间不间断开发）

每个子目录是一个**可并行开发**的功能 spec，遵循根目录 [`AGENTS.md`](../AGENTS.md)：**一个 agent = 一个 feature 分支 = 一个 worktree，从 dev 拉出，做完合回 dev**。

## 已完成（合入 dev）

01 代码高亮 · 02 友链 · 03 附件图床 · 04 评论点赞 —— 详见各目录 spec，均已 merge。

## 待办 · 夜间 8 小时并行批次

按**波次**编排：Wave 1 可**立即全部并行**；Wave 2/3 依赖前置（admin 外壳 / post 表结构），前置合并后再并行。

### Spec 05 · 前端清理 + 后台 Halo 化（前置，建议先起）
- Part A `fix/frontend-cleanup`（前端 head 清理，独立小分支）
- Part B `feature/admin-halo-console`（后台整体 Halo 外壳，**是 Wave 2 的前置**）

### Wave 1 — 立即并行（7 个，互不依赖，无 admin 耦合）

| # | Spec | 分支 | 主模块（独占） |
|---|------|------|---------------|
| 06 | [标签插件扩展包](06-tag-plugins/spec.md) | `feature/tag-plugins` | `internal/post/shortcode.go` + `assets/css/tag-plugins.css` |
| 07 | [KaTeX 数学公式](07-katex-math/spec.md) | `feature/katex-math` | `internal/post/post.go`（goldmark 装配） |
| 08 | [阅读体验增强](08-reading-ux/spec.md) | `feature/reading-ux` | `assets/js/reading.js` + `assets/css/reading.css` |
| 09 | [文章页脚](09-post-footer/spec.md) | `feature/post-footer` | `partial/content.html` 页脚区 + `config` |
| 10 | [侧边栏 widget 扩展](10-sidebar-widgets/spec.md) | `feature/sidebar-widgets` | `partial/rightbar.html` + `fillSidebar` |
| 11 | [Feed 增强](11-feeds/spec.md) | `feature/feeds` | 新 `internal/feed/` 包 |
| 12 | [PWA + 图标](12-pwa/spec.md) | `feature/pwa` | 新 `assets/manifest.webmanifest`/`sw.js` |

### Wave 2 — 依赖 Spec 05 Part B 合并后（内容类型，接入 admin 外壳）

| # | Spec | 分支 | 主模块（独占） |
|---|------|------|---------------|
| 13 | [独立页面系统 Pages](13-pages-cms/spec.md) | `feature/pages-cms` | `internal/store/page.go` + `internal/page/` |
| 14 | [瞬间/说说 Moments](14-moments/spec.md) | `feature/moments` | `internal/store/moment.go` + `internal/moment/` |

### Wave 3 — 依赖 05B + Wave2 合并后（改 post 表结构）

| # | Spec | 分支 | 主模块 |
|---|------|------|--------|
| 15 | [文章增强：置顶/密码/定时](15-post-enhancements/spec.md) | `feature/post-enhancements` | `internal/store/post.go`（+列） + admin edit |

## 冲突矩阵（共享文件 = 唯一需协调点）

| 共享文件 | 06 | 07 | 08 | 09 | 10 | 11 | 12 | 说明 / 合并顺序 |
|---------|----|----|----|----|----|----|----|----|
| `partial/head.html` | +link | +katex | +link | — | — | — | +manifest | **热点**：4 个 append `<link>/<script>`，按 06→07→08→12 顺序合，冲突均为单行 |
| `layout/base.html` | — | — | +元素 | — | — | — | +sw注册 | 仅 08/12，不同位置 |
| `cmd/server/main.go` | — | — | — | — | +fillSidebar | +Register | +routes | 各不同区段，append 式 |
| `internal/template/model.go` | — | — | — | +字段 | +字段 | — | — | 09/10 各加字段；优先放新 `model_*.go`，ViewData/Page 字段不可避免时 +1 行 |
| `config/site.go`+`site.yml` | — | — | — | +段 | — | +feed可选 | +pwa可选 | 09 为主；11/12 若需配置放独立 key |
| `internal/store/*` | — | — | — | — | +读方法(新文件) | — | — | 10 新增读方法用新文件，不碰 post.go |
| `go.mod` | mermaid(前端CDN,无需) | +katex/mathjax | — | — | — | — | — | 仅 07，`go mod tidy` |

**结论**：Wave 1 唯一真正竞争的是 `head.html`（4 个追加者）与 `main.go`（3 个不同区段）。都是 append 式单行冲突，按上表顺序串行 merge，秒解。其余为各自独占。

## 合并顺序总则

1. 先合 **05 Part A**（前端清理，零依赖）。
2. Wave 1 七个并行开发；合并顺序建议 **11 → 12 → 10 → 09 → 06 → 07 → 08**（把改 `main.go`/`head.html` 的靠前，减少后续 rebase）。每合一个，后面未合分支 `git rebase dev`。
3. 合 **05 Part B**（admin 外壳）。
4. Wave 2（13、14）并行 → 合并。
5. Wave 3（15）最后。

## 每个 agent 开工前必读

- 根目录 `AGENTS.md`、`CLAUDE.md`
- 铁律：**前端严格对齐原版 Stellar，禁止自创样式**；**补样式前先 grep `assets/css/main.css` 确认类是否已存在**；参考主题 `../resource/hexo-theme-stellar/`
- 模板路径化命名（`partial/xxx`、`admin/xxx`），基名勿撞现有
- 完成标准：`go build ./... && go vet ./...` 双通过 + spec 内「验证」清单全绿 + 起服务冒烟
