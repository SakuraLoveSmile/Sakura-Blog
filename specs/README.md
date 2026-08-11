# 并行开发 Spec 集（夜间不间断开发）

每个子目录是一个**可并行开发**的功能 spec，遵循根目录 [`AGENTS.md`](../AGENTS.md)：**一个 agent = 一个 feature 分支 = 一个 worktree，从 dev 拉出，做完合回 dev**。

## 当前批次 · Wave 13（上线冲刺）

来源：正式上线前体检（2026-08）。代码面已 ready，剩余工作在配置/内容/部署层。

| # | Spec | 主题 | 优先级 | 主模块 |
|---|------|------|--------|--------|
| 54 | [从 GitHub Friends 仓库导入友链](54-import-friends/spec.md) | console 一键拉取 Friends 仓库 issues 批量导入 links 表 | 中 | `api/import_links.go` + `github/client.go` + `FriendList.vue` |
| 55 | [正式上线准备（脱敏 / 配置转正 / 内容清理 / 部署收尾）](55-launch-prep/spec.md) | PAT 轮换 + site.yml 转正 + 测试文清理 + 反代模板 + 可选备案号 | 高 | `config/`（生产值）+ `content/posts/` + `deploy/` +（可选）`footer.html` |

**零冲突**：54 / 55 无共享文件热点；55 以配置与内容整理为主，唯一可选代码改动是 Part D 备案号字段。

## 最新批次 · Wave 11（前端收官收尾，依赖 Wave 10 已合）

来源：前端 vs 原版 Stellar 收官调研（2026-08，对齐度 ~95% 后的小缺口）+ 站长新增 AI 摘要需求。

| # | Spec | 主题 | 优先级 | 主模块 |
|---|------|------|--------|--------|
| 53 | [前端收尾（作者链接 / swiper 本地化 / wiki 时间）+ AI 文章摘要](53-frontend-finale/spec.md) | Part A 文章头部作者链接 · Part B swiper 本地化 · Part C wiki dateinfo + canonical 处置 · Part D AI 摘要（后台预生成存库 + 读者临时接入自己的 API 透传生成） | 高 | `internal/api/ai.go` + `store/post.go` + `partial/content.html` + 新 `assets/js/ai-summary.js` + Console posts/settings |

**零冲突**：Spec 53 独占全部改动文件，与任何未合分支无共享热点；复用现有 AI 管线（`config.LoadAI` / `callChatCompletion` / settings `ai` 段 / `/ai/generate`）。

## 历史批次 · Wave 10（已完成，存档）

来源：前端 vs 原版 Stellar 排版与标签插件差距收官（代码行号 / grid / 展示容器 / 装饰标签）+ 阅读体验配置 + 内容系统扩展（2026-08）。

| # | Spec | 主题 | 优先级 | 主模块 |
|---|------|------|--------|--------|
| 48 | [排版核心补齐（代码行号 + grid/hashtag/blockquote）](48-typography-essentials/spec.md) | 排版核心补齐（代码行号 + grid/hashtag/blockquote） | 高 | `shortcode.go` + `tag-plugins.css` + Chroma 配置 |
| 49 | [展示容器插件（swiper/banner/quot/chat/navbar/frame）](49-tag-plugins-display/spec.md) | 展示容器插件（swiper/banner/quot/chat/navbar/frame） | 中 | `shortcode.go` + `tag-plugins.css` |
| 50 | [阅读体验与配置（linklist/story/nav_tabs/背景图/评论懒加载）](50-reading-ux-config/spec.md) | 阅读体验与配置（linklist/story/nav_tabs/背景图/评论懒加载） | 中 | widgets + navbar + `site.yml` + `comments.js` |
| 51 | [装饰标签插件（poetry/paper/reel/blur/psw/emp/wavy/sub/folders/albums/posters）](51-tag-plugins-decorative/spec.md) | 装饰标签插件（poetry/paper/reel/blur/psw/emp/wavy/sub/folders/albums/posters） | 低 | `shortcode.go` + `tag-plugins.css` |
| 52 | [内容系统（专栏 Topic + 圆角渐变配置）](52-content-systems/spec.md) | 内容系统（专栏 Topic + 圆角渐变配置） | 低 | `store/topic` + API + Console + `site.yml` |

### Wave 10 冲突矩阵（共享文件热点）

