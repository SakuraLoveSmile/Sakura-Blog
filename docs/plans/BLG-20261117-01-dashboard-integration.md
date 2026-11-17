# BLG-20261117-01：仪表盘本地成果整合与测试发布

状态：待体验。用户于 2026-11-17 授权整合两项既有成果到 dev、发布测试版并更新现有线上实例。

## 范围与既有决定

- 集成 [BLG-20261110-05](BLG-20261110-05-dashboard-halo-grid.md) 的组件实例配置、快捷操作配置和叠放容器。
- 集成 [BLG-20261111-01](BLG-20261111-01-dashboard-card-controls.md) 的 C1–C5/C7 卡片控件改进，保留其既有视觉与移动端 44px 点击区域。
- 原任务 C6 不在本次新增实现范围；保留现有叠放结构，不另行设计。
- 不改公共 API、数据库 schema 或依赖；两份原计划仅增加历史记录续接指针。主 dev 现存 AGENTS、README 与截图改动保留。

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
- 原线上版本为 `2.1.9`；本次已发布 `v2.1.10-beta01` 并升级现有实例，见下方实际核验。

## 2026-11-17 组合态核验与修正

- 首轮 `make test` 通过：Console 592、Stellar 233，Go vet、Staticcheck、Go tests 全通过；真实 Console/UC SPA 与 Go 构建通过。ESLint 通过。
- 初次 24 项仪表盘 E2E 全通过（卡片控件20、配置3、统一尺寸1）。独立代码审查进一步发现并修复：热门文章配置8条的请求数量、旧无config布局的按档位默认、叠放自动切换丢失回复草稿。修复后重新实际构建并重启服务。
- 新增回归：旧无config大档按7/8/全部上限渲染（用足够条数的API夹具，当前recent接口仍固定最多5条，本次不改后端）；8秒轮播期间聚焦回复输入，8.5秒后草稿仍在。两项通过。热门文章配置8条会按limit请求并显示8条，刷新后保留；配置套件4项通过。热门测试首次因默认布局没有热门卡而超时，已明确添加卡片后通过，未改产品行为或降低断言。
- 主 agent 在18119独立数据库真实浏览器复验：快捷操作两个实例分别2/4入口，保存刷新后互不影响；叠放三个子卡切换有效且占一个网格位置；390px无横向溢出，card-link伪元素点击区50×44。截图见 `docs/baselines/dashboard-integration/mobile-independent.jpg`。设计器28px尺寸按钮是既有排除范围，保持原样。
- Stack使用KeepAlive保留子卡状态，容器内焦点或页面弹窗开启时暂停自动切换；未变动视觉、公共API或数据库。
- 功能提交及就绪等待修正均通过提交 hook 与 `make merge-check` 的 feature/dev 两次组合门禁；远端CI、Docker发布与线上核验见下方。
- 复审补齐热门文章无config的小/中/大默认2/4/5，并验证回复草稿手动切换到文章再切回评论仍保留。最新构建后7项配置/回归E2E全通过（35.9秒）。热门8条用按实际请求limit返回数据的夹具核验，旧5条请求会失败；回复草稿使用真实隔离API评论数据。

## PR #83 远端门禁修正

首轮 CI：Go、lint、安全扫描、Console、真实组合构建均通过；全量 E2E 269 通过、1 失败。失败为既有 `console-mobile-a11y.spec.ts:881`：文档 goto 到仪表盘后立即 evaluateAll，SPA 快捷入口尚未渲染，取得0个节点，尚未执行宽高断言。DashboardView 快捷入口实现和该用例本次原本均无变更；本地原用例连续3次通过，属于当前远端暴露的就绪时序缺口，具体加载耗时变化未单独测量。

修正仅在测量前等待首个快捷入口可见；保留入口数量>0及每个入口宽高≥44px全部断言，不跳过、不修改预期或门禁。修正后本地连续3次通过。主 agent 独立核对源码和日志确认修正符合原验收。后续重新执行提交门禁、merge-check及当前PR完整CI。

两个原工作树及本地来源分支已清理；完整/ignored恢复归档逐项与原现场比对3927+3933条目后删除，未删远端分支。集成树保留至发布收尾。


## 2026-11-17 合入、发布与线上核验

