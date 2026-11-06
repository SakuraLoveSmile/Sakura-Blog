# 文章编辑器 AI 写作助手（第一期：选区助手）

任务号：BLG-20261106-01　状态：待体验（改动都在 worktree 里，未提交）

## 目标与范围

用户希望在文章编辑器里有一个随时可以唤出的 AI 助手：对选中的段落润色、改写、扩写、缩写、续写、翻译或按自定义指令处理，辅助作者写作。整体分三期，本任务只做**第一期**：

| 期 | 内容 | 状态 |
| --- | --- | --- |
| 一期 | 选区助手：快捷键唤出的常驻面板，对选区执行动作，流式输出，对比后应用 | 本任务 |
| 二期 | 站内查阅：用现有 RAG 检索自己的文章作为参考资料 | 另开任务 |
| 三期 | 联网搜索与工具循环（agent） | 另开任务 |

一期范围：

- 后端 `POST /api/v1/ai/assist`（SSE 流式），复用设置 → AI 的「对话模型」，不新增配置项、不改库表。
- Console 文章编辑器（富文本与 Markdown 两种模式）的助手面板、快捷键、应用/撤销。
- 仅管理员可用；对话记录只保留在当前页面会话里，不落库；不联网。

一期**不做**（要做另开任务，不在本清单里悄悄追加）：多轮对话、站内检索与联网、持久化历史、页面（Page）编辑器与用户中心投稿编辑器接入、选区浮动气泡、自动保存。

## 已确认决定

- 2026-11-06 用户提出需求，并在看过三期方案后说「开工第一期」。方案中的默认值随之采用：仅管理员、会话内记录不落库、不联网。
- **应用只改编辑器内容，不自动保存。** 应用后文章进入“有未保存修改”状态，保存仍由作者点「保存草稿」或 `Ctrl/⌘+S`。
- **应用必须可撤销。** 富文本走 ProseMirror 事务（`Ctrl/⌘+Z` 一步还原）；Markdown 的 `<textarea>` 用 `execCommand("insertText")` 保留原生撤销栈，不支持时退回直接写值（此时不保证原生撤销，面板提示）。
- **结果先对比再应用。** 面板有「对比 / 结果」切换：对比是逐词的行内差异；翻译与续写没有可比的原文，默认直接看结果。
- 快捷键 `Ctrl/⌘+J` 开关面板；工具栏另有常驻按钮，手机没有键盘也能用。
- 桌面（≥1024px）停靠在右栏，替换大纲/设置内容，关闭后恢复；小于 1024px 用非模态底部面板，手机上位于底部标签栏之上。
- 续写不改原文：在选区（或光标）所在块之后插入新段落。选区跨多个块时，先扩到完整的顶层块再处理，面板会显示实际处理的内容。
- 面板里的限制与后端一致：选区 ≤ 8000 字、自定义指令 ≤ 500 字；前文只带末尾 1500 字、后文只带开头 600 字，大纲最多 60 条，保证长文档不会超过请求体上限。

### 实现中的取舍

下面几点是做的时候定的，和最初方案的字面有出入，清单与测试都按这里的为准：

- **名称统一叫「写作助手」**（清单里的「AI 助手」指同一个入口），避免和已有的「AI 生成」弹窗、AI 摘要混淆。
- **`Ctrl/⌘+J` 是严格的开关**：面板开着时再按一次就关闭，并把焦点还给编辑器；`Esc` 在生成中先停止，其余时候关闭。
- **应用之后焦点回到编辑器**，继续输入或按 `Ctrl/⌘+Z` 都落在正文上。面板上的「撤销」只在应用后文档没再改过时出现；作者自己按 `Ctrl/⌘+Z` 退回这次应用后，面板回到「可以再次应用」。浏览器不支持原生撤销、改成直接写值时，面板会提示这次写入不能用 `Ctrl/⌘+Z`，要退回请点面板里的「撤销」。
- **Markdown 文本框切到「仅预览」再切回时保留原来的选区**，面板随之恢复该选区的概况；光标落下后变回「没有选中文字」。
- **工具栏入口随宽度变化**：工具栏中间的格式工具是横向滚动的，所以工具栏窄于 840px（右栏打开的桌面、800 宽、手机）时入口只留图标，把宽度让给格式工具；收起右栏、工具栏变宽后再带上「写作助手」文字（CSS container query，不支持时保持只有图标）。
- 结果栏的「重试 / 放弃」成组：右栏放不下时整组换到下一行，不让「放弃」单独落单。

