# Spec 25 · 站内搜索增强（SQLite FTS5）

- **分支**：`feature/search-fts` · **Wave 6** · **隔离等级**：中
- **主模块（独占）**：新 `internal/search/` 包 + `/api/search` 端点 + 前端搜索适配
- **前置**：无。`/search.json` 与 `local-search.js` 保留作降级路径。

## Why

现有站内搜索是**纯前端全文匹配**：`/search.json` 导出全量文章标题+正文纯文本，`local-search.js` 在浏览器里逐条 `indexOf`。文章上百篇后索引体积大、加载慢、无相关性排序、无摘要高亮，中文分词只能靠子串硬匹配。升级到 **SQLite FTS5 服务端全文索引**：增量同步、相关性排序（BM25）、snippet 摘要高亮，搜索体验质的提升，且 zero 依赖（modernc.org/sqlite 已内置 FTS5）。

## 现状基线（已实现，本期保留作降级）

- `GET /search.json`（`main.go:754`）：全量 published 文章 `{title, content, path}`，受保护文章正文置空，`no-store`。
- `assets/js/search/local-search.js`：拉 `/search.json` + localStorage 缓存 + 前端子串匹配高亮。
- `base.html` 注入 `ctx.search = { service:'local_search', path:'/search.json' }`。
- 缺口：**无服务端索引**、**无相关性排序**、**无服务端 snippet**、大库下前端匹配慢。

## 功能需求

1. **FTS5 虚表**：`posts_fts`（title, content, slug UNINDEXED），`tokenize='unicode61'`（中文按字切分，够用；不引外部分词器）。
2. **增量同步**：文章 create/update/delete/restore/purge 时同步维护 FTS 索引（钩入 store 写路径）。受保护文章**不入索引正文**（仅标题或整体排除，对齐 Spec 15 口径）。
3. **服务端搜索 API**：`GET /api/search?q=关键词&limit=20` → `{items:[{title, path, snippet}]}`，按 `bm25(posts_fts)` 排序，snippet 用 FTS5 `snippet()` 函数生成（含 `<mark>` 高亮）。
4. **前端适配**：`local-search.js` 优先走 `/api/search`（fetch），失败/空 q 时**降级回 `/search.json`** 本地匹配（保留现有逻辑作 fallback）。
5. **全量重建**：启动时若 FTS 表为空而 posts 非空，触发一次性全量回填（幂等）。
6. **不破坏现有**：`/search.json` 端点保留，搜索 UI 交互（弹窗/高亮/noresult）不变。

## What Changes

- 新 `internal/search/search.go`：
  - `EnsureFTS()` 建虚表 + 触发器或显式同步；`Rebuild(posts)` 全量回填；`Index(post)`/`Unindex(slug)` 增量；`Search(q, limit) []Result`（bm25 + snippet）。
  - 与 store 解耦：search 包持有 `*sql.DB` 或由 store 在写操作后回调。**动手前先读 store/post.go 写路径确认挂钩点**。
- `internal/store/post.go`：create/update/delete/restore/purge 各 +1 行同步调用（或暴露 hook）。
- `cmd/server/main.go`：+`/api/search` 路由 + 启动时 `EnsureFTS` + 空表回填（append 式，唯一热点）。
- `assets/js/search/local-search.js`：搜索函数改为先 fetch `/api/search`，渲染服务端 snippet；catch 降级本地匹配。
- 受保护文章过滤：复用 `IsPasswordProtected()` 判断，不入 FTS 正文。

## 实现步骤

1. search 包：FTS 虚表 + Rebuild/Index/Unindex/Search + 单测（中文查询命中、bm25 排序、snippet 高亮、受保护排除、空 q 返回空）。
2. store 挂钩：5 个写路径同步索引。
3. main.go：路由 + 启动回填。
4. 前端：local-search.js 接 /api/search + 降级。
5. 回归：搜中文词 → 服务端 snippet 高亮返回；停服 FTS（模拟）→ 降级 /search.json 仍可用；受保护文章搜不到正文。

## 技术决策（无需再问）

- **unicode61 而非 jieba 分词**：零依赖优先；中文按字切分对博客搜索召回足够，BM25 排序补精度。引外部分词器（gojieba）增加构建复杂度，不值。
- **保留 /search.json 降级**：渐进增强，搜索可用性不因服务端索引故障归零；旧缓存客户端无感。
- **snippet() 服务端高亮**：前端不再自己切摘要，统一由 FTS 给 `<mark>` 包裹的上下文片段。
- **受保护文章排除正文**：与 Spec 15 一致，密码内容不进任何搜索出口。
- **不做**：拼音搜索、模糊纠错、搜索建议/自动补全、分页深翻（limit 20 封顶）。

## Impact

- **新增**：`internal/search/search.go`、`/api/search` 端点
- **修改**：`internal/store/post.go`（写路径 +同步）、`cmd/server/main.go`（+路由+回填）、`assets/js/search/local-search.js`（接服务端+降级）
- **不影响**：前台搜索 UI 结构、`/search.json` 端点（保留）、其余模板路由

## ADDED Requirements

### Requirement: 站内搜索增强（FTS5）
The system SHALL maintain a SQLite FTS5 index of published posts and serve ranked, snippet-highlighted search results, with graceful fallback to the legacy client-side search.

#### Scenario: 服务端搜索与排序
- **WHEN** 提交搜索关键词
- **THEN** 返回按 BM25 相关性排序的结果，每条含 `<mark>` 高亮 snippet

#### Scenario: 增量同步
- **WHEN** 文章被创建/更新/删除/恢复/彻底删除
- **THEN** FTS 索引随之更新，搜索结果实时反映

#### Scenario: 受保护文章排除
- **WHEN** 搜索命中密码保护文章
- **THEN** 其正文不出现在索引与 snippet 中

#### Scenario: 降级可用
- **WHEN** 服务端搜索不可用或关键词为空
- **THEN** 前端降级回 /search.json 本地匹配，搜索不中断

## 验证

1. `go build ./... && go vet ./... && go test ./internal/search/` 通过
2. 搜中文词：服务端返回 bm25 排序 + snippet 高亮；新发文立即可搜
3. 受保护文章正文搜不到；删除文章后从结果消失
4. 模拟 /api/search 失败：前端降级 /search.json 仍可搜
5. 旧库启动自动建 FTS 表并全量回填，幂等不重复

## Out of Scope

- 拼音/模糊纠错/搜索建议/自动补全
- 外部分词器（jieba）集成
- 搜索结果分页深翻、评论/页面纳入搜索（本期仅文章）
