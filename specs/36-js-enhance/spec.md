# Spec 36 · 前端体验增强（preload / scrollreveal / heti + 插件参数可配）

- **分支**：`feature/js-enhance` · **Wave 8** · **隔离等级**：高（纯前端资产 + head 注入）
- **主模块**：`assets/js/plugins/`、`assets/vendor/`、`partial/head.html`、`config/site.go`（plugins 段）
- **前置**：无

## Why

原版 plugins 段的四个体验增强本项目缺失：**preload**（flying_pages 链接预加载，悬停即预取显著提升点击体感）、**scrollreveal**（滚动入场动画）、**heti**（中文排版增强：标点挤压/中西文间距）。另有两处已实现但参数硬编码：**fancybox selector** 与 **pjax selectors/timeout/cacheBust**。tianli_gpt AI 摘要明确不做（见技术决策）。

## 功能需求（对照原版 `_config.yml` plugins 段）

| 项 | 配置键（site.yml plugins 段） | 行为 |
|---|---|---|
| preload | `plugins.preload.{enable,service}` | flying_pages 模式：视口内/悬停链接预取 HTML；与 PJAX 共存（预取结果进 PJAX 缓存路径） |
| scrollreveal | `plugins.scrollreveal.{enable,distance,duration,interval}` | 首页卡片/文章块入场动画，`prefers-reduced-motion` 时自动禁用 |
| heti | `plugins.heti.enable` | 文章正文加 heti class + 初始化脚本（vendored，不走 CDN） |
| fancybox | `plugins.fancybox.selector` | 现有 base.html 内固定 selector 改为读配置（缺省保持现值） |
| pjax | `plugins.pjax.{enable,selectors,timeout,cacheBust}` | 现有 pjax.js 初始化参数改为配置注入（缺省保持现值） |

## What Changes

- `assets/vendor/`：+flying_pages、scrollreveal、heti 三库 vendored（版本对照原版 _config.yml 声明的 CDN 版本锁定）。
- `assets/js/plugins/`：+`preload.js`、`scrollreveal-init.js`、`heti-init.js`（初始化与配置消费；PJAX 重载钩子对齐现有 plugins 的 pjax:complete 模式——**改前必读 CLAUDE.md「Step 3 新增坑点」PJAX DOM 引用章节**）。
- `partial/head.html` / `layout/base.html`：按 enable 注入；fancybox selector 与 pjax 参数改为模板输出配置值。
- `config/site.go` + `site.yml`：+plugins 段（缺省全关，两个既有项缺省 = 现状值）。

## 实现步骤

1. 读原版 plugins 相关注入逻辑（`layout/_partial/plugins/` 目录）确认注入时机与参数形状。
2. vendored 三库 + 完整性校验（记录版本与来源 URL 于文件头注释）。
3. preload：与 PJAX 协同（预取不触发埋点中间件计数——加 `X-Preload` 头，middleware.go 埋点段跳过，+1 行）。
4. scrollreveal：目标选择器只挂首页/列表页卡片，文章正文不做（阅读干扰）。
5. heti：仅文章页正文容器启用。
6. fancybox/pjax 参数化（回归重点：现有 PJAX 导航测试 `scripts/test/pjax-navigation.test.mjs` 必须全绿）。

## 技术决策（无需再问）

- **tianli_gpt AI 摘要不做**：依赖外部付费 API，与自托管定位不符；后续如做走本地摘要另立 spec。
- **三库全部 vendored**：不引 CDN（与 swiper/KaTeX 本地化决策一致）。
- **缺省全关**：preload/scrollreveal/heti 均为 opt-in，避免默认行为变化。
- **埋点防污染**：preload 预取请求不得计入 pageview 统计（验证项）。

## Impact

- **新增**：3 个 vendor 库、3 个 plugins js、plugins 配置段
- **修改**：head.html、base.html、site.go/site.yml、middleware.go（+1 行埋点跳过）
- **不影响**：模板结构、store、console

## 验证

- [ ] preload 开启后悬停链接出现预取请求且 pageview 不重复计数
- [ ] scrollreveal 首页卡片入场动画生效，`prefers-reduced-motion` 下无动画
- [ ] heti 开启后文章标点挤压可见（长文目检）
- [ ] fancybox selector / pjax 参数改配置生效，pjax-navigation.test.mjs 全绿
- [ ] 全部关闭时行为与现状一致
- [ ] `make test` 全绿
