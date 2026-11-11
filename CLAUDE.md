# Blog 工程参考

这是与工具提供方无关的工程事实与实现约束，供 Codex、Claude、Qoder、Trae 等 agent 按需查阅。执行流程、worktree、交付和 UI 验收规则以 [AGENTS.md](AGENTS.md) 为准；两份文件有意分离，避免每个任务都加载全部技术细节。

代码、`Makefile`、`go.mod`、`package.json`、CI 工作流和受跟踪配置优先于本文；发现不一致时，以代码为准并同步修本文档。

## 当前架构与模块边界

项目是根目录单一 `go.mod` 的模块化单体。`cmd/blog` 是唯一 composition root：负责配置、共享服务、生命周期和将 Console 与 Stellar 挂到同一个 Gin router。

| 位置 | 责任 |
| --- | --- |
| `cmd/blog/` | 进程参数、服务装配、HTTP 服务启动与关闭 |
| `modules/console/` | `/console/` 管理后台、`/uc/` 用户中心、Console API 与前端产物嵌入 |
| `modules/stellar/` | 公开 Stellar SSR 路由、模板、静态资源、主题适配与公开模块挂载 |
| `internal/` | store、内容、评论、设置、搜索、媒体、插件、RAG 等共享业务能力 |
| `packages/` | Console workspace 共享的 API Client、组件、编辑器、运行时和工具包 |
| `config/`、`content/`、`data/` | 配置、Markdown 内容、运行时数据库/上传/缓存；`data/` 是运行时数据 |

`scripts/check-module-deps.sh` 是边界的机械检查：`internal/` 不得导入 Console/Stellar 业务模块；Console 与 Stellar 不得互相导入；`cmd/blog` 只依赖顶层模块包；公开 URL 路由由 Stellar 挂载。遇到跨模块需求，优先增加明确的依赖接口或共享 callback，而不是直接穿透导入。

## 构建、运行与验证

### 构建入口

- `make test`：完整本地门禁。它会为 `go:embed` 写最小 `dist` 与 `dist-uc` 占位（后者供 `cmd/blog` 的 `TestStellarMigrationActualBinary` 挂载 `/uc/`），运行 Console 类型/行为测试、Stellar 资源测试、部署/发布来源/模块检查，以及 Go vet、Staticcheck、全部 Go 测试。
- `make build`：真实构建 Console（包含 `/console/` 与 `/uc/` 产物）后再编译 Go 二进制。只要涉及 SPA 交付、embed 或发布，不能用 `make test` 代替它。
- `make dev`：根据 Console 输入指纹按需重建前端，随后以 `-dev` 启动服务；`make run` 是 embed 模式。
- `pnpm run e2e`：Playwright 关键路径验收。配置会重建 Console、用独立数据库启动 `-dev` 服务并使用 8096；在需要页面交互证据时运行相关用例。

Go 版本由根 `go.mod` 决定，pnpm 版本由根 `package.json` 的 `packageManager` 决定，Node CI 版本以 `.github/workflows/ci.yml` 为准；不要把本机版本当作项目契约。

### 发布流水线

- 发布只产出 Docker 镜像：`.github/workflows/release.yml` 已移除，仓库不再构建或分发单二进制；`deploy/update.sh`、`deploy/update-docker.sh` 与 systemd 更新单元一并移除，服务器统一用 `docker compose pull && docker compose up -d` 升级。
- `.github/workflows/docker-release.yml`：tag 推送时按根 `Dockerfile` 自包含构建并推送 `ghcr.io/sakuralovesmile/sakura-blog`，镜像标签不带 `v`（`1.7.4`、`1.7`、`1`）。`latest` 只由 tag 推送更新，`workflow_dispatch` 不会改写它。tag 推送后该工作流还会用内置 `GITHUB_TOKEN` 创建一个不带资产的 GitHub Release（`--generate-notes`），作为 Console「检查更新」的更新日志来源；镜像仍是唯一发布产物。`ci.yml` 的 `image` 作业在晋级 `release` 的 PR 上预构建 Dockerfile（不推送）。发布步骤见 AGENTS.md「发布 release」。
- v1.7.3 起 Console 只保留「检查更新」（`GET /system/update/check`），一键更新执行链（`internal/updaterunner`、apply/status 端点、systemd path/timer watcher）已移除；版本来源是 GHCR 镜像标签（`internal/ghcr`，公开包匿名 token，6h 缓存）；更新日志、发布时间与发布说明链接来自 tag 对应的 GitHub Release（仅说明、无资产），只有 Release tag 与最新镜像版本一致时才展示，取不到时静默降级。升级一律在宿主执行 `docker compose pull && docker compose up -d`。

