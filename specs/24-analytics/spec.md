# Spec 24 · 访问统计分析（PV/UV 趋势 + 热门榜）

- **分支**：`feature/analytics` · **Wave 6** · **隔离等级**：中
- **主模块（独占）**：新 `internal/store/pageview.go` + 新埋点中间件 + Console 仪表盘图表
- **前置**：无硬前置。复用已有 `posts.views` 计数（Spec 10）与 `dashboard/stats` 端点。

## Why

目前只有「文章总阅读量」一个粗粒度数字（`posts.views` 累加 + `SumViews`），无法回答「最近一周每天多少人看」「哪篇最近最火」「流量从哪来」。单人博客不引第三方统计（隐私 + 依赖），需要一套**自托管的轻量访问明细 + 趋势聚合**，在 Console 仪表盘看到 PV/UV 折线与热门文章榜。

## 现状基线（已实现，本期复用/扩展）

- `posts.views` 列 + `IncrViews/GetViews/SumViews/ListPopular`（`store/post.go`、`store/stats.go`）：文章级累计阅读，`/post/:slug` 命中时 +1。
- `GET /api/v1/dashboard/stats`（`api/dashboard.go`）：返回 post/comment/view/attachment 计数。
- 缺口：**无访问明细表**（无法按天聚合、无 referrer/UA）、**无趋势 API**、**仪表盘无图表**。

## 功能需求

1. **访问明细记录**：新 `page_views` 表（`id, path, referrer, ua_hash, created_at`）。Gin 中间件仅记录**文章页 `/post/:slug` 与独立页面 `/`、`/notes` 等 GET**（排除 /assets、/api、/console、静态资源与机器人 UA）。
2. **UV 去重**：`ua_hash = fnv(UA + IP)`（不存明文 IP/UA，隐私友好）；同 `ua_hash` 同 `path` 同自然日算 1 UV。
3. **趋势聚合 API**：`GET /api/v1/stats/trend?days=7|30` → 每日 `{date, pv, uv}` 序列。
4. **热门文章榜**：`GET /api/v1/stats/popular?days=7&limit=10` → 区间内按 PV 排序的文章（path → 标题解析）。
5. **来源统计**：`GET /api/v1/stats/referrers?days=7` → referrer 域名 Top N（可选，二期可延后）。
6. **仪表盘图表**：Console Dashboard 新增 PV/UV 折线图（近 7/30 天切换）+ 热门文章榜列表。**纯前端绘制**（SVG polyline，不引图表库）。
7. **不引第三方**：无 GA/Umami/百度统计；全部自托管 SQLite。

## What Changes

- 新 `internal/store/pageview.go`：
  - `page_views` 表 + 索引 `(created_at)`、`(path, created_at)`。建表走独立 `EnsurePageViewsTable`（对齐 revision.go 模式）。
  - 方法：`RecordView(path, referrer, uaHash)`、`TrendPVUV(days) []DailyStat`、`PopularByViews(days, limit) []PopularStat`、`TopReferrers(days, limit)`、`PrunePageViews(keepDays)`（清理超期明细，默认留 90 天）。
- `cmd/server/main.go`：+1 中间件 `trackPageView(st)`，挂在文章/页面 GET 路由前（append 式，唯一热点）。
- `internal/api/stats.go`（新文件）：3 端点（trend/popular/referrers）。
- `internal/api/router.go`：+3 行（authed 组）。
- Console：
  - `console/src/api/index.ts`：+`statsApi`（trend/popular）。
  - `console/src/modules/dashboard/DashboardView.vue`：+折线图组件（SVG）+ 热门榜 + 7/30 天切换。

## 实现步骤

1. store：page_views 表 + 5 方法 + 单测（UV 同日去重、跨日累加、trend 空日补零、prune 边界）。
2. 中间件：path 白名单 + 机器人过滤 + uaHash；挂路由。
3. api：3 端点 + router。
4. Console：SVG 折线图（PV/UV 双线）+ 热门榜；7/30 切换。
5. 回归：刷 N 次文章页 → PV=N、同 UA 同日 UV=1；切天 → UV 重新计；趋势图渲染。

## 技术决策（无需再问）

- **明细表 + 聚合查询**而非预聚合表：单人博客日 PV 量级 <1k，90 天明细 <10w 行，`GROUP BY date(created_at)` 实时聚合无压力；prune 控制上限。
- **ua_hash 不存明文**：隐私优先，UV 只需去重能力，无需还原身份。
- **SVG 手绘折线**：数据点 ≤30，polyline + 坐标换算 ~80 行 TS，零依赖。
- **path 白名单制**：只记内容页 GET，避免资源/后台请求污染统计。
- **不做**：IP 地理位置解析、实时在线人数、会话漏斗、按设备/浏览器细分面板。

## Impact

- **新增**：`internal/store/pageview.go`、`internal/api/stats.go`、Console 仪表盘图表组件
- **修改**：`cmd/server/main.go`（+中间件）、`internal/api/router.go`（+3 行）、`console/src/api/index.ts`、`DashboardView.vue`
- **不影响**：前台模板与路由、`posts.views` 既有逻辑（并存：views 是文章级累计，page_views 是明细趋势）

## ADDED Requirements

### Requirement: 访问统计分析
The system SHALL record content-page views into a detail table and expose PV/UV trend, popular posts, and referrer aggregations for the Console dashboard.

#### Scenario: PV/UV 记录与去重
- **WHEN** 同一访客同日多次访问同一文章
- **THEN** PV 逐次累加，UV 该日该路径恒计 1

#### Scenario: 趋势聚合
- **WHEN** 请求近 7 天趋势
- **THEN** 返回 7 个自然日的 {date, pv, uv}，无访问日补零

#### Scenario: 仪表盘图表
- **WHEN** 打开 Console 仪表盘并切换 7/30 天
- **THEN** PV/UV 折线与热门文章榜随之刷新

#### Scenario: 明细裁剪
- **WHEN** 明细超过保留期（默认 90 天）
- **THEN** 超期记录被清理，聚合不受影响

## 验证

1. `go build ./... && go vet ./... && go test ./internal/store/` 通过
2. 刷文章页 N 次：PV=N、同 UA 同日 UV=1；换 UA/换日 UV 增加
3. 仪表盘折线图渲染正确，7/30 切换生效；热门榜按区间 PV 排序
4. /assets、/api、/console 请求不计入统计

## Out of Scope

- IP 地理位置解析、实时在线人数、会话/漏斗分析
- 设备/浏览器/操作系统细分报表
- 数据导出（CSV）、第三方统计对接
