# Stellar 博客项目 - 开发规范

本项目为自建个人博客：Go (Gin) 后端 + Go html/template 服务端渲染 + 从 Stellar Hexo 主题提取的 CSS + 自建管理后台。

> 本文档是与提供方无关的工程坑点知识的 canonical 来源，适用于所有 agent（Qoder / Claude / Trae 等）；按模块的路由入口见 `AGENTS.md` 的「工程坑点知识路由」一节。多 Agent 并行开发流程（分支模型、合并门槛、端口约定、提交规范）以 `AGENTS.md` 为准。

## 第一阶段模块边界

项目现在是根目录单一 `go.mod` 的模块化单体。`cmd/blog` 只装配共享服务并调用 `console.Mount` 与 `stellar.Mount`；Console 的 Go API 和 Vue SPA 位于 `modules/console/`，Stellar 的 SSR 路由与前台资源位于 `modules/stellar/`，共享业务位于 `internal/`。`config/`、`content/` 和 `data/` 的运行时路径保持不变。

## 仓库工作流

### 目录结构（根目录 dev + git worktree）

Blog 根目录是长期保留的 `dev` 工作树；`main` / `release` 只保留在远端，临时 feature worktree 放在 Blog 同级目录：

| 分支 | 工作树路径 | 用途 |
|------|-----------|------|
| `dev` | Blog 根目录 | 日常开发 |
| `main` | `origin/main` | 远端稳定版本 |
| `release` | `origin/release` | 远端发布版本 |

- `.git/`：普通仓库元数据与 worktree 管理信息
- `resource/`：参考资源（hexo-theme-stellar 主题克隆），已 gitignore，不入版本控制
- `.claude/`：本地工具配置（launch.json），不入版本控制

### worktree 操作注意

- **git 命令在 Blog 根目录执行**，根目录当前分支为 `dev`；提交前仍须执行 `make pre-commit-check`
- 新 feature worktree 建在 Blog 同级目录，例如 `git worktree add ../Blog-feat-name -b feature/name`，合入后立即移除
- `make merge-check` 默认从 `git worktree list` 自动定位根目录的 `dev`，非标准路径可传 `DEV_WORKTREE=<路径>`
- `resource/` 通过当前 worktree 根的相对路径引用；sibling feature worktree 应链接到 Blog 根目录的 `resource/`

### 提交规范

提交规范的 canonical 来源见 `AGENTS.md` 的「提交规范」一节，本文档不另行定义。

## Stellar CSS 剥离规范

### 编译链路（已验证）

Stellar 的 Stylus 源码用 `hexo-config()` 读取 Hexo 配置，脱离 Hexo 编译会报 undefined。本项目用 **shim 注入方案**，不改任何 .styl 源文件：

- `modules/stellar/web/build/stylus/config.json`：从 `_config.yml` 提取 `style`/`article`/`menubar`/`comments`/`plugins`/`tag_plugins` 六段配置
- `modules/stellar/web/build/stylus/shim.js`：用 Stylus JS API 的 `define('hexo-config', fn)` 注入函数，按点路径从 config.json 取值
- `modules/stellar/web/build/stylus/compile.js`：编译 `resource/hexo-theme-stellar/source/css/main.styl` → `modules/stellar/web/assets/css/main.css`
- stylus 版本锁定 `0.62.0`（与 hexo-renderer-stylus@3.0.1 依赖的 `^0.62.0` 一致）

### shim 返回值类型（关键）

Stylus `define()` 接收 JS 原生值自动转换，但类型必须精确，否则条件编译出错：

| Stellar 调用形式 | shim 返回 JS 类型 | Stylus 转换结果 |
|---|---|---|
| `convert(hexo-config('style.font-size.body'))` | `string` | nodes.String，`convert()` 转字面量 |
| `if hexo-config('plugins.swiper.enable')` | `boolean` | nodes.Boolean，`if` 正确判真假 |
| `index(hexo-config('comments.custom_css'), 'artalk')` | `Array` | nodes.List |
| 缺失 key | `null` | nodes.Null（falsy，复现 Hexo 行为） |

**坑**：返回字符串 `"false"` 在 Stylus `if` 里会判真，布尔判断必须返回真正的 `Boolean`。

### 条件 import 配置

`_plugins/index.styl` 用 hexo-config 条件加载插件/评论样式。`config.json` 必须包含：
- `plugins.swiper.enable` / `plugins.fancybox.enable` / `plugins.copycode.enable` / `plugins.katex.enable` 等
- `comments.service`（设为 `"artalk"` 才会编译 artalk 评论样式）
- `comments.custom_css`（数组，如 `["artalk"]`）

### 站点背景图（深色模式层次感）

`style.site.background-image` 默认空（`_config.yml` 注释掉），此时深色背景是纯色 `hsla(210 8% 12%)`，显得"平"。原版 demo 站启用了背景图，靠 `.sitebg`（背景图层）+ `.siteblur`（`--bg-a75` 半透明遮罩 + blur 模糊）叠加出层次感。

要还原原版观感，在 `config.json` 的 `style.site.background-image` 设值（如 `url(...site-bg1.webp)`）。编译后 `body>.sitebg` 会有 `background-image`，`.siteblur` 自动叠遮罩（深色黑遮罩、亮色白遮罩）。

### 资源版本化：asset_ver 内容哈希（无需手工管版本戳）

模板对 `/assets` 下所有 css/js 的引用统一写 `?v={{asset_ver "<path>"}}`，`asset_ver` 模板函数在启动时按文件内容计算哈希（`modules/stellar/web/assetver.go`，`SetAssetVersioner` 注入；未注入返回空串）。内容一变 URL 自动变，配合 /assets 的 no-cache 协商缓存中间件（见下文 Step 2）保证新鲜，**不需要手工 bump 任何版本戳**。历史备注：demo 阶段曾在 preview.html 手工改 `?v=demo1` 强刷，该做法已被 asset_ver 取代。

### 菜单图标机制

Stellar 菜单项的 `icon` 字段（iconify 格式，如 `solar:documents-bold-duotone`）经 icon helper 在服务端内联成 `<svg>`（SVG 定义在 `resource/hexo-theme-stellar/_data/icons.yml`）。`menu.styl` 对 `img/svg` 设 `height:28px`，普通项 `filter: grayscale(0.5) brightness(0.8) opacity(0.5)`，active/hover `filter: unset` 还原原色。Go 侧用内联 SVG。

### 图标颜色坑：必须内联 SVG + inline color，不能用 `<img src>`

Solar duotone 图标的 path 用 `fill="currentColor"`，颜色来自父元素的 `color`。

- **`<img src="*.svg">` 方式不行**：img 加载的 SVG 在隔离环境渲染，`currentColor` 解析为初始值（黑），**不继承宿主 CSS 的 `color`**。深色模式下黑图标融进黑背景，表现为"图标没颜色"。`filter: grayscale/brightness` 也无法恢复本就没有的颜色。
- **必须内联 `<svg>`**：inline SVG 才能从父 `a.nav-item` 继承 `color`。`menu.styl` 的 `.nav-item svg { filter: ... }` + active `filter: unset` 才能正常压暗/还原。
- **Go html/template 侧**：菜单图标要走 icon helper 内联 SVG，不要输出 `<img src>`。SVG 源可从 `resource/hexo-theme-stellar/_data/icons.yml` 取（已含 solar 全套 duotone），或调 iconify API 预取缓存。

