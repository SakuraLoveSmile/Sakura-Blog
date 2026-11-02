# AI 助手：对话模型与向量模型拆分独立接口（组合态验收基线）

任务编号 `blg-20261102-07`。本目录是组合态（Go 后端 + Console 表单）验收的证据基线，运行资源与结论只对下面的代码状态有效。

## 参照与来源

- 参照图：`reference-ai-settings-before.png`，复制自 `docs/baselines/ai-settings-adapt/after-desktop.png`（AI 设置页改造前基线：只有一组「本地 AI 服务」接口地址/API Key/对话模型/向量模型）。用途是对照拆分后仍是同一张设置卡片与同一套表单语言，不是像素级复刻目标。
- 任务计划与验收场景：`docs/plans/ai-split-endpoints.md`（lead 维护，本基线不修改）。
- 代码版本：worktree `personal-blog/20261102-feat-blg-20261102-07-ai-split-endpoints`，分支 `codex/feat/20261102-blg-20261102-07-ai-split-endpoints`，基于 `dev` `d1e34a97`，**未提交**。验收期间工作区 diff 快照 `sha256(git diff) = 8e88c28f0401f46e4f41900a10e6945ccd41078c03423aa20a75d39d2bb9eab3`，验收前后一致。
- 最终验收构建：Console 入口 `/console/assets/index-nox47CgV.js`（`modules/console/web/dist/index.html`，`index.html` sha256 `c08152cb…5f7b`，入口 sha256 `c0a883e0…d7c3`），AI 设置页懒加载 chunk `assets/AssistantView-BeTmvl5T.js`（sha256 `61418681…1e5d`）。
- 说明：lead 先前报告的入口 hash 是 `assets/index-C_ONhXpT.js`。验收前最后一次源码修改发生在 `modules/console/web/src/modules/assistant/AiSettings.vue`（mtime 2026-11-02 20:25:02），晚于那次构建；verifier 于 20:26 与 20:36 两次独立执行 `make build`，均稳定产出 `index-nox47CgV.js`，因此本基线以该构建为准。

## 验收项

