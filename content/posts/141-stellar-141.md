---
title: "Stellar 1.41.0 同步验收"
date: 2026-08-16 10:00:00
excerpt: "验收 1.41.0 同步：图标语义化命名空间、文章末尾标签行、AI 成分标签、搜索章节化、表格滚动容器与平滑滚动。"
category: "平台建设"
topic: "Stellar 1.41.0"
tags: [Stellar, 1.41.0, 搜索, 标签, 验收]
status: published
cover: /assets/img/icon-512.png
ai_label: generated
pinned: true
---

## 本文验收点

本文用于 1.41.0 同步验收，各章节对应一个特性。**文章底部会自动渲染本页标签行**（article.tags 默认开启），**面包屑行右侧显示橙色「由 AI 生成」标签**（frontmatter `ai_label: generated`）。

### 如何核对

逐项对照下面的章节，确认对应组件已按 1.41 行为渲染。

## 图标命名空间

1.41 将图标键统一为语义化命名空间，`solar:*`/`ph:`/`bxs:` 旧前缀全部改名。本文使用新键：

- 文档类：{% icon default:documents %} `default:documents`
- 文库类：{% icon example:notebook %} `example:notebook`
- 星球类：{% icon example:planet %} `example:planet`
- 标签类：{% icon quot:hashtag %} `quot:hashtag`
- 分类类：{% icon default:category %} `default:category`

### 图标缺失检查

本页任何图标都不应渲染成 `icon-missing` 占位（旧键经兼容表映射也能正常显示）。

## 文章末尾标签行

正文结束后、版权页脚之前，应渲染居中排列的标签胶囊（本页标签：Stellar / 1.41.0 / 搜索 / 标签 / 验收）。每个胶囊带 `#` 图标，hover 时主题色高亮，链接指向对应标签归档页。

### 关闭开关

Console 设置页「文章末尾显示标签」开关可关闭该行。

## AI 成分标签

本页 frontmatter 写了 `ai_label: generated`，面包屑行最右（阅读时长右侧）应显示橙色「由 AI 生成」，前置盾牌警告图标。未写该字段的历史文章不渲染任何内容。

| 档位 | 文案 | 颜色 |
|------|------|------|
| manual | 本文完全由人类完成 | 蓝 |
| reviewed | 已 AI 审核 | 绿 |
| polished | 已 AI 润色 | 绿 |
| generated | 由 AI 生成 | 橙 |

### 多档对照

各档文案与颜色由语言包与 `article.ai_label` 配置提供，本文仅演示 generated 档。

## 本地搜索章节化

在侧栏搜索框输入「章节化」，本页命中内容应按标题章节拆分展示：

- 结果条目显示所属章节名（如「本地搜索章节化」）
- 点击结果跳转到 `?kw=关键词#锚点`，目标页用黄色 mark 高亮关键词并定位到章节

### 搜索高亮

跳转后正文中的「章节化」字样会包上 `<mark class="tag-plugin colorful mark">` 高亮。

### 懒加载与缓存

搜索数据首次聚焦搜索框才加载（lazy_load），localStorage 缓存 1 天（cache_ttl）。

## 表格滚动容器

普通 Markdown 表格由渲染端包上 `.md-table-scroll` 容器：宽度足够时铺满、内容超出时横向滚动，带圆角边框。上方「AI 成分标签」表格即属此类。

| 列 A | 列 B | 列 C | 列 D | 列 E |
|------|------|------|------|------|
| 值 1 | 值 2 | 值 3 | 值 4 | 值 5 |
| 值 6 | 值 7 | 值 8 | 值 9 | 值 10 |

### 标签表格除外

`{% table %}` 标签渲染的表格自带布局（.tag-plugin.table），不会被再次包装。

## 平滑滚动与吸顶

点击右侧 TOC 或页内锚点链接时平滑滚动（300-600ms 缓动），列表页导航栏吸顶时切换玻璃效果。

### 吸顶玻璃

滚动首页观察导航栏：吸顶后 `.navbar-blur` 获得 `pinned` 类，恢复毛玻璃观感。

## 结语

以上即 1.41.0 同步的主要验收面。感谢阅读。