| 共享文件 | 竞争者 | 处置 |
|---------|--------|------|
| `internal/post/shortcode.go` | 48 / 49 / 51 | **合并顺序 48 → 49 → 51**（48 先定行号与 grid/blockquote 块级基座，49 rebase 后追加，51 最后；全部 append 式，每合一个其余 `git rebase dev`） |
| `assets/css/tag-plugins.css` | 48 / 49 / 51 | 与 shortcode.go 同顺序串行 merge，append 式单段冲突秒解 |
| `config/site.yml` + `site.go` | 50 / 52 | 各加独立段（50 动 nav_tabs/style 消费点，52 动 topics 段 + style 圆角渐变组），append 式无冲突 |

**50 / 52 与 48/49/51 零文件交集，可随时并行**；48 依赖 01（Chroma 行号）/06/16/20/30 已合，49 依赖 06/16/20/30/31 已合，50 依赖 33/34 已合，51 依赖 31 已合。

## 当前批次 · Wave 8（差距对齐，可立即并行）

来源：前端 vs 原版 Stellar + 控制台 vs Halo 2.x 的全面差距调研（2026-07）。

### 前端簇（对齐 Stellar）

| # | Spec | 主模块（独占） | 优先级 |
|---|------|---------------|--------|
| 30 | [标签插件四期（vote/friends/sites/radio+行内）](30-tag-plugins-4/spec.md) | `shortcode.go` + 新 `store/vote.go` | P0（JS 服务已带，低成本高收益） |
| 31 | [标签插件五期（展示类 11 件收官）](31-tag-plugins-5/spec.md) | `shortcode.go` + `tag-plugins.css` | P1 |
| 32 | [文章页脚与文首增强](32-article-footer-plus/spec.md) | `content.html` + 新 `store/related.go` | P0 |
| 33 | [widgets 配置化编排 + gh widgets](33-widgets-config/spec.md) | `sidebar/rightbar.html` → `partial/widgets/` + `web.go` | P1（**35/47 前置**） |
| 34 | [外观样式可配置化](34-style-config/spec.md) | `site.go` style 段 + `head.html` 变量注入 | P1（**47 前置**） |
| 36 | [体验增强（preload/scrollreveal/heti）](36-js-enhance/spec.md) | `assets/js/plugins/` + `assets/vendor/` | P2 |
| 37 | [前端杂项（hero/nav_tabs/文案收敛）](37-frontend-misc/spec.md) | `index.html` + `navbar.html` + 新 `lang.go` | P2 |

### 控制台簇（对齐 Halo）

| # | Spec | 主模块（独占） | 优先级 |
|---|------|---------------|--------|
| 39 | [Console 全局搜索接 FTS](39-console-search/spec.md) | `DefaultLayout.vue` + 新 `api/search.go` | P0（后端 API 已有） |
| 40 | [评论批量操作](40-comment-batch/spec.md) | `CommentList.vue` + `api/comment.go` | P0 |
| 41 | [备份整站恢复](41-backup-restore/spec.md) | `internal/backup/` + `BackupView.vue` | P1 |
| 42 | [SEO 设置 + 代码注入](42-seo-codeinject/spec.md) | `settings` +2 Tab + `head.html` 消费点 | P1 |
| 43 | [瞬间增强（九宫格+可见性）](43-moments-plus/spec.md) | `store/moment.go` + `MomentList.vue` | P2 |
| 44 | [附件分组](44-attachment-groups/spec.md) | `store/attachment.go` + `AttachmentList.vue` | P2 |
| 46 | [版本行级 Diff 视图](46-revision-diff/spec.md) | 新 `diff.ts`(+test) + `RevisionDiff.vue`（纯前端） | P1 |

### Wave 9（依赖 Wave 8 或需串行的大工程）

| # | Spec | 前置 | 优先级 |
|---|------|------|--------|
| 35 | [Wiki 文档体系 + 笔记本体系](35-wiki-notes/spec.md) | **33 先合**（tree/tagtree 走 widget 机制） | P1（大） |
| 38 | [多作者体系](38-multi-author/spec.md) | 33 先合（author widget） | P2 |
| 45 | [用户账户体系（用户名+资料+2FA）](45-users-account/spec.md) | 建议 Wave 8 全合后单独进行（动鉴权主链路） | P1（大） |
| 47 | [Console 外观设置（style/widgets 可视化）](47-console-appearance/spec.md) | **34 + 33 先合** | P2 |

### Wave 8 冲突矩阵（共享文件热点）

