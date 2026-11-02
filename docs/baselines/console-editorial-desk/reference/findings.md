# Console 整体视觉「AI 味」调研与改版方向

- 状态：只读调研完成；未修改生产代码，未提交、未合并、未推送。
- 日期：2026-10-31（Asia/Taipei）。
- 代码：`dev b114e1d4` + 用户既有未提交改动（未触碰）。Console 构建于 13:14，入口 `dist/assets/index-C1Fo-hM1.js`；服务实际返回的脚本与 `modules/console/web/dist` 同名同字节。
- 实测环境：隔离服务 `127.0.0.1:8135`，根目录 `.tmp/e2e-server.*`（独立 DB/config/content/upload），账号 `admin / e2e-password`；仅写入 9 篇夹具文章，未连接生产库。服务已停止。
- 证据目录：`docs/baselines/console-style-refresh/`。
- 结论分层：**【事实】**＝源码/命令/截图可直接核对；**【判断】**＝基于事实的设计评估；**【未知】**＝本轮没有验证。

---

## 0. 结论先行

**AI 味主要来源 Top 5（按视觉影响排序）**

1. **仪表盘是一套通用分析模板**：四张同构 KPI 卡（图标＋大数字＋“暂无对比数据”）、环形图、折线图、2×2 快捷宫格、最近内容/评论列表；趋势卡内又嵌三张 1px 描边小卡。首屏就把“AI 生成后台”的骨架完整暴露。
2. **全站只有一种“卡片皮肤”**：内容列表、评论、设置分区、统计、图表、快捷入口都装进同一种 `#1c1c1e` + 1px 描边 + 10px 圆角的盒子；列表外面再套一层卡片，卡片里再套卡片。所有区块等权，没有主次，只靠边框切分。
3. **蓝色强调泛滥、状态用彩虹胶囊**：页面标题图标、主按钮、Tab 下划线、设置分区蓝色竖条、图表、状态点、徽标都在抢蓝色；已发布/草稿/待审核用绿/橙/红底色胶囊，评论、评论用户、瞬间、文库还散落裸 hex。颜色不是“少量、语义化”，而是装饰。
4. **说明书式文案**：每页标题下一句“功能说明”；设置页每个分区的顶部固定一大块“调整后会发生什么 / 影响位置 / 如何确认 / 保存后…”，与下方分区标题重复；快捷操作 8 条每条都有营销式 `desc`；空态统一“居中图标 + 标题 + 一行灰字”。信息被反复解释，读起来像生成的文档而不是工具。
5. **尺度漂移的“生成感”**：23 种字号（9 / 9.5 / 10 / 10.5 / 11 / 11.5 / 12 / 12.5 / 13 / 13.5 / 14 / 14.5 / 15 / 15.5 / 16 / 16.5 / 17 / 18 / 19 / 20 / 22 / 24 / 28px）、18 种 `border-radius`、122 处 `rounded-full`、35 个裸 hex；标题、副标题、标签层层缩到 11–12px，再叠 `letter-spacing: -0.015em / 0.06em` 这类半像素微调。这正是 LLM 写 CSS 最常见的“每一处都差一点”的痕迹。

**推荐方向 A「编辑台（Editorial Desk）」**：用排版、留白和分隔线取代卡片墙；蓝只留给主操作与焦点；删掉标题图标、彩色胶囊、说明块与装饰性指标。它改变的是信息层级和页面结构，而不是换一套同样模板化的皮肤。**最强备选 C「单色终端」** 更彻底，但牺牲日常舒适度；**B「Apple 分组工作台」** 最贴近本地设计库，却仍是同一套盒子骨架，预计只能把“AI 味”从 10 降到 7。

---

## 1. 证据与范围

### 1.1 实测截图（真实运行界面，未注入）

视口 1280×900 与 390×844，均为独立构建产物、独立 DB。

| 页面 | 1280 | 390 |
|---|---|---|
| 仪表盘 | `before-dashboard-1280.png` | `before-dashboard-390.png` |
| 文章列表 | `before-posts-1280.png` | `before-posts-390.png` |
| 系统设置 | `before-settings-1280.png` | `before-settings-390.png` |
| 评论 | `before-comments-1280.png` | `before-comments-390.png` |
| 主题设置 | `before-appearance-1280.png` | `before-appearance-390.png` |

