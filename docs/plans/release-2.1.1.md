# BLG-20261109-01：GitHub 卡片统计图标修复与 v2.1.1 正式发布

<!-- 状态词只用 未开始 / 实现中 / 待体验 / 已验收；已验收须用户明确确认。 -->

- 项目 / 任务编号：Blog / BLG-20261109-01
- 状态：待体验（修复、两级 PR、签名 tag、镜像发布、生产升级与线上验收均已完成，等待用户确认视觉与体验）
- 日期：2026-11-09（Asia/Taipei）
- 分支 / worktree：`codex/fix/20261109-BLG-20261109-01-ghcard-stat-icons`（修复，已合入 dev）；`codex/chore/20261109-BLG-20261109-01-sync-main`、`codex/chore/20261109-BLG-20261109-01-sync-release`（历史同步，已合入）
- 发布分支：`release`；tag：`v2.1.1`（GPG 签名，key `979B6BCF…3F3AD989`，指向 `038ffab8`）
- 版本依据：用户 2026-11-09 明确要求「完成任务、合并到 dev、发布新版本、更新线上」；版本取 `v2.1.1`（`v2.1.1-beta01` 的稳定版，本次仅含图标修复，无 schema 迁移）
- 基线证据：`docs/baselines/ghcard-stat-icons/`（修复前后浏览器截图）、`docs/baselines/release-2.1.1/`（发布校验、生产验收 JSON、生产卡片截图）

## 问题与修复

- 现象：文章 `/post/pi-desktop-custom-dev-log` 的 GitHub 仓库卡片里，fork 图标看起来没有显示全。
- 定位：统计行字号 `0.75rem`（桌面 12px），⭐/🍴/📄 均为 emoji；CDP 查得字形来自 Apple Color Emoji。单独以 12–48px 渲染 🍴，墨迹宽高比稳定在 0.40–0.46（字形本身细长），12px 时只有 11×24 设备像素，同行的 ⭐ 是 24×26；`.repo-stats` 无 overflow/固定高度，排除 CSS 裁切。
- 修复：`internal/post/shortcode_github.go` 用内联 Octicon SVG（`star-fill` / `repo-forked` / `law`）替换 emoji，`fill=currentColor`、`aria-hidden`，星标与 fork 数量补 `aria-label`；`modules/stellar/web/assets/css/tag-plugins.css` 增加 `.repo-stat-icon` 尺寸 `1.1em`、右侧 `0.35em` 间距、`vertical-align: -0.16em`。
- 测试：`internal/post/shortcode_preview_test.go`、`modules/console/api/preview_test.go`、`modules/stellar/routes/stellar_preview_github_test.go` 断言改为新结构。
- 验收：1280 DPR2/DPR1、390 DPR2、强制浅色主题截图；图标 13.19×13.19（390 下 14.02），许可证在手机上正常换行。

## 发布前同步

- 起点 `origin/dev` = `0ee38efe`，`origin/main` 有 6 条 `dev` 缺少的合并历史提交（strict 保护会阻断发布 PR）。
- 从 `origin/main` 建同步分支 `codex/chore/20261109-BLG-20261109-01-sync-main`，`make merge-check` 通过后合入 `dev`（合并提交 `65d664ae`），`dev..origin/main` = 0。
- 修复分支 `codex/fix/20261109-BLG-20261109-01-ghcard-stat-icons`（提交 `7067c806`）经 `make merge-check` 合入 `dev`（合并提交 `b1c98b0a`）；推送 `origin/dev`。

## 晋级与发布

