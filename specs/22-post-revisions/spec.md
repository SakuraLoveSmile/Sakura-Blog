# Spec 22 · 文章版本历史（快照 / 对比 / 回滚）

- **分支**：`feature/post-revisions` · **Wave 5** · **隔离等级**：中
- **主模块（独占）**：新 `internal/store/revision.go` + 新 api 端点 + Console 新 `RevisionPanel`
- **前置**：**Spec 17（回收站）已合并**——post 表迁移模式已稳定，updatePost 流程无并行改动方

## Why

对齐 Halo 的文章版本管理：目前误改/误删正文只能靠 zip 备份翻档，粒度是「整库」而非「单篇」。加上定时发布、密码文章等长期维护型内容的存在，需要**每次保存自动留快照，可对比差异、可一键回滚**。这是 Console 内容安全的最后一块。

## 功能需求

1. **自动快照**：每次 `PUT /api/v1/posts/:slug` 保存成功后，把**保存前**的旧版本（title/excerpt/content_md/tags/category/status）写入 revisions 表。内容无变化（content_md + title 均相同）时跳过，不产生空版本。
2. **数量上限**：每篇保留最近 **20** 个版本，超出删最旧（保存时顺手清理）。
3. **版本列表**：Console 文章编辑页侧栏/抽屉展示版本列表（时间 + 标题 + 字数变化 ±N）。
4. **差异对比**：选中某版本 → 与**当前内容**逐行 diff 展示（新增绿/删除红），对比对象是 content_md 原文。
5. **回滚**：确认后用该版本内容覆盖当前文章（回滚本身也走保存流程 → 产生新快照，链条不断）。
6. **页面（pages）不做**：pages 内容低频变动，本期仅 posts。

## What Changes

- 新 `internal/store/revision.go`：
  - `revisions` 表：`id, post_slug, title, excerpt, content_md, tags, category, status, created_at`。索引 `(post_slug, created_at)`。建表走独立 `EnsureRevisionsTable`（对齐 attachment.go 模式，不动 createTables）。
  - 方法：`CreateRevision(r)`、`ListRevisions(slug) []RevisionMeta`（不含 content_md，省流量）、`GetRevision(id)`、`prune(slug)`（保留 20）、`DeleteRevisionsByPost(slug)`。
  - slug 变更时（updatePost 支持改 slug 则同步 UPDATE post_slug；不支持则忽略此项——**动手前先读 updatePost 确认**）。
- `internal/api/post.go`：
  - `updatePost` 保存成功后追加快照写入（拿的是本函数开头已查出的 `existing`，零额外查询）。
  - `purgePost` 时连带 `DeleteRevisionsByPost`（回收站恢复则不动）。
- `internal/api/revision.go`（新文件）：
  - `GET /api/v1/posts/:slug/revisions` 列表
  - `GET /api/v1/revisions/:id` 单版本全文
  - `POST /api/v1/revisions/:id/restore` 回滚（内部复用 updatePost 的保存路径，确保重渲染 content_html + 产生新快照）
- `internal/api/router.go`：+3 行。
- Console：
  - `console/src/modules/posts/RevisionPanel.vue`（新组件）：PostEditor 内以抽屉/弹窗挂载，列表 + diff 视图 + 回滚按钮（danger 确认）。
  - diff 渲染：**纯前端实现**逐行 LCS diff（~60 行 TS，不引依赖）；性能上限 20k 行内可接受。
  - `console/src/api/index.ts`：+3 方法。

## 实现步骤

1. store：revisions 表 + 5 方法 + 单测（含 prune 边界：第 21 版删除最旧）。
2. api：updatePost 挂钩 + 3 端点 + router；restore 走完整保存管线（含 shortcode 重渲染）。
3. Console：RevisionPanel 组件（列表 → 详情 diff → 回滚确认）+ PostEditor 挂入口按钮。
4. 回归：保存 3 次 → 3 版本；无变化保存 → 不增版本；回滚 → 内容恢复且多一条新版本；purge 文章 → 版本清空。

## 技术决策（无需再问）

- **存全文不存 delta**：单人博客文章均值 <50KB，20 版 ×1MB 封顶可忽略；全文快照换来回滚零风险。
- **保存前旧值入库**（而非保存后新值）：语义是「我要找回改坏前的样子」；当前值永远在 posts 表里，无需重复存。
- **diff 在前端做**：服务端只给两份原文；避免 Go 引 diff 依赖，且前端可交互（并排/行内切换留扩展位）。
- **回滚即保存**：不做「预览后再确认」二段式；回滚错了再回滚回来（链式快照保证可逆）。
- **不做**：版本命名/备注、pages 版本、自动定时快照、版本间任意两两对比（只做 vs 当前）。

## Impact

- **新增**：`internal/store/revision.go`、`internal/api/revision.go`、`console/src/modules/posts/RevisionPanel.vue`
- **修改**：`internal/api/post.go`（updatePost/purgePost 各 +数行）、`internal/api/router.go`（+3 行）、`console/src/api/index.ts`、`console/src/modules/posts/PostEditor.vue`（+入口按钮与抽屉挂载）
- **不影响**：前台模板与路由、page/moment/attachment、settings

## ADDED Requirements

### Requirement: 文章版本历史
The system SHALL snapshot post content on every effective save, list/diff revisions in Console, and support one-click rollback.

#### Scenario: 保存产生快照
- **WHEN** 修改正文并保存
- **THEN** revisions 新增一条保存前内容的记录；再次原样保存不新增

#### Scenario: 上限裁剪
- **WHEN** 同一篇第 21 次有效保存
- **THEN** 最旧版本被删除，列表恒 ≤20 条

#### Scenario: 差异对比
- **WHEN** 在版本面板选中某历史版本
- **THEN** 展示该版本与当前内容的逐行 diff（增绿删红）

#### Scenario: 回滚闭环
- **WHEN** 点击回滚并确认
- **THEN** 文章内容恢复至该版本、前台页面重渲染生效，且回滚前的「当前内容」成为新快照

#### Scenario: 彻底删除清理
- **WHEN** 回收站中彻底删除文章
- **THEN** 其全部版本记录一并删除

## 验证

1. `go build ./... && go vet ./... && go test ./internal/store/` 通过
2. Console 全链路手测：改→存→列表→diff→回滚→前台确认渲染
3. 旧库启动自动建表；无 revisions 的旧文章打开版本面板显示空态不报错
4. 亮/暗模式下 diff 配色可读

## Out of Scope

- pages / moments 版本历史
- 版本导出、定时快照、任意两版本互相对比
- 编辑器内自动草稿（本地暂存）——另行考虑
