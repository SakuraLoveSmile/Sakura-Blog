# v2.0.7-beta01 发布前服务器基线（只读）

- 采集时间：2026-11-02T19:06+08:00（2026-11-02T11:06Z），采集窗口 19:03–19:06
- 服务器：ssh 别名 `blog-prod`（ubuntu），Ubuntu 24.04.4 LTS，内核 6.8.0-124，时钟 +08:00
- 采集方式：全部只读。docker 相关命令经 `sudo -n`（ubuntu 不在 docker 组），未 pull/push、未重启、未改服务器
- 结构化证据：同目录 `server-preflight.json`

## 1. 生产现状

| 项目 | 值 |
| --- | --- |
| compose 目录 | `/opt/1panel/docker/compose/blog` |
| compose 文件 | `docker-compose.yml`（341 B / 15 行，单服务 `blog`） |
| 当前 image 行 | `image: ghcr.io/sakuralovesmile/sakura-blog:2.0.6` |
| 容器名 / 端口 | `blog` / `0.0.0.0:8081->8080/tcp`（另 `[::]:8081`） |
| 重启策略 | `unless-stopped`，无 healthcheck |
| 容器状态 | running，`Up 42 minutes`，StartedAt 2026-11-02T10:23:22Z |
| 重启次数 | 0（OOMKilled=false，ExitCode=0） |
| 运行镜像 digest | `sha256:944df8d864c54c9021bcf4b284b00317be676946b6f89cc3b5f1d1849a734392` |
| 镜像 label | `org.opencontainers.image.version=2.0.6`，`revision=400ada097ebe6b993b3a0b36df3e1ab213305655`，`created=2026-11-02T10:16:09.861Z` |
| 版本证据 | 公开 `/settings` 页面渲染 `博客框架 Blog v2.0.6`；容器 env 无 VERSION 键 |

镜像没有独立的 `VERSION` / `GIT_COMMIT` / `BUILD_TIME` Docker label（Dockerfile 里是 build-arg，经 ldflags 写入二进制），版本以 `org.opencontainers.image.version` 与公开页面为准。

## 2. 健康与性能

| 探测 | 结果 |
| --- | --- |
| 宿主 `http://127.0.0.1:8081/readyz` | 200，`{"ok":true}`，0.73/0.76/0.80 ms |
| 公开 `https://blog.xn--fhqths51enha.cn/readyz` | 200，`{"ok":true}`，83–98 ms，`server: openresty` |
| 公开首页 | 200，0.41 s |
| 公开 `/settings` | 200，text/html，104286 B，显示 `Blog v2.0.6` |
| 未登录 `/api/v1/system/update/check` | 401（符合预期） |

## 3. GHCR 稳定标签基线（判断「未被移动」的唯一依据）

来源：匿名 token + `GET /v2/.../manifests/<tag>` 响应头 `docker-content-digest`（mediaType 为 OCI image index）。两轮采集（19:04Z、19:06Z）一致。

| tag | docker-content-digest |
| --- | --- |
| `latest` | `sha256:944df8d864c54c9021bcf4b284b00317be676946b6f89cc3b5f1d1849a734392` |
| `2` | 同上 |
| `2.0` | 同上 |
| `2.0.6` | 同上 |

- tags 列表共 29 个（无分页），`2.0.7`、`2.0.7-beta01`、`2.0.6-beta01` 均返回 HTTP 404（不存在，符合发布前预期）。
- 平台：`linux/amd64`（manifest `sha256:62bbb499…`）+ attestation。
- 服务器本地 `2.0.6` 镜像 RepoDigest 与上表完全一致 → 生产运行的正是 GHCR 当前稳定镜像。
- **注意**：服务器本地 `latest` 标签仍是 5 周前的 v1.7.x 缓存（`sha256:c5439b90…`），不能用于任何标签移动判断。

## 4. GitHub 侧基线

- `v2.0.6` Release：`isPrerelease=false`、`isDraft=false`、Latest，publishedAt `2026-11-02T10:21:40Z`，targetCommitish `main`。
- `docker-release.yml` 最近两次运行均 `success`：v2.0.6（run 41427360218，headSha `400ada0…`）、v2.0.5（run 41420700754）。
- 远端 tag 最新为 `v2.0.6`，无 `v2.0.7*`。

## 5. 磁盘与备份

- `/dev/vda2`：50G，已用 40G，可用 7.7G，**84%**（/、/opt、/var/lib/docker 同一根盘）。
- docker：镜像 60 个共 16.39 GB（可回收 9.338 GB），构建缓存 4.716 GB（可回收 4.064 GB）；本次只读未清理。
- 备份目录 `data/backups`：7 份 zip，共 317 MB；最新 `blog-backup-20261102-102323.zip` 47,333,471 B（18:23+08，采集时 43 分钟前），来自 2.0.6 升级窗口。
- 20 份历史 `.restore-*.json`，最新状态 `succeeded`，未发现 running/pending；`maintenance.lock` 无持锁进程。
- 升级前最近一份 compose 备份为 `docker-compose.yml.bak-before-2.0.6`，其中 image 行为 `2.0.5` → **回退目标 2.0.5**（本地已有该镜像）。

## 6. 发布约束与待验证

工作树 `codex/fix/20261102-blg-20261102-05-beta-release-tags`（HEAD `4d4f9633`，与 origin/dev 同步）有未提交改动：`.github/workflows/docker-release.yml`、`deploy/README.md`。其策略为：版本串含 `-` 判定 prerelease，**只发完整版本标签 + sha 标签，不生成 `2.0.7`/`2.0`/`2` 别名、不移动 `latest`，GitHub Release 标 prerelease**。

发布后核对（待第二阶段之后执行）：
1. `2.0.7-beta01`、`sha-<短哈希>` 出现；
2. `latest`/`2`/`2.0`/`2.0.6` 四个 digest 仍等于本文件第 3 节值；
3. GitHub Release `v2.0.7-beta01` 的 `isPrerelease=true` 且不是 Latest。

未验证：镜像可启动性、升级后生产验证、Console 登录态版本页、备份恢复演练、openresty 代理配置。详见 JSON `notVerified` / `risks`。

## 7. 操作提示（供后续阶段）

- `ubuntu` 不在 docker 组：所有 docker/compose 命令需 `sudo`。
- 升级命令形态：改 compose `image` 行为目标版本后 `sudo docker compose pull && sudo docker compose up -d`，再探测 `http://127.0.0.1:8081/readyz` 与公开页面版本。
- 磁盘余量 7.7G 偏低但仍可完成一次约 315 MB 的拉取；不建议在同窗口并发执行大规模构建或清理。
