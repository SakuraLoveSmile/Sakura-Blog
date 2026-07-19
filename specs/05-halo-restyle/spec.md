# Spec 05 · 前端高亮清理 + 后台 Halo Console 化改造

本 spec 含两部分。**Part A** 是小体量前端清理，**Part B** 是后台整体 Halo 化改造。二者互不重叠，可分两个分支：`fix/frontend-cleanup`（Part A）与 `feature/admin-halo-console`（Part B）。Part B 因所有 admin 页共用同一套布局，**不可再拆多 agent 并行**，应在单分支内按阶段推进。

调研依据：前端对标 `../resource/hexo-theme-stellar/`；后台对标 `../resource/screenshot/`（Halo 演示站截图）+ 项目既有记忆「管理后台 Halo Console 风格改造需求 / CSS 变量双轨规范 / Iconify 图标规范 / 模板路径约束 / sitebg 遮罩修复」。

---

# Part A · 前端高亮清理（分支 `fix/frontend-cleanup`）

## Why

代码高亮已迁到服务端 Chroma（`assets/css/highlight.css`，`.chroma` 类）。但 `head.html` 仍保留旧的 highlight.js `atom-one-dark` CDN 样式（`.hljs` 类），全站无任何 hljs JS 使用它——纯死加载，且两套暗色配色语义冲突。需清理并校正 Chroma 暗色作用域。

## What Changes

- `internal/template/partial/head.html`：删除第 44–45 行的 highlight.js `atom-one-dark` CDN `<link>`（含其上方注释）。
- `assets/css/highlight.css`：确认暗色令牌调色板不仅覆盖 `:root[data-theme="dark"]`，也覆盖**系统深色但未显式切换**的场景，即补/核对 `@media (prefers-color-scheme: dark) { :root:not([data-theme="light"]) .chroma ... }`（与 `head.html` 现有 `.theme-toggle` 暗色判定同款模式）。
- `assets/css/comments.css`：核对 `.cmt-like-btn` / `.cmt-like-btn.is-liked`（现 L215–219）着色使用主题变量（`var(--theme)` / `var(--text-p3)` 等），无写死色值。

## 技术决策

- Chroma 服务端高亮是唯一高亮方案，**不恢复 highlight.js**。
- highlight.css 的令牌调色板（github / github-dark）本身是语法高亮配色，允许保留其 palette 字面色值（非 admin UI，不受 admin「禁止写死」约束）；仅需保证亮/暗/系统暗三态都命中。

## Impact

- **修改**：`head.html`（-2 行）、`highlight.css`（暗色作用域校正）、`comments.css`（点赞态变量化，如已合规则免改）
- **不影响**：任何 Go 代码、路由、其它模板

## ADDED / MODIFIED Requirements

### Requirement: 单一代码高亮来源
The system SHALL style code blocks solely via server-side Chroma (`.chroma`), without loading any highlight.js theme.

#### Scenario: 页面不再加载 hljs 主题
- **WHEN** 打开任意含代码块的文章
- **THEN** 页面 `<head>` 不含 highlightjs CDN `<link>`，代码块由 `.chroma` 令牌正常着色

#### Scenario: 系统深色无显式切换
- **WHEN** 用户系统为深色且未点击主题切换（`html` 无 `data-theme`）
- **THEN** 代码块令牌显示暗色调色板，与深色背景对比充足

## 验证

1. `grep -n 'highlightjs\|hljs\|atom-one-dark' internal/template/ assets/js/` 无结果
2. `go build ./... && go vet ./...` 通过（Part A 不改 Go，仅确保未破坏）
3. 三态目视：`data-theme=light` / `data-theme=dark` / 无 data-theme+系统暗，代码块配色均正确
4. 点赞按钮亮/暗态样式正常

## Out of Scope

- 高亮配色主题更换、行号、diff 高亮

---

# Part B · 后台 Halo Console 化改造（分支 `feature/admin-halo-console`）

## Why

当前 admin 是单列 + 内联文字链接的最小可用 UI，**不符合** Halo Console 截图（侧边栏+顶栏、仪表盘、富列表、Iconify、浅色为主），且违反项目既定规范：5 个模板共 **54 处硬编码色值**、**0 处 Iconify**、**无共享 partial**、**无 light/dark 双轨**。本次做一次性对齐。

## 现状差距

| # | Halo/规范 | 现状 | 处置 |
|---|-----------|------|------|
| B1 | 左侧固定侧边栏 + 顶栏 | 单列 960px + 内联链接 | 新建 `admin/sidebar` + 顶栏 |
| B2 | 仪表盘（指标卡+快捷入口+最近评论） | 无 | 新增 `/admin/dashboard` + `admin/dashboard.html` |
| B3 | 颜色禁止写死 + 双轨变量 | 54 处硬编码 | 抽 admin 变量，全量变量化 |
| B4 | Iconify 图标 | 0 处 | 引 Iconify CDN，菜单/操作用 `<iconify-icon>` |
| B5 | 共享 admin/ 前缀 partial | 无 | 新建 `admin/head` + `admin/sidebar` |
| B6 | light/dark 双主题 | 仅深色变量 | 双轨，浅色为主 |
| B7 | 列表富行 + 筛选工具条 | 朴素表格 | 阶段 B-4（可选） |
| B8 | 登录页视觉统一 | 青色按钮/无 logo | 套统一视觉、深色主按钮 |