- [x] **Console 构建**：`make build` exit 0，生成 `/console/` 与 `/uc/` 真实产物；入口 `assets/index-nox47CgV.js`。证据：`verification.json` 的 `distFiles`，日志 `.tmp/ai-split-endpoints/make-build-final.log`。
- [x] **服务加载当前构建**：浏览器实际执行 `document.scripts` 中的入口为 `/console/assets/index-nox47CgV.js`，出现在 `performance` 资源列表；通过 HTTP 取回的入口字节 sha256 与 `dist/index.html` 引用的磁盘文件一致（`servedEqualsDisk: true`）。没有只凭 HTTP 200 判定。证据：`verification.json` 的 `browserLoadedEntry`。
- [x] **两组各三项**：AI 设置页依次渲染「对话模型」「向量模型」「AI 摘要」「AI 问答」四组；前两组各 3 项（接口地址、API Key、模型）。旧平铺 `ai.base_url` / `ai.api_key` / `ai.model` / `ai.embedding_model` 不再渲染为控件。证据：`verification.json` 的 `form`；`after-settings-desktop-chat.png`、`after-settings-desktop-embedding.png`、`after-settings-mobile-390-chat.png`、`after-settings-mobile-390-embedding.png`。
- [x] **桌面 1280 / 移动 390 无横向滚动**：`documentElement.scrollWidth == clientWidth`（1280/1280、390/390），且遍历页面所有 `overflow-x: auto|scroll` 容器均无横向溢出（`horizontalScrollers: []`）。证据：`verification.json` 的 `overflow`；`after-settings-desktop.png`、`after-settings-mobile-390.png`。
- [x] **旧配置回退说明存在**：表单上方提示「旧版单接口配置仍受支持……本表单不会删除或改写旧字段……」；保存说明「对话模型与向量模型独立生效……」。证据：`after-settings-desktop.png`；文案见 `AiSettings.vue:62–65`、`AiSettings.vue:71–72`。
- [x] **保存按钮与表单可用**：`保存 AI 设置` 按钮可见且可点击，两组输入可编辑，保存后出现「AI 设置已保存」toast。证据：`after-saved.png`。
- [x] **两组独立接口真实保存**：表单填入 `https://chat.fixture.test/v1` + `fixture-chat-model` + `fixture-chat-key-20261102` 与 `https://embedding.fixture.test/v1` + `fixture-embedding-model` + `fixture-embedding-key-20261102`，保存后 `GET /api/v1/settings/stellar` 返回的 `blog_extensions.ai.chat` / `ai.embedding` 嵌套字段与输入一致（密钥为 `****`）。证据：`verification.json` 的 `saved`；`after-saved.png`。
- [x] **密钥脱敏与逐路径恢复**：GET 中旧 `ai.api_key`、`ai.chat.api_key`、`ai.embedding.api_key` 均为 `****`；隔离数据库原文核对确认两组密钥按各自嵌套路径恢复为不同原值，未互相覆盖、未写入 `****`。证据：`verification.json`（`storedMasked`）、`db-evidence.json`。
- [x] **旧平铺字段与无关字段未被删除**：保存并重读后 `ai.base_url` / `ai.model` / `ai.embedding_model` 仍是原值，`ai.summary`、`ai.ask` 开关仍未改动，`blog_extensions.unrelated` 的 `null` / `false` 保留；大整数 `9007199254740993` 在 GET 原文中逐位保留（`verification.json` 记录；JS `JSON.parse` 展示会舍入，故以原文正则断言）。证据：`verification.json`、`db-evidence.json`。
- [x] **刷新后值保持**：重新加载 AI 设置页，两组地址/模型仍为新值，两个 API Key 仍为 `****`。证据：`after-refresh-desktop.png`。
- [x] **e2e 规格（组合态）**：`tests/e2e/console-settings-canonical.spec.ts` 11/11 通过，含「AI 设置写入独立 Blog 扩展，对话与向量接口分别保存且密钥占位保持」（`spec.ts:153`，850 ms）。证据：`.tmp/ai-split-endpoints/e2e-canonical-final.log`。
- [x] **RAG 分接口链路（由 Go 测试覆盖，本轮未做真实 HTTP 冒烟）**：`TestRebuildAndAskUseSeparateAIEndpoints`（`internal/rag/rag_test.go:325`）用假 transport 断言重建/增量只请求 embedding 地址与模型，问答分别请求 embedding 与 chat 且各自使用自己的地址/密钥/模型；`TestAskRequiresBothEmbeddingAndChat`、`TestEmbeddingFingerprintTracksEmbeddingEndpointAndModel`、`TestNeedsRebuildTracksEmbeddingEndpointChangesOnly` 覆盖缺一不可与 fingerprint。均在 `make test` 中运行。未使用真实供应商或本地假 OpenAI HTTP 服务做端到端冒烟（见「未验证项」）。
- [x] **旧配置不回归的机械验证**：`TestStellarBlogAIEndpointFieldFallback`、`TestStellarBlogAINestedEndpointValidation`、`TestStellarV2MaskAndPublishSecrets`、`TestAIGenerateRequiresChatEndpoint` 通过（`make test` 范围）。
- [x] **后台进程清理**：本任务启动的服务/e2e 进程均已关闭；收尾复查 8090–8100 无本任务监听，8097、8099 已释放，8096 也已释放（验收过程中曾被另一并行 worktree 的 e2e 服务占用，不属于本任务进程）。

## 证据文件

| 文件 | 内容 |
| --- | --- |
| `verification.json` | 一次性 Playwright 脚本采集：构建指纹、浏览器实际加载入口与字节比对、表单结构、保存前后 GET 断言、1280/390 溢出测量、截图清单 |
| `db-evidence.json` | 隔离服务 SQLite `settings.blog_extensions` 原文核对：两组密钥各自恢复为不同原值、旧平铺字段保留、大整数原文保留 |
| `after-settings-desktop.png` | 1280×900 AI 设置页（顶部：回退说明 + 对话模型组） |
| `after-settings-desktop-chat.png` / `after-settings-desktop-embedding.png` | 桌面宽度下两组各 3 项的完整元素截图（保存前，含 `****`） |
| `after-saved.png` | 保存后：「AI 设置已保存」toast + 保存条 |
| `after-refresh-desktop.png` | 刷新后两组值保持 |
| `after-settings-mobile-390.png` | 390×844 AI 设置页 |
| `after-settings-mobile-390-chat.png` / `after-settings-mobile-390-embedding.png` | 移动宽度下两组各 3 项的完整元素截图 |
| `reference-ai-settings-before.png` | 改动前参照图（来源见上） |

