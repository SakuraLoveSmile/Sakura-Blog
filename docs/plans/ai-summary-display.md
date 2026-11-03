# AI 摘要已生成但前台不显示

任务编号：BLG-20261102-08。状态：待体验（beta03 修复摘要不可见、beta04 修复 HeTi 词内空隙，均已发布并部署；线上文章摘要完整可见，待用户确认体验）。

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

## 后续修复：HeTi 环境摘要词内空隙（beta04）

- 现象：beta03 部署后摘要已显示，但在启用 HeTi 的页面（生产默认启用）出现 `He xo`、`Gi t`、`N ode.js` 等词内空隙；`reference` 为 `production-before-heti-fix.png`。
- 根因：`ai-summary.js` 的页面加载打字机把 SSR 摘要拆成 2 字一段的多个文本节点；HeTi 扩展（CDN 脚本晚于打字机运行）按文本节点处理 `.heti` 内容，把 `Hexo`、`Git` 等西文单词按片段包进 `<heti-spacing>`，相邻片段各带间距形成词内空隙。DOM 实测：`innerHTML` 被拆成 `<heti-spacing>He</heti-spacing><heti-spacing>xo</heti-spacing>`。
- 决定：启用 HeTi 时保留服务端完整摘要文本、交给 HeTi 正常排版，不播放打字机；未启用 HeTi 的站点保留原打字机动画。打字机结束（读者自助生成路径）额外 `normalize()` 合并文本节点，避免任何后置排版脚本按片段拆词。
- 变更：`modules/stellar/web/assets/js/ai-summary.js`；回归用例 `modules/stellar/tests/stellar-v2-native-lifecycle.test.mjs`「已有 AI 摘要启用 HeTi 时不播放打字机，保留服务端完整文本」。
- 验证：`make assets-test` 241 项通过；隔离服务（HeTi 开启）实测 `heti-spacing` 包裹完整单词（`Hexo`/`Git`/`Node.js v12`/`hexo-cli`/`GitHub Pages`），无词内拆分；HeTi 关闭时打字机仍运行（中途 20 个文本节点与光标，结束后合并为 1 个文本节点）。截图 `after-heti-desktop.png`。

## 发布记录（v2.0.7-beta03，2026-11-03）

- 用户授权「发布修复并部署」。提交：`e51ff549` fix: 修复已生成的 AI 摘要在前台不显示（10 files，含计划与验收基线）；经 `make merge-check` 合入 dev 为 `d8c385dd`（含 `origin/main` 历史同步），推送后 PR #39 `dev → main` 合并 `b1ac9421`。
- 晋级：PR #40（`chore: 同步 release 分支历史到 main`，内容零差异）合并 `30800e09`；PR #41 `main → release` 合并 `0796ac76`。三 PR 的必需 CI（go/lint/security/console/e2e/build，发布 PR 另含 release-source/image）全部通过。
- tag：`v2.0.7-beta03`（GPG 签名）打在 `0796ac76`；Docker Release run `41479377596` success（`0796ac76`，约 4m45s）。
- 镜像：`ghcr.io/sakuralovesmile/sakura-blog:2.0.7-beta03` digest `sha256:fd65dba9e2f861a5a99ea542532ab53c712685385d47c76ce9e8d21c608d67cd`；`sha-0796ac7` 同 digest；label `version=2.0.7-beta03`、`revision=0796ac76…`。
- 稳定标签未移动：`latest` / `2` / `2.0` / `2.0.6` 仍为 `sha256:944df8d8…a734392`；GitHub Release `v2.0.7-beta03` 为 Pre-release。
- 生产升级：升级前 `2.0.7-beta02`（image ID `e49df5be…`）、running、restarts 0。备份 `/home/ubuntu/blog-release-v2.0.7-beta03/`（0700）保存 `blog.db` 三件套与 compose 副本，db md5 源与副本一致；compose 仅 image 一行改为 `:2.0.7-beta03` 后 `pull && up -d`。升级后容器 `2.0.7-beta03`（image ID `fd65dba9…` = GHCR digest）、running、restarts 0；宿主 `/readyz` 200、公网 `/readyz` 200、公网 `/settings` 显示 `Blog v2.0.7-beta03`。
- 生产验收：`/post/hexo-blog-setup-guide` 的 `#ai-summary .body` 首次出现完整摘要 `<p>这篇教程…</p>`（回退路径生效）；但同页 HeTi 造成词内空隙，随后在 beta04 修复。回退方案：compose image 改回 `:2.0.7-beta02` 并 `up -d`；本次无数据库 schema 迁移。


