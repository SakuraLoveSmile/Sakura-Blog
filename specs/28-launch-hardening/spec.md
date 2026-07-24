# Spec 28 · 上线前加固（部署健壮性 + 外部依赖收口 + 安全基线）

- **分支**：`feature/launch-hardening` · **Wave 7** · **隔离等级**：中
- **主模块**：`cmd/server/main.go`（优雅退出/中间件/路由）+ `internal/api/auth.go`（Cookie/限流）+ `internal/template/partial/head.html`（外部依赖本地化/JSON-LD）+ `Dockerfile`
- **状态**：📋 待开发。全项目上线前审查结论落地，按 P0/P1/P2 分批实施。

## 背景

项目功能完备（Go SSR + SQLite WAL + FTS5 + PWA + Feed + 备份 + OG），工程质量高。上线前审查发现真正的硬伤不多，主要缺口集中在**部署健壮性**（优雅退出、自定义 500）、**外部依赖/HTTPS 配置**（CDN 强依赖、`site.url` 未设时回退 http）、**安全基线**（Cookie 属性、登录限流）。本 spec 不做大改，只做上线前必要收口。

**审查已澄清的误报（不在本 spec 范围）**：
- `gin.Default()` 已内置 Recovery 中间件，panic 不会导致进程崩溃 —— 只需补自定义 500 页（P1 #8）。
- 文章/页面/瞬间正文均由**博主经鉴权后台或 seed 写入**（`api/router.go` 的 `authed` 路由组），非访客 UGC；访客评论已走 `bluemonday.UGCPolicy()` 消毒（`internal/comment/render.go:23-31`）。故 goldmark `WithUnsafe()` 是"自写自渲染"，**非上线阻塞**。
- `fillSidebar` 是每页**固定 ~8-10 次带 LIMIT 查询**（非随数据量增长的真 N+1），SQLite 本地微秒级；仅 `CountByTag()` 被调用两次可去重（P2 #13）。

## P0 · 上线阻塞项（合入 dev 前必须完成）

### P0-1 优雅退出（graceful shutdown）
- **现状**：`cmd/server/main.go:1355` 直接 `r.Run(addr)`，无信号监听。SIGTERM 直接杀进程，进行中请求被中断，SQLite WAL 可能未 checkpoint。
- **改动**：改为显式 `http.Server` + `signal.NotifyContext(ctx, SIGINT, SIGTERM)`；收到信号后 `srv.Shutdown(ctxTimeout)`（超时 30s），再 `st.Close()`（触发 WAL checkpoint）。备份 goroutine（`backupSvc.StartAuto`）随进程退出即可，无需额外协调。
- **文件**：`cmd/server/main.go`（`main()` 末尾）。

### P0-2 生产 `site.url` 校验 + HTTPS 基址
- **现状**：`site.url` 为空时，canonical / OG / sitemap / RSS / feed 全部回退 `"http://" + c.Request.Host`（`main.go:291-296`、`feed.go:44-48`、`main.go:1304-1308`）。搜索引擎会收录成 http，反代下 Host 亦可能不准。
- **改动**：`config.LoadSite` 之后校验 `site.URL`：release 模式（非 `-dev`）为空则 `log.Printf` 显著告警（不 fatal，避免本地零配置起不来）。文档在 `site.example.yml` 强调生产必填 `url: https://域名`。
- **文件**：`cmd/server/main.go`（`site` 加载后）、`config/site.example.yml`（注释强化）。

### P0-3 Cookie 加 Secure + SameSite
- **现状**：`api/auth.go:37,43,81` 与 `main.go:650` 的 `SetCookie(..., secure=false, httpOnly=true)`，且未设 SameSite（依赖浏览器默认 Lax）。
- **改动**：新增内部辅助：`secure` 由 `site.URL` 是否 `https://` 前缀决定；所有 `SetCookie` 传 `secure`；登录/登出/改密/文章解锁前 `c.SetSameSite(http.SameSiteLaxMode)`。HTTPS 下 admin cookie 不再明文可嗅探。
- **文件**：`internal/api/auth.go`、`internal/api/router.go`（如需共享 secure 判定）、`cmd/server/main.go`（文章解锁 cookie）。

