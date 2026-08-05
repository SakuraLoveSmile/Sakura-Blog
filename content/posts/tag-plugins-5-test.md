---
title: "标签插件五期测试（展示类 11 件）"
date: 2026-07-30 10:00:00
category: "测试"
tags: [标签插件, 测试]
excerpt: "Spec 31：about / albums / posters / banner / blockquote / frame / md / navbar / toc / reel / paper 全量渲染验证。"
status: draft
---

## about

{% about avatar:/assets/img/icon-192.png height:80px %}
**你好**，这是 about 卡片正文，支持 Markdown。
{% endabout %}

## blockquote

{% blockquote indent:true %}
这是一段引用排版文本，*斜体*与**加粗**均可用。
{% endblockquote %}

## navbar

{% navbar active:/about/ [首页](/) [关于](/about/) [评论](#comments) %}

## banner

{% banner 大标题 副标题文字 bg:/assets/img/icon-512.png avatar:/assets/img/icon-192.png %}
{% navbar [首页](/) [归档](/archives/) %}
{% endbanner %}

## frame

{% frame iphone11 img:/assets/img/icon-512.png 设备边框示例 %}

## md

{% md /assets/test/demo.md %}

## toc

{% toc 目录标题 open:true %}

{% toc wiki:stellar %}

## albums

{% albums repo:xaoxuu/albums size:m %}

## posters

{% posters %}

## reel

{% reel 卷轴标题 author:测试作者 date:2026-07-30 footer:完 %}
胶片卷轴正文内容。
{% endreel %}

## paper

{% paper style:underline title:信纸标题 author:落款人 date:2026-07-30 %}
<!-- section 第一节 -->
第一节内容。

<!-- paragraph -->
普通段落内容。

<!-- line right -->
右对齐的一行。
{% endpaper %}
