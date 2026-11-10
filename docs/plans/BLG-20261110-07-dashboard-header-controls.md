# BLG-20261110-07：Console 仪表盘头部控件统一为幽灵工具组

- 项目 / 任务编号：personal-blog / BLG-20261110-07
- 状态：待体验
- 创建日期（Asia/Taipei）：2026-11-10
- 权威记录：本文件；验收清单与取证在 `docs/baselines/dashboard-header-controls/`
- 主仓库 / 当前工作目录：`/Users/sakurasep/Documents/Code/Project/Personal Project/Blog`；`/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261110-feat-BLG-20261110-07-dashboard-header-controls`
- 基线 / 分支 / 合入目标：`dev` @ `4137a719` / `codex/feat/20261110-BLG-20261110-07-dashboard-header-controls` / `dev`
- 当前 HEAD / 未提交差异或快照：任务分支上基线 `4137a719` 之后的一个 `feat:` 提交（哈希以 `git log --grep BLG-20261110-07` 为准，合入 `dev` 时补记），含实现、测试、本文件与基线；无未提交差异。改 `DashboardView.vue`、`dashboard.css`（+171 / −80），新增 `modules/console/tests/dashboard-header-controls.test.ts`、`tests/e2e/dashboard-header-controls.spec.ts`、`tests/e2e/support/dashboard-header.ts`、本文件与 `docs/baselines/dashboard-header-controls/`
- 运行资源：`:8107` 的隔离服务（`scripts/e2e/run-server.sh`，独立数据库与根目录在 `.tmp/e2e-server.*`，仅监听 `127.0.0.1`）归本任务。验收用的实例已停掉；为了体验又用正式产物重新起了一份（22:02:59 启动，入口 `index-BgNUIb_w.js`，与 `dist` 一致），仍在运行，用完请停：`lsof -nP -iTCP:8107 -sTCP:LISTEN -t | xargs kill`。工作树里的 `modules/console/web/dist`、`dist-uc` 与 `blog` 二进制是最后一次 `make build` 的正式产物（均被 gitignore）。重新构建前端后必须重启该服务，浏览器才会拿到新的入口文件

## 目标与范围

用户在 Console 首页（仪表盘）头部能一眼分清三层控件：写作按钮（继续写草稿、撰写文章）不变；视图切换、刷新、设置布局统一成 36px（手机 44px）的无边框幽灵工具组。视图切换的点击区从 16×24 扩到 36×36，悬停或聚焦时显示带预览宽度的提示；手机上写作按钮平分整行、工具一行排完。

范围是设计画布「改动清单」里的 H1–H5：

- H1 设置布局：`.apple-edit-btn`（14px · 39px 高 · card 底 + 边框）→ `VButton` ghost md（13px · 36px · 透明底）。
- H2 刷新：`.apple-icon-btn`（card 底 + 边框）→ 幽灵图标按钮 36×36（手机 44）。
- H3 视图切换：点击区 16×24 → 36×36（手机 44）；选中仍是图标下方的短下划线；悬停出浅底；新增样式化提示，宽度取 `VIEW_MODE_WIDTH`。
- H4 手机头部：写作按钮平分整行（44px），工具一行排完。
- H5 图标尺寸：带文字 14px、纯图标 16px（原为 13 / 16 混用）。

明确不做：卡片内控件 C1–C6（与 BLG-20261110-05 重叠，等其合入 dev 后另开任务）；全站 `.apple-segmented` 与 `VButton` 组件本身（含方块图标变体 S1）；`DashboardDesignerView.vue` 与 `.apple-edit-btn` 规则（设计页保存按钮仍在用）；快捷链接样式；图标集（`ri-subset.json`）。

## 已确认决定

