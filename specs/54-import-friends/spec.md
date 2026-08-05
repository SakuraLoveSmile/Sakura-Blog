# Spec 54 · 从 GitHub Friends 仓库导入友链

- **分支**：`feature/import-friends` · **Wave 12**
- **主模块**：`internal/api/import_links.go`（新）+ `internal/github/client.go`（+GetIssues）+ `console/src/modules/friends/FriendList.vue`
- **前置**：Spec 02（友链页）/ Demo 3（links 表 + console 友链管理）已合入 dev

## Why

友链交换通过 [sakurasep/Friends](https://github.com/sakurasep/Friends) 仓库的 issue 提交（body 内置 `### 友链信息` JSON 块）。目前拿到申请后需手工逐条在 console「链接」页录入，重复劳动且易漏字段（rss/avatar）。本功能让站长在 console 一键拉取该仓库 open issues 并批量写入 `links` 表，自动去重。

数据源快照已存档 `resource/friends/github-issues-raw-2026-08-05.json`（容器根目录，gitignore，仅供参考）。

## What Changes

### 后端

- `internal/github/client.go`：新增 `Issue` 结构（number/title/state/body/labels）+ `GetIssues(owner, repo, query)`——复用既有 `GetIssuesRaw` + 缓存（TTL 30min），解析裁剪字段。
- `internal/api/import_links.go`（新）：`importLinks` handler + 纯函数 `extractJSONBlock(body)` / `parseFriendIssue(issue)`。
- `internal/api/router.go`：Handler 增 `gh *ghpkg.Client` 字段（Mount 时从 `MountDeps.GH` 注入）；authed 组挂 `POST /links/import`。

### 前端

- `console/src/api/index.ts`：`linkApi.importFromGitHub(data)`。
- `console/src/modules/friends/FriendList.vue`：工具条「从 GitHub 导入」按钮 → 弹窗（分组名，默认「GitHub 友链」）→ 调后端 → Toast 展示 `导入 N 条 · 跳过 M 条 · 失败 K 条` → 刷新列表。

## 技术决策（无需再问）

- **数据源**：固定 `sakurasep/Friends` 的 open issues（`state=open&per_page=100`），不支持自定义仓库（简单优先）。
- **复用 GitHub 代理客户端**：不走匿名直连，用 `github.Client`（已支持 `GITHUB_TOKEN` 环境变量提限流、磁盘缓存、测试可注入 apiBase）。`MountDeps.GH` 为 nil 时导入端点返回 503。
- **字段映射**：issue body 的 ```json 块：`title→name`、`url→url`、`avatar|icon→avatar`、`description→description`、`feed|rss→rss`；`screenshot/snapshot/posts` 忽略（头像用 avatar，posts 由聚合器自行抓 RSS）。issue 首个 label（如「失联」）→ `label`，color 留空。
- **分组**：请求体 `group` 字段（默认「GitHub 友链」），单分组导入。
- **去重**：按 `strings.TrimRight(url, "/")` 与既有 links 比对，已存在跳过；JSON 块缺失/解析失败/无 url 计入 `failed` 并附原因，不中断整体。
- **成功回调**：批量写入后调用 `afterLinksChanged()`（站点热重建 + 聚合器即时刷新），与 create/update/delete 一致。

## Impact

- **新增**：`internal/api/import_links.go`、`internal/api/import_links_test.go`
- **修改**：`internal/github/client.go`（+GetIssues/Issue）、`internal/api/router.go`（+1 字段 +1 路由）、`console/src/api/index.ts`（+1 方法）、`FriendList.vue`（+导入弹窗与按钮）
- **不影响**：前台渲染、聚合器、其它 API、其它 console 模块

## ADDED Requirements

### Requirement: 从 GitHub 导入友链

The system SHALL provide an authenticated `POST /api/v1/links/import` endpoint that fetches open issues from `sakurasep/Friends`, parses friend-link JSON blocks, and inserts new links (deduplicated by URL) under a configurable group.

#### Scenario: 正常导入
- **WHEN** 仓库存在可解析的友链 issue 且部分 URL 已存在于 links 表
- **THEN** 返回 200 `{imported, skipped, failed[]}`；新 URL 入库、已存在跳过、坏数据附原因

#### Scenario: 分组自定义
- **WHEN** 请求体带 `group`
- **THEN** 导入条目的 `group_title` 取该值（默认「GitHub 友链」）

#### Scenario: GitHub 代理未启用
- **WHEN** `MountDeps.GH` 为 nil
- **THEN** 返回 503 与明确错误文案

#### Scenario: 未认证
- **WHEN** 无管理员 cookie 调用该端点
- **THEN** 返回 401

## 验证

1. `go vet ./backend/...` + `go test ./backend/...` 通过（含解析纯函数单测与 httptest 集成用例）
2. `vue-tsc -b` + console 行为测试通过
3. 起服务后 console「链接」页点「从 GitHub 导入」：结果 Toast 正确、列表出现新分组与条目、RSS/头像字段正确落库
4. 再次导入：全部跳过（去重生效），不产生重复行
5. `make merge-check` 全绿后合入 dev

## Out of Scope

- 自定义仓库 / label 筛选 / 多分组映射
- 导入数据预览与逐条勾选
- issue 评论同步、申请状态回写（关闭已导入 issue）