### 菜单图标"彩色"机制：每项一个 theme 色

原版菜单每个图标是**不同颜色**（青/绿/橙/红），不是统一色。机制链：

1. `_config.yml` 的 `menubar.items` 每项有 `theme` 字段（如 `#1BCDFC`），注释明确"高亮时的颜色，仅 svg 中 fill='currentColor' 时有效"
2. `menu.ejs` 渲染时 `if (item.theme) el += ' style="color:${item.theme}"'`，把颜色作为 **inline style** 设到 `<a class="nav-item">` 上
3. inline style 优先级（1,0,0,0）高于 `.nav-item { color: var(--text-p3) }` 和 `.nav-item.active { color: var(--text-p1) }`，所以 **inline color 始终主导**，SVG 的 `fill="currentColor"` 解析为该项 theme 色
4. 普通/active 区分靠 `filter`：普通项 `grayscale(0.5) brightness(0.8) opacity(0.5)` 压暗半透明，active 项 `filter: unset` 显示原色

标准配色（`menubar.items` 示例）：博客 `#1BCDFC` 青、文档 `#3DC550` 绿、探索 `#FA6400` 橙、社交 `#F44336` 红。

### 图标标签 `{% icon %}` 的彩色机制（区别于菜单）

文章内的 `{% icon key [color:xxx] %}` 标签走另一套：输出 `<span class="tag-plugin icon colorful" color="accent"><svg>...</svg></span>`，`icon.styl` 里 `span.tag-plugin.icon svg { color: var(--theme) }`，`.colorful[color='accent']` 通过 `x-set-theme-colorful` 把 `--theme` 设成 `$c-accent`。"彩色"是 accent 单色 duotone（实色层 + opacity .5 半透明层），不是彩虹多色。`default_color` 配置留空时与文字同色。

### 验证技巧：SVG 颜色采样

验证内联 SVG 颜色不要用 `XMLSerializer` 序列化后 `drawImage` 采样——脱离父级 `color` 环境后 `currentColor` 会解析回黑，得到假阴性。正确方式：`getComputedStyle(svg.querySelector('path')).fill` 直接读 currentColor 解析后的值；或读 `::after` 的 `background: currentColor`。

### 深色模式背景色

`theme_base.styl` 深色 `--background: x-hsla($c-base-hue, 8%, 12%, 1)` = `hsla(210 8% 12%)` = sRGB **(0.110, 0.120, 0.130)** = 8bit `(28, 31, 33)`。纯色背景本身已正确。

- 若 `style.site.background-image` 设了背景图，`.sitebg` 会叠图 + `.siteblur` 叠 `rgba(0,0,0,0.75)` 黑遮罩 + `saturate(3) blur(100px)`，**实际可见背景就不是纯色**（被背景图 + 遮罩混合）。
- 若要纯色背景（如用户期望 sRGB 0.112/0.122/0.130），把 `config.json` 的 `style.site.background-image` 留空。原版 demo 站启用了背景图造层次感，按需取舍。

### siteblur 黑遮罩坑（无背景图时仍压暗纯色背景）

关掉 `style.site.background-image` 只让 `.sitebg` 没图，但 `.siteblur`（`main.styl:25` 无条件存在）仍调 `blur-effect()` 叠 `background-color: var(--blur-bg)`（默认 `var(--bg-a75)` = 75% 黑）+ `backdrop-filter`。这层黑遮罩会把纯色 `rgb(28,31,33)` 压暗成 `rgb(7,8,8)`（sRGB 0.027/0.030/0.032），导致"背景颜色不对"。

- 合成验证：`rgb(28,31,33)` 上叠 `rgba(0,0,0,0.75)` → `out = 0*0.75 + dst*0.25` ≈ `rgb(7,8,8)`，暗 4 倍
- 修复（Demo 阶段，不改 Stellar 源）：preview.html 加覆写 `body>.sitebg .siteblur { background-color: transparent !important; backdrop-filter: none }`，让 `--background` 原色透出
- Go 正式版思路：`siteblur` 的渲染应与 `$site-background-image` 绑定——无背景图时不输出 siteblur，或 `blur-bg` 设透明。Stellar 原版设计是 siteblur 始终存在（给背景图模糊用），纯色场景需自行屏蔽

## Stellar HTML 结构规范（关键）

### class 名和嵌套层级必须严格对齐 Stellar EJS 模板

**CSS 选择器绑定 Stellar 特定的 class 名和嵌套关系，凭印象写 HTML 会导致样式不生效。** 写 Go html/template 时必须对照 `resource/hexo-theme-stellar/layout/` 下的 EJS 模板。

### 已确认的结构

**主布局骨架**（对应 `layout/layout.ejs`）：
```html
<div class="sitebg"><div class="siteblur"></div></div>
<div id="l_cover">...</div>
<div class="l_body content" id="start" layout="post" type="tech">
  <aside class="l_left">
    <div class="sidebg"></div>
    <div class="leftbar-container">...</div>
  </aside>
  <div class="l_main" id="main">...</div>
  <aside class="l_right">...</aside>
</div>
```

**logo 区**（对应 `_partial/sidebar/logo.ejs`）：
```html
<header class="header">
  <div class="logo-wrap">
    <a class="avatar" href="/">
      <div class="bg" style="opacity:0;background-image:url(...)"></div>
      <img class="avatar" src="...">
    </a>
    <a class="title" href="/">
      <div class="main">标题</div>
      <div class="sub cap">副标题</div>
    </a>
  </div>
</header>
```
- `a.avatar`：48x48 圆形容器（border-radius 48px），`img.avatar` 在内（44x44，margin 2px）

**菜单**（对应 `_partial/sidebar/menu.ejs`）：
```html
<div class="nav-area">
  <nav class="menu dis-select">
    <a class="nav-item active" href="/"><span>首页</span></a>
  </nav>
</div>
```
- 是 `.nav-area > nav.menu > a.nav-item`，不是 `.menu > .menu-item`
- menubar.columns 控制网格列数

**文章内容**：
- 容器用 `<article class="md-text content">`，goldmark 输出的 HTML 包裹在此容器内即可，Stellar 的 `.md-text` 选择器会应用文章样式

### 深色模式

- 主题色靠 CSS 自定义属性（`--hue`/`--sat`/`--light`/`--theme`/`--bg-a*`/`--text*`），由 `theme_base.styl` 在编译期从 Stylus 变量计算成静态值
- 切换靠 `:root[data-theme="dark"]` / `[data-theme="light"]`，JS 改 `document.documentElement.dataset.theme`
- 系统深色模式由 `@media (prefers-color-scheme: dark)` 自动响应，`data-theme` 显式设置会覆盖媒体查询

### ctx 全局对象

