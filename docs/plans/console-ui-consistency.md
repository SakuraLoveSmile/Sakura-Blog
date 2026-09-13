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
| **T2** | 手机控件易点击（44px）；设置空值严格保留，保存与修改提示准确无假脏态 | `../Blog-r2-t2-controls` (`codex/console-r2-t2-controls`) | 待体验 |
| **T3** | 用户、备份等页面采用统一设计，设置 tabs 与面板联动，空状态符合约定 | `../Blog-r2-t3-views` (`codex/console-r2-t3-views`) | 待体验 |

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
  - [x] 校正设计参照基准图 `docs/baselines/console-ui-consistency/design-reference.png` 与 `README.md`（修正对比度比值 6.37:1 / 3.41:1 / 6.16:1，保留运行时原有菜单及更多入口，不删菜单适配截图）。
  - [x] 创建同级独立工作树 `../Blog-r2-t1-layout` (`codex/console-r2-t1-layout`)，挂载 Git hook。
  - [x] `VPageHeader.vue` 增加可选 `actionsLayout="stack-below-lg"`，默认保持现有布局；仅在文章列表、独立页面、媒体库启用。
  - [x] `<1024px` 时标题区和 actions 各占整行；actions 内搜索占第一整行，按钮组占下一行并允许换行。`≥1024px` 保持横排。
  - [x] 保留滚动隐藏、`inert` 和 `aria-hidden`；保留底栏隐藏规则、手机安全区和动态菜单列数。
  - [x] 检查主滚动容器底部留白（保证滚动到底后最后一个内容操作的完整矩形位于底栏上方，无遮挡且可点击）。
  - [x] 扩充 `tests/e2e/console-ui-consistency.spec.ts`：手机 390px 下标题、搜索、按钮组三行上下排列矩形断言；滚动到底无遮挡断言；文章/页面新建实际进入编辑页；上传附件实际触发文件选择器；桌面端底栏保持隐藏。
  - [x] 运行门禁 `make console-test`、E2E 隔离测试、`make test`、`make build`（全绿通过）。
  - [x] 标记为 **待体验**，准备通过 `make merge-check` 合入 `dev`。

### T2：补齐触控尺寸与设置数据边界（待体验）
- **负责人**：设置 worker / 控件 worker
- **改动范围**：
  - `modules/console/tests/settings-normalize.test.ts`
  - `modules/console/web/index.html`
  - `modules/console/web/uc.html`
  - `modules/console/web/src/main.ts`
  - `modules/console/web/src/uc/main.ts`
  - `modules/console/web/src/modules/settings/settings-state.ts`
  - `modules/console/web/src/modules/settings/SettingsView.vue`
  - `modules/console/web/src/styles/apple-base.css`
  - `packages/components/src/ui/FilterDropdown.vue`
  - `packages/components/src/ui/VPagination.vue`
  - `modules/console/web/src/modules/posts/PostList.vue`
  - `modules/console/web/src/modules/pages/PageList.vue`
  - `modules/console/web/src/modules/attachments/AttachmentList.vue`
  - `modules/console/web/src/modules/comments/CommentList.vue`
  - `tests/e2e/console-ui-consistency.spec.ts`
- **执行过程**：
  - [x] 创建工作树 `../Blog-r2-t2-controls` (`codex/console-r2-t2-controls`)，挂载 Git hook，建立共享 `resource/` 链接。
  - [x] 在 `index.html`、`uc.html`、`main.ts`、`uc/main.ts` 中设定 `data-app="console"` 与 `data-app="uc"`，样式隔离保证互相不污染。
  - [x] 手机端 `<768px` 确保输入框、选择器、搜索框、筛选按钮高度至少 44px；独立图标按钮与卡片操作按钮至少 44×44px。
  - [x] `normalizeStyleSettings` 纯函数深拷贝重构（使用 `deepCloneJsonSafe`，支持深冻结对象，严格保留 `"" / 0 / false` 和未知自定义扩展键，正确转换 `blur_px`）。
  - [x] 修复 `SettingsView.saveJson` 与保存快照同步逻辑（成功仅更新该分区快照，保存后重置原始 JSON，杜绝未保存假脏态；请求中再次编辑保留脏态）。
  - [x] 补充并更新自动化测试：单测 8/8 全绿，E2E 16 项自动化用例全部通过。
  - [x] 运行本地全套门禁（`make console-test`、`make test`、`make build`、E2E 测试全部全绿通过）。
  - [x] 标记为 **待体验**，准备通过 `make merge-check` 合入 `dev`。