| 共享文件 | 竞争者 | 处置 |
|---------|--------|------|
| `internal/post/shortcode.go` | 30 / 31 | **合并顺序 30 → 31**（31 开工前 rebase） |
| `config/site.go` + `site.yml` | 32 / 34 / 36 / 37 | 各加独立段（append 式），建议顺序 34 → 32 → 36 → 37；配置键一致性测试每合一个跑一次 |
| `partial/head.html` | 32 / 34 / 36 / 42 | append 式单行/单段冲突，按上述顺序串行 merge |
| `internal/api/router.go` | 30 / 39 / 40 / 41 / 42 / 44 | 各 +1~2 行挂载，冲突秒解 |
| `console/src/api/index.ts` | 39 / 40 / 42 / 43 / 44 | 各加独立 api 对象，append 式 |
| `SettingsView.vue` | 42 / 47(Wave9) | 42 先落，47 后续 rebase |
| `middleware.go` | 36（+1 行埋点跳过） | 独占无冲突 |

**纯前端零冲突快道**：39（除 api 一行）/ 46 与其余全部零交集，随时可合。

### 明确不做 / 暂缓项（差距清单的正式处置，不出 spec）

| 项 | 处置 | 理由 |
|---|------|------|
| 多主题安装/切换 | **不做** | 模板/CSS 走 go:embed，运行时换主题与单二进制架构冲突；主题设置面板由 Spec 47 承接 |
| 运行时插件系统 / 应用市场 | **不做** | 同上，动态加载与单体架构冲突；标签插件速查库（Spec 23）已覆盖写作侧需求 |
| 多存储策略（S3/OSS） | **不做** | 见 Spec 44 技术决策；外置存储走反代层 |
| 多用户 + RBAC | **暂缓** | Spec 45 表结构预留，真实多人需求出现再立 spec |
| 站内通知中心 | **暂缓** | 单管理员场景价值待定，邮件通知（Spec 26）已覆盖评论场景 |
| 完整 i18n 多语言切换 | **暂缓** | Spec 37 做文案表收敛留口 |
| tianli_gpt 外部依赖 / MathJax / algolia | **不做** | 见 Spec 36/34 技术决策（外部付费依赖 / KaTeX、FTS 路线已定）；AI 摘要需求改由 Spec 53 用自建 AI 管线承接（原「不做 AI 摘要」决策作废） |
| canonical 源站保护（克隆站警告） | **不做** | Spec 53 正式处置：单站部署无价值；main.js 既有代码无配置即 no-op，零风险保留 |

> 调研勘误记录：一度误报「页脚 sitemap 链接的 /atom.xml 与 /feed.json 未注册会 404」——实际 `internal/feed` 包的 `Register` 已注册两路由，无此 bug。

## 已完成（合入 dev）

01 代码高亮 · 02 友链 · 03 附件图床 · 04 评论点赞 · 05 前端清理+后台 Halo 化 · 06 标签插件 · 07 KaTeX · 08 阅读体验 · 09 文章页脚 · 10 侧边栏 widget · 11 Feed · 12 PWA · 13 Pages · 14 Moments · 15 文章增强 · 16 标签插件二期 · 17 回收站 · 18 菜单可视化 · 19 附件增强（Wave 4） · 20 标签插件三期 · 21 GitHub widgets · 22 文章版本历史（Wave 5） · **23 Console 插件库 · 24 访问统计 · 25 FTS5 搜索 · 26 评论通知配置化 · 27 OG 社交分享图（Wave 6）** · **28 上线前加固 · 29 Console UX+写作面板（Wave 7）** —— 详见各目录 spec，均已 merge。

## 历史批次 · Wave 7（已完成，存档）

| # | Spec | 主模块（独占） |
|---|------|---------------|
| 28 | [上线前加固](28-launch-hardening/spec.md) | `cmd/server/main.go`（优雅退出/中间件）+ `api/auth.go`（Cookie/限流）+ `head.html`（依赖本地化/JSON-LD）+ `Dockerfile` |
| 29 | [Console UX 打磨 + 标签插件写作面板](29-console-ux-plugins/spec.md) | `console/src/`（PostEditor/PluginLibrary/MarkdownEditor + 列表布局，纯前端） |