- 2026-11-10 用户在设计画布上选定头部工具区「方案 A（幽灵组）」，方案 B（描边工具条）弃用；随后说「开始做头部」。该指令覆盖实现与验证，不含提交、推送、合并。
- 2026-11-10 用户在体验后说「可以 目前来看没问题 提交吧」：授权把本任务提交到任务分支，不含合并、推送、发布；这也不是明确的验收确认，状态保持待体验。
- 唯一参照图：设计画布 `https://claude.ai/artifact/RayrB1iw7eVJ4bVGCo8dDR`（私有，Version 9）画板 01（桌面 · 方案 A）、02（手机 390 · 草稿）、04（控件规格）、05（改动清单）。画布不在仓库内，其中的数值已逐条抄进 `docs/baselines/dashboard-header-controls/README.md` 的验收清单。已读本地设计库（`resource/apple-design`：`SKILL.md`、`README.md`、`components/button.json`、`components.css` 按钮段、`css.json`）：复用 Console 现有语义 token（`--color-*`、`--radius-sm`、`--shadow-lg`、`--duration-fast`、`--ease-standard`）与 `VButton`，不新造裸色值；设计库的 44×44 最小触控目标要求落在 M / X 两组验收项里。
- 实现选择（普通实现决定，未超出授权）：
  1. 视图切换不再套全站 `.apple-segmented`，改用头部局部的 `.dashboard-tool-btn`。`.apple-segmented` 是「文字 + inset 下划线」分段，没有悬停底，点击区也没有 36×36；全站不动，`apple-base.css` 不改。
  2. 提示用纯 CSS：按钮相邻的 `.dashboard-tip`（`role="tooltip"`），`aria-describedby` 只指向说明那一行，避免读屏把模式名读两遍；去掉原生 `title`，避免出现两个提示。`:focus-visible` 一律显示，`:hover` 仅在 `(hover: hover)` 下显示，触屏点击后不残留。第一枚按钮的提示左对齐，避免窄屏溢出左缘。
  3. 刷新按钮沿用同一提示（文案「刷新仪表盘数据」，与 aria-label 一致），保持工具组内一致；这是对画板 ④ 的小幅延伸。
  4. 44px 有两个触发：`data-bp` 为 xs / xxs（桌面预览手机布局）和真实 ≤767px 视口（真机上预览模式选了电脑 / 平板时 `data-bp` 仍是 lg）。`VButton` 自带的 ≤767px 44px 规则不动，另按 `data-bp` 给头部 `.btn` 补 `min-height: 44px`，使桌面预览手机布局时写作按钮与工具同高（改前写作按钮是 36px）。
  5. 刷新加载态按画板只转图标，不再用 10% 主色底（删除 `.dashboard-refresh-btn[aria-busy]` 规则），`aria-busy` 保留。
  6. 手机快捷链接保持现状（居中、44×44）。画板 02 的 ④（文字左对齐）与画板 05「本稿不动：快捷链接样式」互相矛盾；首项文字只偏 2.5px，左对齐还会让相邻文字间距变得不均（21px / 34px），故不改。
  7. 删除只由头部使用的 `.apple-icon-btn`、`.view-switch` 规则；`.apple-edit-btn` 及其手机覆盖保留。
  8. 画板里的两处量值与实测不符，以实测为准：视图切换点击区实测 16×24（画板写 16×28）；手机头部控件区（`.apple-header-actions`）实测 150px → 目标 98px，减少 52px（画板 204 → 152 把快捷链接一行也算了进去，差值同为 52px）。
- 与 BLG-20261110-05（待体验、未合入 dev、工作树 `…/20261110-feat-BLG-20261110-05-dashboard-halo-grid`）的关系：该分支改 `DashboardGrid.vue`、各 widget 等，没有改 `DashboardView.vue`；计划里列了 `dashboard.css`，但当前工作树还没动它，其未提交改动也不含 `ri-subset.json` 之外的头部相关文件。本任务只改 `dashboard.css` 的头部相关区块，不重排其它段落，以减少合并冲突；合入顺序谁先谁后都不需要重做，冲突只可能出现在 `dashboard.css` 同一区块。

## 实现与验收

