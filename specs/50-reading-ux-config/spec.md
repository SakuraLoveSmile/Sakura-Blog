# Spec 50 · 阅读体验与配置增强（linklist widget / Story 布局 / 导航选项卡可配 / 背景图 / 评论区懒加载）

- **分支**：`feature/frontend-gap` · **Wave 10** · **隔离等级**：中
- **主模块**：`config/site.yml` + `internal/template/`（widgets/partials）+ `assets/js/` + `assets/css/`
- **前置**：Spec 33（widget 分派器 dispatch.html）已合；Spec 34（style 配置段）已合

## Why

阅读体验是博客粘性的核心。当前项目已具备基础排版，但对照原版 Stellar 仍缺 6 项中频体验与配置能力：**linklist 作为独立 widget**（目前仅 welcome 实例内嵌，不可独立配置到侧栏）、**Story 布局模式**（tech 紧凑模式已实现，story 图文大间距模式缺失）、**导航选项卡可配**（navbar.html 的固定 tab 不能自定义链接/排列）、**网站背景图**（Spec 34 StyleSite 已定义配置结构但缺前端注入与验证）、**侧栏背景图**（Spec 34 StyleLeftbar 同理）、**评论区懒加载**（当前评论区在文章页首屏同步加载，影响 LCP）。本期补齐后阅读体验配置能力达原版 90%。

## 功能需求

| 功能 | 配置键 / 语法 | 行为 |
|---|---|---|
| linklist widget | widgets library `{layout: linklist, columns: 1, items: [{title, url, icon}]}` | 独立 linklist widget，columns 控制布局：1=列表（图标+文字竖排），2=图标+文字网格，>2=仅图标网格；数据由 library 参数 items 提供 |
| Story 布局模式 | `site.yml article.type: story`（当前值 `tech`） | 文章列表页切换为 story 布局：大间距卡片、大封面、宽标题、时间/分类 meta 改行尾显示。当前 `Page.Type` 字段已存在（tech/story），但 story 缺渲染实现 |
| 导航选项卡可配 | `site.yml nav_tabs` 段增强 | 当前 navbar.html 固定四个 tab（近期发布/分类/标签/归档）+ 可配置额外 tab。增强后：固定 tab 可排序/隐藏/改链接，config 可完全替换固定 tab 列表 |
| 网站背景图 | `site.yml style.site` 段 | 已有 StyleSite 结构（BackgroundImage/BlurPx/BlurSat/BlurBg），补前端注入验证——确保 head.html 的 sitebg CSS 变量正确生效，修复已知 siteblur 遮罩坑点 |
| 侧栏背景图 | `site.yml style.leftbar` 段 | 已有 StyleLeftbar 结构（BackgroundImage/BlurPx/BackgroundOpacity），补前端验证——确保 sidebg 图片显示 + 毛玻璃效果 + 内容文字可读性（`--background-opacity` 控制遮罩透明度） |
| 评论区懒加载 | 无配置键（自动行为） | 使用 IntersectionObserver 监听评论区容器 `.cmt-body`，进入视口后才加载评论数据和交互脚本；SEO 不影响（服务端渲染占位容器） |

## What Changes

### linklist 独立 widget

- `backend/internal/template/partial/widgets/linklist.html`（新文件）：独立 linklist widget 模板，接收 `WidgetInstance` 上下文，参数 `items`（`[]LinkListItem`）、`columns`（int，缺省 1）。布局逻辑：
  - columns=1（含缺省）：竖排列表，每行 icon + title（`<a class="link"><div class="flex">{{icon}}<span>{{title}}</span></div></a>`）
  - columns=2：两列网格，icon + title 同行
  - columns>=3：多列网格，仅 icon（title 作为 tooltip）
  - 对齐原版 `components/linklist.ejs` 的 columns 规则
