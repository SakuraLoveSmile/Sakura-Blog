# Spec 58 · 组件库新增侧边栏组件页（方案保存与套用）

- **分支**：`feature/widget-library` · **Wave 1**
- **主模块**：`frontend/console/src/modules/plugins/PluginLibrary.vue`、`frontend/console/src/modules/plugins/WidgetLibraryView.vue`、`frontend/console/src/modules/settings/widget-meta.ts`、`frontend/console/src/modules/settings/WidgetsTab.vue`
- **前置**：Spec 33（widgets 配置化编排）、Spec 50（linklist）

## Why

侧边栏组件（recent / tagcloud / toc 等 16 种内置组件）的参数目前只能在 设置 → 外观 → 组件编排 里以行内小表单或 JSON 兜底方式编辑，且组件名即配置本身，无法把「同一个组件的不同参数组合」沉淀为可复用的命名方案（如「彩色标签云」= tagcloud + color:true + amount:30）。需要一个专门的组件库视图来浏览组件、编辑参数并保存方案，保存后的方案在设置页组件编排中直接可选用。

## What Changes

### 后端

- 零改动。方案以命名条目写入既有 `widgets.library`（name → layout + 参数），后端 Spec 33 的 `Lookup` 已支持任意命名的 library 条目。

### 前端

- 新建 `frontend/console/src/modules/settings/widget-meta.ts`：16 种内置组件的共享元数据（name/label/layout/desc/字段 schema），来源对齐后端 `DefaultWidgetLibrary` 与 `site.example.yml` widgets 段；WidgetsTab 原有 `BUILTIN_WIDGETS`/`knownFields`/`noExtraParams` 迁移为消费该模块。
- `PluginLibrary.vue` 视图切换扩展为「标签插件 / 素材速查 / 侧边栏组件」三段。
- 新建 `WidgetLibraryView.vue`：左侧列表（内置组件 + 已保存方案两组），右侧详情（说明、参数表单、JSON 兜底、渲染预览面板）；支持「保存为方案 / 保存 / 另存为 / 删除」（删除前检查 `widgets.pages` 引用并警告）。数据经 `settingsApi`（GET /settings、PUT /settings/widgets 整段写回）；预览复用设置页草稿预览管线（POST /preview/site，草稿 widgets 注入保留名 `__preview__`，提取 widget 片段塞入 iframe，草稿参数防抖 500ms 重渲染）。
- `WidgetsTab.vue`：左右栏组件行与添加组件下拉对非内置名条目显示「方案」徽标；页面类型选择器扩展为分组（内容/文库/笔记本/互动/作者），补齐 wiki/wiki-doc/notebooks/notebook/note/friends/moments/topic 等后端已支持但 UI 未暴露的类型。
- `frontend/assets/js/services/timeline.js`：`new URL(api, location.origin)` 抛错时降级取查询串（srcdoc 嵌入预览 origin 为 "null" 的场景），`wrapLazyloadImages` 加可选链；`base.html` 给 ghinfo/timeline service 脚本加版本号（immutable 缓存下可更新）。预览 iframe 保留 jquery + utils + 数据服务 scan 脚本，GitHub 三卡走本地 /api/gh 代理实时取数。

## UI 验收边界

- **参照图路径**：`baselines/plugin-library-baseline.png`（唯一参照：插件库页现有「标签插件」视图基线截图，新视图的版式/配色/控件风格与之对齐）
- **验收清单**：
  - [x] 插件库页出现「侧边栏组件」分段按钮，切换后展示左列表 + 右详情布局，风格与「标签插件」视图一致
  - [x] 左列表分「内置组件（16）」与「已保存方案」两组，方案项显示 layout 徽标
  - [x] 选中 tagcloud 修改 amount=30 并输入方案名保存后，设置存储（settings 段 widgets.library，经 config reload 合入前台渲染链路）出现该条目
  - [x] 设置 → 外观 → 组件编排：添加组件下拉出现该方案且带「方案」徽标，加入序列保存后前台生效
  - [x] 方案的编辑 / 另存为 / 删除（含被 pages 引用时的警告）路径正常
  - [x] 渲染预览面板：tagcloud 草稿参数（amount 20→3）联动 iframe 内真实组件输出；toc 在 post 载体渲染出目录项
  - [x] GitHub 三卡预览取数：ghrepo stars/forks、ghuser followers、ghissues timeline 均经 /api/gh 代理回填
  - [x] 组件编排页面类型下拉含文库/笔记本/友链/瞬间等分组；友链页自定义序列保存后前台 /friends 按序渲染
  - [x] 验收完成后截图存入 `baselines/` 并更新本节勾选状态（含 preview-tagcloud.png / preview-toc-post.png）

## ADDED Requirements

### Requirement: 侧边栏组件方案保存

The system SHALL 在插件库页提供「侧边栏组件」视图，允许用户编辑内置组件参数并以命名方案写入 `widgets.library`。

#### Scenario: 保存新方案
- **WHEN** 用户选中内置组件（如 tagcloud），修改参数并输入不与内置名/已有方案重名的方案名后保存
- **THEN** 设置存储的 `widgets.library` 新增 `{方案名: {layout, ...参数}}` 条目，前台渲染链路可直接引用

#### Scenario: 方案名冲突
- **WHEN** 方案名为空、与内置组件名重复或与已有方案重名
- **THEN** 阻止保存并给出 Toast 提示

### Requirement: 设置页套用方案

The system SHALL 在设置页组件编排的添加组件下拉与左右栏序列中呈现已保存方案。

#### Scenario: 添加组件下拉包含方案
- **WHEN** 用户在组件编排中打开「选择组件」下拉
- **THEN** 已保存方案与内置组件一并列出，且方案带「方案」徽标/前缀标识

### Requirement: 方案管理

The system SHALL 支持对已保存方案的编辑、另存为与删除。

#### Scenario: 删除被引用的方案
- **WHEN** 用户删除一个出现在 `widgets.pages` 左右栏序列中的方案
- **THEN** 确认弹窗提示该方案正被页面类型引用，确认后删除条目

### Requirement: 组件渲染预览

The system SHALL 在侧边栏组件视图提供草稿参数的实时渲染预览，复用前台真实渲染管线。

#### Scenario: 参数联动预览
- **WHEN** 用户修改草稿参数（如 tagcloud amount）
- **THEN** 防抖 500ms 后 iframe 内组件输出随参数变化（标签数量同步）

#### Scenario: 页面载体匹配
- **WHEN** 选中 toc/author/related 等依赖文章上下文的组件
- **THEN** 预览以 post 页为载体渲染（tree/wiki_related 用 wiki 载体，其余 index）

## 验证

1. `go vet ./backend/...` + `go test ./backend/...` 通过（后端零改动，回归确认）
2. UI 子任务：「UI 验收边界」清单逐项对照基线截图勾选全绿
3. `make merge-check` 全绿后合入 dev

## Out of Scope

- 整栏布局方案（左右栏序列级预设）
- PageEditor 页面级侧栏下拉接入方案列表
