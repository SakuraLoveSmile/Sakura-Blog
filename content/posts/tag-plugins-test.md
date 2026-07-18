---
title: 标签插件测试
date: 2026-07-18
excerpt: 测试 note 和 folding 标签插件的渲染效果
tags: [测试, 插件]
---

## Note 标签

{% note 这是一个默认样式的提示 %}

{% note color:blue 带标题的蓝色提示 这里是详细内容 %}

{% note color:green 成功提示 操作已完成 %}

{% note color:warning 警告信息 请注意这个操作 %}

## Folding 标签

{% folding 点击展开详细内容 %}
这里是一些 **Markdown** 格式的内容。

- 列表项 1
- 列表项 2
{% endfolding %}

{% folding color:blue open:true 默认展开的蓝色折叠 %}
这个折叠块默认是展开的。
{% endfolding %}

{% folding color:orange 橙色折叠块 %}
折叠内容支持 **Markdown** 渲染。
{% endfolding %}
