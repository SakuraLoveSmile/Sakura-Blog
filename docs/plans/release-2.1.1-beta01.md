# BLG-20261108-02：v2.1.1-beta01 预发布与生产升级

<!-- 状态词只用 未开始 / 实现中 / 待体验 / 已验收；已验收须用户明确确认。 -->

- 项目 / 任务编号：Blog / BLG-20261108-02
- 状态：待体验（两级 PR、签名 tag、镜像发布、生产升级与线上验收已完成；beta 功能待用户体验确认）
- 日期：2026-11-08（Asia/Taipei）
- 发布分支：`release`；tag：`v2.1.1-beta01`（GPG 签名，key `979B6BCF…3F3AD989`，指向 `24496562`）
- 发布内容：笔记本/笔记前台可见（`b6c10929`）与 GitHub 仓库卡片渲染修复（`264f7cc5`）
- 版本依据：用户于 2026-11-08 明确选择 `v2.1.1-beta01`（未选 `v2.2.0-beta01`）
- 基线证据：`docs/baselines/beta-release-2.1.1/`（发布校验 JSON、生产验收 JSON、生产卡片截图）

## 发布前同步

- 起点：`origin/dev` = `09061da5`，本地 `dev` 另有两个未推送提交（`b6c10929` 笔记本前台可见、`264f7cc5` GitHub 卡片修复）。
- `origin/main` 有一条 `dev` 缺少的提交（上一轮发布合并 `f1bf77cf`）：从 `origin/main` 建同步分支 `codex/chore/20261108-BLG-20261108-02-sync-main`，`make merge-check` 通过后合入 `dev`（合并提交 `c238c317`），`dev..origin/main` = 0，推送 `origin/dev` = `c238c317`。
- 同步分支 worktree 随后复用于 release 历史同步，任务收尾时已删除；远端分支保留。

## 晋级与发布

