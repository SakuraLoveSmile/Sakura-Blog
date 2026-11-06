# BLG-EDIT02：页面编辑与正式页面一致

- 状态：待体验。2026-11-06（Asia/Taipei）。用户在 BLG-FRIEND02 体验后新增页面编辑的四项修复要求，本记录独立维护该施工事项。已完成本地实现、构建、回归及受影响界面核验；本次用户已要求按流程发布新版本并更新线上，发布执行进展见下文。
- 目标：页面编辑不再显示 Stellar 2 原文配置入口；真实友链可加载；去掉 shortcode 预览外围额外底色；可编辑正文复用当前正式页面的实际主题、排版及友链页居中标题，并显示只读本站资料。
- 工作区：复用当前会话工作树 `/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261106-fix-blg-friend02-friends-page`，分支 `codex/fix/20261106-blg-friend02-friends-page`，实现起点 HEAD `2ad23a6225708675a7917cd718d5f92e399cee3d`。原因是此次验收直接依赖尚未集成的 BLG-FRIEND02 样式与本站资料，避免在两个工作树复制未提交的依赖；原任务已完成部分作为本任务输入，未迁移或改名。两项工作由主 agent 顺序接续，不并行运行另一份来源。根 dev 的用户改动及 editor-writing-assistant 工作树不操作。集成目标 dev。
- 唯一目标参照是同一代码与设置下的公开正式页面（本地 `http://127.0.0.1:18086/friends`，BLG-FRIEND02）；用户现状截图在 `docs/baselines/page-editor-wysiwyg/before-user.png`。保留页面编辑工具栏、基本表单、原始 Markdown 和未知配置，不换 Console 风格。

## 已冻结的实现与边界

1. 仅移除 PageEditor 的 StellarContentConfig 入口，保留 `stellar_config`、`blog_extensions`、基本控件同步、错误和旧版本迁移提示；其他编辑器共用组件不删除。
2. 友链是公开只读数据，仅两个现有公开 GET 友链接口支持无凭据的跨域读取；不开放后台、写接口或全站内容 CORS，不改变现有预览 iframe 的 `sandbox="allow-scripts"`。
3. 内嵌 shortcode 画布不注入不透明预览表面，组件库和独立预览保持原表面。去掉的是额外宿主底色；Note 蓝色及友链卡片本身沿正式页样式。
4. 保留 TipTap 实例、事务和无损 shortcode 管线；页面正文使用 app 自己创建的受控编辑画布，通过 Vue Teleport 承载同一 EditorContent，复用服务端完整预览提供的样式、theme/body 属性。编辑画布只接收样式和受控编辑 DOM，不执行返回的 HTML/脚本；动态组件继续使用原 opaque iframe 与写守卫。不得直接向 Console 全局引入正式页 CSS，也不得另抄一套近似标题样式。
5. 画布及嵌套组件 iframe 保持浏览器视口宽度作为媒体查询环境，正文宽度用实际编辑区可用宽度约束，裁去其余空白，防止桌面正文列宽误触发手机样式。必须实测选择、输入、组件编辑、切换、保存重开及媒体查询。
6. 本站信息从完整正文预览抽取后端生成的 `.friends-site-info`，以只读装饰放在交换分区末尾、下一 h1/h2 前。它不进入 TipTap 文档或 Markdown；没有交换标题不显示。保存原始配置与正文不因只读展示被改写。
7. 普通正文使用真实正式页 CSS，Console CSS 仅在组件操作控件内生效。代码块/表格以 NodeView 复用正式页容器，服务端高亮只提取有限 token class，且只在语言与正文匹配时应用；包装、行号及只读信息均不进入文档模型。空白页也通过安全预览占位取得主题，不写入正文。tight list 和表格的 TipTap 段落包装只做必要间距适配。

## 执行与归属

