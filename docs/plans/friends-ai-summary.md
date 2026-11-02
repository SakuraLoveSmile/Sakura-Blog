# 独立页面不展示 AI 摘要

## 目标与范围

用户反馈线上 `/friends` 页出现了「AI 摘要 / Reader API / 用你自己的 API 生成 AI 摘要」卡片，该页不需要 AI 摘要。经确认，本次按「所有独立页面都隐藏 AI 摘要」实施：`layout=page` 不再渲染摘要块与读者自助面板，AI 摘要保留在文章与文档内容页。

- 仅改前台渲染层（Stellar v2 helper），不动 Console 文章 AI 摘要编辑、数据库字段与 `/api/v1/ai/summary/guest` 接口。
- 不改 CSS、共享主题源、`main.css` 生成物。
- 独立页面的评论、分享、打赏、阅读进度等其它能力保持原样。

## 已确认决定

- 2026-11-02 用户选择「所有独立页面都隐藏」，而非只隐藏 `/friends` 或新增页面级开关。
- 独立页面判定沿用现有渲染契约：`StellarPresentation.Layout == "page"`（页面行 profile=page，含 `/friends`、`/links`、`/about` 与自定义页面）。文章/文档内容页（`post`、`wiki-doc`、`note`）不受影响。

## 运行资源

- 基线：根工作区 `dev` / `ddecd77e`。
- worktree：`/Users/sakurasep/Documents/Code/Project/Project Worktree/Blog/20261102-friends-ai-summary`
- 分支：`codex/friends-ai-summary`；合入目标：根工作区 `dev`。
- 隔离验收夹具：`.tmp/friends-ai-summary/`（`prepare.go` 建空白 Stellar 2.0 站、`/friends`、`/about` 与带摘要的对照文章；`runtime/` 为服务根目录，端口 18080）。不连接主工作区数据库或生产数据。

## 验收场景

见 `docs/baselines/friends-ai-summary/README.md`：独立页面无 `#ai-summary`，文章页保留摘要与自助面板，关闭开关时不渲染，Wiki/笔记分支不受影响，无样式回归。

## 代码与验证

- 变更点：`modules/stellar/web/stellar_template_helpers_blog.go` 的 `blogSummary` 对 `layout=page` 直接返回空；新增 `modules/stellar/web/stellar_template_helpers_blog_summary_test.go`，并更新 `TestStellarBlogCapabilitiesUseCanonicalFlagsAndUniqueContainers` 中针对独立页面的能力断言。
- 实测：`make test`（feature worktree）exit 0，含 `go vet`、Staticcheck、`go test ./...`。
- 实测：隔离服务 HTTP 断言 —— `/friends`、`/about` 无摘要卡/自助面板；`/post/ai-summary-control` 保留卡片、已存摘要与自助面板。
- 实测：Playwright 截图（浅色、深色、390px 手机、自定义独立页、文章页）已存入 `docs/baselines/friends-ai-summary/`。
- 未验证：生产部署；四种 appearance 预设未逐一切换截图（无 CSS 改动，抽查 card 预设浅/深/手机）。
