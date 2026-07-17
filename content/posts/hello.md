---
title: Demo 2：Go 服务端渲染验证
date: 2026-07-16
excerpt: 用 Gin + html/template + goldmark 组合出完整文章页，验证从 Markdown 到 Stellar 风格文章页的端到端链路。
tags: [Go, Gin, Stellar]
categories: [技术]
---

## 这是什么

这是 **Demo 2** 的验证文章。页面由 Go 后端服务端渲染：[Gin](https://gin-gonic.com) 路由 + `html/template` 模板 + `goldmark` 把这份 Markdown 渲染成 HTML，再套上从 Stellar 主题编译出的 CSS。

如果你能看到三列布局、彩色菜单图标、正确的文章排版和代码高亮，说明整条链路打通了。

## Markdown 元素验证

### 标题与段落

上面是二级标题，这是三级标题。段落之间留空行即可。Stellar 的 `.md-text` 选择器会给这些元素套上文章样式。

### 代码块

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/yuin/goldmark"
)

func main() {
    r := gin.Default()
    r.GET("/post/:slug", postHandler)
    r.Run(":8080")
}
```

行内代码：`goldmark.New()`、`template.FuncMap`。

### 引用

> 这是引用块，验证 `blockquote` 样式。
>
> 多行引用，第二段。

### 列表

无序列表：

- 普通列表项一
- 普通列表项二
  - 嵌套子项
  - 嵌套子项

有序列表：

1. 第一步：`go mod init`
2. 第二步：引入 Gin 和 goldmark
3. 第三步：写模板和路由

### 文本样式

**加粗**、*斜体*、`行内代码`、[超链接](https://example.com)、删除线~~旧方案~~。

### 图片

![占位图](https://gcore.jsdelivr.net/gh/cdn-x/placeholder@1.0.12/common/wallhaven-rdgw7w.webp)

## 下一步

- Demo 3：管理后台（SQLite + 文章 CRUD + Markdown 编辑器）
- 接入 Artalk 评论
- 文章列表页与归档
