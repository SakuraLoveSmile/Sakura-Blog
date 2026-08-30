# 文章设置 Halo 布局改造

## 唯一参照

- 参照来源：Halo 社区版源码中的 `post` 设置表单，不新增第二套视觉参照。
- 参考版本：本地 `resource/halo` checkout，commit `57a54168bc6f87cf5f1f1f0000ab9bff0fc5406a`。
- 关键源码：
  - `application/src/main/resources/extensions/system-setting.yaml`
  - `ui/console-src/modules/system/settings/SystemSettings.vue`
  - `ui/console-src/modules/system/settings/tabs/Setting.vue`
- 参照结论：外层使用页签卡片，表单内容在卡片内纵向排列，字段之间保持宽松留白，单项控件限制最大宽度，保存按钮固定在内容底部。

## 本阶段边界

- 仅改 Blog Console 的 `/console/settings?tab=post`。
- 只重构现有 `article` 设置的表单结构和视觉布局，不修改共享设置壳、其他页签、前台路由或配置字段语义。
- 保留现有分类/标签映射、图标选择、附件库选择、前台预览、未保存保护和 `PUT /api/v1/settings/article` 保存契约。
- Halo `post` 中 Blog 尚未支持的别名生成策略、附件存储策略和附件存储组不在本阶段新增；它们需要独立的配置模型与前台/编辑器任务。

## 验收清单

- [x] `post` 页签内容不再使用嵌套设置卡片，表单直接承载在外层页签卡片中。
- [x] 文章展示、阅读体验、分类与标签映射、版权与分享、打赏按纵向表单分组展示。
- [x] 输入框、选择框和附件/图标控件保持单列、统一最大宽度，字段之间有清晰留白和分隔。
- [x] 现有文章设置字段均仍可编辑，分类颜色/图标/标签图标的新增、改名、删除行为不回归。
- [x] 保存区固定在卡片底部，桌面端和移动端滚动到最后字段后仍可触达，不遮挡内容。
- [x] `/console/settings?tab=post`、页签切换和旧 `?tab=article` 入口仍然有效。
- [x] 保存仍调用 `PUT /api/v1/settings/article`，成功后 Toast、前台预览和脏状态行为不回归。
- [x] 仅使用项目现有深色语义 token；移动端控件触达尺寸和键盘焦点状态可用。

## 验收记录

- 视口：桌面默认 `1280×720`、移动端 `390×844`。
- 机械验证：Console 类型检查/行为测试、真实 Console 构建、`make test`、`git diff --check`。
- 浏览器验证：页面加载、文章设置字段编辑、分类映射操作、附件控件可见性、保存栏 sticky 行为、移动端横向溢出和浏览器控制台错误/警告。

## 验收截图

已实现并启动本地 Console，补充本次改造后的桌面/移动端截图。

- [桌面端顶部](desktop-top.jpg)
- [桌面端底部与 sticky 保存栏](desktop-bottom.jpg)
- [移动端顶部](mobile-top.jpg)
- [移动端底部与 sticky 保存栏](mobile-bottom.jpg)

实测记录：桌面端 `1280×720`，移动端 `390×844`；移动端 `document.documentElement.scrollWidth === clientWidth`，无横向溢出；分类映射添加后条目数增加，附件菜单包含上传、附件库、输入链接和移除；浏览器控制台无 error/warn。
