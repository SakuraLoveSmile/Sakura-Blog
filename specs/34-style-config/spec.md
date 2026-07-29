# Spec 34 · 外观样式可配置化（原版 style 段对齐）

- **分支**：`feature/style-config` · **Wave 8** · **隔离等级**：中
- **主模块**：`config/site.go`（+style 段）+ `internal/template/partial/head.html`（CSS 变量注入）+ `assets/css/main.css`（变量化改造点位）
- **前置**：无（与 33 并行安全：33 动 sidebar/rightbar 结构，本 spec 只动 head 注入与变量）

## Why

原版 `_config.yml` 的 style 段提供全套外观配置，本项目对应能力全部硬编码。差距清单共 6 组：主题色/圆角、字体、leftbar 与全站背景、代码块高亮主题、prefers_theme 默认偏好与标题前缀等杂项、排版行为（text-align/smooth_scroll/indent/article type）。

## 功能需求（site.yml 新增 style 段，键名对照原版 `_config.yml` style）

| 组 | 配置键 | 行为 |
|---|---|---|
| 主题色 | `style.color.{theme,accent,link,button,hover}` | 覆盖 main.css 对应 CSS 变量（`--theme`/`--accent` 等，先 grep main.css 确认实际变量名） |
| 圆角 | `style.border-radius.{card,block,bar,image,badge}` | 五档圆角变量覆盖 |
| 字体 | `style.font-size.{root,body}`、`style.font-family.{body,code,codeblock}` | 注入 `:root` 变量；LXGW 文楷保留为缺省 body 字体（本项目既有方案） |
| leftbar 背景 | `style.leftbar.{background,blur-px,blur-bg}` | 纯色/渐变/图片 + blur，对照原版 `source/css/_layout/leftbar.styl` 的变量消费点 |
| 全站背景 | `style.site.{background,blur}` | 背景图 + siteblur 遮罩（注意 CLAUDE.md「siteblur 遮罩」坑点章节） |
| 代码块 | `style.codeblock.{highlightjs_theme,scrollbar}` | 高亮主题 URL 可换（缺省保持现状 highlight.css）；滚动条样式 |
| 杂项 | `style.prefers_theme`（auto/light/dark 缺省偏好）、`style.header_prefix`（h2-h5 前缀符号）、`style.paginator.{prev,next}` 箭头图、`style.error_page` 404 插图、`style.loading` 文案、`style.gradient` | 逐项注入 |
| 排版 | `style.text-align`、`style.smooth_scroll`、`article.indent`、`article.type`（tech/story） | body class 或变量切换 |

## What Changes

- `config/site.go` + `site.yml`：+style 段结构体与缺省值（**全部缺省 = 现状**，配置键一致性测试同步扩充）。
- `partial/head.html`：`<style>:root{...}</style>` 注入段——仅输出用户显式配置的变量，未配置不输出（保持 main.css 缺省生效）。
- `assets/css/main.css`：**只在变量缺失处补 `var(--x, fallback)` 引用点**，不重写样式（遵循「补样式前先 grep main.css」铁律与 Stellar CSS 剥离规范）。
- `partial/error404.html` / 分页器模板：插图与箭头图改为读配置（缺省内置资源）。
- `layout/base.html`：prefers_theme 缺省偏好接入现有 toggleTheme 初始化；header_prefix/article.type 输出 body class。

## 实现步骤

1. 精读原版 `_config.yml` style 段全量键 + `source/css/_variables.styl`（或同等文件），映射到 main.css 实际编译产物中的变量名，产出「原版键 → CSS 变量 → main.css 行号」对照表（本步是质量关键）。
2. site.go style 段解析 + 缺省值 + 一致性测试。
3. head.html 变量注入段（含 dark 模式双轨：dark 下的变量覆盖对照原版 dark styl）。
4. 主题色/圆角/字体三组先行（纯变量覆盖，风险低）。
5. leftbar/全站背景（注意 siteblur 坑点，改前读 CLAUDE.md 对应章节）。
6. codeblock 主题 URL、分页箭头、404 插图、loading、header_prefix、排版组。

## 技术决策（无需再问）

- **不做 Console 设置 UI**：本期只做 site.yml 配置能力；Console「外观设置」tab 另行随控制台批次排期（避免与 42-seo-codeinject 的 settings 改动冲突）。
- **注入而非编译**：不引入 stylus 运行时编译，全部走 CSS 变量覆盖；原版 style 键若在编译产物中无对应变量且无法用变量表达（如深度选择器改写），标记「不支持」写入 spec 完成记录，不硬造样式。
- **LXGW 字体保留**：font-family 可配但缺省仍为现有文楷方案，不回退原版系统字体栈。
- **MathJax/algolia 不做**：仅 KaTeX/FTS 路线已定（差距清单可选项，明确放弃）。

## Impact

- **新增**：style 配置段
- **修改**：site.go/site.yml、head.html、main.css（变量引用点）、error404.html、base.html、分页器模板
- **不影响**：shortcode、store、api、console

## 验证

- [ ] 零配置时全站与现状逐像素一致（亮/暗双主题目检）
- [ ] 主题色/圆角/字体/背景/代码块主题逐项改配置可见生效（亮暗双轨都对）
- [ ] prefers_theme: dark 时首次访问直接进入暗色
- [ ] 404 插图/分页箭头/loading 文案可替换
- [ ] 配置键一致性测试覆盖 style 全部键
- [ ] `make test` 全绿
