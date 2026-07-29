# Spec 33 · 侧边栏 widgets 配置化编排 + 缺失 widget 补齐

- **分支**：`feature/widgets-config` · **Wave 8** · **隔离等级**：中（动模板骨架，需独占 sidebar/rightbar）
- **主模块**：`internal/template/partial/sidebar.html`、`rightbar.html` → 拆分为 `partial/widgets/*.html` + `config/site.go`（widgets 段）+ `cmd/server/web.go`（fillSidebar）
- **前置**：无；**是 Spec 35（wiki 体系 tree/tagtree widget）的前置**

## Why

这是前端与原版**最大的架构差距**：原版通过 `_data/widgets.yml` 定义组件库、`site_tree` 按页面类型配置左右栏组件序列；本项目左右栏内容硬编码在 sidebar.html/rightbar.html。同时缺 4 个 widget：**ghrepo / ghuser / ghissues**（`internal/github/` 代理与 `services/ghinfo.js` 已带，只差 widget 形态）与**通用 timeline**（任意 api 驱动）。另有 toc/tagcloud 的参数化不齐。

## 功能需求

1. **widgets 配置段**（site.yml 新增，形状对照原版 `_data/widgets.yml`）：
   ```yaml
   widgets:
     library:            # 组件库：name → {layout, ...参数}
       recent: {layout: recent, limit: 10}
       toc: {layout: toc, list_number: false, min_depth: 1, max_depth: 6, collapse: auto}
       ghuser: {layout: ghuser, username: xxx, avatar: true}
     pages:              # 页面类型 → 左右栏组件序列
       post: {leftbar: [recent, related], rightbar: [welcome, author, toc]}
       index: {leftbar: [welcome, recent], rightbar: [tagcloud, siteinfo]}
   ```
   缺省值 = 当前硬编码布局（**零配置时渲染结果与现状逐像素一致**，这是回归底线）。
2. **widget 模板拆分**：sidebar.html/rightbar.html 内各段落拆为 `partial/widgets/<name>.html`（recent/tagcloud/toc/related/markdown/author/timeline/latest_comment/siteinfo/popular…），骨架按配置序列循环 include。
3. **新 widget**：ghrepo（仓库卡）、ghuser（用户卡）、ghissues（issues 列表，limit/labels 参数）——数据走既有 `/api/gh/*` 代理；通用 timeline（api/user/limit 参数，复用 `services/timeline.js`）。
4. **参数补齐**：toc 的 list_number/min_depth/max_depth/collapse=auto/edit_this_page；tagcloud 的颜色渐变/show_count/orderby（对照原版 widget EJS）。

## What Changes

- `internal/template/`：+`partial/widgets/` 目录（每 widget 一个模板，基名加 `widgets/` 前缀防撞）；sidebar.html/rightbar.html 改为按 `ViewData.Widgets` 序列分派。
- `internal/template/model.go`：+WidgetInstance 视图模型（Name/Layout/Params/Data）。
- `config/site.go` + `site.yml`：+widgets 段解析与缺省注入（缺省逻辑单测覆盖）。
- `cmd/server/web.go` fillSidebar：改为按页面类型解析组件序列并逐个装配数据（现有 45s 快照缓存机制保留，缓存 key 引入页面类型维度）。
- `partial/head.html`：ghissues/timeline 有配置时按需注入 js。

## 实现步骤

1. 精读原版 `_data/widgets.yml` + `layout/_partial/sidebar/index_leftbar.ejs` + 13 个 widget EJS，产出「组件 → 参数 → 数据源」对照表。
2. 先做纯拆分重构：模板拆到 widgets/ 目录、硬编码序列不变、目检像素回归（本步单独提交，作为安全点）。
3. 引入 widgets 配置段 + 缺省注入 + 单测（配置为空 = 现状布局）。
4. fillSidebar 改造为序列驱动装配。
5. 新增 ghrepo/ghuser/ghissues/timeline 四个 widget。
6. toc/tagcloud 参数补齐。

## 技术决策（无需再问）

- **配置入 site.yml 而非独立 widgets.yml**：本项目配置单文件惯例（site.yml 已承载 menu/wiki/links），Console 设置页后续可视化编辑预留（本 spec 不做 Console UI）。
- **markdown widget 通用化**：原版 markdown widget 支持任意 md 文本，本期实现为 library 中可配 `content` 字段的通用组件，现有"欢迎语"迁移为其实例。
- **热门文章/站点统计保留**：本项目自创的两个 widget 注册进 library（layout: popular / siteinfo），不删除。
- **页面类型枚举**：index/post/page/archive/categories/tags/topic/wiki/friends/notes/moments，未配置的类型回退 index 布局。

## Impact

- **新增**：`partial/widgets/*.html`（~15 个）、widgets 配置段
- **修改**：sidebar.html、rightbar.html（重构为骨架）、model.go、site.go/site.yml、web.go（fillSidebar）、head.html
- **不影响**：文章渲染管线、api、console（本期不做设置 UI）

## 验证

- [ ] **零配置像素回归**：不写 widgets 段时所有页面左右栏与改造前目检一致
- [ ] site.yml 调整组件顺序/增删组件后各页面生效
- [ ] ghrepo/ghuser/ghissues 卡片渲染（走本地 /api/gh 代理，断网降级不破版）
- [ ] toc 参数（编号/深度/collapse）与 tagcloud 参数（渐变/计数/排序）逐项生效
- [ ] fillSidebar 快照缓存仍生效（45s 内重复请求不重查库）
- [ ] `make test` 全绿（widgets 缺省注入 + 视图模型单测）
