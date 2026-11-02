# Console「方向 A 编辑台」独立验收报告

- 任务：BLG-20261031-01（`docs/plans/console-editorial-desk.md`）
- 对象：分支 `codex/refactor/20261031-BLG-20261031-01-console-editorial-desk`（HEAD `bc7e2da1`），71 个已修改跟踪文件，未提交/未合并/未推送
- 验收时间：2026-10-31（Asia/Shanghai）
- 验收方式：独立构建 + 隔离运行（8137）+ 真实浏览器交互 + 目标 Playwright 回归
- 机器可读结果：`docs/baselines/console-editorial-desk/acceptance/verification.json`

## 结论先行

**有条件通过（conditional-pass）。**

方向 A 的核心目标已经达成：仪表盘不再有 KPI 卡片墙与图标标题，列表/评论改为单层记录行，设置去掉外层卡片与蓝色条，主操作是白底黑字（computed `#e8e8ec` / `#000`，6px 圆角），彩色状态底被 5px 圆点取代，字号与圆角主体收敛（三处 16px 继承泄漏见 D06）。文章、评论、设置、外观的真实交互全部通过，4 个目标 Playwright spec **64 项全绿**，控制台无未捕获异常。

放行条件：**390px 评论行可读性缺陷（D01）** 建议在合并前修复；D02（导读标题重复）与 D03（两处触控目标 <44px）为非阻断但应记录的收尾项。另发现 **D06：三处可见文字继承 16px**，字号白名单未真正闭合（源码级门禁看不到继承值）。D04/D05 为既有问题，不归因于本次改动。

## 1. 环境与隔离

| 项 | 值 |
|---|---|
| 工作树 | feature worktree（本分支，未提交改动原样保留） |
| 运行端口 | `127.0.0.1:8137` |
| 隔离根 | `.tmp/acc-server.8137`（独立 config/content/data/uploads） |
| DB | `.tmp/acc-server.8137/data/blog.db`（`scripts/e2e/prepare.go` + `.tmp/seed.go`） |
| 账号 | `admin / e2e-password` |
| 浏览器 | Google Chrome 156.0.8078.74（Playwright 自带浏览器未安装，使用 channel=chrome） |
| 构建 | `make console` exit 0（含 `vue-tsc -b` 类型检查与 UC 构建） |

未接触生产库或其他工作树；验收结束后停止 8137 隔离服务（隔离根与证据保留在 `.tmp/acc-server.8137` 与 `acceptance/`）。

## 2. 实际加载产物（不是 HTTP 200 作为证据）

从 `/console/` 取回 HTML，对 6 个入口资源逐个下载并计算 SHA-256，与本机 `modules/console/web/dist` 比较：

| 资源 | SHA-256（前 16 位） | 结果 |
|---|---|---|
| `assets/index-BL0vDRlo.js` | `7a0d3feb02982a54…` | MATCH |
| `assets/api-C-H7IBEY.js` | `28f3d58e9e625d42…` | MATCH |
| `assets/runtime-core.esm-bundler-GmtFT3dp.js` | `bcd39943a10cb760…` | MATCH |
| `assets/pinia-CXeiBUD1.js` | `7e7c94b9658d5172…` | MATCH |
| `assets/preload-helper-l0U7FUPx.js` | `dbd40ae429a13a3e…` | MATCH |
| `assets/index-DYZh7cwz.css` | `cacb82ed72d355a9…` | MATCH |

目标 Playwright 会以 `VITE_E2E=1` 重建 dist；测试结束后已用 `make console` 重建普通产物、重启 8137，并再次核对上述 6/6 哈希一致，且两次构建的入口哈希可复现。

## 3. 夹具数据（非空态）

- 文章 8 篇：5 已发布 / 2 草稿 / 1 回收站，1 篇置顶；分类 `技术`、`技术/Go`、`生活`、`设计`，含多标签
- 页面 2 个：`about`（已发布）、`guestbook-guide`（草稿）
- 评论 6 条：approved 2（含 1 条管理员回复）、pending 2、rejected 1、spam 1
- 附件 5 个（真实文件写入隔离 uploads）
- 近 7 日访问：427 PV / 84 UV（趋势图有真实数据）

默认文章列表显示 7 篇（第 8 篇在回收站），另用 `evidence-posts-trash-seed-1280.png` 证明回收站中的第 8 篇。

## 4. 截图

均存于 `docs/baselines/console-editorial-desk/acceptance/`，同一批 `verification.json` 记录 SHA-256。

