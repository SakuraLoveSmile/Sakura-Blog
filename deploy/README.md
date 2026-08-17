# 部署指南

单二进制部署：systemd 服务 + nginx 反代 + 定时自动更新（`update.sh` 从 GitHub Release 拉取）。

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
   | `app.yml` | `github_token` | 建议改用 `BLOG_GITHUB_TOKEN` 环境变量注入 |

3. **内容导入（仅一次）**：把 `content/posts/*.md`（已清理，只含真实文章）放到 `/opt/blog/content/posts/`，首启用 seed 导入后关闭：

   ```bash
   sudo -u blog /opt/blog/blog -root /opt/blog -seed   # 手动跑一次
   # 或 app.yml 设 seed_on_start: true 启动一次后改回 false
   ```

4. **systemd**：

   ```bash
   cp blog.service blog-update.service blog-update.timer /etc/systemd/system/
   systemctl daemon-reload
   systemctl enable --now blog
   systemctl enable --now blog-update.timer
   ```

   注意 `blog.service` 的 `BLOG_ROOT=/opt/blog`，`app.yml`/`site.yml` 由 root 解析定位。

5. **反代与证书**：按 `nginx.conf.example` 配置站点（改 `server_name` 与证书路径），certbot 签发证书后启用。

6. **自动更新 Token**：

   ```bash
   install -m 600 -o root -g root /dev/null /opt/blog/.token
   echo "<fine-grained PAT, 仅本仓库 contents 读权限>" > /opt/blog/.token
   ```

## 上线验证清单

- [ ] `curl -I https://域名/` 返回 200，响应头含 HSTS
- [ ] http 访问 301 跳 https
- [ ] 首页 canonical/OG 为 https 绝对 URL（`site.url` 生效）
- [ ] `/console/` 登录后 cookie 带 `Secure`；登录失败 5 次触发限流
- [ ] 同 IP 60 秒内第二条评论被拒（`trusted_proxy` 生效）
- [ ] `/robots.txt` 含 `Disallow: /console`；`/sitemap.xml` 正常
- [ ] 前台无测试文章（hello/mock-*/katex-test 等）
- [ ] 页脚备案号渲染（若配置）
- [ ] `data/backups/` 24h 内生成首份自动备份
- [ ] `systemctl status blog blog-update.timer` 均 active

## 日常运维

- 日志：`journalctl -u blog -f`；更新日志 `/opt/blog/data/update.log`
- 手动更新：`sudo /opt/blog/update.sh`（timer 每 6 小时自动跑）
- 回滚：update.sh 健康检查失败自动回滚 `blog.bak`；手工回滚即 `cp blog.bak blog && systemctl restart blog`
- 备份：每日自动全量备份 `data/backups/`（DB + 附件 + site.yml），留存 7 份；建议另行异地同步
