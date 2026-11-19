# BLG-20261111-01：Console 仪表盘卡片内控件统一为幽灵语言（C1–C5、C7）

> 历史实现/验收记录：2026-11-17 提交、合入、发布和工作树收尾续接见 [BLG-20261117-01](BLG-20261117-01-dashboard-integration.md)，当前状态以该记录为准；以下保留原始验收细节。

- 项目 / 任务编号：personal-blog / BLG-20261111-01
- 状态：已验收（2026-11-11 用户体验后回复「验收通过」；提交与合入另记，尚未授权）
- 创建日期（Asia/Taipei）：2026-11-11
- 权威记录：本文件；验收清单、实测与取证在 `docs/baselines/dashboard-card-controls/`
- 主仓库 / 当前工作目录：`/Users/sakurasep/Documents/Code/Project/Personal Project/Blog`；`/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261111-feat-BLG-20261111-01-dashboard-card-controls`（任务工作树）
- 基线 / 分支 / 合入目标：`dev` @ `15e8fd4e` / `codex/feat/20261111-BLG-20261111-01-dashboard-card-controls` / `dev`（尚未提交、尚未合入）
- 当前 HEAD / 未提交差异或快照：`15e8fd4e`。实现与测试都是未提交差异：`dashboard.css` 与 6 个 widget（`TrendChart`、`HourlyChart`、`TopPosts`、`RecentPosts`、`RecentComments`、`RecentMoments`，合计 +342 / −129）；未跟踪的新文件是 `modules/console/tests/dashboard-card-controls.test.ts`、`tests/e2e/dashboard-card-controls.spec.ts`、`tests/e2e/support/dashboard-cards.ts`、本文件与 `docs/baselines/dashboard-card-controls/`
- 运行资源：隔离验收服务 `http://127.0.0.1:8108/`（`E2E_LISTEN=:8108 bash scripts/e2e/run-server.sh`，进程 `bash scripts/e2e/run-server.sh` → `go run ./cmd/blog … -dev` → `blog`，写本文件时 pid 43267 / 43295 / 43371；每次启动都是全新数据库，数据目录是工作树 `.tmp/e2e-server.SIGqj8`），2026-11-11 01:12 起运行在最终构建（入口 `assets/index-zlzqiBKW.js` + `assets/index-nMvIjoiF.css`，与 `modules/console/web/dist/index.html` 一致）上；库里是取证夹具（8 篇文章、9 条评论、含 11 张卡的布局）。不占用 8096（e2e 默认）与 8099（BLG-20261110-05 的体验服务）。一次性私有 Playwright 配置 `.tmp/pw-8108.config.ts` 与取证脚本 `.tmp/card-capture.spec.ts` 不入库

## 目标与范围

用户在 Console 首页（仪表盘）的卡片里，看到的小控件与头部（BLG-20261110-07 已落地）是同一套语言：无边框幽灵按钮、悬停只出中性浅底、焦点有环、触屏上点得到也看得见；这些控件在手机（真实 ≤767px 视口，以及桌面预览手机布局的 `data-bp` xs / xxs）上点击区 ≥44px，而视觉与卡片盒子高度不变。

范围是设计画布「改动清单」里的 C1–C5、C7：

