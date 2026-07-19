# Spec 11 · Feed 增强（Atom + JSON Feed + 分类/标签 RSS + robots.txt）

- **分支**：`feature/feeds` · **Wave 1** · **隔离等级**：高
- **主模块（独占）**：新 `internal/feed/` 包

## Why

现只有一个 `/rss.xml`（全站 RSS 2.0）。补齐现代订阅生态：Atom、JSON Feed 1.1、分类/标签维度 feed，以及 `robots.txt`（声明 sitemap）。把 feed 生成从 main.go 内联抽到独立 `internal/feed` 包，便于扩展与复用。

## What Changes

- 新增 `internal/feed/feed.go`：`Handler`（持 `*store.Store`、`*tmpl.Site`），生成器方法：`atom()`（Atom 1.0）、`jsonFeed()`（JSON Feed 1.1）、`categoryRSS(name)`、`tagRSS(name)`、`robots()`。`Register(r)` 挂路由。
- 路由：`/atom.xml`、`/feed.json`、`/categories/:name/rss.xml`、`/tags/:name/rss.xml`、`/robots.txt`。
- `cmd/server/main.go`：`+ feed.New(st, site).Register(r)`（1 行，挨着现有 rss/sitemap 注册处）。
- `internal/template/partial/head.html`（可选 +2 行）：`<link rel="alternate" type="application/atom+xml">` 与 JSON Feed 的 `<link>`（若做则算入 head.html 共享点；也可省略仅提供端点）。
- （可选）复用现有 `/rss.xml`：可将 main.go 内联 RSS 迁入 feed 包统一维护（保持 URL 不变）。

## 技术决策（无需再问）

- **不引第三方 feed 库**：用 `encoding/xml` 生成 Atom、`encoding/json` 生成 JSON Feed（结构简单、可控）。
- **内容**：全站 feed 取 `store.ListPublished()` 前 N（如 20）；分类/标签 feed 取对应列表。正文用 `ContentHTML`（全文）或摘要——默认**全文**（技术博客友好）。
- **绝对 URL**：用 `site.URL` 拼接文章绝对链接；`site.URL` 为空时从请求 Host 推断（对齐现有 `fillSEO` 做法）。
- **缓存头**：feed 端点 `Cache-Control: max-age=600`；`robots.txt` 可缓存更久。
- **robots.txt**：`User-agent: *` + `Allow: /` + `Disallow: /admin` + `Sitemap: {url}/sitemap.xml`。
- **404**：未知分类/标签 feed 返回空 feed 或 404（择一，默认空 feed 200）。

## Impact

- **新增**：`internal/feed/feed.go`
- **修改**：`cmd/server/main.go`（+Register 1 行；可选迁移现有 rss）、`head.html`（可选 +2 link）
- **不影响**：store（只读）、admin、model、前端页面

## ADDED Requirements

### Requirement: 多格式订阅
The system SHALL expose Atom, JSON Feed, per-category/tag RSS, and robots.txt.

#### Scenario: Atom / JSON Feed
- **WHEN** 请求 `/atom.xml` 或 `/feed.json`
- **THEN** 返回符合规范的 feed（正确 content-type、含最新文章绝对链接）

#### Scenario: 分类/标签 feed
- **WHEN** 请求 `/categories/Go/rss.xml`
- **THEN** 返回该分类下文章的 RSS

#### Scenario: robots
- **WHEN** 请求 `/robots.txt`
- **THEN** 返回含 `Disallow: /admin` 与 `Sitemap:` 行的文本

## 验证

1. `go build ./... && go vet ./...` 通过
2. `curl -I /atom.xml /feed.json /robots.txt` 返回 200 + 正确 content-type
3. Atom/JSON Feed 用在线校验器（或结构目视）通过；链接为绝对 URL
4. `/categories/:name/rss.xml`、`/tags/:name/rss.xml` 内容正确
5. 现有 `/rss.xml`、`/sitemap.xml` 不受影响

## Out of Scope

- WebSub/PubSubHubbub 推送、feed 全文分页
- 播客/媒体 enclosure
