# Spec 65 · 插件库侧边栏组件预览修复

- **分支**：`feature/widget-preview-fix`
- **主模块**：`backend/internal/config`、`backend/cmd/server`、`frontend/console/src/modules/plugins`
- **前置**：`specs/58-widget-library/`

## Why

组件库预览只提交 `widgets` 分段时，草稿应用逻辑把未提交的站点分段清零，导致评论、作者、欢迎语与站点信息丢失。预览 iframe 还缺少 `tagcloud.css`、侧栏交互脚本，搜索框提取选择器也与 Spec 59 的 DOM 结构不一致；文库和笔记本组件没有对应的数据载体，因此 tree、wiki_related、tagtree 无输出。

## What Changes

### 后端

- 将 settings 分段解析与赋值拆开：`RebuildSite` 保持全量替换，`ApplyDraftSections` 仅覆盖 raw 中实际提供的段。
- wiki 预览装配排序后首个项目的文档树，notebooks 预览装配首个笔记本的标签树。
- TOC 从预览草稿的 `widgets.library.toc` 读取 `min_depth`、`max_depth` 与编号参数。
- 抽取 wiki 文档树构建逻辑供真实页与预览复用。

### 前端

- tagtree 使用 `notebooks` 载体；侧栏预览移除任意位置的搜索框。
- iframe 注入 `tagcloud.css`、`main.js` 与 `tagcloud.js`，并按组件布局显示无输出说明。
- timeline 空态检测兼容没有 `.grid-box` 的 widget 结构。

## UI 验收边界

参照基线固定为 Spec 58 的四张截图：

- `../58-widget-library/baselines/preview-tagcloud.png`
- `../58-widget-library/baselines/preview-toc-post.png`
- `../58-widget-library/baselines/preview-welcome.png`
- `../58-widget-library/baselines/preview-ghissues.png`

修复后的截图存放在本目录 `baselines/`，只用于与上述固定参照核对：

- `baselines/preview-tagcloud-fixed.png`
- `baselines/preview-toc-post-fixed.png`
- `baselines/preview-welcome-fixed.png`
- `baselines/preview-ghissues-fixed.png`
- `baselines/preview-tree-fixed.png`
- `baselines/preview-wiki-related-fixed.png`
- `baselines/preview-tagtree-fixed.png`

- [x] tagcloud：卡片样式完整，动态/静态模式切换可点
- [x] latest_comment：显示真实评论数据
- [x] author：站长/作者卡片有头像与简介
- [x] welcome：显示站点配置的 Intro 文案兜底
- [x] toc：目录项正常，折叠交互可用
- [x] tree / wiki_related：wiki 载体显示首个项目的文档树/相关项目
- [x] tagtree：notebooks 载体显示标签树
- [ ] ghrepo / ghuser / ghissues：填参数后经 `/api/gh` 代理回填数据（已确认代理请求发出；当前 GitHub 上游返回 403 rate limit）
- [x] recent / popular / siteinfo：数据正常渲染
- [x] 所有预览无搜索框残留
- [ ] 预览区无控制台报错（GitHub 代理上游 403 产生一条 `bad response`）
- [x] 验收截图已保存到 `baselines/`

## UI 巡检记录

- 2026-08-14，使用 feature worktree 的 `8082` 服务与 `.tmp/widget-preview.db` 副本完成 16 个组件巡检。
- wiki 相关项目、文库项目树、笔记本标签树均验证到真实载体数据；`related` 验证到禁用提示。
- GitHub 组件的 `/api/gh` 代理请求已验证，但本次环境的 GitHub API 返回 403 rate limit，未将外部限流误判为本地预览通过。

## ADDED Requirements

### Requirement: 分段草稿应用

The system SHALL preserve live site sections when a preview request provides only a subset of settings sections.

#### Scenario: widgets-only preview

- **WHEN** the preview request provides only the `widgets` section
- **THEN** site basic fields, comments, authors, and style remain unchanged while widgets are replaced by the draft section

### Requirement: Data-backed preview carriers

The system SHALL provide representative wiki and notebook data to their corresponding sidebar widgets in preview rendering.

#### Scenario: first carrier

- **WHEN** the preview carrier is `wiki` or `notebooks`
- **THEN** the first sorted project or notebook supplies tree/tag data to the sidebar widgets

## 验证

1. `go vet ./backend/...` + `go test ./backend/...`
2. `frontend/console` 测试与 `npm run build`
3. dev server `8080` 逐项巡检并保存截图
4. `make merge-check` 全绿后合入 `dev`

## Out of Scope

- 不新增 timeline Fixture 端点。
- 不改变真实前台页面的 TOC 读取路径与数据服务语义。
- 不修改 Spec 58 既有参照截图。
