# 编辑器 Markdown 导入验收基线

## 唯一参照

唯一视觉参照为仓库内 Apple Design 规范（`resource/apple-design/`，深色模式）。
导入按钮沿用共享 `EditorToolbar` 的按钮尺寸、圆角、语义色和移动端触达尺寸；不引入浅色模式或新的裸色 token。

## 可逐项核对的验收清单

- [x] 文章编辑器和页面编辑器均显示“导入 Markdown 文件”按钮。
- [x] 仅接受 `.md` / `.markdown`，同一文件可重复选择。
- [x] 解析 Front Matter 后先确认，取消不会替换当前标题或正文。
- [x] 确认后只回填表单与正文，不自动保存或发布。
- [x] 未知/当前编辑器不支持的字段在确认框显式列出。
- [x] 富文本模式将 Stellar 块级标签（含 `table`、`tip`）作为可视化节点并原样序列化。
- [x] Markdown 模式保留正文源码并继续使用真实前台预览。
- [x] 1440px 桌面与 390px 手机布局均复用同一工具栏入口，无横向溢出。

## 运行证据

- [桌面基线（1440px）](./desktop-post-editor.png)
- [手机基线（390px）](./mobile-post-editor.png)
- `tests/e2e/editor-md-import.spec.ts`：文章取消/确认、重复选择、页面入口。
- `tests/e2e/tiptap-roundtrip.spec.ts`：`tip` / `table` Stellar 块级标签 round-trip 零差异。
