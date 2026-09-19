---
title: Stellar 1.44.0 新特性验收
date: 2026-08-29 19:55:00
excerpt: 用于验收 Stellar 1.44.0 Card Hover、Wiki Hero Galaxy、搜索快捷键与基础内容渲染的测试文章。
category: 产品体验
tags: [Stellar, 1.44.0, Card Hover, Galaxy, 搜索]
topic: Stellar 验收
status: draft
poster_headline: Stellar 1.44.0
poster_topic: 新特性验收
poster_caption: Card Hover、Galaxy 与搜索体验综合测试
poster_color: "#2E8DFF"
---

# 这是一篇 1.44.0 验收文章

本文是前台 Stellar 1.44.0 的验收样本，默认保持草稿，不应出现在公开首页。通过草稿预览或本地开发站点检查最终 DOM 与交互。

## Card Hover 卡片

下面的 link 与 grid 卡片会输出 1.44 的组合类；开启「卡片 Hover 光斑与倾斜」后，在桌面精细指针下应看到 Spotlight 和轻微 Tilt。

{% link https://github.com/xaoxuu/hexo-theme-stellar Stellar 主题 desc:上游 1.44.0 主题仓库 icon:https://github.com/xaoxuu.png %}

{% grid bg:card c:2 gap:12px %}
<!-- cell -->
### Spotlight

鼠标移入卡片时光斑跟随指针，键盘聚焦时保持居中光斑。
<!-- cell -->
### Touch fallback

窄屏或 `prefers-reduced-motion` 下不启用 Tilt，内容仍保持可读。
{% endgrid %}

## Wiki Hero Galaxy

Wiki 项目配置使用新版结构：

```yaml
hero:
  background: /assets/img/hero-cover.jpg
  animation:
    type: galaxy
    params:
      density: 2
      speed: 0.5
      transparent: true
```

图片与 Galaxy 可以叠加；WebGL、脚本或动态效果被禁用时，Hero 标题、按钮和图片背景仍应保留。

## 搜索快捷键与语义

桌面布局按 `Command+K`（macOS）或 `Ctrl+K`（Windows/Linux）应聚焦搜索框。输入框使用 `type="search"`，外层使用 `role="search"`；在编辑区、输入法组合和窄屏菜单状态下不应抢占快捷键。

## 代码、表格与长文本

```go
func stellar144Ready(version string) bool {
    return version == "1.44.0"
}
```

| 验收项 | 期望结果 |
| --- | --- |
| 文章卡片与 link/grid | 含 `card-hover` 组合类，插件关闭时静态样式不变 |
| Wiki Hero | 图片、Galaxy、叠加和 WebGL 失败四种状态均可降级 |
| 搜索控件 | `role="search"`、`type="search"`、`Command/Ctrl+K` 正常 |
| 移动端 | `390×844` 无 Tilt、无横向溢出 |

这段长文本用于检查正文宽度、代码块滚动和表格在窄屏下的横向滚动行为。验收完成后保留草稿，避免把测试内容误发布到生产站点。
