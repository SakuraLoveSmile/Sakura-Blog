# 文章批量设置为发布 / 草稿

任务编号：BLG-20261102-04。状态：待体验（实现与验证完成，待合入本地 `dev` 后由用户体验确认）。

## 目标与范围

- 用户在 Console「文章管理」列表多选后，底部批量浮条除「批量移入回收站」外，新增「批量发布」与「批量设为草稿」，让批量操作能直接切换文章发布状态。
- 用户要求原文：批量操作应该增加批量设置为发布或者批量草稿（2026-11-02 截图见 `docs/baselines/post-batch-status/reference-before.png`）。
- 范围只含 Console 文章（posts）列表的批量状态切换。不改页面（pages）批量、评论批量、编辑器发布链路、前台模板、Stellar 资源、搜索/RAG 实现细节。
- 状态切换只改 `posts.status` 与 `posts.updated_at`：不应用、不清空草稿头（`draft_json`），不改正文、slug、作者、分类、置顶、口令、定时发布时间 `publish_at`。

## 基线

- 代码基线：`dev` / `ddecd77e`。
- 工作树：`/Users/sakurasep/Documents/Code/Project/Project Worktree/Blog/20261102-feat-blg-20261102-04-post-batch-status`。
- 分支：`codex/feat/20261102-blg-20261102-04-post-batch-status`；本地合入目标 `dev`（`make merge-check`）。
- 根工作区既有未提交内容与其他 worktree 保留，本次不触碰。

## 已确认决定

1. 新增批量端点 `POST /api/v1/posts/batch`，请求 `{slugs, action}`，`action ∈ publish|draft`；与既有 `POST /comments/batch` 同构，不新增 `gin.H` 响应 schema。
2. 语义是「切换已保存内容的发布状态」，不应用草稿头。存在未发布草稿修改（`has_draft`）的文章会在确认弹窗被明确说明：本次只切状态，草稿修改仍保留。
3. 回收站文章不参与：前端不把 `status=trashed` 计入目标；后端遇到回收站/不存在 slug 计入 `failed` 且不改动。
4. 已是目标状态的文章跳过（`skipped`），不刷新 `updated_at`，避免批量误操作打乱「最近更新」排序。
5. 单次上限 100 个 slug（与评论批量一致）。非管理员必须对每个 slug 具备文章署名权限，任一越权整单 403（与单篇接口一致，避免部分成功语义模糊）。
6. 后端成功变更后发布一次 `content.ContentChanged`（搜索索引与内容缓存失效走既有通道）；对每个实际变更的 slug 走 store 既有 `firePostWrite("update", slug)`，让 RAG 等派生数据按既有写路径处理。
7. 复用既有 `VFloatingDock`、`VButton`、`Dialog`、`Toast` 与语义 token，不新增组件、不造裸色值、不重排既有浮条结构。

## 接口契约

`POST /api/v1/posts/batch`（Console 鉴权与既有中间件一致）

请求：

```json
{ "slugs": ["who-am-i", "hexo-basics"], "action": "publish" }
```

- `slugs`：去空白、去重后 1..100 条，空 slug 或超限返回 400。
- `action`：仅 `publish` / `draft`，其他返回 400。
- 非管理员且任一 slug 无署名：403（`无权限：只能管理自己署名的文章`）。

响应 `200`：

```json
{ "succeeded": 2, "skipped": 1, "failed": 0 }
```

- `succeeded`：实际发生状态变更的条数。
- `skipped`：存在但已是目标状态的条数（未写库）。
- `failed`：不存在或位于回收站的条数。
- 全部 `skipped` 或全部 `failed` 仍是 200，由前端按计数提示。

存储层：`internal/store/post.go` 新增 `SetPostsStatusBySlug(slugs []string, status string) (succeeded, skipped, failed int, err error)`，事务内一次 `SELECT slug,status ... WHERE slug IN (...) AND deleted_at = ''` 分类，再一次 `UPDATE ... SET status=?, updated_at=?`，提交后对变更 slug 触发 `firePostWrite("update", slug)`；仅接受 `StatusPublished` / `StatusDraft`。

## 前端行为