- C1 图例 / 范围切换：趋势卡的 PV 改成静态图例，UV 是唯一开关（`aria-pressed` = 折线是否显示）；大档两个都静态。访问趋势、访问时段、热门文章三处 7 天 / 30 天统一为 12px、点击区 28px、补 `type="button"` 与 `aria-pressed`，选中仍是文字下 1px 下划线；手机 44px。
- C2 「全部 ›」：18px → 28px 点击区的幽灵小按钮，悬停浅底、焦点有环；新增 `.card-link` 修饰类，只落在卡片头（最近文章、最近评论、最近说说）。
- C3 文章行操作：与评论行同一条显隐规则（只有 `(hover: hover)` 且 ≥1024px 才隐藏，其余常显）；视觉 28px、手机点击区 44×44、间距 16px；图标补 `aria-label`。
- C4 评论行操作：30px + 绿 / 橙 / 红悬停底与彩边 → 28px + 中性浅底，只有图标变语义色；悬停只在 `(hover: hover)`；处理中只转被点的那个，其余禁用变淡。手机内联、点击区 44px。
- C5 评论弹窗按钮：独立的 `.comment-popover-btn` → `VButton` xs 的 ghost / primary / danger（danger 是共享组件的红底黑字）；`VButton` 的 44px 只认真实视口，卡片里按 `data-bp` 补一条。
- C7 热门文章行悬停：主色 6% 底 + 16% 描边 + 右移 2px → `--color-accent` 底、无描边、不位移，只在 `(hover: hover)`；排名徽标的彩色底不动。

明确不做：

- C6 叠放容器：`StackWidget.vue` 只存在于 BLG-20261110-05 工作树的未提交改动里（该分支本身还没有独立提交，`dev` 上没有这个组件）；用户已选头部结构 B（页签上移、与翻页合成一行），但 B 落在哪个任务里还没定，本任务不碰它的工作树与体验服务。
- S1 `VButton` 方块图标变体；`VButton` 组件本身、全站 `.apple-segmented`、`apple-base.css`、`.apple-link` 规则本身（布局设计页「导入当前浏览器布局」仍在用）、`DashboardDesignerView.vue`、快捷链接、图标集 `ri-subset.json`。
- 叠放页签 ARIA 补全、自动轮播悬停暂停（BLG-05 的备忘）；评论删除确认 `alertdialog` 的名称与焦点管理（dev 上就有，另行处理）。

## 已确认决定

- 2026-11-10 用户在设计画布上要求「开始下一步的设计」，随后对叠放头部结构说「选 B」；设计在画布 Version 12 的 03 / 03b / 03c / 04 / 05 板块。
- 2026-11-11 用户说「开始实现」：覆盖 C1–C5、C7 的实现与验证（叠放 C6 除外，原因见上）；**不含**提交、合并、推送、发布。评论行操作在手机上按「内联」做：画布 03b 的两个草稿里倾向内联，用户没有反对。
- 唯一参照图：设计画布 `https://claude.ai/artifact/RayrB1iw7eVJ4bVGCo8dDR`（私有，Version 12）板块 03（卡片内控件）、03b（手机 390）、04（控件规格）、05（改动清单）。画布不在仓库内，其中的数值已逐条抄进 `docs/baselines/dashboard-card-controls/README.md` 的验收清单；实测与清单不符的以实测为准，差异逐条记在该文件的「校正记录」里。
- 已读本地设计库（`resource/apple-design`：`library-consumption.json` 的 `recommendedReadOrder`、`SKILL.md`、`components/button.json`）：设计库的最小触控目标 44 × 44 落在手机组验收项里；颜色、圆角与动效复用 Console 现有语义 token（`--color-*`、`--radius-sm`、`--duration-fast`、`--ease-standard`），不新造裸色值。
- 与 BLG-20261110-05 的关系：它在这几个组件里只动了 script 段（`RecentPostsWidget` 11–22 / 53–65 行，`RecentCommentsWidget` 11–22 / 137–150 行，`TopPostsWidget` 8–18 / 35–50 行，`TrendChartWidget` 9–20 / 25–40 行；`HourlyChartWidget`、`RecentMomentsWidget` 没动）以及 `ri-subset.json`。本任务只改模板与 style，C4 的「哪个动作在处理中」另加一个 ref（`busyAction`），不改 `actionBusy` 的含义，也不新增 / 删除任何图标，避免与它冲突。**合并预检**（只读演算，不碰它的工作树）：把本任务的 4 个重叠组件与它工作树里的对应文件对 `15e8fd4e` 做三方合并，0 处文本冲突；它没改 `dashboard.css`，本任务没改 `ri-subset.json`。这只说明文本层面能合，没有构建过合并结果。

