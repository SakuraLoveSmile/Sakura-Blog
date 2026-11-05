# BLG-20261104-03：v2.0.7 发布与生产升级

- 项目 / 任务编号：Blog / BLG-20261104-03
- 状态：已验收（用户 2026-11-04 授权“更新线上，发布新版本”）
- 日期：2026-11-04（Asia/Taipei）
- 发布分支：`release`；tag：`v2.0.7`（GPG 签名，`69cad0f8`）
- 发布内容：BLG-20261103-01（系统设置页重排）、BLG-20261104-01（移除已由原生实现替代的第三方服务）、编辑器标签与工具栏修复
- 版本依据：此前已发布 `v2.0.7-beta01`～`v2.0.7-beta05` 预发布；本次为 `v2.0.7` 正式版，正式版标签会移动 `latest` / `2` / `2.0`

## 发布前同步

- 本地 `dev` 已推送（`7aaeacdc`），先同步 `origin/main` 历史（`f5413146`，main 有、dev 无）：
  - 从 `origin/main` 建 `codex/chore/20261104-blg-20261104-02-sync-main`，经 `make merge-check` 合入 dev，合并提交 `16accef7`；内容零差异。
- 功能分支 rebase 到 dev 后经 `make merge-check` 合入：`e8eb8a26`（feat）、`b0ae24cb`（fix）；`dev` 推送为 `b0ae24cb`。
  - 首次 push 后 `dev..origin/main` 为 0，满足发布前置。

## 晋级与发布

- PR #46 `dev → main`（`feat: 系统设置页重排、移除第三方服务并修复编辑器标签`）：必需检查 go / lint / security / console / e2e（22m28s）/ build 全部通过后合并为 `82611cfd`。
- PR #47 `main → release`（`release: 发布 v2.0.7`）：检查 release-source / image / go / lint / security / console / build 通过，e2e 23m0s 通过后合并为 `69cad0f8`。
  - `main→release` PR 合并时 `merge_state=clean`，无需 release 历史同步 PR；本次创建的同步分支 PR #48 随后关闭（未合并），release 历史未进入 main，后续发布前若出现 BEHIND 仍需按既有方式同步。
- tag：`v2.0.7`（GPG 签名，key `979B6BCF…3F3AD989`）打在 `69cad0f8` 并推送。
- Docker Release：run `41860531679` success（约 5 分钟，2026-11-04T15:42:30Z→15:47:37Z），创建无资产 GitHub Release `v2.0.7`（非 prerelease）。

## 镜像与标签核对

- 镜像：`ghcr.io/sakuralovesmile/sakura-blog`
- digest：`sha256:839f243de1ddd1f0917177678d4f0558d26943e9fe517e25493772808a615ed6`
- 同一 digest 的标签：`2.0.7`、`2.0`、`2`、`latest`、`sha-69cad0f`
- `2.0.6` 保持原 digest `sha256:944df8d8…a734392` 不变
- 服务器 `docker run --rm ghcr.io/sakuralovesmile/sakura-blog:2.0.7 -h` 退出码 0

## 生产升级

- 服务器：`blog-prod`（Ubuntu，compose 目录 `/opt/1panel/docker/compose/blog`，宿主端口 8081）。
- 升级前：容器镜像 `2.0.7-beta05`，image ID `sha256:c2afa6db…`，running，restarts 0。
- 备份：`/home/ubuntu/blog-release-v2.0.7/`（0700，ubuntu:ubuntu）保存 `blog.db` 与 `docker-compose.yml`。
  - 先在旧容器运行时 pull 新镜像，再 `docker compose stop blog` 冷停，复制 `blog.db`；干净停机后 WAL/SHM 已 checkpoint 移除，备份只保留 `blog.db`，避免陈旧 WAL 混入恢复。
  - 备份 `blog.db` md5 `8df89326bdd8cef8d494bd8a3eda1a20`，与冷停后的源文件一致。
- 变更：compose `image` 仅一行改为 `ghcr.io/sakuralovesmile/sakura-blog:2.0.7`，`docker compose up -d` 重建容器。
- 升级后：容器镜像 `2.0.7`，image ID `sha256:839f243d…`（与 GHCR digest 一致），running，restarts 0；宿主 `/readyz` 200。

## 生产验收（本机网络实测）

- 公网 `https://blog.xn--fhqths51enha.cn`：`/readyz`、`/`、`/console/`、`/uc/`、`/settings` 均 200；`/settings` 显示 `Blog v2.0.7`。
- 公网文章页 `/post/who-am-i`（Playwright，1280×900）：
  - 对 siteinfo / algolia / giscus / utterances / beaudar / twikoo / waline / artalk 的请求 0 个；
  - 第三方评论节点 0 个，原生评论区 `#blog-comments/#cmt-body` 存在；
  - 页面 error / console error 0 条。
- 未用生产账号登录 Console 做写操作；Console 登录页与 SPA 入口 200，设置页交互在隔离环境（8098/8096）已验证。

## 回退

- 镜像回退：compose `image` 改回 `:2.0.7-beta05`，`docker compose up -d`。
- 本次无数据库 schema 迁移；若需数据回退，停止容器后以备份 `blog.db` 覆盖 `data/blog.db`（WAL/SHM 需一并移除）再启动。

## 收尾

- 已停止 8098 隔离预览，删除功能分支 worktree、sync-main / sync-release-history 临时 worktree 与本地临时分支；远端分支不删除。
- 发布后 `dev` 追加本发布记录并推送；`dev` 领先 `main` 属正常，下一次发布前再同步。