### `go:embed` 与前端重建

`modules/console/web/embed.go` 要求 `dist/` 和非空的 `dist-uc/` 在 Go 编译时存在。`make test` 只为前者补最小占位，`dist-uc/.gitkeep` 只满足编译约束；两者都不验证真实 SPA。始终优先使用 `make build` 生成 Console 与 User Center 的真实产物，不要手动颠倒「前端构建 → Go 编译」顺序。

服务启动时会把 `/console/` 和 `/uc/` 的入口 HTML 读进内存。即使 `-dev` 从磁盘提供静态资源，重新构建前端后也必须重启 Go 服务，浏览器才能收到新的入口文件和 chunk 引用。验证时同时确认运行端口、服务启动时间和浏览器加载的资源 hash。

### 运行时根目录与配置

- 从任意调用环境启动服务时显式传 `-root <worktree-absolute-path>`；不要根据临时 `go run` 二进制位置推断项目根目录。
- `config/app.yml` 的字段可被 `BLOG_*` 环境变量覆盖。配置或部署问题先检查优先级与实际进程环境，不要在日志或文档中输出密钥。
- `-seed` 会按 slug UPSERT `content/posts/*.md`，因此会用文件内容覆盖已有文章字段；日常启动的增量发现只补不存在的 slug，不会更新已有记录。修改导入、迁移或内容回填时必须覆盖已有数据库和新库两条路径的测试。

## Console、User Center 与 API 契约

Console 是 Vue SPA，所有后台数据经 `/api/v1` 访问；User Center 是独立的 `/uc/` SPA。不要恢复已退役的服务端 `/admin` 页面，也不要把公开 Stellar 页面链接到后台路径。

