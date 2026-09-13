# Console UI 一致性修复实施记录

本文档为《Console UI 一致性修复计划》的唯一任务与执行跟踪文档。

- **基线版本**：`dev / 5b66a30`
- **唯一设计参照图**：`docs/baselines/console-ui-consistency/design-reference.png`
- **设计规范**：黑灰底色、胶囊按钮、黑色文字高对比主操作、无发光、2px 焦点环、单卡片表单层级。

---

## 1. 任务进度汇总

| 任务 | 目标与可见结果 | 对应分支 / 工作树 | 状态 |
|---|---|---|---|
| **T1** | 桌面底栏重复消除，手机主要操作完整可见且可点击，主题设置可从侧栏与更多抽屉进入 | `../Blog-t1-nav` (`codex/console-t1-nav`) | 已验收 |
| **T2** | 基础按钮统一尺寸与胶囊形状、主按钮黑字高对比、Tab 焦点环清晰、主题设置黄点误报消除 | `../Blog-t2-controls` (`codex/console-t2-controls`) | 已验收 |
| **T3** | 列表概览卡片统一样式与高度、系统设置与主题设置统一分段 Tabs 并优化多层卡片、修正空列表全选文案 | `../Blog-t3-views` (`codex/console-t3-views`) | 已验收 |

*状态流转规范：未开始 → 实现中 → 待体验 → 已验收（必要验证未结束仍为实现中，仅在用户确认后标记已验收）。*

---

## 2. 施工与验证日志

### T1：主要操作完整可见，导航入口完整（已验收）
- **负责人**：主 agent (T1 worker)
- **改动文件**：
  - `modules/console/web/src/layouts/DefaultLayout.vue`
  - `packages/components/src/ui/VPageHeader.vue`
  - `modules/console/web/src/modules/posts/PostList.vue`
  - `modules/console/web/src/modules/pages/PageList.vue`
  - `modules/console/web/src/modules/attachments/AttachmentList.vue`
  - `internal/module/core/core.go`
  - `modules/console/api/module_manifest_test.go`
  - `tests/e2e/console-ui-consistency.spec.ts`
- **执行过程**：
  - [x] 创建工作树 `../Blog-t1-nav`，挂载 Git hook。
  - [x] 生成唯一设计参照图 `docs/baselines/console-ui-consistency/design-reference.png` 并建立基线说明。
  - [x] 建立跟踪文档 `docs/plans/console-ui-consistency.md`。
  - [x] 移除 `DefaultLayout.vue` 底部 `<nav>` 内联 `display: grid`，声明 `grid md:hidden`。
  - [x] `VPageHeader.vue` actions 允许响应式换行与自适应收缩。
  - [x] 文章、页面、媒体库窄屏（`<1024px`）自适应多行排布，保证 390px 主操作完整可见可点击。
  - [x] `core.go` 增设 `/appearance` 主题设置菜单项（权限 `admin`，分组“外观”，图标 `ri:palette-line`），Go 测试断言通过。
  - [x] 编写并执行 E2E 测试 `tests/e2e/console-ui-consistency.spec.ts`（6 个断言全绿，并留存基准截图）。
  - [x] 执行代码质量与构建门禁（`make console-test` / `make test` / `make build`）。
  - [x] 通过 `make merge-check` 顺次将分支 `codex/console-t1-nav` 合入主 `dev` 分支，组合态复验全绿。用户验收确认通过，标记为 **已验收**。

### T2：公共控件统一，修改状态真实（已验收）
- **负责人**：主 agent (T2 worker)
- **改动文件**：
  - `packages/components/src/ui/VButton.vue`
  - `modules/console/web/src/modules/settings/settings-state.ts`
  - `modules/console/web/src/modules/settings/AppearanceTab.vue`
  - `modules/console/web/src/modules/appearance/AppearanceView.vue`
  - `modules/console/web/src/modules/posts/PostEditor.vue`
  - `modules/console/web/src/modules/pages/PageEditor.vue`
  - `modules/console/web/src/styles/apple-base.css`
  - `modules/console/tests/settings-normalize.test.ts`
  - `tests/e2e/console-ui-consistency.spec.ts`