- T1 主 agent：PageEditor 原文入口及必要提示、公开友链 GET 边界/回归、权威记录和最终集成验证。独占 `modules/console/web/src/modules/pages/PageEditor.vue`、`modules/stellar/api/` 本次涉及文件、必要 Go 测试与说明。
- T2 Canvas worker：独占 `packages/editor/src/RichTextEditor.vue`、Console `components/RichTextEditor.vue`、`components/editor/StellarTagView.vue` 和新增编辑画布/装饰/相关测试文件；先完成可运行画布以便主 agent 尽早浏览器验证。包内只加宿主提供画布的可选接入，Post 等既有编辑器行为保持。
- T3 Preview worker：独占 `modules/console/web/src/modules/plugins/preview/preview-document.ts`、`usePluginPreview.ts` 及其相关测试。新增私有 `surface: standalone|embedded` 选项（缺省 standalone），NodeView 使用 embedded；提供 `withPreviewContentWidth(document, width)` 仅以有限正数设置正文宿主宽度，支持浏览器宽度的 iframe 嵌入较窄正文。此步骤与 T2 并行，主 agent 协调唯一写入者；均不再派生任务，不启服务。
- T4 主 agent：唯一浏览器/服务所有者；端口仍为 `127.0.0.1:18086`，数据库为 `.tmp/blg-friend02-runtime/data/blog.db` 隔离夹具，构建后重启再验收。不得使用生产数据验证保存。完成实际构建、make test 及完整用户路径；复杂画布改动再做独立只读审查。
- T5 Blocks worker：浏览器发现普通代码块和表格结构差异后，独占新增 `stellar-editor-blocks.ts` 及对应测试；Canvas worker 按明确的 plugin/key/extract 接口集成，主 agent 实测。独立 reviewer 只读核查画布信任边界和跨文档生命周期，不写文件、不启服务。

## 验收

- [x] 页面编辑无 Stellar 2 原文配置入口，基本控件、必要错误提示及未知配置保存路径保留。
- [x] 友链读取本地隔离真实资料，包含失联分组；不替换为组件库示例数据。
- [x] shortcode 外围多余底色消失，组件本体与正式页一致；组件库仍正常。
- [x] 同一主题/视口下，普通正文与正式页的字体、字号、行高、标题装饰及居中方式一致；无页面横向溢出。浅深/自动、四 appearance、1280/390 核验。
- [x] 可输入修改、属性编辑、选中与拖拽、查找/标题跳转、富文本与 Markdown 切换、保存重开；生成本站资料不写入原文，非友链页面不出现资料。指针自动化的具体限制见下文，不将不可靠点击计为通过。
- [x] 当前真实 SPA/Go 构建、必要回归、完整 make test 通过；保留浏览器截图与实际资源版本。
- [ ] 用户体验确认（仅用户明确确认后标已验收）。

预览及工作树在两项任务集成验证前保留；后续明确授权集成后统一停服并按项目保全规则清理。BLG-FRIEND02 的历史通过不能当成本任务证据。

## 当前核验记录

