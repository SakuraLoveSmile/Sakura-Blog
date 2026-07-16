# Stellar 博客项目

个人博客项目：Go (Gin) 后端 + Go html/template 服务端渲染 + 从 Stellar Hexo 主题提取的 CSS + 自建管理后台。

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
- 模板：Go html/template（服务端渲染）
- 样式：从 Stellar Stylus 源码编译提取的 CSS
- 数据库：SQLite
- 评论：Artalk

详见 [Demo 验证阶段计划](../.claude/plans/stellar-toasty-panda.md)。
