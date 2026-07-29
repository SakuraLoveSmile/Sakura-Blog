# Spec 44 · 附件分组管理

- **分支**：`feature/attachment-groups` · **Wave 8** · **隔离等级**：中
- **主模块**：`internal/store/attachment.go`（+分组）+ `internal/api/attachment.go` + `console/src/modules/attachments/AttachmentList.vue` + `AttachmentPickerModal.vue`
- **前置**：无

## Why

Halo 附件库对照缺失项：无分组能力——附件量增长后（封面/文内图/图标素材混在一起）检索效率下降。本 spec 补齐分组；**多存储策略（S3 等）明确不做**（见技术决策，这是差距清单该项的正式处置）。

## 功能需求

1. **分组 CRUD**：附件页左侧分组栏（全部/未分组/自定义分组列表），建组/重命名/删除（删除组时组内附件归入未分组，不删文件）。
2. **归组操作**：上传时选目标分组（缺省当前浏览分组）；已有附件单个/批量移动分组。
3. **筛选联动**：分组 + 现有类型筛选（图片/非图片）+ 关键词三者叠加。
4. **选择器同步**：`AttachmentPickerModal` 顶部 +分组下拉，写作时可按组快速定位。

## What Changes

- `internal/store/attachment.go`：+`attachment_groups` 表（id/name/order）+ attachment 表 +group_id 列（ALTER 迁移，0=未分组）+ 分组 CRUD/移动/按组查询方法 + `_test.go`。
- `internal/api/attachment.go`：+分组端点（`GET/POST/PUT/DELETE /api/v1/attachment-groups`、`POST /api/v1/attachments/move`）；列表端点 +group 参数。
- `console/src/modules/attachments/AttachmentList.vue`：分组侧栏 + 批量移动（复用多选交互）+ 上传归组。
- `console/src/components/AttachmentPickerModal.vue`：+分组下拉。

## 实现步骤

1. store 表 + 迁移 + 方法 + 单测（删组归零、移动幂等）。
2. api 端点 + 单测。
3. AttachmentList 分组侧栏 + 移动交互。
4. PickerModal 下拉（小改）。

## 技术决策（无需再问）

- **多存储策略（S3/OSS/WebP 云处理）不做**：单二进制自托管定位下本地磁盘 + 反代缓存已够；引入对象存储会把备份/恢复/引用检查全链路复杂化。若未来外置存储，走反代层（如 rclone mount）而非应用层。此为差距清单「多存储策略」项的正式处置。
- **单层分组**：不做嵌套分组（Halo 也是单层）。
- **物理目录不动**：分组是纯逻辑标签，uploads/年/月 物理布局保持，避免移动文件破坏已发布文章引用。
- **未分组语义**：group_id=0 常量，不建"未分组"实体行。

## Impact

- **新增**：attachment_groups 表、分组端点
- **修改**：store/attachment.go、api/attachment.go、AttachmentList.vue、AttachmentPickerModal.vue、router.go
- **不影响**：上传管线、缩略图、refs 检查、前台

## 验证

- [ ] 建组/改名/删组（附件归未分组）全流程
- [ ] 上传归组 + 批量移动 + 三重筛选叠加正确
- [ ] 写作选择器按组过滤可用
- [ ] 已发布文章图片 URL 不因分组操作变化
- [ ] 旧库迁移幂等
- [ ] `make test` 全绿
