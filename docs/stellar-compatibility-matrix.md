# Stellar 兼容性矩阵与实现规范

本文档为 Sakura-Blog 对 Hexo Stellar 主题（以 2.0.0 为干净基线，锁定 commit `9d11978a24fe6610e3a7dbab741cf62634539d05`）的兼容性事实、配置映射、短代码支持全集、动态服务与安全边界定义。

---

## 1. 视觉模式与设计边界

- **纯深色模式（Dark Mode Only）**：
  - 本项目产品定位为纯深色视觉风格，**不提供、不维护浅色模式**。
  - 不新增浅色主题切换开关、浅色专属 CSS 变量或平行样式分支。
  - 上游主题中残留的亮色样式仅作兼容历史占位，不进入验收基线。
- **设计资产与 Token 规范**：
  - 前台与 Console 严格遵循 `resource/apple-design/library-consumption.json` 的推荐设计约束。
  - 复用已定义的语义 token（如 `--text-p1`, `--text-p2`, `--text-p3`, `--card-bg`, `--block-border`），严禁在业务代码中写入随意裸色值。
- **字体与样式治理**：
  - 上游主题源（`resource/hexo-theme-stellar`）保持完全干净且无侵入式修改。
  - 自定义字体规范（如 Menlo/Monaco 等宽字体与搜索输入框 inherit 补丁）收敛至受控样式 `highlight.css` 与 `search.css`，构建通过 `--clean` 从上游纯净源编译并由 `make css-drift-check` 门禁确保零漂移。

---

## 2. 配置兼容映射表

本博客采用 Go 原生架构（Gin + html/template + SQLite），不依赖 Node.js/Hexo 运行时。配置主要分布在应用配置（`config/app.yml`）与站点配置（`config/site.yml`）中。

| 原版 Hexo / Stellar 配置项 | 本项目对应项 | 存储位置 / 机制 | 默认值 / 处理行为 |
| :--- | :--- | :--- | :--- |
| `title` | `title` | `config/site.yml` | 站点主标题 |
| `subtitle` | `subtitle` | `config/site.yml` | 站点副标题 |
| `description` | `description` | `config/site.yml` | 站点描述（用于 meta 与 SEO） |
| `url` | `url` | `config/site.yml` | 生产必须填写的绝对基址（用于 Canonical/OG/Sitemap/Feed） |
| `avatar` | `avatar` | `config/site.yml` | 博主/站点主头像 URL |
| `style.animated_avatar.background` | `avatar_bg` | `config/site.yml` | 头像悬浮旋转背景图 |
| `menu` | `menubar.items` | `config/site.yml` | 桌面与移动端主导航菜单，支持 icon 与主题色 |
| `widgets` (rightbar) | `rightbar` | `config/site.yml` | 右侧边栏小组件（包含 welcome_linklist 等） |
| `stellar.wiki` | `wiki` | `config/site.yml` | 文库项目列表；支持 Hero、Galaxy 动效、仓库卡片绑定 |
| `stellar.friends` | `friends` | `config/site.yml` | 友链抓取刷新周期与单链 RSS 提取上限 |
| 友链分组数据 (`_data/links.yml`) | LinkGroup / LinkItem | SQLite `settings` 表 | 友链分组管理；支持通过 Console 可视化增删改查 |
| `stellar.comments` | `comments` | `config/site.yml` + Go API | 自研轻量评论引擎，支持审核、点赞、Boost 情绪胶囊 |
| `server.port` | `listen_addr` | `config/app.yml` | 监听端口（默认 `:8080`），可通过 `BLOG_LISTEN` 覆盖 |
| `hexo-generator-searchdb` | 全文搜索 | Go + SQLite FTS5 | 原生 SQLite FTS5 全文索引，毫秒级响应，无需静态 JSON 导出 |
| 多主题切换器 / 浅色配置 | *已弃用/忽略* | 无 | 强制纯深色模式，忽略上游相关配置 |
| 外部评论服务 (Disqus/Valine/Gitalk) | *已弃用/忽略* | 无 | 统一收敛至内部安全评论引擎 |