- 文章列表用 9 篇真实夹具（6 已发布 / 3 草稿 / 2 置顶，含分类与标签），因此行高、状态、标签、操作按钮都是真实内容密度。
- **限制【未知】**：隔离夹具默认关闭评论，`/api/comments` 未挂载（POST 返回 Stellar 404 页面），因此评论页只有空态；评论列表的“每行卡片化”结论来自源码（`CommentList.vue:486`），不是本轮实测行。
- 主题设置页当前是“已停止保存”的重定向说明页（`AppearanceView.vue:16-17`），不是旧版调色界面。

### 1.2 方向注入预览（仅 CSS 覆盖，不改源码）

- `preview-A-dashboard-1280.png` / `preview-A-posts-1280.png` / `preview-A-settings-1280.png`
- `preview-C-dashboard-1280.png` / `preview-C-posts-1280.png` / `preview-C-settings-1280.png`
- 注入样式原样保存为 `preview-direction-a.css` / `preview-direction-c.css`，便于复核。
- 注入方式：Playwright 在页面加载前注入 `<style>`，只覆盖 CSS 变量与选择器，未改任何 Vue/TS/CSS 源文件。
- **限制【事实】**：CSS 注入无法改变 GridStack 的列布局，也无法让环形图真正读取新 token（原因见 §2.3）；预览用于“方向裁决”，不是最终实现像素稿。

### 1.3 390px 实测数据

- 五个页面 `documentElement.scrollWidth == clientWidth == 390`，无整页横向溢出。
- 设置页 Tab 栏 `scrollWidth 1091 / clientWidth 332`（13 个 Tab，需横向滑动 3.3 屏），无可见的滚动提示（`settings-form.css:380-395` + `VTabbar.vue:189-202`）。
- 文章页顶栏高 185px，评论页 169px；内容要等标题、搜索、按钮三行之后才开始。
- 仪表盘 4 张 KPI 卡在 390 下纵向堆叠，首屏只剩统计与一张空趋势图（`dashboard.css:1188-1215`、`dashboard.css:566-576`）。

---

## 2. 当前视觉事实（逐页）

### 2.1 外壳 / 侧栏 / 顶栏
- 侧栏 224px 深灰实色（`DefaultLayout.vue:308-312`），分组标题 12px 灰、导航项 36px、每项都有图标，底部搜索框 + “开始写作”整宽按钮 + 用户资料区。结构本身可用，但“图标 + 分组标题 + 折叠 + 资料区”是标准后台模板（`DefaultLayout.vue` 全文件）。
- 页面标题统一 20px/600 + 前置图标 + 一行 12–13px 功能说明，吸顶并带 1px 下边框（`VPageHeader.vue:78-91`）。图标在文章、评论、设置、主题设置页都染成主色（`PostList.vue:337-340`、`CommentList.vue:314-317`、`SettingsView.vue:371-377`、`AppearanceView.vue:16`）。

### 2.2 仪表盘
- 4 张 KPI 卡：图标 + 标签 + 24px 数字 + “全部内容/公开内容/…/暂无对比数据”（`StatCardWidget.vue:63-79`；`dashboard.css:526-576`）。无数据时显示“— 0.0%”或“暂无对比数据”。
- 访问趋势卡内再嵌 3–4 张带 1px 边框、3% 白色底的指标小卡（`TrendChartWidget.vue:299-314`，以及同文件 scoped 的 `.trend-stat-card:498-503`）。
- 分类环形图：8 色固定调色板，图例每一行又是一张 `rounded-xl + bg-white/[0.03] + border` 的小卡和进度条（`CategoryDonutWidget.vue:34-52`、`281-304`）。
- 快捷操作：2×2 宫格，每格一个图标 + 标签 + 营销短句（`QuickActionsWidget.vue:35-88`、`161-186`）。
- 卡内标题 16px/650 + `letter-spacing:-0.015em`，右侧 badge 12px/500 + `letter-spacing:0.06em`（`dashboard.css:504-524`）。

### 2.3 文章列表
- 默认行列表已经比海报网格克制：`.console-summary` 一行四项事实摘要、`.console-records` 单层列表、状态用文字（`apple-base.css:39-123`）。
- 但列表仍包在 1px 描边 + 10px 圆角 + `#1c1c1e` 的容器里；每行 72px，标题 14px/500、slug 用等宽字体、分类/标签是灰字按钮、状态是彩色胶囊、右侧 5 个图标按钮（`apple-base.css:512-622`、`PostList.vue:755-820`）。
- 海报卡片视图仍保留完整“AI 卡片”语汇：玻璃浮层、圆形勾选、白/黑半透明胶囊、阴影、`bg-white/[0.06]` 标签、hover 变蓝（`PostList.vue:536-575`、`609-618`）。用户只要曾切过一次网格视图，`localStorage` 会让它持续作为默认视图（`PostList.vue:60-62`）。
- 状态色仍是彩色底胶囊：`bg-state-success/10 text-state-success` 等（`PostList.vue:320-330`）。

