# Stellar 2.0.0 完整移植

状态：实现中。此文替代此前只覆盖 CSS、骨架与旧功能兼容的升级范围；不将此前测试通过当作本次完整移植通过。

## 2026-10-24 前台差异修复（BLOG20261024-STELLAR）

- 用户目标：调查当前前台与 Stellar 2.0 的差异，修复实际缺口。沿用本文 T2/T3/T4/T5/T7 及已确认的 Go SSR、原生评论、四外观和三配色选择；真实数据、生产和远端操作不在本次修改范围。
- 状态：待体验（本次差异修复；原完整移植清单仍实现中）。起点 `9651820b3e6b922e643d5354d60b1ab1bfe1477e`，合入目标为根目录 `dev`。任务分支 `codex/fix/20261024-blog20261024-stellar-frontend-parity`，worktree `/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261024-fix-blog20261024-stellar-frontend-parity`。
- 根目录已有 AGENTS/README/历史验收证据改动及 `.pi/` 原样保留；当前任务树的 AGENTS 是根目录规则的本地副本，不纳入功能提交。共享 resource 是绝对软链接，主题工作树仍为 1.44.0，仅用锁定对象 `9d11978a24fe6610e3a7dbab741cf62634539d05` 作对照，不切换或重置它。
- 唯一整体视觉参照：`docs/baselines/stellar-200-parity-20261024/reference-upstream-home.png`，复制自本文原先选定的 `stellar-200-full/upstream-home.png`；这是历史上游截图，当前差异的行为/DOM 依据仍为锁定 2.0.0 源码。保留上游 token、字体、布局与四预设；不换肤、不把该站个人配置当默认配置。
- 运行资源：仅用本任务独立 config/content/data/uploads/cache；验收后迁至根工作区 `.tmp/stellar-parity-20261024-root/`，端口 8124，模块软链接指向根 dev；不 seed 或迁移用户数据库。浏览器证据写入 `docs/baselines/stellar-200-parity-20261024/`。临时树仅保留到本地合并门禁通过，补丁和日志归档在根 `.tmp/stellar-parity-20261024-archive/`，不会删除共享 resource 或其他任务树。
- 实际并行调查：`audit_templates`（Sol/high analyst）只读核查页面 DOM/布局与聚合展示；`audit_runtime`（Sol/high analyst）只读核查运行时/搜索/抽屉/外观接线。均不派生子 agent、不写文件、不启动服务。主 agent 固定视觉差异、实现接入、独占本记录及共享文件，并亲自核验最终页面。
- 验收：先保存能复现差异的当前页面；逐项修复实际确认的缺口，桌面/中间断点/手机核对受影响路由、导航、内容及布局；覆盖四 appearance × auto/light/dark 与受影响交互；执行适用定向检查、真实构建、必需本地门禁和 dev 组合态验证。未运行与历史证据单列，不以旧截图或子任务自报代替当前结果。
- 本次确认并修复的差异：
  1. 置顶轮播以前只出现在首页第一页，分类/标签/归档/专栏索引及后续页缺失；改为列表导航共享全站公开置顶，保留密码文章排除规则，关闭导航时仍独立显示轮播。分页源按日期保留置顶，首页模板去重；平铺首页可显示超过 `per_page` 的全部置顶。
  2. 分页以前直接输出所有页码，手机下一页被裁切；改用 Hexo 默认的首尾各一页、当前左右各两页和省略号。窄屏只收紧数字间距，并以高于 minimal 尾层的选择器优先级保持生效，不改全局 token 和生成 CSS。
  3. 新搜索 dialog 没有加载本地搜索补充样式，旧选择器又绑定旧 id；恢复按需加载、改用共享类名，使结果类型按钮横向排列并支持溢出滚动。
  4. Markdown 实际输出 Chroma，页面却只加载 Highlight.js 主题，导致 token 同色；恢复本地 Chroma 样式，映射锁定上游 token 颜色，限制旧容器规则，保留新版 gutter/padding。
  5. 本地口令页漏接原有专用样式；按页面加载并标记为 page style，不改密码校验、权限和 API。
- 当前候选证据绑定 `candidate-code.json` 的 `codeDigest=25c6e8164ce0c3bfb7c3f08f01bac6391e7ae3341aa87c29bd4931980d1811dd`。8 个业务源文件分别记录 SHA-256；最终二进制以非 `-dev` 模式启动，5 份本地样式的 HTTP 字节与源文件一致。Console 真实构建已完成；本次没有改 Console UI。
- 本次实测：定向 Go routes/web/clientconfig 回归通过；新增置顶/分页用例保存先红后绿；`make build`、`make css`、`make css-drift-check` 通过且生成 CSS 无变化；完整 `make test` 已通过一次，最后提交/合并仍执行仓库强制门禁。无删除、跳过用例或降低检查。原 list+pins 数量预期改为分页源保留 pin、模板去重的实际契约，资源顺序预期增加本次接回的样式。
- 主 agent 浏览器实测：分类页全站 3 篇轮播、第二页轮播；平铺首页在 `per_page=2` 时 3 篇置顶各出现一次；代码关键词紫色/字符串绿色；搜索类型切换为空页面结果再回到 20 条文章结果、Escape 恢复搜索按钮焦点；错误口令提示与重试后正确解锁正文。四外观 × auto/light/dark × 1280/800/390 共 36 组，加 4 组系统浅色跟随，共 40 组，均无横向溢出。另核对 320px 双省略号中间页；minimal 的 next 右边界为 295（320px）/365（390px），均在容器内。
- 独立审查：主体未发现实质问题；补查发现 minimal 分页间距被尾层覆盖，修复后再次审查关闭该 P2。浏览器截图与 DOM 数值、外观矩阵保存在本次基线目录。截图工具部分原始输出是 JPEG，已按原像素转换为正确 PNG 格式；桌面最终截图保留原生 JPEG，宽 1272 对应 CSS viewport 1280 的滚动条槽位。
- 本地组合态曾失败一次：`dea717b` 合入 dev 后，既有 CSS 全量重编回归在 Node → BSD tar 的 stdin 管道出现 `EPIPE`（tar 自身退出 0）。只读诊断用同一合法归档的零填充复现此机制；修正为先将锁定归档写入独占临时文件、再从文件解包，错误继续抛出，finally 清理不变。未忽略 EPIPE、未改上游或降低检查；构建修正的 SHA-256 单列在 `candidate-code.json.buildSourceFiles`。
- 修正后根 dev 候选（HEAD `dea717b` 加上述可核验构建补丁）实际 `make test` 通过：Console 298/298、Stellar 220/220，0 失败/跳过，Go vet/Staticcheck/Go tests 均通过；根 `make build` 通过。`dev-verification.json` 绑定 8 个运行源文件、构建补丁、二进制和资源摘要，5 份前台样式及 2 份 Console 入口资产的 HTTP 字节与当前产物一致。主 agent 再次实际操作根构建的首页→分类、标签/归档/专栏/第二页轮播、代码→搜索、320px 双省略号分页和口令解锁，均通过。正式合入仍执行 `make merge-check` 的 feature 与合并后 dev 全门禁，最终日志保留在本地归档。
- 体验入口：`http://127.0.0.1:8124/`（隔离样例），口令样例 `/post/parity-locked` 使用本任务测试口令 `fixture-unlock`。下一步由用户确认本次修复体验；未得到确认前不标已验收。本次范围不代表原 T1–T7 全部能力已验收；真实外部服务、真实数据迁移、生产体验、远端 CI、推送、发布与部署未运行。

### T2/T7 颜色对齐补验（同任务，2026-10-24）

- 用户已选择“与之前上游参照站一致：minimal 暖纸色”。沿用 `BLOG20261024-STELLAR`，本轮状态待体验；本次 worktree 为 `/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261024-fix-blog20261024-stellar-reference-colors`，分支 `codex/fix/20261024-blog20261024-stellar-reference-colors`，起点 dev `08f9811b`，仍合入本地 dev。
- 冻结参照为当前 `https://xaoxuu.com/`，版本声明 Stellar 2.0.0。唯一整体参照不变，本次颜色快照为 `docs/baselines/stellar-200-parity-20261024/reference-colors/reference-dark.jpg`；旧的 `before-local-dark.jpg` 是调整前现状。本地与参照使用相同视口及配色环境比较颜色，不要求不同站点内容、菜单配置相同。
- 本次配置：站点 `appearance.preset=minimal`，`color_scheme=auto`；`colors.primary=hsl(40 96% 55%)`，`colors.accent=#f60`，`colors.link=hsl(166 66% 32%)`。使用现有设置 API 合并保存指定字段，其余字段和配置保持。暖纸背景与卡片沿用 minimal 最终层，不添加新颜色覆盖或修改官方默认值。
- 写入归属：主 agent 独占现有 8124 隔离样例配置、本文与本次浏览器证据；`review_parity`（Sol/high reviewer）只读核查配置接线与覆盖边界，自行完成，不派生、不改文件或服务。已有字体、圆角、内容和四预设支持继续沿原配置；本次不更换设计系统。
- 验收实测：设置保存后 GET 重读整份 document 与预期一致，其他设置保持。浅/深色 × 1280/390px 共 4 组，背景、卡片、主题色、强调色和链接色的计算值与参照一致；浅色 body 为 `#faf7f0`，深色为 `#22201b`。同视口下文章和搜索共 8 组、320px 分页 2 组均无横向溢出，实际完成搜索打开/结果/关闭及第 10→11 页操作；两侧省略号完整、next 右边界 295px。记录见本次基线目录的 `configuration.json`、`browser-colors.json`、`browser-interactions.json` 与截图。
- 运行代码仍为根 `08f9811b78c24e38a65155d622ef5e32fbab0cdb` 的非开发模式二进制，仅现有配置发生变化。本轮 `make test` 实际通过：Console 298/298、Stellar 220/220、0 失败/跳过，部署/来源/模块边界、Go vet、Staticcheck、Go tests 全部通过。文档和证据提交/合入继续执行仓库强制门禁；最终日志与恢复备份留在根 `.tmp/stellar-reference-colors-20261024-archive/`。未新增源码或测试，无需重新构建、编译 CSS 或重复全部四预设矩阵。
- 体验入口仍为 `http://127.0.0.1:8124/`（隔离样例）。结束时已清除配色模拟并恢复原生视口，确认 auto 实际跟随系统深色，未留下访客配色覆盖。下一步由用户确认当前配色；真实数据、生产、远端 CI、推送、发布与部署未操作，原 T1–T7 清单不因此标为已验收。

### T2/T7 线上颜色差异续查（同任务，2026-10-24）

