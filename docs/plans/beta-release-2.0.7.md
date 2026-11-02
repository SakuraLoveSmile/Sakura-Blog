# v2.0.7-beta01 预发布与生产升级

任务编号：BLG-20261102-05。状态：实现中。

## 目标与范围

- 发布 `v2.0.7-beta01` 预发布版本：镜像 `ghcr.io/sakuralovesmile/sakura-blog:2.0.7-beta01`，并在镜像验证通过后升级生产服务器。
- 发布内容为当前 `dev` 上未进入 v2.0.6 的改动：`7db3ddb3` 独立页面不再展示 AI 摘要、`593bff3f` 文章管理批量发布 / 批量设为草稿。
- 为让预发布不破坏正式版标签，先修复 `docker-release.yml`：预发布 tag（版本串含 `-`）只发布自身完整版本标签与 `sha-` 标签，不生成 `X.Y` / `X` 别名、不移动 `latest`，GitHub Release 标记 prerelease。正式版行为完全不变。
- 范围不含：Console 更新检查对预发布 Release 说明的适配（`/releases/latest` 不含 prerelease，beta 的更新日志不会出现在 Console，作为已知限制记录）；不改动已发布的 v2.0.6 标签与镜像。

## 依据与决定

- 用户 2026-11-02 指示：发布 2.0.6 beta01；经说明「v2.0.6 正式版已于当日 18:21 发布」，用户选择改为发布 **2.0.7-beta01**（当前 dev 内容的预发布），并授权发布成功后升级生产服务器。
- 版本号语义：`v2.0.7-beta01` 比已发布的 `v2.0.6` 新、比将来的 `v2.0.7` 旧；`semverCompare` 支持 prerelease，安装正式版 2.0.6 的 Console 会提示有 2.0.7-beta01 可更新。
- 发布路径沿用 AGENTS.md「发布 release」：feature → `dev`（`make merge-check`）→ `dev→main` PR → `main→release` PR → 在 release 合并提交打 tag。
- 生产升级：compose 固定 `:2.0.7-beta01`，`docker compose pull && docker compose up -d`，探测 `/readyz` 并在 Console 核对版本；回退只改回 `:2.0.6`。本次无数据库 schema 迁移。

## 基线

- 代码：`dev` / `4d4f9633`（= 推送后的 `origin/dev`）。`main` / release 稳定内容为 `v2.0.6`（`400ada09`）。
- 稳定镜像标签基线（2026-11-02 19:03，匿名 GHCR token）：`latest`、`2`、`2.0`、`2.0.6` 的 `docker-content-digest` 均为 `sha256:944df8d864c54c9021bcf4b284b00317be676946b6f89cc3b5f1d1849a734392`。发布后必须保持不变。
- 生产基线：见 `docs/baselines/beta-release-2.0.7/server-preflight.json`（发布前只读核查）。

## 实施步骤与验收

1. 工作流修复（本任务代码改动，仅 `.github/workflows/docker-release.yml` + `deploy/README.md` 说明）；`make test`、`make merge-check` 合入 `dev` 并推送。
2. `dev→main` PR，CI 全绿后合并；`main→release` PR，CI（含 release-source、image 预构建）全绿后合并。
3. `release` 合并提交打签名 tag `v2.0.7-beta01` 并推送，触发 Docker Release。
4. 完成判定：Docker Release success；镜像可启动（服务器 `docker run --rm … -h` 退出码 0）；GHCR 出现 `2.0.7-beta01`，且 `latest`、`2`、`2.0`、`2.0.6` digest 与基线一致；GitHub Release `v2.0.7-beta01` 为 prerelease 且不是 Latest。
5. 生产升级：改 compose `image` → `docker compose pull && docker compose up -d` → `/readyz` 200、容器 running/restarts 0、Console 或容器内版本为 2.0.7-beta01；记录备份与回退方式。
6. 清理 worktree 与已合并本地分支（已推送的远端分支不删除），记录实测与未验证项。

## 验证边界

- 本机 Docker daemon 未运行，镜像内二进制启动验证在生产服务器执行（`docker run --rm … -h`）；不为此启动本地 Docker 或打包二进制。
- 不发版到其它环境，不改共享 `resource/`，不重写已发布 tag/Release。

## 实现与实测

（待补）
