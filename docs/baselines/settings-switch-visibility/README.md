# 设置页开关可见性修复

## 唯一参照

- 参照图：[文章设置 Halo 布局基线](../settings-post-halo-layout/desktop-top.jpg)。
- 参照来源：本仓库既有文章设置页基线，沿用共享 `VSwitch` 的 Halo/iOS 外观，不新增局部设置页样式。

## 本阶段边界

- 仅修复 Console Tailwind 对 `packages/components/src` 共享源码的扫描范围。
- 保留 `VSwitch` 的 `v-model`、`role="switch"`、ARIA 状态和现有视觉规则。
- 不修改设置数据结构、API 或保存逻辑。

## 验收清单

- [ ] 文章设置页实际渲染 8 个开关，按钮和圆点不再是 `0×0`。
- [ ] 关闭状态使用现有深色 accent，开启状态使用 success 色，切换后 `aria-checked` 同步变化。
- [ ] 其他使用 `VSwitch` 的页面未丢失尺寸样式。
- [ ] 页面由最新 Console SPA 产物提供，浏览器控制台无新增 error/warn。

## 验收记录

- 视口：桌面 `1280×720`；按需补充移动端检查。
- 机械验证：Console 类型检查、行为测试、真实 Console 构建；检查生成 CSS 中的 `51×31` 与 `27×27` 尺寸规则。
- 浏览器验证：文章设置页开关数量、尺寸、颜色、ARIA 切换，以及其他 `VSwitch` 使用页。
