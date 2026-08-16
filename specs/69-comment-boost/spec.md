# Spec 69 · 评论 Boost

- **分支**：`dev`
- **主模块**：`backend/internal/store/comment.go`、`backend/internal/comment/`、`frontend/assets/js/comments.js`、`frontend/assets/css/comments.css`
- **前置**：Spec 04 评论点赞

## Why

评论已有回复和点赞，但缺少低成本的简短回应。Boost 让访客用固定表情和最多 16 字的短文本表达回应，同时保留匿名去重、登录身份和作者保护。

## What Changes

- 每条评论最多 50 个 Boost；同一身份对同一评论只有一条，再次提交更新内容。
- 登录身份使用 `u:{user_id}`，匿名身份使用 `ip:{sha256(client_ip + salt)}`；Boost 作者不能回应自己的评论。
- 列表 API 和 SSR 一次返回 Boost chips、当前身份标记、数量和是否还能 Boost。
- 评论操作行显示火箭入口；已提交的自己的 chip 可点击删除，已 Boost 的入口移动到 chip 条末尾。
- Boost 编辑器使用单行深色胶囊；表情入口展开带搜索、分类和常用记录的 Unicode 表情选择器。

## UI 验收边界

- **唯一参照**：`baselines/boost-btn-reference.png`（实心灰色火箭入口）、`baselines/boost-composer-reference.png`（胶囊编辑器闭合态）、`baselines/boost-picker-reference.png`（表情选择器展开态），均为用户确认截图；`boost-reference.md` 仅保留行为约束，不作为第二视觉参照。
- **验收清单**：
  - [x] 评论正文下显示可换行的 Boost chip，chip 含 18px 头像、固定表情和可选短文本。
  - [x] 操作行显示火箭入口；点击后打开头像、`Boost {昵称}...` 输入、表情、确认和取消组成的单行深色胶囊。
  - [x] 点击表情入口展开本地 Unicode 表情选择器，包含搜索、分类、常用记录和可滚动网格，运行时不依赖第三方 CDN。
  - [x] API 接受一个合法 Unicode emoji grapheme cluster（含肤色、旗帜与 ZWJ 组合），仍拒绝普通文本、空值和多个表情。
  - [x] 已 Boost 的当前身份入口显示在 chip 条末尾；点击自己的 chip 可删除。
  - [x] 评论作者和已满 50 条的评论不显示火箭入口。
  - [x] 移动端入口、表情按钮、确认/取消和删除 chip 均保持至少 44pt 触达区域。
  - [x] 暗色模式下 chip 和面板均使用现有 Apple/Stellar 设计 token，控制台无新增错误。

## ADDED Requirements

### Requirement: 评论 Boost

The system SHALL allow an anonymous or logged-in visitor to create one Unicode emoji-and-short-text Boost per comment, subject to identity deduplication, author protection, and a per-comment limit of 50.

#### Scenario: 创建或更新 Boost
- **WHEN** visitor submits one valid Unicode emoji grapheme cluster and optional text of at most 16 Unicode code points
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
4. UI 验收清单逐项对照 `baselines/boost-composer-reference.png`，完成后补充截图到本目录。

## Out of Scope

- 平铺/树形评论切换、举报、外部评论服务。
- 自己删除他人的 Boost、取消点赞、站点级自定义表情集。
