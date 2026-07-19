# Spec 07 · KaTeX 数学公式（服务端 + 前端渲染）

- **分支**：`feature/katex-math` · **Wave 1** · **隔离等级**：高
- **主模块（独占）**：`internal/post/post.go`（goldmark 装配）

## Why

技术博客常需数学公式，当前 goldmark 不解析 `$...$` / `$$...$$`。补 KaTeX 支持，全部落在 goldmark 实例装配 + head.html，与标签插件（改 shortcode.go）互不干扰。

## What Changes

- `internal/post/post.go`：给 `NewService` 的 goldmark 实例挂数学扩展，保留公式定界符到 HTML（推荐 `github.com/litao91/goldmark-mathjax` 或 `github.com/FurqanSoftware/goldmark-katex`）。采用**前端 KaTeX 渲染**方案时，扩展只需把 `$...$` 输出为带定界符/`<span class="math">` 的节点，交给前端 auto-render。
- `internal/template/partial/head.html`：`+` KaTeX CSS + JS + auto-render `<script>`（CDN），并在 `DOMContentLoaded` 与 `pjax:complete` 调 `renderMathInElement`。
- `go.mod`/`go.sum`：如选服务端渲染方案则加依赖并 `go mod tidy`；纯前端方案则无 Go 依赖（仅 head.html）。

## 技术决策（无需再问）

- **方案**：**前端 KaTeX auto-render**（`katex@0.16` + `contrib/auto-render`）。理由：零 Go 依赖、避免 CGO/构建复杂度、与 PJAX 兼容简单。goldmark 侧只需确保 `$...$` 不被转义/破坏——用 `html.WithUnsafe()`（已开）即可透传，无需额外扩展；若行内 `$` 被 goldmark 拆坏，则加轻量 math 扩展仅做定界符保护。
- **CDN**：`https://cdn.jsdelivr.net/npm/katex@0.16/...`（css + katex.min.js + contrib/auto-render.min.js）。
- **定界符**：`$$...$$`（块级）、`$...$`（行内）、`\(...\)`、`\[...\]`。
- **PJAX**：`pjax:complete` 后对 `.md-text` 重跑 `renderMathInElement`。
- 与 Spec 06 都属渲染增强但**不同文件**（06=shortcode.go，07=post.go/head.html）。head.html 为共享追加点，按 README 顺序合并。

## Impact

- **修改**：`internal/post/post.go`（如需 math 扩展）、`internal/template/partial/head.html`（+KaTeX 三件套 + 渲染脚本）
- **不影响**：路由、store、admin、model、base.html、shortcode.go

## ADDED Requirements

### Requirement: 数学公式渲染
The system SHALL render inline `$...$` and block `$$...$$` math using KaTeX.

#### Scenario: 行内与块级公式
- **WHEN** 文章含 `$E=mc^2$` 与 `$$\int_0^1 x\,dx$$`
- **THEN** 页面渲染为排版正确的数学公式

#### Scenario: PJAX 导航后仍渲染
- **WHEN** 通过 PJAX 进入含公式文章
- **THEN** 公式正常渲染（`pjax:complete` 重跑 auto-render）

#### Scenario: 非公式文本不误伤
- **WHEN** 正文含单独 `$`（如价格 `$5`）
- **THEN** 不被误当公式（auto-render 默认忽略无配对定界符）

## 验证

1. `go build ./... && go vet ./...` 通过
2. 测试文含行内/块级/`\(\)`/`\[\]` 四种，`-seed` 后目视正确
3. 亮/暗模式公式清晰；PJAX 进入正常渲染
4. 含 `$5 和 $10` 之类文本不被误渲染

## Out of Scope

- 化学式 mhchem、公式编号自动化
- 标签插件（见 Spec 06）
