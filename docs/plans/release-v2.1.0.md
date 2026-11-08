# BLG-20261106-01：v2.1.0 发布与生产升级

- 项目 / 任务编号：Blog / BLG-20261106-01（功能任务记录：`docs/plans/editor-writing-assistant.md`）
- 状态：待体验（两级 PR、签名 tag、镜像发布与生产升级已完成；Console 内的写作助手仍待用户实际体验确认）
- 日期：2026-11-07（Asia/Taipei）
- 发布分支：`release`；tag：`v2.1.0`（GPG 签名，`c73525eb`）
- 发布内容：文章编辑器 AI 写作助手（一期：选区助手）
- 版本依据：上一正式版为 `v2.0.9`；本次为功能新增，用户确认使用 `v2.1.0`

## 发布前同步

- 开始发布时 `dev` / `origin/dev` 为 `80ee2991`，`origin/main` 为 `4ecf80f2`，`origin/release` 为 `a0aa6114`。
- 功能分支 rebase 最新 `dev` 后提交 `ca07ffc1`（GPG 签名），`make merge-check` 在 feature 与 dev 组合态均通过，`dev` 快进合入。
- `origin/main` 有一条 `dev` 缺少的发布合并记录（`4ecf80f2`）：从 `origin/main` 建 `codex/chore/20261107-blg-20261106-01-sync-main`，经 `make merge-check` 合回 `dev`（合并提交 `082c3375`，树与 `ca07ffc1` 相同），随后 `dev..origin/main` 为 0 并推送 `dev`。

## 晋级与发布

- PR [#56](https://github.com/SakuraLoveSmile/Sakura-Blog/pull/56)（dev→main，"发布 v2.1.0：文章编辑器 AI 写作助手"）：go / lint / security / console / build / e2e 全部通过，合并为 `f1bf77cf`。
  - e2e 为 237 条中 236 passed、1 flaky：既有 `tests/e2e/editor-writing-state.spec.ts:53`（立即输入后保存/预览/切换编辑器）首跑「预览渲染失败」，重试后通过；未修改断言或重试配置。
- PR [#57](https://github.com/SakuraLoveSmile/Sakura-Blog/pull/57)（main→release）：release-source / image / go / lint / security / console / build / e2e 全部通过，合并为 `c73525eb`；`main` 与 `release` 的树差异为空。
  - e2e 为 237 passed、0 flaky（21.3m）。
- tag `v2.1.0`（GPG 签名，key `979B6BCF…3F3AD989`）打在 `c73525eb` 并推送。
- [Docker Release 42280497014](https://github.com/SakuraLoveSmile/Sakura-Blog/actions/runs/42280497014) success（2026-11-06T19:32:20Z→19:36:55Z，约 4m35s），创建无资产 GitHub Release `v2.1.0`（非预发布、Latest）。

## 镜像与标签核对

- 镜像：`ghcr.io/sakuralovesmile/sakura-blog`
- digest：`sha256:c27e314ebc2f92f84f2dc5cde0c026017ea2c94ac65a346cf245c706cbe95d50`
- 同一 digest 的标签：`2.1.0`、`2.1`、`2`、`latest`、`sha-c73525e`
- `2.0.9` 保持原 digest `sha256:7c26a9c2…4f568e` 不变
- 服务器 `docker run --rm ghcr.io/sakuralovesmile/sakura-blog:2.1.0 -h` 退出码 0

## 生产升级

- 服务器：`blog-prod`（Ubuntu，compose 目录 `/opt/1panel/docker/compose/blog`，宿主端口 8081）。
- 升级前：容器镜像 `2.0.9`，image ID `sha256:7c26a9c2…4f568e`，running，restarts 0。
- 备份：`/home/ubuntu/blog-release-v2.1.0/`（0700，ubuntu:ubuntu）保存 `site-state.tar`（864378880 字节，SHA256 `d6674dda…cd633c5`，0600 ubuntu:ubuntu）。
  - 冷停后 tar 整个 compose 目录；tar 前对停服后的 `blog.db` 执行只读 `quick_check`：`ok`，`user_version` 0。
- 变更：compose `image` 一行改为 `ghcr.io/sakuralovesmile/sakura-blog:2.1.0`（原文件备份为 `docker-compose.yml.bak-before-2.1.0`），`docker compose up -d` 重建容器。
- 升级后：容器镜像 `2.1.0`，image ID `sha256:c27e314e…be95d50`（与 GHCR digest 一致），running，restarts 0；宿主 `/readyz` 200。
- 维护窗口：19:38:34Z 开始 → 19:38:35Z 停服 → 19:38:44Z 容器启动 → 19:38:45Z readyz 200，停机约 10 秒。

## 生产验收（本机网络实测）

- 公网 `https://blog.xn--fhqths51enha.cn`：`/readyz`、`/`、`/console/`、`/uc/`、`/settings` 均 200；`/settings` 显示 `Blog v2.1.0`。
- 公网文章页 `/post/who-am-i`（Playwright，1280×900 与 390×844）：状态 200，console error / page error 均为 0；首页与设置页同样 0 条。证据见 `docs/baselines/editor-writing-assistant/production-*`。
- 配置权限未变（`app.yml` / `site.yml` 0600 blog:blog）；本次无数据库 schema 迁移。
- 未用生产账号登录 Console 做写操作：写作助手需要管理员登录，生产「对话模型」配置不属于本任务改动；功能体验留给用户在 Console 中完成，任务状态仍为待体验。

## 回退

- 镜像回退：compose `image` 改回 `ghcr.io/sakuralovesmile/sakura-blog:2.0.9`，`docker compose up -d`。
- 本次无数据库 schema 迁移；如需数据回退，停容器后以备份 `site-state.tar` 恢复 `config/`、`content/`、`data/` 再启动。

## 收尾

- 功能 worktree 与 sync-main worktree 已删除，本地任务分支已删除；远端分支未删除。
- worktree 内被 git 忽略的沙箱脚本 `.tmp/try-assistant.sh` 已归档到根 `.tmp/editor-writing-assistant-release/`（SHA256 见 `docs/baselines/editor-writing-assistant/worktree-preservation.json`），任务记录的体验方式仍可复现。
- 发布后 `dev` 追加本发布记录与证据并推送；`dev` 领先 `main` 属正常，下一次发布前再同步。
