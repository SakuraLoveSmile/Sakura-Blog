---
title: "Stellar 1.42.1 同步验收"
date: 2026-08-18 16:00:00
excerpt: "验收 Stellar 1.42.1 的 hero 文章卡片、自适应文字、专栏上下布局、横幅结构与懒加载修复。"
description: "这篇文章用于现场核对 Stellar 1.42.1 的前台同步效果。"
category: "平台建设"
topic: "Stellar 1.42.1"
tags: [Stellar, 1.42.1, hero, adaptive-text, 专栏, 验收]
status: published
cover: /assets/img/icon-512.png
banner: /assets/img/icon-512.png
ai_label: reviewed
pinned: true
---

## 本文验收点

本文是 Stellar 1.42.1 的现场验收入口。首页启用 `article.card_style: hero` 时，本页封面应显示为全图文字卡片，文字区固定在底部；无封面的文章应继续回退为 classic 摘要卡片。

## Hero 文章卡片

- 标题取文章标题，封面存在时使用 hero 全图卡片。
- 小字按 `subtitle > description > excerpt` 取值，并限制为单行省略。
- 将 `article.card_style` 切换为 `classic` 后，封面文章应恢复封面、标题、摘要、日期的普通卡片结构。

## 自适应文字

本页封面和文章横幅都带有 `data-text-adaptive="split"` 目标。文字颜色应根据图片平均色计算：标题使用高对比色，小字、面包屑和日期使用主题色；图片加载失败时仍保留默认深色模式文字，不应白屏。

## 置顶轮播

本页设置了 `pinned: true`。首页第一页应出现置顶轮播，轮播标题与小字结构和 hero 卡片一致；箭头、圆点和自动播放应保持可用，文章列表不应重复显示本页。

## 专栏上下布局

访问 `/topics`，找到「Stellar 1.42.1」专栏：应看到专栏标题与描述、全宽最新文章封面卡片，以及其余文章的标题/日期列表。没有文章时应显示空态，不应输出空链接。

## 横幅与修复点

文章顶部应是两行结构：第一行包含面包屑、阅读时长或 AI 标签，第二行包含发布日期与更新时间。1.42.1 同步还包括：

- `--gap-margin`、`--gap-padding`、`--gap-max` 已迁移到 `--gap-base`、`--gap-page`。
- `article.auto_banner` 已移除，未配置 banner 时不再自动请求外部横幅。
- 评论数图标统一使用 `default:tocomment`，`weibo:comment` 不再存在。
- 动态图片懒加载只读取 DOM 图片属性，跳过 script、style、noscript、注释和显式 `no-lazy` 图片。

## 验收方式

逐项检查首页、文章页和专栏页的 DOM class 与视觉状态；再在 Console 文章设置中切换 hero/classic、启用或禁用自适应文字，刷新页面确认配置可持久化。
