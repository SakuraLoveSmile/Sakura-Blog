---
title: 标签插件三期测试
date: 2026-07-21
excerpt: 测试 Stellar 标签插件三期（video/audio/copy/poetry/rating/swiper/chat/ghcard）的渲染效果
tags: [测试, 插件]
---

## Video 视频

B 站嵌入（16/9）：

{% video bilibili:BV1xx411c7mD %}

YouTube 嵌入：

{% video youtube:dQw4w9WgXcQ %}

原生 video 标签：

{% video https://example.com/sample.mp4 %}

## Audio 音频

网易云外链：

{% audio netease:1397345903 %}

原生 audio 标签：

{% audio https://example.com/sample.mp3 %}

## Copy 复制框

普通文本：

{% copy npm install hexo-theme-stellar %}

git ssh 展开：

{% copy git:ssh xaoxuu/hexo-theme-stellar %}

git https 展开（带前缀）：

{% copy git:https xaoxuu/hexo-theme-stellar prefix:$ %}

## Poetry 诗词

{% poetry 静夜思 author:李白 date:唐 footer:千古名篇 %}
床前明月光，

疑是地上霜。

举头望明月，

低头思故乡。
{% endpoetry %}

## Rating 评分

{% rating post-quality title:本文质量评分 %}

## Swiper 轮播

{% swiper width:max effect:cards %}
![图一](/assets/img/icon-512.png)
![图二](/assets/img/icon-512.png)
![图三](/assets/img/icon-512.png)
{% endswiper %}

## Chat 聊天记录

{% chat phone style:qq title:Stellar 交流群 me:me %}
me:
  name: 我
  avatar: https://avatars.githubusercontent.com/u/1
  label:
    text: 博主
    textColor: white
    bgColor: #2196f3
friend:
  name: 访客
  avatar: https://avatars.githubusercontent.com/u/2
<!-- chattip 今天 10:24 -->
<!-- chatcell user:friend md:这个主题真好看！ -->
<!-- chatcell user:me md:谢谢，基于 Stellar 做的～ -->
<!-- chatcell user:friend md:标签插件也很全 -->
{% endchat %}

## GHCard GitHub 卡片

用户卡：

{% ghcard xaoxuu %}

仓库卡：

{% ghcard xaoxuu/hexo-theme-stellar theme:dark %}