一次性验收脚本与日志（`.tmp/` 已被 gitignore，不入库）：

- 脚本：`.tmp/ai-split-endpoints/acceptance.spec.ts`、`playwright.config.ts`、`e2e-canonical.config.ts`
- 浏览器验收日志：`.tmp/ai-split-endpoints/acceptance-final.log`（前一次：`acceptance.log`）
- e2e 日志：`.tmp/ai-split-endpoints/e2e-canonical-final.log`（首次 8096 运行：`e2e-canonical-first.log`）
- 构建日志：`.tmp/ai-split-endpoints/make-build-final.log`（前一次：`make-build.log`）
- 服务日志：`.tmp/ai-split-endpoints/server-8097-final.log`（前一次：`server-8097.log`；首个 8096 启动即 `server.log`）
- e2e 服务隔离数据目录：`.tmp/e2e-server.*`（由 `scripts/e2e/run-server.sh` 每次新建，含独立 `data/blog.db`）

## 文案依据

| 文案 / 行为 | 依据 |
| --- | --- |
| 「对话模型」「向量模型」两组、各接口地址/API Key/模型三项 | `modules/console/web/src/modules/settings/settings-canonical-fields.ts:424–461` |
| 「旧版单接口配置仍受支持……本表单不会删除或改写旧字段」 | `modules/console/web/src/modules/assistant/AiSettings.vue:62–65` |
| 保存说明「对话模型与向量模型独立生效……更换向量接口或向量模型后……重建知识库」 | `modules/console/web/src/modules/assistant/AiSettings.vue:71–72`（保存按钮与 `save-help`） |
| API Key 输入框说明「保留 **** 表示不修改已保存的新密钥；留空时沿用旧版 API Key」 | `settings-canonical-fields.ts:433–435`、`:453–455` |
| 新嵌套字段非空白优先、否则逐字段回退旧平铺字段 | `internal/config/stellar_blog_runtime.go:132–135`（chat ← base_url/api_key/model；embedding ← base_url/api_key/embedding_model）、`:140–157`（`applyAIEndpointFallback`） |
| 旧段 `settings.ai` 只用于旧分段读写；本地 AI 只读 canonical `blog_extensions.ai` | `internal/config/reload_ai.go:33–45`、`:60–73` |
| 旧 `ai.api_key` 与新的 `ai.chat.api_key` / `ai.embedding.api_key` 均脱敏，并按路径恢复 | `internal/config/stellar_v2_publish.go:13–22`、`:61–76` |
| 三个密钥路径允许 `null` 清空 | `internal/config/stellar_blog_runtime.go:163–171` |
| 摘要/属性生成只用对话接口 | `modules/console/api/ai.go:80–86`（`cfg.Chat.Configured()` → `callChatCompletion(cfg.Chat, …)`） |
| 知识库问答：向量化用 embedding，回答用 chat | `internal/rag/rag.go:507–522` |
| 建库/增量索引只用 embedding；fingerprint 取 embedding 地址与模型，变更后才需重建 | `internal/rag/rag.go:631–676`、`:602–615`、`:933`（`embeddingFingerprint`） |
| 两组各自独立生效，切换向量接口/模型需重建知识库 | `internal/rag/rag.go:615`；表单说明见 `settings-canonical-fields.ts:444–446` |
| AI 摘要/问答开关默认关闭语义不变 | `settings-canonical-fields.ts:463–482`；`internal/config/reload_ai.go:47–58` |

## 复用与边界

- 复用：AI 设置页仍是 `CanonicalSettingsForm` + `useStellarSettings` 的同一套读写/草稿/保存协议（`AiSettings.vue`），没有新增组件或私有接口；两组字段来自 `canonicalSettingsGroups.ai` 的声明式配置。
- 后端未改 Console 响应 struct（`StellarSettingsDocument` 仍是 `map[string]any` 原文），因此未重新生成 `packages/api-client/src/generated.d.ts`；`tygo.yaml` 未变。
- 不自动迁移旧字段：旧平铺字段保留在原文中作为回退，用户可自行在 Stellar 2 完整配置中清理。旧字段不渲染为表单控件，避免误覆盖。
- 数据隔离：浏览器验收与 e2e 均使用 `scripts/e2e/run-server.sh` 创建的临时根目录（独立 SQLite、上传、配置、内容），不读写主工作区 `data/` 与 `config/app.yml`；没有任何请求发往外部 AI 服务（`*.fixture.test` 只写入表单与数据库，不会被调用）。
- 未改动 `internal/`、`modules/`、`packages/`、`tests/`、`docs/plans/`，未提交、未推送、未部署；本目录与 `.tmp/` 是本次唯一写入范围。
- 端口说明：e2e 首次运行按仓库配置使用 8096；验收时 8096 被另一并行 worktree 的 e2e webServer 占用，最终 e2e 复跑与浏览器验收分别在 8099、8097 的同类隔离服务上完成，隔离方式与 8096 相同。

