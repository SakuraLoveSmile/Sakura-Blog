---
title: "排版核心验证（行号/grid/hashtag/blockquote + Spec 49 展示容器 + Spec 51 装饰标签）"
date: 2026-08-02 16:00:00
category: "测试"
tags: [标签插件, 排版, 测试]
excerpt: "Spec 48：代码块行号、grid 网格、hashtag 标签、blockquote 引用；Spec 49：swiper/banner/quot/chat/navbar/frame 展示容器；Spec 51：poetry/paper/reel 文艺卡片、psw/emp/wavy/blur/sub 行内装饰、folders/albums/posters 数据容器。"
status: draft
---

## 代码块行号（多语言）

### Go 代码块

```go
package main

import "fmt"

func main() {
    fmt.Println("Hello, 行号!")
    for i := 0; i < 10; i++ {
        fmt.Println(i)
    }
}
```

### JavaScript 代码块

```javascript
function fibonacci(n) {
  if (n <= 1) return n;
  return fibonacci(n - 1) + fibonacci(n - 2);
}

const result = fibonacci(10);
console.log(result);
```

### Python 代码块

```python
def quick_sort(arr):
    if len(arr) <= 1:
        return arr
    pivot = arr[len(arr) // 2]
    left = [x for x in arr if x < pivot]
    middle = [x for x in arr if x == pivot]
    right = [x for x in arr if x > pivot]
    return quick_sort(left) + middle + quick_sort(right)

print(quick_sort([3, 6, 8, 10, 1, 2, 1]))
```

### 短代码块（单行）

```bash
echo "只有一行代码，行号 1"
```

### 长代码块（多行，验证行号连续）

```python
# 行号 1
def step_by_step():
    # 行号 2
    step1 = "准备工作"
    # 行号 3
    step2 = "执行任务"
    # 行号 4
    step3 = "检查结果"
    # 行号 5
    step4 = "清理资源"
    # 行号 6
    step5 = "生成报告"
    # 行号 7
    step6 = "发送通知"
    # 行号 8
    step7 = "记录日志"
    # 行号 9
    step8 = "完成回调"
    # 行号 10
    return "done"
    # 行号 11
    # 行号 12
    # 行号 13
    # 行号 14
    # 行号 15
    # 行号 16
    # 行号 17
    # 行号 18
    # 行号 19
    # 行号 20
```

### 行内代码（不应有行号）

这是 `inline code` 片段，这里 `fmt.Println("hello")` 也是行内代码，它们**不应**显示行号。

## Grid 网格容器

### 基础两列网格

{% grid c:2 gap:16px %}
<!-- cell -->
**左列**：支持多段落 Markdown。

第二段落。
<!-- cell -->
**右列**：同样支持多段落。

- 列表项 A
- 列表项 B
{% endgrid %}

### 带背景的三列网格（box）

{% grid bg:box c:3 gap:12px br:8px %}
<!-- cell -->
Box 列 1

内容区域。
<!-- cell -->
Box 列 2

内容区域。
<!-- cell -->
Box 列 3

内容区域。
{% endgrid %}

### 卡片背景网格（card）

{% grid bg:card c:2 gap:16px br:12px %}
<!-- cell -->
卡片列 1

带阴影和 hover 上浮效果。
<!-- cell -->
卡片列 2

带阴影和 hover 上浮效果。
{% endgrid %}

### 动态列宽网格

{% grid w:200px gap:8px %}
<!-- cell -->
动态列 1
<!-- cell -->
动态列 2
<!-- cell -->
动态列 3
<!-- cell -->
动态列 4
{% endgrid %}

## Hashtag 彩色标签

### 默认颜色轮询

{% hashtag 日常 %} {% hashtag 技术 %} {% hashtag 生活 %} {% hashtag 学习 %} {% hashtag 开源 %} {% hashtag 旅行 %} {% hashtag 美食 %}

### 指定颜色

{% hashtag 红色标签 color:red %} {% hashtag 橙色标签 color:orange %} {% hashtag 黄色标签 color:yellow %}

{% hashtag 绿色标签 color:green %} {% hashtag 青色标签 color:cyan %} {% hashtag 蓝色标签 color:blue %} {% hashtag 紫色标签 color:purple %}

### 带链接的标签

{% hashtag Go语言 /tags/go color:blue %} {% hashtag 前端开发 /tags/frontend color:green %}

## Blockquote 增强引用

### 基础引用

{% blockquote %}
这是基础增强引用，显示引号图标。
{% endblockquote %}

### 首行缩进引用

{% blockquote indent:true %}
这是一个带首行缩进的引用块。段落首行会缩进 2 字符宽度，对齐原版 Stellar 的 blockquote 样式。支持多段落 Markdown 内容。
{% endblockquote %}

### 含 Markdown 格式的引用

{% blockquote %}
这行包含 **粗体**、*斜体* 和 `行内代码`。
{% endblockquote %}
## Swiper 轮播容器（Spec 49）

### 基础轮播

