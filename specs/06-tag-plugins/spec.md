# Spec 06 · Stellar 标签插件扩展包

- **分支**：`feature/tag-plugins` · **Wave 1** · **隔离等级**：高
- **主模块（独占）**：`internal/post/shortcode.go` + 新 `assets/css/tag-plugins.css`

## Why

现仅实现 `note`、`folding` 两个 Stellar 标签插件。Stellar 主题还有大量常用标签插件（tabs/timeline/gallery/button/checkbox/mermaid 等），是写作体验的核心。本 spec 补齐一批，全部落在 `shortcode.go` 预处理管线，与其它功能零耦合。

## What Changes

- `internal/post/shortcode.go`：在现有 `processShortcodes` 正则预处理框架内新增标签插件解析与 HTML 生成：
  - **tabs**：多标签页容器（`{% tabs id %}` + `<!-- tab 标题 -->` 分隔）
  - **timeline**：时间线（`{% timeline 标题 %} ... {% endtimeline %}`，内部 `<!-- node 时间 -->`）
  - **gallery**：图片网格（`{% gallery %}` 包裹一组图片 markdown）
  - **button**：按钮链接（`{% button 文字, url, icon, color %}`）
  - **checkbox / checklist**：带样式复选项（`{% checkbox checked 文本 %}`）
  - **mermaid**：图表（`{% mermaid %}...{% endmermaid %}` → `<div class="mermaid">原文</div>`，前端引 mermaid CDN 渲染）
- 新增 `assets/css/tag-plugins.css`：仅补 main.css 中**确无**的插件样式；能复用 main.css 现成类的**不重写**。
- `internal/template/partial/head.html`：`+1` 行 `<link rel="stylesheet" href="/assets/css/tag-plugins.css">`；mermaid 需再 `+1` 行 mermaid CDN `<script>`（`initialize({startOnLoad:true})`，PJAX 后需 `mermaid.run()` 重扫）。

## 技术决策（无需再问）

- **实现方式**：延续现有正则预处理（`processShortcodes`），不引第三方 shortcode 库；`renderMD` 闭包渲染子片段（与 folding 同款）。
- **严格对齐 Stellar**：每个插件的 **HTML 结构与 class 名**必须对照 `../resource/hexo-theme-stellar/scripts/tags/` 和 `layout/` 产出，先 grep `main.css` 看类是否已存在（tabs/timeline/gallery 在 Stellar 多为 `.tabs`/`.timeline`/`.gallery` 系列），已存在则**零新增 CSS**。
- **mermaid**：CDN `https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.min.js`；主题跟随暗色（`theme: 'dark'|'default'` 依 `data-theme`）。
- **语法容错**：未闭合/参数缺失时原样输出，不 panic、不吞正文。
- **seed 依赖**：新插件生效需 `-seed` 重导或 admin 重存（ContentHTML 是预渲染缓存）。

## Impact

- **新增**：`assets/css/tag-plugins.css`
- **修改**：`internal/post/shortcode.go`、`internal/template/partial/head.html`（+1~2 行）
- **不影响**：路由、store、admin、model、base.html

## ADDED Requirements

### Requirement: 扩展标签插件
The system SHALL render Stellar-style tag plugins (tabs, timeline, gallery, button, checkbox, mermaid) from markdown into theme-aligned HTML.

#### Scenario: tabs 渲染
- **WHEN** 文章含 `{% tabs %}` 块
- **THEN** 输出 Stellar `.tabs` 结构，点击标签切换面板

#### Scenario: mermaid 图表
- **WHEN** 文章含 `{% mermaid %}` 块
- **THEN** 输出 `<div class="mermaid">`，前端 mermaid.js 渲染为图，暗色模式配色正确

#### Scenario: 语法错误容错
- **WHEN** 标签插件语法不完整
- **THEN** 原样输出文本，不破坏整篇渲染

## 验证

1. `go build ./... && go vet ./...` 通过
2. 写一篇覆盖全部新插件的测试文（放 `content/posts/tag-plugins-test.md`），`-seed` 后访问，逐个目视对齐 Stellar 观感
3. 亮/暗模式、PJAX 导航后 mermaid 仍能渲染（`pjax:complete` 重扫）
4. 故意写坏一个插件，确认页面其余正常

## Out of Scope

- KaTeX 数学（见 Spec 07）、代码高亮（已完成）
- 需要后端数据的插件（如 sites 动态友链——已有静态 links）
