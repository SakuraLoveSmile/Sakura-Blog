# Spec 53 · 前端收尾（作者链接 / swiper 本地化 / wiki 时间）+ AI 文章摘要

- **分支**：`feature/frontend-finale` · **Wave 11** · **隔离等级**：低
- **主模块（独占）**：`internal/api/ai.go` + `store/post.go`（+1 列）+ `partial/content.html` + `assets/js/ai-summary.js`（新）+ `console/src/modules/posts/`
- **前置**：Wave 10（Spec 48-52）已合；现有 AI 管线（Spec 30 的 `config.LoadAI` / `callChatCompletion` / settings `ai` 段 / `/api/v1/ai/generate` / AiGenerateModal）为本 spec 复用基线

## Why

前端 vs 原版 Stellar 收官调研（2026-08）结论：对齐度 ~95%，剩余真实差距 4 项均为小时级小缺口（作者链接、swiper CDN、wiki dateinfo、canonical 处置）；另按站长需求新增 **AI 文章摘要**能力——原版对应 tianli_gpt（外部付费依赖，原决策"不做"），本 spec 用已有自建 AI 管线替代实现：

1. **后台生成好的**：管理员在 Console 对单篇文章生成摘要存库，前台读者直接看。
2. **读者临时接入自己的 API**：无预生成摘要时，读者可填自己的 OpenAI 兼容 API（endpoint/model/key）现场生成；key 不落服务端、默认不落浏览器，经后端限流透传（解决 CORS + SSRF 防护）。

## 功能需求

### Part A · 文章头部作者链接（dateinfo 对齐）

原版 `dateinfo.ejs` 多作者模式：`#post-meta` 首位渲染 `<a class="author">{name}</a>` + sep，且**有作者时创建时间不带"发布于："前缀**。现状 `content.html` 只有 创建/更新时间，作者仅出现在页脚版权行。

目标：`Page.Author` 命中 `Site.Authors`（web.go 已装配 Author 卡片数据）时，banner `#post-meta` 首位插入作者链接（href = 已装配的 Author Path，即 `/author/{name}/`），创建时间去前缀；无作者时保持现状。

### Part B · swiper 依赖本地化

现状 `assets/js/services/swiper.js` 从 unpkg CDN 加载 swiper@10.3（Spec 28 依赖本地化的唯一遗漏，fancybox/heti/flying-pages 均已本地化）。目标：`swiper-bundle.min.css|js` 放入 `assets/vendor/swiper/`（随 embed.go 自动内嵌），swiper.js 常量改本地路径。

### Part C · wiki dateinfo + canonical 处置

- wiki 文档页：原版 wiki 内页面包屑下显示「更新于：{updated}」；`wiki_doc.html` 补齐（wiki 分支只显示 updated，不显示 created，对齐原版 dateinfo.ejs wiki 分支）。
- canonical 源站保护：**正式决策不做**（单站部署场景无价值）——`main.js` 已有代码在 `window.canonical` 缺失时直接 return，零风险保留；不加配置段。README「明确不做」表追加一行。

### Part D · AI 文章摘要（核心）