- 用户反馈颜色仍与 `https://xaoxuu.com/` 不同。本轮状态待体验；上述隔离样例的结果不代表线上博客。实际打开 `https://blog.xn--fhqths51enha.cn/` 后确认线上原来仍使用 card，前轮遗漏了该实际入口的对照。本轮已获明确授权并完成线上配置更新。
- 同浏览器、同系统深色、同 1280×720px 下，线上 body 为 `rgb(28,31,33)`（`#1c1f21`），参照为 `rgb(34,32,27)`（`#22201b`）；线上主色 `hsla(192deg 98% 55% / 1)`、链接 `hsla(207deg 90% 54% / 1)`，参照分别为金黄 `hsla(40deg 96% 55% / 1)`、绿色 `hsla(166deg 66% 32% / 1)`。线上实际加载 `main-card.css` 和 `main-appearance-card.css`，不是缓存推测。参照站另有衬线字体，字体不作为本轮颜色调整字段。
- 唯一颜色参照与线上原状分别保存在 `docs/baselines/stellar-200-parity-20261024/live-colors/reference-dark.jpg`、`before-live-dark.jpg`，浅色补充与计算值见同目录。最小目标字段见 `proposed-appearance.json`：minimal、auto 及金黄/橙色/绿色三个颜色。用户明确选择“应用到线上博客”后，使用现有已认证 Console 先读取并备份 document，只合并这五项，校验通过后保存一次，再通过“重新读取”确认整份 JSON 与候选完全相等；其余设置保持，敏感字段的遮罩按现有保存机制保留。
- 本轮 worktree 为 `/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261024-fix-blog20261024-stellar-live-colors`，分支 `codex/fix/20261024-blog20261024-stellar-live-colors`，起点 dev `1d40ecc9`，合入目标仍为本地 dev。主 agent 独占本文、证据与后续获准的设置操作；没有源码改动或可独立并行的实施，串行定位即可。
- 已实测：线上浅/深色 × 桌面 1280px/手机 390px 的 4 组首页，body、背景/卡片/主题/强调/链接及文字 token 均与参照一致；另有文章和搜索 8 组交互检查，搜索“Hexo”均返回 3 个结果，均无页面横向溢出。手机是 390×844 CSS 视口的真实浏览器模拟，截图工具缩放有局限，仅保留可靠桌面截图；计算值和交互测量见 `browser-verification.json`。字体、菜单项自己的 accent、分类色等独立配置未纳入这五项；例如左栏博客图标仍使用原有青色，不能宣称整个站点所有视觉细节与作者站完全相同。
- 未登录 HTTP 读取也确认线上已加载 `main-minimal.css` 与 `main-appearance-minimal.css`；实际下载的 CSS 字节与本地锁定上游产物相同，`/readyz` 返回 `ok:true`。完整配置重读结果见 `applied-appearance.json`，最终公开截图为 `after-live-dark-desktop.jpg` 与 `after-live-light-desktop.jpg`。结束时清除配色/视口模拟，确认 auto 跟随系统且 `data-theme` 无访客覆盖。
- 本轮没有业务源码、依赖、镜像或二进制变更，不进行重建/部署或重复全部四预设矩阵。`make test` 本轮实际通过 Console 298/298、Stellar 220/220（0 失败/跳过）及 Go/部署/来源/模块边界检查；文档证据提交与本地合入仍执行强制门禁。配置恢复文件与日志留在根 `.tmp/stellar-live-colors-20261024-archive/`，完整设置不提交。必要恢复时重新读取线上 document，仅恢复这五项原值（原来缺省的字段移除），保留其他最新设置。
- 下一步由用户体验线上效果；当前不标已验收。仅执行了已授权的线上外观设置保存，未推送、发版、替换二进制或改动文章数据；原 T1–T7 清单不因此标完成。

### T2/T4/T7 左下角设置图标续查（同任务，2026-10-24）

- 状态：本地修复待体验，线上修复实现中（尚未部署）。用户反馈线上左下角设置图标消失；继续使用 `BLOG20261024-STELLAR`，起点 dev `b7ffc1ae`，分支 `codex/fix/20261024-blog20261024-stellar-settings-icon`，worktree `/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261024-fix-blog20261024-stellar-settings-icon`，合入目标为本地 dev。主 agent 独占模板、定向回归、本文与证据；这是单一初始化契约修复，串行实施，独立只读复核由 `review_parity`（Sol/high reviewer）完成，未发现实质问题。
- 唯一局部视觉参照：`docs/baselines/stellar-200-parity-20261024/settings-icon/reference-upstream-dark.jpg`，来源 `https://xaoxuu.com/`。`before-live-dark.jpg` 与 `before-live.json` 保存当前线上缺失现场。保留既有图标、DOM、minimal 暖纸色和布局；本次只恢复设置入口图标，不重新设计控件。
- 已确认事实：未登录 HTTP 输出的 `.settings-widget` 含完整 SVG，浏览器执行后 `.ui-icon` 变为空。锁定上游 `scripts/lib/client-assets.js` 会设置 `window.stellarIcons=window.stellarClientData.ctx.icons`；本地已准备同一图标字典，却在 `partial/stellar/scripts.html` 漏接此别名。官方 `profile.js` 挂载/刷新时读取缺失全局并用空内容替换 SVG。颜色配置保存没有修改相关源码。
- 本地可观察验收项：[x] 首次加载与刷新后齿轮可见；[x] 设置页可打开、返回和同集合换页后图标仍在；[x] 桌面折叠侧栏与手机菜单图标可见；[x] 浅/深配色沿用当前 token；[x] 身份头像失败与退出恢复仍使用正确图标（Node 隔离回归）；[x] 当前构建产物浏览器与定向回归通过。真实线上只读调查，必要的生产二进制部署需另外获得明确授权。
- 修复仅在同步 defines 脚本中补回上游的图标别名，先于 defer 的 `profile.js`；不修改官方脚本、生成 CSS、图标形状或外观配置。实际模板 + 官方脚本回归先红 0/2、后绿 2/2；本轮 `make build` 与 `make test` 通过，Console 298/298、Stellar 222/222，0 失败/跳过，部署/来源/模块边界、Go vet、Staticcheck、全部 Go tests 通过。普通 commit hook、feature/dev 合并门禁及收尾日志保存在根 `.tmp/stellar-settings-icon-20261024-archive/`，实际结果以对应日志和 `final-integration.json` 为准；不得把本段当作已完成生产部署。
- 本次隔离服务 `http://127.0.0.1:8125/` 使用现有纯样例 root 的 SQLite 只读快照，config/content/DB/cache 独立，运行归档的真实 embed 二进制（不带 `-dev`）。`candidate-code.json` 绑定源文件与二进制 SHA-256，`runtime-binding.json` 确认首页/设置/文章均输出图标 payload 与正确脚本先后，HTTP profile.js 字节匹配源码且 readyz 正常；没有操作真实数据库或 8124 原样例。
- 主 agent 浏览器实际检查首页、刷新、设置页、返回、侧栏折叠/展开、同集合文章链接及浅/深色、390×844 菜单内图标与点击入口；手机展开动画结束后 SVG 完整位于视口内。早期过渡中采样和菜单关闭后的隐藏状态均单独标记，未当作最终可见证据；手机截图工具缩放有局限，只记录 CSS 视口/DOM 与真实交互。桌面截图及全部采样见本次 `settings-icon/` 目录。身份回退与退出未操作真实读者账号，BFCache/完整四预设矩阵未重复验证（本次无样式修改）。
- 下一步：授权后按项目发布路径将此修复部署线上，再复验真实首页及设置入口。当前线上仍运行原有模板；此前“应用到线上博客”仅授权五项颜色设置，不扩展成二进制发布授权。原完整移植清单仍实现中。

### T2/T3/T4/T5/T7 前台细节系统复核（同任务，2026-10-24）

