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

### dev 分支目录布局（前后端分组）

`dev/` 内前后端代码物理分组：

| 目录 | 归属 | 内容 |
|------|------|------|
| `backend/` | 后端 | Go module（`module blog`）：`cmd/`（入口）、`internal/`（业务代码）、`config/`（app.yml/site.yml）、`go.mod`/`go.sum` |
| `frontend/` | 前端 | 全部前端代码与构建链：`console/`（后台 Vue SPA）、`assets/`（前台静态资源，go:embed）、`build/`（Stylus CSS 编译链）、`scripts/`（node:test 行为测试）、`package.json`（stylus 依赖）、`go.mod`（module blog/frontend，仅容纳 embed 包） |
| `content/` | 内容 | Markdown 文章与笔记（git 跟踪）：`posts/*.md`（-seed 导入源 + 前台文件回退）、`notes.md`（/notes 正文） |
| `data/` | 运行时数据 | DB/上传/备份/缓存（gitignored）：`blog.db`、`uploads/`、`backups/`、`ogcache/`、`ghcache/` |
| `go.work` 等 | 根级 | `go.work`（workspace 关联 backend/frontend 两 module）、`deploy/`（运维：systemd + update.sh）、`specs/`（spec 文档）、`Makefile`（构建入口） |

前后台渲染机制不变：前台 Go html/template SSR（动态渲染，文章发布即时可见），后台 Vue SPA 经 `/api/v1` 与后端通信。

> **Go workspace 运行约束**：所有 go 命令（`go run`/`go build`/`go vet`/`go test`）必须在 `dev/` 根目录（`go.work` 所在）执行，workspace 才会同时解析 backend 与 frontend 两 module；单独进入 `backend/` 或 `frontend/` 内执行会因找不到对端 module 而 import 失败。包路径显式写 `./backend/...`/`./frontend/...`（`./...` 不匹配 use 的 module）。

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
# 1. 先构建 Console SPA（go:embed 编译期要求 frontend/console/dist 存在且非空）
cd frontend/console && npm ci && npm run build && cd ../..

# 2. 再编译 Go 二进制
go build -o blog ./backend/cmd/server

# 3. 运行（默认 embed 模式；-dev 从磁盘读模板/资源/console dist，改样式即时生效）
./blog -seed   # -seed 首次启动导入 content/posts/*.md
```

常用任务：`make build`（完整构建）· `make test`（vet+测试）· `make dev`（开发模式）。

> 构建顺序必须是**先 `npm run build` 再 `go build`**，否则 `frontend/console/embed.go` 的 `//go:embed all:dist` 因目录缺失编译失败（`make build` 已固化该顺序）。

生产部署走 `deploy/` 目录：systemd 服务单元（`blog.service`）+ 自动更新脚本（`update.sh`，从 GitHub Release 拉取单二进制）。

详见 [Demo 验证阶段计划](../.claude/plans/stellar-toasty-panda.md)。
