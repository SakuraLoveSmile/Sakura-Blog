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
| **T2** | 保存过程中继续编辑，输入与未保存提示保持准确（深复制快照、JSON 草稿未保存判定与文本基线、避免丢失与漏报） | `../Blog-r3-t2-save` (`codex/console-r3-t2-save`) | 待体验 |
| **T3** | 修正 Tabs 自身滚动（禁止 scrollIntoView、focus preventScroll）、补齐条件挂载 tabpanel 外壳、空列表隐藏选择入口、ConsoleStatsCard 迁入 Console 本地组件、SEO 与设计参数复原 | `../Blog-r3-t3-views` (`codex/console-r3-t3-views`) | 未开始 |
| **T1** | 保留已完成布局，补齐真实数据滚动到底部末项可操作可点击且无遮挡的端到端验收 | `../Blog-r3-t1-nav` (`codex/console-r3-t1-nav`) | 未开始 |

*状态流转规范：未开始 → 实现中 → 待体验 → 已验收（必须严格按 T2 → T3 → T1 依次交付，必要验证未结束仍为实现中，全部适用验证通过才为待体验，仅在用户确认后标记已验收）。*

---

## 6. 剩余修复施工与验证日志 (Round 3)

### T2：保存过程中继续编辑，输入与未保存提示保持准确（待体验）
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
  - `modules/console/tests/settings-save-race.test.ts`
  - `tests/e2e/console-ui-consistency.spec.ts`
- **执行过程**：
  - [x] 创建工作树 `../Blog-r3-t2-save` (`codex/console-r3-t2-save`)，挂载 Git hook，建立共享 `resource/` 链接。
  - [x] 改造 `AppearanceView.save` 为深拷贝提交与深拷贝快照。
  - [x] 改造 `SettingsView` 维护各分区 JSON 文本基线，未保存计算纳入 JSON 文本草稿比较与无效 JSON 校验。
  - [x] 修复 `SettingsView.save` 与 `saveJson` 保存前后基线对比与条件同步逻辑。
  - [x] 补充保存竞态与未保存漏报单元测试与 E2E 自动化测试。
  - [x] 运行门禁 `make console-test`、`make test`、`make build` 与 21 项 E2E 测试全数通过。
  - [x] 通过 `make merge-check` 合入 `dev`。
  - [x] 标记为 **待体验**。

### T3：修正 Tabs、空状态与设计偏差（待体验）
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
  - [x] 创建工作树并挂载 hook。
  - [x] 实施 VTabbar 自身内部计算平滑滚动与 focus preventScroll。
  - [x] 实施 SettingsView / AppearanceView 条件挂载 tabpanel 外壳。
  - [x] 实施五页空列表选择入口隐藏。
  - [x] 实施 ConsoleStatsCard 本地迁移、SEO 规范与设计参数复原。
  - [x] 运行门禁与 E2E 验证并通过 `make merge-check` 合入 `dev`。

### T1：保留已完成布局，补齐能证明无遮挡的验收（待体验）
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
  - [x] 创建工作树并挂载 hook。
  - [x] 编写 E2E 真实数据滚动至底部、定位最后一项真实可操作元素并在底栏上方触发真实点击的严密用例。
  - [x] 验证四种视口布局与抽屉导航。
  - [x] 运行门禁与 E2E 验证并通过 `make merge-check` 合入 `dev`。

---

## 7. 本轮缺口补齐任务进度汇总 (Round 4)

起点：`dev / 663fa0e`。沿用 T1–T3 编号，只补当前源码尚未满足的要求；执行顺序 **T2 → T3 → T1**。根目录未暂存的 `.gitignore`（新增 `.workbuddy/`）全程保留，未提交、未还原。

| 任务 | 目标与可见结果 | 对应分支 / 工作树 | 合并提交 | 状态 |
|---|---|---|---|---|
| **T2** | 保存期间所有保存按钮反馈明确（全局禁用），并用真实页面拦截式竞态测试约束输入/草稿/快照边界 | `../Blog-r4-t2-save` (`codex/console-r4-t2-save`) | `d4c479b` | 待体验 |
| **T3** | Tabs 横向滚动由组件自身承担、空列表不再渲染选择入口、筛选后清理失效选择 | `../Blog-r4-t3-tabs` (`codex/console-r4-t3-tabs`) | `9316d06` | 待体验 |
| **T1** | 三页四视口矩阵、滚动到底末项真实可操作、真实键盘/对比度/触控/概览与 UC 隔离账号验收；自动截图不再写入受跟踪基线 | `../Blog-r4-t1-a11y` (`codex/console-r4-t1-a11y`) | `d60b3ae` | 待体验 |
| **最终补齐** | 统一手机设置页边距（<768px 统一 16px）、补强 640/641/767/768px 断点与全量容器断言、闭环待决定截图与交付记录 | `../Blog-r4-final` (`codex/console-r4-final`) | `906fd22` | 待体验 |