- 状态：本轮细节修复待体验；原完整移植清单仍实现中，线上尚未部署本批源码。用户反馈仍有很多细节未同步，继续既有完整移植范围。起点 dev `829959d3`，分支 `codex/fix/20261024-blog20261024-stellar-detail-parity`，worktree `/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261024-fix-blog20261024-stellar-detail-parity`，仍合入本地 dev。线上是否已部署、当前源码是否缺功能、站点个性配置差异分别取证；既有五项线上配色授权与未部署源码的边界保持。
- 唯一视觉参照沿用 `https://xaoxuu.com/` 当前 minimal 暖纸色；本轮截图放 `docs/baselines/stellar-200-parity-20261024/details/`。行为和参数仍以锁定 `2.0.0 / 9d11978a24fe6610e3a7dbab741cf62634539d05` 为准。保留已确认 Go SSR、原生评论、本地扩展、Console、四外观及三配色；不能把作者自己的内容、菜单或字体配置直接算成代码缺陷，也不重新设计全局 token。
- 实际独立并行核查：`audit_templates`（Sol/high analyst）只读 T2 壳层/区域/品牌/菜单/widget/footer/page-controls；`audit_runtime`（Sol/high analyst）只读 T4 客户端初始化、运行时和媒体交互；`audit_content_details`（Sol/high analyst）只读 T3/T5 post/listing/Wiki/Notebook 内容模板。均不写文件、不访问生产、不启动服务、不派生。主 agent 唯一写入共享路由、模板、任务记录及浏览器证据，负责复现、确定修复范围、实现和最终组合验收；有独立实现模块时再明确独占文件与工作树。
- 本轮验收说明：先保存当前页面并确认实际运行版本；按上述三条用户路径把有证据的代码缺口列为可观察场景，修复后检查受影响页面、桌面/800/390px、适用浅深色与交互。保留既有 UI，只改与锁定上游不符的分支/配置消费/DOM。按风险复用既有定向检查、真实构建、必要 CSS 核对与强制门禁；没有当前组合态证据的完整清单继续保持实现中。
- 已确认作者规则：多作者且未指定默认作者时，采用当前保存文档的首位作者。保存已将普通键按 JSON 字典序排列；公开投影遵循 `Object.keys` 的规范整数索引优先规则，不能声称恢复了历史输入顺序。保留显式 string/object 选择，补齐公开对象的 id/path/desc，私有字段不公开；`content.authors → article.author → defaultAuthor.id` 优先级不变。
- 本轮冻结的可观察修复：[x] 搜索标题、章节与摘要位于同一链接，空摘要不产生无名链接；[x] 搜索初次结果与类型重绘挂载/卸载 hover；[x] Story H3、引用左右装饰与 loading 的五个 CSS 图标变量使用可信 registry；[x] 文章最近更新按 updated 排序；[x] 页面明确 `active_menu: wiki` 时取全部可索引 Wiki 文档并显示集合前缀，Notebook 首页取跨笔记本、笔记页取本集合；[x] 非文章页显式作者覆盖默认作者，带点 ID 精确查找；[x] 页脚使用站点 author；[x] linklist 等价路径正确高亮；[x] 参考链接补增强图标，许可证维持普通 Markdown。相关文章摘要已证实有去标签处理，未改；菜单和字体仍按站点配置。
- 独占实施：`fix_search_details`（Sol/high worker）在 search 后缀树依次处理搜索 JS/Node 回归、regions widget、content references/日期作者与最小链接增强入口；`fix_icon_css`（Sol/high worker）在 icons 后缀树仅处理 registry CSS helper/head/定向测试。两树不共享分支；补丁逐项检查后由主 agent集成，主 agent独占路由公共数据、recent 选择组合、任务记录和浏览器。`review_details`（Sol/high reviewer）独立只读审查；发现 recent 错用导航/profile fallback 的 P2，已改为只读页面 activeMenu 并复查关闭。
- 旧 `TestWikiDocFrontendChain` 夹具把未配置菜单等同于当前 Wiki 文档，与上游 `recent.ejs` 的页面显式规则不符。本次仅为 intro 夹具明确 `active_menu: wiki`，保留原断言并加强为同时检查另一个集合的文档；新增“仅主题/导航配置 Wiki，页面省略仍用文章”的对照，未跳过、删除或降低门禁。
- 实际验证：默认作者/最近更新、搜索、widget、图标、references 各组先复现红灯再修绿；本轮当前组合 `make test` 通过 Console 298/298、Stellar 225/225（0 失败/跳过）及部署/来源/模块边界、Go vet、Staticcheck、全部 Go tests。`make build` 生成真实 Console/UC 与 embed 二进制；源文件摘要、二进制 SHA-256 和 HTTP 字节绑定见本目录 `details/candidate-code.json`、`details/runtime-binding.json`。未修改生成 CSS 或共享主题源。
- 主 agent实际浏览器核验：1280px 下搜索标题点击到文章、启用 spotlight 后初次/类型切换各恰好一个效果层，鼠标坐标变量随真实指针变化；默认 Alice 与 page/Wiki/Note 指定 Bob、host 页脚、跨两 Wiki/两 Notebook recent 和集合前缀正确。四外观均在 auto 的系统浅/深色下检查受影响图标/链接，再检查 minimal 显式 light/dark 覆盖系统相反配色；800/390px 无横向溢出，390px侧栏设置 SVG 完整在视口、点击进入设置，搜索链接也在视口内。详见 `details/browser-verification.json` 与截图。
- 当前参照站和本地在相同 1280×900 CSS 视口下，浅色背景均为 `rgb(250,247,240)`、主色均为 `hsla(40deg 96% 55% / 1)`、main 均宽 648px。参照站使用衬线字体，本地保留现有系统字体；不将个人配置差异包装成遗漏。原生截图实际为 1272×716 JPEG，手机使用 DOM几何与实际交互证明，不冒称手机截图完整验收。loading 变量已验证，持续加载动画和真实第三方 SiteInfo 服务未实测。
- 体验入口：`http://127.0.0.1:8126/post/details-story`，另有 `/details-about`、`/details-a/intro`、`/notebooks`。root 为根目录 `.tmp/stellar-detail-parity-20261024-root/`，仅从旧纯样例 SQLite 只读备份，不含用户数据；config/content/data/uploads/cache/端口独立，服务运行归档二进制且不使用 `-dev`。8124/8125 原样例保留。提交、合入和组合态日志统一保存在根 `.tmp/stellar-detail-parity-20261024-archive/`，本轮资源和补丁先归档，再按规则清理已完成任务树；最终集成结果见其中 `final-integration.json`。
- 生产边界：本轮未 push、创建 PR/tag/Release 或部署。此前“应用到线上博客”只授权五项配色设置；线上公开程序仍为 `v2.0.0-local.e2f8c664`。实际上线候选是完整当前 dev，会包含前序尚未部署改动，不能称为只发布这一批模板；需沿 AGENTS.md 的明确发布/部署授权与项目流程执行。


#### 本批正式发布与生产升级（2026-10-24）

- 状态：实现中。用户已明确回复“授权”，覆盖当前完整 dev 候选的推送、dev→main 与 main→release PR/合并、正式 tag/双产物发布、生产部署；不再重复询问这些已授权步骤。原候选 `fb066e91` 包含前序设置图标/视觉及已说明的 Console、依赖和发布检查改动；不导入、发布或迁移真实内容。
- 远端刷新后 `origin/main=86737cfc` 仅比 `origin/dev=9651820b` 多 PR #17 的合并记录，两者 tree 相同。按发布前置从 origin/main 创建同步树，合入候选 dev，保持程序源码不变并经 `make merge-check` 回到根 dev。分支 `codex/chore/20261024-blog20261024-stellar-release-sync`，worktree `/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261024-chore-blog20261024-stellar-release-sync`；同属 `BLOG20261024-STELLAR`，不是新业务任务。
- 拟发布 `v2.0.0`：当前公开最新为 v1.7.4，生产已是 2.0 本地构建。必须等待 dev/main/release 适用 CI、tag 的 Release Build 与 Docker Release 都成功，核对三个二进制资产、SHA256/SBOM和镜像可启动，再替换生产固定镜像版本；先检查当前 Compose/数据/维护日志并保存可恢复备份，升级后核对 readyz、实际版本、SPA资产、公开页面、设置图标/搜索/配色及数据保全。
- 发布过程日志和回执存根 `.tmp/stellar-release-20261024/`；最终公开结果/截图存 `docs/baselines/stellar-200-parity-20261024/release/`。旧本地构建与浏览器证据继续绑定 fb066e91 的程序源码；新合并提交、远端 CI、双产物、服务器实际版本与用户待体验分别记录，不能用其一代替另一项。
- PR #18 首轮 CI（40102348646）为 109 项 E2E 通过、5 项失败，其余适用作业通过。根因是 `stellar-200.spec.ts` 的合成页面缺少生产 `clientconfig` 已无条件提供的 `ctx.ui.classes`，搜索结果统一交互卡片在服务端和本地两条渲染路径都会访问此字段。仅补齐夹具的生产 class 契约；原断言、超时、Wiki filter、缓存和卸载场景保持。只读 analyst 独立核对了契约，主 agent 复现并修复。
- 本地先用隔离临时配置与已安装 Chrome 复现首个搜索结果超时（诊断命令限 8 秒），补齐夹具后按原用例默认超时运行整个文件，9 项通过（10.1 秒）；未把本地 Chrome 当作 CI 的 Chromium。默认 Playwright 所需 headless shell 尚未安装，安装进度单独记录；CI 原配置和门禁未修改。证据为根 `.tmp/stellar-release-20261024/pr18-search-{red,green}-chrome.log` 与 `pr18-red-evidence/`。
- 服务器已收到 SHA256 `8ce2dada519dfe00ffcaba679a823f349623d95114e10c1d545c3c94dac6eea9` 的已审查脚本，执行 `preflight` 通过：旧镜像 `sha256:0463cefd…a24d4`、readyz 200、重启 0、无待恢复维护日志、39 个业务表、392 个保护文件，约 861 MB 备份输入、约 16.24 GB 空闲。私有前像和公开检查摘要保留在 `/home/ubuntu/blog-release-v2.0.0/`；尚未 stage、停生产、创建新备份或切换镜像。下一步仍是修复提交的本地门禁、更新 PR #18 完整 CI，再按上述发布路径继续。

## 目标与已确认选择

完整移植官方 tag `2.0.0`，commit `9d11978a24fe6610e3a7dbab741cf62634539d05` 的能力与实际行为。源码依据为该 commit 的 `_config.yml`、`scripts/`、`layout/`、`source/`、`languages/` 和测试。不得把 Hexo 专属能力标成“不适用”后称为完整移植。

- 2026-10-13 用户已批准浅色、深色和自动切换，覆盖旧的“Stellar 产品仅深色”约定。该选择不授权顺带重设计 Console。
- 用户已选择严格采用上游规则：未迁移旧配置和内容应产生诊断，先提供迁移预览，再执行迁移。真实数据当前只读。
- 严格规则以锁定源码为准：配置/root/归属冲突硬错误；上游对普通表现字段使用 recovery/warning 的地方保持同等语义，不人为改成全字段 fail-fast。
- 四种外观预设属于站点配置；上游访客设置页包含评论身份、缓存与关于信息，没有访客选择预设功能。配色切换是独立扩展。
- 用户已确认保留 Go SSR／Console／SQLite，完整实现所有上游等效能力，包括创作命令、Doctor、图片处理和生成路由。
- 用户已确认原生评论、旧 poster、瞬间/links 标签、复制链接等本地能力作为独立 Blog 扩展保留；完整增加上游能力，不将扩展字段混入严格的上游配置结构。本地用户身份不自动替换第三方身份。
- 用户已确认生产仍交付 Go 单二进制，不增加 Node/Sharp 运行依赖；完整图片解码、色彩管理和缩放采用可嵌入实现，开发时可使用锁定 Sharp 0.34.5 作差分依据。格式与算法尚待实际证明，不以简化解码代替完整能力。
- 用户已确认 Gist 在隔离 iframe 内加载官方脚本，保留动态嵌入；iframe 只允许脚本执行，不取得父站同源权限，不开放作者 HTML 的脚本权限。
- 真实 banner/cover 图源冲突等逐记录事项仍在最终候选预览时确认；当前不改真实数据。

## 起点、参考与数据边界

根 dev 起点 `49fe104`；根 `.pi/` 未跟踪项保留。主 agent 复用干净的 sibling `Blog-stellar-200` / `codex/stellar-200`，它与 dev 同步，hook 已指向 `.githooks`。不切换或重置共享 `resource/hexo-theme-stellar`。

唯一整体视觉参照是 [上游主页基线](baselines/stellar-200-full/upstream-home.png)，来源 `https://xaoxuu.com/`，2026-10-13 实际读取，视口 1280×720，页面声明 Stellar 2.0.0、`data-regions=leftbar`。该站使用 minimal，不能将其个人配置误称为上游默认值；card/glass/minimal/flat 的具体行为仍逐项按锁定 Stylus 核验。Wiki Hero 必须按上游 cover slot 放在工作区之前、首屏全宽。

此前 `docs/baselines/stellar-200/` 仅为现状和历史证据。现有 8098 是隔离样例，不是用户真实内容。所有新运行、迁移和浏览器验证使用独立 DB、配置、内容、上传、缓存及端口；不使用根 `data/blog.db` 执行 seed、迁移或验收操作。

本地设计库已按 `library-consumption.json` 阅读基础资料。Stellar 前台视觉由用户指定上游约束，不能用 Apple 商城的颜色/字体替换 Stellar；新增 Console 操作继续消费现有 Console 组件与语义 token。

## 全量能力与验收清单

### T1：固定完整契约和迁移预览

- [ ] 配置树、默认值、类型/枚举、snake_case 规范化、参数袋与热更新完整对应上游。
- [ ] 配置与内容 schema、collection membership、可定位诊断、Doctor 检查对应上游。
- [ ] 存量配置、文章、页面、Wiki、Topic、Notebook、评论和用户的迁移预览逐项有来源、目标及冲突处理。
- [ ] 每项完成证据来自当前实现和当前检查，未决/缺失能力保持未完成。

