# Spec 03 · 附件 / 图床管理（Media Library）

- **分支**：`feature/media-library`
- **主模块**：新增 `internal/media` 包 + `internal/store/attachment.go` + admin 后台页
- **隔离等级**：中高（新包 + 新 store 文件；仅在 admin auth、admin 导航处有小接触）

## Why

写文章时插入图片目前只能贴外链，缺少本地上传/图床。补一个后台附件管理：上传图片、列表浏览、复制 URL、删除。数据与 UI 都走**新增文件**，与文章/评论逻辑零耦合。

## What Changes

### 新增文件（主体）

- `internal/media/media.go`：`Handler`（持 `*store.Store`、`*tmpl.Engine`、`*tmpl.Site`、上传根目录）。方法：`list`（分页列表页）、`upload`（POST 接收 multipart 存盘 + 入库）、`delete`（删记录 + 删文件）。`Register(r, authMW)` 挂 `/admin/media` 路由组。
- `internal/store/attachment.go`：`Attachment` struct + 独立表 `attachments`；方法 `EnsureAttachmentsTable()`（建表，由 media.New 或 main.go 启动时调用，**不动 post.go 的 createTables**）、`CreateAttachment`、`ListAttachments(page,size)`、`CountAttachments`、`GetAttachment`、`DeleteAttachment`。
- `internal/template/admin/media.html`：附件管理页（网格缩略图 + 上传表单 + 复制 URL 按钮 + 删除）。自带完整 HTML 骨架（不套 base，与其它 admin 页一致），复用 main.css 变量。
- `internal/template/model_media.go`：`AdminMediaRow`、`AdminMediaData` 等视图模型（**独立文件，不动 model.go**）。

### 少量修改（接触点）

- `internal/admin/auth.go`：**新增导出函数** `AuthMiddleware(site *tmpl.Site) gin.HandlerFunc`，把现有 `authRequired` 的判定逻辑抽成可复用中间件（`authRequired` 内部改为调用它）。media 复用它保护 `/admin/media`。
- `cmd/server/main.go`：+ 构造 `media.New(...)` 与 `mediaHandler.Register(r, admin.AuthMiddleware(site))`（约 2 行，挨着现有 `adminHandler.Register(r)`）。上传根目录从 `app.yml` 读（见下）。
- `internal/config/app.go` + `config/app.yml`：新增 `upload_dir`（默认 `data/uploads`）、`max_upload_mb`（默认 5）。
- `cmd/server/main.go` 静态服务：新增 `r.Static("/uploads", <uploadDir>)` 暴露已上传文件（放在 `/assets` 静态注册附近）。
- admin 导航：在 `internal/template/admin/list.html`、`admin/comments.html` 顶部导航区加「附件」链接（与「评论管理」并列）。

## 技术决策（agent 无需再询问）

- **存储**：本地磁盘 `upload_dir`，按 `年/月/` 分目录；文件名 = `时间戳_随机短串.ext`（防重名/防猜测），原始名存 DB。**不引第三方对象存储**。
- **鉴权**：复用 admin 的 HMAC cookie（`admin.AuthMiddleware`）。上传/删除必须登录。
- **允许类型**：图片为主 —— `.jpg/.jpeg/.png/.gif/.webp/.svg`，按扩展名 + `Content-Type` 双校验；大小 ≤ `max_upload_mb`。
- **表结构**（`attachments`）：`id / filename(原名) / path(相对 upload_dir) / url(对外 /uploads/...) / mime / size / created_at`。
- **对外 URL**：`/uploads/<相对路径>`，静态托管。**注意与 go:embed 无关**（上传文件是运行时数据，不嵌入二进制）。
- **表创建隔离**：`attachments` 表由 `EnsureAttachmentsTable()` 单独建，**不要**往 `store/post.go` 的 `createTables()`/`migrateColumns()` 里加东西（避免与其它改 store 的 spec 冲突；实际其它 spec 只碰 comment.go）。
- **删除语义**：删记录同时删磁盘文件；文件已不存在时忽略错误继续删记录。
- **模板基名**：`admin/media` 路径化，不冲突。

## Impact

- **新增**：`internal/media/media.go`、`internal/store/attachment.go`、`internal/template/admin/media.html`、`internal/template/model_media.go`
- **修改**：`internal/admin/auth.go`（+导出中间件）、`cmd/server/main.go`（+构造/注册/静态/上传目录约 4 行）、`internal/config/app.go` + `config/app.yml`（+2 配置）、`admin/list.html` + `admin/comments.html`（+导航链接）
- **不影响**：post 渲染、comment 逻辑、前端页面、model.go、post.go

## 与其他 spec 的冲突点

- `internal/admin/*`：本 spec 是唯一修改 admin 的。Spec 01/02/04 均不碰 admin。
- `internal/store/`：本 spec 只新增 `attachment.go`，Spec 04 只改 `comment.go`，互不重叠。
- `app.go` / `app.yml`：本 spec 唯一修改者。
- `main.go`：与 02/04 共享不同区段（本 spec 集中在 admin 注册 + 静态服务区）。
- `model.go`：不碰（新模型走 `model_media.go`）。

## ADDED Requirements

### Requirement: 后台附件上传与管理

The system SHALL allow an authenticated admin to upload, list, and delete image attachments via `/admin/media`.

#### Scenario: 上传图片
- **WHEN** 已登录管理员在附件页选择 ≤ 上限的图片并提交
- **THEN** 文件存入 `upload_dir/年/月/`，DB 新增记录，页面显示缩略图与可复制的 `/uploads/...` URL

#### Scenario: 非法类型/超限被拒
- **WHEN** 上传非图片类型或超过 `max_upload_mb`
- **THEN** 返回 400 并提示原因，不写盘、不入库

#### Scenario: 未登录禁止访问
- **WHEN** 未登录（且 `admin.password` 已配置）访问 `/admin/media` 或上传接口
- **THEN** 303 跳转 `/admin/login`

#### Scenario: 删除附件
- **WHEN** 管理员删除某附件
- **THEN** DB 记录与磁盘文件均被移除（文件缺失时仍成功删记录），列表不再展示

#### Scenario: 访问已上传文件
- **WHEN** 请求 `/uploads/<path>`
- **THEN** 返回该图片内容（200）

## 验证

1. `go build ./... && go vet ./...` 通过
2. 起服务（配置 `admin.password`），登录后访问 `/admin/media` 返回 200
3. `curl -F file=@test.png <带cookie> http://localhost:<port>/admin/media/upload` 成功入库并返回可访问 URL
4. `curl -I http://localhost:<port>/uploads/<year>/<month>/<file>` 返回 200
5. 未带 cookie 访问 `/admin/media` → 303 到登录页
6. 上传 `.txt` 或超大文件 → 400
7. 删除后列表消失、`/uploads/...` 404

## Out of Scope

- 图片压缩/裁剪/水印/EXIF 清理
- 第三方对象存储（S3/OSS/图床 API）
- 在 EasyMDE 编辑器里直接拖拽上传（可作后续增强，届时才改 `admin/edit.html`）
- 附件与文章的关联关系
