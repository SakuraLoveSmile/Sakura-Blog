# BLG-20261107-01：笔记本在前台可见与 Console 笔记本优化

<!-- 状态词只用 未开始 / 实现中 / 待体验 / 已验收；已验收须用户明确确认。 -->

- 项目 / 任务编号：Blog / BLG-20261107-01
- 状态：已验收（用户于 2026-11-07 明确确认验收完成；实现与自动化验证已完成；已获授权提交并合入本地 `dev`）
- 创建日期（Asia/Taipei）：2026-11-07
- 权威记录：本文件；截图与可复核事实见 [docs/baselines/notebook-visibility-console/README.md](../baselines/notebook-visibility-console/README.md)
- 主仓库 / 当前工作目录：`/Users/sakurasep/Documents/Code/Project/Personal Project/Blog`（根 `dev` 工作树，保留用户原有未提交改动，不触碰；用户的开发服务 `./blog -dev` 仍在 8097 运行旧二进制，本任务没有碰它）/ `/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261107-feat-blg-20261107-01-notebook-visibility-console`
- 基线 / 分支 / 合入目标：`09061da5`（dev）/ `codex/feat/20261107-blg-20261107-01-notebook-visibility-console` / 本地 `dev`（用户已于 2026-11-07 授权提交并合入本地 `dev`；推送、发布与真实库迁移未获授权，不做）
- 运行资源：两个隔离沙箱服务（只监听 `127.0.0.1`，由本工作树的 `go run ./cmd/blog -dev` 启动，进程归本任务）已在验收后停止，`lsof -nP -iTCP:8197 -sTCP:LISTEN` 与 `-iTCP:8198` 都没有监听；用户自己的 8097 服务没有被碰。需要重新体验时，按基线 README 的步骤重建沙箱。8197 是已初始化 Stellar 2.0 的空站点（根目录 `.tmp/e2e-server.c9DopU`，由 `scripts/e2e/prepare.go` 初始化），里面的“1.40.0 验收笔记本 / 响应式检查记录（已更新）”由 Console 新界面按用户旧数据的内容创建并发布；8198 是旧版布局（根目录 `.tmp/legacy-sandbox`：旧 `notebooks`/`notes` 表加用户那条测试数据的拷贝，没有 Stellar 2.0 快照），与用户真实数据库同类状态。两个根目录都在被忽略的 `.tmp/`，可删。用户真实数据库只在会话临时目录里读过一份拷贝（任务收尾时已删除），没有写入、没有对其启动服务。不依赖用户真实数据的复现步骤、合成旧库夹具和截图脚本都在基线目录（见 README）。

## 背景与根因

用户在 Console 里建了一条测试笔记，却在博客里看不到它。查下来**不是笔记渲染的问题**：

- 用户运行中的服务（`./blog -dev`，8097，根目录就是主仓库）使用的 `data/blog.db` 还是 Stellar 2.0 之前的旧库：`settings` 里没有 `stellar_v2` 分区，`notebooks`/`notes` 是旧布局（整数 ID、`nb_order`、`title/content/content_html/tags`）。
- 旧库在迁移前被有意拦住：所有公开页面（包括 `/notebooks`）都返回 HTTP 409 `stellar_migration_required`，浏览器里看到的是一段裸 JSON；Console 的笔记本接口同样 409，页面只显示一条红色的后端原文。
- 那条测试笔记是旧库里的 1 行：笔记本“1.40.0 验收笔记本”（旧 ID 2）里的“响应式检查记录（已更新）”。迁移从未执行过，也没有获得授权，所以**没有改动用户的数据**。

## 目标与范围

1. 让“看不到笔记”的原因一眼就能看明白，而不是裸 JSON 或一段后端原文。
2. 让用户能确切看到这条测试笔记在博客里是什么样子（不动真实数据）。
3. 优化 Console 的笔记本：能看到每个笔记本/笔记在博客里的地址并一键打开，编辑时用结构化字段而不是一整块 JSON。

明确不做：执行迁移或改动用户的 `data/blog.db`；改变迁移流程与旧库的拦截规则；去掉笔记本功能（上游 Stellar 2.0.0 自带 Notebook/Note，已向用户说明建议保留）；共享富文本编辑器、图标选择器、附件选择器与“高级”面板的内部样式。

## 实现

**公开站（`modules/stellar/routes`）**