依据：`scripts/schema/`、`scripts/lib/{config-schema,content-config,content-membership,doctor}.js`。当前数据影响见 [迁移预检](stellar-200-migration-preview.md)。

### T2：完整布局、外观和品牌

- [ ] Topbar/Leftbar/Rightbar 独立启停；global → profile → collection → page 级联；空数组清空、省略继承、数组整体替换。
- [ ] Widget 注册、位置能力 warning 与跳过规则；品牌 regular/compact，图片 avatar/icon/plain，GitHub 用户/仓库数据，菜单 1–5 列、选中、底部 actions。
- [ ] 全部 profiles：home/blog_index/post/topic/wiki_index/wiki/notebook/notebooks/note/author/page/settings/error；path、listing_nav、active_menu 及页面覆盖实际生效。
- [ ] card/glass/minimal/flat，auto/light/dark，系统变化、持久化和配色事件同步。
- [ ] 所有 appearance 字段：colors/gradients/typography/shape/backgrounds/code_block；不能仅接受字段而不消费。
- [ ] 桌面、中间断点和手机的 shell、rail、drawer、inert、焦点恢复、Escape、遮罩、搜索 dialog 与目录折叠。

依据：`_config.yml`、`scripts/lib/{regions,brand,menu,layout-config,widget-registry}.js`、`layout/_partial/primitives/`、`source/css/`。

### T3：完整内容模型、聚合与 Wiki/Notebook

- [ ] 文章 tech/story、paragraph_indent、banner 全字段、created/updated/author、四档 AI 标记、阅读时长、分类颜色与相关文章。
- [ ] 置顶 carousel/flat、普通卡 hero/classic、cover_ratio、摘要、标签、排序分页；archive/category/tag/author/topic 聚合及成员顺序。
- [ ] Wiki collection、路由、首页、章节树、关联集合、前后篇、远程 README 与相对链接、版本/GitHub 信息。
- [ ] Wiki Hero 工作区外首屏；背景图片/视频/视差；strands/galaxy/light-rays/ferrofluid；preview image/terminal、命令标签与复制、多动作；离页释放动画资源。
- [ ] Notebook 总索引、多笔记本、多级标签、tagtree/图标、date/updated/title 排序、asc/desc、per_page=0、摘要和可见性。
- [ ] collection.profile/id、显式和路径推导归属、冲突错误、listed/searchable、内容级 render/seo/inject/source/footer/comments。
- [ ] references/license/show_tags、全部上游分享项、贡献者源路径最长匹配、footer token、canonical/allowed_hosts/OG/JSON-LD/robots。
- [ ] zh-CN/zh-TW/en 系统文案、可信注入、preconnect、fallback 与资源配置。

依据：`scripts/lib/models/`、`scripts/lib/collection-pipeline/`、`scripts/generators/`、`scripts/schema/content-config-*`、`layout/`、`languages/`。

### T4：完整运行时、搜索、评论与访客设置

- [ ] 原生 ESM manifest、所有注册扩展、document/region 作用域、按 DOM 启用、资源去重、abort/cleanup、request/cache 与配色同步。
- [ ] 同集合且 shell signature 兼容的 partial navigation；更新 main/rightbar/head/meta/styles/manifest/选中状态；历史滚动、锚点、连续导航取消；不兼容/重定向/非 HTML/失败时整页回退。
- [ ] local/algolia/null；scope/include_content/TTL；标题标签分类正文锚点与 collection domain；快速换词/离页没有迟到响应覆盖。
- [ ] beaudar/utterances/giscus/twikoo/waline/artalk/null；enabled/title/id/provider/options 的完整级联和挂载卸载。
- [ ] `/settings/`：Artalk/Waline/Twikoo 身份、保存退出、头像；搜索/动态/全部缓存统计与清理；关于信息。不得自动将本地登录等同第三方身份。
- [ ] pin-slider/page-controls/deferred-icons/dropdown/lazy-loading/lightbox/link-prefetch/reveal/mathjax/KaTeX/diagrams/code-copy/adaptive-text/card-hover/heti/swiper 开启、禁用与失败路径。

依据：`scripts/lib/{browser-runtime,contribution-registry,partial-navigation,comments,search-index,search-domain}.js`、`source/js/runtime/`、`source/js/search/`、`layout/settings.ejs`。

### T5：全部标签、动态服务及 Markdown/媒体

- [ ] 按 `scripts/tags/index.js` 与 `inline-labels.js` 注册全集逐个核对参数、默认值、HTML 和交互，不仅比较标签名字。
- [ ] 容器：tabs/box/folding/folders/grid/swiper/gallery/banner/dropdown。
- [ ] 数据：friends/albums/posters/sites/ghcard/gist/toc/timeline/md/chat。
- [ ] 表达媒体：checkbox/radio/copy/emoji/icon/frame/image/link/button/mark/navbar/note/poetry/quot/blockquote/hashtag/mbti/okr/audio/video/rating/vote/table/tip；阅读：reel/paper；全部 inline labels。
- [ ] mdrender/siteinfo/ghinfo/rating/vote/sites/friends/friends_and_posts/timeline/memos/rss/weibo/contributors；四类最新评论；voice/video/chat-file 的数据服务与生命周期。
- [ ] 图标 namespace/deferred JSON/未知 fallback；图片失败、lazyload、尺寸平均色；Markdown 链接 favicon/siteinfo 和表格增强；外部 Markdown 相对资源。
- [ ] 现有可视化编辑器 catalog 与解析器、保存、重新打开语义一致；旧写法迁移有预览，不静默重写 Markdown。

依据：`scripts/tags/`、`scripts/filters/lib/`、`source/js/services/`、`source/js/plugins/`。

### T6：完整创作、诊断、图片处理和构建生命周期

- [ ] Doctor text/json：环境版本、配置、内容、归属、Widget 的诊断与退出状态。
- [ ] new note：notebook/title/tags/dry-run、防覆盖、防路径穿越、失败回滚。
- [ ] images：自动 server/generate 对应生命周期、增量元信息、单页扫描/refresh/dry-run、本地图片生成资源、平均色/尺寸；不压缩或替换原图。
- [ ] 远程 15s/32MiB/40MP 等实际边界、持久 metadata、并发锁、原子写、失败延迟重试与避免热更新循环。
- [ ] 图片/远程 Markdown 通过已有网络保护；可信注入仍保持原有权限边界。涉及行为冲突先确认，不照搬无保护 fetch。

依据：`scripts/commands/stellar.js`、`scripts/lib/{doctor,new-note,safe-path,image-metadata}.js`、`scripts/events/`。已确定采用 Go 等效命令和操作入口，不能删减本组。

### T7：组合态验证和用户体验

- [ ] 隔离旧库和新库测试；完整配置保存→重开→前台生效；每个内容系统编辑→保存→渲染→搜索/导航。
- [ ] 全部预设、三配色、全部 profile、各 region 组合及相关断点真实浏览器验收。
- [ ] 全宽 Wiki Hero、四种动效、视频、terminal、搜索、局部导航、历史前进后退、缓存与访客设置实际操作。
- [ ] 全部标签参数/动态服务隔离契约验证；真实外部服务结果与本地 fixture 分开，缺少账号/endpoint 时保留未完成验证项。
- [ ] `make css`、`make css-drift-check`、真实 `make build`、feature `make test`/`make merge-check` 与 dev 组合态复验；服务实际加载当前产物 hash。
- [ ] 对上述每一项审计，证据不足保持实现中；源码复制率、manifest 数量、CSS 编译或旧测试绿色不能单独证明完成。

## 实施分工与启动依赖

本轮实际并行了 T1 上游全集研究（researcher/Sol）与本地冲突链路研究（analyst/Sol）；两者只读、自行完成。主 agent 查根 DB 结构与计数、固定真实视觉参考、冻结规格及冲突选择。

后续实现必须每位写入 agent 使用独立 sibling worktree 和独立 `codex/` 分支，不共用主分支；共享文件只有唯一所有者。主 agent 负责集成，不能用专家自报替代最终验收。

1. T1：先确认架构及存量处理，schema 负责人冻结模型/默认值/验证/迁移契约；对应工作树独占新 schema 模块和相关 tests。主 agent 唯一写入共享 Site/ViewData/store/API 类型并执行生成。
2. T2–T3：布局专家在模型契约满足后独占 templates 和专用布局测试；外观专家独占 build/stylus、生成 CSS、appearance 专用资源及编译检查。Hero 数据模型先由主 agent 接入，模板不自创旧/新接口变体。
3. T4：运行时专家独占 JS/runtime、JS/search、manifest 构造与生命周期测试；原生/第三方评论规则确认后才接入 Provider/访客设置。
4. T5：内容专家独占 shortcode renderer/catalog 及对应往返 tests；运行时负责服务 JS，主 agent 串行接入共享注册/页面渲染，避免重叠写入。
5. T6：工具专家在架构、数据与网络规则冻结后独占命令/图片元信息模块和隔离测试。真数据迁移依赖可审查的完整预览，获确认后才执行。
6. T7：各独立分支先定向验证并由主 agent 核对后集成；主 agent 执行全门禁、组合态与实际浏览器，审查专家只读检查迁移/安全/生命周期风险。

当前实际写入分工：clientconfig 专家独占 `modules/stellar/clientconfig/`；内容专家独占 shortcode renderer/catalog；图片专家独占 `internal/stellarimages/`；配置专家独占新增 `stellar_v2` 配置文件及原子 settings 保存方法（Site 仅新增配置字段）；外观专家独占 Stylus、生成 CSS 和 CSS 门禁。各自独立分支中的 Go 依赖由主 agent 集成去重。主 agent 串行接入路由、模板、Markdown 后处理和 Console，执行真实迁移前仍须提供逐记录候选。

后续已启动的独占模块：页面模型专家负责 `internal/stellarmodels/` 的完整 Collection/Page/Hero 模型；外观专家负责 `modules/stellar/appearance/` 的全部运行时 CSS 注入；配置专家负责严格主题 Console tab/API 与原子热发布。模型模块与图片模块均不自行访问真实数据库或启动服务，宿主装配、真实逐记录迁移候选及最终浏览器验收仍由主 agent 承担。

内容存储专家已在独立 sibling `Blog-stellar-v2-contentstore` 启动：独占 store 内容/集合 CRUD、相关 schema 及 Notebook/Note 新存储。新增版本列默认 0，不改既有正文、作者或评论身份；版本 2 元数据与原内容同次原子保存，真实迁移仍依赖候选预览批准。

普通编码细节自主处理。新的数据损失、重要兼容性或体验冲突交用户确认；仅暂停依赖部分。未授权 push、发布、部署、远程消息、删除 worktree/分支与真实付费调用。