Stellar 的 JS（main.js 等）依赖 `ctx` 全局对象，由 `layout/_partial/scripts/defines.ejs` 服务端渲染注入：
```js
const ctx = {
  date_suffix: { just, min, hour, day },
  root: '/',
  tag_plugins: { chat: {} },
  search: { service, local_search: { path } },
  copycode: { default_text, success_text, toast }
};
```
Go 模板需在页面注入等价的 `ctx` 对象，否则 JS 报 ReferenceError。

## Demo 2 新增坑点（Go 服务端渲染）

### go run 工作目录不可靠，必须用 -root 参数定位工作树路径

`go run ./cmd/blog` 把源码编译到临时目录再运行，`os.Executable()` 返回临时二进制路径而非工作树。`os.Getwd()`/`filepath.Abs(".")` 也取决于调用方 cwd：

- 在 Blog 根目录直接 `go run` 时 cwd 正确
- preview 工具/Bash 工具的 cwd 可能变化，因此应始终显式传入 Blog 根目录；开发期资源路径统一解析为 `<root>/resource/...`

**用法**：`main.go` 的 `-root` 接收 Blog 根目录，launch.json 使用 `-root /Users/sakurasep/Documents/Code/Project/Blog`。preview 启动要留 ~10s（go run 首次编译 Gin 依赖较慢，3s 测试窗口会误判"卡住"）。

### template.HTML 类型不需要 safe 函数转义

Go `html/template` 默认对 `{{.Field}}` 做 HTML 转义。goldmark 输出的 HTML 想原样输出，常见做法是 `template.HTML(p.HTML)` 标记为安全类型。**坑**：在模板里再套 `{{safe .Content}}` 会报 `wrong type for value; expected string; got template.HTML`--`safe` 函数签名是 `func(string) template.HTML`，接收不了已是 `template.HTML` 的值。

**正确**：`Content` 字段类型直接用 `template.HTML`，模板里 `{{.Content}}` 直接输出即可（`template.HTML` 类型的值不会被转义）。只有普通 string 想 inline 原始 HTML 时才用 `safe`。

### icons.yml 图标集合有限，部分 iconify key 不存在

`resource/hexo-theme-stellar/_data/icons.yml` 不是全量 iconify 集合，仅含 Stellar 预置的 6 个 solar duotone + 14 个 default 图标（共约 35 个）。`solar:calendar-bold-duotone` 这类 key 查不到，菜单项会渲染成空的 `<span class="icon-missing">`。

**对策**：菜单图标 key 必须先确认在 icons.yml 中存在；归档菜单用 `default:calendar` 替代不存在的 `solar:calendar-bold-duotone`。IconRegistry 的 `Get` 对未命中 key 返回占位 span，方便排查。

### icons.yml 非 YAML，用正则按行解析

icons.yml 是 flat map 格式（`key: <svg...>`），但 value 含 `<>`/`"`/`#` 等 YAML 特殊字符，`yaml.Unmarshal` 会失败或错切。用正则 `^([a-zA-Z0-9:_-]+):\s*(.+)$` 逐行匹配 key 与 svg 原文，scanner 单行最长 ~3.4KB，默认 64KB buffer 够用。

### Go 模板对齐 Stellar EJS 结构

base.html 必须严格复刻 `layout/layout.ejs` 的 `sitebg/siteblur > l_body > l_left(sidebg+leftbar-container) / l_main / l_right` 骨架，class 名和嵌套层级一字不差（CSS 选择器绑定）。sidebar.html 的 logo 区用 `header > .logo-wrap > a.avatar + a.title`，菜单用 `.nav-area > nav.menu.dis-select > a.nav-item`（不是 `.menu-item`）。menu 项的 active 态靠 `{{if eq .ID $.Page.MenuID}} active{{end}}`。

## Demo 3 新增坑点（管理后台 + SQLite）

### modernc.org/sqlite driver name 是 "sqlite" 不是 "sqlite3"

纯 Go 驱动免 CGO，但注册到 `database/sql` 的 driver name 是 **`"sqlite"`**（不是 `"sqlite3"`，后者是 mattn/go-sqlite3 的 CGO 驱动名）。`sql.Open("sqlite", "file:path.db?_pragma=...")`。用 side-effect import `import _ "modernc.org/sqlite"` 自动注册。

- DSN 用 `file:` 前缀 + `_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)` 启用 WAL 与忙等待
- `db.SetMaxOpenConns(1)` 避免 SQLite 写并发 `database is locked`（WAL 模式下读可并发，写仍串行）
- WAL 模式生成 `*.db-shm`/`*.db-wal` 旁路文件，gitignore 必须补这两条（原来只有 `*.db-journal`）

### 模板名取路径化命名，不同子目录允许同名文件

`engine.go` 用 `filepath.WalkDir` 递归加载所有 .html，模板名取相对 root 的路径去 .html（分隔符 /，如 `partial/head.html` -> `"partial/head"`）。子目录前缀防冲突：历史上曾用基名命名，导致 `admin/comments` 被 `partial/comments` 覆盖的 bug，改路径化命名后不同子目录允许同名文件。handler 调 `engine.Render("layout/base", ...)` 用路径名。部署用 `NewEngineFromEmbed`（`//go:embed layout partial`），命名方案与磁盘加载一致；原 admin/ 模板已随后台退役删除（见下节「管理后台统一到 Vue Console」）。

### Go html/template 模板注释 `{{- /* ... */}}` 在 style 块内易触发解析错误

`{{- /* 注释 */}}` 语法本身合法，但写在 `<style>` 块内、注释文本含 `*/` 或与 CSS 注释 `/* */` 混排时，解析器可能报 `comment ends before closing delimiter`。**对策**：`<style>` 块内的注释用 CSS 原生 `/* ... */`，不要用 Go 模板注释；模板注释只用于 HTML 区域。

### 管理后台统一到 Vue Console，服务端渲染 /admin 已退役

原 Go html/template 服务端渲染后台（`internal/admin/` + `admin/*.html` 模板 + EasyMDE 编辑器）已整体退役：两处目录均已清空/删除、`/admin` 路由已移除，原 EasyMDE forceSync/深色模式适配与 admin 独立 layout 的坑不再适用。现后台为 Vue SPA，源码与测试在 `modules/console/web/`（`src/` + `tests/`）：

- `modules/console/web/embed.go` 用 `//go:embed all:dist` 嵌入 `dist/` 子树（embed 路径只能嵌包目录子树，故 embed.go 放在 console/ 内），随单二进制分发
- 编译期要求 dist/ 已存在且非空：`go build` 前必须先在仓库根执行 `pnpm install --frozen-lockfile && pnpm --filter console run build`（Makefile 已固化该构建顺序）
- `setupConsoleRoutes` 托管 `/console/` 静态资源（`-dev` 开发模式从磁盘读 modules/console/web/dist），未匹配的 `/console/*` 由 NoRoute 返回 index.html 做 SPA fallback
- 数据统一走 `/api/v1` REST API（`modules/console/api/`），编辑器为 `src/components/MarkdownEditor.vue`，不再使用 EasyMDE 与 Go 模板渲染后台页

### /post/:slug DB 优先 + 文件回退的实现

