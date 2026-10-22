# Console UI 去“AI 味”重做（朴素工具型）实施方案

本文档记录 Console 管理后台（`/console/`）与用户中心（`/uc/`）重构为符合 `resource/apple-design` 规范的“朴素工具型”深色后台的系统实施计划、视觉裁决与验收追踪。

- **目标分支**：`codex/console-tool-style`（工作树 `../Blog-console-tool-style`）
- **合并目标**：`dev`
- **唯一设计参照基准**：`docs/baselines/console-ui-tool-style/design-reference.png`
- **基线说明文档**：`docs/baselines/console-ui-tool-style/README.md`
- **核心定位**：去除毛玻璃、渐变光晕、过度装饰和营销文案；实现实色面板、紧凑行距、单色相强调与克制事实文案，建立长期机械门禁守卫。

---

## 1. 核心视觉与架构裁决

### 1.1 材质与阴影（消除浮华装饰）
1. **背景与面板实色化**：
   - 全局背景：`#000000` 实色（`--color-background`）。
   - 卡片与面板：`#1c1c1e` 实色（`--color-card`），消除常驻面板上的 `backdrop-filter: blur(...)` / `backdrop-blur-*`。
   - 边框：`1px solid #3a3a3c`（`--color-border`）。
   - 消除所有面板顶部的 1px 渐变高光线（`.card-wrapper::before`、`.stats-card::before`、`.apple-card-highlight::before` 等）。
2. **阴影分层**：
   - 常驻卡片、表格、列表项、输入框：`box-shadow: none`。
   - 阴影仅保留在真正的浮层控件（下拉菜单、浮动操作坞 `VFloatingDock`、模态弹窗 `VModal` / `Dialog`、Toast 提示、移动端抽屉）。
3. **动效克制**：
   - 消除 `.hover-lift` 常驻浮动位移（`translateY(-2px)`）与发光投影。
   - 消除 `transition: all`，替换为具名的具体属性过渡（`color`, `background-color`, `border-color`, `opacity`, `transform`）。
   - 禁用 `animate-pulse` 等呼吸闪烁动效，骨架屏使用极简平滑的背景色过渡。

### 1.2 圆角与尺寸阶梯（紧凑结构）
1. **圆角阶梯化**（`main.css` `@theme`）：
   - `sm`: 8px（用于输入框 `apple-input`、下拉框、小徽标、内部小控件）
   - `md`: 10px（用于常规卡片 `VCard`、面板、设置分组区块）
   - `lg`: 12px（用于次级复合容器、大卡片）
   - `xl`: 14px（用于模态弹窗 `VModal`、全局抽屉）
   - `2xl`: 16px（最大外层弹层容器）
   - `full`: 9999px（用于按钮 `VButton`、开关 `VSwitch`、胶囊搜索框、头像）
2. **排版密度与文字层级**：
   - 表格与列表项：高密度行式展现（基准行高 ~44px），操作按钮常显，支持快速扫描。
   - 统计 KPI：数值采用 `24px`、字重 `600`、`tabular-nums` 等宽对齐，消除花哨的大写英文标签与装饰性副标。
   - 文本底线：彻底消除 `<12px` 的微小无意义装饰文本（统一提升至 `12px` 或 `13px`）。
   - 文案克制：删除营销式欢迎语（“欢迎回来”、“探索全新功能”等），改为紧凑的事实状态（“系统运行正常”、“共 12 篇文章”）；删除纯装饰性的 emoji 与 uppercase 英文眉标。

### 1.3 外壳与导航
1. **侧边栏（Sidebar）**：
   - 消除毛玻璃滤镜与半透明底色，改为 `#1c1c1e` 实色背景与 1px 右边框 `#3a3a3c`。
   - 精简站点图标至 24px，消除 3px 悬浮条与发光背景。
   - 移除侧栏底部或页脚的营销署名（如 "Powered by ..."）。
2. **命令面板（⌘K CommandDialog）**：
   - 消除渐变光晕与大卡片装饰，改为高密度纯列表布局，维持原有搜索、快捷键与无障碍键盘导航。
3. **User Center（/uc/）**：
   - 对齐相同的实色背景、10px 卡片与 8px 控件，消除浮华的渐变与毛玻璃。

---

## 2. 机械门禁：ui-restraint.test.ts 守卫规范

