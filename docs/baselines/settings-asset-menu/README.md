# 设置页附件字段操作菜单

## 唯一参照

- 参照图：`halo-attachment-menu-reference.png`
- 来源：用户提供的 Halo 社区版附件字段操作菜单截图
- 参考源码版本：本地 `resource/halo` checkout，commit `57a54168bc6f87cf5f1f1f0000ab9bff0fc5406a`
- 参考实现：`ui/src/formkit/inputs/attachment/AttachmentInput.vue` 及其 `UploadDropdownItem.vue`、`AttachmentDropdownItem.vue`、`CustomLinkDropdownItem.vue`

## 本阶段边界

- 将 Blog Console 的 `AttachmentField` 改为 Halo 风格的预览卡片和锚定操作菜单。
- 基本设置中的 Logo、Favicon、Apple Touch Icon 等图片字段使用同一套操作菜单。
- 保留 Blog 现有附件库分页选择和 `modelValue` URL 语义；上传成功后回填附件 URL。
- 继续只使用深色模式和项目已有 Apple Design 语义 token，不复制参照图的浅色配色。
- 不改附件管理页、后端附件 API、主题素材配置语义，也不新增多选能力。

## 验收清单

- [x] 有图片或默认回退图时显示预览卡片，右上角显示更多操作按钮。
- [x] 无图片时显示可添加的空预览卡片，仍能打开同一操作菜单。
- [x] 菜单顺序为“上传 → 从附件库选择 → 输入链接 → 分隔线 → 移除”。
- [x] “上传”调用现有附件上传 API，并将返回 URL 写回字段；同一文件可重复选择。
- [x] “从附件库选择”打开现有图片附件选择弹窗并回填 URL。
- [x] “输入链接”打开可聚焦的链接编辑器，保存后回填并关闭编辑器。
- [x] “移除”清空自定义 URL；带默认回退图的字段仍显示默认图。
- [x] 点击组件外部或按 Escape 可关闭菜单/链接编辑器，焦点状态清晰可见。
- [x] 菜单和按钮在桌面端及 `390×844` 窄屏不溢出，触达区域至少 44×44px。
- [x] 现有不显示缩略图的 URL 字段仍保留文本输入能力，并可使用同一套操作菜单。
- [x] Console 测试、构建、`make merge-check` 和浏览器运行时验收通过。

## 验收截图

实现并启动本地 Console 后补充桌面和移动端改造后截图：

- [桌面端菜单](desktop-menu.png)
- [390×844 移动端菜单](mobile-menu.png)