- 迁移未完成时，浏览器页面导航（`Accept` 含 `text/html`）得到一页说明“站点内容正在升级”（HTTP 409、`Cache-Control: no-store`、`X-Robots-Tag: noindex`，浅色/深色自适应，只含固定文案，不拼接任何请求或站点数据）；`curl`、脚本、API 客户端（`Accept` 为空、`*/*` 或 `application/json`）仍拿到原来的 JSON。位置：`stellar_migration_page.go`、`stellar_http.go` 的迁移分支。
- 笔记本集合的前台路径抽成 `internal/stellarv2.CollectionBasePath`，前台路由与 Console 共用同一份计算（集合 `route.path` 优先，否则主题 `profiles.<index>.path` 加集合 ID；ID 做路径转义）。`stellar_runtime.go` 改为调用它，行为不变。

**Console API（`modules/console/api`，类型经 tygo 生成）**

- `NotebookResponse` 新增 `public_path`、`note_count`、`published_count`；`NotebookNoteResponse` 新增 `public_path`。数量按当前用户的管理权限统计（作者只数自己的，管理员数全部），一次 `GROUP BY` 查询（`store.CountNotes`），不加载正文。
- 笔记的前台路径只给已发布笔记；笔记配置了 `permalink` 时与前台一致地优先使用，但只取其中的站内路径部分，`javascript:`、`//外站/…`、`https://外站/…` 都不会变成跳转链接（见测试）。站点还没有 Stellar 2.0 快照（迁移前）时不给路径，Console 据此不显示打不开的链接。

**Console 界面（`modules/console/web`）**

- 笔记本列表：每个笔记本一张卡片（名称、ID、描述、“N 篇笔记 · M 篇已发布”、管理笔记/编辑/在博客中查看）；没有已发布笔记时说明“发布笔记后，博客前台才会显示这个笔记本”，不给打不开的链接。
- 笔记列表：标题、状态标签、Slug、更新时间、标签、“在博客中查看”（仅已发布）；工具栏输入框有最小宽度，窄屏自动换行。
- 笔记本编辑：名称、描述、排序（写入上游 `listing.order`，数字小的在前；清空则移除该字段）、图标、封面用结构化字段，与 JSON 原文双向同步，只重写被改的节点；完整 JSON 收进折叠的“高级”面板。笔记编辑：标题（留空则前台回退到 Slug）与标签结构化，Front matter 原文折叠。
- 迁移未完成时用一块说明替换红色后端原文：讲清影响（前台只显示“站点内容正在升级”、这里不能编辑）和下一步（管理员在服务器上审阅迁移预览并执行迁移、重启），原始报错收进“技术详情”。笔记本相关请求关闭全局错误 Toast（页面内联展示），写入失败时只弹一条简短 Toast，迁移失败不再把整段 schema 报错堆在界面上。
- 侧栏图标改为 `ri:sticky-note-line`（`ri-subset.json` 已按源码用法重新生成）；手机上“在博客中查看”、笔记标题链接与共享“高级”面板摘要的点击区补到 ≥44px，不改变版面。

## 验证记录

| 项 | 命令 / 做法 | 结果 |
| --- | --- | --- |
| 完整本地门禁 | `make test`（Console 类型与行为测试、Stellar 资源、部署/发布来源/模块边界检查、`go vet`、Staticcheck、全部 Go 测试） | 通过（最终代码上重跑，见“最终复核”；退出码 0，输出无 FAIL） |
| 真实构建 | `make build`（vue-tsc、两套 vite 构建、压缩、Go 编译） | 通过（最终代码上重跑） |
| Console 新增测试 | `modules/console/tests/notebooks.test.ts` 共 10 项，其中 5 项为本任务新增（结构化字段只改被碰节点并保留大整数/显式空值、标题与标签同步且集合归属不变、计数/状态/时间文案、迁移说明与外链 `rel`、笔记本请求不触发全局 Toast） | 10/10 通过 |
| Go 新增测试 | `internal/stellarv2/collection_path_test.go`、`internal/store/note_counts_test.go`、`modules/console/api/notebook_public_test.go`（路径、权限感知计数、`route.path`/`permalink`、恶意 `permalink` 只得站内路径、无主题快照不给路径）、`modules/stellar/routes/stellar_migration_page_test.go`（浏览器得 HTML、其余仍 JSON、无遗留数据泄漏） | 通过 |
| ESLint | `pnpm run lint`（CI `lint` 作业同款，全量，`--max-warnings=0`） | 通过 |
| Prettier | `prettier --check` 笔记本模块与 `notebooks.ts` | 通过；`stellar-form-fields.ts`、`module.ts` 在基线上就不符合 Prettier，没有顺手整文件重排 |
| 真实浏览器验收 | 两个隔离沙箱 + Playwright（Chromium，1280×860 浅色、390×844 深色，DPR 2）逐页截图并记录事实；脚本 `capture-notebook.mjs`、合成旧库夹具 `legacy-fixture.sql`、复现步骤和结果都在基线目录 | 全部检查通过（15 项、0 失败）：公开链接都是站内路径且返回 200；旧布局下浏览器得到可读的 409 页、API 客户端仍得 JSON；Console 显示迁移说明且原始报错默认折叠；手机端本任务控件 ≥44px；所有已检查页面无横向溢出 |

