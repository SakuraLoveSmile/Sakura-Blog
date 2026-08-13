# Spec 63 · 组件库标签真实预览

- **分支**：`feature/tag-preview` · **Wave 63**（Spec 64 暗色收尾在 `feature/preview-dark`）
- **主模块**：`frontend/console/src/modules/plugins/`、`backend/internal/api/preview*`、`backend/internal/post/preview_fixture.go`
- **前置**：Spec 23（插件库）、Spec 58（侧栏组件库）、Spec 62（前端测试基建）

## Why

`PluginLibrary.vue` 旧预览只向 iframe 注入 CSS 并固定暗色主题；前台标签实际还依赖
`main.js`、数据 service、Swiper/vendor、Mermaid/KaTeX 与接口数据，导致：
- Tabs/Folding 无初始化脚本，只能显示静态结构；
- Rating/Vote/Friends/Sites 等数据标签拿不到数据，只剩空容器；
- GitHub 卡片走真实接口，预览受公网影响且不可控；
- 亮暗主题固定为暗色，无法预览亮色下的标签样式。

本次修复统一预览运行时，采用**本地 Fixture 策略**：预览不依赖公网、不写入真实数据，
仍能展示标签的结构、样式、交互与典型数据状态（normal / empty / error）。

## What Changes

### 后端

- `backend/internal/api/preview.go`：`POST /api/v1/preview/markdown` 新增
  `preview_profile=component-library` 字段；命中时走 `post.Service.RenderComponentPreview`
  Fixture 渲染管线，缺省行为完全不变（`TestPreviewMarkdownNormalUnchanged` 覆盖）。
- `backend/internal/post/preview_fixture.go`：
  - `fixtureGHClient`：ghrepo/ghuser/ghcard 的确定性 GitHub Fixture（不发公网请求）；
  - `{% links %}` 友链分组 Fixture 直出（内联 SVG data URI 头像，离线可用）；
  - `rewritePreviewDataAPIs`：把渲染产物中 `ds-rating`/`ds-vote` 的 data-api 改写为
    `/api/v1/preview/*` 并把 data-id 加 `preview-` 前缀（隔离 ID，永不与真实内容冲突）；
    `ds-friends`/`ds-sites`/`ds-friends_and_posts` 外部 URL 一律本地化；
    `ds-mdrender` 的 src 编码进查询参数指向本地 Fixture 端点。
- `backend/internal/api/preview_fixture.go` + `router.go`：新增 `/api/v1/preview/`
  rating / vote / friends / sites / md 五个 Fixture 端点，支持 `?state=normal|empty|error`
  三态；rating/vote 的提交端点只回 OK 不落库。全部挂 `authed` 组（与预览接口一致）。

### 前端

- `frontend/console/src/modules/plugins/plugins-data.ts`：新增预览能力清单
  `PLUGIN_PREVIEW`（capability / mainJs / services / fixture / height / fallback），
  未登记插件由 `previewInfoFor` 兜底为 static（只注入 CSS + HTML）。
- `frontend/console/src/modules/plugins/preview/`（共享预览运行时，标签插件库与
  侧栏组件库复用，纯 TS 可单测）：
  - `resources.ts`：service/vendor 资源映射（rating/vote/friends/sites/mdrender/swiper/
    mermaid→math-mermaid-loader）、marked CDN、main.js、样式清单、能力/ Fixture 文案；
  - `preview-document.ts`：iframe 文档构建（样式注入 + 主题属性 + 前台运行时等价最小
    shim + 按能力脚本 + 降级提示检测 + 高度回传）；
  - `render-gate.ts`：防抖 + 请求序列号（旧请求结果不覆盖新预览）；
  - `usePluginPreview.ts`：Vue 组合式函数（防抖渲染/Fixture 模式/主题切换/高度回传）。
- `PluginLibrary.vue`：预览区增加亮/暗切换、依赖状态、Fixture 标识、
  加载失败/参数缺失可读提示、重新渲染按钮；预览管线全部收敛到共享运行时。
- `WidgetLibraryView.vue`：侧栏组件预览改为复用 `buildPreviewDoc`（同一套 shim /
  降级 / 高度回传），不再自建 iframe 文档。

### 修复：iframe 高度回传正反馈环