## 本次记录

### 2026-10-13 接续实施与真实验收

本任务整体保持**实现中**，下面仅记录已经实际执行的范围。T1–T7 原清单不缩减；用户尚未确认完整体验，也未批准转换真实存量数据。

- T1：迁移快照格式 3 明确绑定 SQLite rowid、WITHOUT ROWID、逐值 storage class 和无损 BLOB。独立审查发现的隐式 rowid 与 INTEGER/REAL 混淆已修复；稀疏身份、external-content FTS MATCH、Notebook 结构恢复与摘要变化测试通过。执行证明只创建新系统临时 root，不是生产 Apply；该阶段尚未覆盖上传字节、原索引/视图/触发器与旧运行目录；后续完整根目录实现见下文。
- T2/T7：真实完成四 appearance × 三配色，全部 14 个 profiles（包括独立 Moments 扩展）× 三语言 × 1280/800/390 断点，以及八组 region 开关组合。证据分别为 `resume-appearance-matrix.json`、`resume-profile-locale-matrix.json`、`resume-region-matrix.json`。这些记录不能替代所有 appearance 字段与交互参数的验收；保留原截图中的失败现场。
- T3/T7：Post、Page、Topic、Wiki、Notebook/Note 的阶段保存重开与前台/搜索证据已增加到基线目录。Wiki 编辑器经用户确认统一为“集合显示名（name）”，真实保存后项目列表与前台卡片均消费 name；旧真实数据取值仍须逐记录审查。Moments 头像经图片缓存增强曾被写成 512px 宽，已固定 16×16 并在桌面和 390px 真实复验。
- T3：独立 Page 的显式 Open Graph 图片等参数此前未完整消费，已修复正式 SSR、Offline 与 opaque preview 路径；浏览器确认显式描述和图片生效。上游仍强制 OG 标题使用 Page title，不把该正式行为改成新的宿主语义。Friends 默认标题/横幅与 false/null RSS 开关、无关页面不受污染、Wiki 集合排序/卡片外链及本地 canonical 已实际操作；robots 附加规则使用真实 HTTP 验证，浏览器客户端阻止 robots.txt 导航，证据如实区分。
- T4：同集合 Post 局部导航真实更新标题、canonical、内容，浏览器 timeOrigin 保持不变；历史返回精确恢复实际点击时的 709.5px 滚动位置，前进恢复目标页。定位器操作前的滚动值可能因浏览器自动将链接滚入视口而变化，证据额外记录了一次性点击时测量，不能将操作前值误作恢复目标。当时连续取消、失败回退、BFCache 等尚需补验；后续结果见下文。预取已经增加完整卸载与真实 URL.origin 边界回归。
- T5：新增有效的隔离 WAV/MP4、两图 Gallery/Swiper 和本地动态服务。浏览器实际完成 Tabs、折叠、目录、灯箱下一张/Escape、轮播、下拉 Escape、复制、音视频播放暂停；官方评分/投票完成 GET→POST→重载防重和 503 失败回滚。Friends/sites/timeline/RSS/weibo 合成服务实际显示成功。checkbox/radio 默认由固定上游样式设为只读，不把默认点击无效改成自创行为。54 标签展示页全部载体无 tag-error，但外部 Markdown、Gist 正式集成、聊天媒体、其它动态服务与参数全集尚未全部验证，原示例中的图片占位不算音频成功路径。
- Console/T7：修复异步专栏目录水合制造文章假脏态，派生 topic_id/topic 不再参与 strict 保存或脏态比较。另修复正文水合/预览 context 覆盖未恢复本地草稿，真实浏览器等待后草稿正文保持原样且状态仍为“所有修改已保存”；仅用户真实编辑安排备份。没有自动发布或改写真实数据日期。
- T6/T7：本次真实 `make css` 和 `make css-drift-check` 通过，20 份产物与锁定对象一致；编译时共享 resource 的 unborn HEAD 与未跟踪源文件原样保留。实际主实现树主题输入仍是 unborn HEAD/未跟踪源文件，锁定 commit object 可用；根日常树另一个既有输入为 detached `1f4cb4bc0a44e829b6d0929ce4102aea9cd9a2a8`、仅未跟踪 `.mimosa/`。两者本任务均未切换或重置，编译仍 archive 锁定对象。`aa1fa93` 组合的真实 `make build`、完整 `make test` 通过；`resume-runtime-aa1.json` 核对运行二进制、嵌入 WASM、Console HTML、六入口资源及 main.js 与当前产物一致。最终 `make merge-check` 与 dev 组合验收尚未执行，本轮没有合 dev。

- T4/T5：正式 Gist 标签已修复父子首次尺寸握手，并给 immutable bridge URL 加入内容版本；真实父页自动得到 850px，CDP 证明 opaque origin、`contentDocument === null`、仅 `allow-scripts`。同集合 partial 离页后旧 Gist 变成 `about:blank` 并脱离 DOM，四段聊天媒体暂停且脱离，返回时使用新 token 和新实例。证据为 `resume-gist-handshake-fixed.json`、`resume-t5-partial-lifecycle.json`。
- T4：真实完成连续导航取消、15 秒超时回退、网络失败回退，以及注入非 HTML XHR 后整页回退；标题、canonical、最终内容一致，loading 清除。证据为 `resume-partial-rapid-cancel.json`、`resume-partial-timeout-fallback.json`、`resume-partial-fetch-failure-fallback.json`、`resume-partial-non-html-fallback.json`。早期重定向注入因工具持续报告 paused document response 未通过，失败现场保留；随后用真实 Gin 301（无 Fetch 注入）已验证整页回退，见 resume-partial-real-redirect-fallback.json。BFCache 原生分支仍未触发，见 resume-bfcache-reasons.json，不能用合成 persisted 事件代替。临时网络延迟已复原，注入后 Fetch patterns 已清空，页面已重新加载。
- T5：四类最新评论与 albums/posters 在 loopback 合成服务真实显示；QQ quote 定位、QQ/WeChat 语音播放暂停、聊天视频全屏与播放暂停实际操作。正式官方 chat-file API 成功下载上游公开 README，5060B、hash 与返回字节一致。畸形 `file.data` 原误造 0B 文件已修复并测试，保留失败现场。证据为 `resume-t5-services-extra.json`、`resume-t5-chat-media.json`、`resume-t5-chat-public-file.json`。外部 Markdown 成功路径受本机 fake-IP 解析到 198.18/保留地址阻断，safefetch 正确拒绝；没有更改网络保护，不记成功。
- T4/T6：Algolia 连续换词/清空/同词往返的旧响应覆盖已修复；图片 Prepare→AnalyzeContext→WASM 执行取消已贯通。实际 SVG 取消从仍等待 14.905 秒的红测改为及时结束且 runtime 可重用、缓存/锁不污染。PNG/GIF 和 AVIF 库内部缺 context API，仅阶段边界检查，不能宣称所有格式即时抢占取消。`c2cee18` 主组合真实 `make build`、完整 `make test` 通过（Console 256、Stellar 174、0 skip）；`resume-runtime-c2c.json` 确认当前服务产物 hash。
- T1：用户已确认官方导航、Wiki 旧 title 作唯一 name/shelf 仅 ID、官方集合首页、posts/10 的列表 cover/详情独立 banner、移除三条占位按钮；Notebook/Note owner 为精确匹配的现有 sakurasep，结构映射及日期/slug/draft 候选均已确认。仅更新未最终批准候选，不转换真实源。完整 root Capture/Rehearsal/Apply/Recover 已接入主树 `b3a9c5c`，合成测试覆盖上传/运行资源及 SQL 对象；独立审查发现的提交日志、放行与不确定 fsync 恢复缺陷已修复并复审通过；正式 CLI preview/rehearse/apply/recover、启动恢复与 canonical lifetime guard 已接通，实际二进制合成 E2E 通过。最终正文摘要批准、完整 fresh root 审查与真实 Apply 均未执行。

### 2026-10-14 接续闭合（仍实现中）

- T1/T6：正式 c161 单二进制对真实来源两次只读 preview 退出 0；32 条候选、172 配置映射，原 766 逐记录映射保留，选值后 777，Notebook 结构 26 项。七条正文及最终完整 RootDigest/ReviewDigest 未批准，没有真实演练或 Apply。DB/WAL/tracked 与全部内容 hash 不变，仅 SHM mtime 变化，不能声称整个目录零写。完整脱敏报告见 `stellar-200-migration-preview.md`。
- T3/T7：Console 正文保存、Note、footer 与 Markdown widget 共用同次公开 inventory/locale/站点链接/Wiki 树；opaque 正文及整页由单 callback 同代快照生成。新回归先复现旧坏正文阻断有效修复，再验证实际 Console HTTP 修复成功、无关坏正文不阻断、坏候选拒绝不写入。启动遇正文错误可进入 Console 修复，FTS 失败先清空，清空失败禁用共享查询；schema/structure/version 仍拒启。独立并发与隐私复审通过。
- T4：六评论提供方在正式前台配合独立 8111 合成 SDK 完成成功、局部导航清理、延迟资源和 404；Artalk/Waline/Twikoo destroy 与三个 embed 的 DOM 移除分别记录。Twikoo 迟到 init 的旧实例及时销毁，新页只一实例。三身份真实保存→重载→退出已操作。证据 `resume-six-comment-lifecycle.json`、`resume-three-visitor-identities.json`；这些不是第三方账号实测。
- T2/T7：完整自定义颜色、字体/九权重/字号/对齐、半径/形状、两个渐变、代码滚动条、背景图/模糊/饱和/覆盖色已实际保存重开并读取浏览器 computed style，代码字体及正文 justify 实际生效。另核验浅/深背景渐变、none、null 图片回退及自定义高亮 stylesheet 真实加载。早期受访客 auto 覆盖和加载时机影响的采样保留，最终证据为 `resume-appearance-custom-complete.json`、`resume-appearance-background-variants-final.json`；临时访客覆盖和配置已恢复。
- T4/T7：设置页动态头像被 LazyLoad 的旧 data-src 覆盖已在正式浏览器复现；SSR 动态头像使用已有 no-lazy 约定，真实本地有效 avatar 的 src/currentSrc 保持正确。`resume-dynamic-avatar-red.json` 与 `resume-dynamic-avatar-green.json` 保留红绿证据，普通正文图仍 lazy。
- T5：代码高亮补齐官方 highlight/code/gutter 载体，真实剪贴板复制精确原文（无行号）及延迟完成离页无 toast 污染已验证，剪贴板已恢复。MathJax 两公式、Mermaid 图、deferred SVG 和 HeTi 8 个 spacing 实际加载；局部离页清除 MathJax registry，资源 404 时保留源内容并报告错误。参见 `resume-code-copy-clipboard.json`、`resume-code-copy-late-cleanup.json`、`resume-plugins-live-lifecycle.json`、`resume-heti-enabled.json` 与 `resume-plugins-resource-failure.json`。
- T5：官方标签 oracle 补齐全部 54 标签 catalog 参数门禁：288 参数输入中 270 严格 DOM、1 decoded URL query 等效、17 确切空 CSS 等效；39 主题默认/三 locale 严格 DOM，Gist 3 官方脚本请求语义。另 19 个安全转义/上游单 KR 缺陷输入单列有意差异，不计严格 DOM 通过。真实多 Swiper 参数检查发现第二实例未挂载，已修复全载体挂载与各自控件；新浏览器确认两实例独立切页，同集合 Note 局部离页时 timeOrigin 不变且两实例均 destroyed、脱离 DOM。红绿证据 `resume-multiple-swiper-red.json`、`resume-multiple-swiper-green.json` 保留。
- T6：新增正式图片 fetch 路径隔离测试，Content-Length 超 32MiB 零读取、未知长度恰读 32MiB+1 拒绝、等值成功、默认 15 秒真实超时及 body 取消通过；原图/旧缓存/无关记录保留。fixture RoundTripper 不访问外网，真实 SVG/WASM 解码证据仍独立。
- T7：审计确认 sitemap 为 Blog 保留扩展（上游无此生成器），已补回首页 priority=1.0、taxonomy 实际配置路径及 links/moments 静态索引，子目录 Root 一次及去重回归通过。最后组合 build/test/merge-check 与新产物 hash 在全部修复合入后执行，不能用此前子树绿色代替。