- **执行过程**：
  - [x] 创建工作树 `../Blog-t2-controls`，挂载 Git hook。
  - [x] 统一 `VButton.vue`：高度阶梯（xs 24 / sm 28 / md 36 / lg 40px）、胶囊圆角在尺寸后定义、主按钮黑字高对比度、无外发光、2px 焦点环、移动端触控保证（<768px 时 min-height: 44px）。
  - [x] 统一 `PostEditor.vue` / `PageEditor.vue` 按钮接入规范，确保 `/uc/` 无样式泄漏。
  - [x] `settings-state.ts` 抽取纯函数 `normalizeStyleSettings`，在 `takeSnapshot()` 前调用，移除 `AppearanceTab.vue` setup 副作用。
  - [x] 增加 `settings-normalize.test.ts` 纯函数单测（覆盖 null/primitive、未知键保留、显式 false 保留、blur_px 规范化与幂等性）。
  - [x] 扩充 `tests/e2e/console-ui-consistency.spec.ts` 自动化覆盖 T2 所有完成条件（按钮黑字高对比、胶囊圆角、无外发光、Tab 焦点环声明与可访问性、主题设置初始无脏变、真实修改出黄点且保存恢复）。
  - [x] 运行本地全套门禁（`make console-test`、`make test`、`make build`、Playwright 9 用例全部全绿）。
  - [x] 执行 `make merge-check` 将 `codex/console-t2-controls` 稳妥合入 `dev` 分支。用户体验后确认通过，标记为 **已验收**。

### T3：视图统一与多层卡片优化（已验收）
- **负责人**：主 agent (T3 worker)
- **改动文件**：
  - `modules/console/web/src/styles/apple-base.css`
  - `packages/components/src/ui/VTabbar.vue`
  - `modules/console/web/src/modules/settings/settings-form.css`
  - `modules/console/web/src/modules/appearance/AppearanceView.vue`
  - `modules/console/web/src/modules/appearance/LayoutSettings.vue`
  - `modules/console/web/src/modules/appearance/EnhancementsSettings.vue`
  - `modules/console/web/src/modules/appearance/AssetsSettings.vue`
  - `modules/console/web/src/modules/posts/PostList.vue`
  - `modules/console/web/src/modules/pages/PageList.vue`
  - `modules/console/web/src/modules/comments/CommentList.vue`
  - `modules/console/web/src/modules/attachments/AttachmentList.vue`
  - `modules/console/web/src/modules/friends/FriendList.vue`
  - `tests/e2e/console-ui-consistency.spec.ts`
- **执行过程**：
  - [x] 创建工作树 `../Blog-t3-views`，挂载 Git hook，符号链接共享 `resource/`。
  - [x] 统一列表概览卡片 `.stats-card`：规范桌面最小高度 80px、手机最小高度 88px、标准 padding 与边框底色，解决高度跳动。
  - [x] 系统设置与主题设置统一分段 Tabs：`AppearanceView.vue` 接入 `VTabbar(type="segmented")`，单卡片包裹，移除多层嵌套卡片与重复标题。
  - [x] 优化 `LayoutSettings.vue`、`EnhancementsSettings.vue`、`AssetsSettings.vue` 内层卡片结构，消除内外卡片重叠。
  - [x] 修正空列表（文章/页面/评论/附件/友链）全选按钮文案显示“取消全选”的边界缺陷，改为禁用并稳定显示“全选本页”。
  - [x] 补充/更新 E2E 测试并运行全套门禁（`make console-test`、`make test`、`make build`、Playwright 12 项测试全部通过）。
  - [x] 通过 `make merge-check` 合入 `dev`。用户验收确认通过，标记为 **已验收**。

---

## 3. 本轮补齐任务进度汇总 (Round 2)

起点：`dev / b825a9c`。本轮根据《Console UI 修复补齐计划：沿用 T1–T3》系统补齐原计划遗漏与规范细节，严格禁止覆盖未暂存的根目录 `.gitignore`。

