# Markdown 缺失专栏导入

状态：待体验（本地修复已完成验证；服务器更新另记）。

唯一问题基线：`docs/baselines/markdown-topic-import/before.png`，来自用户截图。用户补充的真实字段为 `topic: zero_tech`，其余头部保存为 `internal/stellarmigration/testdata/teamspeak-legacy.md`；正文为回归夹具，不冒充用户原文。

本次只扩展备份页现有导入确认流程，复用 `Dialog.warning`、按钮、现有布局与语义样式。缺失专栏时列出待创建的名称和 Slug，提供「取消」和「创建并导入」，不增加页面或改变设置页风格。

- [x] 真实头部导入能提示创建 `zero_tech`，明确名称和 Slug 相同；取消不创建文章或专栏。
- [x] 确认后专栏和文章一起导入；未指定状态的新文章仍为草稿，正文、日期、分类、标签、引用、渲染设置、banner 和 abbrlink 保留。
- [x] ZIP 中相同缺失专栏只创建一次，批次其他文件出错时文章和新专栏都不写入；歧义归属继续拒绝。
- [x] 空 topic 视为未分配；严格导入仍要求显式选择旧版转换，编辑器预览仍只读。
- [x] 1280px 与 390px 真实浏览器确认弹窗可操作，无横向溢出；核对当前服务产物后保存截图。

约束：本次沿用显式 slug 优先、否则文件名的既有身份规则；`abbrlink` 作为原始扩展字段保留，不借此次修复改变覆盖目标。创建仅发生在用户确认后，使用现有副本与整批恢复流程。

本次验证（2026-10-20）：

- `go test ./internal/backup ./modules/console/api -run 'Test(LegacyMarkdown|StellarContent|ExplicitLegacy|MarkdownImportConversionHint)' -count=1` 通过；此前 migration 空值与错误语义用例已先红后绿。
- 新增「预检后现有专栏并发改名」回归先红：未保护时导入错误地成功。补上事务内 ID/Slug/名称全部命中校验后通过，保留并发写入的专栏与所有原文章，拒绝整个导入。
- `pnpm exec playwright test tests/e2e/backup-import.spec.ts tests/e2e/editor-md-import.spec.ts` 11 项通过。`confirm-1280.png`、`confirm-390.png`、`imported-1280.png`、`imported-390.png` 为本次当前构建的浏览器证据。
- 主 agent 使用独立数据库的 8097 站点真实点击创建并导入，文章管理页显示 TeamSpeak 草稿；截图为 `browser-confirm.png`、`browser-imported.png`、`browser-draft.png`。验收前核对 6 个 Console 入口资源的 SHA256，记录在 `local-assets.json`。
- 线上只读查询确认 `zero_tech` 匹配数为 0；未创建生产专栏或导入真实文章。

`make build` 与 `make test` 已通过。提交门禁、合并门禁和部署证据在交付记录中分别报告。本说明的勾选表示本次已实测，不代表用户已验收。
