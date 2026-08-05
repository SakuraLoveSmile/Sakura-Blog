---
title: "标签插件全量测试（express / data / container）"
date: 2026-08-06 10:00:00
category: "测试"
tags: [标签插件, 测试]
excerpt: "合并一至五期全部测试用例，按 Stellar 官方文档三大分类逐项验证：express 表达类（emoji/icon/vote/rating/mark/hashtag/image/blockquote/quot/poetry/paper/reel/note/link/button/okr/copy/radio/checkbox/audio/video/chat/navbar/frame/about/mermaid/文本修饰/快捷语法）、data 数据类（timeline/friends/sites/albums/posters/md/ghcard/toc）、container 容器类（box/folding/folders/tabs/grid/banner/gallery/swiper）。"
status: draft
---

官方文档：[express](https://xaoxuu.com/wiki/stellar/tag-plugins/express/) · [data](https://xaoxuu.com/wiki/stellar/tag-plugins/data/) · [container](https://xaoxuu.com/wiki/stellar/tag-plugins/container/)

# 一、表达类标签（express）

## emoji 表情包

内置 default（qq）与 twemoji 两个表情源：

{% emoji aini %} {% emoji aini %} {% emoji twemoji 1f600 %} {% emoji twemoji 1f389 %}

指定高度：

{% emoji aini height:1em %} {% emoji aini height:2em %} {% emoji aini height:3em %} {% emoji aini height:2em %} {% emoji aini height:1em %}

## icon 图标标签

icons.yml 中配置的图标（支持指定颜色）：

{% icon solar:planet-bold-duotone %} {% icon solar:hashtag-square-bold color:red %} {% icon solar:documents-bold-duotone color:green %} {% icon default:rss color:orange %} {% icon github:star color:purple %}

## vote 投票

{% vote id:default Stellar 是最好的博客主题吗？ %}

自定义图标（yes/no）：

{% vote id:default yes:solar:chat-square-like-bold-duotone no:solar:documents-bold-duotone 自定义图标投票 %}

## rating 评分

{% rating id:default 给 Stellar 五星好评吧～ %}

## mark 标记标签

支持多彩标记：{% mark 默认 %} {% mark 红 color:red %} {% mark 橙 color:orange %} {% mark 黄 color:yellow %} {% mark 绿 color:green %} {% mark 青 color:cyan %} {% mark 蓝 color:blue %} {% mark 紫 color:purple %} {% mark 亮 color:light %} {% mark 暗 color:dark %} {% mark 警告 color:warning %} {% mark 错误 color:error %}

## hashtag 标签

未指定颜色时循环取色：

{% hashtag Stellar https://xaoxuu.com/wiki/stellar/ %} {% hashtag Hexo https://hexo.io/ %} {% hashtag GitHub https://github.com/ %} {% hashtag Gitea color:green %}

## image 图片标签

限宽 + 填充 + 动态背景色（竖图优化）：

{% image /assets/img/icon-512.png 站点图标 width:200px padding:16px bg:var(--card) %}

带描述的普通图片：

{% image /assets/img/icon-512.png 这是一张带描述的图片 %}

## blockquote 段落引用

> 这是使用 "> 引用" 写法的例子

{% blockquote %}
这是使用 blockquote 标签的例子，始终表现为非技术文章的引号样式，支持 **Markdown** 正文。
{% endblockquote %}

{% blockquote indent:true %}
带首行缩进的引用块，段落首行缩进 2 字符宽度。
{% endblockquote %}

## quot 强调引用

适合居中且醒目的引用：

{% quot Stellar 是迄今为止最好用的主题 %}

加上 el:h2/h3/h4/h5/h6 可以作为标题使用：

{% quot el:h3 text:三级小标题（h3） %}

{% quot el:h5 text:五级小标题（h5） %}

## poetry 诗词

{% poetry 游山西村 author:陆游 footer:诗词节选 %}
莫笑农家腊酒浑，
丰年留客足鸡豚。
**山重水复疑无路，柳暗花明又一村。**
箫鼓追随春社近，
衣冠简朴古风存。
从今若许闲乘月，
拄杖无时夜叩门。
{% endpoetry %}

## paper 纸张标签

{% paper style:underline title:文言文 author:诸葛亮 date:三国 footer:节选 %}
<!-- line left -->
出师表
<!-- paragraph -->
先帝创业未半而中道崩殂，今天下三分，益州疲弊，此诚危急存亡之秋也。
<!-- line right -->
后出师表
<!-- paragraph -->
先帝深虑汉、贼不两立，王业不偏安，故托臣以讨贼也。
{% endpaper %}

## reel 卷轴标签

{% reel 滕王阁序 author:王勃 date:重九日 footer:节选 %}
时维九月，序属三秋。潦水尽而寒潭清，烟光凝而暮山紫。
俨骖騑于上路，访风景于崇阿。临帝子之长洲，得天人之旧馆。
{% endreel %}

## note 备注块

{% note 这是一个默认样式的提示 %}

带标题（第一个空格前是标题，空格需用 &nbsp; 代替）：

{% note 这&nbsp;是标题 这是正文 哈哈。 %}

彩色备注块（12 种颜色）：

{% note color:cyan 一共支持 12 种颜色，可以满足几乎所有的需求。 %}

{% note color:warning 这是一条警告提示。 %}

{% note color:error 这是一条错误提示。 %}

## link 链接卡片

{% link https://xaoxuu.com/wiki/stellar/ Stellar 主题文档 icon:/assets/img/icon-512.png %}

带摘要的样式：

{% link https://xaoxuu.com/ xaoxuu 的主页 icon:/assets/img/icon-512.png desc:true %}

## button 按钮

{% button 探索 https://github.com/xaoxuu/hexo-theme-stellar/ icon:solar:planet-bold-duotone %}

指定颜色与最小号尺寸：

{% button 源码 https://github.com/xaoxuu/hexo-theme-stellar/ icon:github:logo color:green size:xs %}

## okr 目标管理

{% okr o1 %}
2026 年的小目标：完成标签插件与官方文档全量对齐
<!-- okr kr1 percent:100 -->
完成 express 表达类标签对齐
- 当 {% mark KR %} 进度为 100% 时，标签默认显示为 {% mark color:green 已完成 %}
- 当 {% mark O %} 未设置进度时，则显示所有 {% mark KR %} 进度平均值
<!-- okr kr2 percent:60 status:at_risk -->
完成 data 数据类标签对齐
<!-- okr kr3 -->
完成 container 容器类标签对齐（未设置进度默认 0%）
{% endokr %}

## copy 复制行

{% copy curl -s https://sh.xaox.cc/install prefix:$ %}

git 快捷写法：

{% copy git:https xaoxuu.com/hexo-theme-stellar %}

{% copy git:ssh xaoxuu.com/hexo-theme-stellar %}

{% copy git:gh xaoxuu.com/hexo-theme-stellar %}

## radio 单选

{% radio 没有勾选的单选框 %}
{% radio checked:true 已勾选的单选框 %}
{% radio checked:true color:red 红色的已勾选单选框 %}

## checkbox 复选

{% checkbox 普通的没有勾选的复选框 %}
{% checkbox checked:true 普通的已勾选的复选框 %}
{% checkbox symbol:plus color:green checked:true 显示为加号的绿色的已勾选的复选框 %}
{% checkbox symbol:minus color:yellow checked:true 显示为减号的黄色的已勾选的复选框 %}
{% checkbox symbol:times color:red checked:true 显示为乘号的红色的已勾选的复选框 %}

## audio 音频标签

外链 mp3：

{% audio https://github.com/volantis-x/volantis-docs/releases/download/assets/Lumia1020.mp3 %}

网易云歌曲 ID：

{% audio netease:1397345903 %}

原生 audio：

{% audio https://example.com/sample.mp3 %}

## video 视频标签

B 站嵌入：

{% video bilibili:BV1GP4y1d729 %}

YouTube 嵌入：

{% video youtube:dQw4w9WgXcQ %}

原生 video 标签：

{% video https://example.com/sample.mp4 %}

## chat 聊天标签

{% chat iphone style:qq title:单聊示例 me:bob %}
alice:
  name: 小明
  avatar: /assets/img/icon-512.png
bob:
  name: 小红
  avatar: /assets/img/icon-512.png
<!-- chattip 上午 10:00 -->
<!-- chatcell user:alice md:内置 qq 和微信风格聊天标签。 -->
<!-- chatcell user:bob md:支持单聊、群聊、多媒体消息。 -->
{% endchat %}

## navbar 导航栏

{% navbar active:/posts/ %}
[首页](/)
[文章](/posts/)
[归档](/archives/)
[友链](/friends/)
{% endnavbar %}

## frame 设备框架

块级写法（device 参数）：

{% frame device:iphone width:320px %}
![screenshot](/assets/img/demo-phone.png)
{% endframe %}

{% frame device:macbook %}
![screenshot](/assets/img/demo-desktop.png)
{% endframe %}

单行写法（img 参数）：

{% frame iphone img:/assets/img/demo-phone.png %}

## about 关于卡片

{% about avatar:/assets/img/icon-192.png height:80px %}
**你好**，这是 about 卡片正文，支持 Markdown。
{% endabout %}

## mermaid 流程图

{% mermaid %}
graph LR
  A[开始] --> B{判断}
  B -->|是| C[处理]
  B -->|否| D[结束]
  C --> D
{% endmermaid %}

## 文本修饰标签集

- 这是 {% blur 高斯模糊 %} 标签
- 这是 {% psw 密码 %} 标签
- 这是 {% u 下划线 %} 标签
- 这是 {% emp 着重号 %} 标签
- 这是 {% wavy 波浪线 %} 标签
- 这是 {% del 删除线 %} 标签
- 这是 {% sup 上角标 color:red %} 标签
- 这是 {% sub 下角标 %} 标签
- 这是 {% kbd 键盘样式 %} 标签，试一试：{% kbd ⌘ %} + {% kbd D %}

代码块内不应被误伤：

```
{% u 这里保持原样 %}
{% kbd Esc %}
```

行内代码同理：`{% wavy 保持原样 %}`

## 快捷语法

[[双中括号下划线]] 与 {red|颜色文字}、{#3498db|十六进制颜色文字}：

这是一段包含 [[下划线文字]] 和 {red|红色文字} 以及 {#3498db|蓝色文字} 的文本。

# 二、数据类标签（data）

## timeline 时间线（静态）

{% timeline %}
<!-- node 2026 年 7 月 18 日 -->
启动标签插件一期：note / folding / tabs / timeline / gallery / button / checkbox / mermaid。
<!-- node 2026 年 8 月 2 日 -->
完成排版核心与装饰标签（Spec 48 / 49 / 51）。
![](/assets/img/icon-512.png)
<!-- node 2026 年 8 月 6 日 -->
按官方三大分类（express / data / container）重新组织验证文章。
{% endtimeline %}

## friends 友链

缺省指向本地 /api/friends 聚合端点：

{% friends %}

## sites 网站卡片

api 必填，数据源格式与友链相同：

{% sites api:https://raw.githubusercontent.com/xaoxuu/friends/output/v2/data.json %}

## albums 专辑

{% albums size:m %}

## posters 海报

{% posters size:s %}

## md 渲染外部 markdown 文件

{% folding 点击展开 Stellar README %}
{% md https://gcore.jsdelivr.net/gh/xaoxuu/hexo-theme-stellar/README.md %}
{% endfolding %}

## ghcard 卡片

{% ghcard xaoxuu %}

{% ghcard xaoxuu/hexo-theme-stellar %}

## toc 文档目录树

{% toc 本文目录 open:true %}

wiki 文档树目录（wiki: 参数）：

{% toc wiki:stellar %}

# 三、容器类标签（container）

## box 盒子容器

{% box 普通盒子标题 %}
盒子正文，支持 **Markdown**。note 标签就是使用 box 容器实现的，它们样式相同。
{% endbox %}

### 彩色代码块（child:codeblock）

{% box 推荐的写法 child:codeblock color:green %}
```go
func test() {
    // ...
}
```
{% endbox %}

{% box 不推荐的写法 child:codeblock color:red %}
```go
func test() -> () {
    // ...
}
```
{% endbox %}

### 嵌套 tabs 标签

{% box 嵌套示例 %}
{% tabs align:left %}
<!-- tab 图文混排 -->
![](/assets/img/icon-512.png)

盒子里嵌套 tabs 也可以正常渲染。
<!-- tab 示例代码 -->
```go
fmt.Println("hello box")
```
{% endtabs %}
{% endbox %}

## folding 折叠容器

{% folding 默认收起的折叠框 %}
这里是折叠框的内容，支持 **Markdown**。
{% endfolding %}

{% folding open:true 默认打开的折叠框 %}
通过 open:true 使其默认展开。
{% endfolding %}

{% folding color:yellow 彩色折叠框 %}
通过设置颜色，以实现更醒目的作用，但不要滥用色彩哦～
{% endfolding %}

{% folding color:red open:true 危险，请不要打开这个 %}
最后一次警告，千万不要打开这个。
{% endfolding %}

## folders 多个折叠容器聚合

{% folders %}
<!-- folder 题目1 -->
这是答案1
<!-- folder 题目2 -->
这是答案2
<!-- folder 题目3 -->
这是答案3
{% endfolders %}

{% folders color:blue %}
<!-- folder 第一章：环境搭建 -->
安装 Go 工具链与 SQLite 驱动。
<!-- folder 第二章：核心逻辑 -->
实现 shortcode 正则解析与渲染。
{% endfolders %}

## tabs 分栏容器

{% tabs active:2 %}
<!-- tab 图片 -->
![](/assets/img/icon-512.png)
<!-- tab 代码块 -->
```go
let x = 123
fmt.Println("hello world")
```
<!-- tab 表格 -->
| a | b | c |
| --- | --- | --- |
| a1 | b1 | c1 |
| a2 | b2 | c2 |
{% endtabs %}

align 对齐参数：

{% tabs align:center %}
<!-- tab 居中一 -->
居中对齐的第一个面板
<!-- tab 居中二 -->
居中对齐的第二个面板
{% endtabs %}

## grid 网格分区容器

### 动态列数（默认最小宽度 240px）

{% grid %}
<!-- cell -->
**cell 1**：动态列数，随页面宽度自适应。
<!-- cell -->
**cell 2**：大于 480px 显示 2 列。
<!-- cell -->
**cell 3**：大于 720px 显示 3 列。
{% endgrid %}

### 固定列数 c:2

{% grid c:2 gap:16px %}
<!-- cell -->
**左列**：支持多段落 Markdown。

第二段落。
<!-- cell -->
**右列**：同样支持多段落。

- 列表项 A
- 列表项 B
{% endgrid %}

### 背景样式 bg:box / bg:card

{% grid bg:box w:150px %}
<!-- cell -->
cell 1
<!-- cell -->
cell 2
<!-- cell -->
cell 3
<!-- cell -->
cell 4
{% endgrid %}

{% grid bg:card w:150px br:4px %}
<!-- cell -->
cell 1
<!-- cell -->
cell 2
<!-- cell -->
cell 3
<!-- cell -->
cell 4
{% endgrid %}

### 设置间距与圆角

{% grid bg:card gap:32px br:12px w:120px %}
<!-- cell -->
cell 1
<!-- cell -->
cell 2
<!-- cell -->
cell 3
{% endgrid %}

## banner 横幅容器

用于独立页面顶部（背景图 + navbar）：

{% banner 随记 bg:/assets/img/icon-512.png %}
{% navbar active:/notes/ [随记](/notes/) [收藏](/bookmark/) %}
{% endbanner %}

用于用户个人资料页（avatar）：

{% banner 某某 这是个人简介 avatar:/assets/img/icon-512.png bg:/assets/img/icon-512.png %}
{% endbanner %}

用作文章摘要卡片（link 整块可点击）：

{% banner 博客进阶：自动化部署 本文讲了如何利用脚本和 GitHub Actions 简化博客搭建和部署流程。 bg:/assets/img/icon-512.png link:/posts/ %}
{% endbanner %}

## gallery 图库

默认瀑布流：

{% gallery %}
![](/assets/img/icon-512.png)
![](/assets/img/icon-512.png)
![](/assets/img/icon-512.png)
![](/assets/img/icon-512.png)
{% endgallery %}

size:m + ratio:square：

{% gallery size:m ratio:square %}
![示例图一](/assets/img/icon-512.png)
![示例图二](/assets/img/icon-512.png)
![示例图三](/assets/img/icon-512.png)
{% endgallery %}

## swiper 轮播容器

默认一张图片 50% 宽度：

{% swiper %}
![](/assets/img/icon-512.png)
![](/assets/img/icon-512.png)
![](/assets/img/icon-512.png)
![](/assets/img/icon-512.png)
{% endswiper %}

width:min（25% 宽度）与切换效果：

{% swiper width:min %}
![](/assets/img/icon-512.png)
![](/assets/img/icon-512.png)
![](/assets/img/icon-512.png)
{% endswiper %}

{% swiper effect:cards %}
![](/assets/img/icon-512.png)
![](/assets/img/icon-512.png)
![](/assets/img/icon-512.png)
{% endswiper %}

# 四、语法容错（应原样输出）

残缺/非法写法不应报错，保持原文展示：

{% button 缺少链接 %}

{% checkbox %}

{% sites %}

{% tabs %}
未闭合的 tabs 块
