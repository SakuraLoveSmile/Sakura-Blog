# 部署指南

发布形态统一为 Docker 镜像：`.github/workflows/docker-release.yml` 在 tag 推送时构建并推送 `ghcr.io/sakuralovesmile/sakura-blog`，服务器用 Docker Compose 拉取固定版本镜像运行。仓库不再构建或分发单二进制，宿主二进制的 `update.sh` / `update-docker.sh` 与 systemd 更新单元已移除。

从旧 1.x 升级到 Stellar 2.0 时，先按 [Docker / Compose 迁移与 Markdown 导入](upgrade-v1-to-v2.md) 完成旧库预览、隔离演练和停服迁移，再启动新版本；普通镜像更新和 `-seed` 不替代数据迁移。

## 目录约定（服务器）

| 路径 | 内容 |
|------|------|
| `/opt/1panel/docker/compose/blog/docker-compose.yml` | Compose 文件（1Panel 示例路径，可按实际面板调整） |
| `<compose 目录>/config/` | `app.yml` / `site.yml`（含密钥，chmod 600；挂载到容器 `/app/config`） |
| `<compose 目录>/data/` | `blog.db`、`uploads/`、`backups/`、`ogcache/`、`ghcache/`（挂载到 `/app/data`） |
| `<compose 目录>/content/` | Markdown 内容（挂载到 `/app/content`） |

只挂载运行态子目录；不要把整个安装目录挂到 `/app`，否则会遮住镜像内二进制。

## Compose 示例

```yaml
services:
  blog:
    image: ghcr.io/sakuralovesmile/sakura-blog:2.0.4  # 固定具体版本，不以 latest 作为版本证据
    container_name: blog
    restart: unless-stopped
    ports:
      - "8081:8080"
    volumes:
      - ./config:/app/config
      - ./data:/app/data
      - ./content:/app/content
    environment:
      GIN_MODE: release
```

`deploy/docker-compose.image.yml` 是同一形态的示例（其中的 `:latest` 仅作演示）。生产环境必须固定具体 `X.Y.Z` 标签，升级与回退都只改这一行。

镜像标签：`X.Y.Z`、`X.Y`、`X`、`latest` 与 `sha-<短哈希>`；`latest` 只由正式版 tag 推送更新。

预发布版本（tag 含 `-`，如 `v2.0.7-beta01`）只发布自身完整版本标签与 `sha-` 标签，不生成 `X.Y` / `X` 别名、不移动 `latest`，对应 GitHub Release 标记为 prerelease。试用 beta 时 compose 仍显式固定该完整标签（如 `:2.0.7-beta01`），升级与回退方式与正式版相同。

## 首次部署步骤

1. **配置文件**：从仓库 `config/` 复制 example 模板并填值

   ```bash
   cp app.example.yml  <compose 目录>/config/app.yml   # chmod 600
   cp site.example.yml <compose 目录>/config/site.yml  # chmod 600
   ```

   必填项：

   | 文件 | 字段 | 说明 |
   |------|------|------|
   | `site.yml` | `url` | 正式 https 域名；canonical/OG/sitemap/cookie Secure 全靠它 |
   | `site.yml` | `title`/`description`/头像/友链/社交 | 替换全部 Demo 文案 |
   | `site.yml` | `admin.password` | `/console/` 登录密码，生产必填 |
   | `site.yml` | `footer.beian` | ICP 备案号（大陆服务器必填） |
   | `app.yml` | `trusted_proxy` | nginx 同机填 `127.0.0.1/32`；不填则 IP 限流失效 |
   | `app.yml` | `github_token` | 可选；仅用于 GitHub 小组件与镜像标签检查的速率提升，公开仓库可留空 |

2. **内容导入（仅一次）**：只把正式文章放到 `content/posts/`。仓库中的测试/验收文章位于 `docs/fixtures/release-posts/`，不得复制到生产内容目录。首启用 seed 导入后关闭：

   ```bash
   docker compose exec blog /usr/local/bin/blog -root /app -seed
   # 或 app.yml 设 seed_on_start: true 启动一次后改回 false
   ```

3. **启动**：

   ```bash
   cd <compose 目录>
   docker compose pull
   docker compose up -d
   curl -fsS http://127.0.0.1:8081/readyz
   ```

4. **反代与证书**：按 `nginx.conf.example` 配置站点（改 `server_name` 与证书路径），certbot 签发证书后启用。

