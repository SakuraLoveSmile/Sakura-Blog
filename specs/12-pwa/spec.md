# Spec 12 · PWA（离线可用）+ 站点图标

- **分支**：`feature/pwa` · **Wave 1** · **隔离等级**：高
- **主模块（独占）**：新 `assets/manifest.webmanifest` + `assets/js/sw.js` + 图标

## Why

站点无 PWA 能力（不可安装、无离线）、无标准 favicon / apple-touch-icon / manifest。补齐后可「添加到主屏」、离线浏览已访问页、提升 Lighthouse PWA 分。全部为静态资源 + 少量 head/路由接线。

## What Changes

- 新增 `assets/manifest.webmanifest`：`name`/`short_name`/`theme_color`/`background_color`/`display: standalone`/`start_url: /`/`icons[]`。
- 新增 `assets/js/sw.js`：Service Worker，缓存策略——静态资源（`/assets/*`）**cache-first**，页面 **network-first 回退缓存**，`/admin` 与 `/api/*` **不缓存**（bypass）。版本化 cache name 便于更新。
- 新增图标：`assets/img/icon-192.png`、`icon-512.png`、`apple-touch-icon.png`、`favicon.ico`（可由现有 avatar/logo 生成占位）。
- `cmd/server/main.go`：`+` 路由把 SW 暴露在**根作用域**——`/sw.js`（SW 作用域受路径限制，必须从根提供）与 `/manifest.webmanifest`、`/favicon.ico`、`/apple-touch-icon.png`（映射到 assets 或直接返回）。
- `internal/template/partial/head.html`：`+` `<link rel="manifest">`、`<link rel="icon">`、`<link rel="apple-touch-icon">`、`<meta name="theme-color">`（已有 theme-color，勿重复）、以及 SW 注册脚本（`navigator.serviceWorker.register('/sw.js')`）。

## 技术决策（无需再问）

- **SW 根作用域**：SW 控制范围不能超过其 URL 路径，故 `sw.js` 必须由 `/sw.js` 提供（不能只放 `/assets/js/sw.js`）。用 gin 路由 `r.GET("/sw.js", ...)` 读 embed 的 sw.js 返回，`Content-Type: application/javascript`，`Service-Worker-Allowed: /`。
- **不缓存动态/私有**：SW fetch handler 对 `/admin`、`/api/`、非 GET 请求直接 `fetch()` 透传不进缓存。
- **更新策略**：cache 名带版本号（如 `blog-v1`），`activate` 时清旧 cache；`skipWaiting`+`clients.claim` 可选。
- **theme_color**：与 head 现有 `<meta name="theme-color">`（`#f9fafb`/`#000`）一致。
- **图标**：若无设计资源，用站点 avatar 生成纯色占位即可，尺寸务必齐全（192/512 为 PWA 必需）。
- **embed**：新静态文件位于 `assets/`，已被 `//go:embed all:...` 覆盖（确认 `manifest.webmanifest`/`.ico` 不被 embed 忽略；`all:` 前缀含点文件）。

## Impact

- **新增**：`assets/manifest.webmanifest`、`assets/js/sw.js`、`assets/img/icon-*.png`、`favicon.ico`
- **修改**：`cmd/server/main.go`（+根作用域路由）、`head.html`（+manifest/icon/SW 注册）
- **不影响**：store、admin、model、渲染逻辑

## ADDED Requirements

### Requirement: PWA 与图标
The system SHALL be installable as a PWA with offline fallback and provide standard site icons.

#### Scenario: 可安装
- **WHEN** 浏览器访问站点
- **THEN** 提供有效 manifest + 192/512 图标 + SW，满足安装条件

#### Scenario: 离线回退
- **WHEN** 断网后访问已浏览过的页面
- **THEN** SW 从缓存返回该页

#### Scenario: 后台不缓存
- **WHEN** 访问 `/admin` 或 `/api/*`
- **THEN** SW 不缓存、直连网络（避免登录态/数据陈旧）

## 验证

1. `go build ./... && go vet ./...` 通过
2. `curl -I /sw.js` 返回 `application/javascript` + `Service-Worker-Allowed: /`；`/manifest.webmanifest`、`/favicon.ico` 200
3. Chrome DevTools → Application：manifest 无错、SW 已激活、图标齐全
4. 断网后刷新已访问页可离线打开；`/admin` 不被缓存
5. Lighthouse PWA 项通过安装性检查

## Out of Scope

- 推送通知（Web Push）、后台同步
- 离线写评论队列