---

### 2.1 Stellar 2.0 前台适配边界

- 页面采用 `site-shell`、`site-workspace`、`site-region`、`brand` 与 `listing-nav`；文章状态由 `body[data-page-layout][data-article-style]` 表达。
- 构建配置使用上游 `appearance/tags/features/article.listing`。站点已有 `style/plugins/site_tree` 等保存接口保留，由 Go 适配器输出新版 CSS/DOM；不要求修改真实站点配置或数据库。
- 原生 ESM runtime 挂载本地 shell/search 扩展，保留上游区域注册、取消与卸载机制。Go FTS/静态搜索索引、自建评论和既有动态服务沿原接口运行。
- 搜索位于品牌区按钮，使用原生 dialog；Ctrl/Command+K 可在桌面与窄屏打开。Wiki 搜索仍限定当前项目，RAG 问答能力保留其配置开关。
- 本项目沿用 card 深色外观；不新增上游的浅色切换、访客外观设置页、Hexo CLI/Doctor、Front Matter 严格 Schema 或 Hexo 图片构建管线。完整 SSR 导航保留，未启用上游 `partial_navigation`，避免跳过自建服务的初始化。
- `about`、块级 `tip`、旧分享链接等本地既有内容能力继续兼容；下方 shortcode 表描述本项目真实实现，不代表完全复制上游 2.0 标签全集。
- CSS 从锁定 git commit 编译；共享 `resource/hexo-theme-stellar` 的 HEAD 无需切换到该 tag。ESM 入口版本覆盖 runtime 全部 JS 内容，依赖变化会一起使浏览器缓存失效。

## 3. 标签短代码（Shortcodes）支持清单

本项目共支持 **40+** 种短代码标签，涵盖提示标注、布局容器、文艺排版、媒体播放、数据图表与 GitHub 集成。各标签均经过严格的参数模式提取、XSS 防护与 Console 可视化双向往返映射。

### 3.1 提示标注类 (Inline Annotations)

| 标签名称 | 语法形式 | 参数列表 (Role) | 渲染策略 | 说明 |
| :--- | :--- | :--- | :--- | :--- |
| `note` | 行内 / 伪块 | `color` (named), `title` (pos), `content` (content) | 静态直出 | 彩色提示横幅，支持 8 种主题色 |
| `quot` | 行内 | `el` (named), `icon` (named), `prefix` (named), `suffix` (named), `text` (named) | 静态直出 | 引述标题段，支持自定义前后缀图标 |
| `mark` | 行内 | `text` (pos), `color` (named) | 静态直出 | 荧光标记文本 |
| `button` | 行内 | `text` (pos), `url` (pos), `icon` (named), `color` (named), `size` (named) | 静态直出 | 独立跳转按钮，校验 URL scheme |
| `checkbox` | 行内 | `text` (pos), `checked` (flag), `color` (named), `symbol` (named) | 静态直出 | 任务清单复选框 |
| `radio` | 行内 | `text` (pos), `checked` (flag), `color` (named), `symbol` (named) | 静态直出 | 单选框项 |
| `hashtag` | 行内 | `text` (pos), `href` (pos), `color` (named) | 静态直出 | 标签胶囊 |
| `emoji` | 行内 | `source` (pos), `name` (pos), `url` (named), `height` (named) | 静态直出 | 内置表情库（QQ/贴吧/阿鲁/Twemoji）或外部图片 |
| `icon` | 行内 | `key` (pos), `color` (named), `style` (named), `text` (pos) | 静态直出 | 安全内联 SVG 图标 |

### 3.2 布局容器类 (Containers & Layouts)