原高度脚本用 `document.documentElement.scrollHeight` 回传，而预览 iframe 的视口高度
等于父级设置的 iframe 高度（`main.css` 的 `html{height:100%}` 使
documentElement.scrollHeight 随视口反馈增长）——每次回传都被父级放大，形成
420→444→468…→900 上限的正反馈环，所有预览最终被撑到最高。

改为 `document.body.scrollHeight`（内容高度）优先，配合父级 `Math.max(MIN_HEIGHT,…)`
钳制，反馈环消除：小内容稳定在 420px 下限，异步加载完成后不被裁剪。

## UI 验收边界

> 载体约定见根目录 [`AGENTS.md`](../../AGENTS.md)「UI 类任务开工约定」。

- **参照图路径**：`baselines/plugin-library-baseline.png`（唯一版式参照，来自 Spec 58；
  预览内容基线为本目录 `baselines/01-note-dark.png` 等 17 张暗色基线（Spec 64 起
  暗色单模式，亮色基线已删除），由同一 srcdoc 独立页截图，与 iframe 内所见一致）
- **验收清单**（完成时逐项勾选）：
  - [x] 静态标签（note）：渲染样式正确，暗色背景（`01-note-dark.png`）
  - [x] Folding 为原生 details 结构可展开（`02-folding.png`；DOM 验证
    `before=false after=true`）
  - [x] Tabs 可切换标签页（`03-tabs.png`；DOM 验证点击后
    `active=标签二 pane=第二页内容。`）
  - [x] Swiper 初始化成功（`04-swiper.png`；`swiper-initialized` + 2 slides）
  - [x] Rating 展示 Fixture 数据（`05-rating.png`：`(4.6) 128人已评分`），点击评分
    不写库（`preview-` 隔离 ID + 后端仅回 OK）
  - [x] Vote 展示 Fixture 计数（`06-vote.png`：86 / 12）
  - [x] Friends 展示 4 条本地 Fixture 友链卡（`07-friends.png`）
  - [x] Sites 展示 3 条本地 Fixture 站点卡（`08-sites.png`）
  - [x] GitHub 卡片为服务端确定性 Fixture（`09-ghrepo.png`：`vuejs/vue 预览 Fixture 仓库`）
  - [x] Mermaid 渲染为 SVG 图（`10-mermaid.png`；失败路径由降级提示脚本在 5s 后
    注入 hint 且布局不破坏）
  - [x] 外部 Markdown 经 marked 渲染本地 Fixture（`11-md.png`：`# 外部 Markdown Fixture`）
  - [x] Gallery 展示 3 张本地真实界面图与明确 caption（`12-gallery.png`）
  - [x] Grid 展示 bg:card 双列信息卡，标题/正文/圆角一眼可辨（`13-grid.png`）
  - [x] Chat 展示本地头像 + 左右消息 + 时间提示 + 暗色气泡（`14-chat.png`）
  - [x] Albums 展示 4 张方形专辑封面（`15-albums.png`）
  - [x] Posters 展示 4 张竖版海报（`16-posters.png`）
  - [x] TOC 展示确定性两级目录 Fixture（`17-toc.png`）
  - [x] Spec 64：预览固定暗色单模式（无亮/暗切换按钮；`12-note-light.png` 已删除）
  - [x] Spec 64：默认示例离线完整展示——拦截非 localhost 请求后 54 标签巡检全过，
    0 破图/0 空容器/0 控制台错误；mermaid/marked 为本地 vendor，预览不加载 CDN
  - [x] 异步内容加载完成后不被 iframe 高度裁剪（高度回传用内容高度，无反馈环；
    推荐高度优先，小插件不再被 420px 画布包围）
  - [x] 前台真实文章渲染不受影响（`/post/tag-plugins-test` 仍输出
    `data-api="/api/rating"`、`data-id="default"`，无 preview- 前缀）

## ADDED Requirements

### Requirement: 组件库预览 Fixture 模式

系统 SHALL 在 `POST /api/v1/preview/markdown` 携带 `preview_profile=component-library`
时，使用确定性本地 Fixture 渲染标签插件：GitHub/友链不发公网请求，rating/vote
使用 `preview-` 隔离 ID 且不写入真实统计，数据标签的 data-api/src 改写为
`/api/v1/preview/*` 本地端点；缺省（无 profile）行为与线上渲染完全一致。

