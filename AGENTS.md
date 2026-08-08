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
| A | 评论系统 | `backend/internal/comment/`、`frontend/console/src/modules/comments/` |
| B | 前端样式 | `frontend/assets/css/`、`backend/internal/template/partial/` |
| C | 后台管理 | `frontend/console/src/`、`frontend/console/tests/` |

#### UI 类任务开工约定（验收边界）

UI/样式类任务（控制台界面、前台样式、编辑器视觉等）开工前必须先固定**单一验收边界**，未固定前不动手改代码：

- **参照图唯一**：开工前确定唯一参照（设计稿 / 原版 Stellar 截图 / 基线截图），路径或来源写进任务描述；后续所有反馈只与该参照对比，中途不得更换参照或以口述追加新方向。
- **可逐项核对的验收清单**：把目标拆成清单，每项是一个可观察的完成判据（例："导航栏滚动时保持固定且无背景模糊"），完成时逐项对照勾选，而不是靠多轮截图来回纠偏。
- **大范围整改必须拆分**：涉及多个页面/组件的 UI 整改，必须拆分为逐项可验收的子任务，每个子任务对应一个明确的完成判据；禁止在单个任务里承载多个验收目标（如"重构编辑框 + 修批注 + 全局样式意见"混在一起）。
- **中途新增按新子任务处理**：清单固定后新出现的诉求另开子任务，不追加进进行中的任务。

### 3. 合并前检查（必做）

feature 分支合回 dev 前，**由负责该 feature 分支的 agent** 在自己的 feature worktree 内执行机械化合并封装：

```bash
# 在 feature worktree 根目录执行（默认 dev worktree 在 ../dev，可用 DEV_WORKTREE= 覆盖）
make merge-check
```

- `merge-check` 强制先执行 `make test`（console 测试 + `go vet` + `go test`），任一环节失败即以非零码退出并中止，**不会产生合并**；禁止绕过该目标手动合并。
- `make test` 全绿后，`merge-check` 才把当前 feature 分支合入 `DEV_WORKTREE` 指向的 dev worktree，门槛由 Make 退出码机械保证，无需自报留痕。
- 本步骤只是本地前置门槛，**不改变现有 CI 触发配置**，CI 仍按 `.github/workflows/` 现状运行。
- **dev 上直接工作的提交门禁**：agent 若未经 feature 合并、直接在 `dev`/`main` 等受保护分支上改动（如紧急修复、用户授权的直接提交），提交前必须在当前 worktree 根目录执行：

  ```bash
  make pre-commit-check
  ```

  - `pre-commit-check` 与 `merge-check` 共用同一 `make test` 门槛：任一环节失败即以非零码退出，**必须中止提交**；禁止绕过该目标手动 `git commit`。
  - 该条款使控制面覆盖 feature 合并之外的直接提交路径，两条路径的准入标准一致（全绿才放行）。

### 4. 先完成先合