*状态流转规范：未开始 → 实现中 → 待体验 → 已验收（必要验证未结束仍为实现中，仅在用户确认后标记已验收）。*

### 本轮开工前锁定的四项裁决（用户确认「没问题」= 采纳推荐默认值）

1. 危险按钮前景改黑字（`var(--color-primary-foreground)`），与主按钮「蓝底黑字」策略一致。
2. 概览栅格维持现状断裂点（文章/页面/附件 ≥640px 四列，用户/备份 ≥1024px 四列），只做验收断言。*(注：历史规划记录；在收尾阶段已按统一规范修正为：全量五个页面统一在 <1024px 为两列，≥1024px 为四列)*
3. 竞态验收全部使用 Playwright 真实页面拦截，不新增 DOM 测试依赖；`node:test` 只保留结构检查。
4. UC 验收使用 `scripts/e2e/prepare.go` 追加的非管理员夹具账号 `e2e-author / e2e-password`（角色 `author`）；空态表头按「保留 `<th>`、仅移除复选框」处理。

---

## 8. 本轮缺口补齐施工与验证日志 (Round 4)

### T2：保存期间反馈明确并验证真实竞态（待体验）

- **改动原因**：计划要求「所有保存按钮增加全局保存禁用条件 `Boolean(saving)`」；核实发现 21 个保存按钮中 0 个带该条件，且 `MenuView.vue`、`ThemeYamlTab.vue` 各自持有本地 `saving`，完全不接收页面级 `savingKey`。竞态验收此前只有源码正则与「复制一套保存函数」的模拟，缺少真实行为证据。
- **改动内容**：
  - 20 个保存按钮统一加 `:disabled="Boolean(saving)"`（`settings/tabs/*`、`AuthorsTab`、`WidgetsTab`、`CoverTab`、`NavTabsTab`、`PluginsTab`、`AppearanceTab`、`appearance/AssetsSettings|LayoutSettings|EnhancementsSettings`）；`MenubarEditor` 改为 `:disabled="hasErrors || Boolean(saving)"`。
  - `ThemeYamlTab` 新增 `saving?: string` 页面级键并接线（`AppearanceView` 传 `savingKey`），本地保存状态更名 `localSaving`；请求期间**不再禁用 YAML 文本域**（允许继续编辑）。
  - `loading` 仍只对应正在保存的分区；评论组合保存保留 `comments || site_tree` 共同 loading；未禁用任何表单/fieldset；`SettingsView.save/saveJson`、`AppearanceView.save` 的全局防重入判断原样保留；未改动共享 `VButton` 的全局禁用语义；普通/JSON 保存快照逻辑本轮**未修改**（拦截式测试未复现偏差）。
- **涉及文件**：`modules/console/web/src/modules/settings/{tabs/*,ThemeYamlTab,MenubarEditor,…}.vue`、`modules/console/web/src/modules/appearance/{AppearanceView,AssetsSettings,LayoutSettings,EnhancementsSettings}.vue`、`modules/console/tests/settings-save-race.test.ts`、`modules/console/tests/settings-workflow.test.ts`、`tests/e2e/console-settings-save-race.spec.ts`（新建）。
- **受控模拟场景（拦截 `PUT /api/v1/settings/{section}`，测试显式释放响应）** —— 12 项拦截用例，全部通过：
  1. 系统设置普通保存期间改表单：请求体为 A、页面保留 B、未保存提示保留、按钮恢复、再存 B 后提示清除。
  2. 主题设置（cover 分区）普通保存期间改表单：同上。
  3. 普通保存期间编辑 JSON：新草稿不被回写覆盖。
  4. JSON 保存期间再次编辑 JSON：新文本保留且继续提示未保存。
  5. JSON 保存期间切换到表单编辑：新表单输入不被响应覆盖。
  6. 保存 A 分区时 B 分区已有修改：B 的输入与黄点不变（基本/SEO 黄点消失、文章黄点保留）。
  7. 保存失败（HTTP 500）后重试：输入不丢、失败不更新快照、按钮可重试。
  8. 无效 JSON 提交：Toast「JSON 格式错误」且保存请求数为 0。
  9. 评论组合保存第一步失败：第二步请求数为 0。
  10. 评论组合保存第二步失败：`comments` 成功（基线更新为本次提交值）、`site_tree` 失败仍提示未保存。
  11. 保存中切换分区：其他分区保存按钮禁用、输入仍可编辑、结束后恢复。
  12. 请求未完成时按 Enter 提交：全局防重入，保存请求数仍为 1。
