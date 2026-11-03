# AI 助手：合并「助手」与「知识库」标签

任务编号：BLG-20261103-02。状态：待体验（v2.0.7-beta05 已发布并升级生产，待用户体验确认）。

## 目标与范围

`/console/assistant` 当前有「助手」「知识库」「设置」三个标签。实测「助手」与「知识库」两个标签渲染的是同一套视图：左侧 AI 对话工作区 + 右侧知识库状态卡，切换标签只改变 URL `?tab=`，页面内容没有任何差异。用户反馈“助手和知识库有点重复了”，要求合并；随后用户授权发布新的 beta 版本并更新线上。

- 目标：AI 助手页把「助手」与「知识库」合并为一个「对话」视图；知识库状态、索引统计、重建入口与对话同屏呈现；「设置」保留为独立标签。发布 `v2.0.7-beta05` 并升级生产。
- 范围：`modules/console/web/src/modules/assistant/AssistantView.vue` 的标签与路由映射、`AiSettings.vue` 的一处指引文案、`modules/console/tests/assistant-view.test.ts` 结构测试、本计划与验收基线，以及 `v2.0.7-beta05` 发布与生产升级。
- 不涉及：RAG 后端、`/api/v1/rag/*` 契约、Stellar 前台问答、Console 其他页面、导航入口。

## 已确认决定

- 2026-11-03 用户原话“ai 助手这里的助手和知识库有点重复了 合并吧”。实测两标签 DOM 文本完全一致（`assistantTabEqualsKnowledgeTab: true`），按“删除重复的标签、保留统一对话视图”实现。
- 合并后标签栏为「对话」「设置」两项；「设置」仍承载 `blog_extensions.ai` canonical 配置，不并入对话视图。
- 旧链接兼容：`?tab=knowledge`、`?tab=assistant` 落到对话视图，`?tab=settings` 不变（`SettingsView` 的 `ai: { path: "/assistant", tab: "settings" }` 跳转继续有效）。

## 实现

- `AssistantTab` 收敛为 `"chat" | "settings"`；默认与历史 query 都映射到 `"chat"`，切回对话时从 URL 移除 `tab`。
- 标签栏改为「对话」「设置」；内容区条件由 `activeTab !== 'settings'` 改为 `activeTab === 'chat'`；页头「清空对话」「重建知识库」操作同样只在对话视图出现。
- 右侧「知识库状态」「使用说明与最佳实践」保持常驻，与对话同屏，不再作为第二套页面。
- `AiSettings` 保存说明由“请在「知识库」中点击「重建知识库」”改为“请回到「对话」标签点击「重建知识库」”。

## 验收场景

- [x] 桌面 1280×900：`/console/assistant` 标签栏只有「对话」「设置」，对话工作区与知识库状态同屏，页头保留「重建知识库」。
- [x] 移动端 390×844：无横向滚动，标签与对话/知识库面板可用。
- [x] `?tab=knowledge` 与 `?tab=assistant` 打开后落在对话视图；`?tab=settings` 打开 AI 设置。
- [x] `/console/settings` 的 AI 入口（`?tab=settings`）仍进入 AI 设置。
- [x] `make test` 通过；`make build` 产出可运行 Console。

## 验证记录

代码版本：分支 `codex/refactor/20261103-blg-20261103-02-assistant-tabs-merge`（基于 `dev` @ `ce8d7e6c`）的工作区，2026-11-03。

- 改动前实测（同一隔离 e2e 服务，`dev` @ `ce8d7e6c` 的 `dist`）：标签为「助手」「知识库」「设置」，两个标签的 `.assistant-layout` 文本完全一致，采集脚本输出 `assistantTabEqualsKnowledgeTab: true`；证据 `docs/baselines/assistant-tabs-merge/before-knowledge-tab-1280.png`。
- 改动后浏览器验收（本分支 `make console` 产物 + `scripts/e2e/run-server.sh`，`admin` 登录）：`tabs: ["对话","设置"] active: 对话`；`?tab=knowledge` 与 `?tab=assistant` 均 `active=对话` 且右侧知识库面板可见；「设置」标签 `settingsVisible: true`；移动端 `{"scrollWidth":390,"innerWidth":390}`。证据：`docs/baselines/assistant-tabs-merge/after-desktop-1280.png`、`after-settings-1280.png`、`after-mobile-390.png`。
- `make test`（`MAKE_TEST_EXIT=0`）：console 类型检查与行为测试 432 项全过（含新增 `assistant-view.test.ts` 2 项）；assets-test 241 项中 239 通过、2 项为 CSS 漂移检查跳过（既有行为）；部署/发布来源/模块边界检查、`go vet`、Staticcheck、`go test ./...` 全过。
- `make build`（`MAKE_BUILD_EXIT=0`）：Console 与 User Center 真实产物构建通过，Go 二进制编译通过。
- 未验证：真实 RAG 问答（隔离站点未配置对话/向量模型）、Safari/Firefox、真实触屏设备；本次未改后端与问答渲染。

