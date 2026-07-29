---
title: 标签插件四期测试
date: 2026-07-29
excerpt: 测试 Stellar 标签插件四期（vote/friends/sites/radio + 行内标签）的渲染效果
tags: [测试, 插件]
---

## Radio 单选框

与 checkbox 并存：

{% checkbox 普通复选框 %}
{% checkbox checked:true 已勾选复选框 %}
{% radio 普通单选框 %}
{% radio checked 已选中单选框 %}
{% radio color:cyan 青色单选框 %}
{% radio checked color:green 绿色已选单选框 %}

## Vote 投票

默认图标：

{% vote id:tp4-test 这篇文章对你有帮助吗？ %}

无标题、缺省 id：

{% vote %}

## Friends 友链卡片流

缺省走本地 /api/friends 聚合数据：

{% friends %}

## Sites 站点卡片流

api 必填透传（示例外部数据源）：

{% sites api:https://api.vlts.cc/output_data/v2 %}

api 缺失时原样输出（语法容错）：

{% sites %}

## 行内标签

- 下划线：{% u 携带下划线的文本 %}
- 着重号：{% emp 带着重号的文本 %}
- 波浪线：{% wavy 带波浪线的文本 %}
- 删除线：{% del 删除线文本 %}
- 键盘按键：按 {% kbd Ctrl %} + {% kbd C %} 复制
- 密码样式：{% psw 划过显示密码 %}
- 模糊文本：{% blur 划过显示内容 %}
- 上标：X{% sup color:red 2 %}
- 下标：H{% sub 2 %}O

代码块内不应被误伤：

```
{% u 这里保持原样 %}
{% kbd Esc %}
```

行内代码同理：`{% wavy 保持原样 %}`
