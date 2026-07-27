# Multi-Agent 并行开发规范

本项目使用 bare 仓库 + git worktree 管理多分支。当多个 AI agent（Qoder / Claude / Trae 等）需要并行开发时，遵循以下规范。

## 核心原则

**一个 agent = 一个 feature 分支 = 一个 worktree，所有分支从 dev 拉出，做完合回 dev。**

绝不让两个 agent 写同一个分支，也绝不让 agent 在独立分支上完整重写项目。

## 分支模型

```
dev（主线，唯一集成目标）
├── feature/comment-system    ← agent A
├── feature/lxgw-font         ← agent B
└── feature/tag-plugins       ← agent C
```

保留分支：`dev`（日常开发）/ `main`（稳定快照）/ `release`（发布）。
所有 feature 分支短生命周期，合完即删。

## 操作流程

### 1. 开分支 + worktree

```bash
# 给 agent A 开 feature 分支
git -C .bare worktree add ../feat-comment -b feature/comment

# 给 agent B 开 feature 分支
git -C .bare worktree add ../feat-font -b feature/font
```

### 2. 任务拆解：模块级隔离

分配任务时确保各 agent 改动的文件/模块尽量不交叉。示例：

| Agent | 任务 | 主要改动范围 |
|-------|------|-------------|
| A | 评论系统 | `internal/comment/`、`admin/comments.html` |
| B | 前端样式 | `assets/css/`、`internal/template/partial/` |
| C | 后台管理 | `internal/admin/`、`admin/*.html` |

### 3. 合并前检查（必做）

feature 分支合回 dev 前，**由负责该 feature 分支的 agent** 在自己的 feature worktree 内执行：

```bash
# 在 feature worktree 根目录执行完整测试
make test
```

- `make test` 必须通过（console 测试 + `go vet` + `go test`），失败则先修复，禁止带红合并。
- 检查结果需记录留痕：在合并说明或任务汇报中写明 `make test` 的执行结果（通过/失败及处理）。
- 本步骤只是本地前置门槛，**不改变现有 CI 触发配置**，CI 仍按 `.github/workflows/` 现状运行。

### 4. 先完成先合

```bash
# agent A 完成后（已通过合并前检查），在 dev worktree 里合并
git -C /path/to/dev merge feature/comment

# 删掉用完的分支
git -C /path/to/dev branch -d feature/comment
```

> 合并只发生在本地 worktree；**推送 dev 分支属外部写操作，必须由用户单独授权后再执行**，agent 不得自行 `git push`。

### 5. 后续 agent 开工前 rebase

如果 agent B 开工时 A 已合入 dev，B 必须先 rebase 到最新 dev：

```bash
git -C ../feat-font rebase dev
# 如有冲突由 agent B 自行解决（B 了解自己在改什么）
```

### 6. 清理 worktree

```bash
git -C .bare worktree remove ../feat-comment
```

## 反模式（禁止）

| 反模式 | 后果 |
|--------|------|
| 多个 agent 各自从同一祖先独立重写整套代码 | 无法 merge，只能人工策展式整合 |
| 两个 agent 写同一个分支 | 提交冲突、互相覆盖 |
| feature 分支长期不合并 | 与 dev 分叉越来越大，合并越来越难 |
| 未跑 `make test` 就把 feature 合回 dev | 破坏 dev 主线，阻塞其他 agent 的 rebase |
| agent 基于过时的 dev 快照开分支 | 合并时大量冲突 |

## 端口约定

> 本节为端口约定的 canonical 来源，运行时真值以 `config/app.yml` 的 `listen_addr`（当前 `:8080`）为准；`.claude/launch.json` 的 go-server 配置与之保持一致。其他文档（如 CLAUDE.md）仅引用本节，不另行定义。

多个 worktree 同时起服务时用不同端口避免冲突：

- dev: `8080`
- qoder 分支: `8081`
- 其他 feature 分支: `8082` / `8083` / ...

## 提交规范

- 中文提交信息，格式：`类型: 简短描述`
- 类型：`feat` / `fix` / `refactor` / `docs` / `chore`
- 代码内容、错误信息、文件路径保持英文；注释用中文
