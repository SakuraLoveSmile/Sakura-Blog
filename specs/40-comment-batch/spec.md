# Spec 40 · 评论批量操作

- **分支**：`feature/comment-batch` · **Wave 8** · **隔离等级**：高
- **主模块**：`console/src/modules/comments/CommentList.vue` + `internal/api/comment.go`（+批量端点）
- **前置**：无

## Why

Halo 对照缺失项：评论管理无批量操作（垃圾评论清理场景一条条点删极痛苦）。前端无多选、后端无批量端点，两端都要补。文章/页面列表已有成熟的批量选择交互（`PostList.vue`），直接复用其模式。

## 功能需求

1. 列表行首复选框 + 表头全选（当前页）；选中后顶部出现批量操作条：**批量通过 / 批量删除**（删除需确认弹窗，显示选中数）。
2. 批量操作后列表刷新、选中态清空、toast 报告成功/失败数。
3. 状态筛选（待审/已通过）下的全选只作用于当前筛选结果当前页。

## What Changes

- `internal/api/comment.go`：+`POST /api/v1/comments/batch`（body: `{ids: [], action: approve|delete}`），事务内逐条执行，返回 `{succeeded, failed}`。router.go +1 行。
- `internal/store/comment.go`：+批量方法（`ApproveComments(ids)` / `DeleteComments(ids)`，单 SQL `IN` 实现）+ `_test.go` 用例。
- `console/src/modules/comments/CommentList.vue`：复选框列 + 批量操作条（交互样式对齐 PostList 现有批量模式）+ api client +1 方法。

## 实现步骤

1. store 批量方法 + 单测（含空 ids、部分不存在 id 的边界）。
2. api 批量端点 + 单测（未登录 401、非法 action 400）。
3. CommentList 前端多选 + 操作条 + 确认弹窗。

## 技术决策（无需再问）

- **不做批量回复/批量驳回**：Halo 也只有通过/删除两个高频批量动作；驳回语义本项目不存在（待审即未通过）。
- **删除为硬删**：与现有单条删除语义一致（评论无回收站），确认弹窗文案明示不可恢复。
- **上限 100 条/次**：后端校验 ids 长度，超限 400。

## Impact

- **新增**：批量端点与 store 方法
- **修改**：CommentList.vue、api/index.ts、comment.go、store/comment.go、router.go
- **不影响**：前台评论展示、通知管线（批量通过不触发邮件，见验证）

## 验证

- [ ] 多选批量通过后前台评论区立即可见
- [ ] 批量删除有确认弹窗且 toast 报数
- [ ] 混入不存在 id 时其余照常成功，failed 计数正确
- [ ] 批量通过不触发回复邮件通知（通知仅在单条回复路径）
- [ ] `make test` 全绿