| 标签名称 | 语法形式 | 参数列表 (Role) | 渲染策略 | 说明 |
| :--- | :--- | :--- | :--- | :--- |
| `folding` | 块级 (`{% endfolding %}`) | `color` (named), `child` (named), `open` (flag), `title` (pos), `content` (content) | 静态直出 | 折叠面板；`child` 仅放行 `codeblock` / `iframe` |
| `tabs` | 块级 (`{% endtabs %}`) | `active` (named), `align` (named), `content` (content) | 交互容器 | 选项卡容器，以 `<!-- tab 标题 -->` 分隔 |
| `timeline` | 块级 (`{% endtimeline %}`) | `api` (named), `user` (named), `limit` (named), `hide` (named), `content` (content) | 静态/服务 | 时间轴；填入 `api` 时走客户端数据服务渲染 |
| `dropdown` | 块级 (`{% enddropdown %}`) | `direction` (named), `align` (named), `open` (flag), `title` (pos), `content` (content) | 交互容器 | 下拉菜单；**内联项图标接入统一注册表白名单直出 SVG** |
| `gallery` | 块级 (`{% endgallery %}`) | `layout` (named), `size` (named), `ratio` (named), `content` (content) | 交互画廊 | 相册画廊容器，每行一张 Markdown 图片 |
| `folders` | 块级 (`{% endfolders %}`) | `color` (named), `content` (content) | 静态直出 | 多折叠项分组，以 `<!-- folder 标题 -->` 分隔 |
| `grid` | 块级 (`{% endgrid %}`) | `bg` (named), `w` (named), `c` (named), `gap` (named), `br` (named), `content` (content) | 静态直出 | 响应式栅格，以 `<!-- cell -->` 分隔 |
| `table` | 块级 (`{% endtable %}`) | `style` (named), `content` (content) | 静态直出 | 增强表格容器（scroll/wrap/compact） |
| `tip` | 块级 (`{% endtip %}`) | `text` (named), `content` (content) | 静态直出 | 提示容器，支持旧版位置参数兼容 |
| `blockquote` | 块级 (`{% endblockquote %}`) | `indent` (named), `content` (content) | 静态直出 | 增强引用块 |
| `banner` | 块级 (`{% endbanner %}`) | `title` (named), `subtitle` (named), `bg` (named), `avatar` (named), `link` (named), `content` (content) | 静态直出 | 横幅卡片，校验 URL 与图片 scheme |
| `navbar` | 混合 (单行/块级) | `active` (named), `content` (content) | 静态/交互 | 页面导航条 |
| `frame` | 混合 (单行/块级) | `device` (named), `width` (named), `content` (content) | 静态直出 | 设备模型外框（iPhone/iPad/MacBook 等） |
| `box` | 块级 (`{% endbox %}`) | `color` (named), `child` (named), `title` (pos), `content` (content) | 静态直出 | 彩色带标题卡片容器 |
| `about` | 块级 (`{% endabout %}`) | `avatar` (named), `border` (named), `height` (named), `back` (flag), `content` (content) | 静态直出 | 关于卡片 |
| `toc` | 行内 | `title` (pos), `wiki` (named), `open` (named), `display` (named) | 静态直出 | **目录外壳或 Wiki 文库全树直出（详见 3.5）** |

### 3.3 文艺装饰类 (Typography & Accents)

| 标签名称 | 语法形式 | 参数列表 (Role) | 渲染策略 | 说明 |
| :--- | :--- | :--- | :--- | :--- |
| `mbti` | 行内 | `type` (pos), `scores` (pos), `url` (named), `image` (named) | 静态直出 | MBTI 人格五维进度可视化 |
| `poetry` | 块级 (`{% endpoetry %}`) | `title` (pos), `author` (named), `date` (named), `footer` (named), `content` (content) | 静态直出 | 诗歌排版 |
| `paper` | 块级 (`{% endpaper %}`) | `style` (named), `title` (named), `author` (named), `date` (named), `footer` (named), `content` (content) | 静态直出 | 论文/手稿排版（段落分隔标记） |
| `reel` | 块级 (`{% endreel %}`) | `title` (pos), `author` (named), `date` (named), `footer` (named), `content` (content) | 静态直出 | 卷轴风格排版 |
| `psw` / `emp` / `wavy` / `blur` | 行内 | `text` (pos) | 静态直出 | 密码隐匿/重点强调/波浪下划线/模糊隐藏文字 |
| `sub` / `sup` | 行内 | `text` (pos), `color` (named) | 静态直出 | 彩色上标与下标 |

