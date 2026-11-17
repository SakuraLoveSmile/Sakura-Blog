# BLG-20261117-01：仪表盘本地成果整合与测试发布

状态：实现中。用户于 2026-11-17 授权整合两项既有成果到 dev、发布测试版并更新现有线上实例。

## 范围与既有决定

- 集成 [BLG-20261110-05](BLG-20261110-05-dashboard-halo-grid.md) 的组件实例配置、快捷操作配置和叠放容器。
- 集成 [BLG-20261111-01](BLG-20261111-01-dashboard-card-controls.md) 的 C1–C5/C7 卡片控件改进，保留其既有视觉与移动端 44px 点击区域。
- 原任务 C6 不在本次新增实现范围；保留现有叠放结构，不另行设计。
- 不改公共 API、数据库 schema、依赖或其他任务文档。主 dev 现存 AGENTS、README 与截图改动保留。

## 工作树与保全

候选分支：`codex/feat/20261117-blg-20261117-01-dashboard-integration`，以 dev `623648f5` 为基线，先同步 origin/main 中缺少的历史，再三方应用两个原工作树的 HEAD 增量。

两个原工作树的全部 tracked 文件、未忽略未跟踪文件和 8 份 SQLite 数据文件已保存到 `/Users/sakurasep/Documents/Codex/2026-11-17/pai/outputs/blog-worktree-preservation/`；每个 tar.gz 对应 JSON 清单、SHA-256 与增量 patch。归档已读取清单核对，收尾前另核验数据库字节与当前原文件一致。

## 验收与发布

1. 组合态 `make test`、真实 SPA 与 Go `make build`；复用组件配置、卡片控件、统一足迹 E2E，在 18118 独立临时数据库运行，主 agent 独立核验真实浏览器。
2. 正式提交只含本次功能与两项原任务证据；`make merge-check` 合入 dev 并复验。
3. 推送 dev，dev → main → release 正常 PR/CI；拟用 `v2.1.10-beta01`（以实际发布时空闲 tag 为准）。Docker Release 成功、镜像 `-h` 与 GitHub 预发布都核验。
4. `blog-prod` 当前固定 `2.1.9`，备份 Compose、数据与内容后升级固定 beta 镜像；核对宿主及 `https://blog.xn--fhqths51enha.cn` readyz、版本与当前 Console 资源。
5. 原任务工作树在成果进入目标、验证通过、忽略数据已保全且没有运行任务后移除；不删除远端分支。

## 当前证据

- 两组 patch 在当前 dev 上三方应用全部无冲突。
- 最新远端正式发布 `v2.1.9`，Docker Release 成功；线上容器镜像 `2.1.9`。
- 本次验证与发布结果在执行后补充；历史通过不当作本次结果。

## 2026-11-17 组合态核验与修正

- 首轮 `make test` 通过：Console 592、Stellar 233，Go vet、Staticcheck、Go tests 全通过；真实 Console/UC SPA 与 Go 构建通过。ESLint 通过。
- 初次 24 项仪表盘 E2E 全通过（卡片控件20、配置3、统一尺寸1）。独立代码审查进一步发现并修复：热门文章配置8条的请求数量、旧无config布局的按档位默认、叠放自动切换丢失回复草稿。修复后重新实际构建并重启服务。
- 新增回归：旧无config大档按7/8/全部上限渲染（用足够条数的API夹具，当前recent接口仍固定最多5条，本次不改后端）；8秒轮播期间聚焦回复输入，8.5秒后草稿仍在。两项通过。热门文章配置8条会按limit请求并显示8条，刷新后保留；配置套件4项通过。热门测试首次因默认布局没有热门卡而超时，已明确添加卡片后通过，未改产品行为或降低断言。
- 主 agent 在18119独立数据库真实浏览器复验：快捷操作两个实例分别2/4入口，保存刷新后互不影响；叠放三个子卡切换有效且占一个网格位置；390px无横向溢出，card-link伪元素点击区50×44。截图见 `docs/baselines/dashboard-integration/mobile-independent.jpg`。设计器28px尺寸按钮是既有排除范围，保持原样。
- Stack使用KeepAlive保留子卡状态，容器内焦点或页面弹窗开启时暂停自动切换；未变动视觉、公共API或数据库。
- 仍需当前版本提交门禁、merge-check组合复验、远端CI、Docker发布与线上核验。
- 复审补齐热门文章无config的小/中/大默认2/4/5，并验证回复草稿手动切换到文章再切回评论仍保留。最新构建后7项配置/回归E2E全通过（35.9秒）。热门8条用按实际请求limit返回数据的夹具核验，旧5条请求会失败；回复草稿使用真实隔离API评论数据。