- 浮条按钮顺序：`批量发布`（primary）→ `批量设为草稿`（default）→ `批量移入回收站`（danger，保持不变）。
- 目标集合 = 当前页选中且非 `trashed`，再按目标状态过滤：`批量发布` 只含非 `published`，`批量设为草稿` 只含非 `draft`。
- 无可操作目标时按钮 `disabled` 并给出 `title` 原因（如「所选文章均已发布」/「所选草稿均已设为草稿」/「回收站文章需先恢复再设置状态」）。
- 确认弹窗：发布用 `Dialog.info`（primary 确认），设为草稿用 `Dialog.warning`（secondary 确认）。描述含条数、前台影响与 `has_draft` 说明。
- 成功后：清空选择 → `invalidateQueries(queryKeys.posts())` → `refetch()` → Toast（`已发布 N 篇文章` / `已设为草稿 N 篇`，有失败时追加 `，M 篇未处理`）。
- 纯规划逻辑（目标集合、计数、弹窗文案）抽成 `modules/console/web/src/modules/posts/batch-status.ts`，供 `PostList.vue` 消费并由 `node:test` 覆盖。

## 验收清单

- [x] API：publish / draft 双向切换；已是目标状态不写库（`updated_at` 不变）；回收站与不存在 slug 计入 `failed`；超过 100 或非法 action 返回 400；非署名作者 403；成功变更后 `ContentChanged` 事件只发一次。见 `modules/console/api/posts_batch_test.go`（4 项）与 `internal/store/post_batch_status_test.go`（5 项）。
- [x] 存储：新库建表路径下真实落库；UPDATE 行数不符整单回滚（无半更新）；空 slug 列表安全；非法 status 报错。
- [x] 前端：按钮顺序、禁用/提示、跳过与回收站计数、确认弹窗文案与按钮类型、成功后选择清空/刷新/Toast。纯逻辑由 `modules/console/tests/post-batch-status.test.ts`（11 项）覆盖，交互由浏览器用例覆盖。
- [x] 浏览器（真实构建 + 隔离服务）：草稿 → 批量发布 → 已发布 → 批量设为草稿 的往返；桌面 1280×800 与移动 390×844 浮条不溢出、按钮可点；Console 实际加载当前 SPA 产物（入口 HTML 与功能 chunk 的 SHA256 与磁盘 `dist` 一致）。
- [x] E2E：新增 `tests/e2e/post-batch-status.spec.ts`（6 项），并跑通受影响回归。
- [ ] 门禁：`make test`、`make build` 已通过；`make merge-check` 合入 `dev` 与合并后复验待执行。

## 验证边界

- 不使用生产数据、不连接线上服务；浏览器验收使用 Playwright 隔离服务（独立数据库与 8096 端口）或本任务独立数据库与独立端口。
- 未验证项：真实用户数据规模下的批量 100 条性能、邮件/SEO 推送（批量路径不触发通知，与评论批量一致）。

## 分工与写入范围

- Lead：任务记录、基线目录、接口契约、最终 diff 复核、`make build`、提交与 `make merge-check`、合并后组合态复验。
- `api-batch`：`internal/store/post.go`、`internal/store/post_batch_status_test.go`、`modules/console/api/post.go`、`modules/console/api/router.go`、`modules/console/api/posts_batch_test.go`。
- `ui-batch`：`packages/api-client/src/api/content.ts`、`modules/console/web/src/modules/posts/PostList.vue`、`modules/console/web/src/modules/posts/batch-status.ts`、`modules/console/tests/post-batch-status.test.ts`。
- `verify-batch`：`tests/e2e/post-batch-status.spec.ts` 与 `docs/baselines/post-batch-status/` 证据文件；不改实现文件。

## 实现与实测

### 实现

- 后端：`internal/store/post.go` 新增 `SetPostsStatusBySlug`（单事务：一次 `SELECT` 分类 + 一次 `UPDATE status/updated_at`；提交后对每个实际变更 slug 触发 `firePostWrite("update")`）；`modules/console/api/post.go` 新增 `batchPosts` handler 与具名 `batchPostsRequest`/`batchPostsResponse`；`modules/console/api/router.go` 注册 `author.POST("/posts/batch", h.batchPosts)`。Gin v1.12 允许同层级静态段与 `:slug` 通配段共存，注册不 panic（已由 `TestMountRegistersCoreRoutes` 与新用例实测）。
- 前端：`packages/api-client/src/api/content.ts` 新增 `postApi.batchStatus`；新增无 Vue 依赖的 `modules/console/web/src/modules/posts/batch-status.ts`（目标集合/计数/禁用原因/弹窗文案/Toast 语义）；`PostList.vue` 仅新增（+53 行、0 删除）浮条按钮与 `handleBatchStatus`，`PostItem` 补 `has_draft`。
- 交互：浮条顺序「批量发布（primary）→ 批量设为草稿（default）→ 批量移入回收站（danger 不变）」；发布 `Dialog.info` + primary 确认，设为草稿 `Dialog.warning` + secondary 确认；禁用态 `title` 区分「未选择文章 / 所选文章均已发布 / 所选文章均已是草稿 / 回收站文章需先恢复再设置状态」；成功后清空选择 → 失效并重取 `queryKeys.posts()` → Toast（`succeeded` 计数，`failed>0` 追加 `，M 篇未处理`）。