- PR [#61](https://github.com/SakuraLoveSmile/Sakura-Blog/pull/61)（dev→main，"发布 v2.1.1：GitHub 卡片统计图标改用内联 SVG"）：go / lint / security / console / e2e（22m10s）/ build 全绿，合并 `e567215a`。
- PR [#62](https://github.com/SakuraLoveSmile/Sakura-Blog/pull/62)（同步 release 合并历史到 main）：`release` 上有历史 `main→release` 合并提交 `24496562`，为满足 `strict` 保护，从 `main` 建同步分支 `git merge origin/release`；`git diff --stat origin/main HEAD` 为空（内容零差异），6 项必需检查全绿，合并 `93dad705`；此后 `main..origin/release` = 0。
- PR [#63](https://github.com/SakuraLoveSmile/Sakura-Blog/pull/63)（main→release，"release: 发布 v2.1.1"）：go / lint / security / console / e2e（18m16s）/ build / release-source / image 全绿，合并 `038ffab8`；合并后 `git diff origin/main origin/release` 为空。
- tag `v2.1.1`（GPG 签名，`git tag -v` 通过）打在 `038ffab8` 并推送。
- [Docker Release 42568753791](https://github.com/SakuraLoveSmile/Sakura-Blog/actions/runs/42568753791) success（05:41:32Z→05:46:33Z，约 5 分钟），创建 GitHub Release `v2.1.1`：`prerelease=false`、`draft=false`、`target=main`、更新日志 787 字符；`releases/latest` 变为 `v2.1.1`。

## 镜像与标签核对

- 镜像：`ghcr.io/sakuralovesmile/sakura-blog:2.1.1`
- digest：`sha256:2e6dbe0a0ea0d74056234244ce4aac3bab859d4a392005bd8f25398b436da4c1`
- label：`org.opencontainers.image.version=2.1.1`、`revision=038ffab8688ee9072b2c543a46a1f5f02c292fba`、`created=2026-11-09T05:41:56.113Z`
- 别名 `2.1` / `2` / `latest` 与 `2.1.1` 指向同一 manifest；`sha-038ffab` 存在
- 稳定版基线：`2.1.1-beta01` digest `sha256:a9464299…aff82c` 未移动
- 服务器 `docker run --rm ghcr.io/sakuralovesmile/sakura-blog:2.1.1 -h` 退出码 0，输出 usage（本机 Docker daemon 未启动，改在生产机执行）

## 生产升级

- 服务器：`blog-prod`（Ubuntu，compose 目录 `/opt/1panel/docker/compose/blog`，宿主端口 8081）。
- 升级前：compose 固定 `2.1.1-beta01`，容器 `49e24fc30e19`，image ID `sha256:a9464299…aff82c`，Up 17 小时，restarts 0；`/readyz` 200；根盘剩余 3.5G；最近应用备份 `blog-backup-20261108-130608.zip`；`/settings` 显示 `v2.1.1-beta01`。
- 备份：`/home/ubuntu/blog-pre-v2.1.1/`（0700，ubuntu:ubuntu）
  - `site-state-pre-2.1.1.tar.gz`（2.9MB，SHA256 `486f73e861044ebc049c4a68f0ecb9b5c7857a239c8f62d9c56dc89a2d924ee7`，含 `config/`、`content/`、`data/blog.db`，按上次瑕疵记录直接使用 `sudo tar`）
  - `docker-compose.yml.before-2.1.1`、`before.txt`（升级前镜像 / health / 磁盘 / 站点版本事实）
- 变更：compose `image` 一行改为 `ghcr.io/sakuralovesmile/sakura-blog:2.1.1`（先 `docker pull` 预拉取），`docker compose up -d` 重建容器。
- 停机窗口：05:49:17Z 冷停 → 05:49:21Z 新容器 running，约 4 秒，无 schema 迁移。
- 升级后：容器 `b1b5fc6ec21e`，镜像 `2.1.1`，image ID `sha256:2e6dbe0a…da4c1`（与 GHCR digest 一致），label `version=2.1.1`，restarts 0，running；宿主 `/readyz` 200。

## 生产验收（本机网络实测，2026-11-09T05:50Z）

- 公网 `https://blog.xn--fhqths51enha.cn`：`/`、`/readyz`、`/console/`、`/uc/`、`/settings` 均 200；`/settings` 显示 `v2.1.1`。
- 文章 `/post/pi-desktop-custom-dev-log`（Playwright 1280×900 DPR2 dark，networkidle）：
  - `.ghrepo-card` 盒子 424.2×110，背景 `rgb(39,37,32)`、1px 边框、12px 圆角、16px 内边距
  - 统计行 3 个 `.repo-stat-icon`，13.19×13.19，颜色 `rgba(255,255,255,0.5)`（跟随 `--text-p3`）；`aria-label` 为 `0 stars` / `0 forks`
  - fork 图标完整可辨（与修复前基线对比见 `docs/baselines/ghcard-stat-icons/compare-before-after-1280-dark-2x.png`）
  - console error / page error 均为 0；截图 `docs/baselines/release-2.1.1/production-ghrepo-card.png`
- 未用生产账号登录 Console 做写操作；Console「检查更新」未在登录态点击验证（版本来源 GHCR tag 与 GitHub Release 均已为 2.1.1）。

## 回退

- 镜像回退：compose `image` 改回 `ghcr.io/sakuralovesmile/sakura-blog:2.1.1-beta01`（本机仍有该镜像），`docker compose up -d`（本次无 schema 迁移，只切镜像即可）。
- 数据回退（如需）：停容器后以 `/home/ubuntu/blog-pre-v2.1.1/site-state-pre-2.1.1.tar.gz` 恢复 `config/`、`content/`、`data/blog.db` 再启动。

## 已知限制

- 本机 Docker daemon 未启动，镜像 manifest / 二进制启动检查在生产机完成。
- 未在手机真机复核；390px 视口的换行与不溢出已由发布前 Playwright 验收覆盖。
- `dev` 现在比 `origin/main` 少一条 PR 合并提交（`e567215a` 等），下一次发布前仍需按同名流程同步 `main` 历史。

## 收尾

- 本记录与 `docs/baselines/release-2.1.1/` 证据提交到 `dev` 并推送。
- 修复与同步 worktree、本地任务分支在核验改动已进入目标分支后删除；远端同步 / 发布分支保留（沿用既有做法）。
