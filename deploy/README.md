# 部署指南

从旧 1.x 升级到 Stellar 2.0 时，先按 [Docker / Compose 迁移与 Markdown 导入](upgrade-v1-to-v2.md) 完成旧库预览、隔离演练和停服迁移，再启动新版本；普通镜像更新和 `-seed` 不替代数据迁移。

单二进制部署：systemd 服务 + nginx 反代 + 手动更新（`sudo update.sh` 从 GitHub Release 拉取，并校验 SHA256）。容器化部署见文末「Docker 容器部署」。v1.7.3 起 Console 只提供「检查更新」，不再从后台一键触发更新。

`update.sh` 依赖 `curl`、`sha256sum`、`systemctl` 和 `python3`。它适用于公开或私有 GitHub 仓库；私有仓库通过 `/opt/blog/.token` 使用仅 `contents:read` 的 PAT 读取 Release 资产。

每个新 Release 必须包含 `blog-linux-amd64`、`blog-linux-amd64.sha256` 和 `blog-linux-amd64.spdx.json`。更新脚本下载二进制和 SHA256 文件，校验通过后才执行替换、健康检查与失败回滚；SBOM 供发布审查和依赖分析使用。

## 目录约定（服务器）

| 路径 | 内容 |
|------|------|
| `/opt/blog/blog` | 二进制（update.sh 维护，勿手工替换） |
| `/opt/blog/blog.bak` | 上一版本备份（健康检查失败自动回滚用） |
| `/opt/blog/data/` | 运行时数据：`blog.db`、`uploads/`、`backups/`、`ogcache/`、`ghcache/` |
| `/opt/blog/config/` | `app.yml` / `site.yml`（含密钥，chmod 600） |
| `/opt/blog/update/` | 更新控制目录（锁文件、status.json 状态记录；手动执行 update.sh 时读写） |
| `/opt/blog/.token` | Release 下载 PAT（chmod 600，仅 repo 读权限） |
| `/opt/blog/version` | 当前版本号（update.sh 维护） |

## 首次部署步骤

1. **系统准备**：创建用户与目录

   ```bash
   useradd -r -s /usr/sbin/nologin blog
   mkdir -p /opt/blog/{config,data,content/posts,update}
   chown -R blog:blog /opt/blog
   ```

2. **配置文件**：从仓库 `config/` 复制 example 模板并填值

   ```bash
   cp app.example.yml /opt/blog/config/app.yml     # chmod 600
   cp site.example.yml /opt/blog/config/site.yml   # chmod 600
   ```

   必填项（详见 Spec 55 Part A）：

   | 文件 | 字段 | 说明 |
   |------|------|------|
   | `site.yml` | `url` | 正式 https 域名；canonical/OG/sitemap/cookie Secure 全靠它 |
   | `site.yml` | `title`/`description`/头像/友链/社交 | 替换全部 Demo 文案 |
   | `site.yml` | `admin.password` | `/console/` 登录密码，生产必填 |
   | `site.yml` | `footer.beian` | ICP 备案号（大陆服务器必填） |
   | `app.yml` | `trusted_proxy` | nginx 同机填 `127.0.0.1/32`；不填则 IP 限流失效 |
   | `app.yml` | `github_token` | 保持为空；通过 `/etc/blog/blog.env` 的 `BLOG_GITHUB_TOKEN` 注入 |

3. **安装首个二进制**：首次安装不要执行 `update.sh`；该脚本只接受已有 `/opt/blog/blog` 的升级。将 Release 中已校验的 `blog-linux-amd64` 安装为 `/opt/blog/blog` 并设为可执行。

4. **内容导入（仅一次）**：只把正式文章放到 `/opt/blog/content/posts/`。仓库中的测试/验收文章位于 `docs/fixtures/release-posts/`，不得复制到生产内容目录。首启用 seed 导入后关闭：

   ```bash
   sudo -u blog /opt/blog/blog -root /opt/blog -seed   # 手动跑一次
   # 或 app.yml 设 seed_on_start: true 启动一次后改回 false
   ```