通过界面（不是直接调接口）完成：新建笔记本（含排序 10）→ 新建笔记（标题、Slug、三个标签、Markdown 正文）→ 发布 → 列表出现“在博客中查看” → 前台 `/notebooks`、`/notebooks/acceptance`、`/notebooks/acceptance/responsive-check` 都显示这条笔记（标题、“检查项”小节、三条列表、标签、许可协议、评论区、目录）。

## 最终复核

最后一次改动源码（对四个 Vue 文件执行 Prettier 整理）之后，在同一份代码上按顺序重跑了下面这些；此后只新增或修改了文档、基线脚本和夹具，没有再动源码（已按文件修改时间核对）：

- `make test`：退出码 0，输出里没有 FAIL。
- `make build`：退出码 0（真实构建 Console 与 User Center，再编译 Go 二进制）。
- `modules/console/tests/notebooks.test.ts` 10/10 通过；`vue-tsc` 退出码 0；`pnpm run lint` 全量退出码 0；`prettier --check` 笔记本模块与 `notebooks.ts` 通过。
- 沙箱验收脚本 15 项检查全部通过、0 失败；基线目录的截图与 `verification.json` 由已跟踪的 `capture-notebook.mjs` 重新生成。
- `legacy-fixture.sql` 在一个临时沙箱上验证过确实能复现旧版布局：浏览器得到 409 说明页、API 客户端得到 JSON、旧表原样保留、没有 `stellar_v2` 分区；验证完已删除该沙箱。

## 未验证与边界

- 只在本机 Chromium（Playwright 与应用内浏览器）验证；没有 Safari/Firefox、真机触屏或读屏器的证据。
- 前台截图来自沙箱里“按用户旧数据内容重新创建”的笔记，不是对用户真实数据库迁移后的结果；迁移后的实际效果（旧标签 `responsive,console,1.40.0` 的转换、旧 ID 2 的新归属等）要等迁移预览与执行才能确认。
- 共享富文本工具栏（30px 按钮）、图标/封面选择器按钮（36px）沿用既有尺寸，不在本任务内改；手机端 44px 的判定只覆盖本模块自己的控件，基线 `verification.json` 里单独列出了共享控件。
- 用户的 8097 服务仍是旧二进制：本任务的改动要合并、重新构建并重启后才会出现在那里；旧库在迁移前仍会返回 409，只是浏览器里看到的是说明页而不是 JSON。

## 验收记录

2026-11-07 用户在对话里明确确认：“可以 验收完成”，状态据此从待体验改为已验收。验收只确认体验结果，不等于授权提交、合入 `dev`、推送、发布或对用户真实数据库执行迁移。验收后已停止两个沙箱服务（见文件开头的“运行资源”）。

同日用户另行明确授权：“提交并合入 dev”。据此本任务按 AGENTS.md 提交并执行 `make merge-check`；推送、发布和真实库迁移仍未获授权，不做。

## 待用户决策

1. **是否执行真实迁移**：要在用户自己的博客里看到这条笔记，必须先把旧库迁移到 Stellar 2.0（预览与执行流程见 `docs/stellar-200-migration-preview.md`）。迁移会改动 `data/blog.db`，需要用户明确授权，并且先备份；本任务没有也不会擅自执行。另外，按该文档里用户已确认的决定，旧笔记 `notes/2`（“响应式检查记录（已更新）”）迁移后 slug 为 `note-2`、状态为 draft（“明确先 draft，不自动发布”），所属笔记本保持公开路径 `/notebooks/2`、排序 10。所以迁移之后还要在 Console 里发布它，博客前台才会显示；发布前笔记本卡片会显示“1 篇笔记 · 0 篇已发布”和“发布笔记后，博客前台才会显示这个笔记本”，不给打不开的链接。这是按该文档推算的预期，迁移本身没有在本任务里运行过。
2. 推送与发布：用户尚未授权（提交与合入本地 `dev` 已于 2026-11-07 获得授权，见“验收记录”）。

## 提交与集成

2026-11-07 用户授权提交并合入本地 `dev`。流程按 AGENTS.md：在本工作树分支用中文 `feat:` 信息提交（提交 hook 会先跑 `make test` 门禁），再执行 `make merge-check`（`make test` → 合入根目录 `dev` → 在合并后的 `dev` 上再跑一次 `make test`），通过后清理本任务 worktree 与已合并的本地分支。没有推送、没有创建 tag/release、没有部署，也没有对用户真实数据库执行迁移。实际的提交与合并结果以 `dev` 的提交历史为准。
