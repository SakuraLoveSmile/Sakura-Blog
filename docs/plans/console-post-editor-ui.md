# Console 文章编辑页 UI 优化（BLG-20261102-06）

- 状态：待体验（实现与本地组合验证完成；用户尚未确认验收）
- 日期：2026-11-02（Asia/Taipei）
- 分支：`codex/refactor/20261102-blg-20261102-06-console-post-editor-ui`
- worktree：`/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261102-refactor-blg-20261102-06-console-post-editor-ui/`
- 代码版本：从 `dev d1e34a97` 创建；合入目标为根目录 `dev`
- 视觉参照：`docs/baselines/console-post-editor-ui/reference/article-editor-ui-preview-20261102-a1.html`
- 证据：`docs/baselines/console-post-editor-ui/`

## 目标与范围

按用户确认的「内容优先 · 少卡片 · 单色强调」预览，重组文章编辑页的页头、写作面、工具栏和设置面板层级。只改 UI 结构与样式，不改变文章数据流、编辑器能力、设置字段或全局组件。

明确不做：全局 `main.css`、`VButton`、`MarkdownEditor`、`RichTextEditor`、`StellarContentConfig` 的源码修改；PageEditor/UC 重设计；Stellar 前台、API、数据库、路由、权限、文章业务逻辑；移动底部导航、设置抽屉、任何设置字段或高级能力删除。

## 已确认决定

1. 保存状态并入页头，页面只保留一个 `role="status" aria-live="polite"`；独立 `.editor-status-bar` 删除。
2. 标题、工具栏、正文形成一个开放写作面：标题与编辑器外层不再有卡片背景/外框/圆角；工具栏只保留上下 hairline。编辑器组件实例通过文章页局部 class 去掉 `bg-card` 与圆角，不改共享组件。
3. `EditorToolbar` 在横向滚动区之后新增 opt-in `#end`；文章页把类型菜单、桌面侧栏开关、移动「文章设置」入口放进 `#end`。未提供 `#end` 时保留原侧栏按钮，PageEditor 的 DOM 与事件不变。
4. 类型菜单保留全部 ID、ref、ARIA、键盘、焦点恢复与切换前 flush；视觉改为语义 token 的单色菜单，不再使用彩色图标底、玻璃背景和装饰箭头。
5. 右侧 Tabs 改为无卡片下划线切换；设置分组、主题 details、配置原文改为细下边线与留白。输入、作者/专栏 popover、分类列表、chip、开关、封面媒体、插件 iframe 的功能边界保留。
6. 移动端发布继续使用 `VButton type="primary" size="md"`；页头与工具栏直接控件至少 44×44，移动抽屉内直接控件也保证 44px 高。
7. 参考预览归档时只修正移动发布按钮的级联冲突（`.mobile-actions .mobile-publish`），其他设计未改。

## 实现摘要

- `modules/console/web/src/modules/posts/EditorToolbar.vue`：新增 `useSlots()`、`#end` 容器；原侧栏开关仅在无 `#end` 时渲染。
- `modules/console/web/src/modules/posts/PostEditor.vue`：保存状态移入 `.editor-header`；类型菜单/侧栏开关/移动设置入口迁入 `#end`；删除 `.editor-status-bar`；新增 `.post-writing-surface`、`.post-toolbar-shell`、`.post-editor-body`、`.post-editor-instance`；类型菜单样式改用语义 token；移动 44px 与无溢出约束；桌面侧栏面板增加 `#post-settings-panel`。
- `modules/console/web/src/modules/posts/EditorSidebar.vue`：分组常量改为 hairline 分组；Tabs 改为下划线；封面占位改中性底；`StellarContentConfig` 加文章侧栏专用 class 并局部去卡；移动抽屉控件 44px。
- `modules/console/web/src/components/editor/PostThemeSettings.vue`：主题 details 改为扁平分组，字段、事件、disabled 与原文写回不变。
- `tests/e2e/editor-writing-state.spec.ts`：状态定位改为 `.editor-save-status`；切换前等待选项渲染，保留正文同 tick flush 竞态。
- `modules/console/tests/page-editor-ui.test.ts`：新增插槽、单一 live region、PageEditor 不注入 `#end`、单层分组、编辑器实例去卡断言。
- `tests/e2e/console-post-editor-ui.spec.ts`：新增本次布局、断点、焦点、无溢出和移动发布按钮可读性验收。