- 功能提交 `c9f233bd`；E2E 就绪等待修正 `ac864531`。两次均执行正式提交 hook（完整 `make test`）及 `make merge-check`，feature/dev 组合态通过。
- [PR #83](https://github.com/SakuraLoveSmile/Sakura-Blog/pull/83) 已合入 main；最终 [CI 44033599121](https://github.com/SakuraLoveSmile/Sakura-Blog/actions/runs/44033599121) 全绿，270 项 E2E 通过。
- [PR #84](https://github.com/SakuraLoveSmile/Sakura-Blog/pull/84) 已合入 release；[CI 44036852535](https://github.com/SakuraLoveSmile/Sakura-Blog/actions/runs/44036852535) 全部8项检查通过，包含270项 E2E 和镜像/来源检查。
- 签名 tag `v2.1.10-beta01` 位于 release 合并提交 `16842e228251b01d08f7f6b3ad4cc3486057d994`，GPG 签名验证通过；[Docker Release 44040602633](https://github.com/SakuraLoveSmile/Sakura-Blog/actions/runs/44040602633) 成功。[GitHub Release](https://github.com/SakuraLoveSmile/Sakura-Blog/releases/tag/v2.1.10-beta01) 为预发布、非草稿、无二进制资产；稳定版仍是 v2.1.9。
- 线上固定镜像已改为 `ghcr.io/sakuralovesmile/sakura-blog:2.1.10-beta01`。原生 Docker pull 完成后镜像 `-h` 退出0，镜像标签与实际容器均为 version `2.1.10-beta01`、revision `16842e228251b01d08f7f6b3ad4cc3486057d994`；镜像及运行 digest 均为 `sha256:ea87971ed46bf3b6e38bee887ee2452dec2bbfafb84d4fb65ebca2573dde46e2`。
- 更新前仅停止 Blog，归档 Compose、config、data、content；tar 清单读取及 SHA-256 完成后才改固定镜像并启动。服务器恢复目录 `/home/ubuntu/blog-release-v2.1.10-beta01-20261117-retry/`，`site-before.tar.gz` 约615 MiB，SHA-256 `65b29e9bee646c83e2e9869c8c8c7ce62cb777789b2f2913da1d213b43aa55d2`，含原 Compose，保留旧2.1.9镜像以回退。
- 激活后宿主 `127.0.0.1:8081/readyz` 与公网 `https://blog.xn--fhqths51enha.cn/readyz` 均 `{"ok":true}`；容器 running、restartCount=0。启动瞬间两次连接重置后在既定 readiness 循环内通过。
- 公网 Console 从实际 HTML 入口递归读取10份 Dashboard/registry/布局/设计器资源，核对叠放容器、按卡片大小、组件设置、回复内容均已包含。完整 URL 与 SHA-256 见 [production-verification.json](../baselines/dashboard-integration/production-verification.json)。生产认证页面未登录复验；真实交互已在本地隔离数据库与当前CI验证，用户观感仍待体验。

### 镜像传输异常与处理

服务器原生 pull 首次失败：Docker 使用的 `127.0.0.1:7890` 代理拒绝连接，现存 mihomo 容器重启中。未修改全局代理、未重启共享 Docker。临时 Crane 方案因服务器下载过慢超时而未完成，未解压/运行/导入；Mac 下载的官方 Linux archive 已核验 SHA-256，但最终未使用。

实际成功路径为：验证 Mac 现有7890代理可访问 GHCR，再开单次 SSH 反向隧道，将服务器127.0.0.1:7890接到Mac现有代理；原生 Docker pull 成功后立即关闭隧道，复查服务器7890无监听。首次失败目录保留现场，正式停服整站备份使用独立 `-retry` 目录，未覆盖归档。

### 收尾与恢复

两个原工作树及已合并本地来源分支已删除；未删远端分支。原成果与非缓存忽略文件在 `outputs/blog-worktree-preservation/` 完整归档，逐项比对3927+3933条目、tracked差异及链接目标后才删除。原历史计划顶部已增加本记录指针，保留原验收细节。

本记录随发布证据提交到 dev；仅文档续接，不再重发产品版。集成工作树收尾前停掉本任务18118/18119隔离服务，保全`.tmp`脚本/取证/隔离测试数据库，核对提交已在dev后正常移除并删除已合并本地集成分支；主dev工作区的用户改动保持原样。恢复资料位于本次清理工作目录 `/Users/sakurasep/Documents/Codex/2026-11-17/pai/outputs/`。
