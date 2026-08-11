---
title: "Stellar 1.38 新特性验证"
date: 2026-08-11
category: "测试"
tags: [Stellar, 1.38, MBTI, Emoji, 测试]
excerpt: "验证 1.38 版本新增特性：mbti 标签、emoji url: 外部图片、以及 photo 大卡 —cover-url 渲染。"
pinned: true
cover: /assets/img/icon-512.png
banner: /assets/img/icon-512.png
poster_headline: 1.38 · 新特性速览
poster_caption: MBTI · Emoji URL · Photo 大卡
poster_topic: Stellar
poster_color: accent
---

## MBTI 人格标签

1.38 新增的 mbti 标签，展示人格类型的五维倾向。

### INTJ — 建筑师

{% mbti INTJ-T 93-72-78-83-51 %}

### ENFP — 竞选者

{% mbti ENFP-A 78-65-82-70-60 %}

## Emoji 标签回归测试

### 普通 emoji（回归验证）

内置 default（qq）与 twemoji 两个表情源：

{% emoji aini %} {% emoji aini %} {% emoji twemoji 1f600 %} {% emoji twemoji 1f389 %}

指定高度：

{% emoji twemoji 1f600 height:24px %} {% emoji kukubi %}

### Emoji url: 参数（直接引用外部图片）

{% emoji url:https://cdn.jsdelivr.net/gh/twitter/twemoji@14.0.2/assets/72x72/1f600.png height:72px %}
{% emoji url:https://cdn.jsdelivr.net/gh/twitter/twemoji@14.0.2/assets/72x72/1f389.png height:72px name:party %}

也可以省略 name/height，只传 url：

{% emoji url:https://cdn.jsdelivr.net/gh/twitter/twemoji@14.0.2/assets/72x72/1f60e.png %}

## 总结

以上验证了 1.38 新增的 mbti 标签、emoji 的 url: 参数以及普通 emoji 的回归兼容性。photo 大卡通过 frontmatter 的 poster_headline/poster_caption/poster_topic 字段展示封面信息，结合 pinned: true 置顶于首页。