- PR [#58](https://github.com/SakuraLoveSmile/Sakura-Blog/pull/58)（dev→main，"发布 v2.1.1-beta01：笔记本前台可见与 GitHub 卡片渲染修复"）：go / lint / security / console / e2e（22m43s）/ build 全绿，合并 `cdab977e`。
- PR [#59](https://github.com/SakuraLoveSmile/Sakura-Blog/pull/59)（同步 release 合并历史到 main）：`release` 上有历史 `main→release` 合并提交（`c73525eb`、`a0aa6114`、`7e930138`），`main` 缺这些提交会触发 `strict` 分支保护；按 v2.0.9 / v2.1.0 同名流程从 `main` 建分支 `git merge origin/release`（内容零差异，`git diff cdab977e 7c13a863` 为空），6 项必需检查全绿，合并 `38b4df4a`；此后 `release` 已是 `main` 祖先。
- PR [#60](https://github.com/SakuraLoveSmile/Sakura-Blog/pull/60)（main→release，"release: 发布 v2.1.1-beta01"）：go / lint / security / console / e2e / build / release-source / image 全绿，合并 `24496562`；合并后 `git diff origin/main origin/release` 为空。
- tag `v2.1.1-beta01`（GPG 签名，`git tag -v` 通过）打在 `24496562` 并推送。
- [Docker Release 42482614542](https://github.com/SakuraLoveSmile/Sakura-Blog/actions/runs/42482614542) success（2026-11-08T12:57:09Z→13:01:24Z，约 4m15s），创建无资产 GitHub Release `v2.1.1-beta01`：`prerelease=true`、`draft=false`、`target=main`、更新日志 474 字符；`releases/latest` 仍为 `v2.1.0`。

## 镜像与标签核对

- 镜像：`ghcr.io/sakuralovesmile/sakura-blog:2.1.1-beta01`
- OCI index digest：`sha256:a9464299fe2e28bea065778837711b72d21869290225b60b99ed8eeb69aff82c`；amd64 manifest `sha256:3fa25d78…90bd5c4`
- 镜像 label：`org.opencontainers.image.version=2.1.1-beta01`、`revision=244965626fa2a293e51db8ff26cc640f87624680`
- 新增标签仅 `2.1.1-beta01` 与 `sha-2449656`（同一 digest）；**未生成** `2.1.1` / `2.1` / `2` 别名
- 稳定标签 digest 未移动（发布前基线 = 发布后核对）：`latest`、`2`、`2.1`、`2.1.0` 均为 `sha256:c27e314e…be95d50`；`2.0.9` 为 `sha256:7c26a9c2…d4f568e`
- 服务器 `docker run --rm ghcr.io/sakuralovesmile/sakura-blog:2.1.1-beta01 -h` 退出码 0，输出 usage

## 生产升级

- 服务器：`blog-prod`（Ubuntu，compose 目录 `/opt/1panel/docker/compose/blog`，宿主端口 8081）。
- 升级前：容器镜像 `2.1.0`，image ID `sha256:c27e314e…be95d50`，running，restarts 0，2026-11-06 启动；`/readyz` 200；compose 固定 `2.1.0`；最近自动备份 `blog-backup-20261107-193848.zip`（46MB，2026-11-08 03:38）；`config` 28K、`content` 96K、`blog.db` 13M；根盘剩余 3.7G。
- 备份：`/home/ubuntu/blog-pre-v2.1.1-beta01/`（0700，ubuntu:ubuntu）保存
  - `site-state-pre-beta.tar.gz`（2.9MB，SHA256 `ba5a2daa89e0060c2309fec679c0e82b8974a61ccc6986c0398ca2b66983fc04`，含 `config/`、`content/`、`data/blog.db`）
  - `docker-compose.yml.before-2.1.1-beta01`、`before.txt`（升级前镜像/健康/端口事实）
- 变更：compose `image` 一行改为 `ghcr.io/sakuralovesmile/sakura-blog:2.1.1-beta01`（原文件备份为 `docker-compose.yml.bak-before-2.1.1-beta01`），`docker compose up -d` 重建容器。
- 升级后：容器镜像 `2.1.1-beta01`，image ID `sha256:a9464299…aff82c`（与 GHCR digest 一致），running，restarts 0；宿主 `/readyz` 200；容器 label `version=2.1.1-beta01`。
- 停机窗口：13:04:37Z 冷停 → 13:05:36Z 以 `2.1.0` 恢复（首次备份因 `config/`、`data/blog.db` 属 root 而 tar 失败，改用 sudo 重做）→ 13:06:04Z 重建 → 13:06:07Z 新镜像 running，累计停机约 62 秒。**过程瑕疵已记录**：权限假设错误导致多停了一次，后续同类升级应直接用 `sudo tar`。
- 本次无数据库 schema 迁移（仅功能与样式修复），回退只需切换镜像。

## 生产验收（本机网络实测，2026-11-08 13:06Z 后）

- 公网 `https://blog.xn--fhqths51enha.cn`：`/readyz`、`/`、`/console/`、`/uc/`、`/settings` 均 200；`/settings` 显示 `v2.1.1-beta01`。
- 首页 `<head>` 已包含 `/assets/css/tag-plugins.css`（本次修复的补充样式随页面加载）。
- 文章 `/post/pi-desktop-custom-dev-log/`（Playwright 1280×900 dark，networkidle）：
  - `.ghrepo-card` 盒子 427×110，背景 `rgb(39,37,32)`、1px 边框、12px 圆角、16px 内边距
  - 卡片内容完整：`SakuraLoveSmile/PI-Desktop`、描述「自用修改版」、⭐ 0、🍴 0、TypeScript、GNU LGPL v3.0、含头像
  - console error / page error 均为 0；截图 `docs/baselines/beta-release-2.1.1/production-ghrepo-card.png`
- 未用生产账号登录 Console 做写操作；beta 功能体验留给用户确认。

## 回退

- 镜像回退：compose `image` 改回 `ghcr.io/sakuralovesmile/sakura-blog:2.1.0`，`docker compose up -d`（本次无 schema 迁移，只切镜像即可）。
- 数据回退（如需）：停容器后以 `/home/ubuntu/blog-pre-v2.1.1-beta01/site-state-pre-beta.tar.gz` 恢复 `config/`、`content/`、`data/blog.db` 再启动。

## 已知限制

- 预发布 GitHub Release 是 prerelease，`/releases/latest` 不返回它，Console「检查更新」的更新日志不会展示 beta 说明（此前 beta 发布已记录的既有行为）；版本比较仍能识别 `2.1.1-beta01` 比 `2.1.0` 新。
- 生产环境 GitHub 请求正常（卡片带完整数据）；本地开发机因 Clash fake-ip 解析被 `internal/safefetch` 拒绝，卡片会降级为仅名称，与本发布无关。

## 收尾

- 本发布记录与基线证据提交到 `dev` 并推送；`dev` 领先 `main` 属正常，下一次发布前再同步。
- 同步分支 worktree 已删除，本地任务分支已删除；远端同步/发布分支未删除。
