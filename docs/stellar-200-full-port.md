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

| 编号 | 正式链路与可核验证据 | 仍需保留的边界 |
| --- | --- | --- |
| T1 | strict schema/模型/Doctor/公开inventory与32条候选完整映射；迁移报告绑定全部来源 | 七条正文、完整Root/ReviewDigest及真实迁移操作仍未批准；不因选值确认自动Apply |
| T2 | 四预设/三配色/14 profiles/三locale/断点矩阵；appearance字段及四级区域级联真实消费者 | 逐项证据不替代用户整体体验确认；最初失败采样保留 |
| T3 | 全内容系统保存重开/导航搜索；模型、Wiki/Notebook排序、OG、分享及Hero真实链路 | 视频离页显式pause和原生BFCache没有新增成功证据；不推导全参数UI逐项验收 |
| T4 | partial成功/取消/超时/失败回退、真实SDK合成Algolia、六评论合成SDK及访客身份 | 真实Algolia/六评论账号、原生BFCache未实测；pin分页整页导航不当partial卸载证明 |
| T5 | 54标签288参数oracle；可视化新增语法往返；动态服务、媒体、Gist隔离生命周期 | 外部Markdown受fake-IP/保留地址保护拒绝，成功路径仍未实测；不降低网络保护 |
| T6 | 实际单二进制CLI与嵌入librsvg/字体；fetch边界、缓存/事件/取消与原图保留 | PNG/GIF/AVIF库内部仅阶段边界取消，不声称即时抢占；真实迁移需批准 |
| T7 | 组合源码、真实CSS/build、服务资产hash及前台证据各自绑定 | 最终feature/dev门禁与新产物绑定结果在实际完成后追加；用户确认前不标已验收 |

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
