# BLG-20261106-03 文库管理：从「文章」导入文档

## 目标与范围

- 在 Console「文库管理」（`/console/wiki`）增加「导入文章」入口：把「文章」中已有的文章（标题 + Markdown 正文）复制成当前选中文库项目的文档。
- 导入只读取原文章，不修改文章本身；只创建新文档，不覆盖项目内已有同名 Slug 的文档。
- 不做：本机 Markdown 文件上传、跨项目批量搬移、导入后自动发布、覆盖已有文档、改写原文章 front matter。

## 已确认决定

- 导入来源：文章管理里的已有文章（用户 2026-11-06 确认；本地 `.md` 文件导入是另一个方向，本次不做）。
- 目标项目：页面当前选中的文库项目（`/wiki/:project`）。
- 默认状态：导入弹窗「导入后状态」默认草稿，用户可显式改为已发布；未显式选择时不公开。
- Slug 冲突：跳过并在结果里说明原因，不覆盖、不自动改名。
- 逐条结果：单篇跳过/失败不回滚已成功导入的文档；结果面板分别列出成功、跳过、失败及原因。
- 复制字段：`title`、`slug`、`content_md`、`stellar_version = 2`、`stellar_config = "{}"`、`blog_extensions = "{}"`、可选父文档、`order = 0`。
  不复制文章级字段（封面、分类、标签、作者、摘要、SEO 等）：`wikidocs` 没有这些列，wiki profile 也不接受文章配置。
- 批量上限：单次 100 篇（弹窗默认每页 20 篇，翻页多选）。

## 接口契约

`POST /api/v1/wiki/:project/docs/import`

请求（JSON）：

```json
{ "slugs": ["post-a", "post-b"], "parent": 0, "status": "draft" }
```

- `slugs`：文章 slug 列表（去重、去空）；未知文章 / 回收站文章进入 `failed`。
- `parent`：目标父文档 ID，0 表示根文档；必须存在且属于当前项目，否则整单 400。
- `status`：`draft`（默认）或 `published`；其他值 400。

响应（200，具名 struct，可经 tygo 生成到 Console 类型）：

```json
{
  "imported": [{ "post_slug": "post-a", "slug": "post-a", "title": "A", "result": "imported", "doc_id": 7 }],
  "skipped": [{ "post_slug": "post-b", "slug": "post-b", "title": "B", "result": "skipped", "reason": "项目内已存在同名 Slug 文档" }],
  "failed": []
}
```

错误语义：项目不存在 404；请求格式、空列表、超上限、非法状态、父文档非法 400；单篇渲染失败只记入 `failed`，不回滚整单。

## 分支与工作区

- worktree：`/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261106-feat-blg-20261106-03-wiki-post-import`
- 分支：`codex/feat/20261106-blg-20261106-03-wiki-post-import`，合入根目录 `dev`。

## 验收场景

1. 文库页头部出现「导入文章」；未选项目或项目无文章时给出空态说明。
2. 弹窗可按关键词搜索文章、翻页、多选；已导入（项目内已有同名 Slug）文章标记「已导入」且不可再选；回收站文章默认不在候选里。
3. 选择 2 篇文章 + 默认草稿导入后：文档出现在当前项目，标题/正文/顺序正确，状态为草稿；原文章内容不变。
4. 切换「导入后状态 = 已发布」导入的文档可在线访问，统计与树同步。
5. 目标父文档生效：导入的文档成为该父文档子节点。
6. 同一批再次导入：全部跳过，不产生重复文档，结果面板说明原因。
7. 回收站文章不会出现在导入候选（文章列表默认排除回收站）；即使绕过界面直接提交，后端也只记失败原因、不写入。
8. 手机 390px：弹窗可用，无横向溢出。

## 验证方式

- `make test`（Go vet/Staticcheck/全部 Go 测试 + Console 类型与行为测试 + assets/module 检查）。
- `make build` 后 Playwright `tests/e2e/wiki-import.spec.ts` 走真实服务与当前 SPA 产物。
- 截图与实测记录存 `docs/baselines/wiki-post-import/`，参照图为用户提供的改造前截图。

## 实现要点

- 后端：`modules/console/api/wiki_import.go` 新增 `POST /api/v1/wiki/:project/docs/import`，复用项目存在性、父文档归属与严格正文渲染链路；逐条创建文档，单篇跳过/失败不整单回滚，确有写入才广播内容变更事件。
- 类型：Go 响应/请求 struct 纳入 `tygo.yaml`，`go generate ./modules/console/api` 生成 `WikiDocImportRequest`/`WikiDocImportItem`/`WikiDocImportResponse` 并更新 `packages/api-client/src/generated.d.ts`；`packages/api-client/src/api/wiki.ts` 暴露 `wikiApi.importDocs`。
- 前端：新增 `WikiImportModal.vue`（服务端搜索、跨页多选、全选本页、父文档/状态选项、结果面板）与纯逻辑 `wiki-import.ts`；`WikiDocsView.vue` 在页头、右侧空态与空树提示接入入口，导入后刷新文档树并打开第一篇新文档。
- 回收站文章：文章列表接口默认排除回收站，因此弹窗候选里不会出现；后端仍对直接提交的回收站 slug 返回 `failed`（「文章在回收站，无法导入」）。
- 弹窗分页独占一行：避免长父文档名把「共 N 条」挤成竖排；手机上搜索独占一行、列表限高 240px。

## 实测与未验证

- 已实测：`make test` 全绿（Console `vue-tsc` + `node:test`、Go vet/Staticcheck/全部 Go 测试、assets/module/release-source 检查）；`pnpm exec playwright test tests/e2e/wiki-import.spec.ts` 4/4 通过；截图与证据见 `docs/baselines/wiki-post-import/`。
- 未验证：生产实例与真实线上文章数据（使用隔离临时库与 API 夹具）、Safari/Firefox/真机、单次 100 篇上限的极端批量性能、正文渲染失败时的浏览器提示（Go 回归覆盖文案）。

## 状态

- 2026-11-06：待体验（实现与本地验证完成，等待用户实际体验确认；合入 `dev`，未推送、未发布）。
