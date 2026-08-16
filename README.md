# Stellar 博客项目

个人博客项目：Go (Gin) 后端 + Go html/template 服务端渲染前台 + Vue 3 Console SPA 管理后台（`/console/`）+ 从 Stellar Hexo 主题提取的 CSS。

## 仓库结构

本仓库采用 bare 仓库 + git worktree 工作流，根目录是 bare 仓库容器，三分支分别 checkout 到独立子目录：

| 分支 | 工作树路径 | 用途 |
|------|-----------|------|
| `main` | `main/` | 主分支，稳定版本 |
| `dev` | `dev/` | 日常开发 |
| `release` | `release/` | 发布版本 |

- `.bare/`：bare 仓库（git 核心），根目录的 `.git` 文件指向它
- `resource/`：参考资源（hexo-theme-stellar 主题克隆），已 gitignore，不入版本控制

在任一工作树目录内执行 git 命令即可操作对应分支。

### dev 分支目录布局（模块化单体）

`dev/` 使用单一 Go module；Console 与 Stellar 仍构建为一个 Go 二进制：

| 目录 | 归属 | 内容 |
|------|------|------|
| `cmd/blog/` | 入口 | 配置、SQLite、共享服务初始化和模块挂载 |
| `modules/console/` | Console | `api/` 管理 API、`web/` Vue SPA 与 embed 资源 |
| `modules/stellar/` | Stellar | `routes/` 前台 SSR 路由、`web/` 模板/静态资源/CSS 构建链 |
| `internal/` | 共享核心 | store、content、comments、settings/config、search、media 等业务服务 |
| `backend/config/` | 运行时配置 | 保留 `app.yml`/`site.yml` 路径，避免改变部署行为 |
| `content/` | 内容 | Markdown 文章与笔记（git 跟踪）：`posts/*.md`（-seed 导入源 + 前台文件回退）、`notes.md`（/notes 正文） |
| `data/` | 运行时数据 | DB/上传/备份/缓存（gitignored）：`blog.db`、`uploads/`、`backups/`、`ogcache/`、`ghcache/` |
| 根级 | 构建与运维 | `go.mod`/`go.sum`、`deploy/`（systemd + update.sh）、`specs/`（spec 文档）、`Makefile`（构建入口） |

前后台渲染机制不变：前台 Go html/template SSR（动态渲染，文章发布即时可见），后台 Vue SPA 经 `/api/v1` 与后端通信。

> **Go module 运行约束**：所有 go 命令从 `dev/` 根目录执行，包路径使用 `./...`；配置仍从 `backend/config/` 加载。

## 技术栈

- 后端：Go + Gin
- 模板：Go html/template（前台服务端渲染）
- 后台：Vue 3 + Vite + Pinia + Tailwind（Console SPA，经 `/api/v1` 与后端通信）
- 样式：从 Stellar Stylus 源码编译提取的 CSS
- 数据库：SQLite
- 评论：自建评论系统

## 构建与运行

单二进制发布：前台模板、静态资源、icons 与 **console 构建产物**均已 `go:embed` 进二进制。

```bash
# 一键构建（Makefile 已固化顺序：先 console 后 go）
make build

# 或手动分步：
# 1. 先构建 Console SPA（go:embed 编译期要求 modules/console/web/dist 存在且非空；
#    Makefile 已做依赖增量：node_modules 与 package-lock.json 一致时跳过 npm ci）
cd modules/console/web && npm ci && npm run build && cd ../..

# 2. 再编译 Go 二进制
go build -o blog ./cmd/blog

# 3. 运行（默认 embed 模式；-dev 从磁盘读模板/资源/console dist，改样式即时生效）
./blog -seed   # -seed 首次启动导入 content/posts/*.md
```

常用任务：`make build`（完整构建）· `make test`（vet+测试）· `make dev`（开发模式，console/dist 或 src 无变化时跳过前端重建）· `make css`（本地从 Stellar styl 源重编 `modules/stellar/web/assets/css/main.css`，见下）。

> 构建顺序必须是**先 `npm run build` 再 `go build`**，否则 `modules/console/web/embed.go` 的 `//go:embed all:dist` 因目录缺失编译失败（`make build` 已固化该顺序）。

### 重编 main.css（make css）

`modules/stellar/web/assets/css/main.css`（约 287KB）是 Stellar Stylus 源的编译产物。源在 `resource/hexo-theme-stellar/`（gitignore，不入仓库），CI 无源不可编译，故 `make css` 是**本地/agent 维护入口**，不进 test 门禁：

```bash
make css   # 依赖 resource/ 主题源 + modules/stellar/web/node_modules（stylus 0.62.0，make 自动补装）
```

产物直接覆盖 git 跟踪的 `modules/stellar/web/assets/css/main.css`，重编后应无漂移（`git diff` 为空即正确）；若编译行为有变（stylus 版本、shim 配置），先对齐 `modules/stellar/web/build/stylus/` 再重编。

> 构建顺序必须是**先 `npm run build` 再 `go build`**，否则 `modules/console/web/embed.go` 的 `//go:embed all:dist` 因目录缺失编译失败（`make build` 已固化该顺序）。

生产部署走 `deploy/` 目录：systemd 服务单元（`blog.service`）+ 自动更新脚本（`update.sh`，从 GitHub Release 拉取单二进制）。

详见 [Demo 验证阶段计划](../.claude/plans/stellar-toasty-panda.md)。
