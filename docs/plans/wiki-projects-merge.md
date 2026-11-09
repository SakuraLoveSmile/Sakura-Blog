# BLG-20261109-04：Console「文库 + 项目管理」合并为单一「项目管理」入口

- 项目 / 任务编号：Blog / BLG-20261109-04
- 状态：待体验（已合入本地 `dev` `057e4ca1`；机械验证与浏览器验收通过，等用户体验确认）
- 创建日期（Asia/Taipei）：2026-11-09
- 权威记录：本文件；截图与浏览器证据见 [docs/baselines/wiki-projects-merge/](../baselines/wiki-projects-merge/)
- 主仓库 / 当前工作目录：`/Users/sakurasep/Documents/Code/Project/Personal Project/Blog`（根 `dev`，用户原有未提交的文档改动保留、未纳入本次提交）/ `/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261109-refactor-BLG-20261109-04-wiki-projects-merge`
- 基线 / 分支 / 合入目标：`dev` `9f8f1f9d`（rebase 基线）/ `codex/refactor/20261109-BLG-20261109-04-wiki-projects-merge`（提交 `057e4ca1`）/ 已合入本地 `dev`
- 运行资源：本机 Playwright 独立 8096 服务与隔离数据库（`.tmp/e2e-server.*`，测试结束即停止）

## 目标与范围

用户在 Console 侧栏看到「文库」和它的子项「项目管理」两个入口，认为应该合成一个，并以项目为主：**项目是主页，文档是项目下的二级内容**。

- 覆盖：Console 侧栏 manifest（Go `internal/module/core` 与前端 core plugin）、Console 路由、`WikiProjectsView.vue`、`WikiDocsView.vue`、受影响的 Go/前端行为测试与 Playwright 用例、`ri` 图标子集、本记录与基线。
- 明确排除：Wiki 后端 API 与数据表、`packages/api-client`、tygo 生成物、Stellar 前台的 `/wiki/...` 公开路由与「文库」文案、系统设置里的侧边栏组件条目（那是前台 widget 名称，不是 Console 导航）。

## 已确认决定

- 结构：**项目为主**，文档是项目二级内容；点侧栏默认落在**项目列表**。
- 命名：侧栏与两个页面标题统一叫「项目管理」，用标题旁的徽标区分当前项目，用页面内容区分文档/项目。
- 路由沿用现有地址族，不做大改名：
  - `/console/wiki` → 重定向 `/console/wiki/projects`（对齐既有的 `/plugins` → `/plugins/shortcodes` 约定，旧书签不失效）；
  - `/console/wiki/projects` 仍是项目列表（落地页）；
  - 文档工作台改为 `/console/wiki/:project/docs`，项目来自路由，不再用下拉框猜上下文。
- 文档页去掉「选择文库项目」下拉与「项目设置」按钮，改为项目徽标 +「返回项目」；切换项目走项目列表。
- 不做数据模型合并：`wiki_projects` / `wikidocs` 表结构与 API 完全不变。

## 实现与验收

| 步骤 | 用户效果 / 实现范围 | 验收情形与预期 | 当前结果及证据 | 状态 |
| --- | --- | --- | --- | --- |
| T1 | 侧栏「内容」组里原本的「文库（含子项项目管理）」变成单项「项目管理」，无展开箭头、无子项 | 登录后侧栏只有 1 个指向 `/console/wiki` 的入口，标签「项目管理」；Go manifest 与前端 core plugin 同源 | `TestCoreManifestMergesWikiIntoProjects` 通过；`module-plugin.test.ts` 断言 side nav 与路由；e2e 断言侧栏 `a[href="/console/wiki"]` 恰 1 个、`svg` 恰 1 个（无子项箭头）、无「文库」入口 | 已完成 |
| T2 | `/console/wiki` 落在项目列表；项目列表标题为「项目管理」 | 直接访问 `/console/wiki` → URL 变 `/console/wiki/projects`，标题「项目管理」，页头有「新建项目」 | e2e「浏览器加载当前 Console 产物」用例断言重定向与标题；`console-ui-consistency` 主导航逐页可达用例新增 `"/console/wiki": "/console/wiki/projects"` 后通过 | 已完成 |
| T3 | 从项目行或页头「编排文档」进入该项目文档工作台 | `/console/wiki/<project>/docs` 打开；页头标题「项目管理」+ 项目名徽标 +「导入文章 / 返回项目 / 新建文档」；无项目下拉 | e2e「项目管理：侧栏单入口…」用例逐项断言（含 `header.locator("select")` 数目为 0） | 已完成 |
| T4 | 访问不存在/无权限的项目给明确空态，不静默落到第一个项目 | 空态标题「项目不存在或无权限查看」，按钮「返回项目管理」回到列表 | e2e 用例第 6 段断言空态文案与返回；截图 `merged-docs-*` 同批产出 | 已完成 |
| T5 | 桌面 1280×800 与手机 390×844 两页都不横向溢出 | `documentElement.scrollWidth <= innerWidth`；四张基线截图归档 | e2e 用例第 7 段；截图见 `docs/baselines/wiki-projects-merge/merged-{projects,docs}-{1280,390}.png` | 已完成 |
| T6 | 既有导入/草稿/发布契约不回退 | 从文章导入、草稿不公开、重复导入标记「已导入」、回收站不进入候选全部照旧 | `wiki-import.spec.ts` 全 5 用例通过（含新增的合并用例） | 已完成 |

