# Spec 72 · 搜索区与加密文章解锁页样式优化

- **分支**：`dev`（直接提交）
- **主模块**：`frontend/assets/css/search.css`、`frontend/assets/css/post-password.css`、`backend/internal/template/partial/post_password.html`、`backend/internal/template/partial/head.html`
- **前置**：无

## Why

前台两处样式缺陷：

1. 搜索区（Spec 25 服务端搜索）动态渲染的 `.search-result-tabs`（全部/文章/页面/瞬间）与 `.search-result-group-title`（分组标题）全站无任何样式，裸按钮/纯文本渲染；`.search-keyword` 命中高亮为 `#f44336` 红字+红色虚线下划线，深色模式下观感像报错。
2. 加密文章解锁页的「解锁阅读」按钮是 `<button>` 元素，而 main.css 按钮样式仅覆盖 `a.tag-plugin.button`，按钮无背景无色、深色下几乎不可见；表单全内联样式、与站点表单语言脱节；`var(--text-error)` 变量不存在导致错误文案颜色失效；yellow note 深色下暗橄榄偏闷。

约束：`resource/` 主题源不在本 worktree，`main.css` 为编译产物禁止手改（`make css-drift-check` 门禁），新样式走 shim CSS + head.html 挂链。

## What Changes

### 后端（模板）

- `post_password.html`：内联 style 替换为语义 class；按钮去除 `tag-plugin colorful button`（避免 colorful 变量污染）仅保留 id；note 加 `password-note` 类用于深色微调作用域。

### 前端

- 新建 `frontend/assets/css/search.css`：tabs 分段圆角容器、分组标题 meta 样式、`.search-keyword` 主题色高亮覆盖。
- 新建 `frontend/assets/css/post-password.css`：解锁按钮（复刻 `a.button.theme` 视觉）、输入框/label/错误提示、note 深色微调。
- `head.html` 追加两条 link（`asset_ver` 模式）。

## UI 验收边界

> 载体约定见根目录 [`AGENTS.md`](../../AGENTS.md)「UI 类任务开工约定」：
> 参照图与验收基线截图统一存入本 spec 目录的 `baselines/`（入库，勿放 `.qoder/`、`.tmp/` 等被忽略目录）；
> 参照图路径必须是指向入库文件的仓库相对路径。

- **参照基准**：站点现有 Stellar 原生视觉语言（`--theme` 青蓝、`--block`/`--card` 圆角卡片、8–12px 圆角、`0.2s ease-out` 过渡）；现状基线截图：`baselines/before-search.png`、`baselines/before-password.png`（唯一参照，中途不得更换）
- **验收清单**（每项一个可观察判据，完成时逐项勾选）：
  - [x] tabs 有圆角容器底；选中项 card 底+深色字，未选中浅字，hover 有反馈
  - [x] 分组标题「文章」为小号浅色 meta 文字，非裸列表项
  - [x] 命中关键词为主题青蓝高亮，无红色报错感
  - [x] 解锁按钮清晰可见：主题色底、hover 变 accent、提交中 disabled 半透明
  - [x] 密码输入框 focus 显示主题色边框；错误提示为红色可见
  - [x] 深色下 note 不再闷橄榄，标题/正文层次清晰；浅色模式无回归
  - [x] 验收完成后截图存入 `baselines/` 并更新本节勾选状态（spec72-{search,password}-{dark,light}.png）

## ADDED Requirements

### Requirement: 搜索结果类型 tab 与分组标题样式

The system SHALL 为搜索结果的类型 tab 栏与分组标题提供与站点 Stellar 视觉语言一致的样式（分段圆角容器、选中态 card 底、meta 级分组标题）。

#### Scenario: 搜索命中多类型结果

- **WHEN** 用户在搜索框输入关键词且服务端返回结果
- **THEN** tab 栏渲染为圆角分段控件，选中 tab 高亮，分组标题为小号浅色文字

### Requirement: 搜索关键词主题色高亮

The system SHALL 以主题色（`--theme` + `--theme-a20` 底）渲染命中关键词，替代红色报错式高亮。

#### Scenario: 摘要含命中词

- **WHEN** 搜索结果摘要包含命中关键词
- **THEN** 关键词显示为青蓝主题色高亮，无红色虚线下划线

### Requirement: 加密文章解锁表单可用且可见

The system SHALL 在加密文章页渲染清晰可见的解锁按钮（主题色底、hover/disabled 态）与站点语言一致的输入框、错误提示。

#### Scenario: 未解锁访问加密文章

- **WHEN** 未通过口令的用户访问加密文章
- **THEN** 页面显示可读的提示 note、带 focus 主题色边框的密码输入框、清晰可见的解锁按钮；密码错误时红色错误文案可见

## 验证

1. `go vet ./backend/...` + `go test ./backend/...` 通过（含于 `make test`）
2. UI 子任务：「UI 验收边界」清单逐项对照基线截图勾选全绿（深/浅两态截图存 `baselines/`）
3. 直接在 dev 提交：提交前 `make pre-commit-check` 全绿

## Out of Scope

- 不重编 `main.css`（`resource/` 源不在本 worktree）
- 不改动搜索交互逻辑（local-search.js）与解锁 fetch 脚本
- 不将 Apple 设计规范引入前台（本次延续 Stellar 原生语言）