### 语义边界（已实现并按此测试）

- 只改 `status` 与 `updated_at`；不应用、不清空 `draft_json`，不改正文/slug/作者/分类/置顶/口令/`publish_at`。存在未发布草稿修改（`has_draft`）的文章在发布确认中明确「本次只切换发布状态，草稿修改保留」。
- 回收站文章不参与：前端不把它计入目标；后端把 `deleted_at != ''` 与不存在 slug 计入 `failed` 且不改动。
- 已是目标状态计入 `skipped` 且不写库（`updated_at` 不变），避免批量操作打乱「最近更新」排序。
- 单次上限 100；非管理员对任一 slug 无署名整单 403（不做部分成功）；`succeeded > 0` 时才发布一次 `content.ContentChanged`。

### 实测（2026-11-02，feature worktree / 基 `ddecd77e`）

- Go：`go test ./internal/store/ -run 'BatchStatus'` 5/5、`go test ./modules/console/api/ -run 'BatchPosts'` 4/4；两目标包全量 `go test` ok；`gofmt` 无差异；`go vet` 与 staticcheck（v0.7.0）通过。
- 前端：`pnpm exec vue-tsc -b` 0 错误；`tsx --test ../tests/post-batch-status.test.ts` 11/11；`eslint` 新增/修改的 4 个前端文件 0 警告。
- 门禁：`make test` 退出 0（Console 240 项测试、Stellar assets 行为测试、部署/来源/模块检查、Go vet、Staticcheck、全 Go 测试全部通过）；`make build` 退出 0（真实 Console + UC 产物与 Go 二进制）。
- 浏览器/E2E（Playwright 1.62.1 + Chromium 151，8096 隔离临时库，`-dev` 由 webServer 重建当前产物）：
  - `pnpm e2e tests/e2e/post-batch-status.spec.ts` 6/6 通过（桌面往返、禁用与目标过滤、回收站禁用、取消保留选择、移动折行与点击、产物指纹）。
  - 回归 `pnpm e2e tests/e2e/console-ui-consistency.spec.ts tests/e2e/console-mobile-a11y.spec.ts` 54/54 通过（38 + 16）。
  - 关键实测值：桌面浮条 479×57.8 单行；移动 390 浮条折行高 149.8、`scrollWidth=390` 无横向溢出、三按钮可点；批量发布响应 `200 {succeeded:2,skipped:0,failed:0}` + 两行「已发布」+ Toast「已发布 2 篇文章」；单篇设为草稿 `200 {succeeded:1}` + 「草稿」+ Toast「已设为草稿 1 篇」；取消确认 0 次 batch 请求且选择/浮条保留。
  - 产物证明：浏览器实际入口 HTML `sha256 f7e474f0…e6a183`、`PostList-DKf4voRC.js`（含「批量发布」）与 `api-A7kJ8ncB.js`（含 `/posts/batch`）的 SHA256 均与磁盘 `modules/console/web/dist` 同名文件一致。
  - 证据：`docs/baselines/post-batch-status/browser-verification.json`（汇总与未覆盖项）、`browser-evidence.json`（原始实测）、`dock-desktop-1280.png`、`dock-mobile-390.png`、`after-published.png`、`after-draft.png`。
- 首次两次 E2E 运行的失败均为验收脚本预期写错（两篇草稿时「批量设为草稿」本就应禁用；前端只提交目标 slug 故后端 `skipped=0`），已确认非产品缺陷并如实记录在证据 JSON 的 `testIterations`。

### 已知边界与未验证项

- 未在浏览器构造 `has_draft` 夹具验证发布确认中「草稿修改保留」那句文案（由 `node:test` 覆盖文案生成）；`failed>0` 的 Toast 后缀未在浏览器验证。
- 未覆盖 768–1279 中间断点、Firefox/WebKit/真机、100 条规模的真实性能。
- 批量状态路径不触发站内/邮件通知与 SEO 推送（与评论批量一致），未做通知验证。
- 风险观察（不阻塞）：390px 下浮条折 3 行、高约 150px，会遮住约两行列表内容（基线允许折行且无横向溢出）；浮条 `sm` 按钮实测高约 43.96px，与既有「批量移入回收站」同规格。

### 状态

实现与验证完成，`docs/baselines/post-batch-status/README.md` 的验收项除「合入 `dev` 后复验」外均已核对。合入本地 `dev` 与合并后复验见本节后续记录。