- **真实服务场景（不拦截响应，隔离 e2e 服务）**：
  - 用例 13：实际点击保存 → `GET /api/v1/settings` 回读 `site.title` 等于提交值 → 浏览器刷新后输入框值仍为该值且无未保存提示。
- **实际命令与结果（本轮实测）**：
  - `make console-test` → 通过（类型检查 + 209 项 `node:test`）。
  - `make test` → 通过；`make build` → 通过。
  - `pnpm exec playwright test tests/e2e/console-settings-save-race.spec.ts tests/e2e/console-ui-consistency.spec.ts` → 34 passed。
  - 提交时 pre-commit hook（等同 `make test`）全绿；`make merge-check` 合并前后组合态复验通过。
- **偏差与处理**：`modules/console/tests/settings-workflow.test.ts` 原先断言 `ThemeYamlTab` 内的 `saving.value`，随更名一并更新为 `localSaving` 与新增的页面级键断言（结构检查，不是行为证据）。
- **结构性证据边界**：`settings-save-race.test.ts` 删除第三条「自建 isDirty 模拟」，仅保留源码结构检查与 `ensureSettingsDefaults` 纯函数断言，并在文件头注明不构成行为验收。
- **明确不在本轮范围**：独立单保存页面（`AiSettings`、`FriendSettings`、`TopicsView`、`WikiProjects/Docs`、`FriendList` 设置弹层、编辑器保存等）各自只有一个保存入口，`:loading` 已禁用自身按钮，未纳入「接收 saving 的子表单」的全局禁用改造。

### T3：Tabs 滚动归属与空列表选择入口（待体验）

- **改动原因**：`VTabbar` 开启 `autoReveal` 时自身并不是滚动容器（横向滚动在两层外层的 `.settings-tabs` 上），位移计算混用相对偏移属性，无 resize 重查、无 reduced-motion；三个表格列表的表头遗留在空列表下禁用态的选择复选框；搜索/筛选/分页导致列表失效时不会清理旧选择。
- **改动内容**：
  - `VTabbar.vue`：新增 `tabbar-autoreveal` 专用修饰类（`width:100%`、`min-width:0`、`overflow-x:auto`、不换行、标签 `flex:0 0 auto`、文字不截断），根元素成为唯一横向滚动区域；位移统一使用容器与标签 `getBoundingClientRect()` + 当前 `scrollLeft`，完整可见不移动、超出按差值移动并 clamp 到 `0..scrollWidth-clientWidth`；新增 `ResizeObserver`（`onBeforeUnmount` 断开）与 `prefers-reduced-motion` → 立即滚动；`preventScroll:true` 保留，全程不调用 `scrollIntoView`、不使用元素相对偏移属性。
  - 外层容器让出滚动：`settings-form.css` 的 `.settings-tabs` 只保留宽度与裁切，删除 `SettingsView.vue` 内重复定义的那一份。
  - 空列表：`PostList`、`PageList`、`AttachmentList` 表头复选框改为 `v-if="…length > 0"`（`<th>` 占位保留）；五类列表在数据落定后剔除不在当前列表中的选择（加载中沿用上一批数据，不伪造空态）。
  - 为让「批量浮条存在性」可被真实断言，给 `VFloatingDock` 根元素补了稳定的 `v-floating-dock` 类名（原测试的 `.v-floating-dock` 断言此前是**空断言**，该选择器并不存在）。
- **涉及文件**：`packages/components/src/ui/VTabbar.vue`、`packages/components/src/ui/VFloatingDock.vue`、`modules/console/web/src/modules/settings/settings-form.css`、`modules/console/web/src/modules/settings/SettingsView.vue`、五类列表组件、`modules/console/tests/components-a11y.test.ts`、`tests/e2e/console-ui-consistency.spec.ts`。
- **受控模拟场景（390px 真实溢出页面）** —— 8 项用例，全部通过：
  1. 系统设置 Tabs：外层 `overflow-x` 为 `hidden` 且自身不产生滚动，内部根元素 `overflow-x:auto` 且 `scrollWidth > clientWidth`；切到尾标签后活动标签矩形完整落在滚动区域内且 `scrollLeft > 0`；切回首标签后回到 `scrollLeft = 0`；全过程 `window.scrollY` 与 `main.content.scrollTop` 与基准完全一致。
  2. 主题设置 Tabs：同上。
  3. query 直接命中末尾标签：初始加载即完成显露；Home/End 切换后选中项、`aria-selected`、关联 `tabpanel` 可见性与焦点位置一致。
  4. 五类列表默认视图空列表：空态提示出现，表头复选框 / 全选 / 取消全选 / 批量浮条计数均为 0。
  5. 四个可切表格视图的列表（文章/页面/附件/友链）空列表：同样无任何选择入口。
  6. 卡片模式「先选择再筛选为空」：浮条消失，恢复列表后旧选择不复活。
  7. 表格模式「先选择再筛选为空」（文章）：表头选择入口与浮条一并消失。
  8. 媒体库表格模式「先选择再筛选为空」（真实上传接口夹具）：同上。
