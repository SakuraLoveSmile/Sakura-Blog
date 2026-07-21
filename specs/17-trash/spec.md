# Spec 17 · 文章/页面回收站（软删除）

- **分支**：`feature/trash` · **Wave 4** · **隔离等级**：中
- **主模块**：`internal/store/post.go`（+列）+ `internal/store/page.go`（+列）+ `internal/api/post.go`/`page.go` + console posts/pages 模块

## Why

当前 `Store.Delete` / `DeletePage` 均为 `DELETE FROM` 硬删除，Console 删除按钮一点即永久丢失，无任何反悔余地。Halo 的核心后台体验之一是**回收站**：删除先进回收站，可恢复、可彻底删除。本 spec 为 posts 与 pages 补软删除全链路。

## 功能需求

1. **软删除**：Console 列表/编辑器点「删除」→ 文章/页面进入回收站（不再出现在前台与后台正常列表）。
2. **回收站视图**：Console 文章/页面列表新增「回收站」状态筛选，展示已删条目及删除时间。
3. **恢复**：回收站条目可一键恢复，回到删除前的 status（draft/published）。
4. **彻底删除**：回收站条目可永久删除（二次确认），才真正 `DELETE FROM`。
5. **前台隔离**：已删文章/页面在首页、归档、分类/标签/专栏、搜索、RSS/Atom/JSON Feed、sitemap 中一律不可见，slug 直达返回 404。

## What Changes

- `internal/store/post.go`：
  - `Post` struct + `DeletedAt time.Time`（零值 = 未删除）。
  - `migrateColumns` + `deleted_at TEXT NOT NULL DEFAULT ''`；列清单/scan 同步。
  - 现有全部查询（`List`/`ListPublished*`/`ListByCategory|Tag|Topic`/`CountBy*`/`GetPrevPost`/`GetNextPost`/`ListRelatedByTags`/`getBySlug` 等）统一追加 `AND deleted_at=''`。
  - `Delete(slug)` 改为 `UPDATE posts SET deleted_at=now`；新增 `ListTrashed()`、`Restore(slug)`、`Purge(slug)`（真删）。
- `internal/store/page.go`：同款四件套（+列、查询过滤、`DeletePage` 改软删、`ListTrashedPages`/`RestorePage`/`PurgePage`）。
- `internal/api/post.go` + `router.go`：
  - `GET /api/v1/posts?status=trashed` 返回回收站列表（复用现有 status 筛选参数）。
  - `DELETE /posts/:slug` 语义改为软删；新增 `POST /posts/:slug/restore`、`DELETE /posts/:slug/purge`。
  - pages 同款三个端点。
- `console/src/modules/posts/PostList.vue` / `pages/PageList.vue`：
  - 状态筛选加「回收站」项；回收站态下行操作变为「恢复 / 彻底删除」，彻底删除弹二次确认。
  - 正常态「删除」文案与确认弹窗改为「移入回收站」。
- `console/src/api/index.ts`：+restore/purge 请求方法。

## 实现步骤

1. store 层：post.go 加列 + 迁移 + 全查询过滤 + 四个新方法；page.go 同款。**先写 store 单测**（软删后各 List 不可见、restore 恢复、purge 真删）。
2. api 层：改 delete 语义、加 restore/purge 路由与 handler，trashed 状态接入 listPosts/listPages 的 status 筛选。
3. console：列表筛选 + 行操作 + api 客户端方法 + 二次确认弹窗（样式对齐现有确认交互）。
4. 全站口径回归：逐一确认 feed/sitemap/search.json/prev-next/related 走的都是已过滤查询（它们复用 store 方法即自动生效，逐个 grep 确认无裸 SQL）。

## 技术决策（无需再问）

- **deleted_at 存储**：RFC3339 字符串，空串=未删除，与 `publish_at` 处理方式一致；不用 SQLite NULL（省 scan 分支）。
- **过滤位置**：在 store 查询层统一加条件，不在 handler 层散落判断；不动 `Upsert`（seed 导入不清 deleted_at，已删文章 seed 重导不复活——`Upsert` 按 slug 冲突更新时**保留** deleted_at 原值）。
- **唯一冲突**：回收站内文章占用 slug 属预期行为；新建同 slug 文章会撞唯一约束，报错文案提示「回收站中存在同名 slug」，不做自动改名。
- **不做**：自动清空（如 30 天）、批量恢复/清空按钮（后续如需再加）、moments/friends/attachments 的回收站。
- **依赖协调**：与 Spec 22（版本历史）都动 `store/post.go`，本 spec 先行合并，22 在其后 rebase。

## Impact

- **修改**：`internal/store/post.go`、`internal/store/page.go`、`internal/api/post.go`、`internal/api/page.go`、`internal/api/router.go`、`console/src/modules/posts/PostList.vue`、`console/src/modules/pages/PageList.vue`、`console/src/api/index.ts`
- **不影响**：shortcode、模板、feed/sitemap 代码本身（经 store 过滤自动生效）、comments、attachments

## ADDED Requirements

### Requirement: 回收站
The system SHALL soft-delete posts and pages into a trash bin, supporting restore and permanent purge.

#### Scenario: 移入回收站
- **WHEN** Console 删除一篇已发布文章
- **THEN** 文章从前台所有入口（列表/归档/RSS/sitemap/搜索/直达 URL）消失，出现在回收站列表

#### Scenario: 恢复
- **WHEN** 在回收站点击恢复
- **THEN** 文章回到删除前状态（published 恢复后前台立即可见）

#### Scenario: 彻底删除
- **WHEN** 在回收站二次确认彻底删除
- **THEN** 数据库记录被物理删除，slug 可被新文章复用

#### Scenario: seed 不复活
- **WHEN** 已删文章的源 md 仍在 content/posts 且执行 -seed
- **THEN** 该文章保持已删除状态，不重新上线

## 验证

1. `go build ./... && go vet ./... && go test ./internal/store/` 通过；旧库启动自动加列（迁移幂等）
2. 删除已发布文章：首页/归档/分类/标签/`/search.json`/`/rss.xml`/`/sitemap.xml`/slug 直达全部不可见（404）
3. 恢复后上述入口全部恢复；prev/next、相关文章不出现已删条目
4. 彻底删除后 DB 无记录；page 同链路重复以上验证
5. Console 亮/暗模式下回收站筛选与行操作样式正常

## Out of Scope

- 定时自动清空回收站
- moments/friends/attachments 软删除
- 文章版本历史（Spec 22）