| 页面 | 1280 | 390 |
|---|---|---|
| 仪表盘 | `after-dashboard-1280.png` | `after-dashboard-390.png` |
| 文章列表 | `after-posts-1280.png` | `after-posts-390.png` |
| 设置（基本） | `after-settings-1280.png` | `after-settings-390.png` |
| 设置（外观 Tab） | `after-settings-appearance-1280.png` | `after-settings-appearance-390.png` |
| 评论 | `after-comments-1280.png` | `after-comments-390.png` |
| 外观 | `after-appearance-1280.png` | `after-appearance-390.png` |

交互证据：`evidence-*.png`（搜索/筛选/排序/全选/批量回收站/编辑器/发布/保存草稿/评论批准与拒绝/设置脏标记与保存/草稿预览/手机底栏/页面列表/导读展开/回收站第 8 篇）。

### 与 `reference/preview-A-*.png` 的对照

- 一致：事实指标行取代 KPI 卡、整宽趋势、最近文章/评论双列、快捷操作文字链接、单层列表、设置去外框、白底黑字主操作。
- 有意的结构差异（任务卡明文覆盖注入预览）：仪表盘采用整宽趋势 + 双列最近内容；设置新增单行导读。
- 非预期偏差：D01（手机评论行）、D02（导读标题重复）与 D06（16px 字号泄漏），见缺陷清单。

## 5. 真实交互结果

文章列表（19 项检查全部通过；其中“全选/取消”首轮因验收脚本同时命中工具栏与浮栏两个同名“取消全选”按钮而中断，限定工具栏按钮后复验通过——脚本选择器问题，不是产品缺陷）：

- 搜索 `SQLite` → 1 行，清除后恢复
- 状态“草稿”→ 3 行、URL `?status=draft`；分类“技术”（含子分类）→ 5 行
- “本页·最早创建”改变首行；全选 9 行出现“已选文章”浮栏，取消后清空
- 点击标题进入 `/console/posts/edit/acc-batch-1`；草稿发布后出现在已发布
- 已发布文章“保存草稿”→ Toast“内容已保存，但还未发布”，线上状态仍为已发布（现有产品语义，无 published→draft 直接按钮）
- 移入回收站 → 回收站可见 → 恢复成功；批量 2 篇移入回收站成功

评论：待审核 → 批准（计数 2→1、已批准 2→3、Toast 正常）；拒绝后出现在已拒绝。

设置：修改副标题出现“未保存修改”（页头与导读同步）；保存后消失；切 Tab、草稿预览（“预览：整站外观”）正常；带未保存修改点击仪表盘触发 `window.confirm` 并停留在设置页。

手机：390 底栏可见并可导航；文章/评论/设置/仪表盘滚到底末项 bottom 均 < 底栏 top（不遮挡）；设置保存条也在底栏之上。

未捕获异常：截图与交互两轮独立会话均 0 条。

## 6. 视觉与可访问性

- 横向溢出：五组页面 + 设置外观在 390 与 1280 的 `scrollWidth - innerWidth = 0`；设置 Tab 的越界项位于组件自身横向滚动容器内。
- 字号（可见文本）：主体已收敛——仪表盘 11/12/13/14/20/24；文章 12/13/14/20/24；设置 11/12/13/14/24；评论 12/13/14/20/24。**但未完全闭合**：仪表盘 2 个 `.apple-link`“全部”、设置-外观 2 段 `.settings-help`、外观页 2 段说明均为继承的 16px（不在白名单内），详见 D06；源码级字号门禁只能检查显式声明，无法看到继承值。
- 圆角：仅 6/8/50%/9999px（另 0），未见 2/3/4/5/7/9/11/12/14px。
- 状态：`record-status` / `comment-status` 等均无彩色背景，仅 5px 彩色圆点。
- 卡片套卡片：真实内容卡面嵌套 0；设置外观的 `example-article` / `preset-example` 是预设风格缩略示意，不计为卡面嵌套。
- 主操作：`background #e8e8ec`、`color #000000`、`border-radius 6px`。
- 页面标题：五页 h1 均无图标。
- 触控目标：文章页搜索/状态/筛选/全选/行操作、评论操作按钮均 44px；**仪表盘快捷链接与评论勾选框不足 44px（D03）**。

## 7. 缺陷清单