P0（优雅退出 · site.url 校验 · Cookie 安全 · 登录限流）+ P1（gzip · 依赖本地化 · 长缓存 · 自定义 500 · Docker 非 root · JSON-LD · SVG 防护）已全量落地并冒烟验证通过；P2（PWA 离线兜底 · fillSidebar 缓存 · 安全响应头 · search.json 瘦身 · DB 索引 · 图片尺寸上限 · 请求体上限/slog · font-display · a11y）留待上线后迭代，见 28 spec P2 表。

## 历史批次 · Wave 6（已完成，存档）

| # | Spec | 主模块（独占） |
|---|------|---------------|
| 23 | [Console 插件库](23-plugin-library/spec.md) | 新 `console/src/modules/plugins/`（静态元数据 + iframe 预览） |
| 24 | [访问统计分析](24-analytics/spec.md) | 新 `store/pageview.go` + 埋点中间件 + Console 仪表盘图表 |
| 25 | [站内搜索增强（FTS5）](25-search-fts/spec.md) | 新 `internal/search/` 包 + `/api/search` + 前端搜索适配 |
| 26 | [评论邮件通知](26-comment-notify/spec.md) | notify seam 单测 + Console SMTP 配置 tab + password 脱敏 + 测试发信 |
| 27 | [归档页 + OG 图](27-archives-og/spec.md) | 新 `internal/og/` 包（OG 图生成）+ head meta |

## 历史批次 · Wave 5（已完成，存档）

| # | Spec | 分支 | 主模块（独占） |
|---|------|------|---------------|
| 20 | [标签插件三期](20-tag-plugins-3/spec.md) | `feature/tag-plugins-3` | `internal/post/shortcode.go` + `tag-plugins.css` + 新 `store/rating.go` |
| 21 | [GitHub widgets](21-gh-widgets/spec.md) | `feature/gh-widgets` | 新 `internal/github/` 包 + `main.go` 路由 |
| 22 | [文章版本历史](22-post-revisions/spec.md) | `feature/post-revisions` | 新 `store/revision.go` + `api/revision.go` + Console `RevisionPanel` |

**Wave 5 冲突矩阵**：唯一热点是 `internal/post/shortcode.go`（20 加 8 插件、21 加 2 插件 + processShortcodes +1 注入参数）。**合并顺序：21 → 20 → 22**（21 先定 processShortcodes 签名，20 rebase 后追加；22 与两者零文件交集，随时可合）。22 依赖 17 已合（post 表迁移模式稳定）。20 的 ghcard 消费 21 的 `/api/gh/card` 代理端点。

## 历史批次 · Wave 4（已完成，存档）

| # | Spec | 分支 | 主模块（独占） |
|---|------|------|---------------|
| 16 | [标签插件二期](16-tag-plugins-2/spec.md) | `feature/tag-plugins-2` | `internal/post/shortcode.go` + `tag-plugins.css` |
| 17 | [回收站](17-trash/spec.md) | `feature/trash` | `store/post.go`+`page.go`（+列）+ api + console 列表 |
| 18 | [菜单可视化管理](18-menu-settings/spec.md) | `feature/menu-settings` | console settings 菜单 tab（新 `MenubarEditor.vue`） |
| 19 | [附件增强](19-attachment-plus/spec.md) | `feature/attachment-plus` | `api/attachment*.go` + `store/attachment.go` + console 附件库 |

Wave 4 四个 spec 无共享文件热点（17 动 `store/post.go`，其余不碰），任意顺序合并；Spec 22（版本历史）依赖 17 先合。

## 历史批次 · 夜间 8 小时并行批次（已完成，存档）

按**波次**编排：Wave 1 可**立即全部并行**；Wave 2/3 依赖前置（admin 外壳 / post 表结构），前置合并后再并行。

### Spec 05 · 前端清理 + 后台 Halo 化（前置，建议先起）
- Part A `fix/frontend-cleanup`（前端 head 清理，独立小分支）
- Part B `feature/admin-halo-console`（后台整体 Halo 外壳，**是 Wave 2 的前置**）

### Wave 1 — 立即并行（7 个，互不依赖，无 admin 耦合）