### 2.4 设置
- 单张 VCard 内 13 个 Tab；默认下划线样式，但每个 Tab 都带图标（`SettingsView.vue:388-400`；`VTabbar.vue:173-221`）。
- 每个分区顶部先渲染 `SettingsEffectGuide`：11px 眉标“调整后会发生什么”、17px 标题、结果句、折叠的“影响位置/如何确认”、底部“保存后……”（`SettingsEffectGuide.vue:31-70`；文案表 `settings-category-guide.ts:9+`）。紧接着分区标题又重复一次（如“站点身份”出现两遍：`BasicSettings.vue:24-29`）。
- 分区标题前有 3.5×16px 蓝色圆角竖条（`settings-form.css:406-423`、`BasicSettings.vue:220-226`）。
- 输入框 36px、8px 圆角、深一档底色；表单密度尚可，但每个控件都有完整边框，页面被切成大量小盒子（`apple-base.css:204-264`）。
- 390 下 Tab 栏被裁切，需要横向滚动；效果说明块占掉约 150px 首屏。

### 2.5 评论
- 顶部摘要四块（`CommentList.vue:367-437`），其中三块数字用裸 hex：`#FF9500` / `#30D158` / `#FF453A`（396/414/432）。
- 状态胶囊同样裸 hex + 10% 底色 + 20% 描边（`CommentList.vue:124-131`）。
- 顶部同时有“快捷键”按钮与一整条可展开的快捷键横幅：`rounded-2xl bg-primary/5 border-primary/20 animate-fade-in`（`CommentList.vue:335-357`），纯装饰性蓝底。
- 每条评论本身就是一张 `rounded-2xl border bg-card p-4` 的卡（`CommentList.vue:486`）。
- 空态：居中 64px 线性图标 + 两行灰字（`VEmpty.vue:12-38`）。

### 2.6 主题设置（外观）
- `/console/appearance` 只剩一张卡：“此处的旧分段已停止保存，请在系统设置的对应分类中修改配置”，一个“打开设置”按钮（`AppearanceView.vue:16-17`）。
- 侧栏同时保留“主题设置”和“设置 → 外观”两个入口（`internal/module/core/core.go:38`），前者是死胡同。这不是“AI 味”，但属于同一次视觉收口应清理的结构噪音。

---

## 3. AI 味来源详解（Top 5）

### 3.1 仪表盘结构：通用分析模板【事实 + 判断】

**事实**：KPI 卡、卡中卡指标、环图图例小卡、快捷宫格、列表卡全部存在；无数据时仍渲染“暂无对比数据”和 “— 0.0%”；每张 KPI 卡都有图标。

**证据**：`StatCardWidget.vue:63-79`、`TrendChartWidget.vue:299-314`、`dashboard.css:498-503`、`CategoryDonutWidget.vue:281-304`、`QuickActionsWidget.vue:35-88`。

**为什么像 AI**：这是“给任何后台都能套”的仪表盘配方——四个同构数字块、一个环图、一个折线、一个宫格。它不回答“今天该做什么”，只证明“系统有数据”。视觉效果上，同一种卡片重复 10 次以上，用户感知不到主次。

**附加缺陷【事实】**：分类环图颜色永远不跟随主题。`themeColors()` 用 `--color-${name}` 读取，`CategoryDonutWidget.vue:34-52` 传入的是 `chart2 / chart5 / primaryHover / error / success / warning / chart4 / primaryActive`，而 CSS 定义的是 `--color-chart-2 / --color-chart-5 / --color-primary-hover / --color-state-*`（`main.css:52-57`、`32-42`），全部命中 fallback 硬编码色；`registry.ts:15-22` 同样失配。因此即使换 token，环图仍是蓝/紫/绿/橙。**这也是“紫色/青色装饰”难以通过主题消化的根因。**

### 3.2 全站同一种卡片皮肤【事实】

**事实**：
- `VCard`：`background: var(--color-card)` + `1px border` + `10px radius` + `overflow:hidden`（`VCard.vue:29-50`）。
- `.apple-card`：同样实色卡 + 10px 圆角 + 1px 边框（`dashboard.css:489-499`）。
- `.console-records`：列表容器又是 `1px border + card 背景 + 10px 圆角`（`apple-base.css:108-113`）。
- 评论行、趋势内指标、环图图例、设置区块、快捷入口各自再造一层“小卡”（`CommentList.vue:486`、`TrendChartWidget.vue:498-503`、`CategoryDonutWidget.vue:281`、`QuickActionsWidget.vue:203-207`）。

