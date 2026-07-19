# Spec 02 · 友链页面（Friends / Links）

- **分支**：`feature/friends-page`
- **主模块**：前端模板 + 配置（`config`）
- **隔离等级**：高（不碰 store、admin、post 渲染管线）

## Why

博客缺少「友情链接」页。这是个人博客标配，且 Stellar 主题原生有 `友链` 布局。本功能**纯配置驱动**（友链数据写在 `site.yml`，不入库），实现成本低、与其它功能零数据耦合，适合并行开发。

## What Changes

- `internal/config/site.go`：`Site` 结构体（对应 `internal/template/model.go` 的 `tmpl.Site`）新增 `Links []LinkGroup` 字段与 `DefaultSite()` 默认值；新增 `LinkGroup{Title string; Items []LinkItem}`、`LinkItem{Name, URL, Avatar, Desc string}`。
  - ⚠️ 注意 `tmpl.Site` 定义在 `internal/template/model.go`。**为避免与其它 spec 抢 model.go**，友链视图模型新建独立文件 `internal/template/model_links.go`（同 package `template`），在其中定义 `LinkGroup`/`LinkItem`，`Site` 结构体加字段这一处改动不可避免（model.go），改动仅 +1 行字段声明，冲突面极小。
- `config/site.yml`：新增 `links:` 段（分组 + 条目示例）。
- 新增 `internal/template/partial/links.html`：友链页正文 partial，**严格对齐** `../resource/hexo-theme-stellar/layout/` 下友链相关布局（`links.ejs` / `flink` widget），复用主题现成卡片类。
- `internal/template/layout/base.html`：`Layout == "links"` 分支派发到 `{{template "partial/links" .}}`。
- `cmd/server/main.go`：新增 `r.GET("/links", ...)`（或 `/friends`），构造 `Page{Layout:"links", MenuID:"links", Title:"友链"}` + `fillSidebar` + `fillSEO`，渲染 `layout/base`。
- `config/site.yml` 的 `menubar.items` 增加一项「友链」（`url: /links`、`icon` 用 icons.yml 已存在的 key），使侧边栏可达。

## 技术决策（agent 无需再询问）

- **数据源**：`site.yml` 配置驱动，**不建数据库表**、**不做 admin 管理页**（保持隔离与简单）。
- **路由**：`/links`（GET）。菜单 `MenuID = "links"`，与现有 post/archive 不冲突。
- **模板基名**：路径化命名 `partial/links`，不与现有 `partial/*` 冲突。
- **样式铁律**：**先 grep `assets/css/main.css`** 确认友链卡片类（Stellar 通常是 `.flink` / `.flink-list` / `.site-card` 一类）是否已在编译产物中。**只用已存在的类，禁止自创样式**；若原主题类不在 main.css，则对照 `resource/hexo-theme-stellar` 的 `_data/widgets.yml` 与 flink 布局确认类名，用最接近的现成类，不新写 CSS。
- **头像取值**：`LinkItem.Avatar` 为图片 URL，缺省时用主题默认占位。
- **YAML 引号坑**：`site.yml` 中含 `#`（颜色）或 `:`（URL）的值按 `CLAUDE.md` 规范加引号。

## Impact

- **新增**：`internal/template/partial/links.html`、`internal/template/model_links.go`
- **修改**：`internal/config/site.go`（+Links 字段与默认值）、`internal/template/model.go`（Site 结构体 +1 行 Links 字段）、`config/site.yml`（+links 段、menubar +1 项）、`internal/template/layout/base.html`（+1 派发分支）、`cmd/server/main.go`（+1 路由）
- **不影响**：store、admin、post/comment 渲染、其它页面

## 与其他 spec 的冲突点

- `base.html`：本 spec 是唯一修改者（Spec 01/03/04 均不碰）。
- `site.go` / `site.yml`：本 spec 是唯一修改者。
- `main.go`：与 03/04 共享，但各自在不同路由区段追加，冲突面为单行。
- `model.go`：仅 +1 行字段；其它 spec 的新模型都走独立文件，不碰 model.go。

## ADDED Requirements

### Requirement: 友链页面

The system SHALL provide a `/links` page that renders friend links grouped by category, driven by `site.yml`.

#### Scenario: 正常访问
- **WHEN** 用户访问 `/links`
- **THEN** 返回 200，按 `links` 配置的分组与条目渲染友链卡片（名称/头像/描述/跳转链接）

#### Scenario: 空配置
- **WHEN** `site.yml` 未配置 `links` 或为空
- **THEN** 页面仍返回 200，主内容区显示空提示（如"暂无友链"），不 panic

#### Scenario: 菜单高亮
- **WHEN** 用户位于 `/links`
- **THEN** 侧边栏"友链"菜单项呈 `active` 态

## 验证

1. `go build ./... && go vet ./...` 通过
2. 起服务后 `curl -s -o /dev/null -w "%{http_code}" http://localhost:<port>/links` 返回 200
3. 浏览器打开 `/links`：卡片布局与 Stellar 原版观感一致，亮/暗模式正常
4. 点击友链条目在新标签打开目标站（`target="_blank" rel="noopener"`）
5. 侧边栏「友链」菜单高亮，PJAX 导航到该页无控制台报错

## Out of Scope

- 友链后台管理 / 入库
- 友链申请表单、友链检测（死链探测）
- 友链朋友圈（聚合友站 RSS）