- Go API struct 是可生成接口的真值。新增响应 schema 用具名 struct，修改后若该类型应供前端使用，更新 `tygo.yaml` 并执行 `go generate ./modules/console/api`；生成的 `packages/api-client/src/generated.d.ts` 不可手改。
- `gin.H` 只存在于历史或不可生成的局部响应中；它不能作为新 API 的模式。新增 endpoint 时先确定稳定的请求/响应 struct 与错误语义。
- 前端 workspace 依赖由根 `pnpm-lock.yaml` 统一锁定。不要在单个 package 目录单独安装依赖或绕过 `pnpm --filter`。
- Console 的 `ri:` 图标来自受跟踪的 `modules/console/web/src/icons/ri-subset.json`，由 `node modules/stellar/web/scripts/gen-ri-subset.mjs` 按源码用法生成。增删图标用法（包括删掉旧页面或组件）后要重新生成；`make test` 的 `assets-test` 会同时校验“用到的都在子集里”和“子集里没有未使用图标”。
- 系统设置按 5 组 12 页组织（`modules/settings/settings-pages.ts` 是页面注册表：分组、页面、分区、字段归属和旧 `?tab=` 映射；字段本身只在 `settings-canonical-fields.ts` 定义，每个字段路径只属于一页）。桌面是左侧纵向导航，手机是“分组列表 → 详情”。除站点信息、访客反馈外的 10 页共用同一份主题草稿（`useStellarSettings` 的原文草稿与完整文档 API），由统一保存栏“保存全部主题设置”提交；`StructuredSetting` 直接消费 `internal/stellarv2/schema.json` 的类型、默认值和枚举，只写用户修改的节点。站点信息与访客反馈仍用宿主分段接口，各自独立保存；两类页面都有“放弃本页修改”，只还原本页。旧 `tab=stellar2` 落到「代码与原文」并展开完整配置原文，`ai`/`security` 仍跳转原目标。
- 行内橙点只表示相对上次保存有未保存修改；“恢复默认”就是移除草稿里的显式覆盖，只在存在覆盖时出现；总开关关闭时依赖项变淡但仍可编辑。浏览或切换页面不得写入配置、补写默认值。
- 设置页的 sticky 页头与底部保存栏会盖住滚到它们下面的内容。`SettingsView` 量取两者的实际尺寸（手机上页头会换行，保存栏随文字变高），以 `--settings-sticky-offset`、`--settings-bar-offset` 注入；分区锚点和内容区里可聚焦的控件用 `scroll-margin` 让出这两块，换页（含手机列表 ↔ 详情）后回到内容起点。Playwright 的可见性断言不看遮挡，验证锚点、换页与键盘聚焦要用 `tests/e2e/support/settings.ts` 的 `expectClearOfHeader` 或命中测试，不要只断言“在视口内”。
- 设置页在手机（≤767px）上每个可点击控件的点击区要 ≥44px，但沿用的小控件（共享 `VSwitch`、结构化编辑器里的“恢复默认”与折叠摘要）视觉不变：`settings-form.css` 末尾的“手机点击区”块用透明伪元素补上下空白，只往空白处延伸、不抢相邻控件的点击，上下各补多少按量到的真实空隙定；不透明度滑块在 `SettingColorEditor.vue` 里加高再用负外边距抵回。`boundingBox` 看不到伪元素补出来的点击区，验收用 `tests/e2e/support/settings.ts` 的 `measureTouchTargets`（逐控件命中测试，同时查被抢的点击）；新增小控件或调整结构化编辑器的间距时，同步看那组用例。
- 设置项的效果说明由 `structured-setting-guidance.ts` 和 `settings-field-effects.ts` 提供展示文案，类型/默认值/保存契约仍以原有 schema 与 API 为准；页面标题下只有一句话说明，不再有单独的“效果说明条”。`useSettingsPreview` 使用当前草稿请求只读整页预览（入口是页头的“前端预览”），支持选择页面与实际视口宽度；静态 iframe 移除脚本并展开懒加载图片，不代替真实交互或外部服务验证。
- 旧的 Console 分段页面 `/console/appearance`、`/console/menus`、`/console/authors` 已退役为入口兼容层：路由经 `modules/console/web/src/router/retired-pages.ts` 重定向到系统设置的「主题外观」「导航与页脚」「作者与友链」页（旧 `?tab=feedback` 仍落到访客反馈），页面组件已删除，`internal/module/core` 与前端 core plugin 都不再挂这些入口。新增设置页或调整页面键时同步核对这份重定向映射。
- 结构化编辑器的分组（`details.structured-item`）默认全部展开，不按路径设“默认折叠”白名单；收起只是用户的临时操作，不写入任何设置。宽屏下分组内容每层缩进 24px，手机只让嵌套的分组每层退进 8px，宽度留给控件。只有“代码与原文”里的完整配置原文是有意默认折叠的。
- Console 的局部 UI 变更除了类型/行为测试外，还需要加载真实服务后的浏览器验收；SPA 构建成功不证明后端仍在服务当前入口。

## 数据、配置与内容生命周期

### SQLite 与迁移

项目使用纯 Go 的 `modernc.org/sqlite`，driver name 是 `"sqlite"`。存储层维持单连接以串行化写入；在同一连接上读完 `rows` 后再发起写操作，避免锁等待/死锁。

`CREATE TABLE IF NOT EXISTS` 不会升级旧表。修改 schema 时沿用现有迁移模式：检查历史列/表，再执行可重复的迁移；为旧库、新库和回填语义各写回归测试。不要在 handler 内临时补 schema，也不要假设开发库代表升级路径。

### 内容、Markdown 与编辑器

`content_md`/原始 Markdown 是编辑与 shortcode 往返的持久化真值；渲染 HTML 是派生结果。导入与编辑功能必须保留未知或复杂 shortcode 的原样回退，不能为了可视化表单静默丢参、重排无法解析的内容或自动发布。

文章与页面编辑器都应走同一份无损导入语义：解析 front matter、保留原 Markdown，并在用户确认后才回填表单。涉及内容恢复、导入或备份时，明确区分预览、草稿、保存和发布，不做隐式写入。

## Stellar 前台实现约束

### CSS 与共享主题源

`modules/stellar/web/assets/css/main*.css`、`comments/*.css` 和 `plugins/*.css` 是从 `upstream.json` 锁定的 Stellar 2.0.0 commit 编译得到的生成物，不应手改。`resource/hexo-theme-stellar` 是被 gitignore 的本地输入仓库；编译通过 `git archive` 读取锁定对象，不消费共享工作树改动。使用 `make css` 重建，再用 `make css-drift-check` 在临时目录重新编译全部生成 CSS 并逐份比较；漂移检查失败不会覆盖真实产物。

