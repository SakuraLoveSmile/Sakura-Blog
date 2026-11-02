# AI 摘要已生成但前台不显示

任务编号：BLG-20261102-08。状态：实现中（本地修复与验证中，未提交、未发布）。

## 现象与根因

用户反馈线上文章 `/post/hexo-blog-setup-guide` 已生成 AI 摘要，但前台摘要卡片只有标题与读者自助面板，正文为空。

- 生产库只读核对（2026-11-02）：该文章 `posts.ai_summary` 列有 299 字摘要；`blog_extensions` 为 `{"status":"draft"}`，**不含** `content.ai_summary`。全站仅此 1 篇有列内摘要，其余 26 篇均无任何已存摘要。
- 前台 `modules/stellar/routes/stellar_runtime.go` 的公开快照只读 `blog_extensions.content.ai_summary`（严格 Stellar 2.0 canonical 投影），因此 `Page.AISummary` 为空，`partial/content.html` 只渲染空 `.body`。
- Console 侧 Spec 53 的 AI 摘要写入路径 `PUT /api/v1/posts/:slug/ai-summary` → `store.SetAISummary` 只写 `posts.ai_summary` 列，从不写 canonical 扩展。该缺口在严格 2.0 快照上线后出现：前台改读 canonical，写入端仍停在旧列。

## 已确认决定

- 摘要的 canonical 存储位置保持 `blog_extensions.content.ai_summary`（与迁移、AI 摘要间距基线一致），不把前台改回只读旧列。
- `SetAISummary` 同时写列与 canonical 扩展：非空写入，空串删除；保留扩展里的其它字段和未知键；`stellar_version` 不是 2 的记录只写列。
- 读取端在 canonical 扩展缺失时回退到 `posts.ai_summary` 列，用于兼容本次及历史上「只写列」的已存摘要；扩展存在时仍优先扩展。
- 不修改 Console 前端、CSS、主题源、API 契约或生成产物。

## 运行资源

- 基线：根工作区 `dev` / `460354bd`（rebase 前基于 `0fb7aadf`；根工作区既有未提交文档与素材全部保留）。
- worktree：`/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261102-fix-blg-20261102-08-ai-summary-display`
- 分支：`codex/fix/20261102-blg-20261102-08-ai-summary-display`；合入目标：根工作区 `dev`。
- 生产只读核对使用 ssh 别名 `blog-prod` 与 `sudo python3` 只读打开 SQLite（`mode=ro`），未写入生产。

## 变更点

- `internal/store/post.go`：`mergeAISummaryExtension` 投影 helper；`SetAISummary` 改为事务内同时更新列与 canonical 扩展。
- `modules/stellar/routes/stellar_runtime.go`：公开快照摘要读取增加旧列回退。
- 测试：`internal/store/aisummary_test.go`（投影往返、清空、未知键保留、非法 JSON 不投影）、`modules/stellar/routes/stellar_ai_summary_test.go`（列回退、canonical 优先）。

## 验证记录

- 机械门禁：`make test` 退出 0（Console 类型/行为、Stellar assets、部署/来源/模块边界、`go vet`、Staticcheck、`go test ./...` 全部通过）；`make build` 生成真实 Console/User Center 产物与 Go 二进制。
- 新增回归：`TestPostAISummaryProjectsToStellarExtensions`、`TestMergeAISummaryExtension`（store）；`TestStellarAISummaryFallsBackToLegacyColumn`（routes，覆盖列回退与 canonical 优先）。既有 `TestStellarBlogProjectionIsExplicitAndPublic` 未改且通过。
- 隔离真实服务（worktree `.tmp/ai-summary-display/runtime`，`127.0.0.1:18089`，`scripts/e2e/prepare.go` 空站与本地合成数据）：Console API 创建文章并保存摘要后，数据库列与 `blog_extensions.content.ai_summary` 同时写入，其它扩展键保留；`GET /post/...` 返回 `<div class="body"><p>摘要…</p></div>`。把扩展改回线上同形 `{"status":"draft"}`、保留列后页面仍回退显示；清空端点后两处均移除且页面不输出；再次保存后恢复。
- 证据：`docs/baselines/ai-summary-display/verification.json`；截图 `reference-before.png`（线上问题状态）、`after-desktop.png`、`after-legacy-column.png`。
- 未验证：生产部署与生产数据写入未执行（需授权）；README 已说明未逐项跑 appearance/移动端截图，因为本次不改 CSS/模板结构。

## 剩余问题

- 发布后需在 Console 重新保存一次摘要或等待回退路径生效：修复后的前台会直接显示既有列内摘要，无需重新生成。
- Console 主表单保存仍可能用旧 `blog_extensions` 覆盖 canonical 摘要；读取端回退保证展示不丢，下一次保存摘要会重新投影。是否把摘要纳入主表单同步属后续产品决定，本次不扩大范围。

