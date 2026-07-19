# Spec 14 · 瞬间 / 说说（Moments）

- **分支**：`feature/moments` · **Wave 2（依赖 Spec 05 Part B admin 外壳）** · **隔离等级**：中高
- **主模块（独占）**：新 `internal/store/moment.go` + 新 `internal/moment/` 包

## Why

Stellar 支持「essay/说说」这类短内容流。补一个轻量「瞬间」：短文本 + 可选图片，时间流展示，后台可发/删。独立表 + 独立包 + 独立模板，与文章/评论零耦合。

## What Changes

- 新增 `internal/store/moment.go`：`Moment` struct（`id/content_md/content_html/images(逗号分隔 URL)/created_at`）+ 表 `moments` + `EnsureMomentsTable()`（不动 post.go）+ `ListMoments(page,size)`/`CountMoments`/`CreateMoment`/`DeleteMoment`。
- 新增 `internal/moment/moment.go`：前台 `GET /moments`（时间流 + 分页），渲染 `layout/base`，`Page.Layout="moments"`。`Register(r)`。
- 新增 admin（`internal/moment/admin.go`）：`/admin/moments`（列表 + 发布框）、`/admin/moments/create`、`/admin/moments/delete/:id`，复用 `admin.AuthMiddleware`。
- 新增模板：前台 `partial/moments.html`（对齐 Stellar essay/时间线卡片，用现成类）；后台 `admin/moment_list.html`（套 Part B 的 `admin/head`+`admin/sidebar`）。
- `internal/template/layout/base.html`：`+` `{{else if eq .Page.Layout "moments"}}{{template "partial/moments" .}}`。
- `admin/sidebar.html`：`+`「瞬间」菜单项（icon `solar:chat-square-like-bold-duotone`，MenuID `moments`）。
- `config/site.yml` menubar：可加「瞬间」→ `/moments`。
- 视图模型：新文件 `internal/template/model_moment.go`。

## 技术决策（无需再问）

- **短内容**：无标题/分类/标签，仅正文（Markdown 渲染，复用 `post.Service.Render`）+ 可选图片列表。
- **图片**：`images` 存逗号分隔 URL（可来自附件 `/uploads/...`），前台以小图网格展示，点击可放大（若 Spec 08 灯箱已合入则自动受益）。
- **展示**：`/moments` 按时间倒序，分页（20/页）。
- **鉴权**：admin 复用 `admin.AuthMiddleware`。
- **依赖顺序**：**Spec 05 Part B 合并后开工**（需 admin 外壳与 AuthMiddleware）。
- **对齐 Stellar**：卡片结构对照 `../resource/hexo-theme-stellar/` essay/时间线布局，先 grep main.css 复用现成类。

## Impact

- **新增**：`internal/store/moment.go`、`internal/moment/*.go`、`partial/moments.html`、`admin/moment_list.html`、`model_moment.go`
- **修改**：`base.html`（+1 派发）、`admin/sidebar.html`（+1 菜单）、`cmd/server/main.go`（+注册）、（可选）`site.yml`
- **不影响**：posts/comments/pages 逻辑

## ADDED Requirements

### Requirement: 瞬间时间流
The system SHALL let an admin post short moments and display them at `/moments` in reverse-chronological order.

#### Scenario: 浏览瞬间
- **WHEN** 访问 `/moments`
- **THEN** 返回 200，按时间倒序展示瞬间（正文 + 图片 + 相对时间），分页可用

#### Scenario: 发布与删除
- **WHEN** 管理员在 `/admin/moments` 发布或删除一条
- **THEN** 前台时间流即时反映

#### Scenario: 空状态
- **WHEN** 无任何瞬间
- **THEN** `/moments` 200 且显示空提示

## 验证

1. `go build ./... && go vet ./...` 通过
2. 后台发一条带图瞬间，`/moments` 正确展示；删除后消失
3. 分页正确；亮/暗模式与 Stellar 观感一致
4. admin 页套 Halo 外壳、侧栏「瞬间」高亮
5. PJAX 导航到 `/moments` 无控制台报错

## Out of Scope

- 瞬间评论/点赞、表情、位置/心情
- 瞬间独立 RSS（可后续接入 Spec 11 feed 包）
