# Stellar 博客项目 - 开发规范

本项目为自建个人博客：Go (Gin) 后端 + Go html/template 服务端渲染 + 从 Stellar Hexo 主题提取的 CSS + 自建管理后台。

> 本文档是与提供方无关的工程坑点知识的 canonical 来源，适用于所有 agent（Qoder / Claude / Trae 等）；按模块的路由入口见 `AGENTS.md` 的「工程坑点知识路由」一节。多 Agent 并行开发流程（分支模型、合并门槛、端口约定、提交规范）以 `AGENTS.md` 为准。

## 仓库工作流

### 目录结构（bare 仓库 + git worktree）

根目录是 bare 仓库容器，三个分支各占独立子目录 worktree：

| 分支 | 工作树路径 | 用途 |
|------|-----------|------|
| `main` | `main/` | 主分支，稳定版本 |
| `dev` | `dev/` | 日常开发 |
| `release` | `release/` | 发布版本 |

- `.bare/`：bare 仓库核心，根目录 `.git` 文件指向它
- `resource/`：参考资源（hexo-theme-stellar 主题克隆），已 gitignore，不入版本控制
- `.claude/`：本地工具配置（launch.json），不入版本控制

### worktree 操作注意

- **git 命令必须在某个 worktree 子目录内执行**（main/dev/release），根目录是 bare 仓库，执行 `git status`/`git add`/`git commit` 会报 `fatal: this operation must be run in a work tree`
- **注意：bash 每次调用的 cwd 可能不保留**，容易在根目录误执行 git 命令。提交时用 `cd /Users/sakurasep/Documents/Code/Project/Blog/dev && git ...` 显式切目录，或用 `git -C <worktree路径> ...`
- 若 worktree 的 `.git` 文件丢失（`git worktree list` 显示 `prunable`），而 `.bare/worktrees/<name>/` 元数据还在，可重建：`echo "gitdir: /Users/sakurasep/Documents/Code/Project/Blog/.bare/worktrees/<name>" > <子目录>/.git`
- `resource/` 通过相对路径引用：从 `dev/` 出发是 `../resource/hexo-theme-stellar/...`

### 提交规范

提交规范的 canonical 来源见 `AGENTS.md` 的「提交规范」一节，本文档不另行定义。

## Stellar CSS 剥离规范

### 编译链路（已验证）

Stellar 的 Stylus 源码用 `hexo-config()` 读取 Hexo 配置，脱离 Hexo 编译会报 undefined。本项目用 **shim 注入方案**，不改任何 .styl 源文件：

- `frontend/build/stylus/config.json`：从 `_config.yml` 提取 `style`/`article`/`menubar`/`comments`/`plugins`/`tag_plugins` 六段配置
- `frontend/build/stylus/shim.js`：用 Stylus JS API 的 `define('hexo-config', fn)` 注入函数，按点路径从 config.json 取值
- `frontend/build/stylus/compile.js`：编译 `resource/hexo-theme-stellar/source/css/main.styl` → `frontend/assets/css/main.css`
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

### CSS 缓存坑

http.server 不禁缓存，修改 .styl 重新编译后浏览器可能仍用旧 CSS。preview.html 的 `<link>` 加版本戳（`?v=demo1`），开发时手动改版本号强制刷新；或浏览器 DevTools 禁用缓存。

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

### go run 工作目录不可靠，必须用 -root 参数定位 dev 路径

`go run cmd/server/main.go` 把源码编译到临时目录再运行，`os.Executable()` 返回临时二进制路径而非 dev/。而 `os.Getwd()`/`filepath.Abs(".")` 取决于调用方 cwd：

- 在 dev/ 下直接 `go run` 时 cwd 正确
- 但 preview 工具/Bash 工具的 cwd 不稳定（可能落在项目根 Blog/ 而非 dev/），此时 `filepath.Join(root, "..", "resource", ...)` 会解析成 `Blog/../resource/...` = `Project/resource/...`（少了 Blog 层），`icons.yml` 报 `no such file or directory`，服务起不来表现为 HTTP 000

