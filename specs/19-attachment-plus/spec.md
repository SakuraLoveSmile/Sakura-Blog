# Spec 19 · 附件增强（缩略图 / WebP / 引用统计 / 搜索）

- **分支**：`feature/attachment-plus` · **Wave 4** · **隔离等级**：中
- **主模块（独占）**：`internal/api/attachment.go` + `internal/store/attachment.go` + `console/src/modules/attachments/AttachmentList.vue`

## Why

附件目前只有「上传 → 落盘 `data/uploads/YYYY/MM/` → 记录 → 列表分页 → 删除」的裸链路：原图多大存多大（手机照片动辄数 MB 直出文章页）、Console 列表加载原图当缩略图、删除前不知道有没有文章在用、附件多了没法找。对齐 Halo 附件库体验，补四项：上传压缩转 WebP、独立缩略图、引用统计、搜索筛选。

## 功能需求

1. **上传即优化**：图片类附件（jpeg/png，>阈值 200KB 或宽 >2000px 时）上传时自动压缩并转存 WebP 作为对外文件；gif/svg/webp 与非图片不动。原始文件不保留（省磁盘，单人博客无需原图归档）。
2. **缩略图**：每个图片附件生成 ~400px 宽缩略图（WebP），Console 列表网格用缩略图加载；非图片附件显示类型图标。
3. **引用统计**：列表每项显示「被 N 处引用」；点开可见引用来源（文章/页面标题 + 链接到对应编辑器）。删除被引用附件时确认弹窗明确警告引用处将裂图。
4. **搜索与筛选**：文件名关键字搜索 + 类型筛选（图片/其它）+ 时间倒序，均走服务端查询参数。
5. **详情信息**：列表项展示尺寸（宽×高）、文件大小、格式；上传后回填这些元数据。

## What Changes

- `go.mod`：新增图像处理依赖。**决策**：用纯 Go 的 `github.com/HugoSmits86/nativewebp`（无 CGO）编码 WebP + 标准库 `image` / `golang.org/x/image/draw` 缩放解码。若实测编码质量/体积不达标，降级方案为压缩后仍存 JPEG（质量 80）+ 缩略图 JPEG，接口结构不变。
- `internal/store/attachment.go`：
  - `Attachment` struct + `ThumbPath string`、`ThumbURL string`、`Width int`、`Height int`。
  - `EnsureAttachmentsTable` 后追加幂等 `ALTER TABLE`（对齐 post.go `migrateColumns` 模式）+4 列。
  - `ListAttachments` 支持 `keyword`（filename LIKE）与 `kind`（image/other，按 mime 前缀）参数；新增 `CountAttachmentRefs` 不在 store 做（引用扫描见 api 层）。
- 新增 `internal/api/attachment_image.go`（同包新文件，避免 attachment.go 膨胀）：解码 → 按需缩放 → WebP 编码 → 缩略图生成的纯函数集合。
- `internal/api/attachment.go`：
  - `uploadAttachment`：图片分支接入优化管线，落盘主图 + `thumb_` 前缀缩略图，回填宽高。
  - `listAttachments`：透传 keyword/kind 查询参数。
  - 新增 `GET /api/v1/attachments/:id/refs`：在 posts.content + pages.content 中 `LIKE '%'||url||'%'` 扫描，返回引用来源列表（slug、title、类型）。
  - `deleteAttachment`：连带删除缩略图文件。
- `internal/api/router.go`：+1 行 refs 路由。
- `console/src/modules/attachments/AttachmentList.vue`：
  - 网格卡片 `<img>` 换 thumb_url（缺省回退 url）；卡片角标显示格式与大小。
  - 顶部工具栏：搜索框（防抖 300ms）+ 类型筛选下拉。
  - 详情/删除弹窗：展示尺寸与引用列表（调 refs 接口），有引用时删除按钮红色警告文案。
- `console/src/api/index.ts`：list 参数扩展 + refs 方法。

## 实现步骤