## What Changes（分阶段）

### 阶段 B-1 · 地基：共享布局 + 变量
- 新建 `internal/template/admin/head.html`：`<head>` 全部内容——`main.css` 引用、**Iconify CDN**、以及一个集中的 `<style>`（admin 变量双轨 + 侧边栏/顶栏/卡片/表格/按钮/徽章/分页组件样式）。各页通过 `{{template "admin/head" .}}` 引入，**消除各页重复 `<style>`**。
- 新建 `internal/template/admin/sidebar.html`：Halo 左侧栏（logo + 搜索框 + 分组图标菜单），当前项按 `.Page.MenuID` 高亮。`{{template "admin/sidebar" .}}` 引入。
- 保留遮罩修复 `body>.sitebg{pointer-events:none !important}`（并入 `admin/head` 的 `<style>`，各页不再各自写）。

### 阶段 B-2 · 迁移现有 5 页到新布局
`login.html` / `list.html` / `comments.html` / `media.html` / `edit.html` 逐一改为：`<html>{{template "admin/head" .}}<body><div class="sitebg">…<div class="admin-shell"><aside class="admin-side">{{template "admin/sidebar" .}}</aside><main class="admin-main"><header class="admin-topbar">标题+操作按钮</header>…页面主体…</main></div></body></html>`。清空各页硬编码色，全部走 `var(--admin-*)`。`edit.html` 的 EasyMDE 覆写样式同样变量化。

### 阶段 B-3 · 新增仪表盘
- 后端：`internal/admin/admin.go` 加 `dashboard` handler + 在 `Register` 挂 `GET /admin/dashboard`，并让 `GET /admin`、`GET /admin/` 302 到 `/admin/dashboard`（或直接渲染仪表盘）。
- 聚合数据（见下「Dashboard 字段」）。
- 新建 `internal/template/admin/dashboard.html`：指标卡网格 + 快捷入口 + 最近评论列表。

### 阶段 B-4 · 富列表（可选增强）
文章行加状态点/日期/（可选封面），评论/附件行对齐截图；顶部加关键词搜索。**非阻塞项**，可后续单独做。

## 技术决策（实现者无需再询问）

### 共享 partial 命名
- 走路径化命名：`admin/head`、`admin/sidebar`、`admin/dashboard`，均在 `internal/template/admin/` 下，`admin/` 前缀防基名覆盖（符合模板路径约束记忆）。
- **更新既有「仅 4 文件」约束**：现允许 `internal/template/admin/` 下全部文件（含 media.html 及新 partial）。

### Admin CSS 变量（双轨，浅色为主）
在 `admin/head` 的 `<style>` 顶部按**四作用域**定义，**字面色值只允许出现在这些定义块内**，所有组件规则一律 `var(--admin-*)`：

| 变量 | 亮色 | 暗色 | 用途 |
|------|------|------|------|
| `--admin-bg` | `#f7f8fa` | `#16181d` | 页面底 |
| `--admin-surface` | `#ffffff` | `#1f2228` | 卡片/侧栏/表格 |
| `--admin-surface-2` | `#f3f4f6` | `rgba(255,255,255,.04)` | 表头/hover |
| `--admin-border` | `#e5e7eb` | `rgba(255,255,255,.08)` | 描边/分隔 |
| `--admin-text` | `#1f2937` | `#e5e7eb` | 主文字 |
| `--admin-text-sub` | `#6b7280` | `#9ca3af` | 次文字 |
| `--admin-primary` | `#111827` | `#2563eb` | 主按钮底 |
| `--admin-on-primary` | `#ffffff` | `#ffffff` | 主按钮字 |
| `--admin-active-bg` | `#eaf5f1` | `rgba(255,255,255,.06)` | 侧栏当前项底 |
| `--admin-active-text` | `#111827` | `#ffffff` | 侧栏当前项字 |
| `--admin-accent` | `var(--theme)` | `var(--theme)` | 链接/hover |
| `--admin-success` / `--admin-warning` / `--admin-danger` | `#3DC550` / `#FA6400` / `#F44336` | 同左 | 状态徽章前景 |

四作用域写法：`:root{ 亮 }`、`@media (prefers-color-scheme: dark){ :root:not([data-theme="light"]){ 暗 } }`、`:root[data-theme="dark"]{ 暗 }`、`:root[data-theme="light"]{ 亮 }`。徽章背景用对应色 + 低透明度 tint（也定义成变量，禁止在规则里写 `rgba(...)` 字面）。

### 侧边栏菜单 + Iconify key（Solar duotone，走 Iconify CDN）
Iconify CDN：`https://cdn.jsdelivr.net/npm/iconify-icon@2/dist/iconify-icon.min.js`，仅 admin 页引入；语法 `<iconify-icon icon="solar:xxx"></iconify-icon>`。

