# BLG-20261031-01：Console「方向 A 编辑台」视觉重构

- 状态：第三轮收尾完成，等待独立复验；未提交、未合并、未发布，未改动根工作树用户 WIP。本轮之后不再扩范围。
- 基线：`dev`（本工作树创建时的最新提交）；分支 `codex/refactor/20261031-BLG-20261031-01-console-editorial-desk`；工作树 `personal-blog/20261031-refactor-BLG-20261031-01-console-editorial-desk`。
- 视觉依据：`docs/baselines/console-style-refresh/findings.md` 第 5A/6/7/8 节与 `preview-A-*.png`；证据已按原名复制到 `docs/baselines/console-editorial-desk/reference/`。
- 范围：仅 Console 视觉与共享控件样式；保留信息架构、路由、权限、字段、API 类型、业务语义、Stellar 前台和编辑器边界。海报/网格视图、存储偏好、设置效果导读与草稿预览只简化视觉，不删除功能。

## 设计裁决

- 全局字号白名单为 11/12/13/14/20/24。任务卡设置分区标题写作 15/600，与白名单冲突；采用 **14/600**，以白名单为上位约束，视觉上仍靠字号、字重与留白区分。
- 主操作白底黑字：`--color-primary: #e8e8ec`，文字 `#000000`；蓝色只用于焦点环 `--color-ring` 与功能性图表。
- 圆角：输入/按钮 6，卡片/容器 8，浮层 10；胶囊只保留头像、开关、未读角标。
- 状态统一为 5px 圆点 + 12px 文字，不再使用彩色状态底；表格/列表行高 56px。
- 卡片套卡片从机械门禁中禁止；设置页取消外层卡片，分区由标题、留白与细分隔线组织。

## 主要改动

- 令牌与共享控件：`styles/main.css`、`styles/apple-base.css`、`VButton`、`VCard`、`VPageHeader`、`VTabbar`、`VEmpty`、`VStatusDot`。
- 仪表盘：事实行、整宽趋势、最近文章/评论双列、文本快捷入口；移除 KPI 图标、环比与“暂无对比数据”，重写默认网格布局。
- 列表与评论：单层记录列表、工具栏与摘要细分隔线、桌面 hover 显隐操作、评论四项一行摘要与圆点状态。
- 设置与外观：去外层卡片与蓝色条、下划线分区导航、扁平分区、导读弱化为单行 + 可展开详情，保留草稿预览与未保存提示。
- 缺陷修复：`themeColors` 现在同时兼容 `chart2`/`chart-2`/`--color-chart-2` 与 `primaryHover`，并拆分字母数字边界；补 `modules/console/tests/theme-colors.test.ts`。
- 机械门禁：`modules/console/tests/ui-restraint.test.ts` 新增方向 A 规则（卡片嵌套、标题图标、彩色状态底、字号白名单、圆角白名单），并用临时探针证明五条规则会真实失败后删除探针。

## 测试预期调整（均保留原检查强度）

| 位置 | 旧预期 | 新预期 | 依据 |
|---|---|---|---|
| `console-ui-consistency.spec.ts` 主按钮 | 8px | 6px | 方向 A 工作控件圆角 |
| `console-mobile-a11y.spec.ts` UC 共享按钮 | 8px | 6px | 共享 `VButton` 同步收紧 |
| 设置分区横向 padding | 手机 16 / 桌面 28 | 分区 0；容器 16 / 32 | 页边距上移到 `.settings-container`，分区只负责纵向留白；仍逐项断言数值 |
| `stellar-settings-groups.spec.ts` 分区 padding | 16 / 28 | 分区 0；容器 16 / 32 | 同上 |
| `console-ui-consistency.spec.ts` 主导航路径 | 逐字等于侧栏 href | 允许既有模块入口重定向（`/seo` → `/seo/overview`、`/plugins` → `/plugins/shortcodes`） | 路由既有语义；标题、横向溢出与页面可达断言不降级 |

## 返工修复（独立审查与交互验收后）

