# 页面横幅与正文间距修复基线

## 参照图

- 项目当前 `/about` 截图：[user-about.png](./user-about.png)
- Stellar 原版 `/notes/` 对照截图：[stellar-notes-reference.png](./stellar-notes-reference.png)

## 验收清单

- [ ] `/about` 横幅底部到正文首个 `.tag-plugin` 顶部的实际间距为 `var(--gap-p)`（当前主题桌面值为 32px）。
- [ ] 启用 Heti 时，正文首块的 `margin-block-start` 不再被 `.heti > :first-child` 的 `!important` 规则清零。
- [ ] `/notes` 等非 `layout="page"` 页面不被该 page 专用规则改变布局。
- [ ] 同尺寸深色模式截图中，横幅与正文首块的留白节奏与 Stellar 原版保持一致。