| 步骤 | 用户效果 / 实现范围 | 验收情形与预期 | 当前结果及证据 | 状态 |
| --- | --- | --- | --- | --- |
| T1 | 头部模板与样式 H1–H5：`DashboardView.vue`、`dashboard.css` | 清单 D1–D11、M1–M8、X1–X2，见 `docs/baselines/dashboard-header-controls/README.md` | 已实现；清单 D1–D11 / M1–M8 / X1–X2 逐项通过（见 T3、T5）；设计页 `.apple-edit-btn` 未改 | 待体验 |
| T2 | 源码契约测试 `modules/console/tests/dashboard-header-controls.test.ts` | 提示与状态语义、`VIEW_MODE_WIDTH` 取值、旧类名清理、44px 双触发 | 6 项通过（含在 `make test` 的 Console 测试 570/570 内） | 待体验 |
| T3 | Playwright `tests/e2e/dashboard-header-controls.spec.ts`（私有端口运行） | D / M / X 的可量测项，含 390 / 360 / 320 无横向滚动 | 4 项通过（D1–D11、M1–M8、X1、X2），测试构建与正式产物各跑一次；相关既有用例 5 项通过（测试构建） | 待体验 |
| T4 | 门禁：`make test`、`pnpm lint` | G1–G4 | `make test` 退出码 0；`pnpm lint` 无告警；`make build` 退出码 0 | 待体验 |
| T5 | 真实服务浏览器验收 + 改后基线 | 截图与 `measurements.json` 的 after 部分 | 已取：7 个场景截图、4 张提示截图和量测，见 README「改后实测」 | 待体验 |

## 执行与验证记录

| 日期 | 代码版本 / 候选差异 | 环境与运行对象 | 命令或实际操作 | 结果 / 证据位置 |
| --- | --- | --- | --- | --- |
| 2026-11-10 | `4137a719`（未改动） | `VITE_E2E=1 pnpm --filter console run build` + `scripts/e2e/run-server.sh`，`:8107`，系统 Chrome（headless） | `tsx .tmp/header-capture.ts before`（量测代码 `tests/e2e/support/dashboard-header.ts`） | 改前基线：`docs/baselines/dashboard-header-controls/before-*.png`、`measurements.json` 的 `before` |
| 2026-11-10 | `4137a719` + 未提交差异 | `VITE_E2E=1 pnpm --filter console run build` 后 `scripts/e2e/run-server.sh`（`:8107`，入口 `index-CXNc83Vk.js`，与 `dist` 一致，启动于构建之后），系统 Chrome（headless） | `tsx .tmp/header-capture.ts after`；`pnpm exec playwright test -c .tmp/pw-8107.config.ts`（私有端口配置，不用官方 `webServer`，避免撞 8096） | 改后基线 `after-*.png`、`measurements.json` 的 `after`、README「改后实测」；头部专项 4/4 通过 |
| 2026-11-10 | 同上 | 同上 | 既有相关用例：页头四视口矩阵、390 快捷链接 ≥44px、仪表盘默认布局、运行时字号白名单、全部主导航无横向溢出 | 5/5 通过 |
| 2026-11-10 | 同上 | 仓库根 | `pnpm lint`；`make test` | `pnpm lint` 无告警；`make test` 退出码 0：Console 测试 570/570（含 `ui-restraint.test.ts` 与新增 6 项源码契约）、Stellar 资源测试 229/229、发布来源与模块依赖检查、Go vet / Staticcheck / 全部 Go 测试通过 |
| 2026-11-10 | 同上 | 仓库根 | `make build` | 退出码 0：Console 与 User Center 真实构建（入口 `index-BgNUIb_w.js`）+ Go 编译；只有既有的 pnpm 字段与 chunk 大小警告；`dist` 与 `blog` 已换回不带 `VITE_E2E` 的正式产物 |
| 2026-11-10 | 同上 | `scripts/e2e/run-server.sh`（`:8107`，正式产物入口 `index-BgNUIb_w.js`，与 `dist` 一致，21:58:25 启动，晚于 `make build`），系统 Chrome | `pnpm exec playwright test -c .tmp/pw-8107.config.ts`（头部专项） | 4/4 通过；随后停止服务，端口已释放；`git status` 只含本任务文件，无其它任务的基线被改写 |
| 2026-11-10 | 同上 | 派生配置 `.tmp/pw-official-list.config.ts`（沿用官方 `playwright.config.ts` 的 `use` / `projects` / testMatch，只去掉会构建并占 8096 的 webServer，`--list` 无副作用） | `pnpm exec playwright test -c .tmp/pw-official-list.config.ts --list` | 共 243 个用例 / 29 个文件，本任务 4 个用例被收集；官方配置没有 `projects`，没有 viewport / 设备预设；本 spec 用 `browser.newContext` 自建 context 并显式指定视口，不受配置层 `use` 影响；写作按钮组由「文章」导航模块是否启用决定（`hasPosts`），与数据无关。**没有用官方配置实际运行**：它的 webServer 会重建 `dist` 并占 8096，本机也没装 Playwright 自带 Chromium |

