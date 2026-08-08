# Spec 55 · 正式上线准备（脱敏 / 配置转正 / 内容清理 / 部署收尾）

- **分支**：`feature/launch-prep` · **Wave 13**
- **主模块**：`backend/config/`（app.yml/site.yml 生产值，不入 git）+ `content/posts/`（内容清理）+ `deploy/`（+反代配置模板）+ `footer.html` + `config/site.go`（可选 beian 字段）
- **前置**：Spec 28（上线前加固）已合入 dev；dev 工作区现有未提交改动（tagcloud/编辑器/瞬间表情等）先提交收尾
- **性质**：运维配置 + 内容整理为主，代码改动极少；不引入新功能

## Why

上线前体检（2026-08）结论：代码面 ready（gzip / 安全头 / robots+sitemap / 登录限流+2FA / 评论限流 / 自动备份 / 带健康检查与回滚的自动更新 / CI Release 流水线均已就位），挡路的集中在**配置与内容层**：

1. `site.url` 为空 → canonical/OG/sitemap/feed 回退 `http://<request-host>`，可能被按 http 收录；cookie `Secure` 推导也依赖它。
2. `site.yml` 全套 Demo 文案（title "Stellar Demo"、占位头像、演示友链/wiki、`#` 社交链接）。
3. `content/posts/` 全是测试文章（hello/mock-*/katex-test/tag-plugins-test/gh-widgets-test），生产 `-seed` 会公开发布。
4. `trusted_proxy` 为空 → 反代后 `ClientIP()` 全为 127.0.0.1，评论限流（1 条/60s）与登录限流变成全站共享一个桶，等于限流失效；登录审计 IP 失真。
5. `app.yml` 明文 GitHub PAT 需轮换；仓库即将推 GitHub，须确认历史无密钥泄漏（已扫描确认干净）。

## What Changes

### Part A · 密钥轮换与配置转正（生产环境操作，不改仓库代码）

- **轮换 GitHub PAT**：GitHub Settings → Tokens 吊销现 token；新 token 用 fine-grained、只读、最小仓库范围。注入方式二选一：写回 `app.yml`（不受 git 跟踪，可接受）或 `BLOG_GITHUB_TOKEN` 环境变量（生产优先）。
- **`app.yml`**：`trusted_proxy` 填反代来源 IP（nginx 同机填 `127.0.0.1/32`）；生产首启临时 `seed_on_start: true`（或手动 `-seed` 一次），seed 完成后改回 `false`。
- **`site.yml`**：
  - `url` 填正式 https 域名（**必填，P0**）。
  - 替换 Demo 文案：`title` / `subtitle` / `description` / `logo_title` / `logo_sub` / `avatar` / `avatar_bg`。
  - 清理演示数据：`wiki` 三条示例项目、`links` 演示友链（保留真实友链或留空待 Spec 54 导入）、`footer.social` 的 `#` 占位链接。
  - `comments.admin_email` 填博主邮箱（通知收件 + 博主徽章匹配）；`notify.smtp_*` 按需配置（Spec 26 已有配置化与测试发信）。
  - `seo`：填 `default_description`、各搜索引擎验证码；确认 `no_index: false`。

### Part B · 内容清理（仓库内，git 跟踪）

- `content/posts/` 删除或置 `draft: true`：`hello.md`、`katex-test.md`、`tag-plugins-test.md`、`gh-widgets-test.md`、`mock-*.md`（5 篇）。
- 测试文章的验证价值保留方式：标签插件/KaTeX 用例改写为真实文章或移入 `resource/`（gitignore），不再随 seed 发布。
- 生产 DB 为全新库时，seed 源即清理后的 `content/posts/`，无历史测试数据残留。

### Part C · 部署收尾（deploy/）

- 新增 `deploy/nginx.conf.example`（或 `caddy` 等价物）：TLS 终结、http→https 301、HSTS、`proxy_pass :8080`、`X-Forwarded-For/Proto` 透传、`/uploads/` 与静态资源长缓存。与 `app.yml.trusted_proxy` 取值呼应。
- `deploy/README.md`（新）：首部署步骤清单——复制 example 配置、填密钥、`-seed` 一次、systemd enable、timer 启用、反代证书。update.sh 已有注释不重复。
- 服务器侧 `/opt/blog/.token`（release 下载 PAT）确认 `chmod 600`。

### Part D · 合规备案号（大陆服务器时启用，条件性子任务）

- `site.yml` `footer` 段新增 `beian: ''`（ICP 备案号，空则不渲染）。
- `config/site.go` Footer 结构 +1 字段；`partial/footer.html` 署名行尾非空时追加渲染，链接 `https://beian.miit.gov.cn/`（对齐 Stellar 原版 footer 的 beian 处理）。
- 非大陆部署则整段跳过，零改动。

