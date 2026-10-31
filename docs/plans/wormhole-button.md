# 虫洞入口

任务编号：BLG-20261031-01。状态：实现中。

- 当前目标：仅在博客侧栏保留十年之约虫洞入口，移除页脚重复按钮，并连同用户确认的 4 个已有提交发布上线；点击在新标签打开 `https://www.foreverblog.cn/go.html`，保留站点地图、自定义页脚与署名。以下初始阶段记录保留历史决定，最新范围以文末发布节为准。
- 已确认：用户选择虫洞按钮；采用官方 32px 深浅色按钮，放在署名下方。沿用 Stellar 页脚结构、间距与语义 token，不添加脚本、挂件或配置字段。使用 `noopener` 隔离新窗口，同时保留来源站点信息供官方统计。
- 基线：`dev` / `84fe1baa13f89b5964ab06a1f40ccc5693066abe`；工作树 `/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261031-feat-blg-20261031-01-wormhole-button`；分支 `codex/feat/20261031-blg-20261031-01-wormhole-button`；本地合入目标 `dev`。根工作区已有文档与验收素材改动，均保留。
- 唯一布局参照：`docs/baselines/wormhole-button/reference-before.png`，2026-10-31 从线上首页页脚实拍；官方按钮来源：<https://www.foreverblog.cn/about.html#ab-8>。验收截图同目录。
- 验收：页脚仅新增一个可聚焦入口；官方按钮浅/深/自动配色清晰；card/glass/minimal/flat 四种外观在桌面及 390px 手机视口无横向溢出；新标签地址正确；现有页脚内容保留。
- 分工：主 agent 独占页脚 helper、辅助 CSS、记录和截图，负责浏览器与最终组合态核验；Luna explorer `footer_entry` 只读核查设置入口与现有行为，无文件写入，无独立服务/数据/端口。
- 验证资源：本任务工作树内隔离预览数据与配置；预览端口 `127.0.0.1:19831`。不使用根工作区数据库。适用检查：`make test`、实际构建、浏览器验收与加载资源指纹核对；`make merge-check` 会合入 `dev`，不作为只读检查。
- 发布边界：本次请求为本地添加与验证；推送、发布和生产部署分别记录，未经授权不执行。

## 执行与验证

- 页脚在自定义内容后追加按钮，不改已有 `theme.footer.content` 或数据库。样式通过现有客户端资源清单独立加载，不依赖文章扩展开关；保持四种外观最终样式层的位置。按钮使用浏览器原生懒加载，不依赖新增脚本。
- 初次 `make test` 暴露固定图片调用 nil 图标注册表的问题。保留原“无图标预览不输出图标 CSS”契约，仅为图片处理链加载局部内置失败图标；没有回写共享注册表。原失败和修复后定向检查均留有日志。
- 既有资源顺序测试的预期只新增 `site-footer.css`，没有删除/跳过测试或降低门槛。定向 `go test ./modules/stellar/web ./modules/stellar/routes ./modules/stellar/clientconfig` 和修复后的 `make test` 通过；原生懒加载 class 的最后调整已编译并通过以下浏览器核验，最终代码还会由提交 hook 和集成门禁再查。
- `make build` 生成真实 Console/User Center 产物；最终 Go 代码调整后再次 `make go-build`。服务以最终 embed 二进制启动，新增 CSS 的 HTTP SHA256 与源码完全相同，Console 入口引用的 6 个资源与构建目录 hash 一致。源码和二进制指纹见 [检查记录](../baselines/wormhole-button/checks-verification.json)。
- [浏览器记录](../baselines/wormhole-button/browser-verification.json)：真实本地服务，独立新建夹具库，card/glass/minimal/flat × 显式浅色/显式深色/自动随系统浅色/自动随系统深色 × 1280×800/390×844，共 32/32 通过；每组保留完整截图。外观通过隔离设置 API 保存；配色状态由开发工具临时切换 `html[data-theme]`，系统偏好由媒体模拟切换，不代表完整设置 UI 的用户旅程验收。已还原媒体和视口。
- 图片实际加载，始终只有一张对应配色图片显示，显示高 32px、点击区域不少于 44px、无横向溢出，署名与站点地图保留。键盘焦点为 2px 实线。回车在新标签打开官方虫洞，再实际跳转到 `https://jiangmiemie.com/`；原博客标签保留。官方“送出读者”计数未登录核验。
- 两轮独立只读 review 均无遗留实质问题。未修改共享主题源或生成主 CSS，因此 `make css` / `make css-drift-check` 不适用。

