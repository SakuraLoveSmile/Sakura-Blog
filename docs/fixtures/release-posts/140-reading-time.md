---
title: "1.40.0 阅读时长与内容密度"
date: 2026-08-15 09:00:00
excerpt: "用一篇结构完整的长文验证阅读时长、文章元信息、目录与代码块在服务端渲染中的表现。"
category: "产品体验"
topic: "Stellar 1.40.0"
tags: [Stellar, 1.40.0, 阅读时长, 长文]
status: published
poster_headline: "阅读体验升级"
poster_caption: "reading time · metadata · outline"
---

# 阅读时长应该帮助读者做什么

阅读时长不是为了制造压力，而是给读者一个轻量的预期。它应该与正文长度同步更新，文章修改后不需要手工维护。

## 从内容计算时长

服务端会从 Markdown 正文计算词数，并在文章页渲染分钟数。中文、英文、代码和列表都应该参与计算，但 front matter 不应被算入正文。

```go
func readingMinutes(words int, wordsPerMinute int) int {
    if wordsPerMinute <= 0 {
        wordsPerMinute = 300
    }
    minutes := (words + wordsPerMinute - 1) / wordsPerMinute
    if minutes < 1 {
        return 1
    }
    return minutes
}
```

## 长文中的层级

- 文章标题只出现一次。
- 二级标题进入目录，三级标题用于分组。
- 列表、引用和代码块保持原来的语义。

> 好的元信息应该安静地工作，让读者把注意力放在内容上。

## 一段英文与公式

The quick brown fox jumps over the lazy dog. This sentence keeps mixed-language word counting visible in the test fixture.

行内公式 $E = mc^2$ 与展示公式都不应破坏文章布局：

$$\int_0^1 x^2\,dx = \frac{1}{3}$$

## 验收清单

1. 文章卡片显示标题、摘要和标签。
2. 文章页显示阅读时长且数值大于零。
3. 移动端正文不会撑破内容容器。
