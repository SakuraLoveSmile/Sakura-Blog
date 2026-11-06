# BLG-20261106-02 发布与上线证据

对应任务记录：[docs/plans/retire-console-segment-pages.md](../../plans/retire-console-segment-pages.md)。本目录保存 `v2.0.9-beta01` 测试版本发布与生产升级的可复核证据；命令输出均为 2026-11-06（Asia/Taipei）实测。

## 证据文件

- `release-verification.json`：tag 签名与指向、PR 链、Docker Release run、GHCR digest 与稳定标签基线对照、GitHub Release 预发布状态、镜像可启动检查。
- `deploy-verification.json`：生产升级前后的镜像、备份与 md5、compose 改动、容器与健康检查、Console 产物一致性；含回退方式。

## 关键结论

- tag `v2.0.9-beta01`（签名，指向 release 合并提交 `7e930138`）触发 Docker Release `42196838286` success，镜像 `ghcr.io/sakuralovesmile/sakura-blog:2.0.9-beta01` digest `sha256:07287aee…`。
- 预发布策略生效：只新增 `2.0.9-beta01` 与 `sha-7e93013`，`latest` / `2` / `2.0` / `2.0.8` 的 digest 保持 `sha256:64c36909…` 不变；GitHub Release 为 Pre-release 且不是 Latest，`v2.0.8` 仍是 Latest。
- 服务器 `docker run --rm …:2.0.9-beta01 -h` 退出码 0；生产容器冷停备份后切换到该镜像，running、restarts 0、镜像 ID 与 GHCR digest 一致。
- 宿主与公网 `/readyz` 均 200；公网 `/settings` 显示 `Blog v2.0.9-beta01`；`/`、`/console/`、`/uc/`、`/sitemap.xml`、文章页均 200，容器近 10 分钟错误行 0。
- 生产 Console 入口 chunk `/console/assets/index-COV9z3AH.js` 的 sha256 与本机同一提交的生产构建逐字一致（`52d5eb24…`），构建产物不再包含 `MenuView` / `AppearanceView` / `AuthorsView` chunk。
- 备份：`/home/ubuntu/blog-release-v2.0.9-beta01/`（0700）含 `docker-compose.yml` 与冷停后的 `blog.db`，db md5 `0263c296…` 与源一致，停机后 WAL/SHM 已 checkpoint 移除；本次无数据库 schema 变更。
- 回退：compose `image` 改回 `ghcr.io/sakuralovesmile/sakura-blog:2.0.8` 后 `docker compose up -d`；数据回退用上述备份。