- **结构性证据（源码检查，不是行为证据）**：`components-a11y.test.ts` 新增 VTabbar 修饰类/rect+scrollLeft/clamp/ResizeObserver/reduced-motion/禁用 `scrollIntoView` 与相对偏移属性的断言、外层容器不再承担滚动的断言、五类列表「清理失效选择 + 空列表不渲染禁用态入口」的断言。
- **诚实边界**：文章/页面/附件在列表为空时渲染的是 `VEmpty` 分支（整张表格不渲染），因此表头复选框的 `v-if` 改造属于**防御性清理**，其可观察价值在于「源码不再保留空列表禁用态入口」，行为验收由上述空态用例与失效选择清理用例承担。
- **实际命令与结果（本轮实测）**：`make console-test`（211 项）→ 通过；`make test` → 通过；`make build` → 通过；`pnpm exec playwright test tests/e2e/console-ui-consistency.spec.ts tests/e2e/console-settings-save-race.spec.ts` → 42 passed；pre-commit hook 与 `make merge-check` 通过。
- **视觉变更需人工确认**：`autoReveal` 模式固定 `width:100%`，分段 Tabs 胶囊在桌面端现在铺满卡片宽度（标签仍左对齐、可横向滚动）。候选截图入口：`.tmp/r4-t3-screens/tabs-appearance-1280.png`、`.tmp/r4-t3-screens/tabs-settings-390.png`（由用例「候选验收截图」以 `testInfo.outputPath` 生成后人工复制到该目录；`.tmp/` 为忽略目录）。受跟踪基线 `docs/baselines/console-ui-consistency/t3-appearance-desktop.png` 仍是**改造前**的画面，需人工核对后再决定是否替换为新的验收图。

### T1：手机操作矩阵与真实可访问性（待体验）

- **改动原因**：390px 末项验收此前用固定 `waitForTimeout(300)` 充当完成证据、靠 `.last()` + 硬编码 `mobile-e2e-post-5` 猜测排序；页面/附件没有对应用例；键盘只扫描 CSS 声明；无 hover/active 计算对比度；UC 只检查登录页 `data-app`；5 处自动截图直接写入受跟踪基线目录。
- **改动内容（实现）**：
  - 危险按钮前景由白字改为 `var(--color-primary-foreground)`（黑字）：默认 6.16:1、active 约 4.6:1，满足普通文字 ≥4.5:1。
  - `apple-base.css` 移动端触控规则补入 `.action-icon-btn`（页面列表的独立图标按钮原为 28px，实测低于 44×44 门槛）。
  - `AttachmentPickerModal.vue`（共享弹层）补焦点管理：打开时记住触发元素并把焦点移入弹层、挂窗口级 Esc 监听，关闭时移除监听并把焦点还给触发元素（此前 `@keydown.esc` 挂在弹层元素上，焦点未进入时 Esc 无效）。
  - `scripts/e2e/prepare.go` 追加非管理员夹具账号 `e2e-author`（角色 `author`）。
- **改动内容（验收）**：
  - 新建 `tests/e2e/console-mobile-a11y.spec.ts`（8 项）：三页四视口矩阵；页面与媒体库的 390px 末项真实点击；真实 Tab/焦点环/方向键/Home/End 联动面板；主操作与危险操作三态计算对比度；六页触控目标；五页概览栅格与最小高度；UC 隔离账号资料保存与通知页、共享弹层焦点往返。
  - `console-ui-consistency.spec.ts`：5 处截图改为 `testInfo.outputPath(...)`；390px 末项用例重写为「夹具 token 过滤 + DOM 末项 + 标题映射 slug + 轮询等待滚动到底 + 硬断言矩形」，删除固定 sleep 与硬编码 slug。
  - 新验收文件已写入实际运行命令（见下），不以运行旧文件代替。
