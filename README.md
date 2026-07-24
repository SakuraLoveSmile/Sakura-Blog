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
# 1. 先构建 Console SPA（go:embed 编译期要求 console/dist 存在且非空）
cd console && npm ci && npm run build && cd ..

# 2. 再编译 Go 二进制
go build -o blog ./cmd/server

# 3. 运行（默认 embed 模式；-dev 从磁盘读模板/资源/console dist，改样式即时生效）
./blog -seed   # -seed 首次启动导入 content/posts/*.md
```

常用任务：`make build`（完整构建）· `make test`（vet+测试）· `make dev`（开发模式）· `make docker`（镜像）。

> 构建顺序必须是**先 `npm run build` 再 `go build`**，否则 `console/embed.go` 的 `//go:embed all:dist` 因目录缺失编译失败。Docker 构建已在多阶段（node → go）固化该顺序。

Docker 部署：`docker build -t blog .`（node 阶段构建 console → go 阶段编译 embed 二进制）。

详见 [Demo 验证阶段计划](../.claude/plans/stellar-toasty-panda.md)。
