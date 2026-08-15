# Spec 70 · 设置页复刻 Stellar 配置

- **分支**：`feature/settings-stellar-parity`
- **主模块**：`backend/internal/config/`、`backend/internal/template/`、`backend/cmd/server/`、`frontend/console/src/modules/settings/`
- **前置**：Spec 68 Stellar 1.40 同步、Spec 57 设置页 Apple 对齐

## Why

Console 设置页已有多数站点、外观和内容段，但 Stellar `_config.yml` 中若干已建模字段没有可视化入口，部分字段保存后也没有接入前台渲染。此工作流补齐适用字段的模型、热更新、设置表单和运行时消费，保留现有旧配置的默认行为。

## What Changes

### 后端

- 补齐文章 `type`、`banner_ratio`、`auto_excerpt`、`auto_banner` 与分类/标签映射；`auto_banner` 使用本地 `default.banner` 回退。
- 补齐首页评论开关、搜索正文/范围/排除路径、Notebook 默认设置。
- 补齐 canonical、preconnect、Open Graph、Twitter ID 和 JSON-LD `sameAs`。
- 补齐 Swiper、KaTeX、MathJax、Mermaid、Copycode 配置及脚本/资源注入开关。
- 补齐加载/错误文案、错误页插图和图片失败占位图。

### 前端

- 设置页增加文章、评论、笔记本、SEO、搜索和插件字段的结构化表单，并保留高级 JSON 兜底。
- 保存配置后沿用现有 `SaveSection` + `RebuildSite` 热更新链路。

## UI 验收边界

- **参照图路径**：`baselines/settings-reference.png`（继承 Spec 57 的桌面设置页布局，新增字段沿用同一 Console 视觉边界）
- **验收清单**：
  - [x] 站点分组、子导航和设置卡片沿用现有桌面 Console 布局。
  - [x] 文章、评论、笔记本、SEO、搜索和插件字段均有可见的结构化编辑控件。
  - [x] 开关使用二元控件，枚举使用选择控件，列表/映射支持编辑和删除。
  - [x] 保存按钮沿用现有 loading/Toast 状态，不改变既有设置页交互。

## ADDED Requirements

### Requirement: Stellar 配置覆盖

The system SHALL expose the applicable Stellar settings through the Console, persist them by section, hot-reload them, and consume them in server-rendered pages or client resources.

#### Scenario: 文章默认横幅

- **WHEN** `article.auto_banner` is enabled and an article has no explicit banner
- **THEN** the article page uses `default.banner` without contacting an external image API

#### Scenario: 本地搜索内容开关

- **WHEN** `search.local_search.content` is false
- **THEN** the local index keeps searchable titles and omits post body text

#### Scenario: SEO 与插件开关

- **WHEN** Open Graph or a supported plugin is disabled
- **THEN** its corresponding tags or scripts are absent while existing defaults remain compatible when the setting is missing

#### Scenario: JSON-LD 社交链接

- **WHEN** `structured_data.links` is empty or contains social URLs
- **THEN** the emitted BlogPosting JSON-LD is valid JSON and includes `sameAs` only when links are configured

## 验证

1. `go test ./backend/...`、`npm test`、`npm run build` 通过
2. `node --check` 覆盖新增/修改的前端资源
3. 设置页 UI 清单逐项对照 `baselines/settings-reference.png`
4. `make hooks-install` 后执行 `make test`
5. `make merge-check` 仅在用户明确授权后执行

## Out of Scope

- Hexo-only `stellar`、`dependencies`、`data_services`、`data_cache` 的完整复刻。
- 引入外部评论服务、Algolia 或 Unsplash API。
- 合并到 `dev` 或向远端推送。
