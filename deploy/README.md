# 部署指南

单二进制部署：systemd 服务 + nginx 反代 + 定时自动更新（`update.sh` 从 GitHub Release 拉取，并校验 SHA256）。

`update.sh` 依赖 `curl`、`sha256sum`、`systemctl` 和 `python3`。它适用于公开或私有 GitHub 仓库；私有仓库通过 `/opt/blog/.token` 使用仅 `contents:read` 的 PAT 读取 Release 资产。

每个新 Release 必须包含 `blog-linux-amd64`、`blog-linux-amd64.sha256` 和 `blog-linux-amd64.spdx.json`。更新脚本下载二进制和 SHA256 文件，校验通过后才执行替换、健康检查与失败回滚；SBOM 供发布审查和依赖分析使用。

## 目录约定（服务器）

| 路径 | 内容 |
|------|------|
| `/opt/blog/blog` | 二进制（update.sh 维护，勿手工替换） |
| `/opt/blog/blog.bak` | 上一版本备份（健康检查失败自动回滚用） |
| `/opt/blog/data/` | 运行时数据：`blog.db`、`uploads/`、`backups/`、`ogcache/`、`ghcache/` |
| `/opt/blog/config/` | `app.yml` / `site.yml`（含密钥，chmod 600） |
| `/opt/blog/.token` | Release 下载 PAT（chmod 600，仅 repo 读权限） |
| `/opt/blog/version` | 当前版本号（update.sh 维护） |

## 首次部署步骤

1. **系统准备**：创建用户与目录

   ```bash
   useradd -r -s /usr/sbin/nologin blog
   mkdir -p /opt/blog/{config,data,content/posts}
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
   cp blog.service blog-update.service blog-update.timer /etc/systemd/system/
   install -d -m 755 /etc/blog
   install -d -m 755 /etc/systemd/system/blog.service.d
   install -m 600 -o root -g root /dev/null /etc/blog/blog.env
   cp blog.service.d/10-secrets.conf.example /etc/systemd/system/blog.service.d/10-secrets.conf
   systemctl daemon-reload
   systemctl enable --now blog
   systemctl enable --now blog-update.timer
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

## Console 一键更新

Console 后台「系统更新」页面（/console/system-update）可检查 GitHub 最新版本并一键触发更新。一键更新通过 `sudo systemctl start blog-update.service` 触发 oneshot unit，需配置 sudoers 免密规则。

### sudoers 配置

blog 运行用户需免密执行 systemctl start/is-active：

```bash
# /etc/sudoers.d/blog-update（权限 0440，owner root:root）
blog ALL=(root) NOPASSWD: /usr/bin/systemctl start blog-update.service
blog ALL=(root) NOPASSWD: /usr/bin/systemctl is-active blog-update.service
```

> 仅允许 `start` 和 `is-active`，不开放 `stop`/`restart`/`edit` 等危险操作。

### GitHub PAT

检查更新需要读取仓库 release 信息。私有仓库需配置 PAT（`github_token`），公开仓库可匿名访问（60 req/h 限流）。PAT 权限只需该仓库的 `contents:read`，复用部署时已有的 `.token` 文件即可。

### app.yml 配置

```yaml
# 系统更新配置（缺省值见下方，多数部署无需修改）
update_repo: "SakuraLoveSmile/Sakura-Blog"  # GitHub 仓库（owner/repo）
update_install_dir: "/opt/blog"               # 安装目录（update.sh 所在）
update_service: "blog-update"                 # systemd oneshot unit 名
```

### 工作流程

1. **检查更新**：Console 调用 GitHub API 获取最新 release，与当前版本（ldflags 注入的 tag 名）做 semver 比较
2. **一键更新**：点击后 `sudo systemctl start blog-update.service` 触发 update.sh，后者拉取新版本二进制、健康检查、自动回滚
3. **状态轮询**：前端每 3 秒轮询 `/api/v1/system/update/status`，对比当前版本与目标版本判断 succeeded/failed
4. **强制刷新**：`refresh=1` 参数绕过 6h 缓存直连 GitHub（用于刚发布新版本后立即检查）

### 版本内嵌

release 构建通过 ldflags 注入 tag 名（`-X blog/internal/app.Version=${GITHUB_REF_NAME}`），当前版本可直接从二进制读取。本地开发版本固定为 `dev`，Console 页面会回退读取 `<Root>/version` 文件或显示 `unknown`。

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
- [ ] `systemctl status blog blog-update.timer` 均 active
- [ ] Console「系统更新」页能检查到版本信息；sudoers 配置后一键更新可触发 blog-update.service

## 日常运维

- 日志：`journalctl -u blog -f`；更新日志 `/opt/blog/data/update.log`
- 手动更新：`sudo /opt/blog/update.sh`（仅升级已有安装；timer 每 6 小时自动跑）
- 回滚：update.sh 健康检查失败自动回滚 `blog.bak`；手工回滚即 `cp blog.bak blog && systemctl restart blog`
- 备份：每日自动全量备份 `data/backups/`（DB + 附件 + site.yml），留存 7 份；建议另行异地同步