| 能力 | 目标 |
|---|---|
| 数据层 | posts 表 `ADD COLUMN ai_summary TEXT NOT NULL DEFAULT ''`（ALTER 迁移既有模式）；API post payload 增 `ai_summary` 字段 |
| AI 生成 | `/ai/generate` 字段白名单增 `summary`（aiResult 增 `Summary`），prompt：`"summary": 120-200 字中文导读摘要，面向读者概括全文亮点，语气自然，不同于 excerpt 的列表摘要` |
| Console | ① EditorSidebar 新增「AI 摘要」面板：textarea 手动编辑 + 「AI 生成/保存/清空」按钮（走独立端点 `PUT /posts/:slug/ai-summary`，独立于主保存链路，避免 seed/常规保存误清）；② AiGenerateModal fieldDefs 增 summary 勾选项；③ 设置 → AI tab 增两开关：`summary_enabled`（前台展示总开关）、`allow_user_api`（读者自助生成开关） |
| 前台展示 | `content.html` 文章顶部（过期提醒之前）：`ai_summary` 非空 → 直接渲染摘要块（`.tag-plugin colorful note` 结构，header 标签「AI 摘要」）；为空且两开关开启 → 渲染折叠的自助生成面板 |
| 读者自助生成 | 新公开端点 `POST /api/v1/ai/summary/guest`：body `{endpoint, api_key, model, content}`，后端用请求参数现组 cfg 调 `callChatCompletion`（固定 system prompt），返回 `{summary}`；**结果不落库**，读者本次会话内展示；前端表单可勾「记住本机配置」写 localStorage `Stellar.ai`（key 除外，key 默认每次重填） |

## What Changes（逐文件）

### 后端

- `internal/store/post.go`：ALTER 迁移 +1 列 `ai_summary`（幂等）；Create/Update/Get/List 读写该列。
- `internal/api/ai.go`：
  - `aiAllowedFields` + `aiResult` + `aiFieldSpecs` 增 `summary`。
  - 新增 `aiSummaryGuest`（公开端点）：校验 https-only + host 解析后拒绝私网段（`127./10./192.168./172.16-31./169.254./::1/fc00::/7`，SSRF 防护）；请求体 ≤ 256KB、content 截断 40000 字；上游超时 60s；响应截断 2000 字；api_key 不打日志不落库。
- `internal/api/router.go`：挂载 `POST /ai/summary/guest`（无需鉴权）+ 简易内存限流（per-IP 10 req/min，复用 loginFails 同款 map + 时间窗模式）；读取 settings `ai` 段 `allow_user_api`，false 时 403。
- `internal/api/settings.go`：`ai` 段校验兼容新键 `summary_enabled` / `allow_user_api`（key 脱敏逻辑复用既有）。
- `cmd/server/routes_post.go`：ViewData 装配 `Page.AISummary`（settings `summary_enabled` 为 false 时置空）。
- `internal/template/model_article.go`（或 Page 所在 model）：`AISummary string`。

### 前端

- `assets/js/ai-summary.js`（新）：自助面板交互——展开/收起、表单（endpoint/model/key + 记住勾选）、调 guest 端点、loading/toast（复用 hud.toast）、结果渲染进摘要块；document 委托绑定，PJAX 友好。
- `partial/content.html`：摘要块 + 自助面板结构（data 开关来自 Site 配置注入的 JSON 或 data-* 属性）；`#post-meta` 作者链接（Part A）。
- `partial/wiki_doc.html`：「更新于」行（Part C）。
- `assets/js/services/swiper.js`：`SWIPER_CSS/SWIPER_JS` 改 `/assets/vendor/swiper/swiper-bundle.min.css|js`（Part B）。
- `assets/vendor/swiper/`（新）：从 unpkg 下载 swiper@10.3 bundle 两个文件（embed.go 已含 `all:vendor`，无需改）。
- `internal/template/lang.go`：文案 `ai.summary.*`（标题/生成按钮/自助提示/key 隐私说明/错误态）。

### Console

- `console/src/api/index.ts`：aiApi 增 `summaryGuest` 不需要（前台自调）；post payload 类型增 `ai_summary`。
- `console/src/modules/posts/EditorSidebar.vue`：「AI 摘要」面板（textarea + 生成按钮 + 未配置提示）。
- `console/src/modules/posts/AiGenerateModal.vue`：fieldDefs 增 `{ key: "summary", label: "AI 摘要", kind: "textarea" }`，apply 回填。
- `console/src/modules/settings/SettingsView.vue`（AI tab）：两开关 + 说明文案。

## 实现步骤

