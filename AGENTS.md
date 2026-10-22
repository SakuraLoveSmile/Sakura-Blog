# Blog Agent 协作与交付规范

`AGENTS.md` 是本仓库自动加载的执行规则：规定怎样开始、验证、集成和交付。技术实现事实、模块边界与已验证的坑点放在 [CLAUDE.md](CLAUDE.md)，按需查阅即可。

两份文件**不合并**，也不新建不被通用 agent 约定识别的 `agent.md`：把工程参考全文塞进 `AGENTS.md` 会让每次任务都加载大量与当前改动无关的历史细节；保留「短执行规则 + 按需工程参考」可以降低误用与上下文噪声。

运行时与构建行为的最终真值始终是代码、`Makefile`、`config/app.example.yml` 和 CI 工作流；本文只记录协作约束。

## 1. 开工前的边界

- Blog 根目录是日常 `dev` 工作树。先看 `git status --short --branch`，已有改动、未跟踪文件和其他 worktree 都属于用户上下文；不得覆盖、清理或顺带格式化无关内容。
- 默认一个实现任务对应一个临时分支和一个独立 worktree，统一放在 `/Users/sakurasep/Documents/Code/Project/Project Worktree/Blog/<任务标识>/`。多个 agent 必须按模块拆分，不能共享分支或写同一批文件。
- 仅做审计、定位或答疑时保持只读；实现前先读与改动范围对应的 [CLAUDE.md](CLAUDE.md) 章节。
- 不得自行 `git push`、创建发布、部署生产、删除远端分支、清理其他任务资源或重置共享的 `resource/`。这些操作须有用户的明确授权；本任务合并验证完成后的本地 worktree 收尾按第 3 节执行，无需再次确认。

## 2. 分支与 worktree

从根目录的最新 `dev` 在统一目录中建立 worktree；默认分支前缀为 `codex/`，用户指定名称时以用户要求为准。所有新 worktree（包括子 agent 和临时验证工作树）均放在上述目录，工具不能指定位置时使用显式绝对路径的 `git worktree add`。

```bash
# 在 Blog 根目录执行
blog_worktree="/Users/sakurasep/Documents/Code/Project/Project Worktree/Blog/<topic>"
mkdir -p "/Users/sakurasep/Documents/Code/Project/Project Worktree/Blog"
git worktree add "$blog_worktree" -b "codex/<topic>" dev
git -C "$blog_worktree" status --short --branch
make -C "$blog_worktree" hooks-install
```

- 创建前检查 `git worktree list` 和任务归属；临时 worktree 必须位于统一目录，不能落在 Blog 同级、仓库内部或工具默认目录。
- `main` 与 `release` 是远端稳定/发布分支；本地长期集成目标只有根目录的 `dev`。
- 如果其他任务已经合入 `dev`，在自己的 feature worktree 中先 `git rebase dev`，解决自己改动的冲突后再继续。
- 需要 `resource/` 的前台/CSS/UI 任务，可让独立 worktree 用绝对路径链接根目录现有的 `resource/`；不要另外克隆、切换或重置这个跨 worktree 共用的目录。

## 3. 验证、集成与清理

每个新 worktree 先执行一次 `make hooks-install`。它只写入当前 worktree 的本地 Git 配置，将 tracked 的 `.githooks/pre-commit` 挂到 `core.hooksPath`。

### Feature 分支

完成后在 feature worktree 根目录执行：

```bash
make merge-check
```

该目标会先确认 hook 已挂载，再运行 `make test`；通过后才合入根目录的 `dev`，并在合并后的 `dev` 上再跑一次 `make test`。若组合态失败，输出的 `ORIG_HEAD` 回退命令仅供人工确认后使用；不要替用户擅自回退或清理。

`make test` 当前包含：Console 类型检查与行为测试、Stellar 资源行为测试、部署/发布来源/模块边界检查，以及 `go vet`、Staticcheck、`go test ./...`。它会用占位 `dist` 与 `dist-uc` 满足 `go:embed`，**不等于**完成真实 Console 构建；涉及前端产物时还应按风险运行 `make build` 和相关浏览器/E2E 验收。

### 直接在 `dev` 工作

除非是用户明确授权的紧急改动，否则不要直接在 `dev` 实现。若确需在 `dev` 提交，提交前必须执行：

```bash
make pre-commit-check
```

