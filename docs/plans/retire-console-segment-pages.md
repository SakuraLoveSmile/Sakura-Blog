# BLG-20261106-02：Console 旧分段页面退役与侧栏入口清理

- 项目 / 任务编号：Blog / BLG-20261106-02
- 状态：待体验（v2.0.9-beta01 已发布并升级线上，等用户体验确认）
- 创建日期（Asia/Taipei）：2026-11-06
- 权威记录：本文件；发布与上线证据见 [docs/baselines/retire-console-segment-pages/](../baselines/retire-console-segment-pages/)
- 主仓库 / 当前工作目录：`/Users/sakurasep/Documents/Code/Project/Personal Project/Blog`（根 `dev` 工作树，用户原有未提交的文档改动全部保留、未纳入本次提交）/ `/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261106-fix-blg-20261106-02-retire-segment-pages`
- 基线 / 分支 / 合入目标：`dev` / `2ad23a62`（= `origin/dev`）/ `codex/fix/20261106-blg-20261106-02-retire-segment-pages` / 本地 `dev`
- 当前 HEAD / 未提交差异或快照：功能提交 `218cfdab`（已合入 `dev` 并推送）；发布晋级 `dev → main`（#51，`2e1058f4`）→ release 历史同步（#52，`214c7009`）→ `main → release`（#53，`7e930138`）
- 运行资源：本机 Playwright 使用 8096（已随测试结束停止）；生产为 `blog-prod` 的 `blog` 容器（compose 目录 `/opt/1panel/docker/compose/blog`，宿主端口 8081）。

## 目标与范围

用户在 Console 里不再遇到“死胡同”式的旧配置页：侧栏不再挂「主题设置」「菜单」「作者」三个空壳入口；旧地址（含旧书签、旧通知里的链接）仍能打开，并直接落到系统设置对应页。

- 覆盖：Console 侧栏 manifest（Go `internal/module/core` 与前端 core plugin）、`/console/appearance`、`/console/menus`、`/console/authors` 三个路由、对应三个空壳页面组件、受影响的 Console 行为测试与 Playwright 用例、`CLAUDE.md` Console 章节。
- 明确排除：系统设置页的内部信息架构与字段（保持 settings-ia 已验收的 5 组 12 页）；`/console/friends` 页内嵌的友链设置提示（不是侧栏入口，属友链页内部切换）；`modules/console/web/src/modules/appearance/` 下未被引用的旧子表单组件（本次不顺手删，避免扩大改动面）；后端旧分段接口与迁移读取路径（保持只读兼容）。

## 已确认决定

- 用户 2026-11-06 在诊断确认后指示：“确认改进 改进完成按照项目工作流发布新的测试版本，然后再更新线上”。据此授权：本地实现 → `dev` 合入 → 发布测试版本（预发布 tag）→ 生产升级。
- 保留旧地址为**重定向兼容层**而不是 404：旧书签与旧通知里的链接仍可用，重定向目标沿用设置页的页面键（`appearance`、`navigation`、`community`），旧 `/console/appearance?tab=feedback` 仍落到访客反馈页。
- 不保留空壳页面组件：页面已无任何可保存内容，保留组件等于继续保留死胡同；重定向映射集中到 `modules/console/web/src/router/retired-pages.ts`，并有单测锁定目标必须是已注册的设置页。
- 不改后端旧分段门禁（`CheckLegacySectionWrite` 继续只读兼容历史迁移预览）；本次只改 Console 入口与路由。
- 版本语义：测试版本用预发布 `v2.0.9-beta01`（比 `2.0.8` 新、比将来的 `2.0.9` 旧），只发布自身完整标签与 `sha-` 标签，不移动 `latest` / `2` / `2.0`。生产 compose 固定到该测试版本，回退只需改回 `:2.0.8`。

## 实现与验收

| 步骤 | 用户效果 / 实现范围 | 验收情形与预期 | 当前结果及证据 | 状态 |
| --- | --- | --- | --- | --- |
| T1 | 侧栏不再出现「主题设置」「菜单」（“外观”分组随之消失）与「作者」入口 | 登录后侧栏无这三个 href；Go manifest 与前端 core plugin 一致 | Go `TestCoreManifestRetiresLegacySegmentPages` / `TestCoreManifestContainsSettingsNav` 通过；e2e 断言侧栏无这三个地址；本机全量 e2e 206 passed | 已完成 |
| T2 | 旧地址 `/console/{appearance,menus,authors}` 直接落到系统设置的对应页 | `/console/menus` → `?tab=navigation`；`/console/appearance` → `?tab=appearance`；`/console/authors` → `?tab=community`；`/console/appearance?tab=feedback` → `?tab=feedback` | e2e“旧分段页面重定向到系统设置”用例通过（1280 桌面，逐条断言 URL 与页面标题） | 已完成 |
| T3 | 删除三个空壳页面组件，旧页面样式不再参与构建 | 仓库内无 `AppearanceView.vue`、`MenuView.vue`、`AuthorsView.vue`；`make test` 通过 | `make test` 通过（含 vue-tsc）；构建产物不再有对应 chunk | 已完成 |
| T4 | 测试反映新契约：旧入口必须不存在、旧地址必须重定向、重定向目标必须是已注册设置页 | `make test` 全绿（新增 `retired-pages.test.ts`）；相关 e2e 改为断言重定向与侧栏缺席 | `make test` 通过、`pnpm run e2e` 全量 206 passed（10.8m）；`ri` 子集重新生成 | 已完成 |
| T5 | 发布测试版本 | 预发布 tag 触发 Docker Release success；镜像可启动；`latest`/`2`/`2.0`/`2.0.8` digest 不变 | tag `v2.0.9-beta01`（签名）→ run `42196838286` success（4m33s）；镜像 digest `sha256:07287aee…`；稳定标签 digest 与发布前一致；GitHub Release 为 Pre-release、非 Latest、0 资产；服务器 `docker run --rm … -h` 退出码 0 | 已完成 |
| T6 | 生产升级到测试版本 | compose 固定新测试版本；容器 running、`/readyz` 200、Console 版本一致；记录备份与回退 | 冷停备份后切换镜像并 `up -d`：容器 running、restarts 0、镜像 ID 与 GHCR digest 一致；宿主/公网 `/readyz` 200；`/settings` 显示 `Blog v2.0.9-beta01`；备份 `/home/ubuntu/blog-release-v2.0.9-beta01`（db md5 与源一致）；无 schema 迁移 | 已完成（待用户体验） |