## 执行与验证记录

| 日期 | 代码版本 / 候选差异 | 环境与运行对象 | 命令或实际操作 | 结果 / 证据位置 |
| --- | --- | --- | --- | --- |
| 2026-11-09 | `1529867b` + 本任务改动（9 文件 + 基线） | 本机 feature worktree | `make test` | 通过：Console vue-tsc + node:test、Stellar assets-test、release-source、module-check、go vet、staticcheck、`go test ./...` 全绿 |
| 2026-11-09 | 同上 | 本机 feature worktree | `make build` | 通过：真实 Console + `/uc/` 产物与 Go 二进制构建成功 |
| 2026-11-09 | 同上 | 本机 Playwright（8096 隔离库，自动重建 Console + 启动 `-dev`） | `pnpm exec playwright test tests/e2e/wiki-import.spec.ts` | 5 passed（导入 4 项 + 合并验收） |
| 2026-11-09 | 同上 | 同上 | `pnpm exec playwright test tests/e2e/console-ui-consistency.spec.ts -g "Console 全部主导航在桌面和手机可达"` | 1 passed（含 `/console/wiki` 落地断言） |
| 2026-11-09 | 同上 | 同上 | 截图归档 | `docs/baselines/wiki-projects-merge/`：`reference-before.webp`（用户原截图）+ `merged-projects-{1280,390}.png` + `merged-docs-{1280,390}.png` + `browser-evidence.json` |
| 2026-11-09 | 同上 | 图标子集 | `node modules/stellar/web/scripts/gen-ri-subset.mjs` | 子集重排（`book-2-line` 仍被项目行图标使用，仅顺序变化）；`assets-test` 通过 |

## 提交与集成

| 提交 | 行为变化与原因 | 验证 | 合并 / 发布情况 |
| --- | --- | --- | --- |
| `057e4ca1` `refactor: 合并 Console 文库与项目管理为单一入口` | 侧栏两个入口合为一个「项目管理」；`/wiki` 重定向到项目列表；文档工作台改为 `/wiki/:project/docs` 并去掉项目下拉；新增项目不存在空态 | `make test`、`make build`、两个 Playwright spec、`make merge-check`（合入前后各跑一次全量门禁） | 已合入本地 `dev`（`057e4ca1`）；未推送、未发布 |

## 交付与恢复

- 体验入口：登录 Console → 侧栏「内容 → 项目管理」→ 项目列表；点某个项目行的「文档」或页头「编排文档」进入该项目的文档工作台。
- 旧地址兼容：`/console/wiki` 仍可打开（落到项目列表）；此前的「文库」入口不再存在。
- 恢复方式：本任务只改 Console 前端入口与路由，不改数据库；如需回退，Revert 本次提交即可，已存在的项目与文档数据不受影响。

## 追加：390px 项目行换行缺陷修复（BLG-20261109-05）

独立复核时在 390×844 实测到一处真实缺陷，同任务内修复（分支 `codex/fix/20261109-BLG-20261109-05-wiki-projects-merge-polish`）：