`resource/` 跨 worktree 共用；统一目录中的工作树可用绝对路径链接根目录的 `resource/`。共享源可能处于 detached HEAD 或带本地补丁。每次编译前先执行 `git -C resource/hexo-theme-stellar status --short --branch`，确认正在消费的版本与变更；不要把某个历史 tag、commit 或旧的产物大小写成永久真值，更不能擅自 reset、checkout 或清理共享资源。

项目自己的辅助样式应保留在其受跟踪源文件中；上述主外观、选配和动态 CSS 均须由 `make css` 生成，其他辅助 CSS 不纳入主题重编或漂移比较。

### Stylus 编译 shim

Stellar 的 Stylus 源通过 `hexo-config()` 读取主题配置。本项目在 `modules/stellar/web/build/stylus/` 以 `config.json`、`shim.js` 和 `compile.js` 注入该函数，不改上游 `.styl`：字符串、布尔、数字、数组和缺失值必须分别保持对应的 Stylus 类型，尤其不能把布尔值写成字符串 `"false"`。

`compile.js` 生成 `main-{card,glass,minimal,flat}.css` 和兼容入口 `main.css`（与 card 逐字一致），保留 swiper/fancybox/Artalk 与 `local.styl` 的旧内容兼容层。独立选配包括 `main-card-hover-{spotlight,tilt}.css`、`main-katex.css`、`main-lazyload-blur.css`；启用选配后最后加载 `main-appearance-{preset}.css`，保持上游最终视觉层的顺序。动态入口 `comments/{beaudar,utterances,twikoo,waline,artalk}.css` 和 `plugins/{fancybox,swiper}.css` 分别编译对应上游文件，供原生运行时按需加载。新增入口时同步编译清单与 CSS 专属测试，不直接追加产物。

### 模板、资源与图标

- Go `html/template` 的结构必须与当前上游 Stellar EJS 和已存在的项目模板对齐。新增/改动前先对照 class、属性和嵌套，不凭记忆复刻过时的 layout 片段。
- `template.HTML` 已是可信渲染结果时直接输出；不要再次传给只接收 `string` 的 `safe` helper。任何作者可控原始 HTML 都应先走现有渲染/净化链。
- 前台采用 Stellar 2.0 `site-shell/site-region/brand/listing-nav` 骨架，运行时外观注入必须输出对应的新选择器；旧 settings 字段保留。原生 ESM runtime 通过 `stellar-runtime-config` 加载本地 shell/search 扩展，入口 `asset_ver` 聚合本地 assets 全部 JS/CSS（包括动态 search/services/plugins 和 vendor）及实际 IconRegistry 公开派生资源（包括继承 runtime query 的 `js/icons/*.json`）；生成素材按 path/body 稳定排序并快照，HTTP 与离线 CLI 使用同一注册表，文档和独立静态图片不参与；磁盘与 embed 使用同一规则。完整移植要求保留 SSR 的同时接通上游 partial navigation；本地扩展须先具备区域挂载、取消与卸载，不能只注册模块而跳过自建服务初始化。前台脚本依赖的 `ctx`、模板函数和 `asset_ver` 是服务端到浏览器的契约。新增依赖时在模板注入与消费端一起修改；静态资源引用使用 `asset_ver`，不手写 cache-busting 版本戳。
- 标签数据服务由 `blog-upstream-services.js` 划分区域所有权，`services.js` 提供共享请求生命周期和局部 DOMPurify 净化。服务脚本必须用该上下文处理远端普通文本、URL 与动态 HTML；完整解码/渲染后才记成功，失败或取消必须允许重挂重试。DOMPurify 的版本、许可证和文件校验在 `assets/vendor/dompurify/UPSTREAM.json`；不要绕开局部边界使用全局 `innerHTML`。
- 部署使用 `internal/settings/data/icons.yml` 的嵌入图标注册表；开发默认也读取这份受跟踪副本。图标为内联 SVG，未知 key 会安全地渲染占位。新配置使用当前语义化键；保留 `legacyIconAlias`，直到迁移数据确认不再需要旧 `solar:`/`ph:`/`bxs:` 键。
- 用户已批准完整移植中的浅色、深色与自动配色以及四种外观预设。当前是否接通以代码与浏览器证据为准，不能再用旧的仅深色约定缩减本任务。
- `/friends` 的宿主正文通过 `stellarFriendsContentHTML` 加上页面专属外壳，并在交换说明分区展示当前站点资料 YAML；包含站名、网址、绝对头像、简介与本站 Atom 订阅地址，使用既有 YAML 编码器和 Stellar 代码复制组件，存储的正文保持原样。Console 预览共用该处理，但资料取公开站点快照，资源仍用预览地址。居中标题辅助样式保存在 `friends-page.css`，只在友链页加载。