`store.GetPublishedBySlug` 返回 `(nil, nil)` 表示 DB 无记录（不是 error）。handler 先查 DB：`if p, err := st.GetPublishedBySlug(slug); err == nil && p != nil { 走 DB }`，否则回退 `post.Service.Load(slug)` 文件渲染。注意 `os.IsNotExist` 能穿透 `fmt.Errorf("...: %w", err)` 的 `%w` 包裹，正确识别文件不存在。

### seed 用 UPSERT 保证幂等 + 启动增量自动发现

`-seed` flag 启动时扫描 `content/posts/*.md`，按 slug 调 `store.Upsert`：DB 无记录则 Create，有记录则保留 created_at 更新内容字段。多次执行 `-seed` 不会产生重复记录，方便重复导入。**注意全量 seed 会用文件覆盖 DB 内容字段**（Console 改过但未导出回文件的文章会被刷掉）。

日常新增文章不需要手动 `-seed`：启动时自动跑增量发现（`seed.ImportMissing`，`internal/app/services.go` 装配），只补录 DB 缺失的 slug（任意状态都算存在），**绝不触碰已有记录**，发现新文件时同步重建搜索索引与 topic 回填。文件更新后同步到 DB 仍走显式 `-seed`；posts 目录缺失时发现路径宽容跳过（不阻断启动）。

## Step 1 新增坑点（首页 + 配置文件化）

### SQLite 加列必须手写 ALTER 迁移

`CREATE TABLE IF NOT EXISTS` 不会给已存在的表加新列。加 `cover` 列时需用 `PRAGMA table_info(posts)` 遍历列名，缺失才执行 `ALTER TABLE posts ADD COLUMN cover TEXT NOT NULL DEFAULT ''`。SQLite 的 `ADD COLUMN` 要求默认值必须是常量（`DEFAULT ''` 合法，`DEFAULT CURRENT_TIMESTAMP` 非法）。

### Go template 无内置算术，分页器需注册 add/sub func

分页器的 prev/next 页码运算（`Current-1`/`Current+1`）在模板里做不了，需在 FuncMap 注册 `add`/`sub`（签名 `func(a, b int) int`）。模板里 `{{sub .Paginator.Current 1}}` 调用。

### base.html 按 Layout 分派 content 模板

`l_main` 里原来是无条件 `{{template "content" .}}`，首页改为 `{{if eq .Page.Layout "index"}}{{template "index" .}}{{else}}{{template "content" .}}{{end}}`。新增 `partial/index.html` 的基名 `index` 不与现有 `base/head/sidebar/content/list/edit` 冲突，可直接注册。

### goccy/go-yaml 是 gin 间接依赖，yaml 值注意引号

`goccy/go-yaml` 已在 go.sum（gin 间接依赖），直接 import 后 `go mod tidy` 自动去掉 `// indirect` 标记。写 site.yml 注意：`#` 开头的值（如主题色 `#1BCDFC`）必须加引号（否则当注释）；raw HTML（如 social icon 的 `<img src>`）用单引号包裹，避免 `:`/`<` 触发解析问题。

### qoder worktree 端口固定 8081

qoder 分支服务固定监听 8081，与 dev 的 8080 区分，两个 worktree 可同时起服务互不影响（端口约定的 canonical 来源见 `AGENTS.md` 的「端口约定」一节）。

## Step 2 新增坑点（归档 + 分类/标签）

### tags 逗号分隔存储的匹配一致性

tags 以逗号分隔存单列（如 `Go,Stellar`）。查询用逗号包裹 LIKE 防子串误匹配：`(',' || tags || ',') LIKE '%,Go,%'`（防 Go 匹配到 Golang）。**坑**：Console 表单输入若直接存原始值（如 `Go, 测试, Go` 带空格/重复），LIKE 的 `%,测试,%` 匹配不到 `, 测试,`（多空格），而 CountByTag 内存拆分有 TrimSpace 又能数到——两个口径不一致。**对策**：Console 保存入口（`modules/console/api/post.go`）用 `normalizeTags` 规范化（逗号拆分、逐项 trim、去空去重、`,` 重连），与 parser.parseList 语义对齐，保证存储值规范。

### 迁移泛化：migrateColumns 用 map 驱动

Step 1 的 migrateCoverColumn 泛化为 `migrateColumns`：`map[列名]ALTER语句` + `PRAGMA table_info(posts)` 检测缺列逐个 ADD COLUMN。以后加列只需往 map 加条目，不用新写迁移函数。

### 主题持久化：localStorage + head 内联脚本防闪烁

toggleTheme 原来只改 `data-theme` 不持久化，导航后被打回系统偏好。**修法**：toggleTheme 写 `localStorage.setItem('theme', next)`；head.html 在 CSS 加载前的内联脚本读 localStorage 设 `document.documentElement.dataset.theme`（防闪烁）；无记录时不设值，让 `prefers-color-scheme` 媒体查询兜底。

### Stellar JS 外部依赖 shim（历史，现状以 Spec 59 去 jQuery 为准）

main.js 从 Stellar 抄来，原依赖两个外部注入：`utils.jq`（原版 jQuery 就绪回调）与 `window.canonical`（原版 defines.ejs 注入），缺失时 `init.toc`/`canonicalCheck` 抛错中断 `initPage()`。当时的修法：base.html 注入 `utils.jq` 最小 shim + `canonicalCheck` 加守卫。**现状**：1.36/Spec 59 起 jQuery 已移除，`utils.jq` 退化为直调（保留兼容签名），DOM 操作走原生 `utils.dom`；事件绑定只走 `DOMContentLoaded`/document 级委托，不要再按 jQuery 就绪回调语义加 shim。

### 静态资源缓存：/assets 加 no-cache 协商缓存（已修复）

Go 服务原来未给 /assets 设缓存头，浏览器启发式缓存 main.js，普通刷新（甚至 Cmd+Shift+R）都可能拿到旧版。**修复（Step 2 收尾）**：gin 中间件对 `/assets/` 前缀路径设 `Cache-Control: no-cache`，注册在 `r.Static` 之前。no-cache 不是禁缓存而是协商缓存：浏览器每次发条件请求，gin Static 底层 http.ServeContent 支持 If-Modified-Since 返回 304，既保证新鲜又省流量。验证：`curl -I` 看响应头，带 If-Modified-Since 请求应得 304。

## Step 2 收尾修复（移动端 sidebar + 缓存头）

### 移动端 sidebar：sidebar 对象 + float-panel menubtn

base.html 的 main-mask `onclick="sidebar.dismiss()"` 依赖 main.js 的 sidebar 对象，缺失时移动端点遮罩抛 `sidebar is not defined`。实现对齐 Stellar `scripts/sidebar.ejs`：`leftbar` toggle l_body 的 `leftbar` attribute，`dismiss` 移除。l_right 恒空，rightbar/toggleTOC 不实现。main.js 顶层 `const sidebar` 虽不是 window 属性，但全局词法绑定对内联 onclick 可见，无需挂 window。

**坑**：float-panel menubtn 必须放在 `l_body` 内部（Stellar layout.ejs 在 l_right `</aside>` 之后、l_body 闭合之前），因为激活态样式是 `.l_body[leftbar] .float-panel button.leftbar-toggle`，放外面选中不到。按钮 class `mobile-only leftbar-toggle mobile` + 内联 `style="display:none"`：桌面靠内联样式隐藏，移动端（≤667px）`.mobile-only` 的 `display:block !important` 显示。图标用 `{{icon "default:leftbar"}}`（icons.yml 已确认存在）。main.css 是 Stellar 编译产物，`[leftbar]` attribute、`.float-panel`、`.main-mask` 样式均已包含，无需覆写。

