# Spec 71 · 评论用户管理（reader 管理 + 封禁 + 用户级评论处置）

- **分支**：`feature/comment-users` · **Wave 12**
- **主模块**：`backend/internal/store/user.go`、`backend/internal/store/comment.go`、`backend/internal/api/users.go`、`backend/internal/api/auth_public.go`、`frontend/console/src/modules/comments/CommentUsersView.vue`
- **前置**：Spec 45（用户账户体系）、读者注册链路（/api/auth/register）

## Why

评论者经 `/api/auth/register` 注册为 reader 后可登录评论（评论带 user_id + verified 徽章），但后台对这批用户完全失焦：Console 用户管理页只管 admin/author，reader 不可见、不可管。出现刷屏/恶意账号时只能逐条处置评论，无法按用户维度查看、封禁或批量清理。

## What Changes

### 后端

- `store/user.go`：users 表补 `status` 列（ALTER 迁移，默认 `active`）；`User` 结构加 `Status`；新增 `UpdateUserStatus` / `CountUsersByStatus` / `ListCommentUsers` / `CountCommentUsers`（LEFT JOIN comments 聚合评论数与最后评论时间）。
- `store/comment.go`：`ListComments` 支持 `user_id` 过滤；新增 `BulkCommentsByUser(userID, action)`（approve/spam/delete，delete 级联清理 boosts/reactions）。
- `api/auth_public.go`：`currentUserMiddleware` 对封禁用户按匿名处理（即时生效）；`publicLogin` / 2FA 拒绝封禁用户并记 login log。
- `api/users.go` + `router.go`：sys 组新增 `GET /api/v1/comment-users`、`PUT /api/v1/comment-users/:id/status`、`DELETE /api/v1/comment-users/:id`、`POST /api/v1/comment-users/:id/comments/bulk`。

### 前端

- `api/index.ts`：`CommentUserItem` 类型 + `commentUsersApi`。
- `router/index.ts`：`/comment-users` 路由（adminOnly）。
- `layouts/DefaultLayout.vue`：「互动」分组新增「评论用户」导航。
- `modules/comments/CommentUsersView.vue`（新）：统计卡 + 分段筛选（全部/已封禁）+ 搜索 + 表格/移动卡片；查看评论弹窗（分页列表 + 全部标为垃圾/全部删除批量条）；封禁/解封、删除二次确认。

## UI 验收边界

- **参照图路径**：`baselines/user-list-reference.png`（唯一参照：现有 /console/users 用户管理页截图，同卡片/表格/徽章/弹窗体系）
- **验收清单**：
  - [x] /console/comment-users 页面结构与参照一致（同款统计卡/表格/徽章/弹窗）—— verify-desktop.png
  - [x] 列表正确显示评论数与最后评论时间；无评论用户评论数为 0（store 测试覆盖 silent-reader）
  - [x] 封禁后该用户前台立即变匿名态、登录被拒；解封恢复（API 链路实测 403「该账号已被封禁」）
  - [x] 查看评论弹窗分页展示，「全部标为垃圾/全部删除」返回受影响条数—— verify-modal.png
  - [x] 移动端卡片布局正常—— verify-mobile.png
  - [x] 验收完成后截图存入 `baselines/` 并更新本节勾选状态

## ADDED Requirements

### Requirement: 评论用户列表

The system SHALL 在 Console 提供按用户维度查看注册评论者（role=reader）的列表，含评论数与最后评论时间聚合。

#### Scenario: 管理员查看评论用户
- **WHEN** admin 访问 `GET /api/v1/comment-users`
- **THEN** 返回 reader 用户分页列表，每项含 comment_count / last_comment_at / status，并附 all/banned 计数

### Requirement: 封禁与解封

The system SHALL 支持封禁/解封评论用户，封禁即时生效。

#### Scenario: 封禁后即时失效
- **WHEN** admin 将用户置为 banned
- **THEN** 该用户既有会话按匿名处理（评论/Boost/me 失效），登录被拒并记 login log

### Requirement: 用户级评论批量处置

The system SHALL 支持按用户对其全部评论执行 approve / spam / delete 批量操作。

#### Scenario: 清理刷屏用户
- **WHEN** admin 对某用户执行 bulk delete
- **THEN** 该用户名下全部评论及其 boosts/reactions 被删除，返回受影响条数

## 验证

1. `go vet ./backend/...` + `go test ./backend/...` 通过
2. UI 子任务：「UI 验收边界」清单逐项对照基线截图勾选全绿
3. `make merge-check` 全绿后合入 dev

## Out of Scope

- 匿名评论者（user_id=0）按邮箱聚合成虚拟用户
- 现有用户管理页「已禁用」空壳分段的修复
- 封禁 IP 维度拉黑、注册开关配置
