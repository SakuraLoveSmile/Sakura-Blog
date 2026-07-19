# Spec 10 · 侧边栏 widget 扩展（标签云 + 热门文章 + 站点统计）

- **分支**：`feature/sidebar-widgets` · **Wave 1** · **隔离等级**：中高
- **主模块（独占）**：`internal/template/partial/rightbar.html`（widget 区）+ `fillSidebar`

## Why

侧边栏目前只有欢迎语、作者卡、最近文章/评论。补三个 Stellar 常见 widget：**标签云**（按文章数着色/字号）、**热门文章**（按阅读量）、**站点统计**（文章/标签/评论/总阅读量）。数据都已在库，只需聚合与渲染。

## What Changes

- `internal/template/partial/rightbar.html`（或 `sidebar.html`）：新增三个 `<widget class="widget-wrapper ...">`，对齐 Stellar `widgets/*.ejs` 结构（`widget-header .name` + `widget-body`）。
- `cmd/server/main.go` 的 `fillSidebar`：补聚合——标签云（`store.CountByTag`）、热门文章（新读方法，按 views 降序 limit N）、统计（各 Count + 总阅读量）。填入 `ViewData`。
- `internal/store/`：新增**只读**方法放**新文件** `internal/store/stats.go`：`ListPopular(limit int)`（`ORDER BY views DESC`）、`SumViews()`（`SUM(views)`）、`CountPosts()`。不碰 `post.go`。
- `internal/template/model.go`：`ViewData` +`TagCloud []TagItem`、`Popular []RecentPost`、`SiteStats *SiteStats` 字段；`SiteStats` 定义放**新文件** `internal/template/model_widgets.go`（`TagItem`/`RecentPost` 已存在，复用）。

## 技术决策（无需再问）

- **对齐 Stellar**：widget 外壳类名严格对照 `../resource/hexo-theme-stellar/layout/widgets/`（`tagcloud.ejs`/`hot.ejs` 一类），先 grep `main.css` 有无 `.tag-cloud`/`.tags-group-all` 等类，复用现成，样式用变量。
- **标签云着色**：按标签文章数映射字号/透明度（纯 CSS/内联 style 区间），不引库。
- **热门数**：默认 top 5，limit 常量。
- **降级**：任一查询失败该 widget 渲染空/隐藏，不影响主内容（对齐现有 fillSidebar 容错）。
- **widget 显隐**：可由 `site.yml` 开关控制（可选；默认全开）。

## Impact

- **新增**：`internal/store/stats.go`、`internal/template/model_widgets.go`
- **修改**：`rightbar.html`（+3 widget）、`cmd/server/main.go`（fillSidebar 聚合）、`model.go`（ViewData +3 字段）
- **不影响**：admin、post/comment 渲染、其它页面

## ADDED Requirements

### Requirement: 侧边栏扩展 widget
The system SHALL render tag-cloud, popular-posts, and site-stats widgets in the sidebar from existing data.

#### Scenario: 标签云
- **WHEN** 任意页面加载
- **THEN** 侧栏标签云按各标签文章数呈现不同字号，点击进入 `/tags/:name`

#### Scenario: 热门文章
- **WHEN** 侧栏渲染
- **THEN** 展示阅读量最高的 N 篇，链接到对应文章

#### Scenario: 数据缺失降级
- **WHEN** 某聚合查询失败
- **THEN** 对应 widget 隐藏或空显示，页面其余正常

## 验证

1. `go build ./... && go vet ./...` 通过
2. 起服务，侧栏三 widget 数据正确（与库内计数一致）
3. 标签云字号分级可见、点击跳转正确；热门按 views 排序
4. 亮/暗模式样式对齐 Stellar；PJAX 换页 widget 正常

## Out of Scope

- 标签云动画/3D 球
- 实时统计（每请求聚合即可，数据量小）