- T3/T7：正文 TOC 原相对路径在嵌套正文与 Console srcdoc 中错误解析，已单独投影公开根相对 Wiki 树，不改正式模型。真实 Note 和 opaque 预览都有两个正确 `/wiki/galaxy/...` 链接；iframe 保持仅 allow-scripts、contentDocument 为 null。footer 与 Markdown widget 实际保存重开后，同请求消费公开 Wiki 树、本地绝对链接和三 locale；MBTI 标题分别显示架构师、架構師、Architect。证据 `resume-context-preview-toc-route-green.json`，临时配置及 Note 原文已恢复。
- T3/T7：Notebook date/updated/title × asc/desc 六种排序完成真实配置保存、重新打开、前台列表检查；使用 beta 的不同 date/updated 候选，避免把相同日期的稳定顺序当证明。per_page=0 同页显示两项，恢复 per_page=1 后真实点击第二页显示另一项。临时日期候选和集合配置均已恢复；见 `resume-notebook-sorting.json`。
- T6/T7：最终生产源码组合 `7ca5a989` 的真实 make build 已通过，新 8103 隔离进程加载当前二进制；`resume-runtime-combination.json` 逐字/hash 核对 Console 入口、六资源、main.js、Swiper extension 和嵌入 WASM。真实 `/sitemap.xml` 为 52 个去重 URL，首页 priority=1.0，保留 links/moments，见 `resume-sitemap-http.json`。此前 runtime JSON 只绑定各自历史构建。
- 证据边界：本轮公开 JSON 和截图经独立只读敏感内容审计，未发现真实凭据或私密迁移全文；20 capability 声明及六个绑定文件 hash 一致。部分工具截图虽然命名 .png，实际输出 JPEG，且部分图片为 1272×716；它们证明可见布局与交互，不作为无压缩的 1280×720 逐像素对比。保留原始字节，避免使既有 hash 绑定失效。

本轮基线截图和 JSON 都位于 `docs/baselines/stellar-200-full/`。隔离服务为 8103，DB/config/content/uploads/cache/runtime 均来自独立根；动态 fixture 为仅监听 loopback 的 8110。真实外部服务与 fixture 证据分别记录。所有私密迁移候选保留在忽略目录，不能提交或输出正文、凭据。

### 2026-10-14 最后组合续接（仍实现中）

已确认的集合显示名、Wiki/导航、banner/cover、Footer 和 Notebook owner/结构选择继续有效；本轮没有再次询问，也没有执行真实迁移。

- T2/T7：Leftbar 原十二组级联与 Topbar menu/widgets、Rightbar widgets 十二组 omit/empty/replace 组合完成真实保存→重开→前台；Topbar 使用官方允许的 menu/spacer，初次不支持的 Markdown fixture 保留失败记录。global replace 正式补采 `topbar:0:menu`；Card 手机原生 `main.inert=true`、Escape 后 false 且焦点回 Open leftbar，关闭的 Rightbar 仍 inert。证据为 `resume-t2-cascade-browser.json`、`resume-t2-upper-regions-cascade.json`、`resume-card-mobile-inert-current.json`。
- T3/T7：正式横幅 disabled/no-bg/empty-headline、references/license、六分享 SVG、QR 点击加载和 avatar currentSrc 已复验；taxonomy 分类/标签菜单及 Wiki `index_wiki/index` 真实显示。早期 `.post-card` 误采零张卡片，已保留并用 `main a.wiki-card` 补采五张；390×844 分类无横溢，Wiki 稳定 PNG 保留。见 `resume-t3-banner-footer-corrected.json`、`resume-share-icons-avatar-browser.json`、`resume-taxonomy-wiki-index-browser.json`。
- T3/T7：OG 使用固定上游完整 head 与 Hexo helper oracle：168 inputs、384 正常输出及120错误边界，覆盖SSR/offline/opaque preview。WHATWG URL 修复后 Unicode OG URL、punycode 图片、data 图片过滤、description、Twitter/FB/author 参数在 Console 保存→重开→正式SSR/iframe 实测；canonical 仍本地。scope-only 的隔离误采与原生 CDP 结果分开记录，见 `resume-og-console-browser.json`。
- T5/T7：属性面板新增字段曾在 Apply 后丢失，已修复精确 present 累积；真实新增 image height、Box 多行/空格 title、Copy 引号与空 prefix、Emoji URL mode、inline Frame 在 Apply→Markdown→保存→重开→前台通过。正文最后恢复。54/288 oracle 是参数契约证据，不等同所有字段逐个 UI 操作，见 `resume-editor-new-parameter-red.json`、`resume-editor-new-parameter-green.json`。
- T3/T7：5815e94 产物真实四种 Hero canvas 绘制，实际链接整页离开时各取消一帧、释放GL program/buffer；兼容 partial 保留同一 cover 是预期行为。终端标签切换后实际多行剪贴板为 `go test ./...\ngo vet ./...`，剪贴板已恢复。视频已加载并播放；此采样不声称显式暂停或原生 BFCache，见 `resume-hero-current-lifecycle.json`。
- T4/T7：605d1186 浏览器发现旧动态 Algolia provider 缓存：HTTP 新代码但已执行 mount 无 queryPageKey，runtime query 未变。已扩展入口指纹到本地动态 JS/CSS；新浏览器实际 queryPageKey=true。精确等待目标 URL/heading 更新、timeOrigin 不变后再释放旧请求，结果为空；新页查询显示合法结果。临时配置/Fetch 拦截已恢复，见 `resume-algolia-partial-current.json`。早期名字带 green 的记录保留为 INVALID EARLY SAMPLE；官方 SDK 配合合成 API 不是真实 Algolia 账号。
- T6/T7：605d1186 单二进制同PID内容事件捕获新公开本地图，metadata 与正式SSR回填960×540/平均色；首个GET尚无元信息的采样保留。禁用自动仍复用cache；新图和原cover hash不变。Note正文及设置保存重开恢复，见 `resume-images-event-current.json`。
- T4/T6：独立审查又发现图标JSON由真实IconRegistry生成，不属于静态assets。5b92b246已把稳定排序的公开生成资源正文纳入runtime指纹，HTTP和两个离线CLI使用同一registry，真实Mount/icons.yml红绿、dev/embed一致性及独立复审通过；最终浏览器query为deae89e4且实际执行queryPageKey=true。四隔离服务Console入口/六资源、main.js、provider/index.js及当前WASM嵌入字节匹配，见`resume-runtime-final-source.json`与`resume-provider-final-binding.json`。
- T1：最新605d1186正式单二进制fresh两次只读preview成功；Source/Root/selectedCandidate/Review及七正文摘要全不变。72项前后指纹仅SHM mtime变化；Approved=false/BodyDigests空/offline全false，未演练或Apply。完整公开报告已追加最新来源复核。
- T7：本轮 make css 与 css-drift-check 成功，20份生成物一致；共享主题仍 unborn HEAD/未跟踪源文件，固定 commit object 可用，未切换/重置。新增证据通过独立只读敏感内容审查；历史扩展名 .png 实际JPEG的字节保留，见 `resume-evidence-review.json`。

- T4/T7：5b92b246 的三张置顶轮播经真实 Leftbar 链接离开，pagehide persisted=false 时移除11个事件监听、abort颜色工作、暂停轮播并清空refresh回调；这是整页卸载证据，不冒充partial disposal。首次移动slide定位器点击未改变URL的采样保留，见`resume-pin-current-lifecycle.json`。
- T7：74ef648 组合的普通hook与feature `make merge-check` 全部通过；该门禁实际合入根dev，再跑dev组合 `make test`，Console261/Stellar202、fail0/skip0，Go vet/Staticcheck/全部Go测试及模块/部署来源检查通过。根dev真实make build也已通过。四服务改用该dev二进制，但-root仍是原四fixture；Console入口/六资产、main/provider/runtime和当前嵌入WASM字节匹配。独立lsof检查四DB/WAL/SHM与16把distinct canonical guard锁，未打开真实根DB；当前dev浏览器实际Galaxy/cover/正文及runtime query71bc6f7a正确。见`resume-runtime-dev-combination.json`、`resume-dev-browser-current.json`；此记录绑定74ef648产物，后续纯文档提交不替代或篡改这次来源。
- T1/T6：用户已批准候选、七条正文及完整隔离演练，明确暂不修改真实源。演练前当前dev单二进制的正式reviewed preview退出1：此前集成的bdf10768改变tracked图标注册表，使旧完整Root/Review绑定过期。已独立逐资源定位，并以正式初次/选值preview两次退出0生成新审查；68资源inventory只该图标data改变，Source/原与选值Candidate/七正文均不变。新Root为8e2206a9…cc371、新Review为d2e8b831…51711，按用户已约定的“摘要变化重新审查”边界待确认；不重复问正文/name/owner。72项本次前后指纹只SHM mtime变化，template仍未批准，未演练、Apply或停服。见迁移报告“隔离演练批准后的来源绑定更新”及`resume-migration-rehearsal-rebinding.json`。

- T1/T6：新来源绑定已获批准；前两次正式演练失败分别暴露缺初始化文件的生产启动遗漏与空站点 URL。98162b2修复从同一持久快照启动，ec63ffad接入用户明确站点地址的具名候选／事务写入。当前组合真实build后，正式初次／选值preview及完整rehearse均退出0，Source/Root及七正文摘要保持；32候选／173映射、原13阻断具名处理为0。主agent只读核对41非迁移表、全部settings、30canonical和七正文一致，source72项仅SHM mtime变化。见迁移报告最新章节及`resume-migration-site-url-rehearsal.json`；真实迁移未授权，后续组合门禁继续执行。