5. **systemd**：

   ```bash
   cp blog.service blog-update.service /etc/systemd/system/
   install -d -m 755 /etc/blog
   install -d -m 755 /etc/systemd/system/blog.service.d
   install -m 600 -o root -g root /dev/null /etc/blog/blog.env
   cp blog.service.d/10-secrets.conf.example /etc/systemd/system/blog.service.d/10-secrets.conf
   systemctl daemon-reload
   systemctl enable --now blog
   # blog-update.service 按需手动触发：sudo systemctl start blog-update（等价 sudo update.sh）
   ```

   `blog.service` 已固定使用 `GIN_MODE=release`。如需 GitHub API 访问，使用 root-only 编辑器填写
   `/etc/blog/blog.env` 中的 `BLOG_GITHUB_TOKEN`，不要把 Token 写进 `app.yml`、tracked unit 或命令行：

   ```bash
   sudoedit /etc/blog/blog.env
   # BLOG_GITHUB_TOKEN=<fine-grained PAT，仅本仓库 contents:read>
   chmod 600 /etc/blog/blog.env
   ```

   注意 `blog.service` 的 `BLOG_ROOT=/opt/blog`，`app.yml`/`site.yml` 由 root 解析定位。

6. **反代与证书**：按 `nginx.conf.example` 配置站点（改 `server_name` 与证书路径），certbot 签发证书后启用。

7. **自动更新 Token**：

   ```bash
   install -m 600 -o root -g root /dev/null /opt/blog/.token
   echo "<fine-grained PAT, 仅本仓库 contents 读权限>" > /opt/blog/.token
   ```

   `app.yml`、`site.yml` 和 `.token` 均必须为 `0600`；不要把 Token 写入 Git 跟踪文件、备份日志或命令输出。

## Console 检查更新与手动升级

Console 后台「系统更新」页面（/console/system-update）检查 GitHub 最新版本并展示更新日志。v1.7.3 起 Console 不再触发更新执行——升级一律在宿主上手动进行：

```bash
sudo /opt/blog/update.sh                 # 二进制部署
# 或等价：sudo systemctl start blog-update
```

`update.sh` 会下载资产 → SHA256 校验 → 备份旧二进制 → 原子替换 → `systemctl restart blog` → 本地 HTTP 探测 `/readyz` → 失败自动回滚 `blog.bak`，并把阶段写入 `/opt/blog/update/status.json`、日志写入 `/opt/blog/data/update.log`。

### 权限与安全机制

- **无 sudo 依赖**：服务进程 `blog` 无任何提权需求，更新只能由管理员在宿主上发起。
- **全局排他锁**：依次取得 `<DB 所在目录>/maintenance.lock` 和 `/opt/blog/update/update.lock`，避免与备份恢复任务并发冲突。
- **失败自动回滚**：健康检查失败即还原 `blog.bak` 并重启旧版本。

### GitHub PAT

检查更新需要读取仓库 release 信息。默认仓库 `Sakura-Blog-Release` 为公开仓库，可匿名访问（60 req/h 限流），无需配置 Token；改用私有仓库时需配置 PAT（`github_token`），权限只需 `contents:read`，复用部署时已有的 `.token` 文件即可。

### app.yml 配置

```yaml
# 系统更新配置（缺省值，多数部署无需修改）
update_repo: "SakuraLoveSmile/Sakura-Blog-Release"  # 公开二进制发布仓库（owner/repo）
```

### 工作流程

1. **检查更新**：Console 调用 GitHub API 获取最新 release，与当前版本（ldflags 注入的 tag 名）做 semver 比较
2. **手动升级**：管理员在宿主执行 `sudo update.sh`（或 `systemctl start blog-update`），脚本完成下载、校验、替换、健康检查与失败回滚
3. **强制刷新**：`refresh=1` 参数绕过 6h 缓存直连 GitHub（用于刚发布新版本后立即检查）

