# Spec 01 · 代码高亮（Chroma 服务端渲染）

- **分支**：`feature/syntax-highlight`
- **主模块**：`internal/post`（Markdown 渲染管线）
- **隔离等级**：高（不碰路由逻辑、store、admin、model）

## Why

当前 goldmark 渲染的代码块只有裸 `<pre><code>`，无语法高亮。三分支整合时 trae 的 Chroma 高亮未被选入，现补齐。选择**服务端高亮**（编译期生成带 class 的 HTML）而非前端 highlight.js，理由：SSR 一致、无额外前端 JS、与现有「ContentHTML 预渲染缓存」架构契合。

## What Changes

- `internal/post/post.go`：给 `NewService` 的 goldmark 实例挂 `goldmark-highlighting/v2` 扩展，启用 **CSS class 模式**（`WithClasses(true)`），class 前缀 `chroma`。
- 新增 `assets/css/highlight.css`：Chroma 令牌配色，**亮/暗双色**（`:root` 亮色 + `:root[data-theme="dark"]` 暗色覆盖），容器背景/圆角复用主题变量。
- `internal/template/partial/head.html`：新增**一行** `<link rel="stylesheet" href="/assets/css/highlight.css">`。
- `go.mod` / `go.sum`：新增 `github.com/alecthomas/chroma/v2` + `github.com/yuin/goldmark-highlighting/v2`，`go mod tidy`。

## 技术决策（agent 无需再询问）

- **依赖**：`github.com/yuin/goldmark-highlighting/v2`（配套 chroma v2）。
- **class 模式而非 inline style**：`highlighting.NewHighlighting(highlighting.WithFormatOptions(chromahtml.WithClasses(true)))`。inline style 无法响应暗色模式，故必须用 class + CSS。
- **是否显示行号**：不显示（`WithLineNumbers(false)`），对齐 Stellar 代码块观感。
- **配色**：亮色参考 Chroma `github` 风格、暗色参考 `github-dark`，用这两套生成 CSS 令牌（agent 可用 `chroma --html-styles` 生成后手工合并到一个文件的两个作用域，或手写核心令牌 `.chroma .k/.s/.c/.n/.o/.m` 等）。
- **代码块容器**：**先 grep `assets/css/main.css` 是否已有 `pre`/`code`/`figure.highlight`/`.copycode` 样式**（记忆铁律）。若已有容器样式（内边距/圆角/字体/copycode 按钮），`highlight.css` **只定义令牌前景色**，不得重复定义容器 padding/background，避免与 Stellar 现成样式打架。仅当容器无背景时才用 `var(--card)` 补背景。
- **不改渲染产物存量数据**：开启高亮后需 `-seed` 重新导入或 admin 重新保存文章，ContentHTML 才带 chroma class（与 AutoHeadingID 同理，写入 spec 验证步骤）。

## Impact

- **新增**：`assets/css/highlight.css`
- **修改**：`internal/post/post.go`（goldmark 装配）、`internal/template/partial/head.html`（+1 link）、`go.mod`、`go.sum`
- **不影响**：所有路由 handler、store、admin、model.go、base.html、其它模板
- **副作用（正向）**：admin 保存文章走 `posts.Render`、seed 也走同一实例，自动获得高亮，无需额外改动

## 与其他 spec 的冲突点

- 唯一潜在共享文件是 `head.html`——本 spec 是**唯一**修改者。其它三个 spec 均不碰 `head.html`、`post.go`、`highlight.css`。**零冲突**。

## ADDED Requirements

### Requirement: 代码块服务端语法高亮

The system SHALL render fenced code blocks with server-side syntax highlighting using Chroma CSS classes.

#### Scenario: 带语言标注的代码块
- **WHEN** 文章含 ```` ```go ```` 围栏代码块并经 seed/保存渲染
- **THEN** 输出 HTML 的代码区包含 `class="chroma"` 容器，关键字/字符串/注释等 token 带对应 `.k`/`.s`/`.c` class

#### Scenario: 暗色模式配色
- **WHEN** 页面 `data-theme="dark"`
- **THEN** 代码令牌颜色切换为暗色调色板（由 `highlight.css` 的 `:root[data-theme="dark"] .chroma ...` 提供），且与页面背景对比度充足

#### Scenario: 无语言/未知语言代码块
- **WHEN** 代码块无语言标注或语言未知
- **THEN** 仍正常渲染为 `.chroma` 容器（fallback 纯文本），不报错、不破坏页面

## 验证

1. `go build ./... && go vet ./...` 通过
2. 起服务（`go run ./cmd/server -dev -root <dev绝对路径> -seed`，端口按 AGENTS.md 分配），访问含代码块的文章
3. `curl -s http://localhost:<port>/post/<slug> | grep -c 'class="chroma"'` > 0
4. 浏览器切换亮/暗模式，代码块配色随之切换，无「黑字黑底」低对比问题
5. 确认 copycode 复制按钮（若原主题有）仍正常，未被 highlight.css 破坏

## Out of Scope

- 前端 highlight.js / 行高亮 / 代码折叠 / diff 高亮
- Mermaid、KaTeX 等其它 goldmark 扩展（另立 spec，注意它们也会改 `post.go` 与 `head.html`，若并行需与本 spec 协调）