**为什么像 AI**：所有内容被同一套盒子包裹，页面节奏只能靠“卡片间距”制造，结果是等高、等宽、等权重的方块阵列。本地设计库明确说“Whitespace carries hierarchy more than separators or color blocks”“Cards are spacing-led containers… not decoration”（`resource/apple-design/README.md`、`components/index.json`），当前实现正好相反：边框与底色承担了全部层级。

### 3.3 蓝色泛滥 + 状态彩虹【事实】

**事实**：
- 蓝出现在：侧栏选中底色（中性，尚可）、主按钮、页面标题图标、Tab 激活下划线、设置分区竖条、统计数字、图表、状态胶囊、快捷键横幅、链接 hover。单屏可数到 5–8 处蓝色。
- 裸 hex 引用（含同一类名重复）：评论页 21、评论用户 10、瞬间 6、文库项目 2（`CommentList.vue:124-131/396/414/432/555/566/577`、`CommentUsersView.vue:223/232/321/336/346`、`MomentList.vue:454/463/691`、`WikiProjectsView.vue:319/328`）。
- 状态一律“彩色底 + 彩色字 + 彩色边”的胶囊（10% 背景 / 20% 边框），而不是“点 + 文字”。
- 全站 35 种裸 hex，其中 `#30d158` 50 次、`#ff453a` 38 次、`#ff9500` 27 次、`#2e8dff` 31 次。

**为什么像 AI**：AI 生成的 SaaS 界面习惯用“状态色胶囊 + 品牌蓝到处点睛”制造丰富感。真正克制的工具（Linear、GitHub）状态只用一个小圆点或中性文字，蓝色只出现在一个主动作。

### 3.4 说明书式文案【事实 + 判断】

**事实**：
- 页面副标题：文章“N 篇文章”（尚可）；评论“审核读者留言与互动评论，支持键盘极速审核与批量处置”；设置“站点信息、主题外观、页面布局与各项功能配置”；主题设置“Stellar 2 上游主题配置与独立 Blog 扩展”（`PostList.vue:339`、`CommentList.vue:318`、`SettingsView.vue:376`、`AppearanceView.vue:16`）。
- `SettingsEffectGuide` 每个分区固定 5 段说明（`settings-category-guide.ts` 13 个分区），且与分区标题重复。
- 快捷操作 8 条 `desc`：“发布新内容 / 互动审核 / 组织结构 / 系统首选项 / 管理已有文章 / 文件与图片 / 版本与升级 / 打开前台页面”（`QuickActionsWidget.vue:35-88`）。
- 空态统一图标 + 标题 + 灰句（`VEmpty.vue:12-38`）。

**为什么像 AI**：生成式界面倾向于“解释自己”，用整齐的名词短语填满每个空白；真实工具把说明放到帮助/悬浮提示里，让界面本身承担引导。设置页尤其严重：用户看到的第一屏不是控件，而是对控件的说明。

### 3.5 尺度漂移与微调痕迹【事实】

**量化**（`modules/console/web/src` + `packages/components/src/ui`，含编辑器边界文件）：

| 维度 | 现状 | 问题 |
|---|---|---|
| 字号 | 23 种，12px 出现 117 次、13px 71 次、11px 54 次、10px 24 次，另有 9/9.5/10.5/11.5/12.5/13.5/14.5/15.5/16.5px | 计划只定义了 20 / 13 / 14 三级；实际是“每处微调一点” |
| 圆角 | 18 种 px 值；Tailwind `rounded-full` 122、`rounded-lg` 98、`rounded-xl` 84、`rounded-2xl` 27 | 计划 8/10/12/14/16；实际 2/3/4/5/6/7/8/9/10/11/12/14/16/18/20/99/999/9999 |
| 颜色 | 35 种裸 hex | 语义 token 被绕过，主题化失效 |
| 阴影 | 卡片已去掉常驻阴影（好）；但海报卡、Tab segmented、分页仍用 `shadow-sm`/自定义阴影 | 与“常驻无阴影”部分冲突 |
| 字重 | 400/450/500/550/600/650/700 | 中间字重大量出现，层级靠 50 的步进 |

**为什么像 AI**：这是 LLM 逐文件生成样式时的典型结果——没有全局尺度表，每个组件“看起来差不多”就加一个 0.5px 或 1px。用户说不清哪里怪，但会觉得“不是人调的”。

---

## 4. 文档结论 vs 当前代码