## 实现选择

- 点击区：`.card-hit::before` 以控件中心为基准，取「自身盒子」与 `--card-hit` 中的较大者；`--card-hit` 桌面 28px，真实 ≤767px 视口与 `data-bp` xs / xxs 两个来源各自解析为 44px，不改盒模型。
- 范围切换的扩大区做成右锚点、只向左补（第一枚多出 2px，正好接上下一枚的左缘；最后一枚右缘与卡片右缘齐平）：手机卡片通栏、`overflow: hidden`，对称补会让 `30 天` 的右半个扩大区点不到（命中只有 38px），还会把卡片撑出 7px 横向溢出。
- 「全部 ›」：中、大档 `margin-block: -4.5px -5.5px`，让扩大区正好落在 [0, 44]；小档（107px 高）仍对称 −5px，扩大区用 `--hit-dy: -9px` 上移。
- 行操作整组左移 8px（`--card-post-inset` / `--card-comment-inset`），给最右一枚向外补点击区留出空白；评论行在 `data-bp` 预览里是 0（预览手机布局里评论行自带内边距）。
- 评论弹窗改用 `VButton` xs，并另起一行（`flex: 1 1 100%`，最宽 480px）：改前它被挤在行内，手机上宽 154px，删除确认的按钮文字被挤成竖排。
- UV 开关 `z-index: 1`：大屏布局里它与范围切换同排只隔 16px，手机 44px 模式下「7 天」的扩大区向左补得更远，会抢走它的 4 个点。
- 悬停底在行内用 `--color-secondary-700`：行本身悬停是 accent，按钮再用 accent 就看不出来。
- 删掉仪表盘里已无元素使用的 `.apple-segmented` data-bp 规则组；全站的 `.apple-segmented` 在 `apple-base.css`，没动。

## 实现与验收

| 步骤 | 用户效果 / 实现范围 | 验收情形与预期 | 当前结果及证据 | 状态 |
| --- | --- | --- | --- | --- |
| T1 | 验收清单与任务记录 | 清单 D / M / X / G 见 `docs/baselines/dashboard-card-controls/README.md` | 已写（实现前），并按实测校正（「校正记录」） | 已验收 |
| T2 | 改前基线（`dev` @ `15e8fd4e`） | 同一份量测代码在改前、改后取同口径数据 | 已取：7 个场景、11 张卡，`measurements.json` 的 `before` 与 `before-*.png` | 已验收 |
| T3 | 模板与样式 C1–C5、C7：6 个 widget、`dashboard.css` | 清单 D1–D12、M1–M8、X1–X2 | 已实现，最终构建上全部通过 | 已验收 |
| T4 | 源码契约测试 `modules/console/tests/dashboard-card-controls.test.ts` | 结构约定不悄悄退化 | 9 项；与 `ui-restraint`、`dashboard-header-controls` 一起 30 / 30 通过，并已纳入 `make test` | 已验收 |
| T5 | Playwright `tests/e2e/dashboard-card-controls.spec.ts`（私有端口） | D / M / X 的可量测项、逐控件命中测试、忙碌态只转被点的图标 | 20 项，最终构建上 20 项通过（48.7s）；反向对照：把 `--card-hit` 钉回 28px 或去掉 `::before` 后命中测试报 44 / 46 处违规 | 已验收 |
| T6 | 门禁：`make test`、`pnpm lint`、`make build` | G1–G6 | 三者均退出 0；`dashboard-widget-size` 回归通过 | 已验收 |
| T7 | 真实服务浏览器验收 + 改后基线 | 截图与 `measurements.json` 的 after 部分 | 已在最终构建上重取：`after-*.png`、`measurements.json` 的 `after` | 已验收 |

## 执行与验证记录