## Step 3 新增坑点（侧边栏动态化 + TOC + 搜索 + 404）

### goldmark AutoHeadingID：id 生成规则与 re-seed 必要性

TOC 依赖标题带 id：`goldmark.New(goldmark.WithParserOptions(parser.WithAutoHeadingID()))`。两个坑：

1. **id 只保留 ASCII**：中文标题生成的 id 会丢非 ASCII 字符（如 `Markdown 元素验证` → `markdown-`，纯中文标题 → `heading-N` 序号兜底）。功能正常但 id 不美观，TOC 提取必须从渲染后 HTML 正则读 id（与标题同源），不能自己从文本推算。
2. **ContentHTML 是预渲染缓存**：posts 表存的是 goldmark 输出，开启 AutoHeadingID 后老数据没有 id，必须 `-seed` 重导（或 Console 重新保存）才生效。

另一个编译坑：post 包内已有 frontmatter `parser` 类型（parser.go），与 `github.com/yuin/goldmark/parser` 包名冲突，import 需别名 `mdparser`。

### TOC 嵌套结构对齐 Stellar hexo toc helper

CSS 的折叠选择器是 `a.toc-link + ol`（子列表必须紧跟在父级链接后面、嵌在父级 `<li>` 内），**层级缩进选择器是 `.toc-child .toc-link`（padding-left 1.5rem 起逐级加深）**——hexo toc helper 输出的子列表是 `<ol class="toc-child">` 不是裸 `<ol>`，ExtractTOC 少写 class 会导致 h3 不缩进。ExtractTOC 用状态机（openH2/openH3）生成该结构；无 h2 前置的 h3 提升为顶层项。widget 结构对齐 widgets/toc.ejs：`collapse="false"` 属性（_data/widgets.yml 默认配置，CSS 只响应 true/auto）、widget-header 的 cap-action（`sidebar.toggleTOC()` 切 #data-toc 的 collapse class，折叠动画是 `grid-template-rows: 0fr` 不是 display:none）、widget-footer「回到顶部」（`util.scrollTop()`）。标题文案用语言包原文「本文目录」（meta.toc）。

### PJAX 已移除（本节为历史，现状是整页刷新）

Step 3 曾接入 pjax.js 做无刷新导航，并围绕 `.l_body` 局部替换设计了一套 DOM 引用/事件绑定规则（`pjax:complete` 重绑、`_searchInitialized` 等绑定标记、属性同步收侧栏）。**Spec 59（对齐 1.35）起 PJAX 已整体移除**，导航为整页刷新，上述规则全部失效：事件绑定只走 `DOMContentLoaded`/document 级委托，不要再写 `pjax:complete` 监听或防重绑标记；head.html 里 `.pjax-loading`/`page-loading-bar` 的手工补丁也已一并删除。保留的结论：**前台不得出现指向 /console 后台的链接**（原 /admin 已退役，见 Demo 3「管理后台统一到 Vue Console」），后续若确需链接注意整页刷新语义。

本节的 404 结论仍然有效：main.css 已含 `.error-page` 全套样式（_components/pages/error.styl 编译产物：h1 4rem、a#back 黄底黑框 #fdb62f），**不要重复自定义**；404 结构对齐 layout/404.ejs（h1 里是 img#error 图片 + p.what/p.why 语言包文案 + `<br><br>` + a#back「返回主页」）。

### 本地搜索：/search.json no-store 与去 jQuery 移植

- 索引端点每次请求全量重建（内容派生缓存见 `modules/stellar/routes/content_cache.go`），响应头 `Cache-Control: no-store`——Console 改文后索引必须立即反映，与 /assets 的 no-cache 协商缓存不同，这里是禁缓存。另注意：前端 local-search.js 优先走 `/api/search`（FTS5），失败才降级回 /search.json 本地匹配，改搜索逻辑时两个端点都要看。
- local-search.js 从 Stellar 移植去 jQuery：入口改 `DOMContentLoaded`（早期曾加 `pjax:complete`，随 PJAX 移除已删），`$(...)`→querySelector，`.focus/.keydown`→addEventListener，`.attr`→getAttribute，`toggleClass`→classList.toggle。核心 searchFunc（关键词组合/高亮/排序）与 localStorage 缓存原样保留。
- 初始化挂在 input 的 **focus 事件**上（对齐原版）。注意 headless/失焦窗口里 `el.focus()` 只设 activeElement 不派生 focus 事件，自动化验证时需直接调 searchFunc；真实用户点击输入框无此问题。

### 自定义 404：NoRoute + render404 统一出口

`r.NoRoute(render404)` 接管未匹配路径，post/page/category/tag 四处未命中也从 `c.String(404)` 改为 render404（HTTP 404 + base 布局 Layout=404 + fillSidebar）。模板分派在 base.html 按 `.Page.Layout == "404"` 走 error404 partial。

### 对照原版先行：补样式前先查 main.css 是否已含

Step 3 初版误以为 main.css 缺 `.error-page` 而自写一套（蓝绿按钮、72px 标题），与原版冲突被覆盖顺序污染。**教训**：补样式/结构前先 grep main.css 编译产物与原版 styl/ejs 三处对照（`_components/pages/error.styl`、`_plugins/pjax.styl`、`layout/404.ejs`、`widgets/toc.ejs`、`_partial/menubtn.ejs`），确认缺失再补，且补的内容必须是原版编译产物而非自创。同类遗漏还有：float-panel 的 rightbar-toggle 按钮（menubtn.ejs 有两个按钮）、sidebar.rightbar/toggleTOC 方法（scripts/sidebar.ejs 四方法全量）。

## 工具链

