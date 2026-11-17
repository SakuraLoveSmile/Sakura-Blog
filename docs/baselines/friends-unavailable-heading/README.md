# `/friends` 「失联友链」分区标题居中

## 唯一参照

- `reference-report.png`：用户反馈截图（线上 `/friends`，`article.style=story` 深色主题，1465×838 设备像素 ≈ 732 CSS px）。
- 既有已验收基线：`docs/baselines/friends-page/after-centered-desktop-dark.jpg`、`after-centered-mobile-dark.jpg`——同页所有分区标题（含「失联友链」）居中。

## 症状与根因

「失联友链」标题由 `friends_and_posts.js` 在客户端补进 `.friends-page`，带 `class="unavailable-title"`。

`main-*.css` 的 story 文章样式给正文 h2 统一 `margin: 2em 0 1.5em`：

```css
body[data-article-style='story'] .md-text.content h2 { … margin: 2em 0 1.5em; }
```

其特异性 (0,3,2) 高于 `friends-page.css` 的分区标题规则 `.md-text.content .friends-page h2`（0,3,1），
左右外边距被清零；主题只用 `h2:not([class])` 给**无 class** 的标题补回居中（`margin-inline: auto`），
带 class 的「失联友链」因此贴左。其余分区标题都是 Markdown 输出的无 class h2，所以看起来正常。

## 修复

`modules/stellar/web/assets/css/friends-page.css` 增加一条同特异性规则，只补回水平居中：

```css
body[data-article-style] .md-text.content .friends-page h2 { margin-inline: auto; }
```

垂直间距仍来自文章样式，字体、斜杠、颜色不变；`tech` 与未设文章样式时该规则是空操作。

## 验收清单

- [x] 「失联友链」标题中心与正文列中心一致（线上实测 4 个宽度：390 / 732 / 1024 / 1280）。
- [x] 同页其他分区标题（友链列表 / 如何交换友链？ / 本站信息）位置、尺寸不变。
- [x] `article.style` = story / tech / 未设置三种情况均居中（静态复刻矩阵）。
- [x] 失联卡片内容（头像、名称、角标、描述）不受影响。
- [x] 线上发布后的实际效果：v2.1.9 上线后 390 / 732 / 1280 三档「失联友链」中心偏移均为 0，页面无 pageerror。

## 证据

- `before-732.png` / `after-732.png`：线上页面（732 CSS px、2x）失联分区注入修复规则前后。
- `production-732-section.png`：v2.1.9 上线后的生产页面失联分区。
- `measurements.json`：各宽度标题中心偏移、静态复刻矩阵与生产复核结果。

验证方式：Playwright 打开线上 `/friends`，测量 `.unavailable-title` 中心相对 `article.md-text.content` 中心的偏移，
再注入本轮规则复测；修复发布前线上 `friends-page.css` 与仓库 HEAD 逐字一致，因此注入等价于上线本改动。
v2.1.9 上线后按同一方法复测生产页面（`?v=6b957e02`）确认。
