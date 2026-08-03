---
title: CSS Grid 布局实战：从网格到复杂页面
date: 2026-07-28
excerpt: 用三个真实场景把 CSS Grid 的常用写法串一遍：两栏、瀑布流、以及响应式栅格。
categories: 技术
topic: 前端
tags: [CSS, 前端, 布局]
cover: https://picsum.photos/seed/css-grid/1280/640
poster_headline: Grid 不只是一张表
poster_caption: display: grid · grid-template-areas · auto-fit
---

## 为什么值得学 Grid

Flexbox 解决的是**一维**排列，Grid 直接解锁**二维**布局。写页面时 90% 的"居中、分栏、对齐"问题，Grid 一行声明就能搞定。

## 场景一：经典两栏

```css
.grid-2col {
  display: grid;
  grid-template-columns: 1fr 2fr; /* 左窄右宽 */
  gap: 24px;
}
```

## 场景二：响应式栅格

不用写媒体查询，`auto-fit` + `minmax` 自动换行：

```css
.cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 20px;
}
```

> 容器越宽，列数自动变多；列再宽也不超过卡片内容合理宽度。

## 场景三：区域命名

适合整页骨架，语义清晰：

```css
.page {
  display: grid;
  grid-template-areas:
    "header header"
    "main   aside"
    "footer footer";
  grid-template-columns: 1fr 300px;
}
```

## 注意事项

- `gap` 已经取代 `grid-gap`，别再用带前缀的老写法。
- 网格子项的 `min-width` 默认是 `auto`，内容过宽会把网格撑破，必要时加 `min-width: 0`。
- 兼容性早已无忧，2026 年可以放心在正式项目里使用。
