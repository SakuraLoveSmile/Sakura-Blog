# 独立页面不展示 AI 摘要

## 参照

唯一参照为本次请求提供的线上 `/friends` 截图：`reference-before.webp`（2692×1558，sha256 `87902d68503df29b58b13f0ee5d9f53fc7a73a6c09033976da99edb4fcba6079`，附件规范化副本）。截图中友链页正文顶部出现「AI 摘要 / Reader API / 用你自己的 API 生成 AI 摘要」卡片，但该页没有已存摘要，语义上也不是可摘要的文章内容。参照图记录的是改动前的问题状态，不是期望效果。

## 目标效果

- 独立页面（`layout=page`，含系统页 `/friends`、`/links`、`/about` 及自定义页面）不再输出 `#ai-summary` 卡片：标题、Reader API 标签、读者自助生成面板与「生成摘要」按钮都不出现。
- 文章内容页（`layout=post`，含专栏/主题文章）行为不变：已存摘要与读者自助面板照旧渲染。
- Wiki 文档（`layout=wiki-doc`）与笔记文章（`layout=note`）保持原有渲染，不在本次调整范围。
- 全局设置 `blog.ai.summary.enabled=false` 时仍不渲染，原有关闭行为不变。
- 只移除 DOM 片段，不改 CSS、共享主题源或新增视觉令牌；不留空容器或额外间距。

## 验收项

- [x] 独立页面（`layout=page`）渲染结果不含 `id="ai-summary"`，也不含 `data-user-api` 与「用你自己的 API 生成 AI 摘要」按钮。
- [x] 文章页（`layout=post`）渲染结果仍含 `id="ai-summary"`，已存摘要与自助面板照旧。
- [x] 摘要开关关闭时任何页面都不渲染摘要块。
- [x] `layout=wiki-doc` / `layout=note` 的渲染分支未被误删（`blogSummary` 单测确认仍渲染）。
- [x] 页面正文首块与标题卡间距不受影响（仅删除卡片，不产生空白元素；见截图）。
- [x] 浅色、深色、自动配色与四种 appearance 预设无视觉回归（无 CSS 改动；按 card 预设抽查浅色、深色与手机宽度截图）。

## 验收记录（2026-11-02）

隔离站点（不连接主工作区或生产数据）：

- 运行：本 worktree 代码编译的 `cmd/blog`，`BLOG_LISTEN=:18080`；SQLite、上传与配置位于 `.tmp/friends-ai-summary/runtime`。站点为空白 Stellar 2.0 库，显式开启 `blog.ai.summary.enabled=true` 与 `summary.allow_user_api=true`；夹具含系统页 `/friends`、自定义独立页 `/about`、带已存摘要的对照文章 `/post/ai-summary-control`。`/readyz` 返回 200。
- HTTP DOM 断言（同一服务、同一配置）：
  - `/friends`：`tag-plugin colorful note ai-summary` 0 次、`data-user-api="1"` 0 次、「用你自己的 API 生成 AI 摘要」0 次；页面其余位置仅剩文章链接文本与 runtime 模块注册中的 `blog-ai-summary`。
  - `/about`：上述三项均 0 次（自定义独立页面同样生效）。
  - `/post/ai-summary-control`：摘要卡 1 次、`data-user-api="1"` 1 次、自助按钮 1 次，已存摘要「这是验收用的文章摘要。」正常输出。
- 浏览器截图（Playwright chromium 1.62.1，`playwright screenshot`）：
  - `after-friends-desktop.png`：1346×779 浅色，友链页标题卡后直接进入「友链列表」，无摘要卡、无空白占位。
  - `after-friends-dark.png`：1346×779，`color-scheme=dark` 下自动配色切到深色，同样无摘要卡。
  - `after-friends-mobile.png`：390×844 手机宽度，无摘要卡、无横向溢出。
  - `after-about-desktop.png`：1346×779，自定义独立页同样无摘要卡。
  - `after-article-desktop.png`：1346×779，文章页保留「AI 摘要 / Reader API / 已存摘要 / 自助按钮」。
- 机械门禁：`make test` 全量通过（exit 0，含 `go vet`、Staticcheck、`go test ./...`）。
- 未验证项：生产服务器尚未部署本次修改；四种 appearance 预设未逐一切换截图（无 CSS/模板结构改动，仅移除 DOM，按 card 预设抽查浅色、深色与手机宽度）。