## 发布记录（v2.0.7-beta04，2026-11-03）

- 提交：`e58672d0` fix: 修复 HeTi 环境下 AI 摘要词内空隙（7 files，含计划、验收基线与 JS 回归用例）；经 `make merge-check` 合入 dev 为 `d17113f9`（合并 `origin/release` 历史，内容零差异）。
- 晋级：PR #42 `dev → main` 合并 `805d86df`；PR #43 `main → release` 合并 `ef7d687c`。两 PR 的必需 CI（go/lint/security/console/e2e/build，发布 PR 另含 release-source/image）全部通过；release strict 前置通过（dev 已含 release 历史，无需额外同步 PR）。
- tag：`v2.0.7-beta04`（GPG 签名）打在 `ef7d687c`；Docker Release run `41489270857` success（约 7m46s）。
- 镜像：`ghcr.io/sakuralovesmile/sakura-blog:2.0.7-beta04` digest `sha256:1f6612a75e4fa05811178f60821c31e0e9336be92409a3a95ca71b2d4897da36`；`sha-ef7d687` 同 digest；label `version=2.0.7-beta04`、`revision=ef7d687c…`；服务器 `docker run --rm …:2.0.7-beta04 -h` 退出码 0。
- 稳定标签未移动：`latest` / `2` / `2.0` / `2.0.6` 仍为 `sha256:944df8d8…a734392`；`2.0.7-beta03` digest 未变；GitHub Release `v2.0.7-beta04` 为 Pre-release。
- 生产升级：升级前 `2.0.7-beta03`（image ID `fd65dba9…`）、running、restarts 0。备份 `/home/ubuntu/blog-release-v2.0.7-beta04/`（0700）保存 `blog.db` 三件套与 compose 副本，db md5 源与副本一致；compose 仅 image 一行改为 `:2.0.7-beta04` 后 `pull && up -d`。升级后容器 `2.0.7-beta04`（image ID `1f6612a7…` = GHCR digest）、running、restarts 0；宿主 `/readyz` 200、公网 `/readyz` 200、公网 `/settings` 显示 `Blog v2.0.7-beta04`，公网首页/`/console/`/`/uc/` 200。
- 生产验收（Playwright，公网真实页面）：`#ai-summary .body p` 文本完整；`heti-spacing` 仅包裹完整 token（`Hexo`、`Git`、`Node.js v12`、`npm`、`cnpm`、`hexo-cli`、`hexo clean && hexo g && hexo s`、`4000`、`Github`、`SSH`、`hexo-deployer-git`、`_config.yml`、`hexo d`、`github.io`），无词内拆分；截图 `after-production.png` 为最终 1280×900 效果。
- 回退：compose image 改回 `:2.0.7-beta03` 并 `up -d`；本次无数据库 schema 迁移。
- 证据：`docs/baselines/ai-summary-display/verification.json` 的 `release` 与 `followUp` 段；发布日志与服务器命令记录保存在本次会话与计划记录中。

## 剩余问题

- 发布后需在 Console 重新保存一次摘要或等待回退路径生效：修复后的前台会直接显示既有列内摘要，无需重新生成。
- Console 主表单保存仍可能用旧 `blog_extensions` 覆盖 canonical 摘要；读取端回退保证展示不丢，下一次保存摘要会重新投影。是否把摘要纳入主表单同步属后续产品决定，本次不扩大范围。

