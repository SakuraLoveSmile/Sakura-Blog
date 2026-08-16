---
title: "远程 Markdown 与安全回退"
date: 2026-08-15 12:00:00
excerpt: "验证 md 标签的 wrap 参数、远程 README 占位和失败时的可读回退。"
category: "测试"
topic: "标签插件"
tags: [1.40.0, Markdown, README, 回退]
status: published
---

## 默认模式

远程 Markdown 使用客户端渲染占位，页面首次加载时仍应保留稳定的布局高度。

{% md https://raw.githubusercontent.com/xaoxuu/hexo-theme-stellar/main/README.md %}

## 包裹模式

{% md https://raw.githubusercontent.com/xaoxuu/hexo-theme-stellar/main/README.md wrap:true %}

## 本地链接

{% md /notes/readme.md wrap:false %}

这些地址只用于渲染链路测试；请求失败时应显示清晰的错误状态，而不是让正文消失。