## 本地集成与收尾

- 代码提交 `b114e1d45664bb2a00c446f01c8db07593ee969d`（`feat: 在博客页脚添加十年之约虫洞入口`）已通过提交 hook 的最终 `make test`，并由 `make merge-check` 合入本地 `dev`；feature 与合并后 `dev` 的完整门禁均为退出码 0。
- 在合并后的根工作区重新 `make build` 成功，再从该 embed 二进制启动隔离预览、核对图片与 CSS/Console 资源指纹。最终源码与 32 组浏览器核验时的五份源码 SHA256、提交中的内容完全相同；组合态截图为 `docs/baselines/wormhole-button/combined-preview.png`，局部展示为同目录 `preview-footer.png`。
- 体验入口：<http://127.0.0.1:19831/>。运行根目录是主工作区 `.tmp/wormhole-button/runtime`，仅使用本任务的合成数据；服务 PID `85672`。该服务保留供体验，未使用主工作区原有数据库。预览状态为 card / 自动配色，浏览器临时媒体和设备模拟已清除。
- 原有 242 个已修改或未跟踪文件逐份 SHA256 保持不变。任务日志、脚本与合成夹具归档到主工作区 `.tmp/wormhole-button/worktree-verification-archive.tar.gz`，归档 hash 与条目检查见相邻 `archive-verification.json`；迁移后的夹具 SQLite 完整性检查为 `ok`。
- 原临时服务已停止；本任务 worktree 与已合并本地分支均已正常删除，`git worktree list` 仅保留根 `dev`。共享 `resource/` 和其他用户资料未清理。
- 此节及组合态证据是合并后补记，保存在根工作区，未为记录单独制造提交。远端 CI、推送、发布、线上部署与官方送出计数仍未执行/核验。

下一步：用户体验确认；线上更新沿仓库既有发布流程另行执行。

## 侧栏入口扩展

- 用户确认：在左侧栏底部、「设置」上方增加虫洞按钮，保留页脚入口，并查看桌面和手机效果。
- 基线：`b114e1d45664bb2a00c446f01c8db07593ee969d`；worktree `/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261031-feat-blg-20261031-01-wormhole-sidebar`；分支 `codex/feat/20261031-blg-20261031-01-wormhole-sidebar`；本地合入目标 `dev`。
- 唯一布局参照：`docs/baselines/wormhole-button/sidebar-before.jpg`，取自当前本地合成夹具页面；沿用既有 Stellar 侧栏和官方按钮，不改变其他布局或主 CSS。
- 验收：桌面展开时按钮在 footer actions 后、设置前；48px 收起栏沿用普通 widget 隐藏规则，不裁剪官方标志；手机抽屉展开显示，关闭时不可聚焦；保留页脚入口；四种 appearance 和浅/深/自动配色正常、无横向溢出；44px 以上点击区及清晰键盘焦点；链接策略与页脚一致。
- 分工：主 agent 负责源码、样式、记录和最终浏览器验收；Luna explorer `sidebar_fit` 定位入口及折叠规则，提供的实现草稿由主 agent 核对修订，无并发写入。
- 验证资源：沿用端口 `127.0.0.1:19831` 与主工作区 `.tmp/wormhole-button/runtime` 的本任务合成数据；构建时重启预览并验证 HTTP CSS 指纹。不连接主工作区原数据库，不推送或上线。

### 本轮验证

- 共享官方按钮 HTML 与配色/焦点样式，左栏使用无 `data-leftbar-rail` 的既有 widget wrapper；未改主 CSS、主题源、配置或数据结构。
- 当前源码 `make build`、`make test` 均退出 0；实际 HTTP CSS 与源码 SHA256 一致，Console 所引用资源也与真实构建目录逐一相同。检查见 `docs/baselines/wormhole-button/sidebar-checks-verification.json`。
- 四种 appearance × 显式浅/深和自动随系统浅/深 × 1280×800 桌面/390×844 手机抽屉，32 组均通过；另核验 16 次抽屉关闭 inert 与 4 次桌面收起隐藏。图片实际显示高 32px，按钮点击区 93.5×56px，位置在设置之前，无横向溢出。
- 键盘 Shift+Tab 可从设置移到虫洞，焦点 2px 实线；回车在独立新标签打开官方虫洞并实际跳至 `https://jaketao.com/blog`，原博客标签保留。官方送出计数未登录核验。
- 这是隔离合成夹具的样式与交互验收；配色通过开发工具模拟，未冒充生产页面或完整设置 UI 旅程。独立 review 未发现实质缺陷。
- 截图：`sidebar-card-dark-desktop.jpg`、`sidebar-card-light-desktop.jpg`、`sidebar-card-dark-mobile.jpg`；其余 appearance 与收起状态同目录。

