# Spec 21 · GitHub Widgets（ghcard 代理缓存 + ghrepo/ghuser 卡片）

- **分支**：`feature/gh-widgets` · **Wave 5** · **隔离等级**：高
- **主模块（独占）**：新 `internal/github/` 包 + `cmd/server/main.go`（+路由数行）
- **建议合并顺序**：**先于 Spec 20**（20 的 ghcard 插件消费本 spec 的代理端点）

## Why

原版 Stellar 的 `{% ghcard %}` 直接热链 `github-readme-stats` 外部 SVG（`api_host.ghcard` 配置），国内访问不稳且无缓存。同时原版还有 `ghrepo`/`ghuser`（`scripts/tags/lib/` 无独立文件，属 link 卡片的 GitHub 特化，走 GitHub REST API 取 star/fork/头像）。本 spec 落一个小而全的 GitHub 数据层：**服务端代理 + 磁盘缓存 + 两个原生卡片渲染**，让 20 的 ghcard 与文章内 GitHub 引用都走本地。

## 功能需求

1. **ghcard SVG 代理**：`GET /api/gh/card?username=x[&repo=y&theme=z...]` → 服务端拉取 github-readme-stats SVG → 缓存（磁盘 + 内存，TTL 24h）→ 透传 `image/svg+xml`。上游失败时返回上次缓存（stale-while-error），无缓存则 404。
2. **repo 元数据接口**：`GET /api/gh/repo?repo=user/name` → GitHub REST `GET /repos/{owner}/{repo}` → 裁剪字段（name/description/stars/forks/language/license/owner.avatar_url）→ JSON，缓存 TTL 6h。
3. **ghrepo 标签插件**：`{% ghrepo user/repo %}` → 服务端渲染时同步调上述缓存层（渲染时机在 seed/保存，非请求时），产出原生 HTML 卡片（星标数、语言色点、描述），结构对照原版 link 卡片 + `.tag-plugin.ghcard` 已有样式，缺样式补 `tag-plugins.css`。
4. **ghuser 标签插件**：`{% ghuser username %}` → `GET /users/{username}` 缓存 → 头像 + bio + followers 卡片。
5. **降级**：GitHub API 不可达/限流时，插件渲染为普通链接卡片（仅 URL 文本），不阻塞文章保存。

## What Changes

- 新 `internal/github/`（独立包，零依赖现有模块）：
  - `client.go`：REST 拉取 + token 可选（config `github.token` 空则匿名 60 req/h，够单人博客）+ 内存缓存（map + RWMutex + 过期时间）+ 磁盘持久化（`data/ghcache/*.json`，重启不丢）。
  - `handler.go`：`/api/gh/card`、`/api/gh/repo` 两个公开 handler。
- `cmd/server/main.go`：+3 行（构造 + 2 路由注册）。
- `internal/post/shortcode.go`：+2 正则（ghrepo/ghuser）+ render 函数；render 需要访问 github client —— 通过 `processShortcodes` 增加一个可选依赖注入（对齐 iconFunc 的注入模式），**+1 参数**（Spec 20 同文件，合并时注意）。
- `internal/config/`：app.yml 增 `github.token`（可选）与 `github.card_host`（默认 `github-readme-stats.vercel.app`）。
- `assets/css/tag-plugins.css`：ghrepo/ghuser 卡片缺失样式（先 grep main.css 的 `.ghcard` 系列确认）。

## 实现步骤

1. `internal/github/client.go`：Get(url) → 内存缓存查 → 磁盘缓存查 → 上游拉取 → 双写缓存。单测覆盖 TTL 过期与 stale 回退（httptest 假上游）。
2. `handler.go` 两个端点 + main.go 注册。
3. shortcode ghrepo/ghuser：注入 client，渲染失败降级纯链接。
4. 样式核对与补齐。
5. 测试文 `content/posts/gh-widgets-test.md`（ghcard/ghrepo/ghuser 各一）。

## 技术决策（无需再问）

- **渲染时机**：ghrepo/ghuser 在 Markdown 渲染时（保存/seed）同步取数，产出静态 HTML 快照；不做前端异步刷新。数据陈旧可接受（重存文章即刷新），换来零前端 JS 与首屏无闪烁。
- **card SVG 走请求时代理**：SVG 由 `<img>` 请求时才拉，与 repo 元数据策略不同——SVG 体积大且 stats 服务生成慢，请求时拉 + 长缓存最划算。
- **缓存目录**：`data/ghcache/`，加入备份忽略清单（若备份逻辑有 include 清单则不动）。
- **不做**：GraphQL、贡献热力图、pinned repos 列表、rate-limit 智能调度。

## Impact

- **新增**：`internal/github/`（client.go + handler.go + 单测）、`data/ghcache/`
- **修改**：`cmd/server/main.go`（+3 行）、`internal/post/shortcode.go`（+2 插件，与 Spec 20 同文件需协调合并顺序）、`internal/config/app.go` + `app.yml`、`tag-plugins.css`
- **不影响**：store、api 管理端、console、模板

## ADDED Requirements

### Requirement: GitHub 数据代理与卡片
The system SHALL proxy GitHub stats cards with caching and render ghrepo/ghuser cards server-side with graceful degradation.

#### Scenario: ghcard 代理命中缓存
- **WHEN** 页面二次请求同一 `/api/gh/card?username=xaoxuu`
- **THEN** 直接返回缓存 SVG，不请求上游；重启服务后磁盘缓存仍有效

#### Scenario: 上游失效降级
- **WHEN** github-readme-stats 不可达且存在过期缓存
- **THEN** 返回过期缓存（stale）而非报错

#### Scenario: ghrepo 卡片渲染
- **WHEN** 文章含 `{% ghrepo xaoxuu/hexo-theme-stellar %}` 且 API 正常
- **THEN** 输出含 star/fork 数与描述的静态卡片；API 失败时输出纯链接卡片，保存不报错

## 验证

1. `go build ./... && go vet ./... && go test ./internal/github/` 通过
2. 断网/mock 上游验证 stale 降级与纯链接降级两条路径
3. `data/ghcache/` 重启后缓存生效（日志确认未走上游）
4. 亮/暗模式卡片目检

## Out of Scope

- GitHub 贡献热力图 / pinned repos
- token 的 Console 配置界面（直接改 app.yml）
- 其它平台卡片（gitee/gitlab）
