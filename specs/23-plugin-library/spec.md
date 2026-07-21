# Spec 23 · Console 插件库（标签插件速查 + 实时预览）

- **分支**：`feature/plugin-library` · **Wave 6** · **隔离等级**：低（纯 Console 前端）
- **主模块（独占）**：新 `console/src/modules/plugins/`（`plugins-data.ts` + `PluginLibrary.vue`）
- **前置**：无。与 Wave 6 其余四 spec 零文件交集，随时可做可合。

## Why

Stellar 主题自带 29 个标签插件（`{% note %}`/`{% tabs %}`/`{% chat %}` 等），语法各异、参数繁多，写文章时全靠翻 `shortcode.go` 源码或记忆。后台缺一个**随写随查、所见即所得**的速查手册：选中插件即看语法签名、参数表、可运行示例，并实时渲染出与前台**完全一致**的效果，一键复制语法到剪贴板。降低标签插件的使用门槛，减少试错成本。

## 功能需求

1. **插件元数据**：29 个插件的静态元数据数组——name、中文标题、分类、类型（行内/块级）、简介、语法签名、参数表（参数名/说明/默认值）、可运行示例。数据源对照 `internal/post/shortcode.go` 现有实现逐一核对。
2. **分类与搜索**：左侧分类筛选（提示标注 / 布局容器 / 媒体播放 / 数据图表 / GitHub，含「全部」）+ 关键字搜索（匹配 name/title/desc）；列表项标注行内/块级徽标。
3. **语法速查**：右侧展示选中插件的语法签名（代码块）+ 参数表 + 简介，语法可一键复制。
4. **可编辑示例**：示例代码区可直接修改，修改后防抖触发重新渲染。
5. **实时渲染预览**：走已有 `POST /api/v1/preview/markdown`（复用前台 goldmark + shortcode 渲染管线，**无需新后端端点**）；预览容器用 **iframe srcdoc**，注入 `/assets/css/main.css` + `/assets/css/tag-plugins.css` + `/assets/css/highlight.css` 与渲染 HTML，保证插件样式与前台完全一致（Console 的 Tailwind 环境没有 Stellar 样式）。
6. **亮/暗切换**：预览顶部提供亮/暗切换，切换 iframe 内 `<html data-theme>`，与前台主题机制一致。
7. **一键复制**：复制语法签名 / 复制示例代码，复制后短暂反馈「已复制」。

## What Changes

- 新 `console/src/modules/plugins/plugins-data.ts`：
  - `PluginMeta` / `PluginParam` / `PluginCategory` / `PluginType` 类型定义。
  - `plugins` 元数据数组（29 项）+ `PLUGIN_CATEGORIES` 分类顺序。
- 新 `console/src/modules/plugins/PluginLibrary.vue`：
  - 左侧：搜索框 + 分类胶囊（带计数）+ 插件列表（行/块徽标 + 标题 + name）。
  - 右侧：标题区（标题/语法 code/分类/类型徽标 + 简介）→ 语法块（复制）→ 参数表 → 示例编辑区（复制 + 防抖渲染）→ 预览区（iframe srcdoc + 亮/暗切换 + 渲染中态）。
  - 预览：`previewApi.render(example)` 取 `data.html`，拼入 srcdoc；`previewTheme` 控制 `data-theme` 与 body 背景。
- `console/src/router/index.ts`：+1 路由 `/plugins` → `PluginLibrary.vue`。
- `console/src/layouts/DefaultLayout.vue`：侧边栏 +1 菜单项「插件」（`ri:puzzle-line`，置于「友链」后）。

## 实现步骤

1. `plugins-data.ts`：对照 shortcode.go 写全 29 个插件元数据。
2. `PluginLibrary.vue`：左列表 + 右详情 + iframe 预览 + 复制 + 亮暗切换。
3. 路由 + 侧边栏菜单接入。
4. `npm run build` 通过；起服务冒烟：任选 mermaid/chat/swiper 等复杂块级插件，预览渲染与前台一致，亮/暗切换正常，复制可用。

## 技术决策（无需再问）

- **元数据放前端静态文件**：插件变更频率低，与 shortcode.go 同步靠人工维护；不引后端表，零迁移成本。
- **预览复用 preview/markdown**：不新增端点，渲染管线（goldmark + shortcode + icon）与前台单一来源，杜绝「后台预览与前台不一致」。
- **iframe srcdoc 而非直接 v-html**：Console 全局是 Tailwind，Stellar 的 `.tag-plugin`/`.md-text` 样式需独立样式环境；iframe 隔离 + 注入前台 CSS 是最稳的一致性方案，且天然支持亮/暗 `data-theme` 切换。
- **不做**：插件在线编辑/新增、示例持久化、预览内 KaTeX/Mermaid 的 JS 二次渲染（仅静态 HTML 结构预览，复杂交互脚本不在本期）。

## Impact

- **新增**：`console/src/modules/plugins/plugins-data.ts`、`console/src/modules/plugins/PluginLibrary.vue`、`specs/23-plugin-library/spec.md`
- **修改**：`console/src/router/index.ts`（+1 路由）、`console/src/layouts/DefaultLayout.vue`（+1 菜单项）
- **不影响**：Go 后端、前台模板与路由、其余 Console 模块

## ADDED Requirements

### Requirement: Console 插件库
The system SHALL provide a Console plugin library page listing all 29 tag plugins with syntax reference, editable example, and live front-end-consistent preview.

#### Scenario: 分类与搜索
- **WHEN** 选择某分类或输入关键字
- **THEN** 列表按分类过滤并按 name/title/desc 模糊匹配，显示匹配计数

#### Scenario: 语法速查与复制
- **WHEN** 选中某插件
- **THEN** 展示其语法签名与参数表；点击复制将语法写入剪贴板并反馈「已复制」

#### Scenario: 实时渲染预览
- **WHEN** 修改示例代码
- **THEN** 防抖后经 preview/markdown 渲染，iframe 内以前台样式呈现结果

#### Scenario: 亮/暗切换
- **WHEN** 切换预览主题
- **THEN** iframe 内 `data-theme` 切换，插件配色随之前台一致地变化

## 验证

1. `npm run build` 通过（vue-tsc 无类型错误）。
2. Console 打开「插件」页：29 个插件全部列出，分类计数正确。
3. 任选 mermaid / chat / swiper / gallery 等复杂块级插件，预览渲染结构与前台一致。
4. 亮/暗切换预览正常；复制语法/示例按钮可用。
5. 搜索「gh」命中 ghcard/ghrepo/ghuser 三个 GitHub 插件。

## Out of Scope

- 插件在线新增/编辑、示例云端保存
- 预览内 Mermaid/KaTeX/Swiper 等 JS 交互的二次初始化（仅静态结构 + 样式）
- 移动端适配（Console 为桌面后台）