| 任务 | 目标与可见结果 | 对应分支 / 工作树 | 状态 |
|---|---|---|---|
| **T1** | 手机标题、搜索、按钮按三行排列，主要操作与底部内容完整可用无遮挡 | `../Blog-r2-t1-layout` (`codex/console-r2-t1-layout`) | 待体验 |
| **T2** | 手机控件易点击（44px）；设置空值严格保留，保存与修改提示准确无假脏态 | `../Blog-r2-t2-controls` (`codex/console-r2-t2-controls`) | 未开始 |
| **T3** | 用户、备份等页面采用统一设计，设置 tabs 与面板联动，空状态符合约定 | `../Blog-r2-t3-views` (`codex/console-r2-t3-views`) | 未开始 |

*状态流转规范：未开始 → 实现中 → 待体验 → 已验收（必须严格按 T1 → T2 → T3 依次交付，仅在用户确认后标记已验收）。*

---

## 4. 本轮补齐施工与验证日志 (Round 2)

### T1：补齐手机布局和无遮挡验收（待体验）
- **负责人**：主 agent / 布局控件 worker
- **改动范围**：
  - `packages/components/src/ui/VPageHeader.vue`
  - `modules/console/web/src/modules/posts/PostList.vue`
  - `modules/console/web/src/modules/pages/PageList.vue`
  - `modules/console/web/src/modules/attachments/AttachmentList.vue`
  - `modules/console/web/src/layouts/DefaultLayout.vue`
  - `tests/e2e/console-ui-consistency.spec.ts`
- **执行过程**：
  - [x] 校正设计参照基准图 `docs/baselines/console-ui-consistency/design-reference.png` 与 `README.md`（修正对比度比值 6.37:1 / 3.41:1 / 6.16:1，补齐三行布局与底栏 5 项）。
  - [x] 创建同级独立工作树 `../Blog-r2-t1-layout` (`codex/console-r2-t1-layout`)，挂载 Git hook。
  - [x] `VPageHeader.vue` 增加可选 `actionsLayout="stack-below-lg"`，默认保持现有布局；仅在文章列表、独立页面、媒体库启用。
  - [x] `<1024px` 时标题区和 actions 各占整行；actions 内搜索占第一整行，按钮组占下一行并允许换行。`≥1024px` 保持横排。
  - [x] 保留滚动隐藏、`inert` 和 `aria-hidden`；保留底栏隐藏规则、手机安全区和动态菜单列数。
  - [x] 检查主滚动容器底部留白（保证滚动到底后最后一个内容操作的完整矩形位于底栏上方，无遮挡且可点击）。
  - [x] 扩充 `tests/e2e/console-ui-consistency.spec.ts`：手机 390px 下标题、搜索、按钮组三行上下排列矩形断言；滚动到底无遮挡断言；文章/页面新建实际进入编辑页；上传附件实际触发文件选择器；桌面端底栏保持隐藏。
  - [x] 运行门禁 `make console-test`、E2E 隔离测试、`make test`、`make build`（全绿通过）。
  - [x] 标记为 **待体验**，准备通过 `make merge-check` 合入 `dev`。

### T2：补齐触控尺寸与设置数据边界（未开始）
- **负责人**：设置 worker / 控件 worker
- **计划目标**：
  - `data-app="console"` 与 `data-app="uc"` 样式隔离；
  - 手机端 `<768px` 输入、搜索、选择器、筛选按钮高度至少 44px，独立图标按钮至少 44×44px；
  - `normalizeStyleSettings` 深拷贝纯函数化（不修改原入参对象，支持深冻结，保留 `"" / 0 / false` 和未知键）；
  - 修复 `SettingsView.saveJson` 与保存快照同步逻辑（成功只更新该分区快照，请求中再编辑保留脏态）。

### T3：补齐页面统一与设置联动（未开始）
- **负责人**：页面 worker
- **计划目标**：
  - 新增展示型 `ConsoleStatsCard` 并复用到文章、页面、媒体库、用户管理、备份；
  - 设置 `VTabbar(type="segmented")` 增加 `tabpanel-*` 容器、`aria-labelledby` 与 `autoReveal` 横向滚入支持；
  - SEO 统一高度与选中样式；仪表盘消除装饰光晕；空列表隐藏批量选择入口。