- **真实服务场景（隔离 e2e 服务）**：全部用例在 `scripts/e2e/run-server.sh` 的隔离根目录（数据库/配置/内容/附件/运行时数据）上运行，服务每次重新构建 Console 后重启。
- **关键实测结果**：
  - 四视口矩阵：390/768 三行且底栏显示（768 底栏隐藏、桌面侧栏显示）、1280/1440 页头横排且 `scrollWidth - clientWidth ≤ 1`。
  - 390px 末项：真实滚动到底（轮询 `scrollTop ≥ scrollHeight - clientHeight - 1`），末项操作矩形非空、`y ≥ 0`、底边不越过底栏顶边，真实点击后进入对应夹具 slug 的编辑页 / 打开对应附件详情。
  - 对比度：主操作与危险操作的默认/hover/active 三态实际计算色对比度全部 ≥4.5:1（在页面内用 canvas 解析 `color-mix`/`oklab` 为 sRGB 后计算）。
  - 触控：文章/页面/媒体库/评论/友链/系统设置六页的必选控件均 ≥44px，独立图标目标 ≥44×44（元素缺失即失败，不做条件跳过）。
  - 概览：文章/页面/媒体库/用户/备份五页手机 2 列（≥88px）、桌面 4 列（≥80px）。
  - UC：`e2e-author` 登录 → 资料页改名保存 → 刷新回读一致；通知页分类 tablist、刷新操作、共享按钮胶囊圆角；共享弹层打开后焦点进入弹层、Esc 关闭且焦点返回触发元素。
- **实际命令与结果（本轮实测）**：
  - `make hooks-install`、`make console-test`、`make test`、`make build` 全部通过。
  - `pnpm exec playwright test tests/e2e/console-mobile-a11y.spec.ts tests/e2e/console-ui-consistency.spec.ts tests/e2e/console-settings-save-race.spec.ts` → 51 passed。
  - 运行前后 `git status --short docs/baselines/` 输出为空（受跟踪基线零变化）。
  - pre-commit hook 与 `make merge-check` 通过。

---

## 9. Round 4 证据分类、截图入口与未验证项

### 证据分类

- **源码检查（结构断言，不是行为证据）**：`modules/console/tests/components-a11y.test.ts`（VTabbar 滚动契约、外层容器归属、五类列表失效选择与空态入口）、`modules/console/tests/settings-save-race.test.ts`（保存结构）、`modules/console/tests/settings-workflow.test.ts`（ThemeYamlTab 接线与本地保存状态）。
- **历史测试（沿用，未作为本轮验收证据）**：`console-ui-consistency.spec.ts` 中 Round 2/3 的布局、按钮、Tabs、空态、SEO、ConsoleStatsCard 用例。
- **本轮实测（受控模拟）**：`console-settings-save-race.spec.ts` 12 项拦截式用例；`console-mobile-a11y.spec.ts` 与 `console-ui-consistency.spec.ts` 中依赖夹具与视口的用例。
- **本轮实测（真实服务）**：竞态用例 13（不拦截响应、刷新回读）；UC 隔离账号资料保存与通知页；媒体库真实上传接口夹具；全部 E2E 的隔离根目录服务。

### 截图入口

- 候选验收图（忽略目录，未入库）：`.tmp/r4-t3-screens/tabs-settings-390.png`、`.tmp/r4-t3-screens/tabs-appearance-1280.png`。
- 自动截图统一改由 `testInfo.outputPath(...)` 产出，落在 `.tmp/playwright-test-results/`；**测试执行前后受跟踪基线目录零变化**。
- **已被最新归档取代（已闭环）**：`docs/baselines/console-ui-consistency/t3-appearance-desktop.png` 已在收尾阶段以满宽分段 Tabs 重新生成并归档入库，基线已对齐最新设计规范，不再有待决定项。

### 产物加载证据（不以 HTTP 200 代替）

在 T1 工作树（HEAD `d60b3ae`）用一次性 Playwright 用例记录隔离服务实际投放并被浏览器真实加载的产物：

- 服务启动时间：`2026/09/14 14:32:01`；启动日志明确输出 `console SPA loaded from .tmp/e2e-server.zcLHeg/modules/console/web/dist (dev)`（每次 E2E 运行都会先重建 Console 再重启 Go 服务）。
- 文档实际引用（服务返回的入口 HTML）：`/console/assets/index-DV1GiYzV.js`、`/console/assets/index-D0XV7o7C.css`。
- 浏览器真实加载：上述入口文件 + 懒加载分块 `runtime-core.esm-bundler-DFn6PQEW.js`、`api-ZU2VHYOh.js`、`pinia-BDaR9qFm.js`、`preload-helper-l0U7FUPx.js`、`Login-CsFiuZau.js`、`Login-psTCZFVP.css`。
- 结论：入口 HTML 引用的 hash 与浏览器实际拉取的资源一致，证明验收针对的是当前构建产物，而不是缓存或旧产物。

### 未验证项

