# BLG-20261026-01：Console 内容工作台重构

- 状态：待体验；2026-10-26（Asia/Taipei）。实现与适用验证已完成，等待用户体验确认。本文件为唯一权威记录。
- 用户目标：全面消除 Console 的模板化 AI 观感，让管理内容、写作和设置成为视觉重心。
- 本次按管理后台 UI 理解“console 后端”，目前没有发现 Go API 重构的必要性。
- 基线：`dev e05aec52fe9cb724f063648ce3132b321702047c`。
- 主工作树：`/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261026-refactor-blg-20261026-01-console-workspace`。
- 分支：`codex/refactor/20261026-blg-20261026-01-console-workspace`；预期合入目标 `dev`，本轮无提交、合并、推送及发布授权。
- 根目录已有用户文档/生产验收成果，保持原状。本工作树同步当前 AGENTS.md 规则。
- 浏览器环境：`127.0.0.1:8132`，使用 `scripts/e2e/run-server.sh` 隔离根、配置、DB、内容和上传；与其他任务的 8130/8131 无关。

## 当前设计决定与范围

用户已要求全面重构，并于本轮明确选择“深色专业工具：紧凑侧栏、清晰列表、少卡片，接近 Linear / GitHub 的克制感”。方向已确认；最终实现仍需实测并由用户体验，不提前标为已验收。

唯一现状参照图：`docs/baselines/console-workspace/before-posts.png`，同批 `before-dashboard.png`、`before-settings.png` 是辅助现状记录。现状实测：黑色基底、灰色侧栏、四张统计卡、海报文章网格、问候与写作引导。以下差异是本次设计选择，而不是声称原设计库已规定。

- 沿用本地 `resource/apple-design` 的语义颜色、字体、焦点环、状态色和现有 Vue 组件。深色基底/面板令牌不换色，不增加组件库或依赖。
- 外壳：桌面侧栏从 256px 收到 224px，折叠仍 64px；品牌区靠左、写作按钮中性化，导航项 36px 高，激活项用中性底色，取消额外蓝色竖条；菜单仍来自 manifest 和权限过滤。移动底栏/更多菜单/搜索快捷键/通知/账号操作保持可达。
- 公共控件：普通工作按钮、搜索和分段控制使用现有 8px/10px 圆角，只有显式 pill、头像、开关保留胶囊；主按钮仍蓝底黑字。标题 20px/600，说明 13px，正文 14px。常驻内容无阴影、无上浮；浮层保留边界和必要阴影。
- 列表：文章与页面的初始视图改为行列表，保存的文章 grid 偏好继续有效，封面视图仍可切换；手机提供紧凑记录行，不把宽表强塞进视口。内容页的四块 KPI 改成紧凑、无图标底块的事实摘要；保留“本页”与“总计”的准确语义，绝不把当前页统计冒充全站统计。
- 控件顺序：页面标题与主操作 → 搜索/状态/筛选 → 结果与批量操作 → 分页。统计不得挤占主内容。媒体库保留图片网格。
- 仪表盘：改为“仪表盘”与日期，去除问候和独立写作宣传区，写作/草稿入口收进标题操作；保留全部部件、布局设计器、历史布局、响应式断点，不迁移已保存布局。统一卡片实色、10px 圆角、无悬浮位移，数字和图表保留功能颜色语义。
- 设置：保留单层分组表单，改为清晰的下划线分区导航，去掉多层胶囊；保留全部设置、query 定位、草稿、未保存提醒、保存竞态、预览。已有 16/28px 表单边距沿用。
- 全部 Console 路由纳入检查：内容（文章、页面、专栏、笔记本、Wiki、作者、瞬间）、互动（评论、评论用户、通知、友链、订阅）、媒体、外观/菜单、分类、用户/日志、插件、模块、设置、备份、更新、SEO、AI。共享样式影响的编辑器和 UC 也做回归；不改变编辑器内容、Markdown/shortcode、发布和权限语义。
- 各页空态/加载/错误保留事实与可执行操作，删去纯装饰性说明；危险动作的确认不可删。
- 窄屏：390px 无整页横向溢出、触控目标至少 44px、批量条及末项操作不被底栏遮挡；768px 侧栏仍可见；1280/1440px 列表可扫描。

## 实施与验收

| 步骤 | 用户效果与归属 | 验收 | 状态 |
|---|---|---|---|
| T1 | 主 agent：公共样式、组件、外壳、登录及集成 | 导航/搜索/折叠/手机抽屉与焦点可用，工作区更紧凑 | 待体验 |
| T2 | 内容列表专家：PostList.vue、PageList.vue | 默认列表、搜索筛选、已存偏好、编辑/批量/分享/回收站、手机行视图 | 待体验 |
| T3 | 工作台专家：dashboard 模块、SettingsView.vue | 去冗余引导、布局保留、设置切换与保存/预览 | 待体验 |
| T4 | 业务页专家：其他 stats-card 页面、attachments/相关列表局部结构 | 摘要收拢、统计筛选动作保留、媒体网格保留、复制样式移除 | 待体验 |
| T5 | 主 agent：最终组合态验证与修复 | make test、make build、现有受影响 E2E、真实页面桌面/手机检查、产物 hash | 待体验 |

### 分工与共享接口