**修复**：main.go 加 `-root` flag 接收 dev 绝对路径，launch.json 显式传 `-root /Users/sakurasep/Documents/Code/Project/Blog/dev`。preview 启动要留 ~10s（go run 首次编译 Gin 依赖较慢，3s 测试窗口会误判"卡住"）。

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

原 Go html/template 服务端渲染后台（`backend/internal/admin/` + `admin/*.html` 模板 + EasyMDE 编辑器）已整体退役：两处目录均已清空/删除、`/admin` 路由已移除，原 EasyMDE forceSync/深色模式适配与 admin 独立 layout 的坑不再适用。现后台为 Vue SPA，源码与测试在 `frontend/console/`（`src/` + `tests/`）：

- `frontend/console/embed.go` 用 `//go:embed all:dist` 嵌入 `dist/` 子树（embed 路径只能嵌包目录子树，故 embed.go 放在 console/ 内），随单二进制分发
- 编译期要求 dist/ 已存在且非空：`go build` 前必须先 `cd frontend/console && npm ci && npm run build`（Makefile 已固化该构建顺序）
- `setupConsoleRoutes` 托管 `/console/` 静态资源（`-dev` 开发模式从磁盘读 frontend/console/dist），未匹配的 `/console/*` 由 NoRoute 返回 index.html 做 SPA fallback
- 数据统一走 `/api/v1` REST API（`backend/internal/api/`），编辑器为 `src/components/MarkdownEditor.vue`，不再使用 EasyMDE 与 Go 模板渲染后台页

### /post/:slug DB 优先 + 文件回退的实现

`store.GetPublishedBySlug` 返回 `(nil, nil)` 表示 DB 无记录（不是 error）。handler 先查 DB：`if p, err := st.GetPublishedBySlug(slug); err == nil && p != nil { 走 DB }`，否则回退 `post.Service.Load(slug)` 文件渲染。注意 `os.IsNotExist` 能穿透 `fmt.Errorf("...: %w", err)` 的 `%w` 包裹，正确识别文件不存在。

### seed 用 UPSERT 保证幂等

`-seed` flag 启动时扫描 `content/posts/*.md`，按 slug 调 `store.Upsert`：DB 无记录则 Create，有记录则保留 created_at 更新内容字段。多次执行 `-seed` 不会产生重复记录，方便重复导入。

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

tags 以逗号分隔存单列（如 `Go,Stellar`）。查询用逗号包裹 LIKE 防子串误匹配：`(',' || tags || ',') LIKE '%,Go,%'`（防 Go 匹配到 Golang）。**坑**：Console 表单输入若直接存原始值（如 `Go, 测试, Go` 带空格/重复），LIKE 的 `%,测试,%` 匹配不到 `, 测试,`（多空格），而 CountByTag 内存拆分有 TrimSpace 又能数到——两个口径不一致。**对策**：Console 保存入口（`backend/internal/api/post.go`）用 `normalizeTags` 规范化（逗号拆分、逐项 trim、去空去重、`,` 重连），与 parser.parseList 语义对齐，保证存储值规范。

### 迁移泛化：migrateColumns 用 map 驱动

Step 1 的 migrateCoverColumn 泛化为 `migrateColumns`：`map[列名]ALTER语句` + `PRAGMA table_info(posts)` 检测缺列逐个 ADD COLUMN。以后加列只需往 map 加条目，不用新写迁移函数。

### 主题持久化：localStorage + head 内联脚本防闪烁

toggleTheme 原来只改 `data-theme` 不持久化，导航后被打回系统偏好。**修法**：toggleTheme 写 `localStorage.setItem('theme', next)`；head.html 在 CSS 加载前的内联脚本读 localStorage 设 `document.documentElement.dataset.theme`（防闪烁）；无记录时不设值，让 `prefers-color-scheme` 媒体查询兜底。

### Stellar JS 外部依赖缺失需 shim/守卫