### 版本内嵌

release 构建通过 ldflags 注入完整构建元数据：
- tag 名（`-X blog/internal/app.Version=${GITHUB_REF_NAME}`）
- Git commit sha（`-X blog/internal/app.GitCommit=${GITHUB_SHA}`）
- 构建时间戳（`-X blog/internal/app.BuildTime=...`）
- Stellar 来源 commit（`-X blog/internal/app.StellarCommit=...`）
- 前端构建指纹（`-X blog/internal/app.FrontendFingerprint=...`）

管理员可通过 `GET /api/v1/system/version` 或「系统更新」页面查阅。本地开发版本固定为 `dev`，Console 页面会回退读取 `<Root>/version` 文件或显示 `unknown`。

## 上线验证清单

- [ ] `curl -I https://域名/` 返回 200，响应头含 HSTS
- [ ] http 访问 301 跳 https
- [ ] 首页 canonical/OG 为 https 绝对 URL（`site.url` 生效）
- [ ] `/console/` 登录后 cookie 带 `Secure`；登录失败 5 次触发限流
- [ ] 同 IP 60 秒内第二条评论被拒（`trusted_proxy` 生效）
- [ ] `/robots.txt` 含 `Disallow: /console`；`/sitemap.xml` 正常
- [ ] `content/posts/` 只包含正式文章，测试/验收 fixtures 未复制到生产目录
- [ ] `scripts/check-release-secrets.sh` 扫描通过，未在 Git 跟踪文件发现凭据
- [ ] `config/app.yml`、`config/site.yml`、`.token` 权限均为 `0600`
- [ ] 生产 `site.yml` 存在且包含有效 admin 密码；缺失配置时服务应拒绝启动
- [ ] 页脚备案号渲染（若配置）
- [ ] `data/backups/` 24h 内生成首份自动备份
- [ ] `systemctl status blog` active
- [ ] Console「系统更新」页能检查到版本信息（只读检测，不再提供一键升级）

## 日常运维

- 日志：`journalctl -u blog -f`；更新日志 `/opt/blog/data/update.log`
- 手动更新：`sudo /opt/blog/update.sh`（仅升级已有安装；等价 `sudo systemctl start blog-update`）
- 回滚：update.sh 健康检查失败自动回滚 `blog.bak`；手工回滚即 `cp blog.bak blog && systemctl restart blog`
- 备份：每日自动全量备份 `data/backups/`（DB + 附件 + site.yml），留存 7 份；建议另行异地同步

## Docker 容器部署（update-docker.sh）

适用于「容器跑二进制、宿主无应用级 systemd 服务」的部署（如 1Panel / compose 管理的 `golang` 镜像容器）。升级由管理员在宿主上手动执行 `update-docker.sh`：下载资产 → SHA256 校验 → 备份 → 替换二进制 → `docker restart` → 宿主映射端口探测 `/readyz` → 失败自动回滚，全程写 `status.json`/`update.log`。stage 词表与 status.json 字段和 `update.sh` 完全一致。

**前提**：安装目录以目录方式 bind mount 进容器（如宿主 `/opt/blog` → 容器 `/app`），容器启动命令执行 `<root>/blog` 二进制。不要以单文件方式挂载二进制（`mv` 替换对单文件 mount 不可见）。

**宿主安装**（需要 docker CLI + curl + sha256sum + python3；systemd 可选，用于 `systemctl start blog-update-docker` 等价手动触发）：

```bash
install -m 755 update-docker.sh /opt/blog/update-docker.sh
install -m 644 blog-update-docker.service /etc/systemd/system/   # 可选：手动触发入口
# 按需写 /opt/blog/update-docker.env：
#   BLOG_CONTAINER=blog        容器名（默认 blog）
#   BLOG_HEALTH_PORT=8081      宿主映射端口（如 8081->8080 则填 8081）
#   BLOG_PROXY=http://127.0.0.1:7890   GitHub 直连不可达时的代理
#   TOKEN_FILE=/opt/blog/.token        私有仓库 PAT（默认即此路径）
```

