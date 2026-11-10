# 友链页分区标题悬停基线

参照为 2026-11-10 20:14 用户提供的线上 `/friends` 两张截图，规范化副本存于本目录：

- `reference-default.webp`：默认态（宋体「友链列表」，两侧橙色斜杠，无底色）。
- `reference-hover-bug.webp`：鼠标悬停（整段标题被强调色块盖住，底部一条链接色细线）。

## 现象与来源

- 标题里的空锚点 `.headerlink` 被 `friends-page.css` 拉满整个标题框（`position:absolute; inset:0`）。
- 悬停色块来自主题 tech 文章标题的 `body[data-article-style="tech"] article.content h2 > a:first-child:hover { background: var(--accent) }`；`friends-page.css` 原先只覆盖了静态态 `background`，没有 `:hover` 覆盖。
- 底部细线来自 Heti 排版库的 `.heti a:hover { border-block-end: 1px solid currentColor }`；锚点的 `color` 是站点链接色，1px 边框落在拉满的锚点底部，橙色背景不覆盖边框，于是露出细线。

## 验收项

- [x] 默认态与修复前一致：双斜杠、宋体标题、无底色。
- [x] 悬停标题不再出现整块强调色，标题文字不被遮挡。
- [x] 悬停不再出现 1px 链接色底线（模拟站点开启 Heti）。
- [x] 标题锚点仍保留，点击可跳转（目录目标不变）。

## 证据

- `before-hover.png`：HEAD 版 `friends-page.css`（浏览器 route 拦截替换）+ 注入 Heti 链接悬停规则；悬停时锚点 `background: rgb(255,87,36)`、`border-bottom: 1px solid rgb(32,148,243)`。
- `after-hover.png`：修复版 CSS，同一环境同一坐标悬停；锚点 `background: transparent`、`border: 0`，标题正常显示。
- `after-hover-online.png`：线上 `/friends` 用 route 替换成本目录同款修复版 CSS 后悬停；真实 Heti 已加载、链接色为线上绿色，锚点 `background: transparent`、`border: 0`，标题正常显示。
- 环境：本地隔离站点 `http://127.0.0.1:8125/friends`（tech 文章样式，`friends-page.css?v=766935b1`），headless Chrome 1280×800、深色、2×；两次均 `hovered=true`，`pageerror` 为 0。
- 本地默认链接色是蓝色；线上是绿色，细线颜色随「链接颜色」设置变化，机制相同。
