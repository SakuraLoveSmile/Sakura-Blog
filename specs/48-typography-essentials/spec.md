# Spec 48 · 排版核心补齐（代码行号 + grid/hashtag/blockquote 三插件）

- **分支**：`feature/frontend-gap` · **Wave 10** · **隔离等级**：中
- **主模块（独占）**：`backend/internal/post/shortcode.go`（grid/hashtag/blockquote 三插件）、`frontend/assets/css/tag-plugins.css`（grid/hashtag/blockquote 样式）、代码块行号渲染管线
- **前置**：无

## Why

项目已完成 Spec 06/16/20 等 30+ 标签插件，经与 `resource/hexo-theme-stellar/` 原始 Stellar 主题对比，仍有 4 项高优排版功能缺失或需补齐：

1. **代码块行号** — 技术博客刚需，几乎所有代码片段都需要行号引用（"参见第 42 行"）。当前 Chroma 服务端渲染输出 `WithLineNumbers(false)`（[post.go:116](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/post.go#L116)），不生成行号。这是本 spec 唯一需要实际代码变更的项目。
2. **grid 网格容器** — `{% grid %}` 标签插件在 shortcode.go 中已实现（[renderGrid:1361](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/shortcode.go#L1361)），CSS 在 main.css 中已存在（[#5122-5177](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/frontend/assets/css/main.css#L5122)），但缺少在 `tag-plugins.css` 中的 spec 注释分区，且未验证是否覆盖原版所有参数。
3. **hashtag 彩色标签** — 已实现（[renderHashtag:1519](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/shortcode.go#L1519)），CSS 已存在（[main.css:5178-5200](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/frontend/assets/css/main.css#L5178)）。需验证 color 色板与随机取色逻辑对齐原版。
4. **blockquote 增强引用** — 已实现 `indent` 参数（[renderBlockquote:2188](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/shortcode.go#L2188)）。需确认技术/非技术文章样式差异的 CSS 行为（[main.css:8505-8567](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/frontend/assets/css/main.css#L8505) 两套选择器共存）。

## 功能需求

| 功能 | 语法 | 当前状态 | 需做的事 |
|---|---|---|---|
| 代码块行号 | 标准 markdown 三反引号代码块 | 未实现（`WithLineNumbers(false)`，[post.go:116](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/post.go#L116)） | 开启 Chroma 行号输出 + 追加行号 CSS |
| grid 网格 | `{% grid [bg:box/card] [w:240px] [c:2] [gap:16px] [br:12px] %}...<!-- cell -->...{% endgrid %}` | 已实现（[renderGrid:1361](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/shortcode.go#L1361)），已验证对齐原版 | 在 tag-plugins.css 追加 spec 注释分区（无样式变更） |
| hashtag 彩色标签 | `{% hashtag text [href] [color:red] %}` | 已实现（[renderHashtag:1519](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/shortcode.go#L1519)），已验证对齐原版 | 在 tag-plugins.css 追加 spec 注释分区（无样式变更） |
| blockquote 增强引用 | `{% blockquote [indent:true/false] %} body {% endblockquote %}` | 已实现（[renderBlockquote:2188](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/shortcode.go#L2188)），已验证对齐原版 | 在 tag-plugins.css 追加 spec 注释分区（无样式变更） |

### 代码块行号 — 详细需求

**技术方案**：服务端 Chroma `WithLineNumbersInTable(true)` 模式。

#### Chroma 输出 HTML 结构

`WithLineNumbersInTable(true)` 模式下，Chroma 为每个代码块生成 `<table>` 结构：

```html
<pre class="chroma">
  <table class="lntable">
    <tbody>
      <tr>
        <td class="lntd">
          <pre class="chroma">
            <span class="lnt"> 1
</span><span class="lnt"> 2
</span><span class="lnt"> 3
</span>...
          </pre>
        </td>
        <td class="lntd">
          <pre class="chroma">
            <code>
              <span class="line">...</span>
              ...
            </code>
          </pre>
        </td>
      </tr>
    </tbody>
  </table>
</pre>
```

关键 class 名：
- `.lntable` — 外层 table
- `.lntd` — 左右两个 td 共用（左列行号 / 右列代码）
- `.lnt` — 行号个体（含前导空格填充，如 ` 1`、` 2`）
- `pre.chroma` — 嵌套两层 pre，外层 pre 是 Chroma 原始容器，内层 pre 是行号/代码各自容器

#### 与 `WithLineNumbers(true)`（纯 `<span>` 模式）对比

| 维度 | `WithLineNumbers(true)` | `WithLineNumbersInTable(true)`（选用） |
|---|---|---|
| 行号输出 | `<span class="ln">1</span>` 内嵌代码行内 | `<td class="lntd"><pre>...<span class="lnt">1</span>...</pre></td>` 独立列 |
| 复制代码 | 行号随代码一起被复制（需 JS 拦截） | 行号在独立列，自然不被复制 |
| CSS 复杂度 | 低 | 中（需处理 table 结构与嵌套 pre） |
| JS 依赖 | 无 | 无 |
| 对齐原版 Stellar | Stellar 使用前端 prism.js 行号，与本方案不同 | 本方案偏服务端，更轻量 |

**结论**：选用 `WithLineNumbersInTable(true)`，复制代码时不带行号是核心优势。

#### 配置变更

**`backend/internal/post/post.go`**（1 行变更）：

```
- Line 103: 注释 / 代码块经 Chroma 服务端高亮（CSS class 模式，class 前缀 chroma，无行号）。
+ Line 103: 注释 / 代码块经 Chroma 服务端高亮（CSS class 模式，class 前缀 chroma，table 行号）。

- Line 116: chromahtml.WithLineNumbers(false),
+ Line 116: chromahtml.WithLineNumbersInTable(true),
```

#### CSS 新增

**`frontend/assets/css/highlight.css`** — 在现有 `pre.chroma` 块（[第 7-16 行](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/frontend/assets/css/highlight.css#L7)）之后追加 table 模式适配样式。

新增样式，总计约 40 行：

```css
/* ---------- Spec 48: Chroma table 行号模式 ---------- */
pre.chroma table {
  display: block;
  width: 100%;
  overflow: auto;
  border-collapse: collapse;
}
pre.chroma table td {
  padding: 0;
  vertical-align: top;
}
pre.chroma .lntd:first-child {
  padding: 1rem 0 1rem 1rem;
  width: 3.5em;
  min-width: 3.5em;
  background: var(--block);
  -webkit-user-select: none;
  user-select: none;
  border-right: 1px solid var(--block-border);
}
pre.chroma .lntd:first-child pre {
  padding: 0;
  margin: 0;
  background: transparent;
  text-align: right;
  color: var(--text-p3);
  font-size: inherit;
  line-height: inherit;
}
pre.chroma .lntd:last-child {
  padding: 1rem;
  width: 100%;
}
pre.chroma .lntd:last-child pre {
  padding: 0;
  margin: 0;
  background: transparent;
}
pre.chroma .lnt {
  display: block;
  padding: 0 0.5em 0 0;
  color: var(--text-p3);
  font-size: 0.8125rem;
  line-height: 1.6;
}
```

**关键设计点**：

1. **`pre.chroma` 保持 `padding: 1rem`**（[highlight.css:12](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/frontend/assets/css/highlight.css#L12)）— 不修改现有 `pre.chroma` 的 padding，而是将 padding 分散到 `td` 上：行号列左 padding 1rem（`padding: 1rem 0 1rem 1rem`），代码列全 padding 1rem（`padding: 1rem`）。这样 table 内部结构对齐原有视觉间距。

2. **行号列宽度**：固定 `3.5em`（`width` + `min-width` 双保险），覆盖 1-999 行（4 位数含空格）。超出 999 行的代码块自动撑宽。

3. **`user-select: none`**：行号列禁止用户选择，复制代码时行号不被选中。

4. **分隔线**：行号列右侧 `border-right: 1px solid var(--block-border)` 与代码区域视觉区分。

5. **暗色模式**：行号列使用 `var(--block)` 背景，与代码背景一致；行号使用 `var(--text-p3)` 淡化。

6. **嵌套 pre**：Chroma 输出两层 `pre.chroma`（外层容器 + 内层行号/代码 pre），内层 pre 设置 `padding: 0; margin: 0; background: transparent` 避免样式冲突。

### grid 容器 — 详细需求

#### 语法

```
{% grid [bg:box/card] [w:240px] [c:2] [gap:16px] [br:12px] %}
<!-- cell -->
左列内容（支持多段落 Markdown）
<!-- cell -->
右列内容（支持多段落 Markdown）
{% endgrid %}
```

#### 参数解析

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| bg | 枚举 | 无（不设背景） | `box` = 浅色块背景（`var(--block)`），`card` = 卡片背景（`var(--card)`）+ 阴影+hover 上浮 |
| w | CSS 长度 | `240px`（当 c 未指定时） | 动态列：`grid-template-columns: repeat(auto-fill, minmax(W, 1fr))` |
| c | 整数 | 无（当 w 指定时） | 固定列数：`grid-template-columns: repeat(C, 1fr)` |
| gap | CSS 长度 | `16px`（CSS 默认） | 网格间距：`grid-gap: GAP` |
| br | CSS 长度 | 无（继承全局圆角） | 每个 cell 的圆角：`border-radius: BR` |

**注意**：`w` 和 `c` 互斥，同时指定时 `w` 优先（[shortcode.go:1365-1367](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/shortcode.go#L1365)）。

#### HTML 输出

```html
<div class="tag-plugin grid" bg="box" style="grid-template-columns: repeat(2, 1fr);grid-gap:16px;">
  <div class="cell" style="border-radius:12px;">
    <p>左列内容（经 goldmark 渲染）</p>
  </div>
  <div class="cell" style="border-radius:12px;">
    <p>右列内容（经 goldmark 渲染）</p>
  </div>
</div>
```

#### 当前实现验证

对照原版 `resource/hexo-theme-stellar/scripts/tags/lib/grid.js`：

| 项目 | 原版 Stellar | 本项目 | 结论 |
|---|---|---|---|
| 正则匹配 | `{% grid ... %}...{% endgrid %}` | 同（[reGrid:49](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/shortcode.go#L49)） | 一致 |
| 参数解析 | `ctx.args.map(args, ['bg', 'w', 'c', 'gap', 'br'])` | `parseNamedPositionalArgs` 同键（[1362](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/shortcode.go#L1362)） | 一致 |
| w/c 默认值 | `w = '240px'` 当两者皆空 | 同（[1365-1367](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/shortcode.go#L1365)） | 一致 |
| HTML 结构 | `<div class="tag-plugin grid" bg="..." style="...">` | 同（[1370-1383](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/shortcode.go#L1370)） | 一致 |
| Cell 分割 | `content.split(/<!--\s*cell(.*?)-->/g)` | `reCellSplit.Split(body, -1)`（[1386](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/shortcode.go#L1386)） | 语义一致 |
| Cell 渲染 | `ctx.render.renderSync({text: cell, engine: 'markdown'})` | `renderMD(cell)`（[1397](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/shortcode.go#L1397)） | 一致 |
| 输出 `columns` 属性 | 原版 `grid.js` 输出 `columns` 属性 | 本项目不输出 | 可接受（main.css 无对应选择器） |

**结论**：实现完全对齐原版，无需修改 shortcode.go。

#### CSS 现状

| 选择器 | 位置 | 作用 |
|---|---|---|
| `.tag-plugin.grid` | [main.css:5122](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/frontend/assets/css/main.css#L5122) | 网格容器：`display: grid; grid-gap: 16px` |
| `.tag-plugin.grid[bg]>.cell` | [main.css:5141](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/frontend/assets/css/main.css#L5141) | 有背景的 cell：padding 1rem，圆角 16px |
| `.tag-plugin.grid[bg='box']>.cell` | [main.css:5146](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/frontend/assets/css/main.css#L5146) | box 背景：`var(--block)` |
| `.tag-plugin.grid[bg='card']>.cell` | [main.css:5149](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/frontend/assets/css/main.css#L5149) | card 背景：`var(--card)` + 阴影 + hover 上浮 |
| 暗色/亮色适配 | [main.css:5156-5177](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/frontend/assets/css/main.css#L5156) | 完整覆盖三种 data-theme 状态 |

### hashtag 彩色标签 — 详细需求

#### 语法

```
{% hashtag text [href] [color:red] %}
```

#### 参数解析

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| text | 字符串（必填） | 无 | 标签显示文本 |
| href | 字符串（可选） | 无 | 链接 URL，不指定时渲染为无 href 的 `<a>`（不可点击） |
| color | 枚举（可选） | 随机轮询 | 色板：[red, orange, yellow, green, cyan, blue, purple]（[hashtagColors:1513](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/shortcode.go#L1513)） |

#### 颜色轮询机制

```go
// [shortcode.go:1527-1529](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/shortcode.go#L1527)
if color == "" {
    idx := atomic.AddUint64(&hashtagColorIdx, 1) - 1
    color = hashtagColors[idx%uint64(len(hashtagColors))]
}
```

- 原子计数器 `hashtagColorIdx`（[shortcode.go:182](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/shortcode.go#L182)）保证并发安全
- 7 色循环：red → orange → yellow → green → cyan → blue → purple → red...
- 对齐原版 `hashtag.js` 的 `tag_colors` 数组与 `tag_index` 轮询逻辑

#### HTML 输出

```html
<a class="tag-plugin colorful hashtag" color="red" href="/tags/example/">
  <svg t="1701408144765" class="icon" viewBox="0 0 1024 1024" width="200" height="200">
    <path d="M426.6 64.8..."></path>
  </svg>
  <span>text</span>
</a>
```

- SVG 图标（[hashtagSVG:1516](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/shortcode.go#L1516)）与原版 `hashtag.js` 内联 SVG 完全一致（p-id 属性差异不影响渲染）
- `color` 属性值直接透传至 HTML 属性，CSS 通过 `[color=red]` 等属性选择器控制颜色

#### 当前实现验证

对照原版 `resource/hexo-theme-stellar/scripts/tags/lib/hashtag.js`：

| 项目 | 原版 Stellar | 本项目 | 结论 |
|---|---|---|---|
| 色板 | `['red', 'orange', 'yellow', 'green', 'cyan', 'blue', 'purple']` | 同（[hashtagColors:1513](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/shortcode.go#L1513)） | 一致 |
| 轮询 | `tag_index` 循环 | `hashtagColorIdx` 原子循环（[1528-1529](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/shortcode.go#L1528)） | 一致 |
| 默认色 | `theme.config.tag_plugins.hashtag.default_color`（可选） | 无站点配置支持 | 差异，但原版 default_color 也很少用 |
| HTML 结构 | `<a class="tag-plugin colorful hashtag" color="red" href="...">` | 同（[1533-1538](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/shortcode.go#L1533)） | 一致 |
| 内联 SVG | 同 | 同（[hashtagSVG:1516](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/shortcode.go#L1516)） | 一致 |
| 文本包装 | `<span>` | 同（[1540](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/shortcode.go#L1540)） | 一致 |

#### CSS 现状

| 选择器 | 位置 | 作用 |
|---|---|---|
| `.md-text .tag-plugin.hashtag` | [main.css:5178](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/frontend/assets/css/main.css#L5178) | 胶囊形标签：圆角 100px，`var(--theme-block)` 背景，flex 对齐 |
| `.md-text .tag-plugin.hashtag span` | [main.css:5194](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/frontend/assets/css/main.css#L5194) | 文本微间距 `margin: 0 2px` |
| `.md-text .tag-plugin.hashtag:hover` | [main.css:5197](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/frontend/assets/css/main.css#L5197) | hover 反转背景/文字色 |

### blockquote 增强引用 — 详细需求

#### 语法

```
{% blockquote [indent:true/false] %}
引用正文（支持多段落 Markdown，可含列表、代码等）
{% endblockquote %}
```

#### 参数解析

| 参数 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| indent | 布尔 | `false`（空字符串） | 段落首行缩进 2 字符（`text-indent: calc(var(--fsp) * 2)`） |

#### HTML 输出

```html
<div class="tag-plugin blockquote" indent="true">
  <p>引用正文（经 goldmark 渲染，换行已移除）</p>
</div>
```

注意：`renderMD(body)` 渲染后的换行符被 `strings.ReplaceAll(..., "\n", "")` 移除（[shortcode.go:2192](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/shortcode.go#L2192)），对齐原版 `blockquote.js` 的 `.split('\n').join('')` 行为。

#### 当前实现验证

对照原版 `resource/hexo-theme-stellar/scripts/tags/lib/blockquote.js`：

| 项目 | 原版 Stellar | 本项目 | 结论 |
|---|---|---|---|
| 参数 | `['indent']` | 同（[2189](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/shortcode.go#L2189)） | 一致 |
| HTML 结构 | `<div class="tag-plugin blockquote" indent="true">` | 同（[2191](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/shortcode.go#L2191)） | 一致 |
| 换行移除 | `.split('\n').join('')` | `strings.ReplaceAll(..., "\n", "")`（[2192](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/shortcode.go#L2192)） | 一致 |
| indent 默认值 | `args.indent`（undefined 时空字符串） | `named["indent"]` 空字符串（[2191](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/post/shortcode.go#L2191)） | 一致 |

#### 技术/非技术文章样式差异

CSS 中两套选择器共存（[main.css:8505-8567](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/frontend/assets/css/main.css#L8505)）：

```css
/* 非技术文章样式（story 类型） */
.l_body[type=story] .md-text.content>blockquote,
/* 技术文章样式（默认 / tech 类型） */
.md-text.content .tag-plugin.blockquote {
  text-align: center;
  --inset: calc(var(--fsp) * 2);
  margin-left: var(--inset);
  margin-right: var(--inset);
  background: var(--block);
  padding: 1rem 2rem;
  border-radius: 16px;
}
```

`l_body` 的 `type` 属性由模板层设定（[base.html:17](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/template/layout/base.html#L17)）：

```html
<div class="l_body {{if eq .Page.Layout "post"}}content{{else}}index{{end}}"
     id="start" layout="{{.Page.Layout}}" type="{{.Page.Type}}">
```

`Page.Type` 的可能值（[model.go:574](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/backend/internal/template/model.go#L574)）：`tech` / `story`。

两套选择器实际共享同一组 CSS 规则，`type=story` 前缀仅用于选择器权重优先级，视觉上无差异。

#### CSS 现状

| 选择器 | 位置 | 作用 |
|---|---|---|
| `.tag-plugin.blockquote` | [main.css:6174](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/frontend/assets/css/main.css#L6174) | 相对定位容器 |
| `.tag-plugin.blockquote p:not([class])` | [main.css:6177](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/frontend/assets/css/main.css#L6177) | 段落缩进重置 |
| `.tag-plugin.blockquote[indent='true'] p:not([class])` | [main.css:6180](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/frontend/assets/css/main.css#L6180) | indent 开启时首行缩进 2 字符 |
| `.md-text.content .tag-plugin.blockquote` | [main.css:8506](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/frontend/assets/css/main.css#L8506) | 内容区居中+背景+圆角+引号图标 |
| `.md-text.content .tag-plugin.blockquote:before` | [main.css:8533](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/frontend/assets/css/main.css#L8533) | 左引号图标（SVG mask） |
| `.md-text.content .tag-plugin.blockquote:after` | [main.css:8545](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/frontend/assets/css/main.css#L8545) | 右引号图标（SVG mask） |
| `.md-text.content .tag-plugin.blockquote p:not([class])` | [main.css:8557](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/frontend/assets/css/main.css#L8557) | 段落颜色/字号 |

## What Changes

### `backend/internal/post/post.go` — 1 行变更 + 1 行注释更新

- **Line 103**：注释更新
  ```
  - 代码块经 Chroma 服务端高亮（CSS class 模式，class 前缀 chroma，无行号）。
  + 代码块经 Chroma 服务端高亮（CSS class 模式，class 前缀 chroma，table 行号）。
  ```
- **Line 116**：Chroma 格式选项变更
  ```
  - chromahtml.WithLineNumbers(false),
  + chromahtml.WithLineNumbersInTable(true),
  ```

### `frontend/assets/css/highlight.css` — 追加行号样式

在 `pre.chroma` 块（[highlight.css:7-16](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/frontend/assets/css/highlight.css#L7)）之后追加约 40 行 table 行号模式样式（CSS 代码见上文"CSS 新增"章节）。

### `frontend/assets/css/tag-plugins.css` — 追加 spec 注释分区

在文件末尾（[tag-plugins.css:378](file:///Users/sakurasep/Documents/Code/Project/Blog/feat-frontend-gap/frontend/assets/css/tag-plugins.css#L378) 之后）追加 3 个 spec 注释分区：

```css
/* ---------- Spec 48: grid 网格容器（对齐 grid.js） ---------- */
/* 样式在 main.css:5122-5177，此处无需重复定义 */

/* ---------- Spec 48: hashtag 彩色标签（对齐 hashtag.js） ---------- */
/* 样式在 main.css:5178-5200，此处无需重复定义 */

/* ---------- Spec 48: blockquote 增强引用（对齐 blockquote.js） ---------- */
/* 样式在 main.css:6174-6182（基础定位）+ 8506-8567（内容区样式），此处无需重复定义 */
```

### `backend/internal/post/shortcode.go` — 无变更

三个标签插件均已实现，经对照原版 js 确认完全一致，无需修改 shortcode.go。

## 技术决策

1. **代码行号 → Chroma table 模式**：不选 `WithLineNumbers(true)`（内联 `<span>` 行号会随代码被复制），不选前端 JS 方案（服务端渲染更直接，零 JS 依赖，PJAX 天然兼容）。`WithLineNumbersInTable(true)` 是 Chroma 官方推荐的行号方案。

2. **CSS 复用原则**：grid/hashtag/blockquote 三插件样式已在 `main.css` 中完整定义，`tag-plugins.css` 仅追加 spec 注释分区，不重复写 CSS。这是遵循现有 CSS 铁律（先 grep main.css 确认类是否已存在，仅补缺失）。

3. **行号列宽度**：固定 `3.5em`（`width` + `min-width` 双保险）覆盖 1-999 行（4 位数），超出部分自然撑宽。

4. **`pre.chroma` 保留 padding**：不修改现有 `pre.chroma` 的 `padding: 1rem`，而是将 padding 分散到 `td` 元素上，避免破坏现有布局。

5. **语法容错**：三个标签插件均遵循现有容错模式（未闭合/参数缺失 → 原样输出，不 panic 不吞正文），无需额外处理。

6. **seed 依赖**：代码块行号变更即时生效（goldmark 渲染时 Chroma 配置已变），无需 `-seed` 重导。grid/hashtag/blockquote 已有 ContentHTML 缓存，若需刷新需 admin 重存或 `-seed`。

## Impact

- **修改**：`backend/internal/post/post.go`（Line 103 注释 + Line 116 配置项，共 2 行变更）
- **新增 CSS**：`frontend/assets/css/highlight.css`（+~40 行 table 行号模式样式）
- **追加注释**：`frontend/assets/css/tag-plugins.css`（+3 个 spec 注释分区，约 6 行）
- **不影响**：路由、store、admin、model、base.html、任何 JS 文件、shortcode.go

## 验证

- [ ] `go build ./... && go vet ./...` 通过
- [ ] 代码块渲染出 `<table class="lntable">` 结构，左列显示行号，右列为高亮代码
- [ ] 选中代码块时行号不被选中（`-webkit-user-select: none; user-select: none` 生效）
- [ ] 复制代码块内容后粘贴到文本编辑器，行号不进入剪切板
- [ ] 行号从 1 开始连续递增，与代码行数一致
- [ ] 暗色模式下行号清晰可读，行号分隔线可见（`var(--text-p3)` 色 + `var(--block-border)` 分隔线）
- [ ] 亮色模式下行号清晰可读，行号分隔线可见
- [ ] 无行号的纯文本块（非代码块）不受影响，不产生空行号列
- [ ] `{% grid %}` 块渲染正常，`bg=box` 和 `bg=card` 两种背景样式正确
- [ ] `{% grid w:240px %}` 动态列模式正常，`{% grid c:3 %}` 固定列模式正常
- [ ] `{% grid bg=card br:12px gap:8px %}` 组合参数正常
- [ ] `{% hashtag %}` 颜色随机轮询正常，7 色循环，连续 7 个各不同色
- [ ] `{% hashtag text color:red %}` 指定色生效，覆盖随机色
- [ ] `{% hashtag text /tags/go color:blue %}` 带 href 时渲染为可点击链接
- [ ] `{% blockquote %}` 正常渲染，引号图标可见（`:before` / `:after` SVG mask）
- [ ] `{% blockquote indent:true %}` 段落首行缩进 2 字符
- [ ] 故意写坏语法（如 `{% grid` 未闭合、`{% hashtag` 无参数、`{% blockquote` 无 body），原样输出不 panic 不吞正文
- [ ] PJAX 导航后代码块行号仍然显示（无 JS 依赖，服务端渲染，天然 PJAX 兼容）
- [ ] 亮/暗色模式切换后各插件样式正确

## Out of Scope

- 前端 JS 行号方案（如 prism.js 客户端行号插件）— 本项目采用服务端 Chroma 渲染
- 行号点击跳转、行号高亮范围（如 `hl_lines`）— 未来可扩展，本 spec 不包含
- 行号起始值自定义（如 `start=10`）— Chroma 支持但本 spec 暂不涉及
- 其他 Chroma 格式选项（如 `WithHighlightStyle`、`WithTabWidth`）— 保持现有配置
- Stellar 原版 `blockquote` 的 `avatar`/`author`/`bg` 扩展参数 — 原版 blockquote.js 仅支持 `indent`，其余参数不存在
- Stellar 原版 `grid` 的 `columns` 属性 — 原版 `grid.js` 输出 `columns` 属性作为 HTML 属性，但 `main.css` 无对应选择器，本项目不输出该属性（与现有 renderGrid 一致）
- 站点级 `hashtag.default_color` 配置 — 原版支持但本 spec 暂不追加
- grid 的 cell 内嵌其他 shortcode — 现有 `renderMD(cell)` 已支持，无需额外处理