手动执行：

```bash
sudo /opt/blog/update-docker.sh
# 或：sudo systemctl start blog-update-docker
```

### 镜像部署（GHCR，可选形态）

`.github/workflows/docker-release.yml` 在 tag 推送时构建并推送 `ghcr.io/sakuralovesmile/sakura-blog:<version>`（含 `latest`）。镜像为自包含多阶段构建（node → go → debian-slim），二进制位于镜像内 `/usr/local/bin/blog`，入口 `blog -root /app`。

```bash
docker compose -f deploy/docker-compose.image.yml up -d
```

与本节挂载部署的关键区别：

- **升级走镜像替换**：`docker compose pull && docker compose up -d`（可配 Watchtower 自动升级）。「检查更新」仍可用。
- **只挂运行态子目录**：`config/`、`data/`、`content/`；不要把整个安装目录挂到 `/app`（会遮住镜像内二进制）。

### 整站恢复与维护窗口

阶段 B 的恢复需要保留 `NoNewPrivileges=true` 和 `ProtectSystem=strict`，并采用仓库 unit 中的精确授权：

```ini
ReadWritePaths=/opt/blog/data /opt/blog/config/site.yml
```

安装时必须创建既有 `site.yml` 文件并设置权限；不要把整个配置目录加到 `ReadWritePaths`：

```bash
install -o blog -g blog -m 0600 site.example.yml /opt/blog/config/site.yml
```

恢复通过受日志保护的原地写入与同步修改该文件，不依赖在只读配置目录中创建临时文件或 rename 覆盖 bind mount。已安装服务器的 unit 更新和重启仍需独立部署授权。

恢复任务记录位于 `<DB 所在目录>/backups/.restore-restore-<ID>.json`，版本字段为 `1`。完整数据库、附件和配置前像位于 `backups/restore-tasks/<任务 ID>/preimage/`。只自动清理最近 3 份成功任务之前的前像；未完成和补偿失败任务不会自动清理。旧 `.pre-restore-*.db` 仍可作为本地快照恢复，不会在升级时自动删除。

启动首先检查并补偿非终态任务，之后才读取站点 YAML、执行迁移并开放业务接口。补偿失败时 `/livez` 可用，`/readyz` 与业务接口不可用；`/api/maintenance` 只公开维护状态和重试提示。修复磁盘空间、权限或完整前像可读性后重启应用继续补偿；不要删除未完成日志、消费唯一前像，或替换仍在运行的 SQLite 文件。

更新脚本与恢复共用 `<DB 所在目录>/maintenance.lock` 的文件锁。自定义数据库路径时，更新 unit 应设置相同的 `BLOG_DB`；也可在博客和更新器中同时设置 `BLOG_MAINTENANCE_LOCK`。恢复未结束或日志版本不受支持时，更新脚本拒绝切换程序版本。因此，存在未完成恢复日志时不可人工降级到不识别该日志的旧程序。只有任务已完成或完整补偿后，才可按正常部署流程回退程序。

本地隔离验证命令：`bash deploy/tests/restore-systemd.sh`。脚本只在新建的 Docker 容器内启动 systemd，验证加固权限、Linux 恢复中断和跨文件系统恢复；结束时清理它自己创建的容器和临时文件。

锁忙时 `blog-update.service` 保持等待（`TimeoutStartSec=infinity`），可由管理员停止服务取消等待。无效锁或未完成/不支持的恢复日志使执行器以 75 退出，服务仅对此退出码按 `RestartSec=30s` 重试（`RestartForceExitStatus=75`）；普通失败或回滚不自动重试。更新部署时须同步安装新的 update unit。

更新服务使用 `Type=exec` 跟踪脚本进程（进程退出即结束），以支持仅对 75 重试；systemd 不允许 oneshot 使用 `RestartForceExitStatus`。
