# Spec NN · <标题>

- **分支**：`feature/<slug>` · **Wave N**
- **主模块**：<独占文件/目录清单>
- **前置**：<依赖的已合 spec，无则写「无」>

## Why

<问题背景与立项理由>

## What Changes

### 后端

- <改动点>

### 前端

- <改动点>

## UI 验收边界（含 UI/样式子任务必填，纯后端可删除本节）

> 载体约定见根目录 [`AGENTS.md`](../../AGENTS.md)「UI 类任务开工约定」：
> 参照图与验收基线截图统一存入本 spec 目录的 `baselines/`（入库，勿放 `.qoder/`、`.tmp/` 等被忽略目录）；
> 参照图路径必须是指向入库文件的仓库相对路径。

- **参照图路径**：`baselines/<唯一参照图文件名>.png`（唯一参照：设计稿 / 原版 Stellar 截图 / 基线截图，中途不得更换）
- **验收清单**（每项一个可观察判据，完成时逐项勾选）：
  - [ ] <可观察判据 1，例："导入弹窗标题为「导入友链」，顶部为两段式来源切换">
  - [ ] <可观察判据 2>
  - [ ] <可观察判据 N>
  - [ ] 验收完成后截图存入 `baselines/` 并更新本节勾选状态

## ADDED Requirements

### Requirement: <需求名>

The system SHALL <行为描述>.

#### Scenario: <场景名>
- **WHEN** <条件>
- **THEN** <结果>

## MODIFIED Requirements

<如无则删除本节>

## 验证

1. `go vet ./backend/...` + `go test ./backend/...` 通过
2. UI 子任务：「UI 验收边界」清单逐项对照基线截图勾选全绿
3. `make merge-check` 全绿后合入 dev

## Out of Scope

- <明确不做项>