`docs/plans/console-ui-tool-style.md` 的状态是 **待体验**（未验收），这与用户本轮“还是很重”的反馈一致。`docs/plans/console-ui-consistency.md` 的 Round 1–4 多项标记 **已验收**，但其中若干视觉契约已被后续 tool-style 方案取代，文档没有“已废弃”标记。

| 文档结论 | 当前代码 | 判断 |
|---|---|---|
| tool-style：深色专业工具、紧凑侧栏 224px、导航 36px、激活中性底 | 侧栏 224px、导航 36px、激活 `--color-sidebar-accent` | **已落地**（`DefaultLayout.vue:846-865`） |
| tool-style：普通工作按钮/搜索/分段 8px/10px，只有 pill/头像/开关用胶囊 | VButton 8px、搜索 8px、segmented 8px；但 122 处 `rounded-full`、84 处 `rounded-xl`、27 处 `rounded-2xl` | **部分落地**；装饰性胶囊与超阶梯圆角仍在 |
| tool-style：标题 20/600、说明 13、正文 14 | H1 20/600；说明 12–13；另有 23 种字号 | **精神落地，尺度失控** |
| tool-style：文章/页面初始视图为行列表 | 默认 `table`，但 `localStorage` 的 grid 偏好会覆盖；海报视图仍重度装饰 | **部分落地**（`PostList.vue:60-62`、`536-575`） |
| tool-style：内容页四块 KPI 改紧凑无图标事实摘要 | `.console-summary` 无图标、无卡；但用户/附件仍用带图标的 `ConsoleStatsCard` 组件 | **部分落地** |
| tool-style：仪表盘去问候、保留部件、卡片实色无悬浮 | 标题改为“仪表盘 + 日期”，无问候；但仍 4 KPI + 卡中卡 + 宫格 | **只做了表面，结构未动** |
| tool-style：设置改下划线分区导航、去多层胶囊 | 下划线 Tab 已用；13 个 Tab 都带图标；效果说明块仍在；移动端裁切 | **部分落地** |
| tool-style：删纯装饰性说明 | 效果说明、快捷描述、快捷键横幅仍在 | **未落地** |
| tool-style：各页空态保留事实与操作 | VEmpty 通用图标 + 文案，部分有操作 | **基本落地，但视觉同质** |
| consistency T2：按钮胶囊、黑字高对比 | 当前默认 8px，仅 `pill` 为胶囊 | **被 tool-style 取代，文档未标注** |
| consistency T3：列表概览卡 `.stats-card` 统一高度 80px；设置 segmented Tabs；SEO 38px 蓝底黑字 | `.stats-card` 已改为无卡摘要；设置已改下划线；SEO 导航已改 `rounded-[var(--radius-md)]` | **被 tool-style 取代，文档未标注** |
| consistency Round 4：Tabs 横向滚动、空列表选择入口、44px 触控 | `VTabbar autoReveal`、空态禁用、触控 CSS 均存在 | **已落地** |

**结论【判断】**：tool-style 计划把“去装饰”理解为“去掉渐变/毛玻璃/阴影”，但没有动**信息架构**（卡片墙、KPI 模板、说明文案、每页标题图标）。机械门禁 `modules/console/tests/ui-restraint.test.ts` 只拦 9 类“浮华效果”（渐变、模糊、transition-all、pulse、uppercase、hover-lift、发光、emoji、越界 hex），不拦“卡片套卡片、标题图标、彩色胶囊、尺度漂移”。所以当前代码可以通过全部门禁，却依然是一套模板。

---

## 5. 三个可执行方向

> 三个方向都**不新增 UI 库/依赖**；均复用 `main.css` 语义 token 与现有 Vue 组件。B 最贴近 `resource/apple-design`，A 在结构上最彻底，C 最大胆。

### 方向 A：编辑台（Editorial Desk）— 推荐

- **一句话气质**：像一本排版讲究的工作日志——黑底、细线、少色；数字和标题负责层级，盒子退出。
- **真实参照**：Ghost Admin（`ghost.org`，编辑优先、克制的侧栏与列表）、GitHub Issues 列表（`github.com`，行式密度、状态用点与文字）、Linear（`linear.app`，紧凑工具感、单一强调色）。均为公开可访问产品界面，不引用虚构站点。
- **token 改动**（示意，已用于注入预览）：
  - 背景 `#0a0a0b`；卡片/面板退到 `#121214` 或直接透明；边框 `#242428`、hover `#37373d`。
  - 主色改为**近白** `#cfcfd6`（`--color-primary`），主按钮白底黑字；蓝保留给链接/图表/焦点以外？本方向建议完全退出按钮，只留焦点环。
  - 圆角：输入/按钮 6px，卡片 8px，浮层 10px；删除 9/11/14/20/999 的混用（胶囊只保留头像、开关、未读角标）。
  - 字号：11（辅助）/12（标签）/13（正文）/14（列表标题）/20（页面标题）/24（仪表盘标题）；删除 9/9.5/10/10.5/11.5/12.5/13.5/14.5/15.5/16.5/17/18/19/22/28 的任意值。
  - 间距：沿用 4/8/12/16/24/32/48/64；用 24/32 的分区间距代替边框。
