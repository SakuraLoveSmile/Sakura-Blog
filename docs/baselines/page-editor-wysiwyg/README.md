# BLG-EDIT02 本地验收

唯一目标参照为同一代码、站点设置及视口下的正式页面。`before-user.png` 是用户问题截图，`before-browser.jpg` 与错误日志保留为修复前证据。权威记录为 `docs/plans/page-editor-wysiwyg.md`。

- `after-desktop-dark.png`、`after-exchange-dark.png`、`after-mobile-dark.png`：最终构建的编辑器和本站资料，使用隔离三条友链，不能当成生产已更新的证据。
- `browser-matrix.json`：所有样式修正完成后的 32 场景（四外观 × 浅/深/自动系统浅/深 × 1280/390），全部通过。后续焦点捕获调整不改样式；`final-smoke.json` 是最终构建的标题、失联分组、资料及透明表面对照。
- `ordinary-styles.json`、`after-ordinary-dark.png`：正文、标题、列表、代码块及表格与正式页相同的样式度量和截图。`table-selection.json` 保存四格实际选区反馈；`blank-page.json` 保存空白页主题及占位提示。普通页验收内容已还原。
- `save-roundtrip.json`：实际修改、保存与恢复后，友链原文相同、配置语义相同，生成资料未写入原文。
- `theme-refresh.json`：同一 SPA 内离开编辑器、切主题再返回，正文和动态友链均采用新主题。
- `interaction-final.json`：拖拽取消、连续移动/撤销和属性输入/关闭。最终鼠标补查受工具对齐高 iframe 的行为影响，日志证明部分点击送达前已滚动、实际目标变化；这些操作不计通过，属性功能已通过 Enter/输入/Escape 核验。鼠标连续操作待实际体验确认。临时诊断代码已移除。
- `checks-verification.json`：最终源文件/二进制 hash、真实 build/test 结果和服务资源 hash。Console 469、Stellar assets 229，加 Go vet、Staticcheck 及全部 Go 测试通过；实际服务资源全部与当前磁盘一致。

当前仅本地，未提交、合入或上线。没有把高级动态图形在代码编辑状态下的显示声明为全部所见即所得。
