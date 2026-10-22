# AI 助手设置页适配 Stellar 2 canonical 配置

## 参照与来源

- 问题截图（改动前，唯一参照）：`docs/baselines/ai-settings-adapt/before-ai-settings-notice.webp`，来自用户反馈截图。
- 改动前「AI 助手 → 设置」只显示“旧分段已停止保存”的提示卡片，没有任何可编辑的 AI 配置项。
- 目标：该页直接编辑独立 Blog 扩展 `blog_extensions.ai`，与系统设置中的 Stellar 2 原文使用同一套读取、脱敏与保存协议。

## 验收项

- [x] 展示三组字段：本地 AI 服务（接口地址、API Key、对话模型、向量模型）、AI 摘要（2 个开关）、AI 问答（2 个开关）。证据：`after-desktop.png`、`after-saved.png`；e2e「AI 设置写入独立 Blog 扩展，密钥占位与无关字段保持不变」。
- [x] 公开 AI 开关默认关闭。依据：`ParseStellarBlogRuntime` 注释「所有公开 AI 开关缺省关闭」，缺省字段解码为 `false`（`internal/config/stellar_blog_runtime.go:41–53`）；表单默认值由 `settings-canonical-fields.test.ts` 断言。
- [x] API Key 显示为 `****`；未修改时保存，提交体中的密钥仍为 `****`。服务端恢复原值沿用 `SaveAndPublishStellarV2`，由 `TestStellarV2MaskAndPublishSecrets` 覆盖（本次未改后端）。
- [x] 保存后刷新，字段与开关保持新值；无关字段 `blog_extensions.unrelated.large` 原样保留。证据：e2e 用例。
- [x] 未保存修改时切换路由会弹出确认：取消则留在当前页（`staysOnDirtyLeaveCancel: true`），确认则进入 `/console/settings`（`leaveAcceptedUrl`）。
- [x] 「助手」「知识库」标签切换不丢失未保存草稿（`draftSurvivesTabSwitch: true`）。
- [x] 默认「助手」标签不读取配置：加载期间 Stellar 2 GET 为 0，AI 字段未挂载，离开时无守卫弹窗（`states-final.json` → `defaultTab`）。
- [x] 读取失败显示错误与「重新读取」，重新读取后表单恢复（`state-load-error.png`，`loadFailure`）。
- [x] 迁移未完成显示迁移提示；「打开 Stellar 2 配置」进入 `/console/settings?tab=stellar2`（`state-migration.png`，`migration.landedOn`）。
- [x] 完整配置无法解析显示解析错误，文案无重复句号；同一入口进入 Stellar 2 标签（`state-parse-error.png`，`parseError`）。
- [x] 保存成功显示「AI 设置已保存」（`saveToastShown: true`）。
- [x] 移动端 390px 宽度无横向滚动（`mobileOverflowPx: 0`；`after-mobile-390.png`）。
- 不适用 · 浅色模式：AI 设置页不随系统配色变化。`colorScheme: light` 与 `dark` 的整页截图字节一致（`darkDesktopByteIdenticalToLight: true`）。佐证：Console 全局样式声明了 `html { color-scheme: dark }`（`modules/console/web/src/styles/main.css:97–99`）。未逐一检查其他页面的配色逻辑，因此不据此断言整个 Console 没有浅色主题。

异常状态的响应由 Playwright 拦截模拟，形状与生成类型 `StellarSettingsProblem`（`packages/api-client/src/generated.d.ts`）一致。这些检查验证的是 UI 分支，不是在真实服务上制造的故障。

采集脚本未入库，位于会话 scratchpad。本次结果来自 19:30:00 启动的 `-dev` 服务（端口 8096），服务端引用的入口资源 `index-DNNIF3ms.js` 与 `dist/index.html` 一致。

## 文案依据

| 文案 | 依据 |
| --- | --- |
| 接口地址会追加 `/chat/completions` 与 `/embeddings` | `internal/llm/client.go:72,108` |
| API Key 保留 `****` 表示不修改；清空可清除 | `internal/config/stellar_v2_publish.go`：仅当值等于 `****` 时恢复原值；空串或 `null` 写入后解析为空密钥（`validateBlogTypedValue` 允许 `ai.api_key` 为 `null`） |
| 对话模型用于文章属性生成与知识库问答 | `modules/console/api/ai.go:54`（`aiGenerate`）；`internal/rag/rag.go:521` |
| 向量模型留空时知识库不可用 | `internal/rag/rag.go:421` |
| 更换接口地址或向量模型后需重建索引 | `internal/rag/rag.go:601–614`（`needsRebuild` 比较 `embeddingFingerprint(BaseURL, EmbeddingModel)`） |
| 关闭时前台不展示摘要面板 | `modules/stellar/web/stellar_template_helpers_blog.go:60–63`，仅 canonical `blog.ai.summary.enabled` 为真时输出 |
| 读者 Key 不保存；「记住」只存接口与模型 | `modules/stellar/web/assets/js/ai-summary.js:4` |
| 读者自助摘要需先开启 AI 摘要 | `internal/config/stellar_blog_runtime.go:121`（`Summary.Enabled && AllowUserAPI`）；`modules/console/api/ai.go:208` |
| 问答默认关闭，仅配置向量模型不会公开问答 | `internal/config/reload_ai.go:24–26`；`modules/console/api/rag.go:140` |
| 读者自带模型仅在问答开启后生效 | `modules/console/api/rag.go:140`（先判断问答开关），之后才到 `:158` |
| 保存后，之后发起的请求使用新配置 | 依据代码阅读：`config.LoadAI` 每次从 Store 快照读取（`internal/config/reload_ai.go:33–44`），未做行为探测 |