- **组件改动**：
  - `VPageHeader`：去掉 `icon` 插槽使用，标题 24/600，说明仅在必要时保留一行；去掉下边框（用间距分隔）。
  - `VCard`：改为“无背景、无边框、仅顶部 hairline”的 section 容器；真正的卡片只保留给浮层/媒体。
  - `VButton`：6px 圆角；primary 白底黑字；default 透明 + 1px 细边。
  - `VTabbar`：默认样式去图标，激活态白色下划线；不使用 segmented 阴影。
  - 状态：统一“5px 圆点 + 12px 文字”，删除彩色底胶囊。
  - `VEmpty`：保留图标但降到 40px、改左对齐或与操作同列；或直接用一行文字 + 操作。
- **要删掉的元素**：页面标题图标；设置分区蓝色竖条；彩色状态胶囊；`SettingsEffectGuide` 整块；快捷操作 `desc`；“暂无对比数据”；评论快捷键横幅；海报卡片的玻璃浮层/胶囊/阴影（如保留网格视图，改成极简封面 + 文字）。
- **页面结构变化**：
  - 仪表盘：顶部一行事实（文章/已发布/浏览/评论，无卡片，只有数字与细线）→ “访问趋势”大区块（无内嵌小卡）→ 两列“最近文章 / 最近评论” → 快捷操作降级为标题右侧的小文字链接。删除四张 KPI 卡的图标与 delta。
  - 文章/页面：单层列表；工具栏与摘要之间用一条 hairline；行高 56；操作图标降为 hover 显现（移动端常显）。
  - 设置：去掉说明块，表单直接开始；分区标题 15/600 + 上下 24px 留白；Tab 去图标、无卡片外框；移动端 Tab 横向滚动但显示渐隐遮罩提示。
  - 评论：摘要降为一行四个数字；状态用点 + 文字；每条评论改为分隔线行，不用卡片。
- **影响文件**：`styles/main.css`、`styles/apple-base.css`、`layouts/DefaultLayout.vue`、`VPageHeader.vue`、`VCard.vue`、`VButton.vue`、`VTabbar.vue`、`VEmpty.vue`、`modules/dashboard/*`（View + 5 个 widget + dashboard.css）、`PostList.vue`、`PageList.vue`、`CommentList.vue`、`SettingsView.vue`、`settings-form.css`、`SettingsEffectGuide.vue`、`BasicSettings.vue`、`AppearanceView.vue`、`CommentUsersView.vue`、`MomentList.vue`、`WikiProjectsView.vue`。
- **风险 / 工作量**：中高。CSS 面广、E2E 断言绑定了 8px 圆角与 `.stats-card` 结构（`console-ui-consistency.spec.ts:370-382`），需按新规格更新并说明依据。估算 3–3.5 人日（P0 0.5、P1 1.5、P2 1、验证 0.5）。
- **新依赖**：无。
- **390 影响**：底部导航不变；页头内边距需从 40 降到 16–20；4 项统计仍 2×2，但去掉卡片后要用分隔线与 24px 间距维持分组；列表 56px 行 + 44px 触控；设置说明块移除可把首屏上移约 150px；下划线输入需保证 `#333` 以上的可见度；13 个 Tab 去图标后可少滑 1–2 屏，但仍需横滑。

### 方向 B：Apple 分组工作台（最贴近设计库）