| 项 | 内容 |
| --- | --- |
| 现象 | 长 slug 的项目行里，「N 篇文档」被挤成逐字竖排：元信息列 176px 内 slug 占 141px、`·` 占 2px，文档数只剩 16px，折成 4 行、行高总计 64px |
| 根因 | 元信息行缺 `min-w-0`，slug 不截断；换成 390px 后列宽不足，我新增的「文档」按钮再压缩了一截可用宽度 |
| 修法 | 该行加 `min-w-0`；slug `truncate` 并把完整 id 放进 `title`；分隔点与「N 篇文档」`shrink-0 whitespace-nowrap` |
| 实测 | 修复前：文档数 span 高 64px、宽 16px；修复后：高 16px、宽 47px，整行元信息 16px 高 |
| 回归用例 | `wiki-import.spec.ts` 新增断言「项目行内文档数 span 高度 ≤ 20px」；去掉修复代码重跑，该断言实测报错（64px），负向验证有效 |

同时复核了「390 文档页空态看起来空白」的反馈，结论是**非缺陷**：空态卡片正常渲染（`选择左侧文档开始编排` 与两个按钮都在 DOM 内），只是首屏在折叠线以下（页头 140px + 概览 179px + 文档树卡片），标题 y≈793 而底部导航顶边 y≈787；内容区可滚动（scrollHeight 1136），滚到底标题移到 y≈501。该纵向密度合并前即存在，本次不改版式；若需要空态上移，另开任务。

### 追加执行记录

| 日期 | 运行对象 | 命令 | 结果 |
| --- | --- | --- | --- |
| 2026-11-09 | 本机 isolation 8096 | `pnpm exec playwright test tests/e2e/wiki-import.spec.ts` | 5 passed（含新增换行断言） |
| 2026-11-09 | 同上（去掉修复的负向对照） | 同上 | 断言按预期失败：「0 篇文档 被折行成竖排（高度 64px）」 |
| 2026-11-09 | feature worktree | `make test` | 通过 |
| 2026-11-09 | 同上 | `pnpm exec playwright test`（截图重生成） | `docs/baselines/wiki-projects-merge/` 四张截图 + 证据更新 |

## 发布与线上升级（v2.1.4）

用户 2026-11-09 22:00 左右确认体验通过并指示「发布新版本 然后更新线上」，据此执行完整发布流程。

| 步骤 | 内容 | 结果 |
| --- | --- | --- |
| 版本判定 | `v2.1.3` 已占用且不含本次改动（PR #67 只到 `9f8f1f9d`）；`origin/main` 内容已全在 dev（零差异） | 取 `v2.1.4` |
| dev → main | PR #69，8 项检查 | build/console/go/lint/security/e2e(21m50s) 全过；`image`、`release-source` 按 dev→main 规则跳过；合并 `8409b050` |
| main → release | PR #70，8 项检查含 `release-source`(14s) 与 `image` 预构建(2m0s) | 全过；合并 `70ea2a7a` |
| 签名 tag | `v2.1.4` → `70ea2a7a`，GPG key `979B6BCF…3F3AD989` | 签名验证 Good signature |
| 镜像发布 | `docker-release.yml` run `42631195212` | success 5m55s；digest `sha256:c4924099…330577`；GitHub Release 已建、0 资产、非预发布 |
| 镜像可启动 | 服务器 `docker run --rm <image> -h` | 退出码 0 |
| 生产升级 | 备份 614M（sha256 `702651dd…`）+ compose `2.1.3` → `2.1.4` + `up -d` | 容器 `bf3a0c04de86`，Up，restarts 0，镜像 digest 与 GHCR 一致 |
| 线上验收 | `/` `/console/` `/settings` `/wiki` | 全 200；`/readyz` 200；5 分钟内日志 0 错误 0 panic |
| 产物核对 | 线上入口 `assets/index-Bty0ZzDD.js` | 含 `wiki/projects` 重定向、`wiki/:project/docs` 路由、「项目管理」标签；「文库」出现 **0** 次 |

- 证据：`docs/baselines/release-2.1.4/release-verification.json`、`docs/baselines/release-2.1.4/production-verification.json`
- 回退：compose `image` 改回 `2.1.3` 并 `up -d`（镜像仍在服务器，本次无 schema 迁移）；数据回退用 `/home/ubuntu/blog-pre-v2.1.4/site-state-pre-2.1.4.tar.gz`
- 未验证：生产 Console 登录态下的完整交互（无生产账号密码）；Firefox/Safari 差异；公网域名入口（`site.yml` 的 `url` 为空，只验证了宿主 8081）
- 磁盘提示：升级后根盘剩余 2.2G（96%），`/home/ubuntu` 下有多份历史 staging/update 目录与悬空镜像未清理（属历史资源，本次未动）
- 状态：**待体验**（发布与升级已完成，等用户在生产确认）