### T3：补齐页面统一与设置联动（待体验）
- **负责人**：页面 worker
- **计划目标**：
  - 新增展示型 `ConsoleStatsCard` 并复用到文章、页面、媒体库、用户管理、备份；
  - 设置 `VTabbar(type="segmented")` 增加 `tabpanel-*` 容器、`aria-labelledby` 与 `autoReveal` 横向滚入支持；
  - SEO 统一高度与选中样式；仪表盘消除装饰光晕；空列表隐藏批量选择入口。
- **涉及文件**：
  - `packages/components/src/ui/ConsoleStatsCard.vue`
  - `packages/components/src/ui/index.ts`
  - `packages/components/src/ui/VTabbar.vue`
  - `modules/console/web/src/modules/posts/PostList.vue`
  - `modules/console/web/src/modules/pages/PageList.vue`
  - `modules/console/web/src/modules/attachments/AttachmentList.vue`
  - `modules/console/web/src/modules/users/UserListView.vue`
  - `modules/console/web/src/modules/backup/BackupView.vue`
  - `modules/console/web/src/modules/settings/SettingsView.vue`
  - `modules/console/web/src/modules/appearance/AppearanceView.vue`
  - `modules/console/web/src/modules/seo/SeoLayout.vue`
  - `modules/console/web/src/modules/dashboard/dashboard.css`
  - `tests/e2e/console-ui-consistency.spec.ts`
- **执行过程**：
  - [x] 创建工作树 `../Blog-r2-t3-views` (`codex/console-r2-t3-views`)，挂载 Git hook，建立共享 `resource/` 链接。
  - [x] 新建 `packages/components/src/ui/ConsoleStatsCard.vue` 统一展示型指标卡组件并在 `packages/components/src/ui/index.ts` 导出。
  - [x] 将文章列表 (`PostList.vue`)、独立页面 (`PageList.vue`)、附件媒体库 (`AttachmentList.vue`)、用户管理 (`UserListView.vue`) 与备份管理 (`BackupView.vue`) 的指标概览卡片统一重构成 `ConsoleStatsCard`。
  - [x] `VTabbar.vue` 实现 `autoReveal` 平滑滚入居中（支持横向溢出视口下的标签平滑聚焦）。
  - [x] `SettingsView.vue` 与 `AppearanceView.vue` 增加规范的 `tabpanel-*` 容器包裹与 `aria-labelledby="tab-*"` / `tabindex="0"` 联动。
  - [x] SEO 二级导航 (`SeoLayout.vue`) 高度统一为 38px 且选中态对齐主题 Primary 蓝底黑字。
  - [x] 仪表盘 (`dashboard.css`) 消除过度装饰光晕（去除径向渐变背景光晕、去除卡片过度投影发光与原件呼吸光晕）。
  - [x] 空列表保持全选禁用（文章、独立页面等列表在空态下保持全选按钮禁用，杜绝出现“取消全选”）。
  - [x] 扩充 E2E 测试套件，19 项端到端用例全部通过；全量单测 `make test` 与 `make build` 编译均通过。
  - [x] 运行 `make merge-check` 完成特征分支验证，成功合入 `dev` 分支。
  - [x] 标记为 **待体验**。

---

## 5. 剩余修复任务进度汇总 (Round 3)

起点：`dev / 2ec14c9`。本轮保留已有实现，继续使用 T1–T3 编号，修复源码审查发现的真实缺口。执行顺序：**T2 → T3 → T1**（优先消除保存丢输入与漏报的可靠性风险）。根目录未暂存的 `.gitignore` 严格保留。

| 任务 | 目标与可见结果 | 对应分支 / 工作树 | 状态 |
|---|---|---|---|
| **T2** | 保存过程中继续编辑，输入与未保存提示保持准确（深复制快照、JSON 草稿未保存判定与文本基线、避免丢失与漏报） | `../Blog-r3-t2-save` (`codex/console-r3-t2-save`) | 实现中 |
| **T3** | 修正 Tabs 自身滚动（禁止 scrollIntoView、focus preventScroll）、补齐条件挂载 tabpanel 外壳、空列表隐藏选择入口、ConsoleStatsCard 迁入 Console 本地组件、SEO 与设计参数复原 | `../Blog-r3-t3-views` (`codex/console-r3-t3-views`) | 未开始 |
| **T1** | 保留已完成布局，补齐真实数据滚动到底部末项可操作可点击且无遮挡的端到端验收 | `../Blog-r3-t1-nav` (`codex/console-r3-t1-nav`) | 未开始 |