{% swiper %}
![](https://picsum.photos/600/340?1)
![](https://picsum.photos/600/340?2)
![](https://picsum.photos/600/340?3)
![](https://picsum.photos/600/340?4)
{% endswiper %}

### width:min 迷你轮播（slide 25% 宽）

{% swiper width:min %}
![](https://picsum.photos/600/340?11)
![](https://picsum.photos/600/340?12)
![](https://picsum.photos/600/340?13)
{% endswiper %}

### width:max 限宽轮播（容器最大 800px）

{% swiper width:max %}
![](https://picsum.photos/600/340?21)
![](https://picsum.photos/600/340?22)
![](https://picsum.photos/600/340?23)
{% endswiper %}

### coverflow 效果

{% swiper effect:coverflow %}
![](https://picsum.photos/600/340?31)
![](https://picsum.photos/600/340?32)
![](https://picsum.photos/600/340?33)
{% endswiper %}

### cards 效果

{% swiper effect:cards %}
![](https://picsum.photos/600/340?41)
![](https://picsum.photos/600/340?42)
![](https://picsum.photos/600/340?43)
{% endswiper %}

## Banner 横幅容器

### 完整横幅（背景图 + 头像 + 标题/副标题 + 整块链接）

{% banner title:我的博客 subtitle:记录技术成长 bg:https://picsum.photos/1200/400 avatar:https://picsum.photos/80/80 link:https://example.com %}
欢迎来到我的小站，支持 **Markdown** 正文。
{% endbanner %}

### 极简横幅（仅标题，无背景图）

{% banner title:纯文字横幅 %}
没有背景图时使用卡片底色，正文正常渲染。
{% endbanner %}

## Quot 强调引用（增强：h5/h6）

{% quot el:h5 text:五级小标题（h5） %}

{% quot el:h6 text:六级小标题（h6） %}

原有 h2/h3/h4 继续可用：{% quot el:h3 text:三级小标题（h3） %}

## Chat 聊天气泡（增强）

### 群聊模式 scene:group（全员左对齐，me 不生效）

{% chat iphone style:qq title:技术交流群 scene:group me:bob %}
alice:
  name: 小明
  avatar: https://picsum.photos/80/80?1
bob:
  name: 小红
  avatar: https://picsum.photos/80/80?2
<!-- chattip 上午 10:00 -->
<!-- chatcell user:alice md:大家好！群聊模式所有消息左对齐。 -->
<!-- chatcell user:bob md:是的，即使设置了 me 参数也不会靠右。 -->
{% endchat %}

### 多媒体 cell 占位（type 未实现时显示占位框）

{% chat iphone style:qq title:文件传输群 scene:group %}
alice:
  name: 小明
  avatar: https://picsum.photos/80/80?1
bob:
  name: 小红
  avatar: https://picsum.photos/80/80?2
<!-- chattip 上午 10:05 -->
<!-- chatcell user:alice md:发个文件给你 -->
<!-- chatcell type:image -->
<!-- chatcell type:video -->
{% endchat %}

## Navbar 文章内导航

### 基础导航（active 项高亮）

{% navbar active:首页 %}
[首页](https://example.com/)
[关于](https://example.com/about)
[作品](https://example.com/works)
[友链](https://example.com/friends)
{% endnavbar %}

## Frame 设备框架

### iPhone 手机壳

{% frame device:iphone width:320px %}
![screenshot](https://picsum.photos/600/1200)
{% endframe %}

### iPad 平板壳

{% frame device:ipad %}
![screenshot](https://picsum.photos/800/600)
{% endframe %}

### MacBook 笔记本壳

{% frame device:macbook %}
![screenshot](https://picsum.photos/800/500)
{% endframe %}

## Poetry 诗词卡片（Spec 51）

### 全参数诗词卡

{% poetry 静夜思 author:李白 date:唐 footer:录自《唐诗三百首》 %}
床前明月光，
疑是地上霜。
举头望明月，
低头思故乡。
{% endpoetry %}

### 极简诗词卡（无标题/元信息）

{% poetry %}
落霞与孤鹜齐飞，
秋水共长天一色。
{% endpoetry %}

## Paper 纸张卡片（Spec 51）

### 基础信纸（paragraph/line 标记）

{% paper title:一封家书 author:张三 date:2026-07-01 %}
<!-- paragraph -->
见字如面，近来可好？
<!-- paragraph -->
家中一切安好，勿念。
<!-- line right -->
此致敬礼
{% endpaper %}

### 带 section 分节与 style:underline

{% paper style:underline title:学习笔记 author:李四 %}
<!-- section 第一章 -->
Go 语言并发模型基于 goroutine 与 channel。
<!-- section 第二章 -->
SQLite 用 WAL 模式提升并发读写性能。
{% endpaper %}

## Reel 卷轴卡片（Spec 51）

{% reel 观影手记 author:阿甘 date:2026-07 footer:完 %}
第 1 帧：开场长镜头扫过小镇。
第 2 帧：主角踏上旅程。
第 3 帧：主题曲响起。
{% endreel %}

## 行内装饰标签（Spec 51）

### psw 密码遮罩（悬停显示）

这里藏着 {% psw 密码内容 %}，鼠标悬停即可显示明文。

### emp 着重强调（下划线+粗体）

这段话里 {% emp 重点强调 %} 一下，其余保持普通样式。

### wavy 波浪线

这里用 {% wavy 波浪线 %} 标注可疑内容。

### blur 模糊（悬停清晰）

剧透警告：{% blur 结局大反转 %}，不想被剧透请别悬停。

### sub/sup 上下角标

化学式 H{% sub 2 %}O，面积 m{% sup 2 %}，彩色下标 X{% sub color:red 1 %}。

## Folders 折叠文件夹组（Spec 51）

### 双组折叠（color 配色）

{% folders color:blue %}
<!-- folder 第一章：环境搭建 -->
安装 Go 工具链与 SQLite 驱动。
<!-- folder 第二章：核心逻辑 -->
实现 shortcode 正则解析与渲染。
{% endfolders %}

### 无配色折叠（默认样式）

{% folders %}
<!-- folder 归档 2025 -->
去年写的读书笔记。
<!-- folder 归档 2026 -->
今年的技术文章。
{% endfolders %}

## Albums / Posters 卡片（Spec 51）

### albums 专辑卡片（repo 数据源 + size:m）

{% albums repo:user/repo size:m %}

### posters 海报卡片（api 数据源 + size:s）

{% posters api:https://example.com/posters.json size:s %}
