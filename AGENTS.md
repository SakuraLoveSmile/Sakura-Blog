# Blog Agent 协作与交付规范

`AGENTS.md` 是本仓库自动加载的执行规则：规定怎样开始、验证、集成和交付。技术实现事实、模块边界与已验证的坑点放在 [CLAUDE.md](CLAUDE.md)，按需查阅即可。

两份文件**不合并**，也不新建不被通用 agent 约定识别的 `agent.md`：把工程参考全文塞进 `AGENTS.md` 会让每次任务都加载大量与当前改动无关的历史细节；保留「短执行规则 + 按需工程参考」可以降低误用与上下文噪声。

运行时与构建行为的最终真值始终是代码、`Makefile`、`config/app.example.yml` 和 CI 工作流；本文只记录协作约束。

## 1. 开工前的边界

- Blog 根目录是日常 `dev` 工作树。先看 `git status --short --branch`，已有改动、未跟踪文件和其他 worktree 都属于用户上下文；不得覆盖、清理或顺带格式化无关内容。
- 默认一个实现任务对应一个临时分支和一个同级 worktree。多个 agent 必须按模块拆分，不能共享分支或写同一批文件。
- 仅做审计、定位或答疑时保持只读；实现前先读与改动范围对应的 [CLAUDE.md](CLAUDE.md) 章节。
- 不得自行 `git push`、创建发布、部署生产、删除分支/worktree 或重置共享的 `resource/`。这些操作须有用户的明确授权。

## 2. 分支与 worktree

从根目录的最新 `dev` 建立同级 worktree；默认分支前缀为 `codex/`，用户指定名称时以用户要求为准。

```bash
# 在 Blog 根目录执行
git worktree add ../Blog-<topic> -b codex/<topic>
git -C ../Blog-<topic> status --short --branch
make -C ../Blog-<topic> hooks-install
```

- 临时 worktree 必须位于 Blog 同级目录，不能落在根目录内成为未跟踪文件。
- `main` 与 `release` 是远端稳定/发布分支；本地长期集成目标只有根目录的 `dev`。
- 如果其他任务已经合入 `dev`，在自己的 feature worktree 中先 `git rebase dev`，解决自己改动的冲突后再继续。
- 需要 `resource/` 的前台/CSS/UI 任务，可让 sibling worktree 链接根目录现有的 `resource/`；不要另外克隆、切换或重置这个跨 worktree 共用的目录。

## 3. 验证、集成与清理

每个新 worktree 先执行一次 `make hooks-install`。它只写入当前 worktree 的本地 Git 配置，将 tracked 的 `.githooks/pre-commit` 挂到 `core.hooksPath`。

### Feature 分支

完成后在 feature worktree 根目录执行：

```bash
make merge-check
```

该目标会先确认 hook 已挂载，再运行 `make test`；通过后才合入根目录的 `dev`，并在合并后的 `dev` 上再跑一次 `make test`。若组合态失败，输出的 `ORIG_HEAD` 回退命令仅供人工确认后使用；不要替用户擅自回退或清理。

`make test` 当前包含：Console 类型检查与行为测试、Stellar 资源行为测试、部署/发布来源/模块边界检查，以及 `go vet`、Staticcheck、`go test ./...`。它会用占位 `dist` 满足 `go:embed`，**不等于**完成真实 Console 构建；涉及前端产物时还应按风险运行 `make build` 和相关浏览器/E2E 验收。

### 直接在 `dev` 工作

除非是用户明确授权的紧急改动，否则不要直接在 `dev` 实现。若确需在 `dev` 提交，提交前必须执行：

```bash
make pre-commit-check
```

这与 hook 共用同一测试门槛；禁止用普通手工合并或跳过验证来绕开它。获得用户授权后，才可在确认合并已完成且 worktree 不再需要时执行清理；正确顺序是先 `git worktree remove <path>`，再 `git branch -d <branch>`。

### CSS 产物

涉及 `resource/hexo-theme-stellar` 或 `modules/stellar/web/assets/css/main.css` 时：

```bash
make css
make css-drift-check
```

先检查共享主题源的版本和脏状态。`main.css` 是生成物，不能手改；`css-drift-check` 会重编并比较产物，可能暴露主题源与工作树的漂移。它是本地核对，不进入 CI/`make test`。

## 4. UI 任务的验收边界

UI/样式任务在写代码前必须固定一份可复核的验收说明：

- 选定唯一参照图（设计稿、上游 Stellar 截图或基线截图），并在任务说明写明其仓库相对路径或来源。
- 参照图和验收截图存入 `docs/baselines/<task>/`，不可放入 `.tmp/`、`.qoder/` 等忽略目录。
- 将目标拆成可观察、逐项可勾选的验收项；范围很大时拆成独立子任务。中途新增方向另开任务，不能悄悄扩写当前清单。
- 本项目产品只交付深色模式。不得新增浅色模式开关、浅色 token、浅色专用样式或浅色验收基线；上游保留的亮色兼容代码不代表产品能力。
- 修改 Console 或前台 UI 前，先按 `resource/apple-design/library-consumption.json` 的 `recommendedReadOrder` 阅读本地设计库。复用其语义 token 和既有组件结构，不临时造裸色值或平行组件体系。
- 浏览器验收必须确认服务实际加载当前 SPA 产物；Console 重建后要重启 Go 服务，不能把一个 HTTP 200 当作当前界面的证据。

## 5. 模块路由与 API 约束

| 改动范围 | 先读的工程参考 |
| --- | --- |
| 仓库结构、依赖方向、构建/运行 | `CLAUDE.md`「当前架构与模块边界」「构建、运行与验证」 |
| Console、User Center、API Client | `CLAUDE.md`「Console、User Center 与 API 契约」 |
| SQLite、配置、内容导入 | `CLAUDE.md`「数据、配置与内容生命周期」 |
| Stellar CSS、模板、图标、前台脚本 | `CLAUDE.md`「Stellar 前台实现约束」 |
| Markdown/shortcode 可视化编辑 | `CLAUDE.md`「编辑器与 shortcode」 |

- 新 API 响应使用具名 Go struct，不新增 `gin.H` 作为响应 schema；既有遗留 `gin.H` 不是新接口的范例。
- 修改纳入 `tygo.yaml` 的 Console API 类型后，在根目录运行 `go generate ./modules/console/api`，提交生成的 `packages/api-client/src/generated.d.ts`；新增可暴露类型时同时更新 `tygo.yaml`。
- 保持模块边界：`internal/` 不依赖 Console/Stellar 业务模块；Console 与 Stellar 不互相导入；公开 URL 路由归 Stellar 挂载。修改后让 `make test` 的 `module-check` 验证。

## 6. 端口、提交与外部操作

- `config/app.yml` 的 `listen_addr` 是根工作树的运行时真值（当前为 `:8080`）。并行启动时用 `BLOG_LISTEN` 为各 worktree 分配不同端口，避免按 agent 品牌硬编码端口。
- 用户授权提交时，使用中文信息：`feat:`、`fix:`、`refactor:`、`docs:` 或 `chore:` + 简短描述。代码、路径与报错保持英文；注释使用中文。
- 推送、创建 tag/release、远程 CI 等待和生产部署是独立外部操作，必须得到相应的明确授权；本地通过门禁不代表远端 CI 或生产已验证。