- 未在真实移动设备/真机浏览器上验证，仅在 Chromium 视口模拟下测量 390/768/1280/1440。
- 未验证 Safari/Firefox 上的滚动与 `color-mix`/`oklab` 解析差异；对比度计算依赖 Chromium 的 canvas 色彩解析。
- 未验证弱网/慢请求下的保存按钮与加载态表现（拦截式测试是即时释放，不代表真实网络时延分布）。
- 未验证深色以外的显示环境（产品只交付深色模式）。
- 本轮未推送、未发布、未部署、未清理任何 worktree/分支。

---

## 10. 收尾任务实施与验证记录 (Final Cleanup)

基于 `dev / cc63474` 执行收尾修复，统一概览列数、消除设置页内层嵌套卡片并更新基线截图归档。

### 1. 改动要点与执行结果

1. **T3 概览栅格列数统一**：
   - 将文章（`PostList.vue`）、页面（`PageList.vue`）、媒体库（`AttachmentList.vue`）的概览卡片栅格由 `sm:grid-cols-4` 改为 `sm:gap-4 lg:grid-cols-4`，与用户管理、备份导出完全一致。
   - 严格固定规则：宽度 `<1024px` 为两列，`≥1024px` 为四列；业务列表卡片与附件缩略图栅格原样保留。
2. **T3 设置页表单单卡片层级**：
   - 消除系统设置中 `BasicSettings`、`PostSettings`、`CommentSettings`、`SeoSettings`、`SearchSettings`、`AdvancedSettings` 六个子表单中 `.settings-section-body` 重复包裹的卡片背景、1px 边框、圆角及内边距。
   - `.settings-section-body` 统一调整为 `background: transparent; border: none; border-radius: 0; box-shadow: none; padding: 0;`。
   - 手机端水平 16px 边距统一由外层 `.settings-section` / `.settings-basic-section` 提供，消除了内外双重内边距叠加。
   - 保留代码编辑器、警告条、输入框以及图片预览控件的必要边界与间距。
3. **T1 扩展针对性验收用例**：
   - 在 `tests/e2e/console-mobile-a11y.spec.ts` 中增强概览列数断言：守住 `390px`、`639px`、`640px`、`768px`、`1023px`（两列）与 `1024px`、`1280px`（四列）断点。
   - 增加设置页单卡片层级断言：验证在 `390px`、`768px`、`1280px` 下 `.settings-section-body` 无多余背景、无可见边框、无阴影、无圆角且无横向溢出。
   - 运行完整 E2E 测试集（`console-ui-consistency.spec.ts`、`console-settings-save-race.spec.ts`、`console-mobile-a11y.spec.ts`）共 52 项用例，全部通过。
4. **基线截图更新与归档**：
   - 桌面主题设置验收图使用当前最新代码重新生成，更新归档至 `docs/baselines/console-ui-consistency/t3-appearance-desktop.png`（反映满宽分段 Tabs 真实画面）。
   - 新增手机端高级设置截图归档：`docs/baselines/console-ui-consistency/t1-settings-advanced-390.png`（验证警告框与代码输入框清晰可辨，无多层卡片包裹）。
   - 新增 768px 平板文章概览截图归档：`docs/baselines/console-ui-consistency/t1-posts-overview-768.png`（验证平板视口下两列概览布局）。
   - 唯一设计参照 `design-reference.png` 保持原样不被替换。

### 2. 实际验证命令与结果（本轮实测）

- `make console-test`：211 项测试通过。
- `make test`：包含 vet、staticcheck、资源检查与单元测试全部通过。
- `make build`：Console (含 `/console/` 与 `/uc/`) 前端构建成功，Go 二进制编译成功。
- Playwright E2E（3 套 spec）：
  ```bash
  pnpm exec playwright test tests/e2e/console-ui-consistency.spec.ts tests/e2e/console-settings-save-race.spec.ts tests/e2e/console-mobile-a11y.spec.ts
  ```
  结果：**52 passed**（耗时约 1.2m）。
- 产物加载与服务状态：
  - 临时测试服务器以 `-dev` 启动，加载当前重新构建的 SPA 产物（资源 hash 与文档引用严格对齐）。
  - 测试完成后生成的目标截图由人工复核后从 `.tmp/playwright-test-results/` 复制归档到 `docs/baselines/console-ui-consistency/`。

---

## 11. 最终补齐实施与验收交付记录 (Final Polish)

在独立 worktree `../Blog-r4-final`（分支 `codex/console-r4-final`，基于 `ed20839`）完成剩余三项补齐，保留已完成的设计、保存和 Tabs 逻辑，不改动 API、不新增功能、不推送或部署。

### 1. 改动要点与执行结果