- **一句话气质**：iOS 设置页式的分组表单 + Apple Store 式留白，圆润但安静。
- **真实参照**：Apple Human Interface Guidelines 的 Settings 模式与 `developer.apple.com/design`；仓库自带 `resource/apple-design`（卡片由边框与留白定义、按钮全胶囊、1.2rem 全局圆角、阴影只给浮层）。
- **token 改动**：严格按库里 `.dark` 语义层；卡片 19.2px（`--radius: 1.2rem`），输入 12px，按钮全胶囊；阴影只在 hover/浮层；主色保持 Apple 蓝 `#2e8dff`；背景 `#000`、卡 `#1c1c1e`。
- **组件改动**：`VButton` 默认胶囊；`VCard` 恢复 19.2px 圆角与轻阴影；设置改为左侧分组索引 + 右侧 inset grouped 表单（label 左、控件右、行分隔线内缩 16px）；列表改为分组容器（如 iOS 的 grouped list）。
- **要删掉的元素**：卡中卡、状态彩色胶囊、标题图标、蓝色竖条、说明块、快捷宫格（改为文字行）。
- **结构变化**：保留少量大卡，但一屏最多 2 层容器；设置改为“分组列表”而不是“卡片套表单”；仪表盘 KPI 合并为一张卡内的四列。
- **影响文件**：与 A 类似，但 `VCard`/`VButton`/`SettingsView` 改动更大（需要改模板结构），列表页只需去内层卡。
- **风险 / 工作量**：中。最安全、最符合仓库既定设计库；但“Apple 商城语言”用于高密度后台，19.2px 圆角 + 全胶囊可能牺牲扫描效率；且它没有改变模板骨架，预计 AI 味只能部分降低。2–3 人日。
- **新依赖**：无。
- **390 影响**：分组列表与胶囊在窄屏表现最好；大圆角在小屏略显“卡片化”，需要把 19.2px 在 ≤640 降到 12–14px，否则列表会显胖。

### 方向 C：单色终端（Monochrome Desk）

- **一句话气质**：把 Console 当成一台本地工具——等宽数字、2px 直角、没有彩色，信息像日志一样排列。
- **真实参照**：Linear（`linear.app`）的密度、Warp（`warp.dev`）的终端语汇、JetBrains Mono 的工程感。
- **token 改动**：`--font-sans` 改为 `JetBrains Mono`（中文回退 PingFang）；`--radius-*` 全部 2px；背景 `#070708`、边框 `#1d1d1f`；主色近白 `#dedee2`；图表 5 色全部替换为灰阶；状态只保留绿/橙/红小方点。
- **组件改动**：按钮 2px 直角；输入只留下划线；列表行 46px；侧栏去图标、行 28px；仪表盘卡片改为 1px 线框；环图灰阶；分段控件变为下划线文字。
- **要删掉的元素**：所有彩色胶囊与图表彩色、侧栏图标、快捷宫格、页面标题图标、说明块、渐变/阴影、圆角。
- **结构变化**：事实指标横向排成一行等宽数字；列表更密；设置分区像配置文件；仪表盘保留趋势与最近内容，但去掉“快捷操作”。
- **影响文件**：`main.css`、`apple-base.css`、`DefaultLayout.vue`、全部列表/设置/仪表盘 widget、`CategoryDonutWidget.vue`（必须修 token 名）、`registry.ts`。
- **风险 / 工作量**：高。识别性下降（去图标）、12px 等宽在 390 上偏小、长时间阅读疲劳；适合“工程师自用”，不适合作为面向内容创作者的默认皮肤。3–4 人日。
- **新依赖**：无（JetBrains Mono 已引入）。
- **390 影响**：等宽 12px 需提升到 13px、行高 48；底栏图标必须保留；设置 Tab 去图标后仍可横滑；整体可读性风险最高。

---

## 6. 推荐：方向 A，分三步落地

**为什么 A 能真正消除 AI 味**：

1. **改变层级来源**：从“每个模块一张卡”变成“标题—数字—分隔线—留白”。同一屏的方块数量减半，视觉重心回到内容与数字。
2. **改变强调语法**：蓝色退出标题、状态、分区装饰，只保留主操作与焦点；状态变成“点 + 文字”。这会立刻去掉“AI SaaS 模板”的最大色彩特征。
3. **删掉“解释自己”的层**：说明块、desc、delta 占位、快捷键横幅都是生成式界面的典型自证；删掉后界面更像“人做的工具”。
4. **尺度收敛**：23 种字号 → 6 种，18 种圆角 → 4 种，35 个裸 hex → 0（除功能性色板）。这一步单独就能显著降低“怪但说不出”的感觉。
5. **保留本地设计库的语义 token 与 DM Sans**，不引入新体系；只把“盒子优先”改为“留白优先”，这与设计库 README 的 “Whitespace carries hierarchy” 一致。

**落地切片（每片 ≤1 天，可独立验收）**：

