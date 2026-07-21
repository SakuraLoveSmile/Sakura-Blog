# Spec 16 · Stellar 标签插件二期（高频 9 件套）

- **分支**：`feature/tag-plugins-2` · **Wave 4** · **隔离等级**：高
- **主模块（独占）**：`internal/post/shortcode.go` + `assets/css/tag-plugins.css`

## Why

Spec 06 已落地 10 个标签插件（note/folding/tabs/timeline/gallery/mermaid/button/checkbox/quot/okr），但日常写作最高频的一批仍缺失：**link 链接卡片**（原版辨识度最高的插件）、**image 增强图片**、**folders 折叠组**、**grid 多列布局**，以及 `inline-labels` 系的内联小组件（box/mark/emoji/icon/hashtag）。本期补齐这 9 个，全部落在现有 `processShortcodes` 预处理管线，与其它 spec 零耦合。

## 功能需求（语法对照原版 `scripts/tags/lib/*.js`）

| 插件 | 语法 | 说明 |
|---|---|---|
| link | `{% link url [title] [desc:true/false] [icon:src] %}` | 站外链接卡片：图标 + 标题 + 描述 + 域名 |
| image | `{% image src [alt] [width:400px] [bg:#eee] [download:true/url] [fancybox:true/false] [ratio] %}` | 带说明/尺寸/下载按钮的图片，接入现有 Fancybox 灯箱 |
| folders | `{% folders [color] %}` + `<!-- folder 标题 -->` 分隔 + `{% endfolders %}` | 折叠面板组（folding 的分组版） |
| grid | `{% grid [bg:box/card] [w:240px] [c:2] [gap:16px] [br:12px] %}` + `<!-- cell -->` 分隔 + `{% endgrid %}` | 多列网格容器，cell 内嵌 Markdown |
| box | `{% box [color] [child:codeblock/tabs] title %} body {% endbox %}` | 彩色盒子（复用 `.note` colorful 结构） |
| mark | `{% mark text [color:color] %}` | 内联荧光标记 `<mark class="tag-plugin colorful mark">` |
| emoji | `{% emoji [source] name [height:1.75em] %}` | CDN 表情图（source 映射内置，对照原版 `_config.yml` 的 `tag_plugins.emoji`） |
| icon | `{% icon key [color:color] [style:css] %}` | 内联图标，key 走现有 `icons.yml` 体系 |
| hashtag | `{% hashtag text [url] [color] %}` | 彩色话题标签链接 |

## What Changes

- `internal/post/shortcode.go`：新增 9 个正则 + render 函数，接入 `processShortcodes`。
  - 块级（folders/grid/box）先于行内处理；body 用 `renderMD` 闭包渲染（与 folding/tabs 同款）。
  - 行内（link/image/mark/emoji/icon/hashtag）在块级之后统一替换。
- `assets/css/tag-plugins.css`：**仅补 main.css 确无的类**。原版 `.tag-plugin.link` / `.tag-plugin.image` / `.folders` / `.grid` / `.colorful` 系列大概率已在 main.css（Stellar 全量样式），动手前逐个 grep。
- `internal/template/partial/head.html`：**预计零改动**（tag-plugins.css 的 link 已在 Spec 06 引入）。

## 实现步骤

1. 逐个阅读 `../resource/hexo-theme-stellar/scripts/tags/lib/{link,image,folders,grid,box,mark,emoji,icon,hashtag}.js`，记录**精确的输出 HTML 结构与 class/属性名**（尤其 `colorful` 的 `color=` 属性写法）。
2. `grep assets/css/main.css` 确认每个插件的类是否已存在，列出真正缺失的样式清单。
3. 先实现块级三件（folders/grid/box）：正则 + 分隔符切分（`<!-- folder xxx -->` / `<!-- cell -->`）+ renderMD 子渲染。
4. 再实现行内六件。icon 复用 `processShortcodes` 已注入的 `iconFunc`；emoji 内置 source→CDN URL 前缀映射表（取原版默认配置）；link 卡片 title 缺省取 URL hostname、icon 缺省取 `https://{host}/favicon.ico`（原版 siteinfo API 不引入）。
5. image 的 fancybox 参数输出 `data-fancybox` 属性，对齐 reading-ux 已有灯箱初始化选择器（注意现有选择器限定 `p>img`，image 插件产出的结构需确认能被选中，必要时扩选择器但**不得**放开到链接内图片）。
6. 补缺失样式到 `tag-plugins.css`（变量色、亮暗双轨，禁止写死颜色）。
7. 写覆盖全部 9 插件的测试文 `content/posts/tag-plugins-2-test.md`，`-seed` 重导验证。

## 技术决策（无需再问）

- **不引外部 API**：原版 link 卡片靠 `data_services.siteinfo` 自动抓标题/图标，本项目纯静态降级：写全参数则所见即所得，缺省项用 hostname/favicon 兜底。
- **emoji source 表**：硬编码原版默认源（如 blobcat 等），不做配置化；未知 source 原样输出。
- **语法容错**：参数缺失/未闭合原样输出，不 panic、不吞正文（与 Spec 06 同标准）。
- **seed 依赖**：ContentHTML 是预渲染缓存，新插件生效需 `-seed` 重导或 Console 重存文章。Console 编辑器分屏预览走 `/api/v1/preview/markdown`（同一渲染管线），应自动生效。

## Impact

- **修改**：`internal/post/shortcode.go`、`assets/css/tag-plugins.css`
- **不影响**：路由、store、api、console、模板结构

## ADDED Requirements

### Requirement: 高频标签插件二期
The system SHALL render link/image/folders/grid/box/mark/emoji/icon/hashtag tag plugins from markdown into Stellar-aligned HTML.

#### Scenario: link 卡片
- **WHEN** 文章含 `{% link https://example.com 标题 icon:xx.png %}`
- **THEN** 输出 Stellar `.tag-plugin.link` 卡片结构，点击新窗口打开，观感与原版一致

#### Scenario: grid 多列
- **WHEN** 文章含 `{% grid c:2 %}` 与两个 `<!-- cell -->`
- **THEN** 输出两列网格，cell 内 Markdown 正常渲染，窄屏自动降列

#### Scenario: 内联组件
- **WHEN** 正文行内使用 mark/emoji/icon/hashtag
- **THEN** 输出内联元素不破坏段落流，颜色走主题变量、亮暗模式均正常

#### Scenario: 语法容错
- **WHEN** 插件参数缺失或块未闭合
- **THEN** 原样输出文本，整篇其余内容渲染不受影响

## 验证

1. `go build ./... && go vet ./...` 通过；shortcode 单测覆盖 9 个插件的正常/容错分支
2. 测试文 `-seed` 后逐个与 xaoxuu.com 文档页目视对齐（结构、间距、圆角、色板）
3. 亮/暗模式、PJAX 导航后 image 灯箱可用、样式不闪
4. Console 编辑器分屏预览中 9 个插件同样渲染
5. 故意写坏 link 与 grid 各一处，页面其余正常

## Out of Scope

- 中低频插件（video/audio/swiper/copy/poetry/rating/chat/ghcard → Wave 5 的 Spec 20）
- 已明确放弃项：vote/about/navbar/md/sites/albums/posters/frame
- link 卡片的站点信息自动抓取服务