- 首轮真实构建并重启后，已实测友链/失联分组加载、标题使用正式页样式、属性输入与 Escape 关闭、富文本/Markdown 往返及保存重开。即时输入“验收”已保存并读回，随后通过原文编辑恢复夹具；本站信息未进入 Markdown。`save-roundtrip.json` 证明最终正文逐字与起点相同、配置语义不变（旧空 blog_extensions 按既有保存入口规范为 `{}`）。
- 独立审查发现空白页样式、跨文档拖拽结束、代码块/表格结构及旧测试 harness 缺口，已修正。原高度/source/token 断言保留，新增实际父窗口监听迁移与解绑验证；没有删除用例、降低门槛或跳过。新增代码及 CSS 层隔离也已独立复审，无新增可操作缺陷。
- 首轮 `make test` 因既有 SFC harness 未接入新增 import 失败，属于本次引入，已适配并重跑通过。最终完整检查为 Console 469、Stellar assets 229，加 Go vet、Staticcheck、全部 Go 测试；真实 `make build` 已通过并重启当前隔离服务。浏览器发现 Console reset 影响列表圆点后，将 Console CSS 限在组件操作控件内并置于明确层顺序，正式页 CSS 不受 reset 覆盖。
- 当前浏览器 32 场景（四 appearance × 浅/深/自动跟随系统浅/深 × 1280/390）全部通过；四个友链标题与正式页字体、字号、颜色、行高、对齐和装饰相同，组件 iframe 的额外表面透明，无页面横向溢出。`browser-matrix.json` 为度量证据，普通正文、列表、代码和表格的全部样式度量也相同，见 `ordinary-styles.json`。
- 查找高亮、侧栏标题跳转、空白页提示、属性面板 14px/20px 排版、代码行号更新/撤销、表格 Tab 新增行及序列化均已实际验证；只读资料及辅助 DOM 未进入 Markdown。离开编辑器后在同一 SPA 中换站点主题再返回，正文和友链均采用新主题，见 `theme-refresh.json`。
- 拖拽把手曾被正式页 article 的 overflow:hidden 裁掉，已仅在编辑画布放开 article 裁剪，保留外层横向裁剪；真实浏览器已核验拖出取消、连续两次移动和撤销，结束时 is-dragging 为 false，见 `interaction-final.json`。画布捕获 pointerdown 时提前无滚动聚焦真实控件；候选优先为直接控件、label.control、tabindex，正文仍由 PM 聚焦，背景只阻止默认聚焦、不阻止事件传播。针对性用例与独立复审通过。
- 指针自动化限制：最终补查中，工具遍历/对齐高 iframe 会在点击送达应用前滚动外层，日志中实际事件目标变成 IFRAME，不能拿这些点击证明产品属性按钮通过或失败。已以 Enter 打开真实属性面板、输入字段及 Escape 关闭核验功能；没有将无效坐标点击计为成功。临时诊断日志已完全移除，最终源码恢复为经过检查的实现。鼠标连续操作留给用户实际体验确认。
- 32 场景在全部样式改动完成后运行；后续仅完善焦点捕获。最终无诊断日志的实际构建再次通过标题/失联分组/本站信息、透明表面及 390 无横向溢出的核验，见 `final-smoke.json` 和最终截图。普通页面验收夹具已恢复，友链正文与最初基线逐字相同、stellar_config 相同，生成资料未写入正文。
- 最终版本：HEAD `2ad23a6225708675a7917cd718d5f92e399cee3d` 加本工作树未提交改动；`checks-verification.json` 保存逐文件 SHA256、实际 SPA/样式/脚本与磁盘一致的证据、二进制 hash 和检查结果。任务进行期间根 dev 已由其他工作推进到 `74ac06f087d0d64ed19bb7334a14d70f611a6f9c`；本次未重置或触碰其改动，后续授权集成前应按项目流程同步并验证。
- 体验入口：`http://127.0.0.1:18086/console/pages/edit/friends`；公开对照 `http://127.0.0.1:18086/friends`。保留 `blog` PID `69032`（exec session `5750`），仅监听本机 18086，运行根 `.tmp/blg-friend02-runtime`、隔离 DB `data/blog.db`。主题恢复为 minimal/dark，浏览器临时视口及系统配色模拟已清除，两个体验标签保留。未执行会实际合并的 `make merge-check`；未访问生产写接口、提交、合入、推送或发布。

## v2.0.9 发布与上线

- 用户明确要求“按照流程发布新版本，然后更新线上”，授权本任务必要的提交、集成、推送、两级 PR、正式 tag/镜像发布及生产更新。版本定为 `v2.0.9`；不把发布授权当作用户体验验收。
- 开始发布时，dev/origin/dev 为 `515ed89a487976a05536d182c1418f23bfeeadbb`，origin/main 为 `214c7009b64f594b97cec65f8ec2744f78c002cf`，origin/release 为 `7e930138dd73bebfdc52b235346fb934edcb55cc`。main 有 4 条 dev 缺少的发布合并记录，先在同一任务工作树建立临时同步分支，经 merge-check 回到 dev，再将实现分支 rebase dev 并重新验证；不操作其他活跃工作树。
- 生产只读预检：当前容器固定 `2.0.9-beta01`、running、RestartCount 0，宿主 `/readyz` 200。部署前保全当前 compose、配置、数据及内容，仅升级镜像，不改站点正文或友链资料。
- 当前下一步：提交本任务实现与既有验收证据，同步最新 dev，完成组合态门禁与真实构建，再执行 dev→main→release、Docker Release 和生产核验。最终结果与资源收尾继续记在本节。
