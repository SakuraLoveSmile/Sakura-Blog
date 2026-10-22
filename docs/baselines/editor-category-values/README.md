# 文章分类值的编辑器兼容

状态：实现中。唯一界面参照为用户提供的 `reference-error.png`（保存时 categories 字符串无法解析为列表）。沿用现有文章编辑器、分类控件和项目 Apple 设计库。

## 行为与验收

- [x] 单个分类与分类列表回填后均按列表提交，空分类不产生空标签。
- [x] 分类名称中的逗号、中文逗号和层级斜杠保留；其他控件修改不改写原分类配置。
- [x] 编辑分类可正常增加、移除，原 Markdown 正文及其他配置保持。
- [x] 隔离环境真实浏览器完成原文导入、保存、重新加载、发布和预览；核验当前 SPA 资源。
- [ ] 构建、提交与合入门禁通过，上线核验服务与原数据保留。

本次仅修正表单回填时的分类类型投影，不改变接口 schema 或自动发布行为。

## 本次验证

新增两项真实 Vue 同步回归，修复前均失败：字符串原样提交，以及空字符串没有投影为空列表。修复后现有 `stellar-form-json.test.ts` 全部 9 项通过；完整 `make build` 通过，独立 reviewer 未发现阻断项。数组原文携带的精度元数据在测试中按实际 JSON 提交值核验，不误当业务分类字段。

`localhost:8110` 的 config、DB、content、uploads 完全隔离。已有 Perfdog（原始 `categories` 为字符串）直接保存、重载、发布成功；未修改的完整 FilterBox 原文经兼容确认导入后保存成功，再通过分类控件添加 `补充/兼容`，保存、重载、草稿预览和发布均成功。Perfdog 原配置保持分类字符串，FilterBox 仅分类控件修改后变为列表。分别 15,213 与 5,835 字节正文逐字一致，两篇原文件未修改；既有两篇夹具、users/settings/topics 保持，integrity 为 ok。

29 个实际加载的 Console scripts/styles/modulePreloads 均与本次 dist 和服务响应哈希一致。见 `local-verification.json`、`local-browser-assets.json`、`local-scalar-saved.png`、`local-list-saved.png`、`local-list-preview.png`。上述发布均发生在隔离环境；线上文章未执行发布操作。