main.js 从 Stellar 抄来，依赖两个外部注入：`utils.jq`（原版 jQuery 就绪回调，demo 未加载 jQuery）与 `window.canonical`（原版 defines.ejs 注入）。缺失时 `init.toc` 抛 `utils is not defined` 中断 `initPage()`（连带 relativeDate 失效），`canonicalCheck` 抛 `originalHost of undefined`。**对策**：base.html 注入 `utils = { jq: (fn) => { if (window.jQuery) fn(); } }` 最小 shim；`canonicalCheck` 开头加 `if (!canonical) return;` 守卫。`sidebar.dismiss` 原本也未实现（移动端 main-mask 点击报错），已在 Step 2 收尾补齐（见下节）。

### 静态资源缓存：/assets 加 no-cache 协商缓存（已修复）

Go 服务原来未给 /assets 设缓存头，浏览器启发式缓存 main.js，普通刷新（甚至 Cmd+Shift+R）都可能拿到旧版。**修复（Step 2 收尾）**：gin 中间件对 `/assets/` 前缀路径设 `Cache-Control: no-cache`，注册在 `r.Static` 之前。no-cache 不是禁缓存而是协商缓存：浏览器每次发条件请求，gin Static 底层 http.ServeContent 支持 If-Modified-Since 返回 304，既保证新鲜又省流量。验证：`curl -I` 看响应头，带 If-Modified-Since 请求应得 304。

## Step 2 收尾修复（移动端 sidebar + 缓存头）

### 移动端 sidebar：sidebar 对象 + float-panel menubtn

base.html 的 main-mask `onclick="sidebar.dismiss()"` 依赖 main.js 的 sidebar 对象，缺失时移动端点遮罩抛 `sidebar is not defined`。实现对齐 Stellar `scripts/sidebar.ejs`：`leftbar` toggle l_body 的 `leftbar` attribute，`dismiss` 移除。l_right 恒空，rightbar/toggleTOC 不实现。main.js 顶层 `const sidebar` 虽不是 window 属性，但全局词法绑定对内联 onclick 可见，无需挂 window。

**坑**：float-panel menubtn 必须放在 `l_body` 内部（Stellar layout.ejs 在 l_right `</aside>` 之后、l_body 闭合之前），因为激活态样式是 `.l_body[leftbar] .float-panel button.leftbar-toggle`，放外面选中不到。按钮 class `mobile-only leftbar-toggle mobile` + 内联 `style="display:none"`：桌面靠内联样式隐藏，移动端（≤667px）`.mobile-only` 的 `display:block !important` 显示。图标用 `{{icon "default:leftbar"}}`（icons.yml 已确认存在）。main.css 是 Stellar 编译产物，`[leftbar]` attribute、`.float-panel`、`.main-mask` 样式均已包含，无需覆写。

## Step 3 新增坑点（侧边栏动态化 + TOC + 搜索 + PJAX + 404）

### goldmark AutoHeadingID：id 生成规则与 re-seed 必要性

TOC 依赖标题带 id：`goldmark.New(goldmark.WithParserOptions(parser.WithAutoHeadingID()))`。两个坑：

1. **id 只保留 ASCII**：中文标题生成的 id 会丢非 ASCII 字符（如 `Markdown 元素验证` → `markdown-`，纯中文标题 → `heading-N` 序号兜底）。功能正常但 id 不美观，TOC 提取必须从渲染后 HTML 正则读 id（与标题同源），不能自己从文本推算。
2. **ContentHTML 是预渲染缓存**：posts 表存的是 goldmark 输出，开启 AutoHeadingID 后老数据没有 id，必须 `-seed` 重导（或 Console 重新保存）才生效。

另一个编译坑：post 包内已有 frontmatter `parser` 类型（parser.go），与 `github.com/yuin/goldmark/parser` 包名冲突，import 需别名 `mdparser`。

### TOC 嵌套结构对齐 Stellar hexo toc helper

