# Spec 56 · 通用友链导入 + 检测自动失效标签

- **分支**：`feature/generic-link-import` · **Wave 13**
- **主模块**：`internal/api/import_links.go`（来源泛化）+ `internal/api/friends.go`（检测打标）+ `console/src/modules/friends/FriendList.vue`
- **前置**：Spec 54（从 GitHub 导入友链）已合入 dev

## Why

Spec 54 的「从 GitHub 导入」只覆盖 Friends 仓库 issue 一种来源，弹窗与按钮语义写死 GitHub；站长从其他渠道（复制的 JSON/YAML 列表）拿到友链数据时仍需手工逐条录入。另外「检测全部」的结果只停留在前端内存，失联友链没有任何持久标记，刷新页面后状态丢失。

本 Spec 将导入入口通用化（GitHub 仓库为其中一种来源，新增粘贴文本来源），并让检测自动为失联友链打「失效」标签、恢复后自动摘除。

## What Changes

### 后端

- `internal/api/import_links.go`：`POST /links/import` 请求体扩展 `{source, group, content}`；
  - `source=github`（默认，兼容 Spec 54）：逻辑不变，group 默认「GitHub 友链」。
  - `source=text`：新增纯函数 `parseTextLinks(content)`——先 JSON 数组、后 YAML（goccy/go-yaml）解析，均支持 `links` 顶层键；字段别名 `name|title`、`avatar|icon`、`rss|feed`，透传 `label`/`color`（去 `#`）/`sort`；group 默认「导入友链」。
  - 共用写入逻辑抽为 `writeImportedLinks`（URL 去重 + 计数 + 失败原因）。
  - 未知 source / 无法解析 / 空内容 → 400。
- `internal/api/friends.go`：`checkLinks` 探测完成后调用 `applyExpiredLabels`——
  - 不可达且无标签 → 打「失效」标签（颜色 `EF4444`），计数 `tagged`；
  - 可达且标签恰为「失效」 → 清空标签与颜色，计数 `restored`；
  - 已有其他标签 → 不覆盖。
  - 有变更时触发 `afterLinksChanged`；响应扩展为 `{items, tagged, restored}`。

### 前端

- `console/src/api/index.ts`：`linkApi.importFromGitHub` → `linkApi.importLinks({source, group?, content?})`；`check` 注释同步。
- `console/src/modules/friends/FriendList.vue`：
  - 工具条按钮「从 GitHub 导入」→「导入友链」（`ri:import-line`）。
  - 弹窗标题「导入友链」，顶部两段式来源切换（GitHub 仓库 / 粘贴文本）；粘贴来源显示多行 textarea（JSON 数组或 YAML）；切换来源时分组默认值跟随（GitHub 友链 / 导入友链）。
  - `runCheck` Toast 追加「已标记失效 X · 恢复 Y」（有变更时），并刷新列表展示新标签。

## ADDED Requirements

### Requirement: 粘贴文本导入友链

The system SHALL accept `POST /api/v1/links/import` with `source=text` and a JSON-array or YAML-list `content`, deduplicate by URL, and insert links under a configurable group.

#### Scenario: JSON 数组导入
- **WHEN** `content` 为含 name/url 等字段的 JSON 数组且部分 URL 已存在
- **THEN** 返回 200 `{imported, skipped, failed[]}`；缺 url/name 条目记入 failed（「第 N 条: 原因」）

#### Scenario: YAML 列表导入
- **WHEN** `content` 为 YAML 列表或含 `links:` 顶层键
- **THEN** 与 JSON 数组等价处理

#### Scenario: 非法内容
- **WHEN** `content` 为空、非列表结构或整体无法解析
- **THEN** 返回 400 与明确错误文案

#### Scenario: 未知来源
- **WHEN** `source` 不是 `github`/`text`
- **THEN** 返回 400

### Requirement: 检测自动维护「失效」标签

The system SHALL tag unreachable links with label「失效」(color EF4444) after `POST /api/v1/links/check`, remove the tag when a tagged link recovers, and never overwrite other existing labels.

#### Scenario: 失败打标
- **WHEN** 友链不可达且标签为空
- **THEN** 标签置「失效」、颜色 `EF4444` 并落库，响应 `tagged` 计数 +1

#### Scenario: 恢复摘标
- **WHEN** 标签恰为「失效」的友链恢复可达
- **THEN** 清空标签与颜色并落库，响应 `restored` 计数 +1

#### Scenario: 已有标签不覆盖
- **WHEN** 友链已有其他标签（如「大佬」）且不可达
- **THEN** 标签保持原值不变

#### Scenario: 幂等
- **WHEN** 连续两次检测且结果不变
- **THEN** 第二次 `tagged` 与 `restored` 均为 0

## MODIFIED Requirements

### Requirement: 从 GitHub 导入友链（Spec 54）

端点路径与响应结构不变；请求体新增可选 `source` 字段，缺省视为 `github`，Spec 54 全部 Scenario 保持成立。console 入口由「从 GitHub 导入」改为通用「导入友链」弹窗，GitHub 仓库为其中一种来源。

## UI 验收边界（导入弹窗 UI 子任务）

> 载体约定见根目录 [`AGENTS.md`](../../AGENTS.md)「UI 类任务开工约定」；基线截图存放于本目录 `baselines/`（入库）。

- **参照图路径**：`baselines/import-modal-reference.png`（粘贴文本来源弹窗基线，唯一参照，中途不得更换；GitHub 来源对照图为 `baselines/import-modal-github-source.png`）
- **验收清单**（每项一个可观察判据，完成时逐项对照基线截图勾选）：
  - [ ] 「链接」页工具条按钮文案为「导入友链」（图标 `ri:import-line`），不再出现「从 GitHub 导入」
  - [ ] 弹窗标题为「导入友链」，顶部为两段式来源切换（GitHub 仓库 / 粘贴文本）
  - [ ] 选择粘贴来源时显示多行 textarea，提示文案说明支持 JSON 数组或 YAML
  - [ ] 切换来源时分组默认值跟随（GitHub 友链 / 导入友链）
  - [ ] 「检测全部」后有变更时 Toast 追加「已标记失效 X · 恢复 Y」，列表中「失效」标签为红色（`EF4444`）
  - [ ] 验收完成后的截图存入 `baselines/` 并更新本节勾选状态

## 验证

1. `go vet ./backend/...` + `go test ./backend/...` 通过（parseTextLinks 纯函数单测 + text 导入/检测打标 httptest 集成用例）
2. `vue-tsc -b` + console 行为测试通过
3. console「链接」页：两种来源导入各跑一次，Toast/分组正确，重复导入全部跳过
4. 「检测全部」：失联友链出现红色「失效」标签，恢复后摘除，已有标签不被覆盖
5. UI 子任务：「UI 验收边界」清单逐项对照 `baselines/` 基线截图勾选全绿
6. `make merge-check` 全绿后合入 dev

## Out of Scope

- 文件上传导入（OPML/CSV 等）与逐条预览/勾选
- 失效标签颜色/文案自定义
- 定时自动检测（仍为手动触发）
- GitHub 来源支持自定义仓库 / label 筛选（沿用 Spec 54 边界）
