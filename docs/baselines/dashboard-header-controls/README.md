# 仪表盘头部控件验收基线（BLG-20261110-07）

任务记录：[`docs/plans/BLG-20261110-07-dashboard-header-controls.md`](../../plans/BLG-20261110-07-dashboard-header-controls.md)

## 参照图

设计画布 `https://claude.ai/artifact/RayrB1iw7eVJ4bVGCo8dDR`（私有，Version 9）：01 头部操作区 · 桌面 1440（方案 A，已选定）、02 手机 390 · 草稿、04 控件规格、05 改动清单与范围。画布不在仓库内，下面把它的数值抄成可量测的验收项；方案 B 未选用。

## 取证文件

| 文件 | 内容 |
| --- | --- |
| `before-*.png` / `after-*.png` | `.dashboard-heading` 元素截图。`lg-1440`：桌面 1440×900 自适应；`lg-1440-preview-xxs`：桌面预览「手机」布局；`xxs-390`：390×844 触屏视口；`xxs-390-mode-lg`：同视口切到「电脑」模式。仅改后另有 `lg-1440-preview-sm`（桌面预览「平板」800）、`xxs-360`、`xxs-320`（窄屏触屏视口） |
| `after-lg-1440-tooltip-{hover,focus,refresh}.png` | 桌面 1440 的提示状态：悬停「平板」、键盘 Tab 聚焦第一枚视图按钮、悬停刷新（截头部右侧 620×150） |
| `after-xxs-390-tooltip-focus.png` | 390 触屏视口键盘聚焦第一枚视图按钮：提示左对齐，不溢出视口左缘 |
| `measurements.json` | 同一份量测代码（`tests/e2e/support/dashboard-header.ts`）在改前（`before`）、改后（`after`，另含 800 预览、360、320）构建上取的按钮盒子与计算样式 |

## 验收清单

### 桌面（视口 1440×900，自适应，`data-bp=lg`）

| # | 预期 |
| --- | --- |
| D1 | 写作组不变：「继续写草稿」描边 36px、「撰写文章」主按钮 36px，字号 13px，图标 14px |
| D2 | 工具组按钮全部 36px 高：视图切换 4 枚与刷新为 36×36；「设置布局」为 `VButton` ghost（13px、图标 14px、36px 高） |
| D3 | 工具组无描边无底色：视图切换、刷新、设置布局默认态 `background` 透明、边框透明或 0；工具组与写作组之间保留原有 12px 间距 + 1px 分隔线 |
| D4 | 视图切换 4 枚间距 2px；切换组、刷新、设置布局之间 8px |
| D5 | 选中态：当前模式图标色 foreground，其下有一条 16×1 的 foreground 短线（`::after`，计算样式 `bottom: 5px`，即线底距按钮底 5px）；其余模式无线；`aria-pressed` 与之一致；不使用蓝色 |
| D6 | 悬停背景 accent、图标 foreground；按下 secondary-700；过渡只作用于 `background-color` / `color`，150ms |
| D7 | 悬停（`hover: hover`）或键盘聚焦（`:focus-visible`）任一视图按钮出现提示：首行模式名，次行说明——自适应「跟随窗口宽度」、电脑「预览宽度 1440px」、平板「预览宽度 800px」、手机「预览宽度 390px」（取 `VIEW_MODE_WIDTH`）。提示在按钮下方 6px，水平居中（第一枚左对齐）；12px / 11px、popover 底、border-strong 边、圆角 6、阴影 shadow-lg |
| D8 | 刷新按钮提示「刷新仪表盘数据」；视图切换与刷新按钮都没有原生 `title` |
| D9 | Tab 顺序：写作 → 视图切换 → 刷新 → 设置布局；聚焦环 2px ring、offset 2px |
| D10 | 刷新中：图标换成旋转的 `ri:loader-4-line`，`aria-busy="true"`，无额外底色，850ms 后恢复 |
| D11 | 功能不变：点击视图按钮后画布宽度为 100% / 1440 / 800 / 390；刷新触发 `DASHBOARD_REFRESH_EVENT`；「设置布局」进入 `/console/dashboard/designer` |