1. 数据层：posts 迁移 + 单测（建列幂等、读写往返）。
2. AI 管线：`/ai/generate` 增 summary 字段 + guest 端点（含 SSRF/限流单测：私网地址 403、限流 429、key 不出现在响应/日志）→ router 挂载。
3. Console：AI tab 开关 → EditorSidebar 面板 → AiGenerateModal 勾选项。
4. 前台：content.html 摘要块/自助面板 + ai-summary.js → Part A 作者链接 → Part C wiki dateinfo。
5. Part B：swiper 本地化 → 回归 swiper 标签页。
6. 全量回归：`make test` + 双主题冒烟（摘要块有/无/未配置三态）。

## 技术决策（无需再问）

- **复用既有 AI 管线**：不新建 LLM 适配层；guest 端点与 `/ai/generate` 共用 `callChatCompletion`，仅 cfg 来源不同（请求参数 vs settings）。
- **摘要保存走独立端点**：`ai_summary` 列在 store.Update 中走 COALESCE 保留（对齐 poster/banner/references 模式），seed/Upsert 等不携带该字段的写路径不会误清；Console 面板用「保存摘要/清空」按钮显式落库。
- **站点 AI key 绝不下发前台**：读者侧只能「看预生成」或「用自己的 key」，不提供用站点 key 的在线生成按钮。
- **读者生成结果不落库**：仅本次会话展示；入库必须走管理员 Console（内容审核责任在站长）。
- **guest 端点安全基线**：https-only + 私网段拒绝 + per-IP 限流 + 体积/超时上限；`allow_user_api` 可整体关闭。
- **摘要 ≠ excerpt**：excerpt 是列表页摘要（100 字内），summary 是文章顶部导读（120-200 字），两个字段并存互不替代。
- **canonical 不做**：main.js 代码保留（无配置即 no-op），不新增配置段。
- **swiper 版本锁定 10.3**：与原版 CDN 版本一致，避免升级引入行为差异。

## Impact

- **新增**：`assets/js/ai-summary.js`、`assets/vendor/swiper/*`、lang 文案、guest 端点
- **修改**：`store/post.go`、`api/ai.go`、`api/router.go`、`api/settings.go`、`routes_post.go`、template model、`partial/content.html`、`partial/wiki_doc.html`、`services/swiper.js`、Console 4 文件
- **不影响**：shortcode、评论、搜索、备份等模块（迁移仅加列）

## 验证（验收清单）

- [ ] `go build ./... && go vet ./...` 通过；`make test` 全绿（含迁移幂等、SSRF 私网 403、限流 429 单测）
- [ ] Console：AI 未配置时生成按钮禁用并提示；配置后单篇生成 → 保存 → 前台可见；手动编辑覆盖 AI 结果正常保存
- [ ] 前台三态：有摘要直接显示（亮/暗主题样式正确）；无摘要+开关开 → 自助面板可用；`summary_enabled=false` 时完全不渲染
- [ ] 读者自助：填自己的 API 生成成功；key 不写 localStorage（除非勾记住，且记住项不含 key）；断网/错误 API 有友好 toast
- [ ] guest 安全：`http://` 端点 400、`http://127.0.0.1` 等私网 403、超频 429、api_key 不出现在任何响应字段与日志
- [ ] 作者链接：多作者文章 banner 显示作者链接（跳转 `/author/{name}/`），创建时间无「发布于：」前缀；单作者/无作者文章保持现状
- [ ] swiper：断网环境（或屏蔽 unpkg）下 `{% swiper %}` 标签正常轮播
- [ ] wiki 文档页显示「更新于：{日期}」；PJAX 切页后摘要块与作者链接渲染正常

## Out of Scope

- 流式输出（一次性生成，loading 态即可）
- 发布文章时自动生成摘要（手动触发，避免意外 API 费用）
- 非 OpenAI 兼容协议的提供商适配
- 摘要的多语言生成、读者点赞/反馈
- tianli_gpt 的「推荐文章/矩阵穿梭」等附属交互