## 后端契约（已实现）

- `POST /api/v1/ai/assist`，仅 `sys` 管理员（`PermissionSystemManage`）。请求体 `AIAssistRequest`：`action`（`polish|rewrite|expand|shorten|continue|translate|custom`）、`selection`、`before`、`after`、`title`、`outline`、`instruction`、`target_lang`。
- 校验失败与未配置是流开始前的普通 JSON 错误（400/413/429）；开始后的失败是 `error` 事件（HTTP 200）。响应 `Content-Type: text/event-stream`，事件载荷 `AIAssistEvent{type: delta|done|error, text, message, finish_reason, truncated}`，每 15 秒一个 `: ping` 注释。
- 客户端断开会取消上游请求并释放并发名额；并发上限 4，超出返回 429。
- 类型由 `go generate ./modules/console/api` 生成到 `packages/api-client/src/generated.d.ts`（仅新增 `AIAssistRequest`、`AIAssistEvent`）。

## 运行资源

- 基线：根工作区 `dev` / `2ad23a62`。
- worktree：`/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261106-feat-blg-20261106-01-editor-writing-assistant`
- 分支：`codex/feat/20261106-blg-20261106-01-editor-writing-assistant`；合入目标：根工作区 `dev`（提交/合入需用户授权）。
- 浏览器验收：`scripts/e2e/run-server.sh` 的隔离空站，监听 8101（避开其他 worktree 的 8096），账号 `admin` / `e2e-password`。
- 前端文件：`modules/console/web/src/components/editor/assistant/`（助手核心、行内差异、composable、面板、Markdown 桥接），富文本桥接与桥接契约在 `packages/editor/src/assistant-bridge.ts`、`assistant-rich-bridge.ts`（经 `RichTextEditor.vue` 与 `MarkdownEditor.vue` 暴露），SSE 客户端在 `packages/api-client/src/sse.ts` 与 `api/editor.ts`；接线在 `modules/posts/PostEditor.vue`（入口放在 `EditorToolbar` 已有的 `end` 插槽里，没有改 `EditorToolbar.vue`）。

## 参照图

唯一参照图是改动前的文章编辑页基线（文章标题「写作助手验收夹具」，2 个标题 + 2 段正文）：

- `docs/baselines/editor-writing-assistant/before-editor-1280.png`（1280×900）
- `docs/baselines/editor-writing-assistant/before-editor-390.png`（390×844）

验收时用同一视口截 `after-*.png` 放进同一目录（各图说明见该目录的 README）；除新增的助手入口与面板外，标题、工具栏、正文与右栏的版式不应变化。

两点要如实说明：

- 参照图拍摄时用的是临时正文，`after-*` 用的是浏览器用例里固定的夹具正文（含加粗与链接），文字不同；对比的是版式（标题、工具栏、正文、右栏的位置与尺寸），不是像素逐一相同。
- 入口占掉工具栏固定区的一个图标宽度（桌面 32px，手机 44px），工具栏中间横向滚动的格式工具少露出同样宽度：1280 宽、右栏打开时，原来最后露出的「项目符号列表」按钮要横向滚动才能看到。工具都还在，没有被移走。

## 验收清单

默认视口：桌面 1280×900，手机 390×844，另测 800×900（小于 1024 的底部面板）。浏览器验收用 Playwright 控制 SSE 响应的节奏；真实模型服务不在验收范围（见“验证边界”）。

### A. 入口与布局

每条末尾的「验证」写的是对应的浏览器用例（`tests/e2e/writing-assistant.spec.ts` 里以编号开头的用例）或其他证据；全部在 Chromium 下、用 Playwright 控制 SSE 响应的节奏验证（边界见文末）。

