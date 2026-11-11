# Release v2.1.7：站点设置页个人资料与友链多角标

- 状态：已验收（发布与生产升级完成，等待用户用自己的账号体验登录态保存）
- 日期：2026-11-11（Asia/Taipei）
- tag：`v2.1.7` → `77d765af239e5ae4288043af148bea70a7d78698`（`release` 的 PR 合并提交）
- 仓库：`SakuraLoveSmile/Sakura-Blog`

## 发布内容

| 类型 | 内容 | 任务 |
| --- | --- | --- |
| feat | 站点设置页补齐「个人资料」：原生账号下 `/settings/` 渲染头像 + 昵称/Email/URL + 保存/退出登录；新增 `POST /api/auth/profile`，`users.website` 老库自动补列 | BLG-20261111-03 |
| fix | 设置页固定 tech 排版：全站 `story` 时标题不再居中、正文不再首行缩进 | 20261111 本地修复 |
| feat | 友链最多三枚手填角标（`links.labels` JSON，首枚仍存 `label`/`color`） | BLG-20261111-02 |
| feat | 仪表盘头部控件统一为无边框幽灵工具组 | BLG-20261110-07 |
| chore | Go 工具链 1.26.10 → 1.26.11（修 GO-2026-6859 `net/http`） | 本次发布阻塞修复 |
| test | 幽灵工具组刷新态底色断言不受 CSS 过渡影响（D10 在 CI 稳定失败） | 本次发布阻塞修复 |

## 流程与证据

1. **同步 origin/main 回 dev**：`dev..origin/main` 由 1 变 0（同步分支 `codex/chore/20261111-BLG-20261111-04-sync-main`，`make merge-check` 通过后合入并清理）。
2. **CI 第一轮失败**：`go` 作业的 `govulncheck` 报 `net/http@go1.26.10` 命中 GO-2026-6859（Fixed in go1.26.11）。属新补丁发布后同样代码从绿变红，与功能无关；把 `go.mod` 提到 1.26.11 后本地 `make test` 绿、`govulncheck` 0 个被调用漏洞（`c3ce24fa`）。
3. **e2e 抖动**：`dashboard-header-controls.spec.ts:225` 连续两个 run 读到 `rgba(27,27,30,0.04)` 这类过渡中间值。测量前临时关闭 transition 取最终态（`928886bc`），断言语义不变；本地单跑该用例通过，CI 随后 e2e 全绿。
4. **PR #77 dev → main**：run `43000572484` 全部检查 pass（go、lint、security、console、build、e2e 24m41s，242 passed），合并提交 `ef273c49`。
5. **PR #78 main → release**：run `43004299546` 全部通过，其中 `release-source` pass、`image` pass（1m35s，不推送）、e2e pass（20m19s），合并提交 `77d765af`；release 树与 main 树无差异。
6. **tag `v2.1.7`**：annotated tag 推送到 `77d765af`；`Docker Release` run `43007249328` success（5m25s）。`release.yml` 不存在，发布产物只有镜像。
7. **GitHub Release `v2.1.7`**：工作流自动创建，无资产、非 prerelease，用于 Console「检查更新」的更新日志。
8. **GHCR**：`ghcr.io/sakuralovesmile/sakura-blog` 现有 `2.1.7`、`2.1`、`2`、`latest`（以及历史标签）。
9. **镜像冒烟**：服务器 `docker run --rm ghcr.io/sakuralovesmile/sakura-blog:2.1.7 -h` 退出码 0。

## 生产升级

- 升级前快照：`/home/ubuntu/blog-release-v2.1.7/pre-2.1.7-20261111T185812/`（暂停容器后拷 `blog.db`、`blog.db-wal`、`blog.db-shm`、`app.yml`、`site.yml`，`sha256sum` 已记录；`blog.db` = `f90992bc…`、`site.yml` = `4084c0d9…`）。另有应用自动备份 `data/backups/blog-backup-20261110-135954.zip`。
- 变更：`docker-compose.yml` 的 `image` 由 `…:2.1.6` 改为 `…:2.1.7`，`docker compose pull && up -d`。容器 `Up`，`RestartCount = 0`，日志只有 `listening on http://localhost:8080`，无 panic/迁移错误。
- 完成判定：宿主 `http://127.0.0.1:8081/readyz` = `{"ok":true}`；公开 `/settings/` 的「关于」显示 `Blog v2.1.7`；`/`、`/settings/`、`/readyz` 均 200（`/friends/` 301 → `/friends`）。
- 功能验证：线上 `/settings/` 分区为 个人资料 → 缓存 → 关于，`data-provider="native"`，资料区 624×364 与基准一致；未登录点保存返回「请先登录后再保存个人资料」（`data-state=failure`），没有写入。证据见 `docs/baselines/BLG-20261111-03-settings-profile/production-verification.json` 与三张生产截图。
- 回退：把 compose 的 `image` 改回 `…:2.1.6` 后 `up -d`。本版本 schema 只新增 `users.website`（旧二进制按列名读取，忽略该列），回退不需要恢复数据；如需完全回到升级前状态用上面的快照。

## 未验证与限制

- 未在生产用读者账号登录并保存资料：线上只验证了公开页面与未登录分支，登录态保存往返的证据来自隔离站点（`docs/baselines/BLG-20261111-03-settings-profile/`）。
- 未逐屏核对 Console 内部页面；版本以公开页的 `Blog v2.1.7` 与容器镜像标签为准。
- 「关于」仍只有「博客框架」「主题版本」两行；示例站多出的「隐私政策」「备用站」属于主题配置内容（`profiles.settings.about.items`），需要真实链接后再加。
- 侧栏「设置」挂件对原生账号仍不显示读者头像/昵称（只对 artalk/waline/twikoo 生效）。
