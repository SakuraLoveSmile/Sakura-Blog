# BLG-20261110-06 发布记录：v2.1.6

- 状态：已发布并上线生产（界面待用户确认）
- 日期：2026-11-10（Asia/Taipei）
- 版本依据：最新稳定版 `v2.1.5`；`v2.1.6-beta01` 为下一个 patch 的预发布并已上线，本次收口为正式 `v2.1.6`
- 功能记录：`docs/plans/BLG-20261110-06-friends-heading-hover.md`、`docs/plans/page-editor-wysiwyg.md`
- 证据：本文件、`docs/baselines/friends-heading-hover/`

## 发布内容

- `11301e8e` feat: 页面编辑器「本站资料」只读提示（EDIT03 后续）
- `d4a875a8` fix: 友链分区标题悬停不再出现整块强调色与 1px 链接色下划线（BLG-20261110-06）
- 其余为 `v2.1.6-beta01` 预发布、NAS 验证与任务状态记录

## 晋级与发布

- PR [#75](https://github.com/SakuraLoveSmile/Sakura-Blog/pull/75)（dev→main）：console / go / lint / security / build 通过，e2e 20m44s 通过；`image`、`release-source` 在 dev→main 阶段跳过属预期。合并 `304379a3`。
- PR [#76](https://github.com/SakuraLoveSmile/Sakura-Blog/pull/76)（main→release）：八项全绿（e2e 18m01s、image 2m05s、release-source 12s）。合并 `4345c752`；`git diff origin/main origin/release` 为空。
- tag `v2.1.6`（GPG 签名，key `979B6BCF…3F3AD989`，`git tag -v` Good signature）打在 `4345c752` 并推送。

## 镜像与分发

- [Docker Release 42828611658](https://github.com/SakuraLoveSmile/Sakura-Blog/actions/runs/42828611658) success（13:53:00Z → 13:58:25Z）。
- 镜像 `ghcr.io/sakuralovesmile/sakura-blog:2.1.6`，digest `sha256:9f4cf91a3f97fcd5a5e8ff72a6421645b37663f823d97354127c0c89ccaa3ab5`。
- 镜像标签核对：`org.opencontainers.image.version=2.1.6`、`revision=4345c752…`、`created=2026-11-10T13:53:24Z`；`docker run --rm …:2.1.6 -h` 退出码 0。
- GitHub Release `v2.1.6`：`prerelease=false`、`draft=false`、0 资产。

## 生产升级（2026-11-10 21:59 CST）

- 目标：`VM-0-12-ubuntu`，compose `/opt/1panel/docker/compose/blog/docker-compose.yml`，宿主端口 `8081:8080`。
- 升级前：容器 `ghcr.io/…:2.1.6-beta01`（Up 6h，RestartCount 0），`/readyz` 200，根分区 51%。
- 变更：先 `docker pull` 新镜像（digest 与 GHCR 一致），备份 compose 为 `docker-compose.yml.before-2.1.6`，把 `image` 改为 `:2.1.6`，`docker compose up -d` 重建容器。
- 升级后：
  - 容器 `ghcr.io/…:2.1.6` running，RestartCount 0，镜像 digest 与 GHCR 一致。
  - `/readyz` 200、`/` 200、`/settings` 200、`/console/` 200、`/rss.xml` 200；启动日志无 error/panic。
  - 线上 `/friends` 已加载新样式 `friends-page.css?v=766935b1`；headless Chrome hover 实测锚点 `background: transparent`、`border: 0`，标题不再被色块遮挡、无链接色细线。
- 本次无数据库 schema 迁移。回滚：compose `image` 改回 `:2.1.6-beta01` → `docker compose up -d`（无需恢复数据；回退文件 `docker-compose.yml.before-2.1.6`）。

## 未验证

- Console「检查更新」卡片里的当前版本需管理员登录后确认；本次以镜像 tag、OCI `version` 标签与运行 digest 作为版本证据。