CSS 的折叠选择器是 `a.toc-link + ol`（子列表必须紧跟在父级链接后面、嵌在父级 `<li>` 内），**层级缩进选择器是 `.toc-child .toc-link`（padding-left 1.5rem 起逐级加深）**——hexo toc helper 输出的子列表是 `<ol class="toc-child">` 不是裸 `<ol>`，ExtractTOC 少写 class 会导致 h3 不缩进。ExtractTOC 用状态机（openH2/openH3）生成该结构；无 h2 前置的 h3 提升为顶层项。widget 结构对齐 widgets/toc.ejs：`collapse="false"` 属性（_data/widgets.yml 默认配置，CSS 只响应 true/auto）、widget-header 的 cap-action（`sidebar.toggleTOC()` 切 #data-toc 的 collapse class，折叠动画是 `grid-template-rows: 0fr` 不是 display:none）、widget-footer「回到顶部」（`util.scrollTop()`）。标题文案用语言包原文「本文目录」（meta.toc）。

### PJAX 与 DOM 引用的相处方式

pjax.js 对 `.l_body` 只做属性同步 + 子节点局部替换（换 `.l_main`、就地更新 `.l_left`/`.l_right`），因此：

- main.js 顶层缓存的 `l_body` 引用**不失效**，`sidebar.leftbar/dismiss` 导航后仍可用；
- 导航时属性同步会清掉 `leftbar` attribute，移动端侧栏自动收起，无需手动处理；
- 侧栏/TOC 节点会被替换成**新节点**，事件绑定要么委托到 document（init.sidebar 的 TOC 点击收侧栏），要么把绑定标记挂在 DOM 节点上（`_searchInitialized`/`_localSearchBound`/`tocState.bound`），`pjax:complete` 重跑 init 时新节点重新绑、旧节点不叠加；
- TOC 锚点点击被 pjax.js 的 handleHashLink 优先拦截平滑滚动，不会触发整页导航；
- **前台不得出现会被 PJAX 抓取的 /console 后台链接**：后续若加需 `data-pjax="false"`（当前前台无后台链接；原 /admin 路由已退役，见 Demo 3「管理后台统一到 Vue Console」）。

main.css 已含 `.error-page` 全套样式（_components/pages/error.styl 编译产物：h1 4rem、a#back 黄底黑框 #fdb62f），**不要重复自定义**；404 结构对齐 layout/404.ejs（h1 里是 img#error 图片 + p.what/p.why 语言包文案 + `<br><br>` + a#back「返回主页」）。main.css **缺** `.pjax-loading`/`.pjax-loaded`（_plugins/pjax.styl 未被编译入口包含），在 head.html 按原版 styl 手工补等价产物（page-loading-bar 居中图标 + l_main/#l_cover 的 opacity 0.4/translateY(8px) 过渡 + pjax-fade-in + prefers-reduced-motion），并在 body 闭合前加 `<div class="page-loading-bar">`（对齐 layout.ejs）。

### 本地搜索：/search.json no-store 与去 jQuery 移植

- 索引端点每次实时查 DB（demo 数据量小），响应头 `Cache-Control: no-store`——Console 改文后索引必须立即反映，与 /assets 的 no-cache 协商缓存不同，这里是禁缓存。
- local-search.js 从 Stellar 移植去 jQuery：`utils.jq` 入口改 `DOMContentLoaded + pjax:complete`，`$(...)`→querySelector，`.focus/.keydown`→addEventListener，`.attr`→getAttribute，`toggleClass`→classList.toggle。核心 searchFunc（关键词组合/高亮/排序）与 localStorage 缓存 `search_cache_v1` 原样保留。
- 初始化挂在 input 的 **focus 事件**上（对齐原版）。注意 headless/失焦窗口里 `el.focus()` 只设 activeElement 不派生 focus 事件，自动化验证时需直接调 searchFunc；真实用户点击输入框无此问题。

### 自定义 404：NoRoute + render404 统一出口

