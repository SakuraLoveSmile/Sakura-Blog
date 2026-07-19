# Spec 04 · 评论点赞（Comment Reactions）

- **分支**：`feature/comment-reactions`
- **主模块**：`internal/comment` + `internal/store/comment.go` + `partial/comments.html`
- **隔离等级**：中高（评论模块自成一体；路由挂在 `comment.Register` 内，**不碰 main.go**）

## Why

自建评论系统当前只能发/审/回复，缺少轻量互动。给每条评论加「点赞」：读者可对评论点赞，同 IP 去重，实时显示计数。全部改动落在评论模块与评论区模板，与文章/后台/前端其它页零耦合。

## What Changes

- `internal/store/comment.go`：
  - `Comment` struct 增 `Likes int64` 字段；`commentCols` 与 `scanComments` 同步加 `likes` 列（保持列序一致——见文件内 `commentCols` 注释铁律）。
  - 新增 `likes INTEGER NOT NULL DEFAULT 0` 列迁移（在评论表建表/迁移处，仿现有列管理方式；**不要动 `store/post.go` 的 createTables**）。
  - 新增去重表 `comment_likes(comment_id INTEGER, ip_hash TEXT, created_at, UNIQUE(comment_id, ip_hash))`。
  - 新增方法 `LikeComment(id int64, ipHash string) (liked bool, likes int64, err error)`：插入去重记录成功则 `likes+1` 并返回最新计数；已存在（重复点赞）返回当前计数、`liked=false`。
- `internal/comment/handler.go`：
  - `Register(r)` 内新增路由 `POST /api/comments/:id/like`（**在评论包内注册，main.go 无需改动**）。
  - 新增 `like(c *gin.Context)` handler：取评论 id、算 `ipHash = sha256(clientIP + salt)`、调 `store.LikeComment`，返回 JSON `{ok, likes, liked}`。带简单频率限制（复用现有 `allow(ip)` 思路）。
- `internal/template/partial/comments.html`：
  - 每条评论渲染点赞按钮 + 计数（用 `.Likes`）。
  - 追加**内联 `<script>`**：点击按钮 `fetch('/api/comments/{id}/like', {method:'POST'})`，成功后乐观更新计数、按钮置灰防连点。**内联在本模板，避免改 head.html**（head.html 归 Spec 01）。
- 列表接口回显：`internal/comment/handler.go` 的 `list` 返回的评论 JSON 增加 `likes` 字段（前端首次渲染显示计数）。

## 技术决策（agent 无需再询问）

- **点赞粒度**：针对**评论**点赞（非文章点赞）。
- **去重**：同 IP 对同一评论只计一次，用 `sha256(ip + 固定 salt)` 存 `ip_hash`（不存明文 IP，隐私友好）。salt 可用常量或复用 site 配置，无需新增配置项。
- **无需登录**：游客可点赞（与发评论一致的开放度）。
- **匿名撤销**：本期**不支持取消点赞**（重复点击不增不减），保持简单。
- **样式铁律**：**先 grep `assets/css/main.css`** 看是否有可复用的点赞/action 图标类（Stellar 评论区或 `.cap-action`/`.like`/`.tag-plugin.icon`）。**优先复用现成类**，图标用 `icons.yml` 已存在的 key（若无爱心图标则用文本「赞」+ 计数，勿引外部图标库）。
- **接口约定**：`POST /api/comments/:id/like` → `200 {ok:true, likes:N, liked:bool}`；非法 id → 400；评论不存在 → 404。
- **模板改动最小**：只在单条评论渲染块内插入按钮，不重构评论区结构。

## Impact

- **新增**：去重表 `comment_likes`（DB 内，非文件）
- **修改**：`internal/store/comment.go`（+Likes 字段/列/方法/去重表）、`internal/comment/handler.go`（+like 路由与 handler、list 回显 likes）、`internal/template/partial/comments.html`（+按钮 + 内联脚本）
- **不影响**：`cmd/server/main.go`（路由在 comment.Register 内）、post 渲染、admin、其它前端页、model.go、head.html、base.html

## 与其他 spec 的冲突点

- `internal/store/comment.go`：本 spec 唯一修改者（Spec 03 只新增 `attachment.go`，不碰 comment.go）。
- `partial/comments.html`：本 spec 唯一修改者。
- `internal/comment/`：本 spec 唯一修改者。
- **不碰 main.go**（区别于其它 spec，进一步降冲突）。

## ADDED Requirements

### Requirement: 评论点赞

The system SHALL allow visitors to like a comment, deduplicated per IP, with live count display.

#### Scenario: 首次点赞
- **WHEN** 访客点击某评论的点赞按钮
- **THEN** `POST /api/comments/:id/like` 返回 `{ok:true, liked:true, likes:N+1}`，页面计数 +1，按钮进入已赞态

#### Scenario: 重复点赞去重
- **WHEN** 同一 IP 再次点赞同一评论
- **THEN** 计数不再增加，接口返回当前计数、`liked:false`

#### Scenario: 评论区展示计数
- **WHEN** 加载文章评论区
- **THEN** 每条评论显示其当前点赞数（来自 `Comment.Likes`）

#### Scenario: 非法/不存在的评论
- **WHEN** 对非法 id 点赞
- **THEN** 返回 400；对不存在评论返回 404，页面不崩

## 验证

1. `go build ./... && go vet ./...` 通过
2. 起服务，打开含评论的文章，点赞按钮出现且显示初始计数
3. `curl -X POST http://localhost:<port>/api/comments/1/like` → `{ok:true,liked:true,likes:1}`；再次调用 → `liked:false,likes:1`
4. 刷新页面计数持久（已入库）
5. 亮/暗模式按钮样式正常，PJAX 切走再回评论区脚本重新生效、无重复绑定
6. 数据库 `comment_likes` 表有去重记录，`comments.likes` 同步

## Out of Scope

- 文章点赞 / 收藏 / 分享计数
- 取消点赞、多种表情（👍😂❤️ 等 reaction 类型）
- 登录用户维度的点赞（当前按 IP）