## 技术决策（无需再问）

- **不改代码语义**：除 Part D 一个可选字段外不新增任何功能逻辑；全部为配置值与内容整理。
- **密钥处置**：PAT 轮换为硬要求（token 已经过终端输出）；`app.yml`/`site.yml` 继续不入 git（`.gitignore` 已覆盖，全历史扫描无 `ghp_`/`gho_` 痕迹与配置文件提交记录，已验证）。
- **trusted_proxy 必填**：生产反代场景不信任 XFF 时 IP 级限流全部失效，列为 P0 而非建议项。
- **seed 策略**：生产只 seed 清理后的真实内容；不在生产库内手工删测试文（源头治理）。
- **测试文章去留**：物理删除优先（git 历史仍可找回）；如需保留用例则移 `resource/`，禁止以 `draft` 状态长期滞留 seed 源（防误发布）。
- **release 流程**：沿用现状——dev 合入 release 分支 → 打 `v*` tag → CI 构建单二进制 → 服务器 `update.sh` 定时拉取；不新增流水线。

## Impact

- **新增**：`deploy/nginx.conf.example`、`deploy/README.md`；（Part D 时）`footer.html` beian 渲染
- **修改**：`content/posts/`（删测试文）；生产 `app.yml`/`site.yml`（不入 git）；（Part D 时）`config/site.go` +1 字段
- **不影响**：任何 API / 前台渲染逻辑（除可选 beian 行）/ Console / CI

## ADDED Requirements

### Requirement: 生产配置转正清单

The system SHALL launch with site.url set to the canonical https domain, trusted proxy configured for the reverse proxy, and all demo content replaced, such that canonical/OG/sitemap URLs, cookie Secure flag, and IP-based rate limiting all behave correctly in production.

#### Scenario: site.url 生效
- **WHEN** release 模式启动且 `site.url` 已填 https 域名
- **THEN** 无 "site.url is empty" 告警；canonical/OG/sitemap/feed 输出该域名的绝对 URL；HTTPS 请求下 auth cookie 带 `Secure`

#### Scenario: 反代 IP 还原
- **WHEN** `trusted_proxy: 127.0.0.1/32` 且请求经 nginx 携带 XFF 到达
- **THEN** 评论限流与登录限流按真实客户端 IP 计桶；登录审计日志记录真实 IP

#### Scenario: 测试内容不发布
- **WHEN** 生产首启执行 seed
- **THEN** 前台无 hello/mock-*/katex-test/tag-plugins-test/gh-widgets-test 任一文章可见

### Requirement: 密钥轮换与仓库脱敏（Part A）

The system SHALL run with a rotated least-privilege GitHub token, and the repository pushed to GitHub SHALL contain no secrets in tracked files or history.

#### Scenario: 推库前脱敏校验
- **WHEN** 推送前对目标分支全历史执行密钥扫描（`ghp_`/`gho_` 模式 + 配置文件路径 + `*.db`）
- **THEN** 零命中方可推送；本地 `app.yml` 中旧 PAT 已吊销并替换

### Requirement: 备案号展示（Part D，条件性）

The system SHALL render an ICP filing number in the page footer when `footer.beian` is configured, linking to the MIIT filing site.

#### Scenario: 配置即渲染
- **WHEN** `footer.beian: 某ICP备00000000号`
- **THEN** 全站页脚署名行展示该号并链接 `https://beian.miit.gov.cn/`；字段为空时页脚与现状逐像素一致

## 验证清单（完成标准）

- [ ] PAT 已轮换，旧 token 吊销；gh widgets（ghrepo/ghcard）在 60 req/h 匿名额度之外正常返回
- [ ] `make test` 全绿（Part D 若启用需补 footer 渲染单测）
- [ ] release 模式启动日志无 site.url 告警；`curl` 首页 canonical/OG 为 https 绝对 URL
- [ ] 反代链路：`/console/` 登录 cookie 带 Secure；同 IP 连续 2 条评论第二条被限流
- [ ] 前台首页/归档无测试文章；robots.txt `/console` Disallow 与 sitemap 正常
- [ ] 推 GitHub 前全历史密钥扫描零命中
- [ ] 生产 `data/backups/` 生成首份自动备份

## 上线顺序（建议）

1. Part A/B/C/D 在 dev 完成（feature/launch-prep），`make merge-check` 合回 dev。
2. dev 合入 release → 打 tag → CI 出包。
3. 服务器首次部署按 `deploy/README.md`：配置填充 → seed 一次 → systemd + timer → 反代证书。
4. 上线后一周内：搜索引擎站长平台提交 sitemap、观察访问统计与备份留存。
