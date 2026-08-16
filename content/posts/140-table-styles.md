---
title: "表格样式与提示块的三种阅读场景"
date: 2026-08-15 11:00:00
excerpt: "集中验证 1.40.0 table 的 scroll、wrap、compact 样式，以及 tip 的标题和正文渲染。"
category: "测试"
topic: "标签插件"
tags: [1.40.0, table, tip, 响应式]
status: published
---

## 横向数据适合 scroll

{% table style:scroll %}
| 浏览器 | 桌面 | 平板 | 手机 |
| --- | ---: | ---: | ---: |
| Safari | 100% | 98% | 96% |
| Chrome | 100% | 99% | 97% |
| Firefox | 99% | 97% | 95% |
{% endtable %}

## 短句适合 wrap

{% table style:wrap %}
| 方案 | 说明 |
| --- | --- |
| scroll | 保持列宽，窄屏横向滚动 |
| wrap | 单元格文字允许换行 |
| compact | 减少内边距，适合密集数据 |
{% endtable %}

## compact 适合对比

{% table style:compact %}
| 状态 | 数量 | 备注 |
| --- | ---: | --- |
| published | 5 | 可在前台访问 |
| draft | 1 | 仅后台可见 |
{% endtable %}

{% tip text:提示块也要保持语义层级 %}
提示标题来自 `text:` 参数，正文仍然是标准 Markdown，可包含 **强调** 和链接。
{% endtip %}
