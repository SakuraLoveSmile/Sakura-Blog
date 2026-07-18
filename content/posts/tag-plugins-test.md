---
title: 标签插件测试
date: 2026-07-18
excerpt: 测试 Stellar 标签插件（note/folding/tabs/timeline/gallery/button/checkbox/mermaid）的渲染效果
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

## Tabs 标签

{% tabs %}
<!-- tab 简介 -->
这是第一个标签页，支持 **Markdown**。

- 要点 A
- 要点 B

<!-- tab 代码 -->
```go
fmt.Println("Hello, tabs!")
```

<!-- tab 链接 -->
访问 [示例链接](https://example.com)
{% endtabs %}

{% tabs active:2 align:center %}
<!-- tab 左对齐测试 -->
第一个面板

<!-- tab 默认激活 -->
第二个面板（active:2）

<!-- tab 第三个 -->
第三个面板
{% endtabs %}

## Timeline 标签

{% timeline %}
<!-- node 2024-01-01 -->
项目启动，完成基础架构设计。

<!-- node 2024-06-15 -->
发布 **v1.0** 版本，上线核心功能。

<!-- node 2025-03-20 -->
重构渲染管线，新增标签插件支持。
{% endtimeline %}

## Gallery 标签

{% gallery size:m ratio:square %}
![示例图一](https://picsum.photos/400/400?random=1)
![示例图二](https://picsum.photos/400/400?random=2)
![示例图三](https://picsum.photos/400/400?random=3)
{% endgallery %}

## Button 标签

{% button 默认按钮 https://example.com %}

{% button color:blue 蓝色按钮 https://example.com %}

{% button color:green size:xs 小按钮 https://example.com %}

## Checkbox 标签

{% checkbox 未选中项 %}

{% checkbox checked 已选中项 %}

{% checkbox checked:true color:cyan 青色已选 %}

{% checkbox color:orange symbol:plus 加号符号 %}

## Mermaid 标签

{% mermaid %}
graph LR
  A[开始] --> B{判断}
  B -->|是| C[处理]
  B -->|否| D[结束]
  C --> D
{% endmermaid %}

## 语法容错（应原样输出）

{% button 缺少链接 %}

{% checkbox %}

{% tabs %}
未闭合的 tabs 块