## 发布记录（v2.0.7-beta05，2026-11-03）

- 用户指令：“发布新的 beta 版本 然后更新线上的版本”，据此发布 `v2.0.7-beta05` 并升级生产。
- 发布前同步：`origin/main` 的 `805d86df`（PR #42 合并提交，内容零差异）经临时 worktree 与 `make merge-check` 合回 `dev`（`c5c04f25`），本地 worktree 与分支已清理。
- 推送：`ce8d7e6c..c5c04f25 dev -> dev`。PR #44 `dev → main` 合并 `f5413146`；PR #45 `main → release` 合并 `1a106f7d`。两 PR 必需 CI 全部通过（PR #44：go 4m42s、lint 2m20s、security 3m31s、console 1m3s、e2e 13m43s、build 2m2s；PR #45 另含 release-source 11s、image 1m37s）。
- tag：`v2.0.7-beta05`（GPG 签名，密钥 `979B6BCF…`）打在 `1a106f7d`；Docker Release run `41605144627` success（08:41:25Z → 08:47:11Z）。
- 镜像：`ghcr.io/sakuralovesmile/sakura-blog:2.0.7-beta05` digest `sha256:c2afa6ddeb7add2d5410a262030746178466edf189d8050b965f0177eba046c6`；`sha-1a106f7` 同 digest；label `version=2.0.7-beta05`、`revision=1a106f7d…`、`linux/amd64`；服务器 `docker run --rm …:2.0.7-beta05 -h` 退出码 0。
- 稳定标签未移动：`latest` / `2` / `2.0` / `2.0.6` 仍为 `sha256:944df8d8…a734392`；`2.0.7` 未生成（404）；`2.0.7-beta04` digest 未变；GitHub Release `v2.0.7-beta05` 为 Pre-release，Latest 仍是 `v2.0.6`。
- 生产升级：升级前 `2.0.7-beta04`（image ID `1f6612a7…`）、running、restarts 0。备份 `/home/ubuntu/blog-release-v2.0.7-beta05/`（0700）保存 `blog.db` 三件套与 compose 副本，db md5 源与副本一致（`a0430d5f…`）；compose 仅 image 一行改为 `:2.0.7-beta05` 后 `pull && up -d`。升级后容器 `2.0.7-beta05`（image ID `c2afa6dd…` = GHCR digest）、running、restarts 0；宿主 `/readyz` 200（三次 0.8–1.0ms）、公网 `/readyz` 200（88.9ms）、公网 `/settings` 显示 `Blog v2.0.7-beta05`，公网首页/`/console/`/`/uc/` 200，启动日志错误行 0。
- Console 产物核对：公网入口 `index-DiGlT8fC.js` sha256 `ed6d6bcd…530d`、`AssistantView-RKaBL5H_.js` sha256 `a6223f84…6efa`，与本分支 `make build` 的 `modules/console/web/dist` 一致；chunk 含「对话」标签与「AI 助手分类」，不再含 `"知识库"` 标签按钮文案。
- 回退：compose image 改回 `:2.0.7-beta04` 并 `up -d`；本次无数据库 schema 迁移，备份仅作保护。
- 证据：`docs/baselines/assistant-tabs-merge/release-verification.json`（预检基线、CI、镜像、Release、生产升级与产物核对）。
- 未验证：生产 Console 登录态人工点选（无生产后台凭据），以实际下发 chunk 哈希与内容、HTTP 200 为上线证据；未演练 DB 恢复（无 schema 迁移）。


