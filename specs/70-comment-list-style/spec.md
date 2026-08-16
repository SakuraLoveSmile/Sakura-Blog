# Spec 70 · 评论列表深色卡片样式

- **分支**：`dev`
- **主模块**：`frontend/assets/js/comments.js`、`frontend/assets/css/comments.css`
- **前置**：Spec 69 评论 Boost

## Why

现有评论列表已经具备顶层卡片和嵌套回复结构，但卡片层次、元信息排列、正文节奏和子回复缩进与用户确认的深色参照仍有明显差异。

## What Changes

- 顶层评论改为独立深灰圆角卡片，卡片间保留紧凑间距。
- 头像、昵称、验证图标、博主徽章、回复引用和时间统一成单行元信息。
- 子回复保留在父评论卡片内，以虚线分隔、透明背景和缩进布局展示。
- 回复、点赞和 Boost 行为全部保留，仅调整视觉层级；评论编辑器与 Boost 编辑器不属于本任务。

## UI 验收边界

- **唯一参照**：`baselines/comment-list-reference.png`，来源为用户确认的评论列表截图。
- **验收清单**：
  - [x] 顶层评论为独立深灰卡片，圆角、内边距和卡片间距与参照接近。
  - [x] 顶层头像约 36px，子回复头像约 30px，昵称使用主题蓝，时间为低对比灰色。
  - [x] 已验证用户显示小型 icon-only 徽章，博主显示紧凑橙色 pill。
  - [x] 正文为浅灰高对比文字，行高和段落间距接近参照，长文本不溢出。
  - [x] 子回复位于同一卡片内，顶部虚线分隔并缩进，回复引用清晰可读。
  - [x] 回复、点赞、Boost 入口保留且降低视觉权重；移动端触达区域不少于 44pt。
  - [x] 本任务只验收深色模式，桌面和移动端均无重叠、裁切或横向滚动。

## ADDED Requirements

### Requirement: 深色评论卡片

The system SHALL render each top-level comment as a dark rounded card and render its child replies inside the same card with dashed separation and indentation while preserving existing comment interactions.

#### Scenario: 顶层评论
- **WHEN** a page contains multiple approved top-level comments
- **THEN** each comment is displayed as an independent dark card with avatar, metadata, body, actions, and consistent spacing

#### Scenario: 子回复
- **WHEN** a top-level comment contains child replies
- **THEN** replies remain inside the parent card, use a dashed divider and indentation, and show the replied-to identity in the metadata line

## 验证

1. `node --test frontend/scripts/test/comments.test.mjs`
2. `make test`
3. 深色桌面与移动视口逐项对照 `baselines/comment-list-reference.png`

## Out of Scope

- 评论编辑器、登录弹窗和排序功能重构。
- 评论 API、数据库结构和审核逻辑调整。
- 亮色模式适配。