| 阻断/问题 | 修复 | 证据 |
|---|---|---|
| 仪表盘紧凑布局未生效 | 默认尺寸保留为声明值，归一化只修正历史非声明尺寸；统计 h12 / 趋势 h44 | `layout.ts`、`grid-layout.test.ts`、`widget-size-presets.test.ts`；1280 统计 96px、趋势 352px |
| 规范保存条透明、横向 padding 回归 | 保存条 `background: var(--color-background)`、`padding: 16px 0 + safe-area`，横向留白只由容器 32/16 提供 | `CanonicalSettingsForm.vue`；`final/measurements.json#settings` 背景 `rgb(10,10,11)`、padding 16/0 |
| 390 评论行可读性回归 | 信息列 `min-width:0`，状态 `nowrap`，操作整行换到信息下方，勾选命中区 44px | `CommentList.vue`；390 状态 47×18、邮箱 115px、操作 top ≥ 信息 bottom |
| Shortcode 重试白底白字 | 重试文字改用 `--color-primary-foreground` | `ShortcodeStudioView.vue`；E2E 断言文字 `rgb(0,0,0)` |
| 海报选择反馈/命中区/置顶可读 | 选中底 `--color-accent`、操作常显；选择 28px 桌面/44px 窄屏；置顶移入静态 meta 行 | `PostList.vue`；选中背景 `rgb(27,27,30)`、操作 opacity 1、置顶 position static |
| 仪表盘快捷链接越权/模块删减 | 新增 `buildDashboardQuickLinks`，同时检查导航 manifest、模块启用与权限 | `dashboard-quick-links.ts`、`dashboard-quick-links.test.ts`（管理员/作者/读者/模块禁用四组） |
| 头像预览非圆形 | 预览容器 `border-radius: 50%` | `settings-form.css`；运行期 128px / 50% |
| 设置导读标题重复、对比度、触控 | 可见标题移入 `sr-only`，摘要与结果同为 `--color-text-tertiary`，手机 44px | `SettingsEffectGuide.vue`；标题节点 0、摘要高 44px、两色一致 |
| 字号白名单未闭合（继承 16px） | 补齐 `.apple-link`、`.settings-help`、`.color-current` 等继承字体；门禁增加运行时全页扫描 | `main` 六页 visible text 仅 11/12/13/14/20/24，`bad` 全为 0 |
| 触控目标不足 | 仪表盘快捷链接 44×44、评论勾选 44×44；相关列表控件统一 44px | `final/measurements.json`；E2E 同时断言宽与高 |
| Feedback padding / Trend 提示色 / J-K 焦点 / 空媒体 | 反馈分区 24px 0；提示点描边改 background、提示框用 popover/border；评论焦点行内描边；删除 PostSettings 空查询并新增空 @media 机械门禁 | 对应源码 diff 与 UI 门禁 |
| MenuView / ModulesView 残留旧视觉 | 菜单容器 32/16 边距；模块页改单层记录行，移除外层卡片与标题图标 | `MenuView.vue`、`ModulesView.vue`；E2E 断言无 `.card-wrapper`、记录行透明 |
| 源码门禁盲区 | 门禁范围改为实际改动文件；状态规则覆盖 `bg-[#hex]`；圆角命名档位与样式块内字体生效；Hex 全局/逐文件白名单 | `ui-restraint.test.ts`；临时探针证明状态/字号/圆角/Hex 规则会失败后删除 |

## 第三轮收尾修复（独立复验 P0/P1）