#### Scenario: Fixture 模式改写数据端点
- **WHEN** 请求 `preview_profile=component-library` 且正文含 `{% rating %}` / `{% friends %}` / `{% ghrepo %}`
- **THEN** 渲染 HTML 中 rating data-api 为 `/api/v1/preview/rating`、data-id 带
  `preview-` 前缀，friends data-api 为本地端点，ghrepo 卡片内容为确定性 Fixture 数据

#### Scenario: 缺省渲染不受影响
- **WHEN** 请求不带 `preview_profile`
- **THEN** 渲染 HTML 与线上一致（`data-api="/api/rating"`、无 ID 前缀、外部友链 URL 原样保留）

### Requirement: 共享预览运行时

系统 SHALL 提供标签插件库与侧栏组件库共用的预览 iframe 文档构建与脚本注入逻辑：
按插件能力清单按需注入前台 CSS/service/vendor、固定暗色主题（Spec 64）、
防抖 + 请求序列号防止旧请求覆盖新预览、推荐高度 + 内容高度回传避免异步内容被裁剪、
对资源不可用显示可读降级提示。

#### Scenario: 快速连续编辑示例
- **WHEN** 用户连续修改示例内容触发多次渲染
- **THEN** 只执行最后一次渲染，先返回的旧请求结果被序列号丢弃，不覆盖新预览

#### Scenario: 异步内容加载完成
- **WHEN** Mermaid/外部 Markdown/数据服务异步加载完成后
- **THEN** iframe 高度按内容高度回传（无正反馈环），内容不被裁剪；
  加载失败时保留源码并显示降级提示，布局不破坏

### Requirement: 暗色单模式与依赖本地化（Spec 64）

系统 SHALL 将插件库预览收敛为暗色单模式：预览文档固定 `data-theme="dark"`，
无主题切换接口；默认示例与预览运行时的资源全部本地化（/assets/img/preview/、
/assets/media/preview/、marked/mermaid 本地 vendor），预览运行时不得加载 CDN；
用户手动输入的真实第三方 URL 仍按真实 URL 预览。

#### Scenario: 离线预览
- **WHEN** 拦截所有非 localhost 网络请求后逐项巡检 54 个标签
- **THEN** 所有默认示例仍完整展示（结构/样式/交互/数据 Fixture），
  无破图、无空容器、无横向溢出、无控制台错误

#### Scenario: 推荐高度
- **WHEN** 选中标签或 iframe 内容高度变化
- **THEN** 先应用插件推荐高度（PluginPreviewInfo.height），再按内容高度回传在
  [60, 900] 内调整；小型行内组件不被固定 420px 画布包围

### Requirement: 预览不持久化用户操作

系统 SHALL 保证组件库预览中的评分/投票操作只作用于 Fixture，
不写入真实数据库统计。

#### Scenario: 预览中评分
- **WHEN** 用户在组件库预览中对 rating/vote 组件操作
- **THEN** 请求到达 `/api/v1/preview/rating`（POST）/`/api/v1/preview/vote/update`，
  服务端仅返回成功，不落库、不改动真实统计

## 验证

1. `make test` 全绿（go vet/go test + console 测试 65 用例 + assets/js 测试 37 用例）
2. `frontend/console` `npm run build` 通过（dist 真实产物）
3. 浏览器验收：54 标签在线/离线双跑巡检全过，交互（tabs/folding/swiper/copy/rating/vote）
   通过，884×795 / 1159×795 / 移动端宽度三视口无溢出无重叠；
   暗色基线 17 张存于 `baselines/`
4. 真实文章页 `/post/tag-plugins-test` 端点未改写（curl 抽查）
5. `git diff --check` 无空白错误；`make merge-check` 全绿后合入 dev

## Out of Scope

- 不新增标签类型；不改已有文章 shortcode 语法
- 不新增持久化数据表/迁移；Fixture 只服务预览
- 预览默认使用 `component-library` Fixture 模式，不调用公网接口作为验收前置条件
- KaTeX 失败降级沿用 math-mermaid-loader 的静默降级（源码保留），
  本次未 vendor KaTeX（不在默认示例中），未新增独立 KaTeX 降级文案
- math-mermaid-loader 的 mermaid 改为本地 vendor 优先 + CDN 兜底，
  前台真实文章的图表渲染行为不变（仅资源来源本地化，对齐 Spec 53 swiper 先例）