### P0-4 登录接口限流
- **现状**：`api/auth.go:15` `/api/v1/auth/login` 无任何限流，admin 密码可被无限暴力尝试。
- **改动**：复用评论已验证的 IP 限流模式（`internal/comment/handler.go:262-277` 的 `allow(ip)` + `sync.Mutex` + map 清理）。登录失败按 IP 计数，短窗口内失败 N 次（如 5 次/5 分钟）返回 429；成功清零。
- **文件**：`internal/api/auth.go`（Handler 加限流状态字段 + 校验）。

## P1 · 强烈建议（上线体验/安全提升明显）

### P1-5 gzip 压缩
- **现状**：无压缩中间件，HTML/CSS/JS 原样传输。
- **改动**：引入 `github.com/gin-contrib/gzip`，对 text/html、css、js、json、xml 启用 `gzip.DefaultCompression`；排除已压缩资源（图片/woff2/png）。注册在静态与路由之前。
- **文件**：`cmd/server/main.go`（中间件）、`go.mod`。

### P1-6 核心外部依赖本地化（墙内可用性）
- **现状**：jQuery/KaTeX/Mermaid/Fancybox/lazyload/flying-pages（`head.html:40,53-55,88`、`base.html:62-71,234,370`）+ 分享图标/头像（`icons.yml:44-47`、`site.example.yml`）全走 jsdelivr/weavatar。CDN 抖动 → 公式不渲染、灯箱失效、图标裂图。
- **改动（分层）**：
  - **必做**：jQuery、vanilla-lazyload、分享图标 4 个品牌 SVG **vendored 到 `assets/`**（走 embed）。分享图标沿用原版 icon 助手的 `/` 前缀 → `<img>` 渲染逻辑（`funcs.go:53-59`），仅把 `icons.yml` 的 URL 改为 `/assets/img/share/*.svg`，**零结构改动**。
  - **可选**：KaTeX/Mermaid/Fancybox 加 `onerror` 降级 + SRI 完整性校验（保留 CDN 但失败有兜底）。
- **文件**：`assets/js/vendor/`（新）、`assets/img/share/`（新）、`internal/template/data/icons.yml`、`head.html`/`base.html`（本地路径 + onerror）。
- **约束**：⚠️ 严格对齐原版 Stellar，图标沿用 `.social-wrap` 既有样式（main.css），不新增自定义。

### P1-7 静态资源长缓存
- **现状**：`main.go:451-456` 对 `/assets/*` 统一 `Cache-Control: no-cache`，尽管带 `?v=` 版本号仍每次回源验证。
- **改动**：带版本号/指纹的 css/js/字体改 `Cache-Control: public, max-age=31536000, immutable`；`sw.js`/`manifest`/无版本号资源保持 no-cache。破缓存继续靠 `?v=` bump。
- **文件**：`cmd/server/main.go`（`/assets` 中间件按路径/后缀分级）。

### P1-8 自定义 500 页
- **现状**：`gin.Default()` 的默认 Recovery 返回裸文本；各 handler 的 `c.String(500, "...: %v", err)` 直接回显内部错误（`main.go` 多处）。
- **改动**：`gin.New()` + `gin.Logger()` + `gin.CustomRecovery(fn)`，`fn` 渲染 error 页（复用 base 布局，新增 `Layout=="500"` 分派或复用 404 结构）；把关键 handler 的 `c.String(500, err)` 收敛为通用文案 + `log.Printf` 详情。
- **文件**：`cmd/server/main.go`（引擎构造 + `render500`）、`internal/template/partial/error500.html`（新，对齐 error404.html 结构）、`base.html`（+500 分派）。

### P1-9 Docker 非 root 运行 + HEALTHCHECK
- **现状**：`Dockerfile:38` runtime 阶段以 root 运行；HEALTHCHECK 仅在 compose 定义。
- **改动**：runtime 阶段 `adduser -D -u 10001 app`，`chown` `/app/data`，`USER app`；加 `HEALTHCHECK CMD wget -qO- http://localhost:8080/ || exit 1`。
- **文件**：`Dockerfile`。

### P1-10 JSON-LD 结构化数据
- **现状**：`head.html:31-36` 有 OG/Twitter Card，无 JSON-LD。
- **改动**：文章页注入 `<script type="application/ld+json">` BlogPosting/Article schema（headline/datePublished/dateModified/image=/og/:slug.png/author/mainEntityOfPage）。依赖 `site.URL`（P0-2）生成绝对 URL。
- **文件**：`internal/template/partial/head.html`（文章页条件块）。