| 切片 | 内容 | 验收 |
|---|---|---|
| P0（0.5d） | 删页面标题图标与重复副标题；状态胶囊改点+文字；去设置蓝色竖条；移除效果说明块；删“暂无对比数据”/快捷 desc；评论快捷键横幅移除；`/appearance` 重定向到 `/settings?tab=appearance` 并去掉重复菜单 | 1280/390 五页截图对比；`grep` 无新增裸 hex；E2E 现有关键路径全绿 |
| P1（1.5d） | 仪表盘结构（事实行、去卡中卡、快捷操作文字化、修 donut token）；列表/评论去内层卡；统一状态与操作按钮 | 仪表盘首屏无同构 KPI 卡；文章/评论列表单层；donut 颜色跟随 token |
| P2（1d） | 设置表单去卡片外框、分组留白；字号/圆角/颜色尺度收敛；更新 `ui-restraint` 增加“禁止卡片套卡片、禁止标题图标、禁止彩色状态底、字号/圆角白名单”规则 | 字号 ≤6、圆角 ≤4、非功能性 hex =0；`make console-test` + 受影响 E2E + 真实浏览器 1280/390 |

**取舍**：A 把 primary 从蓝改成近白，是对 tool-style 计划“主按钮仍蓝底黑字”的显式修改，需要用户裁决；若保留蓝，则至少把标题图标、状态胶囊、分区竖条、横幅里的蓝全部收掉。

**不建议直接把 B 作为主方向**：它最合规、风险最低，但只是把现有盒子变圆、变软，模板结构仍在；用户大概率仍会说“AI 味”。B 更适合作为 A 的“表单语言”补充（设置分组、浮层圆角）。

---

## 7. 390px 影响汇总

| 方向 | 影响 | 风险 |
|---|---|---|
| A | 页头 16–20px 内边距；统计 2×2 用线分隔；列表 56px 行 + 44px 触控；设置首屏上移约 150px；Tab 去图标但仍需横滑 | 下划线输入/选择器需 ≥#333 对比；原生 select 箭头需保留；hover-only 操作在手机上必须常显 |
| B | 分组列表 + 胶囊最适合触屏；19.2px 圆角在 ≤640 应降到 12–14px | 卡片仍多，首屏效率一般；大圆角可能显“胖” |
| C | 等宽 12px 需升到 13px；行 48px；保留底栏图标 | 可读性与识别性风险最高；不建议作为默认 |

---

## 8. 风险、未验证项与待裁决

**待用户裁决**
1. 是否接受主按钮从“蓝底黑字”改为“白底黑字”（A 的核心视觉变化之一）。
2. 是否允许删除 `/console/appearance` 入口（或改为重定向），统一到设置 → 外观。
3. 海报卡片视图：删除、保留但极简化、还是仅保留为可选且不再记忆为默认。
4. 设置页 13 个 Tab 是否合并（如“作者与友链”“反馈”降为分组内子项），还是仅去图标。

**风险**
- E2E/单测绑定当前 8px 圆角、`.stats-card` 高度、`ConsoleStatsCard` 结构（`tests/e2e/console-ui-consistency.spec.ts:370-382`、`tests/e2e/console-mobile-a11y.spec.ts:530-570`）。改规格必须同步改测试并写明依据，否则违反仓库的“测试预期变更需说明”规则。
- 现有 `ui-restraint.test.ts` 的 9 条规则会放行 A 要删除的东西（卡片、胶囊、图标、字号漂移）；需要新增规则，否则改动会回潮。
- 分类环图/registry 的 token 名失配属于功能性缺陷，A/C 都依赖修复；修复后需确认颜色确实变化，而不是只看代码。

**未验证**
- 未在真机/多浏览器验证；仅 Chromium（Playwright + 本机 Chrome）。
- 未能看到带评论数据的评论列表（隔离夹具未启用评论）；相关结论仅来自源码。
- 注入预览是 CSS-only，不能验证 GridStack 重排、环形图重着色、真实移动端滚动性能。
- 未检查 `/uc/`（用户中心），不在本轮范围。
- 未运行 `make test`/`make build` 之外的完整门禁；本轮没有改代码，构建仅用于截图。

---

## 9. 复现方式

```bash
# 1) 构建当前 Console
make console

# 2) 隔离服务（独立 DB/config/content/upload，端口 8135）
E2E_LISTEN=:8135 bash scripts/e2e/run-server.sh

# 3) 登录 admin / e2e-password，创建夹具并截图
#    脚本位于 .tmp/style-refresh/（未入库）；截图与注入 CSS 已归档到 docs/baselines/console-style-refresh/
```

截图均为 1280×900 / 390×844，Chromium；`before-*` 无注入，`preview-A/C-*` 为 CSS 注入，预览文件仅供方向裁决，不代表已实现。