已执行只读 subagent：Sol analyst 核查结构；Luna explorer 核查验证与隔离。实施使用 Sol worker，各自在主任务目录的职责后缀 worktree，独立分支，不共享写入；主 agent 将独占文件差异串行接入主工作树，自己维护共享文件及本记录。原子任务不继续派生，不提交、不合并。

共享接口只引用现有 API/types、路由和事件，不新建业务接口。共享统计外观集中于现有 `apple-base.css` 的 `.stats-*` 和 `ConsoleStatsCard.vue`，子任务删除 scoped 重复定义，按主 agent 约定使用：`.console-summary`（桌面一行/手机两列、细分隔、无外层卡）、`.console-toolbar`（可换行工具行）、`.console-records`（单层列表容器）。新类由主 agent 实现；子任务不写共享 CSS、components、测试或其他专家文件。

所有实现依赖上述视觉规则；缺少规格时由主 agent 补齐，不自行换肤。业务行为不改，结构选择可适度调整来满足紧凑、可访问且无溢出的实际结果。

## 本次验证记录

- 基线 `dev e05aec52fe9cb724f063648ce3132b321702047c`，本轮代码尚未提交。最终源码差异和二进制的 SHA-256、Console/UC 实际返回的入口与资产字节比对见 `docs/baselines/console-workspace/verification.json`，不将 HEAD 单独当成候选代码版本。
- `make test`：本轮最终文案调整后重新通过，Console 300/300、Stellar 225/225，部署/发布来源/模块边界检查、Go vet、Staticcheck、Go tests 均通过。日志 `/tmp/blog-console-workspace-make-test-final.log`。
- 组合态 Playwright：99/99 通过（3.9 分钟），包含 Console UI/手机可访问性、UC 共享按钮、写作工作区、设置原文/保存竞态/主题设置、文章发布/写作状态/Markdown 导入/shortcode、附件分组。`8096` 使用独立临时 DB、配置、内容和上传根，`VITE_E2E=1`。日志 `/tmp/blog-console-workspace-e2e-final.log`。之后仅将尺寸文案改为“紧凑/标准/展开”，并移除重复英文标签；最终主导航专项回归与正式构建另记，不把 99 项声称为最后文案调整后全部重跑。
- 新增主导航验收从真实 manifest 侧栏逐项读取 26 条路径，在 1280px 和 390px 访问全部入口，检查标题、未捕获异常、整页及主工作区横向溢出。桌面另有 768/1280/1440px 外壳验证；设置页覆盖 390/640/641/767/768/1280px。
- 实际发现并修复：390px 评论筛选条溢出、评论用户标题被操作挤到零宽、系统更新长错误将重试按钮挤出屏幕；保留手机 44px 触控、底栏无遮挡和焦点检查。独立 Sol reviewer 指出 active 按钮透明度影响对比度，已移除透明度变化并补充 computed opacity 断言，复审未发现实质问题。
- 测试预期变更依据：用户确认从海报/四张大卡/胶囊改为记录行/紧凑摘要/8px 工具按钮，因此对应选择器、统计尺寸和圆角断言随规格更新；明确海报行为的用例先主动切换海报。选择、清空、筛选、发布、保存、对比度、触控和焦点检查均保留。文章与页面空态不再渲染表格表头，静态检查验证空态与记录互斥；附件仍验证表头。图标检查通过生成器移除 13 个无引用图标，没有弱化门禁。
- 截图归档：`before-*` 为本轮现状参照；`e2e-*` 是最后主导航专项的隔离夹具截图；`settings-regression/` 是 99 项回归生成的设置截图；`after-*` 为正式构建在 8132 的真实浏览器截图。历史 `stellar-settings` 基线被测试写入后已把新产物移入本任务目录，并恢复历史图片，未覆盖其他任务证据。
- 最后文案调整后，主导航专项 1/1 通过（26 路由 × 2 宽度，18.3 秒），`make build` 通过；仅重启本任务 8132 的嵌入式服务，`/readyz` 为 200。Console/UC 入口和真实浏览器加载的脚本、样式逐字节匹配本次正式 dist，验证文件保留 SHA-256。主 agent 实际查看仪表盘、布局设计器/尺寸弹窗、设置基本/外观分区，并完成文章 8 条 → 草稿 3 条 → 指定草稿编辑页 → 返回全部 8 条的操作检查；最终截图已更新。独立端口 8130/8131 与生产服务未动。

## 体验与交接

- 本地隔离入口：`http://127.0.0.1:8132/console/posts`。测试账号 `admin` / `e2e-password`；只含 8 篇样例内容，未接入生产数据库。运行根为主任务工作树的 `.tmp/e2e-server.giRGjG`。
- 本轮修改管理后台视觉和共享 UI，未改 Go API/数据库/权限/发布规则；没有调用外部 AI 或付费服务，没有验证生产数据和真实第三方服务。
- 主任务分支尚未提交、未合入 `dev`、未推送或发布；`make merge-check` 会合并，未运行。根工作树和其他任务工作树保留原状。
- 三个已接入的子任务树（主路径追加 `-content`、`-workbench`、`-management`）保留未提交来源成果；无运行服务。主树保留 8132 体验服务。本任务合入约定目标并验证后，按 AGENTS 的保全/清理流程收尾。
- 下一步：用户体验视觉与日常操作。只有用户明确确认才改为“已验收”；需要提交或集成时按相应授权执行。

---

## 历史方案（2026-10-26 前，保留追溯；本轮以以上决定为准）

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
