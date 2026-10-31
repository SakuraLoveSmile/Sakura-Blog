# 虫洞入口

任务编号：BLG-20261031-01。状态：待体验。

- 目标：在博客现有页脚添加十年之约虫洞入口，点击在新标签打开 `https://www.foreverblog.cn/go.html`；保留站点地图、自定义页脚与署名。
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

下一步：本地提交与组合态集成核验，保留可体验预览。推送、远端 CI 和生产部署未执行。