这与 hook 共用同一测试门槛；禁止用普通手工合并或跳过验证来绕开它。

### 合并后的及时清理

本任务合入约定目标且组合态验证通过后，在同一任务内及时清理已完成的 worktree 和已合并本地任务分支，无需另行确认。删除前核对预期提交或等效改动已在目标中，工作树无未保存或独有成果，且无运行任务、服务或仍需保留的路径引用；需保留的未跟踪、忽略文件先迁出或建立可核验、可恢复的备份。

从待删除目录之外先执行 `git worktree remove "<path>"`，再执行 `git branch -d <branch>`，最后核对 `git worktree list`、本地分支和实际目录。只移除本任务不再使用的空容器，保留统一根目录、共享 `resource/`、主工作区、其他任务与恢复归档；无法安全移除时保留并报告具体路径与原因，不强制删除。远端分支删除仍需明确授权。

### 发布 release

发布路径只有一条：`dev` → `main`（PR）→ `release`（PR）→ 在 `release` 的合并提交上打 `vX.Y.Z` tag。服务器采用 Docker 部署，**一次发布 = 二进制 Release + GHCR 镜像，两者都成功才算完成**。

用户要求「完成 feature 后按要求发布」时按本节执行。标注「需授权」的外部步骤（推送、开/合并 PR、打 tag、服务器部署）仍须在当次请求中获得明确同意；流程要求本身不构成预先授权。

1. **准备（本地）**：feature 已通过 `make merge-check` 合入 `dev`，且 `dev` 已推送。先 `git fetch origin`，确认 `git rev-list --count dev..origin/main` 为 `0`。若 `origin/main` 有 `dev` 缺少的提交（例如直接合入 `main` 的热修），先在 feature worktree 中从 `origin/main` 建同步分支，经 `make merge-check` 合回 `dev` 并解决冲突，再开 PR。
2. **dev → main（需授权）**：开 PR，CI 全部通过后合并。
3. **main → release（需授权）**：开 PR。`release-source` 校验来源必须是本仓库的 `main`；`ci.yml` 的 `image` 作业预构建 Dockerfile（不推送）。两者都通过才能合并。推送 `release` 分支只产生 Actions 构建产物，不创建 Release，也不推送镜像。
4. **打 tag（需授权）**：在 `release` 的合并提交上打 `vX.Y.Z` 并推送。tag 同时触发两个 workflow：
   - `release.yml`：构建 `blog-linux-amd64`、`.sha256` 与 SPDX SBOM，发布到 `SakuraLoveSmile/Sakura-Blog-Release`。
   - `docker-release.yml`：推送 `ghcr.io/sakuralovesmile/sakura-blog`。镜像标签不带 `v`，为 `X.Y.Z`、`X.Y`、`X`、`latest` 与 `sha-<短哈希>`。
5. **完成判定**（缺任一项即未完成，不得宣布发布）：
   - 该 tag 上 `Release Build` 与 `Docker Release` 均为 `success`（`gh run list --workflow=release.yml` 与 `--workflow=docker-release.yml`）。
   - 公开 Release 含 `blog-linux-amd64`、`blog-linux-amd64.sha256`、`blog-linux-amd64.spdx.json` 三个资产。
   - `docker run --rm ghcr.io/sakuralovesmile/sakura-blog:X.Y.Z -h` 退出码为 0，证明镜像内二进制可启动。
6. **服务器升级（需授权，且第 5 步已完成）**：compose 的 `image` 固定为 `ghcr.io/sakuralovesmile/sakura-blog:X.Y.Z`，执行 `docker compose pull && docker compose up -d`，再探测宿主映射端口的 `/readyz`，并在 Console 核对版本。不要以 `latest` 是否变化作为版本证据。
   - 回退：把 `image` 改回上一个已验证的版本标签并 `up -d`。若该版本含数据迁移，回退前必须先恢复停服前的完整数据备份，只切回镜像不足以回退（见 `deploy/README.md`「整站恢复与维护窗口」）。
   - 若服务器改用宿主挂载二进制（`update-docker.sh`），第 5 步的 Release 资产检查同样适用，升级步骤见 `deploy/README.md`。

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
- Stellar 前台按用户已确认的完整 2.0 移植目标支持浅色、深色和自动配色，四种 appearance 预设均须验收；不得借此次前台移植顺带重设计 Console。
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