## 提交与集成

| 提交 | 行为变化与原因 | 验证 | 合并 / 发布情况 |
| --- | --- | --- | --- |
| 任务分支上的 `feat: 仪表盘头部控件统一为无边框幽灵工具组`（2026-11-10 用户授权提交；哈希以 `git log --grep BLG-20261110-07` 为准，合入 `dev` 时补记） | 头部视图切换、刷新、设置布局统一为 36px（手机 44px）幽灵工具；视图切换点击区 16×24 → 36×36 并带预览宽度提示；手机写作按钮平分整行、工具一行排完，头部控件区 150 → 98px。原因：三层控件原先分不清，点击区过小，手机头部占 3 行 | `make test`（提交时由 pre-commit hook 再跑一次）、`pnpm lint`、`make build`；头部 Playwright 4/4（测试构建与正式产物各一次） | 已提交到任务分支；未合并、未推送、未运行 `make merge-check`（它会实际合入根目录 `dev`，而根 `dev` 工作树有大量用户未提交改动，需另行确认） |

## 交付与恢复

- 体验入口和操作步骤：浏览器打开 `http://127.0.0.1:8107/console/dashboard`，登录用 `scripts/e2e/prepare.go` 里的 e2e 种子管理员账号（独立数据库，不是真实站点数据）。服务没在跑时，在本工作树根目录执行 `E2E_LISTEN="127.0.0.1:8107" bash scripts/e2e/run-server.sh`（它提供当前的 `modules/console/web/dist`，改过前端后先 `make build`）。桌面看头部右侧工具组，悬停或 Tab 聚焦视图按钮看提示；切到「手机」预览，或把窗口收窄 / 用 390 宽的手机模式，看手机头部。
- 尚未完成 / 尚未验证：没有真机与触屏硬件（390 / 360 / 320 只是 Chrome 触屏模拟，“触屏点按后提示不残留”也只由模拟覆盖）；只在系统 Chrome 验证，没有 Safari / Firefox；屏幕阅读器只核对了 `aria-pressed`、`aria-label`、`aria-describedby`、`aria-busy` 的取值，没有实际朗读；没有跑完整 `pnpm run e2e`（它会重写其它任务的基线并替换 `dist`）；`make merge-check`、合并、推送都没做（未授权）；提交已获授权并完成（见「已确认决定」与「提交与集成」）。
- 后续（不在本任务内）：卡片内控件 C1–C6 等 BLG-20261110-05 合入 dev 后另开任务；设计画布里的 16×28 与手机 204→152 两处数字与实测不符（见「已确认决定」第 8 条），画布未改动。
- 用户确认（明确确认后才改为已验收）：尚未明确验收。2026-11-10 用户说「目前来看没问题」并授权提交，这不等于验收确认，状态保持待体验。
- 下一步和恢复任务时首先核对的内容：工作树路径、分支与 HEAD、`git status`、浏览器加载的 `dist` 资源 hash 是否与当前构建一致。`:8107` 体验服务是否仍在跑（见「运行资源」）；需要再跑 e2e 时重新 `VITE_E2E=1 pnpm --filter console run build` 并起服务，跑完用 `make build` 换回正式产物。
- 需要时的回退 / 数据保留方式：纯前端样式与模板改动，无数据变更；丢弃分支即可回退。
- 本任务 worktree 收尾：尚不满足清理条件（未合入）。
