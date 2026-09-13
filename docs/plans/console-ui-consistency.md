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
| **T2** | 基础按钮统一尺寸与胶囊形状、主按钮黑字高对比、Tab 焦点环清晰、主题设置黄点误报消除 | `../Blog-t2-controls` (`codex/console-t2-controls`) | 待体验 |
| **T3** | 列表概览卡片统一样式与高度、系统设置与主题设置统一分段 Tabs 并优化多层卡片、修正空列表全选文案 | `../Blog-t3-views` (`codex/console-t3-views`) | 未开始 |

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

### T2：公共控件统一，修改状态真实（待体验）
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
  - [x] 执行 `make merge-check` 将 `codex/console-t2-controls` 稳妥合入 `dev` 分支。当前状态标记为 **待体验**。