### 3.4 媒体播放与服务类 (Media & Dynamic Services)

| 标签名称 | 语法形式 | 参数列表 (Role) | 渲染策略 | 说明 |
| :--- | :--- | :--- | :--- | :--- |
| `link` | 行内 | `url` (pos), `title` (pos), `desc` (named), `icon` (named) | 静态直出 | 外部站点/链接卡片 |
| `video` | 行内 | `src` (pos), `bilibili` (named), `youtube` (named), `type` (named), `ratio` (named), `width` (named), `autoplay` (flag) | 静态直出 | 本地或第三方视频播放器 |
| `audio` | 行内 | `src` (pos), `netease` (named), `type` (named), `autoplay` (flag) | 静态直出 | 本地或网易云音乐播放器 |
| `swiper` | 块级 (`{% endswiper %}`) | `width` (named), `effect` (named), `content` (content) | 交互画廊 | Swiper 轮播图 |
| `albums` | 行内 | `group` (pos/named), `repo` (named), `api` (named), `size` (named) | 静态直出/服务 | **方形相册卡片直出（详见 3.5）** |
| `posters` | 行内 | `group` (pos/named), `repo` (named), `api` (named), `size` (named) | 静态直出/服务 | **竖版海报卡片直出（详见 3.5）** |
| `copy` | 行内 | `text` (pos), `git` (named), `prefix` (named) | 静态直出 | 一键复制文本块 |
| `rating` | 行内 | `title` (pos), `id` (named), `icon` (named) | 客户端服务 | 文章打分卡片，本地去重与服务端沉淀 |
| `vote` | 行内 | `title` (pos), `id` (named), `yes` (named), `no` (named) | 客户端服务 | 文章赞成/反对投票卡片 |
| `friends` | 行内 | `api` (named), `posts` (flag) | 客户端服务 | 友链 RSS 动态聚合列表 |
| `links` | 行内 | 无参数 | 静态直出 | 全部友链分组卡片 |
| `moments` | 行内 | `limit` (named) | 客户端服务 | 即时动态说说列表 |
| `sites` | 行内 | `api` (named) | 客户端服务 | 站点导航流 |
| `md` | 行内 | `src` (pos), `wrap` (named) | 外部引用 | 动态载入外部 Markdown |

### 3.5 阶段 E 特性对齐与修复细节

1. **`toc` Wiki 项目全树序直出**：
   - 语法：`{% toc wiki:<project_id> [标题] %}`。
   - 机制：通过注入的 `WikiTreeProvider` 仅拉取已发布（`published`）的文档，杜绝暴露草稿；递归先序遍历生成目录树。
   - DOM 结构：完全对齐上游 `toc.js` 与 `main.css`（`<details class="toc"><summary>...</summary><div class="body fs14 multi">...</div></details>`）。
   - 容错语义：若项目不存在或公开文档为 0，线上渲染对齐上游 Hexo 行为输出空字符串；Console 编辑预览模式则输出安全诊断提示。
2. **`albums` 与 `posters` 友链分组卡片直出**：
   - 语法：`{% albums [分组名] [size:s|m|l|xl] %}` 或 `{% posters [分组名] [size:xs|s|m|l|xl] %}`。
   - 优先级：`api > repo > group`。指定 `api` 时渲染客户端容器；指定 `group` 时服务端直接解析友链并直出 HTML，无需额外客户端请求。
   - 同名组合并：按数据库中同名组的顺序稳定合并条目。
   - 封面回退链路：`Item.Snapshot -> Item.Avatar -> defaultCover`，无任何可用封面时丢弃该卡片。
   - 卡片 DOM：生成 `class="grid-cell album-card"`（方形）与 `class="grid-cell poster-card"`（竖直）。
3. **`dropdown` 安全内联图标**：
   - 语法：内容项支持 `- [标题](链接)` 或 `- icon:ri:github-line [标题](链接)`。
   - 安全：通过 `safeIconKey` 校验后调用图标注册表 `icons.Get` 输出原生内联 SVG，未知图标安全降级为占位符，杜绝任何外部未清洗 HTML。