`r.NoRoute(render404)` 接管未匹配路径，post/page/category/tag 四处未命中也从 `c.String(404)` 改为 render404（HTTP 404 + base 布局 Layout=404 + fillSidebar）。模板分派在 base.html 按 `.Page.Layout == "404"` 走 error404 partial。

### 对照原版先行：补样式前先查 main.css 是否已含

Step 3 初版误以为 main.css 缺 `.error-page` 而自写一套（蓝绿按钮、72px 标题），与原版冲突被覆盖顺序污染。**教训**：补样式/结构前先 grep main.css 编译产物与原版 styl/ejs 三处对照（`_components/pages/error.styl`、`_plugins/pjax.styl`、`layout/404.ejs`、`widgets/toc.ejs`、`_partial/menubtn.ejs`），确认缺失再补，且补的内容必须是原版编译产物而非自创。同类遗漏还有：float-panel 的 rightbar-toggle 按钮（menubtn.ejs 有两个按钮）、sidebar.rightbar/toggleTOC 方法（scripts/sidebar.ejs 四方法全量）。

## 工具链

- Go 1.25.5（Gin + goldmark + modernc.org/sqlite）
- Node 22 + stylus 0.62.0（CSS 编译；`make css` 从 resource/ 主题源重编 `frontend/assets/css/main.css`，依赖 frontend/node_modules，CI 无源不可编译、不进 test 门禁）
- SQLite（`modernc.org/sqlite` 纯 Go 驱动，免 CGO，driver name `"sqlite"`）
- 预览：`python3 -m http.server 8090`（配置在根目录 `.claude/launch.json`，指向 dev/）
- Go 服务：`go run ./backend/cmd/server/ -root <worktree 绝对路径>`，`-seed` 可选（导入 content/posts/*.md 到 DB）。dev 端口 8080，qoder 端口 8081（约定以 `AGENTS.md` 的「端口约定」为准）
- Go workspace：`go.work` 在 worktree 根关联 `backend`（module blog）与 `frontend`（module blog/frontend）两 module。**所有 go 命令必须在 worktree 根执行**（`go run ./backend/cmd/server/`、`go vet ./backend/... ./frontend/...`），单独进入子目录会因 import 解析失败报错；`./...` 模式不匹配 use 的 module，须显式写包路径。

## Demo 进度

- [x] Demo 0：bare 仓库 + worktree 初始化
- [x] Demo 1：CSS 编译验证（shim 方案，产物 267.9 KB，artalk/深色模式/插件样式均正确）
- [x] Demo 2：Go 服务端渲染（Gin + html/template + goldmark，/post/:slug 路由 + 彩色菜单图标 + 深色模式切换 + ctx 注入，preview 验证通过）
- [x] Demo 3：管理后台（SQLite + 文章 CRUD + EasyMDE 编辑器，/admin 路由 + DB 优先/文件回退 + -seed 导入，preview 验证通过）
- [x] Step 1：首页 + 文章列表（`/` 渲染文章卡片列表 + `/page/:num` 分页 + cover 封面（含老库 ALTER 迁移）+ config/site.yml 配置文件化（DefaultSite 保留 fallback），preview 验证通过）
- [x] Step 2：归档 + 分类/标签（`/archives` 年份分组 + `/categories` `/tags` 索引页（badge 计数）+ `/categories/:name` `/tags/:name` 文章列表（复用首页卡片）+ posts 表加 category/tags 两列（migrateColumns 泛化）+ navbar 抽共用 partial 四 tab + 主题持久化 + JS shim 修控制台错误，preview 验证通过）
- [x] Step 2 收尾：移动端 sidebar（main.js sidebar 对象 leftbar/dismiss + base.html float-panel menubtn）+ /assets no-cache 协商缓存中间件（curl 304 与移动/桌面视口浏览器验证通过）
- [x] Step 3：侧边栏动态化（recent/tags 从 DB 实时生成 + 菜单死链清理）+ 文章页 TOC 右栏（AutoHeadingID + ExtractTOC 嵌套 + 原生 scroll-spy）+ 本地搜索（/search.json no-store + 去 jQuery local-search.js）+ PJAX 无刷新导航（pjax.js 接入 + 加载指示样式）+ 自定义 404（NoRoute/render404 统一出口），浏览器全项验证通过（__mark 跨导航存活、scroll-spy 高亮、搜索高亮/noresult、404 双主题截图、控制台零错误）

## Spec 59 · Stellar 1.38.0 同步（2026-08，取代上文 PJAX/jQuery 相关坑点）

资源克隆 `../resource/hexo-theme-stellar` 已切到 `1.38.0` tag；`frontend/build/stylus/config.json` 同步 1.38 形状（增 `style.corner-shape`/`gradient.angle`，删 `smooth_scroll`），重编 `main.css`。**上文 Step 3 的 PJAX 与去 jQuery 移植章节自此为历史**，现状如下：

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

### 静态资源版本戳 bump 规范

SW（Service Worker）对 `/assets/*` 走 `cache-first` 策略，静态资源内容变化后老访客拿到新 HTML 但 SW 返回缓存的旧资源，导致新旧资源错配（CSS 新类名引用旧样式文件等）。**对策**：

- 每次修改 `frontend/assets/` 下的文件（main.css、main.js、plugins/ 脚本等），必须同步 bump `head.html`/`base.html` 中的 `?v=` 版本戳（如 `main.css?v=dark6` → `dark7`）
- bump 清单对照 `git diff --name-only` 变化的 assets 文件，只 bump 确实变了的文件，不变的不动
- bump 对照基线是最近一次改动 assets 的提交（本次评审对应 1.38 同步提交）；plugins/ 下脚本（scrollreveal-init.js、heti-init.js 等）内容变动后同样需要 bump
- 此坑不影响 `-dev` 开发模式（磁盘实时加载），但影响生产构建产物

### seed.go Pinned 字段赋值

`seed.go` 的 `Post` 结构体含 `Pinned bool` 字段（置顶标记），但在 `-seed` 导入流程中曾漏掉对该字段的赋值 —— 从 frontmatter 解析出 `pinned: true` 后未映射到数据库记录，导致置顶功能不生效。**已修复**：`seed.go` 导入循环中补充 `Pinned: p.Pinned` 映射。

### seed.go 草稿语义

`seed.go` 的 `Import` 原硬编码 `Status: store.StatusPublished`，未尊重 frontmatter 的 `status: draft`，导致草稿文章被公开发布（如 tag-plugins-test.md）。**已修复**：对齐 `backup.go` 的 `importOne` 写法，在导入循环前置判断 `meta.Status == store.StatusDraft` 时设 `status = store.StatusDraft`，否则保持 Published。

## Spec 60 · 主题配置复刻 + 页面统一管理（2026-08）

- **主题 YAML**：`GET/PUT /api/v1/settings/style/yaml`（goccy/go-yaml 解析校验 → `SaveSection(style)` → `RebuildSite` 热更新）。设置页外观组「主题 YAML」tab（ThemeYamlTab.vue）直接编辑 site.yml `style` 段，与 AppearanceTab/高级·JSON 同源。`StyleConfig` 对齐 1.38 style 面（font-size code/codeblock、border-radius 七档、codeblock.highlightjs_theme、animated_avatar；header_prefix 值为 URL 时注入图标 background-image）。**注意**：main.css 不消费 `--radius-*` 变量，圆角运行时注入实际无效，颜色/`--fsp`/字体/左栏/全站背景才是有效旋钮。
- **页面统一管理**：PageList.vue 顶部 `apple-segmented` 三分段（普通页面/文库/友链），深链 `?module=wiki|friends`，内嵌 WikiDocsView/FriendList；数据仍存 wikidocs/links 表。**坑**：分段/module 逻辑必须放在 `useRouter()/useRoute()` 声明之后（TDZ），否则 PageList 挂载失败白屏。