- `backend/internal/template/partial/widgets/dispatch.html`：+`{{else if eq .Layout "linklist"}}{{template "partial/widgets/linklist" .}}` 分派分支
- `backend/internal/template/model_widgets.go`：确认 `LinkListConfig` 字段完整（Columns int, Items []LinkListItem, AlignClass/ShowText/EffectiveColumns 等计算字段已在现有 model 中，验证 linklist widget 实例化时正确填充）
- `frontend/assets/css/tag-plugins.css` 或 `main.css`：先 grep 确认 `.linklist` 相关样式是否已存在（markdown.html 中已有 `linklist` 类渲染，样式可能在 main.css 中），仅补缺失

### Story 布局模式

- `backend/internal/template/partial/index.html`：当前 `.post-card` 渲染为 tech 紧凑模式。新增 story 分支——当 `.Page.Type` 为 `story` 时（或 `$.Site.Article.Type` 为 `story`——需确认配置读取链路），渲染 story 卡片：
  ```html
  {{if eq $.Site.Article.Type "story"}}
  <div class="post-card-wrap story">
    <a class="post-card post story" href="/post/{{.Slug}}">
      {{if .Cover}}
      <div class="story-cover"><img src="{{.Cover}}"/></div>
      {{end}}
      <article class="md-text">
        <h2 class="post-title">{{.Title}}</h2>
        <div class="excerpt"><p>{{.Excerpt}}</p></div>
        <div class="meta cap">
          <span class="date">{{date .Date "2006年1月2日"}}</span>
          {{if .Category}}<span class="category">{{.Category}}</span>{{end}}
          {{if .Pinned}}<span class="pin">{{icon "default:pin"}}</span>{{end}}
        </div>
      </article>
    </a>
  </div>
  {{else}}
  ...现有 tech 紧凑模式...
  {{end}}
  ```
- `backend/internal/template/partial/author.html`：同步追加 story 分支（与 index.html 同结构）
- `backend/internal/config/site.go`：确认 `DefaultSite()` 中 `Article.Type` 缺省值为 `"tech"`（当前 site.yml 无 article.type 字段时回退 tech）
- `frontend/assets/css/story.css`（新文件）：story 布局样式——大间距卡片（`margin-bottom: 2rem`）、宽封面（`story-cover` 宽高比 16/9）、大标题字号（`1.5rem`）、meta 行在底部右侧。与现有 tech 紧凑样式互不冲突
- `backend/internal/template/layout/base.html` 或 `partial/head.html`：按 `article.type` 动态加载 story.css（仅 story 模式加载，tech 模式下不额外请求）

### 导航选项卡可配（nav_tabs 增强）

- `backend/internal/template/partial/navbar.html`：当前固定四个 tab + 可配 nav_tabs。增强后：
  - 新增 `site.yml nav_tabs.enable_custom` 开关（缺省 `false` = 保持当前固定 tab 行为）
  - `enable_custom: true` 时，固定 tab 列表完全由 `nav_tabs.items` 定义（不再硬编码"近期发布/分类/标签/归档"），每个 item 含 `{title, url, icon, active_rules}`（active_rules 为可选的正则/路径匹配规则，缺省按 URL 精确匹配）
  - 向后兼容：`enable_custom: false` 时行为不变
- `backend/internal/template/model.go`：`NavTabItem` 结构体补 `Icon` 和 `ActiveRules` 字段（可选，用于自定义 tab 的图标与高亮规则）
- `backend/config/site.example.yml`：更新 `nav_tabs` 段示例，展示 `enable_custom` 和 `items` 完整用法
- `backend/internal/config/site.go`：`DefaultSite()` 补 `NavTabs` 缺省值（空列表，`enable_custom` 缺省 false）

### 网站背景图