4. **可视化属性编辑往返安全**：
   - Console 属性表单严格依据 Catalog Schema 解析。
   - 未知参数（如非法命名参数）直接拒绝并提示作者回退源码模式，严禁自动静默剔除或修改 Markdown 原始内容。

---

## 4. 客户端动态服务与安全边界

### 4.1 客户端动态服务清单

1. **评论系统 (Comments)**：
   - 接口：`GET /api/comments?post_id=...`, `POST /api/comments`, `POST /api/comments/like`, `POST /api/comments/boost`。
   - 客户端行为：支持匿名模式、登录态自动识别、待审核提示、表情面板、本地记忆。
2. **友链 RSS 聚合 (Friends RSS)**：
   - 接口：`GET /api/friends`。
   - 客户端行为：拉取最新 RSS 文章列表；失联友链自动降级置底且禁用外部跳转。
3. **互动反馈 (Rating & Vote)**：
   - 接口：`POST /api/rating`, `POST /api/vote`。
   - 客户端行为：`localStorage` 隔离记录投票历史，防刷拦截，网络异常时乐观计数回滚。
4. **视觉动效 (Galaxy / Swiper / ScrollReveal)**：
   - 针对 `prefers-reduced-motion` 媒体特性提供强制降级；WebGL 初始化失败安全静默。

### 4.2 安全与防护策略

- **URL 与协议白名单**：
  - 所有涉及跳转或外部加载的属性（`href`, `src`, `bg` 等）均经过 `safeURL` 校验。
  - 严禁 `javascript:`, `data:` (非受控), `vbscript:` 等伪协议，非法值一律丢弃或降级。
- **HTML 与属性转义**：
  - 服务端直出的所有动态文本（标题、描述、昵称、标签）统一经过 `html.EscapeString`。
  - CSS 相关属性（`width`, `gap`, `color`）严格限制在已知设计系统枚举内，丢弃包含分号、花括号或表达式的非法值，防御 CSS Injection。
- **权限与数据隔离**：
  - 普通作者仅能查看和修改自己或共同署名的文章；管理端与公开前台严格隔离。
  - 草稿（`draft`）状态的文章与 Wiki 文档严禁在公开前台的任何接口、短代码或目录树中泄露。

---

## 5. 派生内容与缓存失效策略

当数据发生变动时，系统依据以下策略对派生数据和缓存进行失效与刷新：

| 变动触发源 | 影响的派生内容 | 失效与刷新机制 |
| :--- | :--- | :--- |
| **文章发布 / 更新 / 撤回** | - Markdown 短代码派生渲染结果<br>- SQLite FTS5 全文索引<br>- Sitemap / RSS Feed / 归档计数 | - 写入数据库后即时更新 FTS5 索引<br>- 重新请求页面时按新版本重新渲染并更新渲染缓存<br>- 页面元数据（Title/OG）即时更新 |
| **Wiki 文档新增 / 修改 / 状态变更** | - 文库全局文档树结构<br>- `{% toc wiki:project %}` 直出结果<br>- 文库文档侧边栏导航 | - `store.BuildDocTreeFromDocs` 查询已发布文档，变更即刻反映在后续渲染<br>- 草稿切换为已发布后即时纳入树，反之即刻移出 |
| **友链管理 (LinkGroup / LinkItem 增删改)** | - `/friends` 页面分组展示<br>- `{% albums %}` / `{% posters %}` 分组直出卡片<br>- 后台友链 RSS 抓取目标 | - 友链分组缓存即时失效<br>- 下一次访问含有对应分组的 shortcode 页面时重新拉取最新数据库记录直出卡片<br>- 后台 RSS 抓取轮询任务重新载入活动友链列表 |
| **站点配置 (`site.yml`) 重新保存** | - 导航栏 (Menubar)<br>- 侧边栏 (Rightbar)<br>- Wiki 项目列表与 Hero 配置 | - 服务端重新加载 `SiteConfig` 单例，后续 HTTP 请求统一使用最新内存配置 |