## 验证记录

| 检查 | 命令 / 范围 | 结果 | 时间（+08:00） |
| --- | --- | --- | --- |
| 完整本地门禁 | `make test`（lead 运行，verifier 未重跑，避免占位 `dist` 覆盖真实构建产物） | exit 0；Console 行为测试 238 pass / 2 skipped / 0 fail；go vet、staticcheck、全部 go test 通过 | 验收开始前 |
| Console + Go 构建 | `make build` | exit 0；入口 `assets/index-nox47CgV.js`；两次独立构建 hash 一致 | 20:26、20:36 |
| e2e（组合态） | `pnpm exec playwright test tests/e2e/console-settings-canonical.spec.ts`（8096） | exit 0；11 passed（40.6s），AI 用例 803 ms | 20:25 |
| e2e（最终源码复跑） | 同规格，隔离端口 8099（`.tmp/ai-split-endpoints/e2e-canonical.config.ts`） | exit 0；11 passed（44.1s），AI 用例 850 ms | 20:35–20:36 |
| 浏览器验收 | `.tmp/ai-split-endpoints/acceptance.spec.ts`（8097 隔离服务） | exit 0；2 passed（2.4s），`servedEqualsDisk: true`，保存断言全通过 | 20:38 |
| 隔离库原文核对 | 只读查询 `.tmp/e2e-server.geTi9T/data/blog.db` | 两组密钥各自为不同的输入原值；旧平铺字段与无关字段保留；`9007199254740993` 原文保留 | 20:39 |
| 进程与端口清理 | `lsof -nP -iTCP:8090-8100 -sTCP:LISTEN` | 无本任务监听；8097/8099 已释放；8096 已释放（验收中曾由另一 worktree 的 e2e 进程占用，非本任务） | 20:42 |

补充：验收使用的服务启动于 20:37:54（最终构建之后），`-dev` 从磁盘提供静态资源，入口 HTML 在启动时读入内存；因此不存在「构建后未重启导致旧页面」的情况。

## 未验证项与风险

- **真实供应商兼容性未验证**：没有使用真实 OpenAI 兼容服务或本地假 OpenAI HTTP 服务跑通「建库只打向量接口、问答只打对话接口」的线上链路；该链路由 Go 测试 `TestRebuildAndAskUseSeparateAIEndpoints` 等覆盖（假 transport 层），不是真实 HTTP 冒烟。公开 AI 摘要/问答的端到端行为需要外部网络与密钥，不在本次范围。
- **e2e 首次运行的时序**：8096 首次 e2e 的构建窗口与 `AiSettings.vue` 最后一次修改（20:25:02）重叠；为消除该不确定性，20:35–20:36 在最终源码上复跑同一规格，11/11 通过。首次日志仍保留在 `e2e-canonical-first.log` 供对比。
- **端口偏离仓库默认**：浏览器验收与最终 e2e 复跑没有使用 8096，原因是该端口被另一并行 worktree 的 e2e 服务占用；隔离数据目录与 `-dev` 行为相同，结论不受影响。8096 上的监听进程属于另一任务，本任务未终止它；收尾复查时该进程已自行结束，8096 已释放。
- **构建 hash 与 lead 报告不同**：以本目录记录的 `index-nox47CgV.js` 为准（两次独立 `make build` 一致）；建议 lead 合并前在最终源码上再跑一次 `make test`/`make build` 以刷新其记录。
- 未验证 Console 其他页面的回归；仅运行了 `console-settings-canonical.spec.ts` 一个规格（按任务范围）。
- 未提交、未执行 `make merge-check`、未推送、未部署。不做 `make test` 复跑，因此 `make test` 的通过记录来自 lead 的运行，不是本目录产生的独立证据。
- 状态：待体验（用户确认后方可视为已验收）。