- T3/T7：a80bdd15 修复 Hero 视频 region 取消/清理与尚未导入时的同步 pagehide 暂停。真实浏览器正常离页、模块 Script 请求精确挂起后离页，均记录一次 pause-call、pagehide persisted=false、随后 paused=true；返回为新文档 back_forward，重新播放。Fetch 拦截、缓存覆盖与探针 storage 已恢复，用户 Console 草稿未操作。见 `resume-hero-video-browser-green.json`、`resume-hero-video-runtime-binding.json` 与 `resume-hero-video-current.png`。视频第二章无 cover，属于整页回退，不冒充兼容 partial；原生 BFCache 仍未实测。追加 Hero 后最终组合门禁继续执行。

- T1/T3/T6/T7：a61d0cd5（含两项 Hero 修复）的正常 hook、feature `make merge-check` 与 dev 组合 `make test` 均通过：Console261/Stellar211、fail0/skip0，Go vet/Staticcheck/全部Go测试及模块/部署来源检查通过。dev 真实 `make build` 成功，四隔离服务改用 dev 二进制17de7e55…34cfc5，Console入口/各六资源、main/runtime/hero/provider及嵌入WASM匹配。最终dev浏览器真实播放→整页离开暂停一次→新文档返回播放，探针已清除。见 `resume-hero-final-combination-gates.json`、`resume-hero-video-dev-runtime-binding.json`、`resume-hero-video-dev-browser.json`。
- T1/T6/T7：同一最终dev单二进制正式初次preview、选值preview和完整rehearse均退出0，Source/Root/原与选值Candidate/Review及七正文摘要与已批准指定URL演练完全相同。新隔离stage只读核对41非迁移表、全部settings、30canonical、67对象/索引、46rowid集合、两tracked完整文档及FTS身份/原标题；quick_check/FK正常。真实来源72项仅SHM mtime变化。见 `resume-migration-site-url-final-rehearsal.json`、`resume-migration-site-url-stage-final-review.json`；真实Apply/Recover/停真实服务仍未授权。

- T4/T5/T7：用户反馈原生评论 loading 与预览表情显示错误。以 `native-comment-user-preview.png`、`native-comment-user-loading.png` 为局部现状参照，冻结修复目标为：加载圈和提示同行、表情与相邻文本同行、普通附件保持图片宽度，原生评论布局和配色保持。真实红样本发现上游 `svg.loading` 的 absolute/100% 样式和 `.md-text img` 的 block/auto margin 污染原生组件；身份请求挂起时尚未建立列表懒加载。现以独立请求解除身份依赖，局部 CSS 重置 spinner 并复用列表表情内联规则，预览按服务端来源规则区分表情/附件。两项回归先红（44/46）后绿（52/52）；真实单二进制浏览器确认主动拦截身份请求时公开列表已成功、注入503提示替换loading、32px 内联表情及568px普通附件、390px无横溢。拦截、缓存覆盖及视口均恢复，用户原页/草稿未修改，没有提交评论。见 `resume-native-comment-red.json`、`resume-native-comment-green.json`、`resume-native-comment-runtime-binding.json` 与相关截图。完整组合门禁继续执行。
- 评论使用边界：用户实际使用内置评论，无需外部评论账号或接入。上游六种第三方适配能力继续保留；真实账号未实测属于可选适配器的证据边界，不作为用户使用内置评论的前置条件。

- T4/T5/T7：原生评论修复 fe7246d0 的普通hook、feature `make merge-check` 与dev组合测试通过，Console261/Stellar213、fail0/skip0，Go vet/Staticcheck/全部Go测试及模块/部署来源检查通过；dev真实build成功。四隔离服务加载当前dev二进制2961d1ee…187ed90，Console入口/各六资源、原生comments.js/comments.css及runtime/嵌入WASM逐字hash匹配。最终dev浏览器身份和公开列表请求完成、loading消失、空状态正确、两个表情32px inline-block。极早自动点击发生在事件初始化前，未打开预览的初次采样保留为无效预览证据；初始化后真实点击成功，不篡改早期结果。见 `resume-native-comment-final-gates.json`、`resume-native-comment-dev-runtime-binding.json`、`resume-native-comment-dev-browser.json`、`resume-native-comment-dev-current.png`。本局部修复为待体验，完整移植仍实现中；用户原页未重载或提交评论，真实来源/迁移和生产未操作。

| 编号 | 正式链路与可核验证据 | 仍需保留的边界 |
| --- | --- | --- |
| T1 | strict schema/模型/Doctor/公开inventory与32条候选完整映射；七条正文/候选及隔离演练已批准 | 新完整Root/Review绑定与指定URL已接入，完整隔离演练通过；真实迁移明确未授权 |
| T2 | 四预设/三配色/14 profiles/三locale/断点矩阵；appearance字段及四级区域级联真实消费者 | 逐项证据不替代用户整体体验确认；最初失败采样保留 |
| T3 | 全内容系统保存重开/导航搜索；模型、Wiki/Notebook排序、OG、分享及Hero真实链路 | 视频正常/迟导入离页显式pause已真实验证；原生BFCache仍未实测，不推导全参数UI逐项验收 |
| T4 | partial成功/取消/超时/失败回退、真实SDK合成Algolia、六评论合成SDK及访客身份 | 真实Algolia/六评论账号、原生BFCache未实测；pin分页整页导航不当partial卸载证明 |
| T5 | 54标签288参数oracle；可视化新增语法往返；动态服务、媒体、Gist隔离生命周期 | 外部Markdown受fake-IP/保留地址保护拒绝，成功路径仍未实测；不降低网络保护 |
| T6 | 实际单二进制CLI与嵌入librsvg/字体；fetch边界、缓存/事件/取消与原图保留 | PNG/GIF/AVIF库内部仅阶段边界取消，不声称即时抢占；真实迁移需批准 |
| T7 | 组合源码、真实CSS/build、服务资产hash及前台证据各自绑定 | a61d0cd5含Hero后的feature/dev门禁、真实build/hash与完整隔离演练已完成；外部账号、原生BFCache、真实迁移和用户完整体验仍未关闭 |

### 历史起始阶段（不表示当前实现状态）

- 起始时仅完成源码调研、根 DB 只读结构/计数预检和上游主页基线保存；当时没有宣称实现完成。
- 将独立上游导出的全部 628 个普通文件与锁定 commit 的 `git archive` 逐文件 SHA-256 比较，0 个缺失或不一致；完整源指纹见 `docs/baselines/stellar-200-full/upstream-inventory.json`。这只证明输入源码完整、准确，不证明移植完成。
- 根 DB 通过 SQLite `mode=ro` 打开，未修改配置、内容、评论或用户；未启动/重启真实服务。
- 已移入 42 个之前完全缺失的上游浏览器资源，保留 20 个存在差异的脚本等待适配；清单见 `docs/baselines/stellar-200-full/browser-resource-port.json`。此记录仅证明资源准备，不代表 manifest、生命周期或页面已经接通。
- 旧“仅深色／保留完整 SSR／不移植 Hexo 工具”的工程约束须按本任务批准选择更新，不得继续作为缩减目标的理由。

### T3 首屏布局修正（阶段证据）

- Wiki Hero 从 `partial/wiki_doc` 移至独立 `partial/wiki_cover`，由 base 的 `site-cover` 渲染，位于 shell/workspace 之前。
- DOM 回归用真实路由和隔离 SQLite：旧实现失败（Hero 不在独立 cover slot），修复后通过；Galaxy 参数的 HTML/JSON 转义测试保留。不是仅断言模板出现某个 class。
- 主 agent 用真实新构建服务、独立样例 DB/config/content/uploads 验证 `http://127.0.0.1:8102/wiki/stellar-test/intro`。1280×720 下 Hero 1272px 宽、720px 高，工作区 y=720，Galaxy 挂载；文档按钮跳至 #start（工作区 top=44）。截图与 DOM 记录在 `docs/baselines/stellar-200-full/after-wiki-cover-desktop.png`、`hero-layout-check.json`。
- 390×844 下 DOM 显示 Hero 与 body 同宽且无横向溢出；浏览器截图表面与模拟 CSS 像素不一致，因此没有把该截图认作窄屏视觉验收。相关完整响应式验收仍未完成。
- 本次 `make build` 完成 Console/UC 与新 Go 二进制。这里只验证独立 cover slot；新 Hero 模型、指定首页、视频/四种动效/terminal 的完整行为仍须后续接入与验收。

### T1/T2/T5 接口与资源接通（阶段证据）

- 严格主题保存独立 `stellar_v2`（version 2、原始 snake_case theme）和 `blog_extensions`；未迁移库返回 `MigrationRequiredError`，无隐式初始化或迁移。原子事务验证无效候选不写入、第二段失败整体回滚、并发快照和重新打开；对应分支定向 race 与完整 hook 通过。生产启动和 Console 接入尚待完成。
- CSS 已生成全部 20 份产物，包括官方 5 种评论独立 CSS 与 Fancybox/Swiper；修正字重变量带引号的无效声明。漂移检查在临时目录重编全部产物，失败不覆盖工作区。浏览器加载仍待接通新模板后验证。
- 新 `ParseStellarDocument` 使用严格上游 Page schema，分离 `blog` 扩展，保留原始正文、false/null/数值与原始字段名，支持 YAML/JSON front matter。导出采用 Hexo JSON front matter，保留开放参数袋大整数。当前定向导入/导出/归属检查通过；现有编辑和备份消费者尚未改用新入口。
- Gist 端点已挂载，按当前严格主题 `services.github.gist_url` 生成官方脚本 URL；脚本只能在无同源权限的 sandbox 文档内执行。请求校验、file 编码、防脚本注入与主题热替换检查通过；region 尺寸监听检查通过。真实浏览器动态加载、manifest 和内容标签接通仍待完成。
- 上述模块组合后的主实现分支已在 `08a5d39` 正常提交 hook 中执行完整 `make test`，全绿。这证明本次组合代码门禁，不代表尚未接入的完整前台可以使用。
- HTML 后处理库已合入：662 个官方差分覆盖普通链接/表格、图片 lazyload/onerror、图标命名空间及失败图资产；新包需按 README 在真实渲染与资产发布链调用，当前不能仅凭包内检查称为接通。

### 本地集成记录

- HTML 专家误执行了会自动合并的 `make merge-check`，根 `dev` 从 `49fe104` 快进到既有基础模块祖先 `02ee112`，不含该专家新增 HTML 包。已停止第二阶段检查，未 reset、回退或清理用户文件；`.pi/` 保留，真实服务未重启。第一阶段检查通过，第二阶段中断，不能称根组合态验收通过。
- 主实现仍在 `codex/stellar-200` 继续接通；最终完整检查与正式本地合入门禁必须继续执行，不能把这次误提前快进当作任务完成。