| 日期 | 代码版本 / 候选差异 | 环境与运行对象 | 命令或实际操作 | 结果 / 证据位置 |
| --- | --- | --- | --- | --- |
| 2026-11-11 | `dev` @ `15e8fd4e` | 任务工作树 | `git worktree add … -b codex/feat/20261111-BLG-20261111-01-dashboard-card-controls dev`；`make hooks-install`；`pnpm install --frozen-lockfile` | 工作树已建，hook 已挂载 |
| 2026-11-11 | 改前（无差异） | `:8108` 隔离服务，改前构建 | 摆布局、灌夹具，`CARD_LABEL=before` 取证 | `measurements.json` 的 `before`、`before-*.png` |
| 2026-11-11 | 候选差异（实现中） | 同上，改后构建 | 逐轮 `make build`、重启 `:8108`、`CARD_LABEL=after` 取证并对照 | 取证中发现并修掉：手机范围按钮 `30 天` 扩大区被卡片右缘裁掉（命中 38px）且卡片多出 7px 横向溢出 → 右锚点只向左补；「全部 ›」扩大区被卡片上缘裁掉 0.5px（命中 42px）→ 非对称负外边距；真机 390 + 电脑预览里 UV 被「7 天」扩大区抢走 4 个点 → `z-index: 1`；评论弹窗被挤成窄条 → 另起一行 |
| 2026-11-11 | 同上 | `modules/console` | `tsx --test` 契约测试 + `ui-restraint` + `dashboard-header-controls` | 30 / 30 通过（新增 UV `z-index` 断言后再跑一次，仍 30 / 30） |
| 2026-11-11 | 同上 | `:8108` | `tests/e2e/dashboard-card-controls.spec.ts` 首轮 | 夹具缺陷：`seedCardData` 用 `recent-posts` 数夹具，但该接口只回最近 5 篇，「已灌过就跳过」永远不成立，重复灌数据撞 409 → 改为直接查第一篇夹具文章在不在；修后 20 / 20 通过 |
| 2026-11-11 | 同上 | `:8108`，真机 390 完整布局 | 一次性探针 + `page.route` 改写 CSS（反向对照） | 当前实现：55 个盒子不足 44px 的小控件里 44 个补到 44px、违规 0；`--card-hit` 钉回 28px：0 个、违规 44；去掉 `.card-hit::before`：0 个、违规 46。据此给 `M1` / `X1` / `X2` 加「补到 44px 的个数 ≥40」下限，防止空转通过 |
| 2026-11-11 | 同上 | 工作树 | `pnpm lint`；`node modules/stellar/web/scripts/gen-ri-subset.mjs` | lint 退出 0；生成器只把 `arrow-up-s-line`、`arrow-down-s-line` 挪了位置（集合不变），属顺序噪声，已 `git checkout --` 还原，任务差异不含 `ri-subset.json`；改动文件的 prettier 检查：改前 7 个文件里 6 个就不合 prettier（CI 与 `make test` 不检查），改前合规的 `RecentMomentsWidget.vue` 已保持合规，其余不为本任务重排整份文件 |
| 2026-11-11 | 同上 | 工作树 | `make test`（跑两遍：构建前一遍；契约测试加了 UV `z-index` 断言、收尾后在最终代码树上再一遍） | 两遍都退出 0：Console 579 项全过、`assets-test` 229 项（227 通过、2 跳过）、发布来源、模块边界、`go vet`、`staticcheck`、`go test ./...` |
| 2026-11-11 | 同上 | 工作树 | `make build` | 退出 0，Console（`/console/` + `/uc/`）与 Go 二进制；入口 `assets/index-zlzqiBKW.js` + `assets/index-nMvIjoiF.css` |
| 2026-11-11 | 同上 | `:8108`，最终构建 | 重启 `:8108`，对比服务入口与 `dist/index.html` 的 hash，再跑 `dashboard-card-controls.spec.ts`、`dashboard-widget-size.spec.ts`，最后重取 `after` 取证 | hash 一致；新用例 20 / 20（48.7s）；`dashboard-widget-size` 1 / 1，它改写了已跟踪的 `docs/baselines/dashboard-unified-size/*`，已 `git checkout --` 还原该目录；取证与上一轮只差评论时间戳与访问时段卡片里随数据变宽的 1.45px 位移 |
| 2026-11-11 | 同上 | 临时文件 | 与 BLG-20261110-05 工作树对应文件做三方合并演算 | 4 个重叠组件 0 冲突，见「已确认决定」 |
| 2026-11-11 | 同上，未提交 | 用户在 `:8108` 上体验 | 用户回复「验收通过」 | 状态改为已验收；这句话只确认验收，不含提交、合入、推送或发布的授权，代码与文档仍是未提交差异 |