本轮本地集成已完成，证据见下节；推送、远端 CI 和生产部署未执行。

### 侧栏本地集成

- 提交 `bc7e2da11c68346dbd48375efa7844a5126f5b90` 已通过最终提交 hook，并由 `make merge-check` 合入本地 `dev`；feature 和组合态完整门禁均退出 0。
- 合并后 `make build` 退出 0，以根工作区最终 embed 二进制重启合成夹具预览；HTTP CSS、6 个 Console 资源与矩阵验证/当前源码一致。`/readyz` 正常，PID `12003`，运行根目录仍为 `.tmp/wormhole-button/runtime`。
- 组合态再验桌面、手机打开/关闭抽屉，按钮位置、图片和 inert 正常；最终截图为 `sidebar-combined-desktop.jpg`、`sidebar-combined-mobile.jpg`。系统媒体和临时视口模拟已还原，card / 自动配色预览保留在 <http://127.0.0.1:19831/>。
- 任务日志与脚本已核验归档到 `.tmp/wormhole-button/sidebar-verification/worktree-verification-archive.tar.gz`。262 份既有文件与本轮快照 hash 相同；另有一份与本次无关的 Console 截图在执行期间变化，未触碰或回写。前阶段任务记录原件已单独保留，内容并入本次记录。
- 此节和组合态证据为合并后的补记，不为记录单独创建提交；远端 CI、推送、发布、生产部署和官方计数仍未执行。

- 本轮临时 worktree 与已合并本地分支已正常删除，`git worktree list` 仅保留根 `dev`；预览使用根工作区二进制和夹具，不依赖已删除路径。

下一步：用户查看本地效果，状态为待体验。

## 侧栏独立入口与线上发布（2026-10-31）

- 用户最新决定：删除页脚虫洞，仅保留侧栏；明确授权线上应用，并确认连同 dev 中已有的 Console 工作台、设置效果预览、标签插件等 4 个提交一起上线。允许本任务提交、推送、PR 晋级、tag 双产物发布及生产镜像升级。
- 本轮基线：`bc7e2da11c68346dbd48375efa7844a5126f5b90`；worktree `/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261031-fix-blg-20261031-01-wormhole-release`；分支 `codex/fix/20261031-blg-20261031-01-wormhole-release`；本地合入 `dev`，经 main/release PR 发布。
- 唯一 UI 参照：`docs/baselines/wormhole-button/sidebar-combined-desktop.jpg`；指定差异仅移除页脚按钮，保留署名、站点地图及侧栏既有位置、配色和折叠规则。验收为侧栏一个入口、页脚零入口，桌面/手机与四种外观正常。
- 目标补丁版本：`v2.0.3`（当前公开最新 v2.0.2）；发布前复核 tag 无冲突。无新增配置、数据库迁移或内容导入。生产切换前检查维护日志、固定镜像、完整数据备份并核对，使用线上数据隔离副本预演，保留旧版本回退。
- 主 agent 独占源码、记录、临时服务和发布/生产操作；独立 reviewer 只读检查历史部署助手的适用边界，不运行或操作服务器。
- 验证：make build、完整提交/集成门禁、lint 与发布凭据检查；本地侧栏/页脚浏览器核验；两次 PR 的各项 CI；同一 release tag 的二进制 Release 与 GHCR 镜像；镜像 -h、服务器隔离预演、生产健康/版本/资源指纹及真实公开页面验收。

- 本轮实际构建完成，HTTP CSS 与本次源码一致；四种 appearance × 四种浅深/自动状态 × 桌面/手机共 32 组检查通过，均为侧栏 1 个入口、页脚 0 个入口，署名保留。参见 `release-local-browser-verification.json` 与 `release-local-checks-verification.json`。
- 服务器当前已实查仍为 v2.0.2 / 镜像 `sha256:ccfccc57...e008`，readyz 200、39 张业务表、392 个受保护文件，无未完成维护任务，约 10.85GB 可用；本轮未改变 Compose、业务数据或配置。记录为 `release-server-preflight.json`。

