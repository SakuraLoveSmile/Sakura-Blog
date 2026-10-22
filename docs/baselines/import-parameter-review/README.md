# 不支持参数的导入选择

状态：实现中。唯一现状参照为用户截图 `import-error-reference.png`（备份页旧版 Markdown 导入仅返回英文错误）。本次沿用现有 BackupView、VModal、VButton 及语义 token，修改导入交互，不更换页面布局或设计体系。

## 冻结行为与分工

T1 由后端 worker 在独立 sibling worktree `Blog-import-parameter-backend` 实现；独占 migration/backup/API 及其测试和生成类型。T2 由主 agent 在 `Blog-import-parameter-review` 实现；独占 BackupView、API client 调用、前端行为测试、本目录说明及集成验证。两者依赖以下共享契约，后端完成后由主 agent 集成，主 agent 亲自核对浏览器用户流程。

- 仅显式旧版转换支持忽略。400 Problem `legacy_markdown_unsupported_parameters` 的 `unsupported_parameters` 元素为 `{source,path,field,code,message,requires_draft}`，完全不含参数值；`path` 为 RFC6901 JSON pointer。
- 原文件、File 对象、已确认的创建专栏和忽略列表在本次重试中保留。multipart `ignored_parameters` 为精确 `{source,path}` 数组；服务端重新诊断，仅允许删除当前实际不支持的字段。
- 生命周期字段 `password/publish_at/authors/draft_json`（含 `blog.host` 对应项）及迁移后未支持的 unknown/removed 字段可经确认忽略。类型、格式、映射冲突及正文语法错误仍需用户修改。
- 忽略任意生命周期字段的对应文章强制草稿，覆盖源 `published` 和已有记录状态；既有宿主保护与所有权按既有规则保留。被忽略的密码不进入回包或归档。未确认、取消或仍有错误时整批零写入。

## T2 界面差异

保持初次导入、旧版转换和创建专栏确认。遇到不支持参数时打开标题为“发现不支持的参数”的 VModal，正文按文件名、参数名、原因逐项列出；只显示元数据位置，不显示原值。列表内容可滚动，长文件名和字段名换行。

底部两个操作为“取消并自行修改”和“忽略并继续导入”。取消、关闭、Escape 均结束本次导入且不写数据；继续仅发送清单中的精确字段，执行期间禁用重复导入。存在生命周期字段时明确“对应文章将保存为草稿”，其余情况沿用已有状态语义。成功后显示导入结果。

弹窗最大宽度 640px；沿用 VModal 的遮罩、焦点约束、关闭行为和现有卡片层级。移动端 footer 使用纵向或换行按钮，避免 390px 下溢出；不新增全局样式或裸颜色。

## 验收

- [ ] T1：空/非空密码、多个未知字段和 ZIP 多文件返回完整清单，第一次请求零写入。
- [ ] T1：只能忽略清单中的精确字段；strict、任意路径和无效授权继续拒绝。
- [ ] T1：忽略后真实隔离库导入成功，正文保留；生命周期忽略强制草稿且密码不泄漏；其他硬错误仍使整批零写入。
- [ ] T2：真实浏览器展示清单、取消不导入、确认后完成导入，缺失专栏链保留忽略选择。
- [ ] T2：390px 下长列表/文件名可读，按钮可触达，键盘关闭有效。
- [ ] 集成：Console 类型/行为、Go 相关测试、完整项目门禁和真实构建通过；服务重启后确认加载当前 SPA 产物，再保留浏览器截图。

验证仅使用隔离配置、数据库、内容和上传目录，避免生产内容变化。生产发布状态另行据实记录。

## 本次实测

2026-10-21 在独立 root（config、DB、content、uploads、maintenance 均隔离）的 localhost:8106 完成真实构建、重启与 SPA hash 校验。浏览器依次验证取消零写入、忽略成功、空密码提示、缺失专栏取消及创建成功、ZIP 两文件四参数分组提示与关闭、390×844 长列表完整滚动和 Escape。最终隔离库 3 篇草稿、1 个专栏，正文逐字一致且忽略值不进入配置。见 `local-verification.json`、`local-assets.json` 与 `local-browser-assets.json`。

截图：`local-desktop-parameters.png`、`local-mobile-parameters.png`、`local-mobile-parameters-end.png`、`local-zip-parameters.png`、`local-nested-parameters.png`、`local-topic-confirmation.png`、`local-import-success.png`。旧 references 别名与依赖 type 的 footer action 漏报在独立审查中发现，已补准确原路径诊断和回归，复审关闭全部发现。

后端三包测试通过。项目完整门禁在提交 hook 与 `make merge-check` 中执行；生产更新和数据保留证据将在部署完成后追加，不提前记为通过。