- [x] A1 桌面工具栏出现「写作助手」入口（清单原称「AI 助手」，见「实现中的取舍」），悬停提示含快捷键；点击后右栏切换为助手面板，编辑区无横向滚动、标题与正文位置不跳动。验证：A1（打开前后标题与编辑区的位置和尺寸完全相等）、A6（窄时仅图标、收起右栏后带文字）。
- [x] A2 `Ctrl/⌘+J` 在标题输入框、编辑区、面板内都能开关面板，并阻止浏览器默认行为；打开后焦点进入面板，关闭后焦点回到编辑器；`Esc` 在面板内关闭面板，生成中先停止生成。验证：A2（直接派发 keydown 并断言事件被取消默认行为；真实浏览器里是否压过自带的 `Ctrl+J` 没有手动实测）。
- [x] A3 手机工具栏有助手入口；点击后出现非模态底部面板，位于底部标签栏之上、不盖住顶栏，编辑区仍可滚动；页面无横向滚动，面板内每个可点击控件点击区 ≥44px。验证：A3（面板下沿与标签栏上沿对齐，误差 ≤1px）。
- [x] A4 800×900 同样是底部面板，右栏不被挤出；关闭后右栏恢复为大纲/设置，之前所在标签保留。验证：A4。
- [x] A5 切换富文本/Markdown 编辑器或离开文章时，进行中的请求被取消，面板状态重置，没有残留请求。验证：A5（断言请求被中止的次数：切换编辑器 1 次、离开文章后累计 2 次）。

### B. 选区与动作

- [x] B1 Markdown 模式：选中文字后，面板显示选区摘录与字数；未选中时只有「续写」「自定义指令」可用，其余动作禁用并说明原因。验证：B1（另含仅预览模式下整体不可用、切回后选区恢复）。
- [x] B2 富文本模式：同上；选区含加粗、链接等行内格式时，发给后端的 `selection` 是对应的 Markdown。验证：B2。
- [x] B3 选区超过 8000 字时禁用所有需要选区的动作并提示；自定义指令为空或超过 500 字时不可发送。验证：B3。
- [x] B4 七个动作发出的 `action` 正确；翻译带目标语言；请求带标题、大纲与前后文，长文档下请求体仍小于 256KB。验证：B4（两条）。
- [x] B5 选区跨多个块（富文本）时，面板显示扩展后的完整块，处理与应用针对这些块。验证：B5（提示文案、`selection` 与前文的内容）、D5。

### C. 流式与结果

- [x] C1 生成中文字逐步出现，显示「停止」，其余动作禁用；停止后保留已生成的内容，可应用、重试或放弃。验证：C1。
- [x] C2 完成后「对比 / 结果」可切换；对比用删除线与高亮标出修改处；翻译与续写默认显示结果；对比视图在 390px 下不横向溢出。验证：C2；截图 `after-diff-1280.png`。
- [x] C3 未配置对话模型、上游失败、并发已满（429）、网络中断都在面板内给出中文提示与「重试」，不弹遮挡式对话框；未配置时指向「设置 → AI」。验证：C3；截图 `after-error-1280.png`。
- [x] C4 输出被截断或因长度结束时，面板提示“结果可能不完整”。验证：C4。
- [x] C5（验收中补充）手机底部面板里，生成结束后对比结果与「应用」自动滚进面板的可见区域，不用再滚动就能点到。验证：C5（断言对比文字与「应用」按钮完整落在视口内）；截图 `after-diff-390.png`。

### D. 应用与撤销

- [x] D1 应用后编辑器内容更新，文章进入未保存状态；全程没有调用保存接口。验证：D1-D3（Markdown 与富文本各一条，监听写请求）、D1 富文本「先输入再应用再撤销」。
- [x] D2 `Ctrl/⌘+Z` 一步撤销应用，富文本与 Markdown 两种模式都成立。验证：D1-D3 两条；Markdown 的原生撤销依赖 `execCommand`，只在 Chromium 实测。
- [x] D3 应用后面板显示「已应用」，文档未再被修改时带「撤销」按钮，点击后还原；文档再被修改后按钮消失。验证：D1-D3 两条；作者自己按 `Ctrl/⌘+Z` 退回后面板回到可再次应用，见单元测试 `writing-assistant`。
- [x] D4 生成期间原文被改动（选区文本不再匹配）时，「应用」禁用并提示重新选择；「重试」基于当前选区重新生成。验证：D4。
- [x] D5 续写在所在块之后插入新段落，原有段落不变；光标在围栏代码块内时插到围栏之后。验证：D5（Markdown 与富文本各一条）。
- [x] D6 选区含 `{% … %}` 标签而结果中的标签与原文不一致时给出提示，不阻止应用。验证：D6（标签一致时无提示）。
- [x] D7 「复制」把结果文本写入剪贴板；「放弃」清空结果并回到可选动作。验证：D7（授予剪贴板权限后读回内容）。