在 `modules/console/tests/ui-restraint.test.ts` 中建立自动化静态扫描测试：
1. 扫描范围：`modules/console/web/src` 与 `packages/components/src` 中的源码文件。
2. 边界隔离：白名单登记编辑器域保留文件（`PostEditor.vue`, `PageEditor.vue`, `EditorToolbar.vue`, `EditorSidebar.vue`, `PageEditorSidebar.vue`, `AiGenerateModal.vue`, `MarkdownEditor.vue`, `RichTextEditor.vue`, `packages/editor/*`）。
3. 严格规则检查：
   - 禁止未登记的 `backdrop-filter` / `backdrop-blur`
   - 禁止未登记的渐变（`linear-gradient`, `radial-gradient`, SVG `<linearGradient>`, `<radialGradient>`）
   - 禁止 `transition: all`
   - 禁止 `animate-pulse`
   - 禁止纯装饰性 `uppercase`
   - 禁止非语义调色板的越界裸 hex 颜色
   - 禁止纯装饰性 emoji
   - 禁止 `.hover-lift`
   - 禁止 0 偏移发光（`0 0 ...px` 辉光）与染色投影
4. 所有功能性例外必须在白名单对象中逐条登记原因，任何新增装饰性样式均会被 CI/make console-test 机械拦截。

---

## 3. 分阶段实施路线

### 阶段一：建立机械门禁与文档准备
- 编写实施方案文档与基线 README。
- 拍摄 4 个视口（390/768/1280/1440）下的全套 Before 基线截图。
- 编写 `modules/console/tests/ui-restraint.test.ts`，定义检查规则与白名单模型。

### 阶段二：设计令牌与公共原语收紧
- 重构 `main.css`：收紧 `@theme` 圆角阶梯（sm 8 / md 10 / lg 12 / xl 14 / 2xl 16），侧栏实色化，移除 `tailwindcss-animated`。
- 重构 `custom-theme.css`：移除 `.hover-lift`，骨架屏去渐变。
- 重构 `apple-base.css`：移除高光渐变线、无意义 11px 大写标签，规范高密度卡片与控件。
- 重构 `packages/components/src/ui/`：
  - `VCard.vue`：实色卡片、md 10px 圆角、无阴影、无顶部渐变线。
  - `VButton.vue`：消除 `transition: all`，保持胶囊圆角、36px 高度与黑字高对比。
  - `VModal.vue`：实色背景、xl 14px 圆角、消除 `transition: all`。
  - `VFloatingDock.vue`, `FilterDropdown.vue`, `VSkeleton.vue` 等控件规范化。

### 阶段三：外壳导航与业务模块重构
- 外壳重构：`DefaultLayout.vue`（侧栏实色化、精简 24px logo、移除署名、抽屉实色化、⌘K 命令面板列表紧凑化）。
- 仪表盘重构：`DashboardView.vue`、`ConsoleStatsCard.vue`、`dashboard.css` 及组件（KPI 规范为 24px/600 `tabular-nums`，消除毛玻璃与染色大底）。
- 内容模块重构：`PostList.vue`、`PageList.vue`、`AttachmentList.vue`、`AttachmentField.vue`（高密度行式列表，操作常显）。
- 设置中心重构：`SettingsView.vue` 及各 Tabs（消除多层重复嵌套边框，中性实色表单分区）。
- 用户中心重构：`UCLayout.vue`、`UCPostsView.vue`、`UCAttachmentsView.vue`、`UCProfileView.vue`、`UCLoginView.vue` 等。

### 阶段四：竖切样张确认与 After 截图
- 在 8098 端口拉起构建后服务，通过 Playwright 拍摄全套 After 对比截图。
- 生成唯一设计基准参照图 `design-reference.png`（包含桌面列表+卡片、手机操作区+底栏、设置分区+Tabs、弹窗+控件阶梯、仪表盘）。
- 核对视觉语言、对比度（≥4.5:1 AA）与触控边界。

### 阶段五：门禁与集成验收
- `make console-test`（含 `ui-restraint.test.ts` 源码扫描全绿）。
- `make test`（含 `assets-test` 图标双向校验、`module-check`、Go 测试全绿）。
- `make build`（Console SPA 与 Go 二进制打包全绿）。
- `BLOG_E2E_SCREENSHOT_DIR=/tmp/e2e-ss pnpm run e2e`（现有 E2E 测试全部通过）。
- `make merge-check` 验证并合入 `dev` 分支。
