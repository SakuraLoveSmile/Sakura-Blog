# Spec 27 · 归档页（已实现）+ OG 社交分享图

- **分支**：`feature/archives-og` · **Wave 6** · **隔离等级**：中
- **主模块**：前台 `/archives`（**已存在**）+ 新 `internal/og/` 包（本期核心）
- **状态**：⚠️ **归档页已实现**。本 spec 记录归档现状，核心增量是 **OG 图生成**。

## 现状（归档页已实现，审计确认）

经代码审计，归档页**已完整落地**，无需重做：

- **路由**：`main.go:942` `r.GET("/archives")` —— `ListPublished` 按 `created_at` 年份分组（`ArchiveYear/ArchiveItem`），无标题文章用日期占位（对齐 Stellar）。
- **模板**：`internal/template/partial/archive.html`（`div.post-list.archives`，年份 header + 文章行）。
- **导航**：`navbar.html` 归档 tab（`Layout=="archive"` 高亮）；`site.go` 默认菜单含归档；sitemap 含 `/archives`。
- 缺口：**无 OG 社交分享图**（分享文章到社交平台时无标题卡片图）。

## 核心增量：OG 图生成

为每篇文章生成固定版式的 OG 图（标题 + 站名 + 日期），路由 `/og/:slug.png`，供 `head.html` 的 `og:image` / `twitter:card` 引用。

### 功能需求

1. **PNG 生成**：新 `internal/og/` 包，用 Go 绘制（`github.com/fogleman/gg` 或标准库 `image` + `golang.org/x/image/font`）。版式固定：纯色/渐变背景 + 文章标题（自动换行）+ 站名 + 发布日期。尺寸 1200×630（OG 标准）。
2. **中文字体**：⚠️ **关键约束**——现有 `assets/fonts/lxgw/` 仅是 **woff2 子集 CSS**（按 unicode-range 切分的多份 `.woff2`），**无单一 TTF**。Go 光栅化需 TTF/OTF：
   - 方案 A（推荐）：bundle 一份完整 `LXGWWenKai-Regular.ttf` 到 `assets/fonts/`（embed 进二进制），与前台 lxgw 视觉一致。
   - 方案 B：降级用 Go 内嵌的等宽/无衬线字体（`golang.org/x/image/font/gofont/goregular`），中文显示为方块则不可接受——**中文博客必须方案 A**。
   - **动手前先确认能否取得 LXGW TTF**（开源 OFL 协议）；取不到则与用户确认字体方案。
3. **路由与缓存**：`GET /og/:slug.png` —— 首次请求生成并写磁盘缓存（`data/ogcache/<slug>.png`），后续直读；文章更新时失效（update/delete 删缓存文件）。`Cache-Control` 长缓存。
4. **head.html 接入**：文章页 `<head>` 注入 `og:image`/`twitter:image` 指向 `/og/:slug.png`，`og:title`/`og:description`/`og:type=article`。**需 site.yml 配 `url`（canonical 基址）才能生成绝对 URL**——已有 `Site.URL` 字段。
5. **降级**：字体加载失败/生成失败时返回 1×1 透明 PNG 或跳过 og:image（不 500 影响页面）。

### What Changes

- 新 `internal/og/og.go`：`Generate(post, siteTitle) (png bytes)`、`CachePath(slug)`、`Invalidate(slug)`；字体 `embed`。
- `assets/fonts/`：+`LXGWWenKai-Regular.ttf`（方案 A，embed）。
- `cmd/server/main.go`：+`/og/:slug.png` 路由（append 式，唯一热点）；启动确保 `data/ogcache/` 目录。
- `internal/api/post.go`：update/delete/purge 后 `og.Invalidate(slug)`。
- `internal/template/partial/head.html`：+`og:image`/`twitter:card` meta（文章页）。
- `go.mod`：+`github.com/fogleman/gg`（或仅 `golang.org/x/image`）。

### 实现步骤

1. 确认/获取 LXGW TTF，embed 进 `assets/fonts/`。
2. og 包：1200×630 绘制（背景 + 标题换行 + 站名 + 日期）+ 磁盘缓存 + Invalidate + 单测（生成非空 PNG、尺寸正确、缓存命中、失效重建）。
3. main.go 路由 + post.go 失效挂钩。
4. head.html 注入 og:image（文章页，依赖 Site.URL）。
5. 回归：访问 /og/:slug.png 得正确卡片图；改文章后缓存刷新；分享 meta 正确；无 URL 配置时降级不报错。

### 技术决策（无需再问）

- **归档页不重做**：审计确认已完整实现，本 spec 仅补 OG 图；原计划「/archives 路由与模板」基于未实现假设，作废。
- **Go 光栅化而非 headless 浏览器**：零外部进程依赖，单二进制部署；固定版式无需 HTML/CSS 灵活性。
- **磁盘缓存 + 写时失效**：OG 图生成有成本（字体光栅化），缓存到文件；文章变动主动失效，避免 stale。
- **必须 bundle 中文 TTF**：lxgw 现有 woff2 子集无法被 Go 直接光栅化，中文 OG 图离不开完整 TTF（OFL 开源可嵌入）。
- **不做**：OG 图模板自定义、封面图合成、按分类配色、动态字体子集化。

## Impact

- **新增**：`internal/og/og.go`、`assets/fonts/LXGWWenKai-Regular.ttf`、`/og/:slug.png` 路由
- **修改**：`cmd/server/main.go`（+路由）、`internal/api/post.go`（+失效）、`head.html`（+og meta）、`go.mod`
- **不影响**：归档页（已完成）、前台其余路由模板、Console

## ADDED Requirements

### Requirement: OG 社交分享图
The system SHALL generate a per-post OG card image (title + site name + date) served at /og/:slug.png with disk caching, and reference it in article head meta.

#### Scenario: 生成与缓存
- **WHEN** 首次请求 /og/:slug.png
- **THEN** 生成 1200×630 PNG 并写磁盘缓存；再次请求直读缓存

#### Scenario: 写时失效
- **WHEN** 文章被更新/删除
- **THEN** 对应 OG 缓存失效，下次请求重新生成

#### Scenario: head 接入
- **WHEN** 渲染文章页且配置了站点 URL
- **THEN** head 注入指向 /og/:slug.png 的 og:image/twitter:image

#### Scenario: 降级容错
- **WHEN** 字体缺失或生成失败
- **THEN** 返回占位图或跳过 og:image，文章页正常渲染不 500

## 验证

1. `go build ./... && go vet ./... && go test ./internal/og/` 通过
2. 访问 /og/:slug.png：得到含中文标题/站名/日期的 1200×630 卡片图，中文正常显示
3. 改文章标题后缓存刷新，图随之更新；删除文章缓存清理
4. 文章页 head 含正确 og:image（配 Site.URL）；社交平台分享预览正常
5. 归档页 /archives 回归正常（既有功能不受影响）

## Out of Scope

- 归档页重做（已实现）
- OG 图模板/版式自定义、封面图合成、按分类配色
- 动态字体子集化、headless 浏览器截图方案