### E. 权限与边界

- [x] E1 非管理员访问 `/api/v1/ai/assist` 返回 401/403。验证：Go 测试 `modules/console/api/ai_assist_test.go` 的 `TestAIAssistRequiresAdmin`（未登录 401、普通作者 403）。
- [x] E2 助手只请求 `/api/v1/ai/assist`；现有「AI 生成」弹窗与右栏 AI 摘要行为不变。验证：E2（助手的请求只走受控流，页面上没有 `/api/v1/ai/` 的真实网络请求；「AI 属性生成」弹窗仍能打开和关闭，弹窗开着时 `Ctrl/⌘+J` 让位给弹窗）。右栏 AI 摘要的代码本次没有改动（diff 不涉及），也没有单独的浏览器用例。

### F. 质量

- [x] F1 单元测试覆盖：SSE 解析、请求构建与截断、逐词差异、标签校验、块边界；`make test` 通过。验证：见下方验证记录。
- [x] F2 `make build` 通过；Playwright 专项用例通过；浏览器控制台无新增错误；服务加载的是当前 SPA 产物（入口 hash 与 `dist/index.html` 一致）。验证：见下方验证记录；每条用例结束时断言页面没有 console error。
- [x] F3 键盘可达：面板内全部控件可 Tab 到达、焦点可见；结果区域有 `aria-live="polite"`；`prefers-reduced-motion` 下无强制动画。验证：F3。

## 验证记录

状态：待体验。以下都是在同一份源码上取得的（改动全部留在 worktree，未提交）。

- 后端：`internal/llm/stream.go`、`modules/console/api/ai_assist.go` 及测试。`go test -count=1 -race` 覆盖流式解析、空闲超时、上游错误、SSRF 拒绝、并发上限、gzip 中间件下逐块到达、客户端断开取消上游与释放名额、未登录 401 与普通作者 403。
- 前端单元测试（`modules/console/tests/`）：新增 `sse-parser`、`assist-stream`、`assistant-core`、`inline-diff`、`assistant-markdown-bridge`、`writing-assistant` 六个文件，覆盖 SSE 解析、请求构建与截断、逐词差异、标签校验、Markdown 桥接（块边界、围栏、应用与撤销）和助手状态机（流式、停止、重试、过期、撤销、被作者自己撤回后再次应用、焦点）；`markdown-editor-draft`、`uc-consumer-lifecycle` 两个 SFC 测试的依赖白名单随新 import 更新。
- `make test` 通过：Console 类型检查 + 行为测试 532 条、另一组 229 条，模块边界检查、`go vet`、Staticcheck 与全部 Go 测试均 0 失败。`make build` 通过（Console、User Center 与 Go 二进制）。`eslint`（与 `pnpm run lint` 同范围，`--max-warnings=0`）通过；本次改动的文件里，属于 `pnpm run format:check` 范围（`modules/console/web/src`、`packages`）的都符合 Prettier（该范围内另有 90 个未改动的文件本来就不符合，不在本任务内）；`modules/console/tests` 不在这个范围，沿用现状（现有 67 个测试里有 27 个不符合 Prettier，新增的测试文件没有单独格式化）。
- 浏览器验收：`tests/e2e/writing-assistant.spec.ts` 共 27 条，在隔离空站（`VITE_E2E=1` 构建，监听 8101）上一次跑完 **27/27 通过**；跑用例时服务加载的入口 `index-BbGDEYih.js` / `index-BjZn6m9r.css` 与当时 `dist/index.html`（`VITE_E2E=1` 构建，带测试钩子）一致；之后 `make build` 已把 `dist` 换回不带钩子的正式产物（入口现在是 `index-Bv8NBxKt.js`），所以现在的 `dist` 与那个 hash 对不上是正常的。每条用例结束时断言没有 console error。验收中发现并修掉的问题：生成中按 `Esc` 因焦点随按钮禁用丢失而无效（加了焦点接力）；仅预览切回编辑后概况滞后一帧（等下一个 tick 再刷新）；窄工具栏被入口文字挤占（见「实现中的取舍」）；结果栏「放弃」单独落单；出错时结果栏头部是个空行、在错误提示上方撑出一截空白；手机底部面板里生成结束后「应用」排在可视区域之外（原来只在开始生成时滚动，现在底部面板在结束时再滚一次，桌面右栏不动）。另外 D1 富文本用例偶发丢第一个字（TipTap 的 `focus()` 下一帧才聚焦），用例改为等聚焦后再输入。
- 截图：`docs/baselines/editor-writing-assistant/`（各图说明见该目录 README），都由最终源码上的用例重新生成并逐张看过：桌面 1280 与 800 的面板、手机 390 的面板与对比视图、未配置时的错误提示、收起右栏后的宽工具栏。