*状态流转规范：未开始 → 实现中 → 待体验 → 已验收（必须严格按 T2 → T3 → T1 依次交付，必要验证未结束仍为实现中，全部适用验证通过才为待体验，仅在用户确认后标记已验收）。*

---

## 6. 剩余修复施工与验证日志 (Round 3)

### T2：保存过程中继续编辑，输入与未保存提示保持准确（实现中）
- **负责人**：T2 执行者
- **计划目标**：
  - 请求发出前，深复制本次提交内容；成功后只用提交副本更新分区快照，禁止读取请求结束时的可变表单对象。
  - 请求期间允许继续编辑和切换设置标签；后续编辑必须保留，且继续显示未保存。
  - 保留现有串行保存，不新增跨分区并发。请求期间禁用保存入口并显示加载状态。
  - 保存 A 只能更新 A 的快照，不清除 B 的修改。评论设置顺序保存行为保留：第一步失败不发送下一步，第二步失败不得将第二部分标记为已保存。
  - 请求失败时不修改输入或快照，展示已有错误提示并允许重试。
  - 修复主题设置普通保存 (`AppearanceView.save`)：请求前深复制，API 参数与成功快照都使用该副本。
  - 修复系统设置 JSON 文本覆盖与漏报 (`SettingsView.save/saveJson`)：
    - 保存各分区对应的 JSON 文本基线；加载时默认值规范化完成后同时初始化表单快照和文本基线。
    - 分区未保存状态为“表单与已保存快照不同，或 JSON 草稿与其文本基线不同”。
    - JSON 草稿采用文本比较：格式修改也算未保存；无效 JSON 同样保留并触发离页提醒。
    - 高级标签的未保存提示覆盖其中原始 JSON 编辑器可编辑的分区；页面级离页提醒使用完整分区集合。
    - 普通保存前记录该分区 JSON 文本。成功后，只有文本仍等于请求开始时文本且开始时无独立草稿，才自动同步为提交内容；否则保留用户草稿。
    - `saveJson` 解析失败时不发送请求；成功快照使用本次提交内容规范化后的副本。
    - JSON 保存期间若结构化表单也发生变化，不覆盖新的表单数据；分别比较请求开始时的表单和文本状态，再决定是否回填各自内容。
  - 保留已完成的配置兼容规则（`normalizeStyleSettings` 纯函数深冻结/空值/未知键/blur_px 保留）。
- **涉及文件**：
  - `modules/console/web/src/modules/appearance/AppearanceView.vue`
  - `modules/console/web/src/modules/settings/SettingsView.vue`
  - `modules/console/web/src/modules/settings/settings-state.ts`
  - `modules/console/tests/settings-normalize.test.ts`
  - `tests/e2e/console-ui-consistency.spec.ts`
- **执行过程**：
  - [ ] 创建工作树 `../Blog-r3-t2-save` (`codex/console-r3-t2-save`)，挂载 Git hook，建立共享 `resource/` 链接。
  - [ ] 改造 `AppearanceView.save` 为深拷贝提交与深拷贝快照。
  - [ ] 改造 `SettingsView` 维护各分区 JSON 文本基线，未保存计算纳入 JSON 文本草稿比较与无效 JSON 校验。
  - [ ] 修复 `SettingsView.save` 与 `saveJson` 保存前后基线对比与条件同步逻辑。
  - [ ] 补充保存竞态与未保存漏报单元测试与 E2E 自动化测试。
  - [ ] 运行门禁 `make console-test`、`make test`、`make build` 与 E2E 测试。
  - [ ] 通过 `make merge-check` 合入 `dev`。

