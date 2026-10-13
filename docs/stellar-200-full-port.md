# Stellar 2.0.0 完整移植

状态：实现中。此文替代此前只覆盖 CSS、骨架与旧功能兼容的升级范围；不将此前测试通过当作本次完整移植通过。

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

普通编码细节自主处理。新的数据损失、重要兼容性或体验冲突交用户确认；仅暂停依赖部分。未授权 push、发布、部署、远程消息、删除 worktree/分支与真实付费调用。

## 本次记录

- 当前仅完成源码调研、根 DB 只读结构/计数预检和上游主页基线保存；没有宣称实现完成。
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