| 菜单 | 路由 | icon | MenuID(active) |
|------|------|------|----------------|
| 仪表盘 | `/admin/dashboard` | `solar:widget-5-bold-duotone` | `dashboard` |
| 文章 | `/admin` | `solar:document-text-bold-duotone` | `posts` |
| 评论 | `/admin/comments` | `solar:chat-round-line-bold-duotone` | `comments` |
| 附件 | `/admin/media` | `solar:gallery-bold-duotone` | `media` |
| 查看前台 | `/`（新标签） | `solar:home-2-bold-duotone` | — |
| 退出 | POST `/admin/logout` | `solar:logout-2-bold-duotone` | — |

顶栏「新建」`solar:add-circle-bold-duotone`；搜索框 `solar:magnifer-linear`。侧栏 logo 用 `{{.Site.Title}}`。

### 当前项高亮
各 admin handler 把 `Page.MenuID` 设为对应 active 键（`posts`/`comments`/`media`/`dashboard`），`admin/sidebar` 用 `{{if eq $.Page.MenuID "posts"}}active{{end}}` 判定。现有 handler 都设的是 `"admin"`，逐个改为具体键（属 Part B 后端改动，允许）。

### Dashboard 字段与 store 方法
| 指标 | 取值 |
|------|------|
| 文章总数 / 已发布 | `len(store.List())` / `store.CountPublished()` |
| 评论总数 / 待审 | `store.CountComments("")` / `store.CountComments(store.CommentPending)` |
| 总浏览量 | 新增 `store.SumViews()` → `SELECT COALESCE(SUM(views),0) FROM posts`（加在 `internal/store/post.go`） |
| 附件数 | `store.CountAttachments()` |
| 最近评论 | `store.ListRecentComments(5)` |

视图模型：新增 `internal/template/model_admin.go` 定义 `AdminDashboard`（各计数 + `[]*AdminCommentRow` 最近评论 + 快捷入口），在 `AdminData` 加 `Dashboard *AdminDashboard` 字段（model.go +1 行）。

## Impact

- **新增**：`admin/head.html`、`admin/sidebar.html`、`admin/dashboard.html`、`internal/template/model_admin.go`
- **修改**：`admin/login.html`、`admin/list.html`、`admin/comments.html`、`admin/media.html`、`admin/edit.html`（套布局+变量化）；`internal/admin/admin.go`（dashboard handler + 路由 + 各 handler 的 MenuID）；`internal/store/post.go`（+`SumViews`）；`internal/template/model.go`（AdminData +Dashboard 字段）
- **不影响**：前端所有页面、评论/文章渲染逻辑、Part A 涉及文件

## ADDED Requirements

### Requirement: Halo 风格后台布局
The admin console SHALL use a persistent left sidebar + top bar layout across all admin pages, styled with dual-track light/dark CSS variables and Iconify icons, with no hardcoded color values in component rules.

#### Scenario: 侧边栏导航与高亮
- **WHEN** 管理员访问任一 admin 页
- **THEN** 左侧显示 logo+搜索+图标菜单，当前页对应菜单项高亮，顶栏显示页标题与操作按钮

#### Scenario: 双主题
- **WHEN** 在 light / dark（含系统暗）下浏览 admin
- **THEN** 所有页面配色正确、对比充足，无写死色导致的错配

#### Scenario: 遮罩不拦截
- **WHEN** 在任一 admin 页点击按钮/链接/输入框
- **THEN** 均可正常交互（sitebg `pointer-events:none` 生效）

### Requirement: 仪表盘
The system SHALL provide `/admin/dashboard` showing post/comment/view/attachment metrics, quick-access entries, and recent comments.

#### Scenario: 访问仪表盘
- **WHEN** 管理员访问 `/admin` 或 `/admin/dashboard`
- **THEN** 返回 200，展示指标卡（文章/评论/浏览量/附件）、快捷入口、最近 5 条评论

## 验证（沿用记忆硬约束）

1. `go build ./... && go vet ./...` 通过
2. **零硬编码色**：`grep -rnoE '#[0-9a-fA-F]{3,6}|rgba?\([0-9]' internal/template/admin/*.html | grep -v 'admin-'` 仅命中变量定义块（组件规则内为 0）
3. 六项功能实测：登录 / 文章列表 / 新建编辑 / 删除 / 评论管理（审核/回复/删除）/ 附件上传删除，全部可用
4. light + dark（含系统暗）双模式所有 admin 页目视正常
5. 侧边栏当前项高亮正确；`/admin` 落仪表盘、指标数与库内一致
6. 移动端视口（对齐截图 20–22）侧栏可收起/不遮挡主体

## Out of Scope

- 用户管理 / 插件 / 主题商店 / 系统设置等 Halo 大板块（截图 11–16，本项目无对应后端）
- 富文本块编辑器（保持 EasyMDE Markdown）
- 列表富行的封面缩略图/`...`浮层菜单可留至 B-4 迭代