| # | Spec | 分支 | 主模块（独占） |
|---|------|------|---------------|
| 06 | [标签插件扩展包](06-tag-plugins/spec.md) | `feature/tag-plugins` | `internal/post/shortcode.go` + `assets/css/tag-plugins.css` |
| 07 | [KaTeX 数学公式](07-katex-math/spec.md) | `feature/katex-math` | `internal/post/post.go`（goldmark 装配） |
| 08 | [阅读体验增强](08-reading-ux/spec.md) | `feature/reading-ux` | `assets/js/reading.js` + `assets/css/reading.css` |
| 09 | [文章页脚](09-post-footer/spec.md) | `feature/post-footer` | `partial/content.html` 页脚区 + `config` |
| 10 | [侧边栏 widget 扩展](10-sidebar-widgets/spec.md) | `feature/sidebar-widgets` | `partial/rightbar.html` + `fillSidebar` |
| 11 | [Feed 增强](11-feeds/spec.md) | `feature/feeds` | 新 `internal/feed/` 包 |
| 12 | [PWA + 图标](12-pwa/spec.md) | `feature/pwa` | 新 `assets/manifest.webmanifest`/`sw.js` |

### Wave 2 — 依赖 Spec 05 Part B 合并后（内容类型，接入 admin 外壳）

| # | Spec | 分支 | 主模块（独占） |
|---|------|------|---------------|
| 13 | [独立页面系统 Pages](13-pages-cms/spec.md) | `feature/pages-cms` | `internal/store/page.go` + `internal/page/` |
| 14 | [瞬间/说说 Moments](14-moments/spec.md) | `feature/moments` | `internal/store/moment.go` + `internal/moment/` |

### Wave 3 — 依赖 05B + Wave2 合并后（改 post 表结构）

| # | Spec | 分支 | 主模块 |
|---|------|------|--------|
| 15 | [文章增强：置顶/密码/定时](15-post-enhancements/spec.md) | `feature/post-enhancements` | `internal/store/post.go`（+列） + admin edit |

## 冲突矩阵（共享文件 = 唯一需协调点）

| 共享文件 | 06 | 07 | 08 | 09 | 10 | 11 | 12 | 说明 / 合并顺序 |
|---------|----|----|----|----|----|----|----|----|
| `partial/head.html` | +link | +katex | +link | — | — | — | +manifest | **热点**：4 个 append `<link>/<script>`，按 06→07→08→12 顺序合，冲突均为单行 |
| `layout/base.html` | — | — | +元素 | — | — | — | +sw注册 | 仅 08/12，不同位置 |
| `cmd/server/main.go` | — | — | — | — | +fillSidebar | +Register | +routes | 各不同区段，append 式 |
| `internal/template/model.go` | — | — | — | +字段 | +字段 | — | — | 09/10 各加字段；优先放新 `model_*.go`，ViewData/Page 字段不可避免时 +1 行 |
| `config/site.go`+`site.yml` | — | — | — | +段 | — | +feed可选 | +pwa可选 | 09 为主；11/12 若需配置放独立 key |
| `internal/store/*` | — | — | — | — | +读方法(新文件) | — | — | 10 新增读方法用新文件，不碰 post.go |
| `go.mod` | mermaid(前端CDN,无需) | +katex/mathjax | — | — | — | — | — | 仅 07，`go mod tidy` |

**结论**：Wave 1 唯一真正竞争的是 `head.html`（4 个追加者）与 `main.go`（3 个不同区段）。都是 append 式单行冲突，按上表顺序串行 merge，秒解。其余为各自独占。

## 合并顺序总则

1. 先合 **05 Part A**（前端清理，零依赖）。
2. Wave 1 七个并行开发；合并顺序建议 **11 → 12 → 10 → 09 → 06 → 07 → 08**（把改 `main.go`/`head.html` 的靠前，减少后续 rebase）。每合一个，后面未合分支 `git rebase dev`。
3. 合 **05 Part B**（admin 外壳）。
4. Wave 2（13、14）并行 → 合并。
5. Wave 3（15）最后。

## 每个 agent 开工前必读

- 根目录 `AGENTS.md`、`CLAUDE.md`
- 铁律：**前端严格对齐原版 Stellar，禁止自创样式**；**补样式前先 grep `assets/css/main.css` 确认类是否已存在**；参考主题 `../resource/hexo-theme-stellar/`
- 模板路径化命名（`partial/xxx`、`admin/xxx`），基名勿撞现有
- 新 spec 按 [`_template/spec.md`](_template/spec.md) 起草；含 UI/样式子任务必须填写「UI 验收边界」节（参照图路径 + 逐项验收清单），参照图与基线截图存入本 spec 目录的 `baselines/`（入库），详见根目录 `AGENTS.md`「UI 类任务开工约定」
- 完成标准：`go build ./... && go vet ./...` 双通过 + spec 内「验证」清单全绿 + 起服务冒烟
