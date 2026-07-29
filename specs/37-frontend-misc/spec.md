# Spec 37 · 前端杂项对齐（首页 hero / nav_tabs / i18n 决策）

- **分支**：`feature/frontend-misc` · **Wave 8** · **隔离等级**：中
- **主模块**：`internal/template/partial/index.html`（hero 区）+ `partial/navbar.html`（tabs 可配）+ `config/site.go`
- **前置**：无（与 34 有 site.go 共享，均为 append 式段落，34 先合则本分支 rebase）

## Why

差距清单收尾的三个独立小项：(1) **首页封面 hero 区**——原版 `_partial/cover/` 三件套（大图/标题/引导按钮）本项目整体缺失；(2) **nav_tabs 自定义外链 tab**——原版导航条支持配置任意 tab（如「朋友文章」外链），本项目 navbar tab 固定；(3) **i18n 语言包**——本项目文案硬编码中文，本 spec 做**决策级处理**（不做完整 i18n，仅将文案收敛到语言表，见技术决策）。

## 功能需求

### 1. 首页 hero（对照 `layout/_partial/cover/` 三文件）
```yaml
cover:                 # site.yml 新增；不配置 = 无 hero（现状）
  title: xxx
  description: xxx
  scheme: dock         # 对照原版 cover scheme 枚举
  features:            # 引导按钮组
    - {title: 开始阅读, url: /archives/, icon: xxx}
```
- 仅首页第一页渲染（/page/2 起不出现），结构 class 对照原版 EJS。

### 2. nav_tabs 可配
```yaml
nav_tabs:              # 追加在固定 tab 之后
  - {title: 朋友文章, url: https://xxx, external: true}
```
- external 项新窗口打开 + external 图标（icons.yml 已有 key）。

### 3. 文案收敛（i18n 的最小正确形态）
- 新 `internal/template/lang.go`：`Lang(key)` 模板函数 + 内置 zh-CN 表（键名对照原版 `languages/zh-CN.yml`，值用原版原文）。
- 模板内硬编码中文文案逐处替换为 `{{ lang "key" }}`（一次机械替换，不改变输出）。
- **不做多语言切换**：仅收敛文案层，未来要支持 en 时只需加表。

## What Changes

- `partial/index.html`：+hero 段（条件渲染）；`assets/css/main.css` grep cover 相关类（原版 styl 有 `_layout/cover.styl`，确认编译产物已含则直接用）。
- `partial/navbar.html`：tabs 循环追加配置项。
- `config/site.go` + `site.yml`：+cover、+nav_tabs 段。
- `internal/template/funcs.go`：+lang 函数注册；+`lang.go`（表数据 + 单测：所有 key 有值）。

## 实现步骤

1. hero：精读 cover 三 EJS + cover.styl → 模板段 + 配置 + 目检（与原版 demo 站首页比对）。
2. nav_tabs：配置段 + navbar 循环（小时级）。
3. 文案收敛：grep 全部模板中的中文字面量 → 建表 → 机械替换 → 输出 diff 目检为零变化。

## 技术决策（无需再问）

- **完整 i18n 不做**：单人中文博客场景下多语言切换收益低；本 spec 只做文案表收敛，为未来留口。这是差距清单「i18n 缺失」项的正式处置。
- **hero 缺省关闭**：不配置 cover 段时首页与现状一致。
- **404 插图/分页箭头可配**：已划入 Spec 34 style 段，本 spec 不重复。

## Impact

- **新增**：`internal/template/lang.go`（+test）、cover/nav_tabs 配置段
- **修改**：index.html、navbar.html、funcs.go、site.go/site.yml
- **不影响**：文章渲染、store、api、console

## 验证

- [ ] 配置 cover 后首页 hero 与原版 demo 结构一致；/page/2 无 hero；不配置无变化
- [ ] nav_tabs 外链 tab 出现且新窗口打开
- [ ] 文案替换后全站渲染输出与替换前逐字节一致（curl diff 抽查 5 个页面）
- [ ] lang 表单测：无空值、无未使用 key
- [ ] `make test` 全绿