- Go 1.25.5（Gin + goldmark + modernc.org/sqlite）
- Node 22 + stylus 0.62.0（CSS 编译；`make css` 从 resource/ 主题源重编 `modules/stellar/web/assets/css/main.css`，依赖 modules/stellar/web/node_modules，CI 无源不可编译、不进 test 门禁）
- SQLite（`modernc.org/sqlite` 纯 Go 驱动，免 CGO，driver name `"sqlite"`）
- 预览：`python3 -m http.server 8090`（配置在根目录 `.claude/launch.json`）
- Go 服务：`go run ./cmd/blog/ -root <worktree 绝对路径>`，`-seed` 可选（导入 content/posts/*.md 到 DB）。dev 端口 8080（约定以 `AGENTS.md` 的「端口约定」为准）
- Go module：根目录 `go.mod` 是唯一 module。**所有 go 命令必须在 worktree 根执行**（`go run ./cmd/blog/`、`go vet ./...`、`go test ./...`）。

## Demo 进度

> 以下为历史进度快照，不代表现状：其中 Demo 3 的 `/admin` 路由与 EasyMDE 已退役（见「管理后台统一到 Vue Console」），PJAX 已移除（见 Spec 59）。当前能力以后续 Spec 章节与代码为准。

- [x] Demo 0：bare 仓库 + worktree 初始化
- [x] Demo 1：CSS 编译验证（shim 方案，产物 267.9 KB，artalk/深色模式/插件样式均正确）
- [x] Demo 2：Go 服务端渲染（Gin + html/template + goldmark，/post/:slug 路由 + 彩色菜单图标 + 深色模式切换 + ctx 注入，preview 验证通过）
- [x] Demo 3：管理后台（SQLite + 文章 CRUD + EasyMDE 编辑器，/admin 路由 + DB 优先/文件回退 + -seed 导入，preview 验证通过）
- [x] Step 1：首页 + 文章列表（`/` 渲染文章卡片列表 + `/page/:num` 分页 + cover 封面（含老库 ALTER 迁移）+ config/site.yml 配置文件化（DefaultSite 保留 fallback），preview 验证通过）
- [x] Step 2：归档 + 分类/标签（`/archives` 年份分组 + `/categories` `/tags` 索引页（badge 计数）+ `/categories/:name` `/tags/:name` 文章列表（复用首页卡片）+ posts 表加 category/tags 两列（migrateColumns 泛化）+ navbar 抽共用 partial 四 tab + 主题持久化 + JS shim 修控制台错误，preview 验证通过）
- [x] Step 2 收尾：移动端 sidebar（main.js sidebar 对象 leftbar/dismiss + base.html float-panel menubtn）+ /assets no-cache 协商缓存中间件（curl 304 与移动/桌面视口浏览器验证通过）
- [x] Step 3：侧边栏动态化（recent/tags 从 DB 实时生成 + 菜单死链清理）+ 文章页 TOC 右栏（AutoHeadingID + ExtractTOC 嵌套 + 原生 scroll-spy）+ 本地搜索（/search.json no-store + 去 jQuery local-search.js）+ PJAX 无刷新导航（pjax.js 接入 + 加载指示样式）+ 自定义 404（NoRoute/render404 统一出口），浏览器全项验证通过（__mark 跨导航存活、scroll-spy 高亮、搜索高亮/noresult、404 双主题截图、控制台零错误）

## Spec 68 · Stellar 1.40.0 同步（2026-08）

`resource/hexo-theme-stellar` 已固定到 `1.40.0`，`modules/stellar/web/assets/css/main.css` 必须通过 `make css` 从该主题源重编；CSS 版本使用模板的 `asset_ver` 内容哈希，不再手写固定版本号。同步内容包括 `corner-shape`、左栏 card、头像渐变、文章置顶轮播、reading time、card tags、table/tip/md 标签和 Wiki README 首页回退。

**主题源版本漂移警示**：`resource/hexo-theme-stellar` 是 gitignore 的本地克隆、跨 worktree 共享，可能被其他 feature 分支切到别的版本。执行 `make css` 前先确认克隆版本与本分支同步版本一致（当前以本文档最新 Spec 章节记录的版本为准，见 Spec 73；`git -C resource/hexo-theme-stellar describe --tags --always` 核对），不一致先切回，避免产物跨版本漂移导致 `css-drift-check` 失败。

- 置顶轮播仅在 `article.pin_style=carousel` 且文章列表为首页第一页时装配；首页后续分页、flat 模式不重复显示。
- `ReadingStats` 中文按字符、英文/数字按词统计，空正文阅读分钟仍为 1；展示开关由 `article.reading_time` 控制。
- Wiki 项目配置 `repo` 且首页文档正文为空时，输出客户端 `ds-mdrender` README 占位；`branch` 缺省为 `HEAD`。Wiki SEO 描述为空时回退项目描述。
- 当前项目没有 Memos `ds-memos` 模板/配置入口，因此不引入未使用的 Memos 服务；Artalk/Waline 外部评论适配同样不在自建评论范围内。
- Go 分类字段仍为单一字符串，没有 Hexo category parent 图，因此不复制 `category_tree.js` 的虚假层级；Wiki `wikidocs.parent` 树仍按最多三级递归渲染。

## Spec 59 · Stellar 1.38.0 同步（2026-08，取代上文 PJAX/jQuery 相关坑点）

资源克隆 `resource/hexo-theme-stellar` 已切到 `1.38.0` tag；`modules/stellar/web/build/stylus/config.json` 同步 1.38 形状（增 `style.corner-shape`/`gradient.angle`，删 `smooth_scroll`），重编 `main.css`。**上文 Step 3 的 PJAX 与去 jQuery 移植章节自此为历史**，现状如下：

- **PJAX 已移除**（对齐 1.35）：导航为整页刷新；`pjax.js`/`pjax-transitions.css`/`plugins.pjax` 配置与 console PJAX 卡片均删除；保留 `flying-pages` 悬停预取（`plugins.preload.enable`）缓解整页刷新。所有 `pjax:complete` 监听已删，事件绑定只走 `DOMContentLoaded`/委托。
- **jQuery 已移除**（对齐 1.36）：head 不再加载 vendored jquery；`utils` 增加原生 `qs/qsa/dom` 封装（find/append/class/attr/text/html/val/on），`utils.jq` 退化为直调；services 以 1.38 `source/js/services/*` 为蓝本（`utils.dom` 直跑），rating/vote 保留自建本地版。
- **配置形状对齐**：`style.gradient.button→start`（+`angle`）；删 `style.smooth_scroll`；增 `data_cache`（客户端 SWR：utils.request 同 URL 去重 + localStorage TTL）、`site_tree.home.comments`（首页评论区，默认关）、`style.corner-shape`。
- **新特性**：mbti 标签（`{% mbti TYPE scores %}`，shortcode.go renderMbti，CSS 随重编带入）；首页评论区（routes_home fillComments "/" + base.html index 分支挂载）。
- **菜单列表化**（1.38）：sidebar.html 菜单改 `a.nav-item`（icon + span + active-dot + 每项内联 grad-def SVG，`--item-grad`/`--item-theme`），Go 模板函数 `gradsvg` 计算渐变坐标（角度取 `style.gradient.angle`，默认 210deg）。

### cover-url 注入：photo 大卡 `.cover:before` 模糊层

1.38 CSS `.cover:before` 伪元素的 `background-image` 读 `var(--cover-url)` 做模糊层（花/叶图 Next 风格），photo 类型大卡必须手动注入内联 style。**模板中写法**：

```html
<div class="cover" style="--cover-url:url('/uploads/xxx.jpg')">
```

**坑**：`html/template` 在 style 属性 CSS `url()` 上下文中自动把单引号转义为 `%27`（如 `url('...')` → `url(%27...%27)`），这与上游 Stellar Hexo 行为一致，浏览器正确解析 `%27` 为单引号，无需额外转义函数。如果手动用 `printf` 或 `safe` 函数反而可能双转义，保持默认即可。

### 静态资源版本化：asset_ver 内容哈希（手工 bump 已退役）

SW（Service Worker，head.html 注册 `/sw.js`）对 `/assets/*` 走 `cache-first` 策略，早期静态资源内容变化后需手工 bump `?v=` 版本戳防新旧资源错配。**现状**：资源引用已统一改为 `?v={{asset_ver "<path>"}}`（Spec 68 引入，模板函数启动时按内容算哈希，见 `modules/stellar/web/assetver.go`），内容一变 URL 自动变，SW cache-first 天然不会命中旧资源，**不再需要手工 bump**。新增 assets 引用时一律用 `asset_ver` 模板函数，不要写死版本号；此坑不影响 `-dev` 开发模式（磁盘实时加载）。

### seed.go Pinned 字段赋值

`seed.go` 的 `Post` 结构体含 `Pinned bool` 字段（置顶标记），但在 `-seed` 导入流程中曾漏掉对该字段的赋值 —— 从 frontmatter 解析出 `pinned: true` 后未映射到数据库记录，导致置顶功能不生效。**已修复**：`seed.go` 导入循环中补充 `Pinned: p.Pinned` 映射。

### seed.go 草稿语义

`seed.go` 的 `Import` 原硬编码 `Status: store.StatusPublished`，未尊重 frontmatter 的 `status: draft`，导致草稿文章被公开发布（如 tag-plugins-test.md）。**已修复**：对齐 `backup.go` 的 `importOne` 写法，在导入循环前置判断 `meta.Status == store.StatusDraft` 时设 `status = store.StatusDraft`，否则保持 Published。

## Spec 60 · 主题配置复刻 + 页面统一管理（2026-08）

- **主题 YAML**：`GET/PUT /api/v1/settings/style/yaml`（goccy/go-yaml 解析校验 → `SaveSection(style)` → `RebuildSite` 热更新）。设置页外观组「主题 YAML」tab（ThemeYamlTab.vue）直接编辑 site.yml `style` 段，与 AppearanceTab/高级·JSON 同源。`StyleConfig` 对齐 1.38 style 面（font-size code/codeblock、border-radius 七档、codeblock.highlightjs_theme、animated_avatar；header_prefix 值为 URL 时注入图标 background-image）。**注意**：main.css 不消费 `--radius-*` 变量，圆角运行时注入实际无效，颜色/`--fsp`/字体/左栏/全站背景才是有效旋钮。
- **页面统一管理**：PageList.vue 顶部 `apple-segmented` 三分段（普通页面/文库/友链），深链 `?module=wiki|friends`，内嵌 WikiDocsView/FriendList；数据仍存 wikidocs/links 表。**坑**：分段/module 逻辑必须放在 `useRouter()/useRoute()` 声明之后（TDZ），否则 PageList 挂载失败白屏。

## Spec 71 · Stellar 1.41.0 同步（2026-08）

`resource/hexo-theme-stellar` 已切到 `1.41.0`，`make css` 重编 `main.css`（293 KB）；自本节起项目同步版本以 1.41.0 为准（Spec 68 的 1.40.0 描述为历史）。坑点：

### 图标命名空间迁移（1.41 破坏性变更）

- `internal/settings/data/icons.yml` 已整体替换为上游 1.41 版：**98 键、12 语义化命名空间**（`default:`/`example:`/`quot:`/`chat:`/`github:`/`share:`/`vote:`/`weibo:`/`image:`/`copy:`/`hashtag:`/`rating:`），`solar:*`/`ph:*`/`bxs:*` 旧前缀全部废除。引用一律用新键（新旧映射以上游 1.41 `icons.yml` 为准）；最长行 `chat:safari` 13.6 KB，Scanner 默认 64 KB buffer 仍够。dev 侧自有的 `default:reward`（打赏）键已在迁移时保留。
- 上游的「异步图标加载」（`<svg data-icon>` 占位 + `/js/icons/{ns}.json` 拉取替换）属 Hexo 构建系统项**不移植**，本项目保持 IconRegistry 全内联 SVG。
- **旧键兼容**：settings 表历史持久化值（menubar/rightbar/category_icon/tag_icon）仍带 `solar:` 前缀，`icons.go` 的 `legacyIconAlias` 表在 `Get` 未命中主表时回退——不要删该表，除非确认运行库已无旧键；未知键仍走 `icon-missing` 占位（防注入转义路径）。

### CSS 编译入口变化：1.41 移出的样式段经 compile.js 追加

1.41 把 swiper/fancybox/评论样式移出 `main.styl` 编译入口（上游改按需外置 `<link>`）。本项目不引入按需加载，`compile.js` 的 `EXTRA_STYLES` 在主入口后追加编译 `plugins/swiper.styl`、`plugins/fancybox.styl`、`comments/artalk.styl` 拼进单文件 `main.css`（顺序对齐 1.40 产物）。**上游再移动这些文件位置时需同步更新 EXTRA_STYLES 清单**。

- `config.json` 顺带修正：lazyload transition 的消费路径是 `dependencies.lazyload.transition`（1.40 起即如此，原写在 `plugins.lazyload` 段恒 null 走默认）；`comments`/`plugins.fancybox/swiper/mermaid/pjax` 段已删（无消费者）。

### 配置新增键的「整段覆盖」坑：用 *bool 而非裸 bool

`config` 的 `SectionXxx` 解析是**整段覆盖式合并**（`site.Article = parsed.article`）：settings 表历史 JSON 没有新键时，unmarshal 零值（false/0）会吃掉代码默认值。1.41 新增的 `article.tags`（默认开）、`search.local_search.lazy_load`（默认开）/`cache_ttl`（默认 86400）都用 **`*bool`/`*int` + `Effective*()` 方法**（nil=未配置走默认），`ai_label.default` 留空即零渲染。以后加「默认开启」型配置键沿用此模式。**嵌套配置结构体**同理会整段吃掉默认：`AILabelConfig.FillDefaults()`（nil 档填主题内置四档）需在解析后与主题默认归一化两处都调用（现落点：`internal/config/reload_rebuild.go` 的 parseSections 与 `internal/config/site.go` 的 normalizeThemeDefaults）——只靠 DefaultSite 预填会被 settings 表旧行覆盖清零。

### ai_label 的 store 写路径：NULLIF 保留语义

posts 表 `ai_label` 列在 Update/Upsert SQL 里是 `ai_label=COALESCE(NULLIF(?,''), (SELECT ai_label ...))`：seed 传 frontmatter 新值可更新；Console 旧客户端不传字段（零值 ""）**不清空**已有值。副作用：frontmatter 删掉 ai_label 后 seed 重导不置空，清空需显式 API 调用（见 spec Out of Scope）。

### 搜索章节化：anchors 偏移必须用 UTF-16 单位

`textutil.StripHTMLWithAnchors` 产出的 `Anchor.Offset` 单位是 **UTF-16 code unit**（与 JS `String.slice`/`length` 一致）——前端按它切章节文本；Go 侧消费（`resultAnchor` 命中映射）需自行换算（`Utf16Len`），直接当字节偏移用会中文错位。测试断言同样要用 `[]rune` 索引而非字节切片。

- 搜索缓存升 **v4**（`{ts, ttl, data}` 结构 + `cache_ttl` 秒，0=禁用）；懒加载默认开（首次聚焦才拉 `/search.json`）。
- `?kw=` 高亮在独立文件 `assets/js/search/highlight.js`（base.html 已引入，asset_ver 内容哈希自动版本戳）。
- 服务器端 `/api/search` 结果带 `anchor_id/anchor_text`，前端拼 `?kw=首词#锚点`。

## Spec 72 · Stellar 1.42.1 同步（2026-08，历史）

该版本说明已被 Spec 73 的 `origin/main@c078d5d` 版本锚点替代。历史上曾固定到 `1.42.1` tag；A 阶段从该源重编 `modules/stellar/web/assets/css/main.css`，验证前必须先确认共享主题源没有被其他 worktree 切换。

- 配置同步：`article.card_style` 默认 `hero`，`plugins.adaptive_text.enable` 默认开启，`article.auto_banner` 移除，Waline CDN 版本为 `3.15.2`。新增默认开启配置使用指针或 `Effective*` 归一化，避免 settings 表旧行的零值覆盖默认值。
- CSS 令牌：上游已删除 `--gap-margin`、`--gap-padding`、`--gap-max`，项目侧统一使用 `--gap-base`（组件内间距）与 `--gap-page`（页面级留白）。`compile.js` 的 EXTRA_STYLES 仍固定追加 swiper/fancybox/artalk，1.42.1 未移动这些入口。
- 自适应文字：`color.js` 与 `plugins/adaptive-text.js` 通过 `data-text-adaptive` 按需加载。Go 模板没有 Hexo bootstrap，`base.html` 的 `utils.initPlugin`/`stellar.initPlugin` 是等价同步注册点；脚本自身在 DOM 就绪后扫描目标元素。图片懒加载通过 DOM 属性读取 `img[src]`，不做 HTML 正则替换，并跳过 script/style/noscript 与 `no-lazy` 标记。
- 模板契约：hero 卡片和置顶轮播共用 `.cover-info`/`.pin-slide-text` 的标题+单行小字结构；专栏索引使用 `.post-list.topic > .post-card-wrap > .post-card.topic`，最新文章复用 `partial/latest_post_card`，其余文章复用 `.post-panel`。
- 破坏性变更：`weibo:comment` 已删除，评论数使用 `default:tocomment`；未配置 banner 时不再回退 `default.banner`。验收文章为 `content/posts/142-stellar-142.md`。

### 平滑滚动与「天然规避」项

- `main.js` 新增 `bindAnchorClick`（document 级委托 `a[href^="#"]`，`#start` 贴顶、其余 32px、`defaultPrevented` 跳过）+ `smoothScrollTo`（easeOutCubic，`behavior:'instant'` 防与全局 smooth 叠加）。放 `initPage` 内绑定，有 `anchorClickBound` 防重标记。
- 上游「评论表情面板误触发 fancybox」与「wiki 标题去重」两项在本项目架构下**天然规避**：fancybox selector 只认 `div.lazy-box>img.lazy`（wrapLazyloadImages 仅处理 `.md-text.content`，评论面板动态渲染不在内）；wiki 文档页 title 不拼项目名前缀。排查同类问题先看本项目 selector/拼装方式，不要照搬上游补丁。

## Spec 73 · Stellar Wiki 改版同步（2026-08）

当前共享主题源版本锚定为 `origin/main@c078d5d`（`git describe --tags --always` 为 `1.42.1-10-gc078d5d`，上游 main 无 tag），替代 Spec 72 的 `1.42.1` 固定说明。该源跨 worktree 共享；重编前必须确认 detached HEAD 仍指向 `c078d5d`。

- 上游 `source/css/_components/wiki-card.styl` 为 209 行版本：Wiki 列表必须使用 `.post-list.wiki > .post-card-wrap > a.wiki-card`，卡片内部为 `.wiki-card-content`、3:4 `.wiki-card-cover`、自适应文字 `.wiki-card-info`、标签/适用范围/GitHub 热度元信息和 `.wiki-project` 底栏。
- `modules/stellar/web/templates/partial/wiki.html` 已对齐 `index_wiki.ejs` + `wiki_card.ejs`，保留本地项目文档/外链跳转语义；封面覆盖层等待图片与自适应主题色共同就绪后再加 `cover-loaded`，避免默认主题色闪现。
- Wiki 文档 Hero 使用 `.l_cover.wiki-hero` / `.wiki-cover-background.galaxy` 结构；`assets/js/plugins/galaxy.js`（423 行）按需挂载 WebGL 星河，`dropdown.js`（435 行）由基础布局加载用于 footer/social 浮层。`main.js` 只合入 Wiki Hero 终端/Galaxy 初始化相关改动。
- `modules/stellar/web/assets/css/main.css` 只能由 `make css` 生成；完成后执行 `make css-drift-check`。本地 `highlight.styl` 与 `search.styl` 小改补丁保存在 `.tmp/stellar-local-tweaks.patch`，不纳入主题源同步提交。

### c078d5d 缺口补齐（2026-08-20）

- **adaptive-text.js**：补入 `--adaptive-background` 背景解析链首位（wiki_doc.html galaxy 背景 `#000000` 直接走纯色路径）；新增 `setWikiOverlayColor`（封面平均色 enhanceSaturation→darken 0.26/lighten 0.46 写 `--wiki-overlay-color`/`--wiki-border-color`）与 `notifyWikiOverlayReady`（派发 `wiki-overlay-ready` 事件）；`split` 样式下父元素含 `wiki-card-cover` 时成功分支写变量并通知、失败分支仅通知解除等待。保留项目特有文件尾部（defer 扫描块）。
- **icons.yml**：`default:loading` 替换为 SMIL 三圆点内联 SVG（旧 class="loading" 旋转箭头与外部 iconify URL 一并删除）；新增 `default:github`/`default:monitor`/`default:platforms`/`default:fire`；删除 `default:loading-spinner`/`default:loading-placeholder`/`default:goback`（旧键无外部引用）。
- **--icon-loading 注入**：`funcs.go` 新增 `iconLoadingURL` 模板函数（`default:loading` → data-URI `url("data:image/svg+xml,...")`，URL 值直用，键缺失返回空）；`head.html` 注入 `:root{--icon-loading:...}`，`.lazy-icon` mask 不再回退外部 iconify。
- **siteinfo 图标兜底**：新增 `assets/js/services/siteinfo.js`（`setSiteCardIcon` + `stellar:sites-ready` 监听）；`sites.js` 补 `siteinfoApi` 计算（无 icon/avatar 且配 api 时 `data-siteinfo-api="{href→encodeURIComponent}"`）、图标 img 加 `siteinfo-icon` class、渲染完成派发 `stellar:sites-ready`。配置面 `model.go` 加 `ServicesConfig`/`SiteInfoConfig`，`Site` 加 `Services` 字段；`site.example.yml` 加 `services.siteinfo.api` 示例（默认空=关闭）；`base.html` ctx 注入 `services.siteinfo.api`、懒加载数组追加 siteinfo.js（scan 去重键改为 `s.js` 允许同 `cls` 多脚本）。
- **navbar 2px 滚动阈值**：`main.js` `navbarPin` 合入上游 `SCROLL_THRESHOLD = 2`，`update()` 内 `scrolled = window.scrollY >= SCROLL_THRESHOLD`，toggle 条件改为 `scrolled && top <= stickyTop + TOLERANCE`（无轮播页顶部即吸顶，需滚动达阈值才切玻璃）。
