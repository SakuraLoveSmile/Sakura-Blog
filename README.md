# Stellar 博客项目

个人博客项目：Go (Gin) 后端 + Go html/template 服务端渲染 + 从 Stellar Hexo 主题提取的 CSS + 自建管理后台。

## 仓库结构

本仓库采用单仓库 + git worktree 工作流，三分支对应三份工作树：

| 分支 | 工作树路径 | 用途 |
|------|-----------|------|
| `main` | `.`（根目录） | 主分支，稳定版本 |
| `dev` | `dev/` | 日常开发 |
| `release` | `release/` | 发布版本 |

`resource/` 目录存放参考资源（hexo-theme-stellar 主题克隆），已 gitignore，不入版本控制。

## 技术栈

- 后端：Go + Gin
- 模板：Go html/template（服务端渲染）
- 样式：从 Stellar Stylus 源码编译提取的 CSS
- 数据库：SQLite
- 评论：Artalk

详见 [Demo 验证阶段计划](../.claude/plans/stellar-toasty-panda.md)。