### P1-11 /uploads SVG 防护
- **现状**：`api/attachment.go:27` 允许上传 `.svg`；`main.go:504` `/uploads` 静态托管，恶意 SVG 直接访问可执行内嵌脚本（虽仅 admin 能传）。
- **改动**：`/uploads` 响应加 `Content-Security-Policy: default-src 'none'; style-src 'unsafe-inline'; sandbox` 或对 `.svg` 强制 `Content-Disposition: attachment`；二选一，优先 CSP（不破坏正常图片内联展示）。
- **文件**：`cmd/server/main.go`（`/uploads` 自定义 handler 替换 `r.Static`）。

## P2 · 优化增强（可上线后迭代）

| # | 项 | 位置 | 说明 |
|---|-----|------|------|
| 12 | PWA 离线兜底页 | `assets/js/sw.js:101` | 离线且无缓存时 `throw Error` → 白屏；改为返回内置 `offline.html`（预缓存） |
| 13 | fillSidebar 缓存 + 去重 | `main.go:306-380` | `CountByTag()` 调用 2 次；侧栏数据加 30~60s TTL 内存缓存 |
| 14 | 安全响应头 | 全局中间件 | `X-Content-Type-Options: nosniff`、`X-Frame-Options: SAMEORIGIN`、`Referrer-Policy`、基础 CSP |
| 15 | /search.json 瘦身 | `main.go:894-919` | 已有 FTS `/api/search`，降级端点可只回标题+路径，省带宽 |
| 16 | 补 DB 索引 | `store/post.go:206-207` | `idx_posts_category`、`(status, publish_at)` 复合索引（文章上千后有感） |
| 17 | 图片上传尺寸上限 | `api/attachment.go` | 已有 optimizeImage，补最大宽高约束防超大图 |
| 18 | 请求体上限 + 结构化日志 | `main.go` | 显式 `r.MaxMultipartMemory`；可选 `slog` 支持日志级别 |
| 19 | 字体 font-display | `assets/fonts/lxgw/*.css` | 确保 `font-display: swap`，加载时不遮挡文字 |
| 20 | a11y 补 aria-label | `partial/content.html` | 分享图标 `<img>` 补 `aria-label`（对齐原版结构前提下兼顾无障碍） |

## 实现步骤（建议顺序）

1. **P0 批次**（最小可上线集）：P0-1 优雅退出 → P0-3 Cookie → P0-4 登录限流 → P0-2 site.url 校验。每步 `go build ./... && go vet ./...` + 起服务冒烟。
2. **P1 必做子集**：P1-5 gzip → P1-6 核心依赖本地化（jQuery/lazyload/分享图标）。
3. **P1 其余**：P1-7 长缓存 → P1-8 自定义 500 → P1-9 Docker → P1-10 JSON-LD → P1-11 SVG 防护。
4. **P2**：上线后按周迭代，按上表逐项。

## 技术决策（无需再问）

- **P0-2 告警而非 fatal**：`site.url` 为空仅告警，保留本地 `-dev` 零配置启动能力；生产靠文档 + 告警提醒填写。
- **P0-3 secure 由 site.url 协议推导**：避免新增配置项；`https://` 前缀 → `secure=true`，http/空 → false（本地开发不被 Secure 挡住）。
- **P0-4 复用评论限流模式**：不引入外部限流库，沿用 `comment/handler.go` 已验证的内存 map + Mutex 方案，保持依赖精简。
- **P1-6 分享图标走既有 img 逻辑**：`funcs.go` 的 icon 助手已支持 `/` 前缀渲染 `<img>`，本地化只改 `icons.yml` 的值，不动模板/CSS，严格对齐原版。
- **P1-8 用 CustomRecovery**：Gin 原生能力，不引入第三方；500 页复用 base 布局与 404 同构。
- **不做**：JWT/session 重构（单管理员 HMAC cookie 够用）、密码 bcrypt 化（cookie 签名 key 即密码，改动牵连大，收益低）、CORS（无跨域场景）、Prometheus metrics（个人博客过度设计）、标签规范化关联表迁移（逗号存储在当前规模够用）。

## Impact