## 体验方式（沙箱）

主目录的本地库还是 Stellar v2 迁移前的旧库（启动日志有 `notebook migration pending`，首页返回 409）：不带 `-dev` 的嵌入模式直接拒绝启动（`settings.stellar_v2: missing versioned theme document`）；`-dev` 虽能起来，但 `config.LoadAI` 同样读不到版本化主题文档，AI 设置读不出来，写作助手和已有的 AI 功能都用不了。迁移是需要单独审阅的操作，不在本任务内，所以体验写作助手用沙箱：

1. 在 worktree 根目录执行 `bash .tmp/try-assistant.sh`。脚本在被 git 忽略的 `.tmp/` 里，不受跟踪；`PORT=…` 换端口，`RESET=1` 清空沙箱数据。它依次做三件事：`make build` 得到嵌入正式前端的二进制；用 `scripts/e2e/prepare.go` 在 `.tmp/try-assistant-root` 建一份隔离库，并写最小的 `site.yml` 与 `app.yml`；以 `-root` 指向该目录启动（默认 8098），不读写主目录的数据与配置。
2. 打开 `/console/assistant?tab=settings`，在「对话模型」填写公网可达的 OpenAI 兼容接口地址、API Key 与模型并保存；再打开任意文章编辑器，选中文字按 `Ctrl/⌘+J`。

在这个沙箱的真实服务上（不拦截任何请求）已走通：登录 → 保存 AI 设置（`PUT /api/v1/settings/stellar` 返回 200，密钥回显 `****`，刷新后保留）→ Markdown 编辑器选中文字 → 点「润色」。接口地址故意填内网地址时，面板显示「AI 接口地址不可用：不允许访问内网地址」并给出「重试」，说明助手读到了刚保存的配置，请求到达了真实后端与 LLM 客户端。公网模型的流式返回没有验证。

## 验证边界

- 真实模型服务未验证：`safefetch` 会拒绝回环与内网地址，隔离 e2e 站点无法指向本机的假上游；浏览器侧用 Playwright 在页面里控制 SSE 响应，后端流式行为由 Go 测试（注入测试客户端）覆盖。
- 只在 Chromium 下验证；没有 Safari、Firefox 与真机证据。`execCommand("insertText")` 的撤销行为在其他浏览器里没有实测。
- 手机验收是 390×844 视口模拟，不代表真机触摸与软键盘表现。
- `Ctrl/⌘+J` 是否压过真实浏览器自带的快捷键（例如 Chrome 在 Windows/Linux 上的下载列表）没有手动实测：用例直接派发 keydown 并断言事件被取消默认行为。
- 没有跑过完整的 `pnpm run e2e`，也不应该跑：它会重写其他任务的 `docs/baselines/**` 并替换 `dist`。官方配置固定监听 8096，其他 worktree 同时在用时会撞端口，所以本任务的专项用例用手动启动的 8101 站验证。复现：`VITE_E2E=1 pnpm --filter console run build`，再 `E2E_LISTEN=":8101" bash scripts/e2e/run-server.sh`，然后用一份把 `baseURL` 指向 8101、去掉 `webServer` 的临时 Playwright 配置跑 `tests/e2e/writing-assistant.spec.ts`；跑完后用 `make build` 换回正式产物。`ASSISTANT_SHOTS_DIR=<目录>` 会让用例把截图写到该目录，不设置时不写文件。
- 用户中心的投稿编辑器与「页面」编辑器没有接入（一期范围外）。