| ID | 级别 | 阻断 | 问题 | 证据 |
|---|---|---|---|---|
| D01 | 中 | 否 | 390 评论行信息列被压到 34px：`.comment-status` 34×36 竖排折行，邮箱可见 34/144px；本次新增的窄屏 `.action-btn` 44px 把 5 个操作按钮撑到 220px+，但行布局未调整（旧版按钮更小，属本次引入的回归） | `after-comments-390.png`、`measurements-dom.json#/comments390`；`CommentList.vue:443-512, 758-766` |
| D02 | 低 | 否 | 设置基本 Tab 导读标题“站点身份”与下方分区 h2 同名，同屏重复；信息本身完整（影响位置/如何确认/保存说明/草稿预览/未保存提示） | `evidence-settings-guide-basic-1280.png`、`evidence-settings-guide-expanded-1280.png`、`after-settings-390.png` |
| D03 | 低 | 否 | 390 仪表盘快捷链接 39×20 / 26×20；评论行 checkbox 16×16 且无 label 包裹；现有 mobile-a11y spec 未覆盖 | `measurements-dom.json`、`after-dashboard-390.png` |
| D04 | 低（既有） | 否 | 390 外观页页脚“访问网站”bottom 793 被底栏 top 787 压住 6px；改动前截图同样存在 | `after-appearance-390.png`、`reference/before-appearance-390.png` |
| D05 | 提示（既有） | 否 | 置顶文章在“本页·最近创建”下不优先（客户端按 created_at 重排）；本次 diff 未触及该排序 | `after-posts-1280.png` |
| D06 | 低 | 否 | 字号白名单未闭合：仪表盘 `.apple-link`“全部” 16px（`font-size:12px` 被后面的 `font: inherit` 重置）；设置-外观 `.settings-help` 16px（样式只在 SearchSettings 的 scoped 中）；外观页两段说明 16px（`CanonicalSettingsNotice` 的 p 未设字号）。源码门禁看不到继承值 | `measurements-structure.json`、`after-dashboard-1280.png`、`after-settings-appearance-1280.png`、`after-appearance-1280.png`；`dashboard.css:926-940`、`SearchSettings.vue:163-168`、`CanonicalSettingsNotice.vue:21-26` |

D01 复现：admin 登录 → 390×844 → `/console/comments` → 查看待审核行。期望昵称/单行状态/可读邮箱；实际状态竖排、邮箱只剩几个字符。桌面 1280 不受影响，操作按钮仍可点。

## 8. 命令与结果

| 检查 | 命令 | 结果 |
|---|---|---|
| Console 构建/类型检查 | `make console` | exit 0 |
| 隔离启动 | `ACCEPTANCE_ROOT=$PWD/.tmp/acc-server.8137 bash .tmp/acceptance-run.sh` | `/readyz` 200 |
| 产物哈希 | `curl /console/` + `shasum -a 256` ×6 | 6/6 MATCH |
| 截图与度量 | `node .tmp/acceptance-shots.mjs` | 12 张 after-*，0 console error |
| DOM 度量 | `node .tmp/inspect.mjs`、`node .tmp/structure.mjs` | 见 `measurements-*.json` |
| 真实交互 | `node .tmp/interactions.mjs` + `node .tmp/select-all.mjs` | 19 项检查全部通过 |
| 目标 Playwright | `pnpm exec playwright test --config=.tmp/pw-chrome.config.ts <4 specs>` | **64 passed / 0 failed（3.2m）** |
| UI 门禁 + 主题色单测 | `cd modules/console/web && pnpm exec tsx --test ../tests/ui-restraint.test.ts ../tests/theme-colors.test.ts` | 17/17 pass；另用 15px 临时探针验证字号规则会真实失败（13 pass / 1 fail，删除探针后恢复） |

Playwright 说明：仓库未安装自带浏览器，`.tmp/pw-chrome.config.ts` 仅把 `use.channel` 设为 `chrome` 并按配置位置调整 webServer cwd，测试目标与断言未改。运行前记录了 `git status`；`console-settings-canonical.spec.ts` 覆写了 10 个受跟踪的 `docs/baselines/stellar-settings/existing-*.png`，**已仅恢复该目录**（`git checkout -- docs/baselines/stellar-settings/`），运行前后 git status 完全一致，生产改动未回退。

## 9. 未覆盖与限制

- 未复跑完整 `make test`（Go vet/staticcheck/go test）；本次无 Go 改动，Console 类型检查已由构建独立执行，目标 Playwright 与 UI 门禁已独立运行。
- 仅 Chrome；未验证 Safari/Firefox 与真机移动端。
- 未使用生产数据/生产库；交互测试会改变隔离 DB，`after-*.png` 均拍摄于破坏性交互之前。
- 设置保存条的 sticky 透明底在滚动中会与内容视觉重叠，但滚到底无永久遮挡，未判为缺陷。

## 10. 建议的收尾顺序

1. 修 D01（390 评论行）：窄屏折叠操作或允许操作区换行，保证信息列可读。
2. 修 D02：导读行不再重复分区标题。
3. 修 D06：移除 `.apple-link` 中覆盖字号的 `font: inherit`；把 `.settings-help` 提升为共享样式；给 `CanonicalSettingsNotice` 的 p 明确 13px，并把字号白名单检查下沉到渲染 DOM。
4. 补 D03：快捷链接 44px 命中区、评论勾选框包 44px label，并把它们加入 mobile-a11y 断言，避免回归。
5. D04/D05 单独记录为既有问题，避免混入本次视觉改版范围。