1. store：+4 列迁移 + List 查询参数化；单测覆盖迁移幂等与筛选。
2. `attachment_image.go`：实现 `optimizeImage(src []byte, mime string) (main []byte, thumb []byte, w, h int, outMime string, err error)` 纯函数 + 单测（用 testdata 小图，断言输出 mime、缩略宽度、超阈值才压缩）。
3. upload/delete/list/refs 四个 handler 改造与新增。
4. Console 列表改造（缩略图、搜索筛选、引用展示、删除警告）。
5. 兼容性回归：**存量附件**（无 thumb、宽高为 0）列表正常回退原图展示，不报错。

## 技术决策（无需再问）

- **纯 Go 无 CGO**：保持单二进制交叉编译能力（现有 Docker 构建链），禁用 libwebp CGO 绑定。
- **不保留原图**：上传即优化是终态，Console 不提供「下载原图」概念；用户要原图自己本地留档。
- **缩略图命名**：与主图同目录，`thumb_` 前缀，URL 同规律，不单独建目录。
- **引用扫描实时 LIKE**：单人博客文章量级（<1k）下实时扫描足够，不建引用关系表、不做反向索引；扫描字段为 markdown 原文 content（图床 URL 以 `/uploads/` 开头，无歧义）。
- **动图与矢量**：gif/svg 直接透传（nativewebp 不处理动图；svg 无需位图化）。
- **不做**：附件分组/文件夹、重命名、图片编辑裁剪、外部对象存储（S3 等）。

## Impact

- **新增**：`internal/api/attachment_image.go`（+ testdata）
- **修改**：`internal/store/attachment.go`、`internal/api/attachment.go`、`internal/api/router.go`（+1 行）、`console/src/modules/attachments/AttachmentList.vue`、`console/src/api/index.ts`、`go.mod`
- **不影响**：post/page store、模板、前台路由、Markdown 编辑器上传调用方（接口签名不变，返回体新增字段向后兼容）

## ADDED Requirements

### Requirement: 附件优化与管理增强
The system SHALL optimize uploaded images (compress/WebP/thumbnail), expose reference usage, and support attachment search/filter.

#### Scenario: 大图上传自动优化
- **WHEN** 上传一张 4MB 的 JPEG 照片
- **THEN** 落盘为明显更小的 WebP（或降级 JPEG），返回 URL 指向优化后文件，宽高元数据正确

#### Scenario: 小图与特殊格式透传
- **WHEN** 上传 50KB 小图 / gif / svg
- **THEN** 文件原样保存，不被重编码

#### Scenario: 缩略图列表
- **WHEN** 打开 Console 附件库
- **THEN** 网格加载 thumb 而非原图；存量无 thumb 的附件回退原图不报错

#### Scenario: 引用警告
- **WHEN** 删除一个被 2 篇文章引用的附件
- **THEN** 弹窗列出这 2 篇文章并警告裂图，确认后附件与缩略图文件均被删除

#### Scenario: 搜索筛选
- **WHEN** 输入文件名关键字并选「仅图片」
- **THEN** 列表服务端过滤返回匹配项，分页正确

## 验证

1. `go build ./... && go vet ./... && go test ./internal/api/ ./internal/store/` 通过；旧库启动自动加列
2. 上传 4MB JPEG / 100KB PNG / gif / svg / pdf 各一：优化行为符合分支预期，编辑器插入的 URL 可正常显示
3. Console 列表缩略图加载（Network 面板确认请求的是 thumb）、搜索筛选、引用列表、删除连带清理 thumb 文件
4. Markdown 编辑器粘贴图片直传链路回归正常（返回体兼容）
5. 亮/暗模式下附件库 UI 正常

## Out of Scope

- 附件分组/文件夹与批量操作
- 外部存储（S3/OSS）与 CDN 集成
- 图片编辑（裁剪/旋转/水印）
- 上传时的 EXIF 清理（可在后续安全 spec 考虑）