### T1/T5 Console 与 Gist 本次实测

- 新增后台 `Stellar 2` 标签页与完整 JSON GET/PUT/只读校验；未迁移库 409，严格主题校验不接受旧 style，已迁移库支持独立 Blog 扩展。共享锁内保存与 Site 指针发布；明确路径的密钥脱敏及 `****` 保留原值。
- 主 agent 真实 `make build` 后启动独立 `127.0.0.1:8103` 夹具（DB/config/content/uploads 全隔离，虚构管理员和虚构密钥），比对当前 Console HTML 及 6 份入口资源逐字和 hash 匹配本次产物。真实根数据库未迁移；根 8080 当前未运行。
- 浏览器完成配置读取→校验→保存→重新读取：`9007199254740993`、false、null 保真，原生评论通知密码与 AI key 只显示 `****` 且 DB 原值保留；旧 style 校验显示定位诊断；跨标签草稿保留，无 Console error。新页缺少内边距的实际偏差已修正为现有设置页的 28px/窄屏 16px，重新构建与重启后桌面实测 28px、编辑区无溢出。窄屏视觉尚未实测。
- 当前截图与检查：[Console](baselines/stellar-200-full/after-console-stellar2.png)、`console-settings-check.json`。本次证明后台存储和配置快照更新，前台新模板/颜色/布局消费尚待接通，不能称所有配置实际显示已通过。
- 官方公开 Gist `octocat/9257657` 在独立测试 HTML 的 sandbox iframe 中动态加载，实际使用新增端点和 `blog-gist.js`，尺寸自动更新为 850px。CDP 的真实执行环境确认父页 `contentDocument === null`、读取子窗 location 抛 SecurityError，sandbox 只有 allow-scripts，无浏览器错误日志。[Gist](baselines/stellar-200-full/after-gist-isolated.png)、`gist-isolation-check.json` 保存证据。该测试 HTML 只存在隔离 uploads；文章标签和完整 manifest 接入仍待完成。
- 严格内容导入/导出新增只读 author API，返回独立配置文本，避免 JS Number 转换损失；旧字段错误、无副作用往返和认证边界定向通过。编辑器表单、备份和内容保存版本门禁仍需接入。

- T4/T7：用户反馈原生评论“登录”点击无效。局部参照为 `resume-comment-login-red.png`，冻结目标：保留既有弹窗/评论视觉，登录按钮打开身份选择，读者登录保留草稿，提交后重载可见，关闭/Escape与换页重挂正常。真实红证据初始化标记为1但 modalOwned=false；实际 Stellar 模板误将弹窗放在评论根之外，scoped runtime 无法查询/绑定。现仅将弹窗纳入同一 `#blog-comments` 根，未扩大全局查询。真实 embed SSR DOM 祖先断言先红后绿，scoped JS回归验证打开/Email/关闭/Escape/取消/解绑/重挂单次登录。隔离 localhost:8112 用虚构读者完成 login200→保留草稿→comments200→reload count1；错误密码401与明确服务端错误、普通换页后再次登录200且一次请求已核验。该换页发生Document导航，不作真实partial/BFCache证据。管理员前台登录维持403和“管理员账号请从后台登录”，不扩大权限。8103与8112仅新增可测试虚构读者，真实源未变，用户原页/草稿未操作。见 `resume-comment-login-red.json`、`resume-comment-login-green.json`、`resume-comment-login-posted.png`。当前局部门禁与最终dev运行态验证继续执行。

- T4/T7：10187eac普通hook、feature与dev组合门禁全部通过（Console261/Stellar214，fail0/skip0，Go vet/Staticcheck/全部Go测试及模块/部署来源检查）。dev真实build通过，四隔离服务重启并加载dev二进制f19d0918…31b10b；Console各六资源、comments/runtime及嵌入WASM匹配，DB句柄只属各隔离root。8103最终浏览器确认modalOwned=true、唯一弹窗、Email表单可见，普通换页与历史返回后仍能打开；两次均Document导航，未将此称为partial/BFCache实测。新增截图 `resume-comment-login-dev-modal.png`、运行绑定 `resume-comment-login-dev-runtime-binding.json`、最终DOM证据 `resume-comment-login-dev-browser.json`。局部修复待体验，已在新独立8103页面留下空登录表单；原用户页和草稿保留，真实来源未迁移、未push/发布/部署。

- T4/T7：用户反馈注册总报“验证码错误或已过期”。本次局部参照 `resume-register-red.png`，冻结目标为既有原生弹窗内补验证码题目/答案/换一题，首次进入注册获取挑战，加载/失败禁提交，携带token/answer注册，任何注册失败后刷新一次性题目并保留其他表单与评论草稿。真实红样本证明无验证码输入、未请求challenge、POST400；后端既有10分钟/一次性验证码和限流保持。现复用既有GET /api/auth/register/challenge与组件scope，不新增接口/权限或外部依赖；刷新/提交互斥，错误文本在表单持久显示。三项回归先红0/3后绿3/3，comments+lifecycle51全绿，Go web/identity通过；真实单二进制已显示挑战，注入隔离503后禁注册且可换题恢复，390×640无横溢，关闭与注册控件可达，拦截/视口已恢复。完整浏览器注册→评论等待浏览器工具要求的自动填写验证码确认，未把mock/构建写作注册成功，用户原页/资料未改。见 `resume-register-red.json`、`resume-register-verification.json`、`resume-register-mobile.png`、`resume-register-mobile-error.png`。

### T4/T7 邀请码注册接续（实现中）

- 用户已明确将注册验证码替换为管理员生成的一次性邀请码，默认有效期7天，生成时可选1/7/30天。d73d96a0 的验证码接线是中间成果，已被本方案替代，没有以验证码注册成功为结论。
- 本局部UI参照：`docs/baselines/stellar-200-full/resume-invitation-users-before.png` 为现有用户管理唯一界面基线；注册弹窗沿用 `resume-register-mobile.png` 所记录的现有组件（邀请码替换验证码行），用户中心沿用既有注册页。保持列表、弹窗、输入和语义token，不修改生成CSS或重设计Console。
- 可观察验收项：管理员用户管理生成（默认7天/期限选择/只显示一次/复制）；公开评论与UC统一邀请码注册；资料错误和重复用户名不消费；同码第二次拒绝；注册自动登录后可发评论且重载保留；错误资料与评论草稿保留；390px可达无横溢；离页取消和迟到回调不覆盖当前页面。
- 独立审查发现正常启动缺少建表入口，已通过正式schema migration4接入 `Store.New`，新旧库真实启动先红后绿；同时用户中心已接入取消信号和卸载保护。真实源不做迁移，服务验证限完整隔离root。组合门禁、当前build/hash与浏览器结果待本轮后续记录。
- T4/T7：正式邀请码存储、启动schema migration4、管理员API已集成；公开注册只接受invitation_code，成功事务绑定一次消费，资料错误/重名不消费，生成默认7天可选1/7/30天，明文只在本次弹窗展示。真实8112完成默认/1天生成、关闭重开清明文、无效码400→重名409→同码换用户名201、自动登录及评论保存重载；UC复用已用码拒绝、1天码注册通过并保持reader。复制初次采样false保留，后续按钮toast及实际剪贴板匹配true；不把第一次采样改写为通过。
- T3/T4/T7：实际双cookie发现UC资料读取管理员的P1，已分离ucAuth，只接受有效公共reader/author身份，所有UC资料/改密/2FA/Token/通知接口继承它。改密响应仅删除blog_user，Console保留blog_admin优先级；真实HTTP检查实际Set-Cookie、旧读者会话401、管理员会话仍200。UC账户缓存中央取消清空、通知key含user ID；列表和写操作贯穿取消signal，避免旧401清新身份。Profile/Picker/Markdown异步离页、逆序、迟到响应回归及UC文章附件上传/选择分流已接通。普通hook子提交8c8003e0独占7文件逐文件集成，复制验证依赖未混入该提交。
- T4/T7：公开登录校验已启用的TOTP，六位验证码或八字符一次性恢复码均支持；恢复码CAS绑定账户密码/密钥/状态/角色/原恢复码JSON，不新增schema。跨2Store并发race、错误密码、写入失败、限流、复用与账户变化检查通过；当前单二进制真实HTTP完成reader注册、改密、开启TOTP、缺码401、正确码200、错误密码不消费恢复码、恢复码首次200/复用401、公共注销不清管理员。浏览器没有启用TOTP或改密，凭据变更证据为真实HTTP及后端测试，不冒称界面完整实测。
- T4/T7：新增评论即时计数原先错误重置为1，现累加并同步root/child排序模型，先红后绿；真实从2到3，重载仍3。UC双cookie浏览器资料保存重开属于读者，管理员资料未变；通知实际GET /api/v1/uc/notifications 200；虚构author编辑器本人附件库原空，自制PNG通过POST /api/v1/uc/attachments/upload 201，重开仅见本人图片，未发布文章。邀请码390×640 DOM无横向溢出，关闭与提交可达；IAB截图比例和CDP裁切与CSS视口不一致，相关图保留但不作完整窄屏视觉验收。
- T7：本轮当前源码Console273项及comments/lifecycle53项零失败/零跳过，聚焦store/identity/ConsoleAPI/Stellar web Go检查与真实make build通过。四完整隔离root重启到b3a9be1a…b3078，Console每站六资源、UC入口/资源、原生评论/runtime/WASM逐字及hash匹配。证据为resume-invitation-browser.json、resume-invitation-deployed-http.json、resume-invitation-combination-runtime-binding.json及对应截图；普通主树hook、feature/dev最终门禁和dev产物重验继续执行。真实源未改、未push/发布/部署。
- T7：生产代码f14e454c的普通hook、feature make merge-check和dev组合复验全部完成：Console273/Stellar220，fail0/skip0，Go vet/Staticcheck/全部Go测试及模块/部署来源检查通过。dev真实make build后四隔离服务加载3e80e6ef…576bb，Console每站六资源、UC、comments/runtime和嵌入WASM均匹配；浏览器最终确认新注册角色显示“读者”、UC作者资料与后台管理员分离、原生弹窗含空邀请码输入且无captcha，密码/两步码分别有独立label。最终记录resume-invitation-final-gates.json、resume-invitation-dev-runtime-binding.json、resume-invitation-dev-browser.json、resume-invitation-dev-admin.png。localhost:8112同站留下空注册弹窗和后台默认7天生成入口，测试作者已退出公共会话，管理员后台保持登录。该局部为待体验；完整T1–T7仍实现中，真实来源未迁移、未push/发布/部署。后续证据提交仅更新文档，不改变上述生产代码与运行产物。