| 问题 | 修复 | 证据 |
|---|---|---|
| P0 尺寸弹窗按列宽去重、识别按宽×高 | `uniquePresetOptions` 与 `getWidgetSizePreset` 共用 `w×h`；删除“sm 只保留 small”的旧预期；补钳制撞档与任意宽高识别的穷举断言 | 默认布局遍历单测（sm/xs/xxs）；sm/xxs 趋势 6×36/44/52、4×32/42/50；五个组件 medium 选中；xs 为兼容断点，仅单测覆盖 |
| 缺少真实尺寸弹窗验证 | 新增 Playwright：sm/xxs 逐项打开趋势、最近文章、最近评论、分类、版本 | `console-ui-consistency.spec.ts`；截图 `final/after-size-modal-{sm,xxs}-1280.png` |
| 评论快捷键帮助被删 | 恢复紧凑入口 + 232px 弹层；44×44、`aria-expanded`、Esc 关闭 | 390/1280 E2E；`final/after-comments-shortcuts-*.png` |
| `.guide-save` 对比度 3.7:1 | 改 `--color-text-tertiary`，加入 E2E 与 final 度量 | 4.91:1（`rgb(126,126,133)` on `#0a0a0b`） |
| PostSettings 残留空 `@media` | 删除空块，新增空 `@media` 机械门禁（Console 源码） | 门禁 0 违规 |
| J/K 焦点行操作不可见 | `.comment-row.is-focused .comment-actions` 恢复 opacity | E2E 焦点行 opacity = 1 |
| VPageHeader `#icon` 死插槽 | 清理 11 个页面标题图标插槽与无用 import；门禁改为扫描 Console 全部 `.vue` | 页面标题规则 0 违规 |
| 残留状态/分类胶囊 | FriendList/MomentList/UserListView/CommentUsersView 主表改圆点+文字；分类/角色/计数胶囊改 6px；移除内联状态背景 | 状态/圆角门禁 0 违规；CommentUsersView 抽屉旧标签保留为已知残留 |
| Shortcode 重试 34px / 评论孤立“·” | 手机 44px、圆角 6px；meta 分隔条件渲染 | 390 E2E 宽高 ≥44 |
| 门禁口径 | 状态规则抓任意 `bg-[#hex]` 与内联 `backgroundColor`（逐文件功能色白名单）；`rounded-full`/9999/50% 改选择器上下文白名单；空 media、标题图标全量扫描 | `ui-restraint` 15 条规则；临时探针证明会真实失败后删除 |

## 验证记录

- `make test`：**exit 0**。Console 369/369、Stellar 240 项中 238 通过/2 skipped、Go vet/staticcheck/go test 全通过；重新生成 `ri-subset.json` 纳入快捷键图标。
- `make build`：**exit 0**（Console、UC、Go 二进制）。
- 定向门禁：`ui-restraint` + `widget-size-presets` + `theme-colors` + `dashboard-quick-links` 共 27/27 通过；状态/字号/圆角/Hex/空 media 规则用临时探针确认会真实失败后删除探针。
- 四个目标 Playwright（Chrome 通道，**73/73**）：`console-ui-consistency`、`console-mobile-a11y`、`console-settings-canonical`、`console-settings-save-race` 全部通过（3.3 分钟）。
- 8140 隔离真实运行：独立根 `.tmp/rework-server.8140`（DB/config/content/upload），真实 `blog` 二进制 + 生产 dist；0 未捕获异常、0 整页横向溢出；验收后已停止服务并清理临时 root/脚本。
- 构建与 Playwright 完成后仅追加了尺寸识别穷举单测（不改生产源码），产物哈希与 73/73 证据不变；`make test` 复跑为 369/369。
- 实际加载产物：从 `/console/` HTML 取回 6 个入口资源并逐个 SHA-256 与 `modules/console/web/dist` 比较，**6/6 MATCH**：
  - `index-Bw6UwQ7A.js` `bae86c9d0ec1af45…`
  - `index-D-j-weAu.css` `6b69777047727a69…`
  - `api-C-H7IBEY.js` `28f3d58e9e625d42…`
  - `pinia-CXeiBUD1.js` `7e7c94b9658d5172…`
  - `preload-helper-l0U7FUPx.js` `dbd40ae429a13a3e…`
  - `runtime-core.esm-bundler-GmtFT3dp.js` `bcd39943a10cb760…`