### T3：修正 Tabs、空状态与设计偏差（未开始）
- **负责人**：T3 执行者
- **计划目标**：
  - 修改公共 `VTabbar`：`autoReveal?: boolean` 默认改为 `false`，仅系统设置和主题设置显式开启。
  - 使用实际横向滚动容器引用，根据活动标签边界计算所需水平位移并限制在合法滚动范围内，**严禁使用 `scrollIntoView`**；标签完整可见时不滚动；键盘聚焦使用 `focus({ preventScroll: true })`，自动显露不得改变纵向滚动位置。
  - 系统设置和主题设置始终为每个标签创建唯一的 `tabpanel-*` 外壳并设置 `aria-labelledby`；非活动外壳使用 `hidden`，表单子组件使用条件挂载（不为了补齐 ARIA 同时挂载全部表单）。
  - 空列表隐藏选择入口：覆盖文章、页面、附件、评论、友链，当前列表为空时不渲染全选复选框、全选按钮和批量操作入口；有数据未选择显示“全选本页”，非空且全选时显示“取消全选”；将原“空态按钮禁用且可见”测试改为“不存在对应选择入口”。
  - 恢复固定设计参数：
    - `VButton` 字距改为 0，保留现有尺寸、圆角及语义；验证 User Center。
    - SEO 按规范实现：桌面 36px、手机 44px，文字 13px；选中项深灰背景、蓝色图标，带 `aria-current="page"`；保留桌面纵向、手机横向导航。
    - 资源集合入口统一称“媒体库”（包括仪表盘快捷入口）；文件对象、详情和提示中保留“附件”。
    - 管理员统一使用蓝色，作者使用中性色，不用危险红表达角色。
    - `ConsoleStatsCard` 迁入 Console 本地组件目录（`modules/console/web/src/components/ConsoleStatsCard.vue`），更新五处消费并从 `packages/components` 移除导出。
    - 清理五页被替代的重复概览样式。
    - 核对仪表盘去装饰修改，保留 GridStack 尺寸、真实状态指示和图表行为。
- **涉及文件**：
  - `packages/components/src/ui/VTabbar.vue`
  - `packages/components/src/ui/VButton.vue`
  - `packages/components/src/ui/ConsoleStatsCard.vue` (删除公共包导出)
  - `packages/components/src/ui/index.ts`
  - `modules/console/web/src/components/ConsoleStatsCard.vue` (新建本地组件)
  - `modules/console/web/src/modules/settings/SettingsView.vue`
  - `modules/console/web/src/modules/appearance/AppearanceView.vue`
  - `modules/console/web/src/modules/seo/SeoLayout.vue`
  - `modules/console/web/src/modules/posts/PostList.vue`
  - `modules/console/web/src/modules/pages/PageList.vue`
  - `modules/console/web/src/modules/attachments/AttachmentList.vue`
  - `modules/console/web/src/modules/comments/CommentList.vue`
  - `modules/console/web/src/modules/friends/FriendList.vue`
  - `modules/console/web/src/modules/users/UserListView.vue`
  - `modules/console/web/src/modules/backup/BackupView.vue`
  - `modules/console/web/src/modules/dashboard/`
  - `tests/e2e/console-ui-consistency.spec.ts`
- **执行过程**：
  - [ ] 创建工作树并挂载 hook。
  - [ ] 实施 VTabbar 自身内部计算平滑滚动与 focus preventScroll。
  - [ ] 实施 SettingsView / AppearanceView 条件挂载 tabpanel 外壳。
  - [ ] 实施五页空列表选择入口隐藏。
  - [ ] 实施 ConsoleStatsCard 本地迁移、SEO 规范与设计参数复原。
  - [ ] 运行门禁与 E2E 验证并通过 `make merge-check` 合入 `dev`。

### T1：保留已完成布局，补齐能证明无遮挡的验收（未开始）
- **负责人**：T1 执行者
- **计划目标**：
  - 不重做已落实的三行结构，继续仅在文章、页面、媒体库使用 `actionsLayout="stack-below-lg"`，保留默认布局与滚动隐藏时的 `inert/aria-hidden`。
  - 在 390、768px 下验证标题、搜索、操作组依次占行；1280、1440px 下验证桌面布局。
  - 使用足够产生滚动的数据，滚动实际 `main.content` 到底部；必须找到最后一项真实可操作元素，取得完整矩形并断言其位于底栏上方。
  - 实际点击最后一项操作，确认事件到达并产生预期界面变化，防止矩形正确但被透明层遮挡。
  - 保留所有运行时导航项和“更多”；验证更多抽屉、焦点返回和主题设置入口。
  - 若实测仍遮挡，仅调整实际滚动容器底部留白与安全区，禁止隐藏内容、缩小触控目标或删导航项规避。
- **涉及文件**：
  - `modules/console/web/src/layouts/DefaultLayout.vue`
  - `tests/e2e/console-ui-consistency.spec.ts`
- **执行过程**：
  - [ ] 创建工作树并挂载 hook。
  - [ ] 编写 E2E 真实数据滚动至底部、定位最后一项真实可操作元素并在底栏上方触发真实点击的严密用例。
  - [ ] 验证四种视口布局与抽屉导航。
  - [ ] 运行门禁与 E2E 验证并通过 `make merge-check` 合入 `dev`。