## 执行与验证记录

| 日期 | 代码版本 / 候选差异 | 环境与运行对象 | 命令或实际操作 | 结果 / 证据位置 |
| --- | --- | --- | --- | --- |
| 2026-11-06 | `2ad23a62` + 本任务改动（16 文件） | 本机 feature worktree | `make test` | 通过（Console 458 测试、Go 全部包、assets/module/release-source 检查） |
| 2026-11-06 | 同上 | 本机 Playwright（8096 隔离库） | `pnpm run e2e` | 206 passed，10.8m；覆盖重定向、侧栏缺席、主导航逐页可达、字号与可访问性用例 |
| 2026-11-06 | `218cfdab` | 本机 feature → 根 dev | `make merge-check` | 合入 dev（快进）并复验通过；推送 `origin/dev` |
| 2026-11-06 | `74ac06f0`（含 release 历史） | PR #51 `dev → main` | CI 全绿（e2e 15m3s） | 合并 `2e1058f4` |
| 2026-11-06 | `codex/chore/…-sync-release` | PR #52（release 历史同步，内容零差异） | CI 全绿（e2e 21m0s） | 合并 `214c7009`；`main..release` 归零 |
| 2026-11-06 | `214c7009` | PR #53 `main → release` | release-source 11s、image 1m51s、e2e 17m9s 等 8 项全过 | 合并 `7e930138`（内容与 main 相同） |
| 2026-11-06 | tag `v2.0.9-beta01` | GitHub Actions `docker-release.yml` | tag 推送 → 镜像构建 | run `42196838286` success，4m33s；GitHub Release 标记 Pre-release |
| 2026-11-06 | 生产 `blog-prod` | 冷停备份 + 固定镜像升级 | `docker compose stop blog` → 备份 → 改 compose image → `up -d` | 见 `docs/baselines/retire-console-segment-pages/deploy-verification.json`；生产 Console 入口 chunk 与本地生产构建 sha256 逐字一致（`52d5eb24…`） |

## 提交与集成

| 提交 | 行为变化与原因 | 验证 | 合并 / 发布情况 |
| --- | --- | --- | --- |
| `218cfdab` refactor: 退役 Console 旧分段页面并清理侧栏死胡同入口 | 侧栏移除 3 个空壳入口；旧地址重定向到系统设置；删除 3 个页面组件；新增重定向映射与测试；重新生成 ri 图标子集 | `make test`、全量 e2e、merge-check 组合态 | 已合入 dev 并推送；经 PR #51/#52/#53 进入 release，tag `v2.0.9-beta01` |
| 待提交（docs） | 本记录与发布/上线证据 | pre-commit-check | 发布后追加到 dev |

## 交付与恢复

- 体验入口和操作步骤：登录 Console 确认侧栏不再有「主题设置」「菜单」「作者」；地址栏打开 `/console/menus` 应落到「导航与页脚」；打开 `/console/appearance?tab=feedback` 应落到「访客反馈」。线上当前版本为 `v2.0.9-beta01`。
- 尚未完成 / 尚未验证：Console 登录态的人工操作（无生产后台凭据，交互由同一构建的 e2e 覆盖）；旧书签在真实浏览器中的重定向（本机 e2e 已覆盖同一构建，未用生产账号复验）；预发布的更新日志不会出现在 Console「检查更新」（读取 `/releases/latest`，预发布不参与，与 v2.0.7-beta01 相同）。
- 用户确认（明确确认后才改为已验收）：待用户体验确认。
- 下一步和恢复任务时首先核对的内容：`git worktree list`、`dev` HEAD、`v2.0.9-beta01` 镜像与生产 compose、`docs/baselines/retire-console-segment-pages/`。
- 需要时的回退 / 数据保留方式：镜像回退把 compose `image` 改回 `ghcr.io/sakuralovesmile/sakura-blog:2.0.8` 并 `docker compose up -d`；本次无数据库 schema 变更，数据回退用备份 `/home/ubuntu/blog-release-v2.0.9-beta01/blog.db`（先停容器并移除 WAL/SHM）。
- 本任务 worktree 收尾：功能 worktree 与本地分支、release 同步 worktree 与本地分支已按规范清理；远端分支不删除。

## 遗留与相近问题

- `modules/console/web/src/modules/appearance/` 下 `AssetsSettings.vue`、`LayoutSettings.vue`、`EnhancementsSettings.vue` 已无引用（旧主题设置子表单），本次未删，避免扩大范围；后续可单独立项清理。
- 友链页内「设置」标签仍是 `CanonicalSettingsNotice` 提示（友链 RSS/巡检已并入「设置 → 作者与友链」），属页面内切换而非侧栏死胡同，本次按范围保留。