- 评论证据使用隔离 DB 真实种子（6 条，覆盖 approved/pending/rejected/spam 与一条管理员回复），不是空态截图；文章夹具共 8 篇（5 已发布 / 2 草稿 / 1 回收站，1 篇置顶）。
- `scripts/e2e/prepare.go` 的隔离站点预开 `native_comments.enabled` 并固定 `admin_email`，使公开评论接口在 E2E 启动时挂载、3 条评论夹具不受同 IP 60 秒 1 条限流影响；生产限流规则未改。
- 字号运行时扫描口径为**六个主页面**（仪表盘、设置基本/外观、外观、评论、文章）；字号/状态/圆角源码门禁扫描 `ui-restraint` 的 `DIRECTION_A_SCOPE` 白名单，空 `@media` 与 `VPageHeader #icon` 规则扫描 Console 全部 `.vue`；不声称“Console 整体”全量覆盖。

### 需要兼容修正的既有调用方

- `--color-primary` 变为近白后，旧代码中「主色底 + 白字」不再可读：已将 `VTag`、附件字段/媒体库选中态、Shortcode/素材/脚本插件选中态、Wiki 文档选中态改为 `--color-primary-foreground`。
- 导航冒烟暴露出旧页面的手机触控目标：已补齐 topics/taxonomy/attachments/friends/subscriptions/users/notifications/plugins/modules/moments/system-update 的图标与筛选控件 44px 目标；标题页仍保留既有信息结构。
- `VTabbar` 在 767px 以下统一 44px 高；分段图标按钮补齐 44×44。

## 量化对比（方向 A 收敛范围）

| 指标 | 返工前（独立验收） | 返工后（final 度量） |
|---|---|---|
| 仪表盘统计卡 | 声明 h12，加载后扩为 h19（约 152px） | h12 保持，1280 实测 96px；390 四卡 2 列、均 96px |
| 仪表盘趋势卡 | 声明 h44，加载后扩为 h52（约 416px） | h44 保持，1280 实测 352px |
| 尺寸弹窗选项 | sm 趋势仅 small；xxs 中档不可选 | 5 类组件 sm/xxs 均三档，medium 选中 |
| 设置保存说明对比度 | quaternary 约 3.7:1 | tertiary 4.91:1 |
| 快捷键帮助 | 无入口 | 入口 44×44、弹层 232px |
| 390 评论状态 | 34×36，竖排折行 | 47×18，单行 `nowrap` |
| 390 评论邮箱 | 可见 34px | 115–144px |
| 评论操作位置 | 与信息列争宽 | 整行换到信息下方（top ≥ info bottom） |
| 评论勾选命中区 | 16×16，无 label | 44×44 |
| 仪表盘快捷链接 | 39×20 / 26×20 | 44×44 |
| 设置保存条 | 透明底、横向 padding 重复 | `rgb(10,10,11)` 不透明、padding 16/0、容器 32/16 |
| 可见字号 | 仍有 16px 继承泄漏 | 六页仅 11/12/13/14/20/24，`bad = 0` |
| 字号种类 | 20 | 6 |
| 圆角越界值 | `2/3/4/5/7/9/11/12/14px` | 清零；仅 0/6/8/10/50%/9999px 与半径 token |
| 裸 Hex | 205 | 124；剩余为设计令牌、图表功能色与上游默认值 |

## 证据

- 最终证据：`docs/baselines/console-editorial-desk/final/`（14 张 PNG、`measurements.json`、`verification.json`、`report.md`）。
- 独立审查/验收原始证据保留在 `docs/baselines/console-editorial-desk/acceptance/`，未被覆盖。
- 参考依据：`docs/baselines/console-editorial-desk/reference/`（findings.md、preview-direction-A.css、preview-A-*.png、before-*.png）。
- 运行条件：8140 隔离根 `.tmp/rework-server.8140`，真实 `blog` 二进制 + 生产 dist；截图使用安装版 Chrome 通道（Playwright 浏览器下载在当前网络下不可用，未把下载失败当作通过）。