1. **T3：补齐手机设置页边距**：
   - 将表单分区左右 16px 的适用范围统一规范为 `<768px`（采用 `@media (max-width: 767px)`），彻底修复 641–767px 回退到 28px 的缺陷。
   - 严格落实“只调整水平边距，不改变其他布局和控件”：各设置 tab 的垂直边距（`padding-top/bottom`）、网格布局（`.settings-form-grid-*`）、开关控件（`.settings-switch`）以及底部操作栏的样式保持原样不被移动。
   - 涉及文件：
     - `modules/console/web/src/modules/settings/settings-form.css`
     - `modules/console/web/src/modules/settings/tabs/BasicSettings.vue`
     - `modules/console/web/src/modules/settings/tabs/PostSettings.vue`
     - `modules/console/web/src/modules/settings/tabs/CommentSettings.vue`
     - `modules/console/web/src/modules/settings/tabs/AdvancedSettings.vue`
     - `modules/console/web/src/modules/settings/tabs/SearchSettings.vue`
     - `modules/console/web/src/modules/settings/tabs/SeoSettings.vue`
2. **T1：补强对应测试**：
   - 在 `tests/e2e/console-mobile-a11y.spec.ts` 中增加 `640px`、`641px`、`767px`、`768px` 关键断点矩阵检查（覆盖 `390/640/641/767/768/1280`）。
   - 遍历实际表单容器（`.settings-basic-section`、`.settings-section`、`.settings-post-section`），断言 `<768px` 视口下水平 padding 严格为 `16px`，`≥768px` 下为 `28px`。
   - 针对内层表单容器 `.settings-section-body`，遍历每一个匹配容器明确断言：背景透明（`transparent` / `rgba(0, 0, 0, 0)`）、无重复边框（`none` / `0px`）、无阴影、无额外圆角且左右内边距为 `0px`。
   - 应存在的容器缺失必须直接抛错失败，杜绝仅断言首项或 `if (count > 0)` 静默跳过零匹配。
   - 明确断言保留警告框（`.settings-warning`）和输入框（`.apple-input` / `.code-editor-wrapper`）的必要边框边界与间距，防止误伤。
   - 修正测试头部关于概览旧断点的注释，消除与统一规范（`<1024px` 两列，`≥1024px` 四列）的冲突。
3. **T1／T3：整理交付记录**：
   - 修正旧断点注释（包含测试用例与历史规划日志），明确历史记录已被统一规范取代。
   - 将文档中“截图待决定”标注为已被最新归档取代（`t3-appearance-desktop.png` 已闭环归档入库）。
   - 详尽记录本次真实服务启动时间、具体资源 hash 以及截图对应的代码版本。

### 2. 真实产物加载与服务启动证据（不以 HTTP 200 代替）

在独立测试工作树 `../Blog-r4-final` 中，使用隔离服务（`-dev` 模式）与 Playwright 测量真实浏览器加载行为：

- **服务启动时间**：`2026/09/14 23:25:42`
- **服务启动日志**：
  ```
  console SPA loaded from /Users/sakurasep/Documents/Code/Project/Personal Project/Blog-r4-final/.tmp/e2e-server.JQD2wY/modules/console/web/dist (dev)
  user center SPA loaded from /Users/sakurasep/Documents/Code/Project/Personal Project/Blog-r4-final/.tmp/e2e-server.JQD2wY/modules/console/web/dist-uc (dev)
  ```
- **文档实际引用（服务返回的入口 HTML `dist/index.html`）**：
  - 入口脚本：`/console/assets/index-xoZIK908.js`（`make build` 产物为 `/console/assets/index-Mr6rN0V7.js`）
  - 入口样式：`/console/assets/index-D0XV7o7C.css`
  - 表单样式分块：`/console/assets/settings-form-C8cWjgsO.css`
  - 预加载分块：
    - `/console/assets/api-ZU2VHYOh.js`
    - `/console/assets/runtime-core.esm-bundler-DFn6PQEW.js`
    - `/console/assets/pinia-BDaR9qFm.js`
    - `/console/assets/preload-helper-l0U7FUPx.js`
- **User Center 入口（`dist-uc/uc.html`）**：
  - 入口脚本：`/uc/assets/index-CA90lmEM.js`
  - 入口样式：`/uc/assets/index-CFdsD7Up.css`
- **截图对应代码版本**：
  - Git 分支与版本：`codex/console-r4-final` (基于最新 `dev / ed20839`)
  - 自动截图落在 `.tmp/playwright-test-results/`，受跟踪基线目录 `docs/baselines/console-ui-consistency/` 在测试运行前后严格零变化。

### 3. 门禁验证与交付状态

- `make console-test`：**212 passed**（耗时 1.7s，通过率 100%）。
- Playwright E2E（3 套 spec，共 52 项用例）：**52 passed**（耗时 1.1m）。
- `make test`：全套门禁（Staticcheck、vet、部署检查、Go 单元测试）全部通过。
- `make build`：Console 与 User Center 双前端构建成功，Go 二进制编译成功。
- **当前交付状态**：**待体验**（必要验证全部通过，等待用户体验确认后方可标记为“已验收”）。

