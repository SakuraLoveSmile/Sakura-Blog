# Spec 13 · 独立页面系统（Pages CMS）

- **分支**：`feature/pages-cms` · **Wave 2（依赖 Spec 05 Part B admin 外壳）** · **隔离等级**：中高
- **主模块（独占）**：新 `internal/store/page.go` + 新 `internal/page/` 包

## Why

博客缺少「独立页面」（关于、隐私政策、自定义页等），这类内容不属于时间流文章，需单独的 slug 直达页 + 后台管理。数据与 UI 走全新文件，仅在 admin 外壳与前台布局派发处有小接触。

## What Changes

- 新增 `internal/store/page.go`：`Page` struct（`id/slug/title/content_md/content_html/status/created_at/updated_at`）+ 独立表 `pages` + `EnsurePagesTable()`（不动 post.go 的 createTables）+ CRUD/查询方法。
- 新增 `internal/page/page.go`：前台 handler，`GET /pages/:slug`（published 才展示，否则 404）；渲染走 `layout/base`，`Page.Layout="page"`。`Register(r)`。
- 新增 admin：`internal/page/admin.go`（或并入 page 包）——`/admin/pages` 列表、`/admin/pages/new`、`/admin/pages/edit/:slug`、`/admin/pages/save`、`/admin/pages/delete/:slug`，复用 `admin.AuthMiddleware`。
- 新增模板：`admin/page_list.html`、`admin/page_edit.html`（**套 Spec 05B 的 `admin/head`+`admin/sidebar`**，EasyMDE 编辑）；前台 `partial/page.html`（对齐 Stellar 普通页布局，用 `.md-text`）。
- `internal/template/layout/base.html`：`+` `{{else if eq .Page.Layout "page"}}{{template "partial/page" .}}`。
- `admin/sidebar.html`（Part B 产物）：`+` 「页面」菜单项（icon `solar:file-text-bold-duotone`，MenuID `pages`）。
- `cmd/server/main.go`：`+` `pageHandler.Register(r)` 与 admin 注册（各 1 行）。
- 视图模型：新文件 `internal/template/model_page.go`。

## 技术决策（无需再问）

- **与文章分离**：pages 独立表，不复用 posts（语义不同：无分类/标签/归档，不进首页/RSS）。
- **渲染**：Markdown 经 `post.Service.Render` 复用同一 goldmark（含标签插件/高亮），保存时预渲染 `content_html`。
- **鉴权**：admin 路由复用 `admin.AuthMiddleware(site)`（Part B 已导出）。
- **路由**：前台 `/pages/:slug`（避免与 `/post/:slug`、根级 slug 冲突）。
- **菜单接入**：页面可被 `site.yml` menubar 引用（如「关于」→ `/pages/about`）。
- **依赖顺序**：**必须在 Spec 05 Part B 合并后开工**（需要 `admin/head`/`admin/sidebar` 与 `AuthMiddleware`）。

## Impact

- **新增**：`internal/store/page.go`、`internal/page/*.go`、`admin/page_list.html`、`admin/page_edit.html`、`partial/page.html`、`model_page.go`
- **修改**：`base.html`（+1 派发）、`admin/sidebar.html`（+1 菜单）、`cmd/server/main.go`（+注册）
- **不影响**：posts/comments/media 逻辑、前端其它页

## ADDED Requirements

### Requirement: 独立页面
The system SHALL let an admin manage standalone pages and serve published ones at `/pages/:slug`.

#### Scenario: 前台访问
- **WHEN** 访问已发布页 `/pages/about`
- **THEN** 返回 200，渲染页面标题与正文（`.md-text`）

#### Scenario: 草稿不公开
- **WHEN** 页面为 draft 或不存在
- **THEN** 前台 404

#### Scenario: 后台 CRUD
- **WHEN** 管理员在 `/admin/pages` 新建/编辑/删除页面
- **THEN** 变更即时生效，列表反映最新状态；侧栏「页面」高亮

## 验证

1. `go build ./... && go vet ./...` 通过
2. 后台新建「关于」页并发布，`/pages/about` 200 且正文正确
3. 设为草稿后前台 404；删除后 404
4. admin 页套用 Halo 外壳、侧栏高亮正确；亮/暗正常
5. menubar 配置 `/pages/about` 可从侧边栏直达

## Out of Scope

- 页面版本历史、自定义模板/布局选择
- 嵌套页面/层级导航
