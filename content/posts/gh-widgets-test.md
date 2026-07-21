---
title: GitHub Widgets 测试
date: 2026-07-21
excerpt: 测试 GitHub widgets（ghcard 代理卡片 / ghrepo 仓库卡 / ghuser 用户卡）的渲染与降级
tags: [测试, 插件]
---

## GHCard 统计卡片（请求时代理 + 缓存）

用户统计卡：

{% ghcard xaoxuu %}

仓库卡（dark 主题）：

{% ghcard xaoxuu/hexo-theme-stellar theme:dark %}

## GHRepo 仓库卡片（渲染时取数，静态快照）

{% ghrepo xaoxuu/hexo-theme-stellar %}

## GHUser 用户卡片（渲染时取数，静态快照）

{% ghuser xaoxuu %}