- `backend/internal/config/site.go` + `site.example.yml`：StyleSite 配置结构已在 Spec 34 中定义（BackgroundImage/BlurPx/BlurSat/BlurBg），本 spec 验证并修复已知问题：
  - 确认 `head.html` 的 `style` 注入块正确读取 `site.Style.Site` 字段（当前 `model_style.go` 的 `siteCSS()` 方法已生成对应 CSS，验证 `head.html` 调用 `{{.Site.Style.InjectCSS}}` 包含该输出）
  - 修复 siteblur 遮罩透明问题（CLAUDE.md 记录的已知坑点：`body>.sitebg .siteblur` 的 `background-color` 在无背景图时设 `transparent`，有背景图时需覆盖为 `blur_bg` 值）
- `backend/internal/template/partial/head.html`：验证/修复 sitebg 样式注入逻辑——无背景图时 siteblur 透明（保持现状），有背景图时 siteblur 显示 `blur_sat` + `blur_px` + `blur_bg`（缺省 `saturate(300%) blur(100px)` + `var(--bg-a75)`）
- `frontend/assets/css/main.css`：确认 `body>.sitebg` 和 `.siteblur` 相关样式存在，仅补缺失变量引用点

### 侧栏背景图

- `backend/internal/config/site.go` + `site.example.yml`：StyleLeftbar 配置结构已在 Spec 34 中定义（BackgroundImage/BlurPx/BackgroundOpacity/BackgroundColorLight/Dark），本 spec 验证：
  - 确认 `leftbarCSS()` 方法在 `InjectCSS()` 中被正确调用（当前 `model_style.go` 的 `InjectCSS()` 包含 `b.WriteString(s.leftbarCSS())`）
  - 确认 head.html 中纯色模式覆盖块（`background: var(--card)`）与图片模式不冲突——图片模式时 `leftbarCSS()` 输出 `.l_left .leftbar-container{background:transparent;}` 覆盖纯色块
- `backend/internal/template/partial/head.html`：验证 sidebg 图片加载后的毛玻璃效果——`--blur-px` 和 `--background-opacity` 变量在 main.css 中是否正确消费（main.css 中 sidebg 的 CSS 使用 `backdrop-filter: blur(var(--blur-px, 100px))` 和遮罩 `opacity: var(--background-opacity, 0.8)` 等变量引用点）
- 若 main.css 中 sidebg 样式的变量引用点缺失，补 `assets/css/main.css` 变量引用

### 评论区懒加载

- `backend/internal/template/partial/comments.html`：评论区容器增加 `data-lazy` 属性标记（或使用 CSS class `.cmt-lazy`），JS 检测进行懒加载。当前结构：
  ```html
  <section class="body cmt-body native" id="cmt-body" data-page-key="..." data-admin-badge="...">
  ```
  改为：
  ```html
  <section class="body cmt-body native cmt-lazy" id="cmt-body" data-page-key="..." data-admin-badge="..." data-lazy>
  ```
- `frontend/assets/js/comments.js`：新增 `initLazyComments()` 函数：
  - 使用 `IntersectionObserver` 监听 `.cmt-lazy` 容器
  - 进入视口时：移除 `data-lazy` 属性、执行原本的评论加载逻辑（获取评论列表 + 绑定编辑器事件）
  - 如果 IntersectionObserver 不可用（旧浏览器），回退立即加载（退化兼容）
  - PJAX 兼容：在 `pjax:complete` 事件中重新注册 observer
  - 注意：`data-page-key` 等属性需在评论区容器上保留，确保懒加载触发后能正确请求数据
- `frontend/assets/css/comments.css`：添加 `.cmt-body.cmt-lazy` 的占位样式（加载中显示占位骨架或「加载评论中…」提示，避免空白闪烁）
- 后端无需改动——评论区 HTML 始终由服务端渲染（`.cmt-body` 容器 + 数据属性），懒加载仅控制前端 JS 的评论列表获取时机

## 技术决策（无需再问）