---

## 12. 验收收尾：四设置标签覆盖与计划记录同步 (Round 4 Closeout)

起点：`dev / 906fd22`。本轮**只补测试覆盖与同步文档，无任何产品代码改动**，不扩大 UI 修复范围、不改 API、不新增功能。工作树 `../Blog-ui-closeout`（`codex/console-ui-closeout`，基于 `906fd22`）；根目录未暂存的 `.gitignore`（新增 `.workbuddy/`）原样保留，未提交、未还原。

### 1. 改动要点

1. **T1：补齐系统设置四个标签的矩阵覆盖**（`tests/e2e/console-mobile-a11y.spec.ts`）：
   - 设置页矩阵由 5 项扩为 9 项，新增 `/console/settings?tab=post`、`?tab=comments`、`?tab=seo`、`?tab=search`，四项均 `hasBodies: true`。
   - 复用既有断言、未降低标准：`390/640/641/767/768/1280px` 六视口；表单分区容器**逐个遍历**（零匹配直接失败，不允许静默跳过）；`<768px` 左右内边距严格 16px、`≥768px` 严格 28px；`.settings-section-body` 背景透明、无可见边框、无阴影、无额外圆角且左右内边距 0；页面水平溢出 ≤1px；警告框与输入框的必要边界保留。
   - 新增一条前置守卫：对 `/console/settings?tab=<key>` 断言 `#tabpanel-<key>` 可见。原矩阵只验证「页面能打开 + 分区样式正确」，若 `tab` query 失效退回 `basic` 仍会全绿；该守卫使四个新标签的覆盖真正落在各自表单上，而不是空转。
2. **T1／T3：同步唯一计划记录**：
   - 第 7 节 Round 4「最终补齐」的合并提交由「待合并」更正为实际提交 `906fd22`。
   - 追加本节，记录本次新增覆盖、实际命令与结果、负向对照，以及哪些检查本次重新运行、哪些沿用历史证据。
   - 第 1–11 节的历史证据全部保留、未改写；状态维持 **待体验**，不代用户标记「已验收」。

### 2. 实际命令与结果（本轮实测）

- 针对性用例（隔离服务：Playwright webServer 先 `VITE_E2E=1 pnpm --filter console run build` 重建 Console，再以 `-dev` 启动独立根目录的 Go 服务）：
  ```bash
  pnpm exec playwright test tests/e2e/console-mobile-a11y.spec.ts --grep "设置页单卡片层级与边距"
  ```
  结果：**1 passed**（8.7s），覆盖 9 个设置标签 × 6 个视口（54 组页面断言），四个新增标签全部通过。
- **负向对照（证明新增覆盖不是空转）**：将 `?tab=search` 临时改为不存在的 `?tab=searches` 后重跑同一命令，断言在 `#tabpanel-searches` 处 **1 failed**；随后已还原为 `?tab=search`，失败产物目录 `.tmp/playwright-test-results/` 已清理。
- 本轮真实验证的隔离服务产物加载（不以 HTTP 200 代替）：
  - 服务启动时间：`2026/09/15 00:32:14`
  - 启动日志：`console SPA loaded from .../Blog-ui-closeout/.tmp/e2e-server.Q94VLy/modules/console/web/dist (dev)`、`user center SPA loaded from .../dist-uc (dev)`
  - 每次 E2E 运行都会重建 Console 并重启 Go 服务，因此断言针对当前构建产物，而非缓存或旧产物。

### 3. 本轮重新运行 / 未重新运行的检查

- **本次重新运行**：针对性 Playwright 用例（含上述负向对照）；交付门禁 `make merge-check`（先 `make test`：`console-test` 类型检查与 `node:test`、`assets-test`、`deploy-test`、`release-source-test`、`module-check`、`go vet`、`staticcheck`、`go test ./...`，通过后合入根目录 `dev`，并在合并后的 `dev` 复跑一次 `make test`）。
- **本次未重新运行**（本轮无产品/样式代码改动，仅测试与文档变更）：完整三套 E2E spec 的 52 项用例、`make build`。第 10／11 节记录的这两项结果作为历史证据保留，未被本轮结果取代或改写。
- **边界**：仍仅在 Chromium 视口模拟下验证，未在真机/多浏览器验证；本轮未推送、未发布、未部署、未清理任何 worktree 或分支。

### 4. 交付状态

- 完成标准：四个新增标签通过全部视口断言、文档与 Git 状态一致 —— 已满足。
- 状态：**待体验**（等待用户体验确认后方可标记「已验收」；无论断标准未降低）。