友链页视觉基准是 <https://xaoxuu.com/friends/>（同款 Stellar 2.0 卡片：64px 圆头像、12px 胶囊角标、1fr/2fr 两栏、描述 19.2px 灰字）；对比差异先查站点配置（如 `article.style=story` 会带来 18px 内容基准），不要为对齐另写覆盖样式。

友链角标最多 3 枚：首枚仍存 `links.label`/`color`（`linkwatch` 的自动「失效」标签与旧客户端依赖这两列），第 2、3 枚存 `links.labels` JSON；`/api/friends(-aggregated)` 的 `labels` 数组按「首枚 → 附加」顺序输出，前台 `friends_and_posts.js` 按颜色亮度选文字颜色。Console 只传 `label`/`color` 时保留既有附加角标。

## 编辑器与 shortcode

shortcode 的解析、表单 schema 和重建路径必须共享一份 catalog：以后端 `internal/post/shortcode_catalog.go` 和各 renderer 实际接受的 named/positional 参数为准，而不是复制第二份前端字段表。

文章与页面共用 `ShortcodeInsertPanel`；Studio 与面板通过 `useShortcodeDraft` 将源码交后端解析并同步参数。预览消费服务端完整文档和 token/resize 协议。富文本的 `stellar-tag-syntax.ts` 按完整外层区间保留 raw；未知或残缺结构也保存为源码节点，避免 Markdown 解析丢弃 marker 注释。后端严格语法校验继续决定能否保存，源码回退不代表无效内容可发布。

页面富文本通过宿主提供的 `StellarEditingCanvas` 将同一个 TipTap 编辑 DOM 放到受控同源 iframe；仅复用完整预览的样式和主题属性，不执行返回的正文 HTML/脚本。Console CSS 限定在组件操作控件范围，避免 reset 改变正文。媒体查询沿浏览器视口，正文宽度沿编辑区。本站资料使用净化后的只读装饰，代码块/表格使用编辑 NodeView 复用正式页容器；Chroma token 只在语言和正文匹配时作为有限 class 装饰接入，均不进入 Markdown。

动态 shortcode 仍在 `sandbox="allow-scripts"` 的 opaque iframe 内运行；页面画布内的正文嵌入采用透明表面，其他编辑器按宿主底色与站点表面色的对比度决定是否铺站点表面（同色系透明、异色系保留不透明表面以保证对比度），独立预览保持原表面。高度消息和拖拽/属性控件事件应绑定实际 `ownerDocument` 的窗口或文档，保留消息来源及 token 校验；两个既有公开友链 GET 提供无凭据 CORS，其他后台和写接口不开放。

- 编辑既有内容时，不为缺失字段补默认值；否则只改一个字段也会污染原始 Markdown。
- 无法可靠解析的未知参数、多余位置参数或未闭合块级标签必须回退到原始源码编辑，而非猜测性重建。
- hybrid/块级标签的 variant 由原始标签结构决定；自由文本的尾部位置参数要保留，不应被 token 化截断。
- 新增 catalog 项时同步补 parser/build/renderer 以及覆盖 catalog 的 round-trip 测试；测试应验证重建后的渲染语义与原始内容一致，而不只验证表单能打开。

## 验收与排障原则

- 先跑与改动相称的机械验证，再做真实服务/浏览器检查。构建成功、HTTP 200 或单个单元测试都不足以代替端到端证据。
- CSS、布局和模板问题先在当前 `main.css`、项目模板和上游源三处对照，确认缺口后再修改；不要为已经存在的上游样式另造覆盖层。
- 对性能、移动端和弱网等结论，只有实际相应运行环境的测量才可标为已验证；没有设备/视口证据时如实说明验证边界。
- 所有密钥、token、用户数据和运行时配置都按敏感信息处理：不写入受跟踪示例之外的文件、不回显、不通过调试日志泄漏。