- **linklist 独立 widget 不依赖 welcome 实例**：当前 linklist 仅存在于 markdown widget 的 welcome 实例中（`markdown.html` 的 `{{if eq .Name "welcome"}}` 分支）。本 spec 创建独立 linklist layout，数据由 library 参数 `items` 提供，可配置到任意页面类型的左右栏
- **Story 布局不新增配置段**：复用现有 `article.type` 字段（tech/story），site.yml 中 `article.type: story` 切换。不新增 `style.article_type` 等重复配置键
- **nav_tabs 增强向后兼容**：`enable_custom: false`（缺省）时行为与当前完全一致，用户无感知；`enable_custom: true` 后固定 tab 完全由配置驱动
- **背景图验证不新增配置**：StyleSite 和 StyleLeftbar 已在 Spec 34 中定义，本 spec 只做验证修复。如果发现 main.css 变量引用点缺失，仅补变量引用，不重写样式
- **评论区懒加载保底**：IntersectionObserver 不可用时退化到立即加载（内联 `<script>` 检测 `window.IntersectionObserver`，不存在时直接执行原有逻辑）
- **评论区懒加载不改变后端渲染**：评论区 HTML 容器始终由服务端渲染（含 `data-page-key` 等数据属性），懒加载仅控制前端 JS 请求评论数据的时机。SEO 不受影响（容器内容为空，爬虫不执行 JS 获取评论列表，这与当前 SPA 获取方式一致）

## Impact

- **新增**：`partial/widgets/linklist.html`、`assets/css/story.css`
- **修改**：`dispatch.html`（linklist 分派）、`index.html`（story 分支）、`author.html`（story 分支）、`navbar.html`（nav_tabs 增强）、`comments.html`（懒加载标记）、`comments.js`（懒加载逻辑）、`comments.css`（占位样式）、`head.html`（背景图验证修复）、`model.go`（NavTabItem 扩展）、`model_widgets.go`（linklist 实例化验证）、`site.go`（缺省值）、`site.example.yml`（示例更新）、`main.css`（变量引用点补缺）
- **不影响**：shortcode 系统、post/page store、api 管理端、console 主体、feed/OG/middleware

## 验证

1. **linklist widget**：widgets library 配置 `{layout: linklist, columns: 2, items: [{title: 主页, url: /, icon: solar:home-bold}]}` 后，侧栏渲染两列图标+文字网格；columns=1 渲染竖排列表；columns=3 渲染仅图标网格
2. **Story 布局**：`site.yml article.type: story` 后首页文章列表切换为大间距图文卡片；`article.type: tech` 回退紧凑模式
3. **nav_tabs 可配**：`enable_custom: true` + `items: [{title: 友链, url: /friends/}]` 后导航栏仅显示"友链"一个 tab；`enable_custom: false` 保持四个固定 tab + 额外可配 tab
4. **网站背景图**：`style.site.background_image: https://...` 后全站出现背景图 + 毛玻璃遮罩；无背景图时行为不变（siteblur 透明）
5. **侧栏背景图**：`style.leftbar.background_image: https://...` 后侧栏出现图片背景 + 模糊效果；文字可读性良好
6. **评论区懒加载**：文章页首屏不加载评论数据，滚动到评论区附近时触发加载，出现评论列表及编辑器；不支持 IntersectionObserver 的浏览器回退立即加载
7. **零配置回归**：全部配置项缺省时，全站与现状逐像素一致（亮/暗双主题目检）
8. **`make test` 全绿**

## Out of Scope

- linklist widget 的图标库选择器（Console 设置 UI——本期仅 site.yml 配置）
- Story 布局的文章级覆盖（单篇文章独立切换 tech/story——本期仅全站切换）
- nav_tabs 的 dropdown 多级菜单、图标支持（本期仅基本链接+高亮规则）
- 评论区懒加载的骨架屏动画（Skeleton——本期仅简单占位文本）
- 背景图的 parallax 滚动视差效果
- 背景图上传与管理（Console 媒体库——本期仅 URL 配置）
- 背景图主题变量在 Console 外观设置页的 UI 编辑（Spec 34 技术决策已说明 Console 设置 UI 另行排期）