- **新增**：`assets/js/vendor/`（jQuery/lazyload）、`assets/img/share/*.svg`、`internal/template/partial/error500.html`、`assets/offline.html`(P2)
- **修改**：`cmd/server/main.go`（优雅退出/中间件/引擎构造/uploads handler/site.url 校验）、`internal/api/auth.go`（Cookie/限流）、`internal/template/data/icons.yml`（分享图标本地路径）、`head.html`/`base.html`（本地依赖/JSON-LD/500 分派）、`Dockerfile`、`config/site.example.yml`、`go.mod`（+gin-gzip）
- **不影响**：文章/评论/搜索/OG/备份等既有功能；Console SPA

## ADDED Requirements

### Requirement: 优雅退出
The system SHALL handle SIGINT/SIGTERM by gracefully shutting down the HTTP server (draining in-flight requests within a timeout) and closing the database before exit.

#### Scenario: 收到终止信号
- **WHEN** 进程收到 SIGTERM
- **THEN** 停止接收新请求，等待进行中请求完成（≤30s），关闭 DB 后退出，不中断活跃请求

### Requirement: 生产站点 URL 基址
The system SHALL warn on startup when running in release mode without a configured site URL, and use the configured HTTPS URL as the canonical base for SEO/feed/OG absolute links.

#### Scenario: release 模式缺 URL
- **WHEN** 非 -dev 模式启动且 site.url 为空
- **THEN** 打印显著告警日志（不中断启动）

### Requirement: 安全 Cookie 属性
The system SHALL set HttpOnly and SameSite=Lax on all auth/session cookies, and set Secure when the site is served over HTTPS.

#### Scenario: HTTPS 下签发登录 Cookie
- **WHEN** site.url 为 https 且登录成功
- **THEN** cookie 带 Secure + HttpOnly + SameSite=Lax

### Requirement: 登录限流
The system SHALL rate-limit failed login attempts per client IP to mitigate brute-force attacks.

#### Scenario: 连续失败超阈值
- **WHEN** 同一 IP 在窗口内登录失败达到阈值
- **THEN** 返回 429，短时间内拒绝该 IP 的后续登录尝试

### Requirement: 响应压缩
The system SHALL compress text responses (HTML/CSS/JS/JSON/XML) using gzip.

#### Scenario: 请求支持 gzip
- **WHEN** 客户端 Accept-Encoding 含 gzip 且响应为文本类型
- **THEN** 响应经 gzip 压缩返回

### Requirement: 核心前端依赖本地化
The system SHALL serve core frontend dependencies (jQuery, lazyload) and share icons from local embedded assets rather than external CDNs.

#### Scenario: CDN 不可达
- **WHEN** 外部 CDN 不可访问
- **THEN** jQuery/lazyload/分享图标仍从本站加载，评论/懒加载/分享图标不失效

### Requirement: 自定义错误页
The system SHALL render a friendly 500 page on server errors/panics without exposing internal error details or stack traces to the client.

#### Scenario: handler 内部错误
- **WHEN** 请求处理发生 panic 或内部错误
- **THEN** 返回友好 500 页面，内部细节仅记录到日志

## 验证

1. `go build ./... && go vet ./...` 双通过；起服务冒烟正常
2. **P0-1**：`kill -TERM <pid>` 时进行中请求正常完成、DB 干净关闭（无 WAL 残留告警）
3. **P0-2**：release 模式空 URL 有告警；配 https URL 后 canonical/OG/sitemap 均为 https 绝对地址
4. **P0-3**：HTTPS 下 DevTools 检查 admin cookie 含 Secure + HttpOnly + SameSite=Lax
5. **P0-4**：脚本连续错误密码达阈值后返回 429；正确密码登录成功且计数清零
6. **P1-5**：响应头含 `Content-Encoding: gzip`，HTML 体积明显下降
7. **P1-6**：断开外网/屏蔽 jsdelivr 后，首页/文章页 jQuery、图片懒加载、分享图标正常
8. **P1-8**：构造错误触发 500，页面为友好错误页，响应体无堆栈/内部路径
9. **P1-9**：`docker run` 后 `whoami` 非 root；`docker ps` 健康检查 healthy
10. **P1-10**：文章页 head 含合法 JSON-LD（Google Rich Results 测试通过）

## Out of Scope

- JWT/session 体系重构、密码 bcrypt 化、CORS 配置
- Prometheus metrics / 分布式追踪 / WAF
- 标签逗号存储 → 关联表规范化迁移
- 页面级 SSR 全量缓存、HTTP/2 server push
- 多语言 i18n / hreflang（当前单语言）