## 验证结果

### 命令

| 命令 | 结果 |
| --- | --- |
| `make console-test` | 430 pass / 0 fail（最终代码） |
| `pnpm lint` | 通过 |
| `pnpm format:check` | 仓库级失败：既有 `modules/console/web/src/modules/settings/tabs/FeedbackSettings.vue` 解析错误，以及大量既有格式漂移；本次触碰文件单独 `prettier --check` 全部通过 |
| 定向 Playwright（计划列出的 10 个 spec） | 116 passed |
| `make test` | 通过（最终代码） |
| `make build` | 通过（最终代码） |
| `pnpm e2e` | 最终单轮 182 passed / 0 failed |

完整 E2E 的两次非最终失败均已定位为非本次改动的时序问题：一次是仪表盘 `.dashboard-quick-links` 未等待加载（单独复跑通过）；一次是既有测试在菜单渲染前同 tick 点击选项，已在 `editor-writing-state.spec.ts` 增加“选项可见”等待，仍保留正文 flush 竞态。

### 真实 Chromium 证据

独立端口 `127.0.0.1:8136`，独立 DB/config/content/uploads；使用普通生产构建，重建后重启服务。视口矩阵 390/639/640/768/1023/1024/1280/1440：

- 所有视口 `documentElement.scrollWidth == clientWidth`，无页面横向溢出；工具栏自身 `overflow-x: auto`。
- 每个视口 `.editor-status-bar` 数量 0，`.editor-header` 内 live region 数量 1。
- 类型菜单与桌面侧栏开关位于 `.editor-toolbar-end`，不在 `.editor-toolbar-scroll` 内；菜单 bounding box 在 1280 视口内。
- 768 设置面板位于写作面下方；1024 起位于右侧；390/639 使用设置抽屉。
- 390 抽屉：滚动到底 `scrollTop 2410 == max`，最后内容「配置原文（JSON）」bottom 779.7，底部导航 top 787，无遮挡；打开后焦点在关闭按钮，Tab 不逃逸，Escape/遮罩关闭后焦点返回触发器。
- 390 页头发布按钮：`backgroundColor rgb(232,232,236)`、`color rgb(0,0,0)`、13px/500、72×44、视口内，WCAG 对比度 17.19:1。
- 归档参考预览移动发布按钮：近白 `rgb(232,232,236)`、深字 `rgb(9,9,10)`，对比度 16.29:1，浅字浅底冲突已消失。
- 控制台 `console.error` / `pageerror` 均为 0。
- 服务端加载的 37 个 `/console/assets/*`（含入口、`PostEditor`、`RichTextEditor`、`MarkdownEditor` 与 UC）与 `modules/console/web/dist` 同名文件 SHA-256 全部一致；`/console/` HTML 哈希一致。

截图与 JSON：

- `before/before-1280.png`、`before/before-768.png`、`before/before-390*.png`、`before/before-metrics.json`
- `after/after-1280.png`、`after-768.png`、`after-390.png`、`after-639.png`、`after-640.png`、`after-1023.png`、`after-1024.png`、`after-1440.png`
- `after/after-390-header.png`、`after-390-drawer.png`、`after-390-drawer-bottom.png`
- `after/reference-mobile-390.png`、`after/after-metrics.json`

## 未验证

- Safari、Firefox、真实触屏与软键盘未执行；Chromium 视口模拟不等于真机。
- 未推送、未创建 PR、未发布镜像、未部署生产；远端 CI 未运行。
- 全仓格式化门禁仍受既有 `FeedbackSettings.vue` 解析错误阻塞，本任务未修改该文件。

## 后续

按用户授权提交并合入 `dev`；合并后基于 dev HEAD 的临时 detached worktree 复验，再清理本任务 worktree/分支。状态保持「待体验」，用户明确确认后才标「已验收」。
