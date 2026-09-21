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

- `make test`：完整本地门禁。它会为 `go:embed` 写最小 `dist` 占位，运行 Console 类型/行为测试、Stellar 资源测试、部署/发布来源/模块检查，以及 Go vet、Staticcheck、全部 Go 测试。
- `make build`：真实构建 Console（包含 `/console/` 与 `/uc/` 产物）后再编译 Go 二进制。只要涉及 SPA 交付、embed 或发布，不能用 `make test` 代替它。
- `make dev`：根据 Console 输入指纹按需重建前端，随后以 `-dev` 启动服务；`make run` 是 embed 模式。
- `pnpm run e2e`：Playwright 关键路径验收。配置会重建 Console、用独立数据库启动 `-dev` 服务并使用 8096；在需要页面交互证据时运行相关用例。

Go 版本由根 `go.mod` 决定，pnpm 版本由根 `package.json` 的 `packageManager` 决定，Node CI 版本以 `.github/workflows/ci.yml` 为准；不要把本机版本当作项目契约。

### 发布流水线

- `.github/workflows/release.yml`：tag 推送时构建 `blog-linux-amd64`，Release 发布到公开仓库 `SakuraLoveSmile/Sakura-Blog-Release`（跨仓库需 `RELEASE_REPO_PAT` secret）；`update.sh`/`update-docker.sh` 与该仓库的 `BLOG_REPO`/`update_repo` 默认值保持同一目标。
- `.github/workflows/docker-release.yml`：tag 推送时按根 `Dockerfile` 自包含构建并推送 `ghcr.io/sakuralovesmile/sakura-blog`。镜像内置二进制，与「宿主挂载二进制 + `update-docker.sh` 替换」是互斥的部署形态。
- v1.7.3 起 Console 只保留「检查更新」（`GET /system/update/check`），一键更新执行链（`internal/updaterunner`、apply/status 端点、systemd path/timer watcher）已移除；升级一律在宿主手动执行 `update.sh`/`update-docker.sh` 或 `docker compose pull`。

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

`modules/stellar/web/assets/css/main.css` 是从本地、被 gitignore 的 `resource/hexo-theme-stellar` 编译得到的生成物。它不应手改；调整 Stylus 配置、上游同步或主题源后使用 `make css`，再用 `make css-drift-check` 验证。

`resource/` 跨 sibling worktree 共用，可能处于 detached HEAD 或带本地补丁。每次编译前先执行 `git -C resource/hexo-theme-stellar status --short --branch`，确认正在消费的版本与变更；不要把某个历史 tag、commit 或旧的产物大小写成永久真值，更不能擅自 reset、checkout 或清理共享资源。

项目自己的辅助样式应保留在其受跟踪源文件中；只有 `main.css` 受「必须由 `make css` 生成」约束。

### Stylus 编译 shim

Stellar 的 Stylus 源通过 `hexo-config()` 读取主题配置。本项目在 `modules/stellar/web/build/stylus/` 以 `config.json`、`shim.js` 和 `compile.js` 注入该函数，不改上游 `.styl`：字符串、布尔、数字、数组和缺失值必须分别保持对应的 Stylus 类型，尤其不能把布尔值写成字符串 `"false"`。

`compile.js` 会在主入口之后追加上游移出的 swiper、fancybox 与 artalk 样式，保持单一 `main.css`。上游移动样式入口或新增需要保留的段时，先核对当前源，再同步 `EXTRA_STYLES` 与测试；不要直接向产物追加 CSS。

### 模板、资源与图标

- Go `html/template` 的结构必须与当前上游 Stellar EJS 和已存在的项目模板对齐。新增/改动前先对照 class、属性和嵌套，不凭记忆复刻过时的 layout 片段。
- `template.HTML` 已是可信渲染结果时直接输出；不要再次传给只接收 `string` 的 `safe` helper。任何作者可控原始 HTML 都应先走现有渲染/净化链。
- 前台脚本依赖的 `ctx`、模板函数和 `asset_ver` 是服务端到浏览器的契约。新增依赖时在模板注入与消费端一起修改；静态资源引用使用 `asset_ver`，不手写 cache-busting 版本戳。
- 部署使用 `internal/settings/data/icons.yml` 的嵌入图标注册表；开发默认也读取这份受跟踪副本。图标为内联 SVG，未知 key 会安全地渲染占位。新配置使用当前语义化键；保留 `legacyIconAlias`，直到迁移数据确认不再需要旧 `solar:`/`ph:`/`bxs:` 键。
- 产品只面向深色模式。上游 CSS 的亮色选择器可能为兼容保留，但不能据此新增产品切换入口或浅色验收范围。

## 编辑器与 shortcode

shortcode 的解析、表单 schema 和重建路径必须共享一份 catalog：以后端 `internal/post/shortcode_catalog.go` 和各 renderer 实际接受的 named/positional 参数为准，而不是复制第二份前端字段表。

- 编辑既有内容时，不为缺失字段补默认值；否则只改一个字段也会污染原始 Markdown。
- 无法可靠解析的未知参数、多余位置参数或未闭合块级标签必须回退到原始源码编辑，而非猜测性重建。
- hybrid/块级标签的 variant 由原始标签结构决定；自由文本的尾部位置参数要保留，不应被 token 化截断。
- 新增 catalog 项时同步补 parser/build/renderer 以及覆盖 catalog 的 round-trip 测试；测试应验证重建后的渲染语义与原始内容一致，而不只验证表单能打开。

## 验收与排障原则

- 先跑与改动相称的机械验证，再做真实服务/浏览器检查。构建成功、HTTP 200 或单个单元测试都不足以代替端到端证据。
- CSS、布局和模板问题先在当前 `main.css`、项目模板和上游源三处对照，确认缺口后再修改；不要为已经存在的上游样式另造覆盖层。
- 对性能、移动端和弱网等结论，只有实际相应运行环境的测量才可标为已验证；没有设备/视口证据时如实说明验证边界。
- 所有密钥、token、用户数据和运行时配置都按敏感信息处理：不写入受跟踪示例之外的文件、不回显、不通过调试日志泄漏。
