# Spec 35 · Wiki 文档体系 + 笔记本体系（大工程）

- **分支**：`feature/wiki-notes` · **Wave 9**（依赖 Wave 8 的 33 先合）· **隔离等级**：低（跨 store/路由/模板/Console 多层）
- **主模块**：新 `internal/store/wikidoc.go` + `cmd/server/routes_taxonomy.go`（wiki/notes 段重写）+ 新 partial 模板 + `partial/widgets/tree.html`、`tagtree.html` + Console 文库管理
- **前置**：**Spec 33（widgets 编排）必须先合**——tree/tagtree 是 widget 形态，依赖组件编排机制

## Why

原版 Stellar 的两大内容体系本项目只有壳：(1) **wiki**——原版有完整文库（项目 → 树形文档页，leftbar 挂 tree 页面树、上下篇导航、wiki 维度面包屑），本项目 /wiki 只是配置驱动的卡片跳转；(2) **笔记本**——原版是 notebooks（笔记本列表）→ notes（笔记列表分页）→ note（笔记页，tagtree 导航）三层体系，本项目 /notes 是单页手记。这是页面类型维度最大的缺失，也是把本项目从「博客」升级为「博客 + 知识库」的关键。

## 功能需求

### A. Wiki 文库
1. **数据模型**：wiki 项目（已有 site.yml wiki 段：id/title/description/cover 等）+ 文档页（新增：project_id、slug、title、order、parent、content_md/html、updated）。
2. **前台路由**：`/wiki`（项目列表，现状保留）、`/wiki/:project`（项目首页 = 第一篇文档）、`/wiki/:project/:slug`（文档页）。
3. **文档页布局**：leftbar 挂 **tree widget**（项目内页面树，当前页高亮、支持层级），文档底部上一篇/下一篇，面包屑用 wiki 维度（首页 / 文库 / 项目 / 文档，对照原版 `_partial/main/navbar/breadcrumb/` wiki 分支）。
4. **Console 管理**：文库模块——项目下文档 CRUD、排序（拖拽或 order 字段）、Markdown 编辑（复用 MarkdownEditor）。

### B. 笔记本
1. **数据模型**：notebook（name/description/cover/order）+ note（notebook_id、title、content、tags、created/updated）。
2. **前台路由**：`/notebooks`（笔记本列表）、`/notebooks/:name`（笔记列表，分页 per_page 可配）、`/note/:id`（笔记页）。
3. **tagtree widget**：笔记标签树（expand_all/expand_active/show_tagcon 参数，对照原版 tagtree.ejs）。
4. **Console 管理**：notes 模块从「单页编辑」升级为笔记本/笔记两级管理（现有 content/notes.md 单页保留为 `/notes` 兼容入口）。

## What Changes

- `internal/store/`：+`wikidoc.go`、`notebook.go`（各含建表迁移 + CRUD + `_test.go`）。
- `cmd/server/routes_taxonomy.go`：wiki 段扩展三级路由；+notebooks 路由段。
- `internal/template/`：+`partial/wiki_doc.html`、`notebooks.html`、`notebook.html`、`note.html`、`partial/widgets/tree.html`、`tagtree.html`；面包屑组件加 wiki/notebook 维度分支。
- `internal/api/`：+`wiki.go`、`notebook.go`（/api/v1 管理端点）；router.go 挂载。
- `console/src/modules/`：+`wiki/`（项目/文档管理）；`notes/` 升级两级管理。
- feed/sitemap：wiki 文档与笔记纳入 sitemap（feed 不纳入，见技术决策）。

## 实现步骤（建议拆两个子分支串行）

1. **35a wiki**：store → 前台路由/模板/tree widget → api → Console 文库模块 → 面包屑。
2. **35b notebooks**：store → 前台三层页面/tagtree → api → Console notes 升级。
3. 每步过 `make test` + 前台目检对照原版文档站（https://xaoxuu.com/wiki/stellar/ 的结构即 wiki 体系样板）。

## 技术决策（无需再问）

- **wiki 文档入 SQLite 而非文件树**：与 posts/pages 一致的存储与备份路径；导入导出走既有 backup Markdown 管线扩展。
- **树形用 parent + order 字段**：不引入嵌套集/路径枚举，层级预期 ≤3 层，递归组装在内存完成。
- **`/notes` 兼容**：现有单页保留并在页头加「进入笔记本」入口，不做 301。
- **多作者体系不随本 spec**：原版 author 页依赖 authors.yml，见 Spec 38。
- **加密文档不支持**：wiki/note 无 password 字段（需求出现再加列）。

## Impact

- **新增**：2 个 store 文件、2 个 api 文件、6+ 模板、Console wiki 模块
- **修改**：routes_taxonomy.go、router.go、面包屑、sitemap、Console notes/侧栏菜单
- **不影响**：posts/pages/moments 主链路、shortcode

## 验证

- [ ] /wiki/:project/:slug 渲染文档 + tree 高亮 + 上下篇 + wiki 面包屑（对照原版文档站结构）
- [ ] /notebooks 三层页面可走通，tagtree 参数生效
- [ ] Console 可建项目/文档/笔记本/笔记并排序，前台即时可见
- [ ] 备份导出包含 wiki/note 数据；sitemap 收录
- [ ] 现有 /wiki 卡片页与 /notes 单页不回归
- [ ] `make test` 全绿（store/api 单测齐）