5. **备份**：应用每 24h 自动全量备份到 `data/backups/`（DB + 附件 + site.yml），留存份数由 `app.yml` 的 `backup_keep` 控制；建议另行异地同步。

## Console 检查更新

Console「系统更新」页面（`/console/system-update`）读取 GHCR 公开镜像标签，与镜像内 ldflags 注入的当前版本做 semver 比较，只做检测、不触发升级。匿名访问公开镜像包即可，无需 GitHub Token；`refresh=1` 绕过 6h 缓存。

更新日志、发布时间与「发布说明」链接来自该 tag 对应的 GitHub Release。发布工作流会自动创建一个不带资产的 Release 并生成 commit/PR 说明；Release 不存在或版本不一致时只显示版本信息，不影响升级判断。

服务器无法访问 `ghcr.io`、或镜像包不是公开的时，页面会显示具体失败原因。

## 手动升级与回滚

```bash
cd <compose 目录>
# 把 image 改为 ghcr.io/sakuralovesmile/sakura-blog:X.Y.Z
docker compose pull
docker compose up -d
curl -fsS http://127.0.0.1:8081/readyz   # 宿主映射端口
```

- **回滚**：把 `image` 改回上一个已验证的版本标签并 `docker compose up -d`。若该版本含数据迁移，回退前必须先恢复停服前的完整数据备份，只切回镜像不足以回退。
- 升级前后核对 `docker inspect blog --format '{{.Config.Image}} {{.Image}} {{.RestartCount}}'`，确认运行的是预期镜像且没有反复重启。

## 上线验证清单

- [ ] `curl -I https://域名/` 返回 200，响应头含 HSTS；http 访问 301 跳 https
- [ ] 首页 canonical/OG 为 https 绝对 URL（`site.url` 生效）
- [ ] 容器 `image` 为预期 `X.Y.Z`，`/readyz` 200，`RestartCount` 为 0
- [ ] `/console/` 登录后 cookie 带 `Secure`；登录失败 5 次触发限流
- [ ] 同 IP 60 秒内第二条评论被拒（`trusted_proxy` 生效）
- [ ] `/robots.txt` 含 `Disallow: /console`；`/sitemap.xml` 正常
- [ ] `content/posts/` 只包含正式文章，测试/验收 fixtures 未复制到生产目录
- [ ] `scripts/check-release-secrets.sh` 扫描通过，未在 Git 跟踪文件发现凭据
- [ ] `config/app.yml`、`config/site.yml` 权限均为 `0600`
- [ ] `data/backups/` 24h 内生成首份自动备份
- [ ] Console「系统更新」页能读到当前版本与 GHCR 最新标签

## 日常运维

- 日志：`docker logs -f blog`
- 升级：改 compose `image` → `docker compose pull && docker compose up -d` → 探测 `/readyz`
- 回滚：见上
- 备份：每日自动全量备份 `data/backups/`；恢复通过 Console 的整站恢复入口执行

## 整站恢复与维护窗口

恢复任务在应用内执行，容器化部署只需要保证挂载目录可写：

- `data/` 必须可写（恢复前像、备份与数据库都在这里）；`config/site.yml` 必须是单个可写文件，不以只读目录或只读绑定的方式挂载。
- 恢复任务记录位于 `<DB 所在目录>/backups/.restore-restore-<ID>.json`，版本字段为 `1`。完整数据库、附件和配置前像位于 `backups/restore-tasks/<任务 ID>/preimage/`。只自动清理最近 3 份成功任务之前的前像；未完成和补偿失败任务不会自动清理。旧 `.pre-restore-*.db` 仍可作为本地快照恢复，不会在升级时自动删除。
- 启动首先检查并补偿非终态任务，之后才读取站点 YAML、执行迁移并开放业务接口。补偿失败时 `/livez` 可用，`/readyz` 与业务接口不可用；`/api/maintenance` 只公开维护状态和重试提示。修复磁盘空间、权限或完整前像可读性后重启容器继续补偿；不要删除未完成日志、消费唯一前像，或替换仍在运行的 SQLite 文件。
- 恢复与升级共用 `<DB 所在目录>/maintenance.lock` 的文件锁。自定义数据库路径时让容器与备份任务保持同一 `BLOG_DB`；也可同时设置 `BLOG_MAINTENANCE_LOCK`。存在未完成恢复日志时不可人工降级到不识别该日志的旧程序；只有任务已完成或完整补偿后，才可按正常镜像回滚流程回退。