### 发布门禁必要修复

- `a2df83589856b460877e06af066d706345cc2053` 已通过提交 hook 和 `make merge-check` 的 feature/dev 两轮完整测试，并推送 dev；晋级 PR 为 <https://github.com/SakuraLoveSmile/Sakura-Blog/pull/25>。
- PR #25 首轮 Go CI 在原有 govulncheck 门禁失败，本地使用同一 `govulncheck@v1.1.4 ./...` 复现。`golang.org/x/net v0.60.0` 的 GO-2026-6866 与 GO-2026-6860 分别经图片 HTML 渲染、导航和评论 HTML 解析可达，均由上游 v0.62.0 修复。
- 仅执行 `go get golang.org/x/net@v0.62.0`：x/crypto 升至 v0.58.0、x/sys v0.49.0、x/text v0.43.0，为新版本的最低依赖；依赖图的 x/tools v0.51.0、x/sync v0.24.0 校验随之记录。现有 Go 1.26.10 满足模块最低 Go 1.26.0，无配置、API 或数据结构变动。
- 相同漏洞扫描重跑退出 0：当前代码可达漏洞为 0；工具仍报告导入包/模块中存在当前代码未调用的漏洞，未修改或降低门禁。升级后的 `make build` 退出 0，独立只读 review 未发现差异或兼容问题；后续提交、集成及 PR CI 将绑定最终提交重新验证。
- `go mod verify` 为 `all modules verified`。从升级后真实 embed 二进制重启本任务隔离预览，34 项 HTTP/资源检查通过，Console/CSS 指纹对应当前产物；真实浏览器复查桌面和 390×844 手机抽屉，侧栏 1、页脚 0，图片可见、无横向溢出。设备模拟已还原，证据为 `release-security-checks-verification.json` 与 `release-security-browser-verification.json`。

### 组合 E2E 暴露的问题与修复

- 安全修复提交 `859d8751b64b10e6639afa9c50477c242490aeb3` 已通过 hook、feature/dev 两轮完整集成门禁并推送；该提交的 Go、lint、security、Console、build CI 均通过。前轮 E2E 有 7 项失败及 1 项重试后通过，随后在独立夹具完整复现并定位，未放行失败门禁。
- 六项设置布局矩阵因全页保留的隐藏表单增多，逐节点跨浏览器采样耗尽原 30 秒预算。改为两次 `evaluateAll` 全量采样，仍逐项检查全部匹配元素、六视口、16/28px 间距及背景/边框/阴影/圆角/溢出，保留非零守卫与原超时；增加活动表单就绪守卫。定向 11 项通过，矩阵各约 3 秒，未删减断言或仅筛可见元素。
- 图片预览失败是真实选文问题：冻结 trace 的 `page=post` 请求选到 06:17 的导入文章，却漏掉 06:21 的新建图片文章。独立 SQLite 副本证实旧文存为 `2026-10-31T06:17:58.61895Z`、新文为 `2026-10-31 06:21:16.131126 +0000 UTC`，原字符串排序将 `T` 提前。只在前台只读快照查询中统一日期/时间分隔符，保留既有秒级和 ID 倒序，不重写保存值。三场景回归先复现较旧导入与同秒 ID 错选，修复后连同原预览/历史日期用例通过。
- 本地完整组合还复现导航检查错误：`/console/plugins` 与 `/console/seo` 按已登记路由进入默认子页，原断言要求停在父入口。改为精确核对 `/console/plugins/shortcodes` 与 `/console/seo/overview`，继续遍历全部主导航、桌面/手机和全部布局/未捕获错误断言。
- 本地 E2E 使用本任务临时配置、独立 `127.0.0.1:19931` 和 `.tmp/e2e-server.*` 数据，不复用其他任务的 8096 服务。Playwright 对应 Chromium 下载连续中断后停止下载，使用已安装的 Chrome 跑真实浏览器检查；远端 CI 仍须使用其固定 Chromium 完整通过。首轮完整组合为 141 通过/2 失败，保留原日志；上述修复后的完整组合 143/143 通过（5.1 分钟），日志为 `.tmp/wormhole-release/ci-e2e-combined-after.log`。
- 独立只读 reviewer 对本轮 4 份源码/测试差异未发现实质问题；保留全部布局、导航及预览验收断言。提交 hook、feature/dev 组合门禁与本 PR 新一轮 CI 将绑定最终提交检查，生产尚未切换。
