# Stellar 1.43 Collection 回归基线

## 唯一视觉参照

- 上游主题：`resource/hexo-theme-stellar-143/`
- 固定版本：`1.43.0`（`a558ac7c5abf058b5331f7146734dde487f4ae3a`）
- 上游预览源：`resource/hexo-theme-stellar-143/layout/collection-preview.ejs`
- 深色截图：`upstream-collection-preview-dark.png`

截图只用于本次回归的结构与交互验收，项目产品能力仍固定为深色模式。

## 截图条件

截图页面按上游 `collection-preview.ejs` 的四个 surface 与四种 collection 变体渲染：

- `glass` / `card` / `sidebar` / `content`
- `list/nav/compact`
- `grid/nav/auto`
- `list/summary/auto`
- 层级 list 的 `depth`、active、长标题截断和描述截断

## 验收清单

- [ ] compact list 中条目逐项排列，长标题不撑破容器。
- [ ] grid 根据容器宽度自动降列，条目保持最小可用宽度。
- [ ] summary 显示标题和描述，描述最多两行并保持文本层级。
- [ ] active 条目保留 `is-active`、`aria-current` 和主题色反馈。
- [ ] hover/focus-visible 反馈在四种 surface 下可见且不依赖旧的 `post-list`、`post-card` 或 `.linklist .link` 选择器。
- [ ] 深色模式下背景、文字、边框和 active/hover 反馈与截图基线一致。
