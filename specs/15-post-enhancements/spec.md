# Spec 15 · 文章增强（置顶 + 密码保护 + 定时发布）

- **分支**：`feature/post-enhancements` · **Wave 3（依赖 05B + Wave2 合并后）** · **隔离等级**：中
- **主模块**：`internal/store/post.go`（+列 + 迁移）+ admin 编辑 + post handler

## Why

文章缺三项常用能力：**置顶**（首页优先展示）、**密码保护**（加密文章需口令阅读）、**定时发布**（到点自动公开）。均需给 posts 表加列并调整查询/handler，属改 `store/post.go` 的收尾项，放最后一波避免与其它 store 改动争抢。

## What Changes

- `internal/store/post.go`：
  - `Post` struct +`Pinned bool`、`Password string`、`PublishAt time.Time`（可空）。
  - `migrateColumns` map +3 列：`pinned INTEGER NOT NULL DEFAULT 0`、`password TEXT NOT NULL DEFAULT ''`、`publish_at TEXT`（或 INTEGER 时间戳）。
  - 列表/首页查询：排序 `ORDER BY pinned DESC, created_at DESC`；「已发布」判定追加 `AND (publish_at IS NULL OR publish_at <= now)`（定时未到不公开）。
  - `commentCols` 式的列清单/扫描同步加 3 列。
- `internal/admin/admin.go` + `admin/edit.html`：编辑表单 +「置顶」勾选、+「密码」输入、+「定时发布时间」；`save` 落库这三字段。
- `cmd/server/main.go` `/post/:slug` handler：
  - **密码 gate**：受保护文章未通过口令时渲染「输入密码」占位（POST 校验，通过后置 cookie/session 放行，或表单提交后渲染正文）。
  - **定时**：`publish_at` 未到的文章前台按未发布处理（404 或不可直达）。
- 首页/归档/RSS/sitemap 的「已发布」口径统一走新判定（定时未到不出现）。

## 技术决策（无需再问）

- **置顶**：布尔列 + 排序前置；首页卡片可加「置顶」角标（复用 badge，用变量色）。
- **密码保护**：口令存 `password` 列（明文或 hash；本项目单人博客可明文，校验用常量时间比较）。校验通过后写短期 cookie（`post_pw_<slug>`，HMAC，仿 admin cookie），避免每次刷新重输。**受保护文章不进 RSS/sitemap/search.json 正文**（仅标题或整体排除，默认排除正文）。
- **定时发布**：`publish_at` 判定在查询层做（`publish_at <= 当前时间`），**无需后台定时任务**——到点后自然被查询命中即公开（惰性）。
- **时间存储**：`publish_at` 存 RFC3339 字符串或 Unix 时间戳，与现有 created_at 处理保持一致。
- **依赖顺序**：在 **05B（admin edit 已 Halo 化）与 Wave2 合并后**开工，避免与它们争 `admin/edit.html`、`store/post.go`。

## Impact

- **修改**：`internal/store/post.go`（+3 列 + 迁移 + 查询）、`internal/admin/admin.go`（save 逻辑）、`admin/edit.html`（+3 字段）、`cmd/server/main.go`（密码 gate + 定时判定）；`internal/feed`/sitemap/search 的已发布口径（若已抽包则改一处）
- **不影响**：comments/media/pages/moments、前端样式体系

## ADDED Requirements

### Requirement: 置顶 / 密码 / 定时
The system SHALL support pinned posts, password-protected posts, and scheduled publishing.

#### Scenario: 置顶排序
- **WHEN** 某文章被标记置顶
- **THEN** 首页/列表中该文排在非置顶之前（置顶内部按时间倒序）

#### Scenario: 密码保护
- **WHEN** 访问受保护文章且未提供正确口令
- **THEN** 显示密码输入，输入正确后展示正文并记住（短期 cookie）；错误则提示

#### Scenario: 定时发布
- **WHEN** 文章 `publish_at` 晚于当前时间
- **THEN** 前台/列表/RSS/sitemap 均不展示；到点后自动可见

#### Scenario: 受保护文章不泄漏
- **WHEN** 文章受密码保护
- **THEN** 其正文不出现在 `/search.json`、RSS 全文与 sitemap 摘要中

## 验证

1. `go build ./... && go vet ./...` 通过；旧库启动自动 `ALTER` 加列（迁移幂等）
2. 置顶一篇，首页顺序正确、角标显示
3. 受保护文章：错口令被拒、对口令放行并记住；`/search.json` 不含其正文
4. 设未来 `publish_at`：前台 404/不出现；改到过去后可见
5. 亮/暗模式下密码页与角标样式正常

## Out of Scope

- 分组权限/会员可见、付费阅读
- 后台定时任务调度器（用惰性查询即可）