```bash
# agent A 完成后，在 feature worktree 里执行机械化合并封装（test 通过才会合入 dev）
make merge-check

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
| 未通过 `make merge-check` 就把 feature 合回 dev | 破坏 dev 主线，阻塞其他 agent 的 rebase |
| 在 dev/main 上直接改动却未跑 `make pre-commit-check` 就提交 | 未验证变更混入主线，绕过既有控制面 |
| agent 基于过时的 dev 快照开分支 | 合并时大量冲突 |
| UI 任务无参照图/验收清单就开工 | 多轮截图纠偏返工，方向偏差只能事后发现 |
| 不查坑点知识就修改对应模块 | 重踩已验证过的坑（编译链路、模板结构、SQLite 等），浪费排查轮次 |

## 工程坑点知识路由

> 与提供方无关的工程坑点知识（已验证的编译链路、模板结构、图标机制、SQLite、PJAX 等坑点）的 canonical 来源是 [CLAUDE.md](CLAUDE.md)，本节只做路由、不复制内容，与「端口约定」「提交规范」的 canonical+回指治理模式一致。该知识适用于所有 agent（Qoder / Claude / Trae 等），不限于 Claude。

任一 agent 动手修改对应模块前，必须先查阅 `CLAUDE.md` 相应章节：

| 改动范围 | `CLAUDE.md` 必读章节 |
|---------|-------------------|
| bare 仓库 / worktree 操作 | 「仓库工作流」（worktree 操作注意、cwd 不保留坑） |
| CSS / Stylus 编译、图标与深色模式 | 「Stellar CSS 剥离规范」（shim 注入、返回值类型、siteblur 遮罩、图标颜色） |
| 前台 HTML / Go 模板结构 | 「Stellar HTML 结构规范（关键）」（class 名与嵌套必须对齐 Stellar EJS） |
| Go 服务端渲染 | 「Demo 2 新增坑点」（-root 参数、template.HTML、icons.yml 解析） |
| 管理后台 / SQLite | 「Demo 3 新增坑点」（driver name、模板路径化命名、Vue Console 迁移现状） |
| DB 迁移 / 配置文件 | 「Step 1 新增坑点」（ALTER 迁移、yaml 引号） |
| 归档 / 标签 / 缓存头 / JS shim | 「Step 2 新增坑点」「Step 2 收尾修复」（tags 匹配一致性、no-cache、移动端 sidebar） |
| TOC / 搜索 / PJAX / 404 | 「Step 3 新增坑点」（AutoHeadingID、PJAX DOM 引用、对照原版先行） |
| 工具链版本 | 「工具链」（Go / Node / stylus / SQLite 驱动版本与启动方式） |

## UI 设计规范

> 本节为 UI 设计规范的 canonical 路由；真值以 `.design_library/苹果/` 内文件为准（Pinguo Apple，`metadata.json` id `dl_builtin_apple`）。

任一 agent 新增/修改管理后台或前台 UI 前，必须先读 `.design_library/苹果/`，阅读顺序见 `library-consumption.json`（SKILL.md → README.md → css.json → components/index.json → components/{slug}.json → preview/component-{slug}.html → components.css → ui_kits/website/index.html），并遵守：

- 只消费语义 token（`:root` 亮 / `.dark` 暗两层映射，`--primary`/`--background`/`--muted-foreground`/`--border`/`--radius`/`--shadow-*` 及 `--apple-*` 前缀别名），不新造裸色值。
- 组件模式限 button / card / input / navigation / product-grid / comparison-table 六类，类名与结构对齐 `components.css`。
- 品牌蓝 `#007AFF`（dark 用 `brand-400`）、单页主色 ≤ 2；DM Sans + JetBrains Mono；圆角 `1.2rem`、按钮 pill；间距基 `0.24rem`、留白优先于分隔线；1px 边框优先、阴影从轻（深阴影仅 hover/overlay）；动效 `cubic-bezier(0.32,0.72,0,1)` 150/250/350ms；移动端触达 ≥ 44×44pt；文案简短克制、无 emoji。

## 端口约定

> 本节为端口约定的 canonical 来源，运行时真值以 `config/app.yml` 的 `listen_addr`（当前 `:8080`）为准；`.claude/launch.json` 的 go-server 配置与之保持一致。其他文档（如 CLAUDE.md）仅引用本节，不另行定义。

多个 worktree 同时起服务时用不同端口避免冲突：

- dev: `8080`
- qoder 分支: `8081`
- 其他 feature 分支: `8082` / `8083` / ...

## 提交规范

> 本节为提交规范的 canonical 来源，其他文档（如 CLAUDE.md）仅引用本节，不另行定义。

- 中文提交信息，格式：`类型: 简短描述`
- 类型：`feat` / `fix` / `refactor` / `docs` / `chore`
- 代码内容、错误信息、文件路径保持英文；注释用中文