## 复用与边界

- 字段定义集中在 `modules/console/web/src/modules/settings/settings-canonical-fields.ts` 的 `ai` 标签，渲染复用 `CanonicalSettingsForm`。保存按钮文案、保存说明与校验失败提示经 `saveLabel` / `saveHelp` / `invalidHelp` 传入，系统设置页默认文案不变。
- 未修改后端 Go 代码。相关既有测试为 `internal/config/stellar_v2_publish_test.go` 与 `modules/console/api/stellar_settings_test.go`，均在 `make test` 中运行。
- 已知边界：Stellar 2 原文仍是整份文档的最后写入者；两个浏览器标签同时编辑时，后保存者覆盖先保存者。这与系统设置页既有行为一致，未在本次范围内修改。
- 遗留字段：`internal/settings/model.go:1174–1175` 的 `AISummaryOn` / `AIUserAPIOn` 生产代码从不赋值，`partial/content.html` 中依赖它们的旧摘要块因此不会渲染。前台摘要块实际由 `blogSummary` 按 canonical 开关输出；`stellar_template_helpers_test.go` 的 `TestStellarBlogCapabilitiesUseCanonicalFlagsAndUniqueContainers` 也断言旧字段不会覆盖 canonical 开关。本次未改动，可作后续清理。
- 未核验：`modules/console/web/src/modules/settings/Stellar2Tab.vue:23` 的「保存后前台即时生效」属于 Stellar 2 原文编辑器，不在本次范围内，未做行为探测。

## 验证记录

验证对象：分支 `codex/ai-settings-adapt`（基于 `dev` @ `e2f8c664`）的未提交工作区。

| 检查 | 结果 |
| --- | --- |
| `make test`（最终代码，`make-test-3.log`） | 通过（`MAKE_TEST_EXIT=0`）：console-test（`vue-tsc -b` 与控制台行为测试）298/298；assets-test 220 项中 218 通过、2 跳过（CSS 漂移检查）；部署/发布/模块检查、`go vet`、Staticcheck、`go test ./...`（部分包来自构建缓存，本次未改 Go 代码） |
| `make build`（最终代码，`make-build-3.log`） | 通过（`BUILD3_EXIT=0`）；入口 `assets/index-DNNIF3ms.js` |
| 最终构建的浏览器采集与状态检查 | `CAPTURE_EXIT=0`，`STATES_EXIT=0`，`PORT_8096_FREE`；服务端 19:30:00 启动 |
| Playwright `tests/e2e/console-settings-canonical.spec.ts` | 10/10 通过（`e2e-canonical-2.log`）。该运行早于最后一次 `save-help` 文案修改，该文案未被任何断言引用 |
| 其他 10 个 e2e 规格 | 19:08 与 `dev` 基线对比：94 项中 53 失败、41 通过，失败与通过的标题集合完全相同，与本次改动无关。此后未重跑全量；这些规格中只有 canonical 规格访问 AI 助手页 |
| `pnpm run lint`（`--max-warnings=0`，`lint-final-2.log`） | 未通过，3 个既有错误：`notebooks/notebook-form.ts` 2 个 `no-control-regex`，`plugins/PluginParamForm.vue` 1 个 `vue/no-dupe-keys`。与 `dev` 基线的文件相同 |

## 待决事项

- 未提交。提交到 `codex/ai-settings-adapt` 并执行 `make merge-check` 需先获得确认；`make merge-check` 依赖 `make hooks-install`。
- `pnpm run lint` 的 3 个既有错误会让 `.github/workflows/ci.yml` 的 `lint` 任务失败，`dev` 上同样存在；修复需单独处理 `notebooks` 与 `plugins` 模块。
- 其他 10 个 e2e 规格的 53 个失败为 `dev` 既有问题（日志特征：30 秒超时 23 次、`element(s) not found` 12 次、夹具文章创建失败 2 次），原因未逐一排查。
- 最后一次文案修改之后，全量 e2e 未重跑，只跑了 canonical 规格。