### 手机（视口 390×844 触屏，自适应，`data-bp=xxs`）

| # | 预期 |
| --- | --- |
| M1 | 写作按钮平分整行：两枚宽度相等（±1px）、高 44px，合计宽 = 内容宽度（366px） |
| M2 | 工具一行排完：4 个视图 + 刷新 + 设置布局的 `y` 相同（±1px），每枚 ≥44×44；工具间距 4px，切换组内 2px |
| M3 | 工具行总宽 ≤ 351px（375 宽手机可用宽度），预期约 332px |
| M4 | `.apple-header-actions` 高度：改前 150px（3 行），改后 ≤ 98px（2 行） |
| M5 | 选中短线（`::after`）计算样式 `bottom: 9px`，即线底距按钮底 9px（44px 按钮） |
| M6 | 视口 390 / 375 / 360 无横向滚动（`scrollWidth ≤ clientWidth`）；320 时设置布局换到下一行而不是溢出 |
| M7 | 键盘聚焦第一枚视图按钮时，提示左缘 ≥ 0，不溢出视口左缘 |
| M8 | 触屏（`hover: none`）点击切换后没有提示常驻 |

### 两个 44px 触发源

| # | 预期 |
| --- | --- |
| X1 | 真机视口 390×844 把视图模式切到「电脑」（`data-bp=lg`，画布 1440px 宽）：头部每个按钮宽、高都 ≥44px（改前：视图切换宽 16px、刷新 36×36、设置布局 39px 高） |
| X2 | 桌面 1440×900 把视图模式切到「手机」（`data-bp=xxs`，预览手机布局）：写作按钮与工具同为 44px 高，布局与 M1–M4 一致（改前：写作按钮 36px） |

### 守卫与不回归

| # | 预期 |
| --- | --- |
| G1 | `modules/console/tests/ui-restraint.test.ts` 通过：字号 11–14、圆角 6、无 `transition: all`、无渐变 / 发光、无裸 hex、无空 `@media` |
| G2 | `make test`、`pnpm lint` 通过；`ri-subset.json` 无改动，`assets-test` 通过 |
| G3 | `/console/dashboard/designer` 的保存按钮仍是 `.apple-edit-btn` 原样 |
| G4 | 其它页面的 `.apple-segmented` 样式不变（本任务不改 `apple-base.css`） |

## 改前实测（`measurements.json` → `before`，2026-11-10，`dev` @ `4137a719`）

| 场景 | 控件 | 实测 |
| --- | --- | --- |
| 桌面 1440 | 视图切换 ×4 | 16×24，无底无框，原生 `title` 提示 |
| 桌面 1440 | 刷新 | 36×36，card 底 + 1px 边框，原生 `title` |
| 桌面 1440 | 设置布局 | 112×39，14px，card 底 + 1px 边框 |
| 桌面 1440 | 继续写草稿 / 撰写文章 | 118×36 / 105×36，13px |
| 桌面预览手机（`data-bp=xxs`） | 写作按钮 | 36px 高（未随手机布局升到 44px） |
| 桌面预览手机 | 视图切换 / 刷新 / 设置布局 | 44×44 / 44×44（card 底 + 边框）/ 100×44，设置布局单独占一行；`.apple-header-actions` 142px 高 |
| 390 触屏（`xxs`） | 写作按钮 | 118×44、105×44，靠左，不平分；`.apple-header-actions` 150px 高（3 行） |
| 390 触屏 | 视图切换 / 刷新 / 设置布局 | 44×44 / 44×44（card 底 + 边框）/ 96×44，设置布局单独占一行 |
| 390 触屏 + 「电脑」模式（`lg`） | 视图切换 / 刷新 / 设置布局 | 16×44 / 36×36 / 112×39，均小于 44px |

## 改后实测（`measurements.json` → `after`，2026-11-10，候选 = `dev` @ `4137a719` + 本任务未提交差异）

量测取自 `VITE_E2E=1` 测试构建（入口 `index-CXNc83Vk.js`）；之后已用 `make build` 换回正式产物（入口 `index-BgNUIb_w.js`），头部专项 Playwright 在正式产物上复跑 4/4 通过。390 / 360 / 320 是 Chrome 触屏模拟（`isMobile` + `hasTouch`，设备像素比 2），不是真机。

