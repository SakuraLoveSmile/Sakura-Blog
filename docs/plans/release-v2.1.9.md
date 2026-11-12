# Release v2.1.9：友链页「失联友链」分区标题居中

- 状态：已验收（发布与生产升级完成，生产页面实测居中）
- 日期：2026-11-12（Asia/Taipei）
- tag：`v2.1.9` → `3765a5f733307d03384051b4210b8aff4e396eb2`（`release` 的 PR 合并提交）
- 仓库：`SakuraLoveSmile/Sakura-Blog`

## 发布内容

| 类型 | 内容 | 任务 |
| --- | --- | --- |
| fix | 友链页「失联友链」分区标题恢复居中：story 文章样式的高特异性 h2 margin 清零左右外边距，`friends-page.css` 补 `margin-inline: auto` | 20261112 本地修复 |

详细症状、根因与验收清单见 `docs/baselines/friends-unavailable-heading/README.md`。

## 流程与证据

1. **提交**：`dev` 直接提交 `2b2ef7f8`（pre-commit hook 跑 `make test` 全绿后落盘），只包含 `friends-page.css` 与本次基线目录，未夹带工作区里其他未提交改动。
2. **PR #81 dev → main**：run `43164258935` 全部通过（console 1m27s、go 5m0s、lint 2m57s、security 4m46s、build 1m45s、e2e 19m54s），合并提交 `6fadb3ec`。
3. **PR #82 main → release**：run `43166595924` 全部通过，其中 `release-source` pass、`image` pass（2m2s，不推送）、e2e pass（20m48s），合并提交 `3765a5f7`；`release` 树与 `main` 树无差异。
4. **tag `v2.1.9`**：GPG 签名的 annotated tag 推送到 `3765a5f7`；`Docker Release` run `43169182702` success（5m47s）。
5. **GitHub Release `v2.1.9`**：工作流自动创建，无资产、非 prerelease，用于 Console「检查更新」的更新日志。
6. **GHCR**：`2.1.9`、`2.1`、`2`、`latest` 指向同一 manifest `sha256:10314871…`；服务器拉取镜像 `sha256:81e3ba4f…`。
7. **镜像冒烟**：服务器 `docker run --rm ghcr.io/sakuralovesmile/sakura-blog:2.1.9 -h` 退出码 0。

## 生产升级

- 升级前快照：`/home/ubuntu/blog-release-v2.1.9/pre-2.1.9-20261112T144208/`（停容器后拷 `blog.db`、`app.yml`、`site.yml`；停服后 WAL 已 checkpoint，无 `-wal`/`-shm`）。`sha256`：`blog.db = a4558556…`、`app.yml = 82865d0c…`、`site.yml = 4084c0d9…`（`site.yml` 与 v2.1.7 时一致）。
- compose 备份：`docker-compose.yml.before-2.1.9`；`image` 由 `…:2.1.8` 改为 `…:2.1.9`。镜像在停服前已 `docker pull`，停服窗口只有快照与 `up -d`。
- 完成判定：容器 `Up`、`RestartCount = 0`，日志只有 `listening on http://localhost:8080`，无 panic/迁移错误；宿主 `http://127.0.0.1:8081/readyz` = `{"ok":true}`；启动时自动备份 `blog-backup-20261112-064215.zip`；`/`、`/friends`、`/settings/`、`/readyz` 均 200，公开「关于」显示 `Blog v2.1.9`。
- 功能验证（生产）：`/friends` 加载 `friends-page.css?v=6b957e02`；Playwright 在 390 / 732 / 1280 三档测量「失联友链」中心相对正文列偏移均为 0（发布前为 -96.9 / -240.3 / -216.3），同页其余三个分区标题偏移仍为 0，无 pageerror。截图 `docs/baselines/friends-unavailable-heading/production-732-section.png`。
- 回退：把 compose 的 `image` 改回 `…:2.1.8` 后 `up -d`。本版本只改 CSS、无 schema 变化，回退不需要恢复数据；如需完全回到升级前状态用上面的快照。

## 未验证与限制

- 只核对了友链页与公开页面，未逐屏复核 Console 内部页面；版本以公开页 `Blog v2.1.9` 与容器镜像标签为准。
- 移动端只验证 390 / 732 两档视口，未在真机浏览器复核。
