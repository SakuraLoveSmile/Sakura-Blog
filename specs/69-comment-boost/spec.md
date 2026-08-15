# Spec 69 · 评论 Boost

- **分支**：`feature/comment-boost`
- **主模块**：`backend/internal/store/comment.go`、`backend/internal/comment/`、`frontend/assets/js/comments.js`、`frontend/assets/css/comments.css`
- **前置**：Spec 04 评论点赞

## Why

评论已有回复和点赞，但缺少低成本的简短回应。Boost 让访客用固定表情和最多 16 字的短文本表达回应，同时保留匿名去重、登录身份和作者保护。

## What Changes

- 每条评论最多 50 个 Boost；同一身份对同一评论只有一条，再次提交更新内容。
- 登录身份使用 `u:{user_id}`，匿名身份使用 `ip:{sha256(client_ip + salt)}`；Boost 作者不能回应自己的评论。
- 列表 API 和 SSR 一次返回 Boost chips、当前身份标记、数量和是否还能 Boost。
- 评论操作行显示火箭入口；已提交的自己的 chip 可点击删除，已 Boost 的入口移动到 chip 条末尾。

## UI 验收边界

- **唯一参照**：`baselines/boost-reference.md`，来源为本需求附件中 Part A 的已确认形态描述；本任务不引入第二参照。
- **验收清单**：
  - [ ] 评论正文下显示可换行的 Boost chip，chip 含 18px 头像、固定表情和可选短文本。
  - [ ] 操作行显示火箭入口；点击后打开固定 6 表情、最多 16 字输入和发送动作的小面板。
  - [ ] 已 Boost 的当前身份入口显示在 chip 条末尾；点击自己的 chip 可删除。
  - [ ] 评论作者和已满 50 条的评论不显示火箭入口。
  - [ ] 移动端入口、表情按钮和删除 chip 均保持至少 44pt 触达区域。
  - [ ] 亮色/暗色下 chip 和面板均使用现有 Apple/Stellar 设计 token，控制台无新增错误。

## ADDED Requirements

### Requirement: 评论 Boost

The system SHALL allow an anonymous or logged-in visitor to create one emoji-and-short-text Boost per comment, subject to identity deduplication, author protection, and a per-comment limit of 50.

#### Scenario: 创建或更新 Boost
- **WHEN** visitor submits one allowed emoji and optional text of at most 16 Unicode code points
- **THEN** the comment has one Boost for that identity and a repeated submission updates its emoji/text without increasing the count

#### Scenario: 删除自己的 Boost
- **WHEN** visitor deletes their own Boost
- **THEN** only that identity's Boost is removed and other Boosts remain unchanged

#### Scenario: 保护作者和上限
- **WHEN** the visitor is the comment author or the comment already has 50 Boosts from other identities
- **THEN** the API rejects the request and no row is written

## 验证

1. `go test ./backend/internal/store ./backend/internal/comment ./backend/internal/api`
2. `node --test scripts/test/comments.test.mjs`
3. `make merge-check`
4. UI 验收清单逐项对照 `baselines/boost-reference.md`，完成后补充截图到本目录。

## Out of Scope

- 平铺/树形评论切换、举报、外部评论服务。
- 自己删除他人的 Boost、取消点赞、可配置表情集。