| 场景 | 控件 | 改前 → 改后 |
| --- | --- | --- |
| 桌面 1440 | 视图切换 ×4 | 16×24 → 36×36，无底无框；原生 `title` 换成样式化提示 |
| 桌面 1440 | 刷新 | 36×36（card 底 + 边框）→ 36×36，透明底无边框 |
| 桌面 1440 | 设置布局 | 112×39（`.apple-edit-btn`，card 底 + 边框）→ 106×36（`VButton` ghost，透明底） |
| 桌面 1440 | 继续写草稿 / 撰写文章 | 118×36 / 105×36 → 119×36 / 106×36；按钮样式未改，图标改为显式 14px（原随按钮字号），宽度各 +1px |
| 桌面 1440 | 头部操作区整体 | 530×39 → 564×36 |
| 桌面预览「手机」 | 写作按钮 | 118×36 / 105×36，靠左 → 175×44 ×2，平分整行 |
| 桌面预览「手机」 | 工具 | 设置布局 100×44 单独占一行 → 98×44 与视图切换、刷新同一行；`.apple-header-actions` 142 → 98px；标题区 233.5 → 189.5px |
| 390 触屏 | 写作按钮 | 118×44 / 105×44，靠左 → 179×44 ×2，平分整行（合计 366px = 内容宽度） |
| 390 触屏 | 工具 | 设置布局 96×44 单独占一行 → 98×44 与其余工具同一行（工具行 332px，x 12→344）；`.apple-header-actions` 150 → 98px；标题区 264 → 212px（−52px） |
| 390 触屏 + 「电脑」（`data-bp=lg`） | 全部头部按钮 | 视图切换 16×44、刷新 36×36、设置布局 112×39 → 全部 ≥44×44（视图 / 刷新 44×44，设置布局 106×44，写作 119×44 / 106×44） |
| 桌面预览「平板」800（`sm`） | 全部头部按钮 | 改前未取 → 36px 一行（564×36），不换行 |
| 360 触屏 | 写作 / 工具 | 改前未取 → 写作 164×44 ×2，工具一行，头部操作区 336×98，`scrollWidth` 360 / 360 |
| 320 触屏 | 写作 / 工具 | 改前未取 → 写作 144×44 ×2；工具行放不下设置布局，它换到下一行（296×146），`scrollWidth` 320 / 320，无横向滚动 |

## 验收结果

| 清单 | 结果 | 依据 |
| --- | --- | --- |
| D1–D11 | 通过 | `tests/e2e/dashboard-header-controls.spec.ts`「D1–D11 桌面 1440…」逐项断言（spec 注释按 D 编号分段）；截图 `after-lg-1440*.png` |
| M1–M8 | 通过（触屏模拟，非真机） | 「M1–M8 手机 390…」；截图 `after-xxs-*.png` |
| X1 / X2 | 通过 | 「X1 真机视口 390 + 「电脑」预览…」「X2 桌面 1440 + 「手机」预览…」；截图 `after-xxs-390-mode-lg.png`、`after-lg-1440-preview-xxs.png` |
| G1 / G2 | 通过 | `make test` 退出码 0（Console 测试 570/570，含 `ui-restraint.test.ts` 与新增 6 项源码契约；Stellar 229/229；Go 全部通过）；`pnpm lint` 无告警；`make build` 退出码 0；`ri-subset.json` 未改动 |
| G3 | 通过 | D 组用例末尾核对 `/console/dashboard/designer` 的 `.apple-edit-btn` 仍为 14px；源码契约测试断言设计页仍引用 `.apple-edit-btn` |
| G4 | 通过 | 本任务没有改 `apple-base.css`；改动的受跟踪文件只有 `DashboardView.vue`、`dashboard.css` |

未覆盖：真机与触屏硬件；Safari / Firefox（只用了系统 Chrome）；屏幕阅读器的实际朗读（只核对了 `aria-pressed`、`aria-label`、`aria-describedby`、`aria-busy` 的取值）。