## 提交与集成

尚无提交。2026-11-11 用户已确认验收，但尚未授权提交、合并、推送或发布（验收与这些操作分开授权）。待用户说「提交吧」后在任务分支提交，再等「合入 dev」才合入本地 `dev`；提交时 pre-commit hook 会跑完整 `make test`，提交信息用中文 `feat:` 前缀，只暂存本任务路径。合入并验证通过后按既有保全规则收尾本任务工作树，并停掉 `:8108` 验收服务。

## 交付与恢复

- 体验入口和操作步骤：`http://127.0.0.1:8108/console/dashboard`（登录用 e2e 夹具账号，见 `scripts/e2e/prepare.go`；库里已摆好 11 张卡与夹具数据）。
  1. 桌面宽屏：趋势中档的 UV 是开关、PV 是静态文字；7 天 / 30 天选中文字下 1px 线紧贴文字；最近文章 / 评论的行操作在悬停该行时出现，悬停按钮只出中性浅底（评论按钮的图标变绿 / 主色 / 橙 / 红）；点评论的回复、删除看弹窗；点「通过」看只有它的图标在转；热门文章行悬停是浅底、不位移；
  2. 视图模式切「平板」「手机」：控件仍是 28px 视觉，手机布局下点击区扩到 44px（可用开发者工具的元素检查看 `::before`）；
  3. 手机或开发者工具的 390 宽：行操作常显、间距 16px，评论弹窗另起一行、按钮 44px。
- 尚未完成 / 尚未验证：C6 叠放容器按范围不做；没有真机、Safari、Firefox 与屏幕阅读器验证，触屏是 Chrome 仿真；新 e2e 只在私有配置（`workers: 1`、`:8108`）上跑过，没有走仓库自带 `playwright.config.ts`（固定 `:8096` + `VITE_E2E=1` 构建，这次没占用 8096），整包 `pnpm run e2e` 里与其它用例共用一个服务的情形没实跑过（两份配置并行度相同，`VITE_E2E` 不影响仪表盘）；已知例外（小档 `全部 ›` 约 35px、手机最近评论大档 `全部 ›` 有时约 43px、改前就有的行级按钮 41 / 28 / 42px、真机 390 + 电脑预览里趋势「7 天」约 32px）写在 `docs/baselines/dashboard-card-controls/README.md` 的「已知例外」。
- 用户确认（明确确认后才改为已验收）：2026-11-11 用户体验后回复「验收通过」，已改为已验收。
- 下一步和恢复任务时首先核对的内容：工作树路径、HEAD（应为 `15e8fd4e` 或其后继）、`git status` 的未提交差异、`:8108` 服务是否还在（入口 hash 应与 `modules/console/web/dist/index.html` 一致，否则重新 `make build` 并重启）。接下来等用户授权提交（「提交吧」）与合入 `dev`（「合入 dev」）；C6 的落点（BLG-20261110-05 合入前改，还是合入后另开）也等用户定。
- 需要时的回退 / 数据保留方式：纯前端样式与模板改动，无数据变更；未提交时丢弃工作树改动即可，提交后用 `git revert`。停掉验收服务：结束 `bash scripts/e2e/run-server.sh` 及其子进程（`go run` 与 `blog`），数据目录 `.tmp/e2e-server.*` 是可丢弃的临时库。
