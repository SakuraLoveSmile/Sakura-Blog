# Spec 39 · Console 全局搜索接入全文检索

- **分支**：`feature/console-search` · **Wave 8** · **隔离等级**：高（纯 Console 前端）
- **主模块**：`console/src/layouts/DefaultLayout.vue`（⌘K 弹窗改造）
- **前置**：无（后端 FTS API 已就绪，Spec 25 产物）

## Why

Halo 对照差距中的最低成本项：Console 的 ⌘K 全局搜索目前**只搜侧栏菜单项**，而后端 `/api/search`（SQLite FTS5，`cmd/server/routes_search.go`）已存在。把内容搜索接进弹窗即达到 Halo「全局搜索文章/页面」的体验。

## 功能需求

1. ⌘K 弹窗分两区：**菜单跳转**（现状保留）+ **内容结果**（输入 ≥2 字符时防抖 300ms 查询）。
2. 内容结果按类型分组显示：文章 / 页面（图标区分），显示标题 + 高亮片段（FTS snippet），↑↓ 键统一在两区间导航，回车跳转对应 Console 编辑页（`/posts/:id/edit` 等，**不是**前台 URL）。
3. 空结果显示「无匹配内容」；FTS 不可用（接口 5xx）时静默降级为仅菜单搜索。
4. 草稿与回收站内容也可被搜到并标注状态徽标（见技术决策）。

## What Changes

- `console/src/layouts/DefaultLayout.vue`：搜索弹窗组件扩展（结果分组/键盘导航/防抖）。
- `console/src/api/index.ts`：+searchApi（消费管理端搜索端点）。
- **后端小改**：`/api/search` 是前台公开端点（只搜 published、不含草稿）；Console 需要管理端搜索 → `internal/api/` +`search.go`（`GET /api/v1/search?q=`，走同一 FTS 索引但不过滤状态，返回 id/type/title/status/snippet）。router.go 挂载 +1 行。

## 实现步骤

1. 后端 `/api/v1/search` 端点（复用 `internal/search` 查询，去掉 published 过滤，+status 字段）+ `_test.go`。
2. api client + 弹窗改造（分组渲染/键盘导航）。
3. 降级路径与空态。

## 技术决策（无需再问）

- **管理端独立端点**：不改公开 `/api/search` 的过滤语义（避免密码文章正文泄漏风险扩散），管理端点在鉴权中间件之后。
- **搜索范围本期 = 文章 + 页面**：瞬间/评论/附件不进 FTS 索引（索引结构不动）；后续有需求另立 spec。
- **跳转目标**：文章 → 编辑器；页面 → 页面编辑器；回收站项 → 对应列表页并带筛选参数。

## Impact

- **新增**：`internal/api/search.go`（+test）、Console searchApi
- **修改**：DefaultLayout.vue、api/index.ts、router.go（+1 行）
- **不影响**：前台搜索、FTS 索引结构

## 验证

- [ ] ⌘K 输入关键词出现文章/页面分组结果，含草稿（带徽标）
- [ ] 键盘 ↑↓/回车全链路可用，回车进编辑器
- [ ] 未登录调 /api/v1/search 返回 401
- [ ] FTS 端点异常时弹窗退回菜单搜索不报错
- [ ] `make test` 全绿
