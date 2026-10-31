# BLG-20261031-03：编辑器标签插件实时预览的整体性优化

- 状态：**待体验**（2026-11-01，Asia/Taipei；最终验收后复审发现的 MarkdownEditor 跨文档身份回归（M1）与写入守卫反斜杠边界（L1）已完成「收尾修复」并通过开发自测；复验由测试工程师执行）。
- 用户需求：『整体性优化一下标签组件的实时显示吧』；经确认范围是**编辑器域内 Stellar 标签插件（shortcode）的实时预览**，包含正文 NodeView、属性气泡、侧栏插入面板、标签 Studio、顶栏插入菜单悬停预览与 Markdown 分屏预览。
- 基线：`dev` @ `023d7cac`（worktree 创建时）；上游 Stellar 2.0 与既有 `docs/plans/tag-plugin-parity.md` 的语义、raw 保真、fixture 隔离、SSRF/沙箱安全要求继续有效。
- 工作树：`/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261031-feat-blg-20261031-03-tag-live-preview`
- 分支：`codex/feat/20261031-blg-20261031-03-tag-live-preview`；合入目标 `dev`（合入/提交需用户授权）。
- 唯一写入者：本任务的全栈工程师；主 agent 只做集成核验，测试工程师与代码审查员在实现完成后进入。

## 目标

让编辑器里的标签组件预览做到**更新及时、不闪、不塌、不老、不误写**：

1. 输入/上下文变化合并请求，不因标题或配置逐字输入让所有标签重复整站渲染。
2. 刷新期间保留上一次成功预览，不白屏、不卸载 iframe、不塌陷高度。
3. 迟到响应不覆盖新结果；相同输入命中缓存，不重复请求。
4. 失败可辨识、可重试，并保留上一次成功预览。
5. 四处预览（正文 NodeView、侧栏插入面板、标签 Studio、顶栏悬停气泡）使用一致的整文档预览管线和交互语义；Studio/插入面板使用 component-library fixture，正文预览沿用真实上下文。
6. 顶栏悬停预览在 390px 视口不横向溢出；动态标签不再只是 CSS 空壳。

## 非目标

- 不重做 Console 或 Stellar 视觉，不新造颜色/圆角/阴影；必须复用 `resource/apple-design` 语义 token 与既有组件。
- 不改变 shortcode 语法、官方标签目录、公开站渲染语义与 54 项/288 参数 oracle。
- 不修改 `resource/hexo-theme-stellar` 的生成 CSS；若必须改 CSS，仅走 `make css`。
- **禁止**为让动态预览工作而放宽 iframe sandbox 或加 `allow-same-origin`（会让预览脚本获得 Console 同源权限）。
- 不让预览写真实评分/投票；正文预览不改为 component-library fixture（该策略变更需另立任务）。
- 不新增后端整站预览缓存、专用预览域名或数据库结构；本任务只允许为公开、只读、无凭据的 preview fixture 端点补 CORS 头（若浏览器证据表明 sandbox 动态数据被 CORS 阻断）。
- 不顺带重构编辑器其他区域、保存流程或公开站 runtime。

## 已核实问题（调研证据）

- `StellarTagView.vue:68` 直接 watch `[raw, context]`，无防抖/缓存；context 含 title/slug/config（`PostEditor.vue:61-71`、`PageEditor.vue:66-76`），每次标题输入都让每个标签各发一次 `POST /api/v1/preview/markdown`，后端每次重建整站快照与完整布局。
- `usePluginPreview.ts:38-40` 在每次 `scheduleRender` 开头 `clearPreview()`；`ShortcodeInsertPanel.vue` 输入即调度；`ShortcodeStudioView.vue:413-431` 用 `v-if rendering` 卸载 iframe，导致闪烁、高度塌陷、页签状态丢失。
- `EditorToolbar.vue:134-194` 是独立 CSS-only 预览：只取 `data.html`，不加载运行时脚本，`friends/timeline/tabs` 等是空壳；`:219/:242` 菜单 208px + 气泡 320px，390px 视口必然溢出；缓存只按插件名，无请求序号。
- 正式预览 iframe 均为 `sandbox="allow-scripts"`（`StellarTagView.vue:244`、`MarkdownEditor.vue:491`、`ShortcodeInsertPanel.vue:117`、`ShortcodeStudioView.vue:430`），opaque origin 下 fixture 端点无 CORS 头，动态数据可能被拦截；rating/vote 还依赖 localStorage。
- `MarkdownEditor.vue:87-90` 失败时把 `previewSrcdoc` 清空，分屏白屏无提示。
- 高度：NodeView 初值 64 且刷新过程保留旧高度但不保证稳定；Studio iframe 被卸载会塌陷；preview-only 模式重渲染后 iframe 回顶（`MarkdownEditor.vue:127-129`）。

## 实施步骤

### T1 共享预览引擎（`usePluginPreview.ts` 及同目录 helper）

- `scheduleRender` 不再清空当前 document/token/height；仅显式 `clearPreview()`（空输入、切换插件）清空。
- 增加合并防抖（建议 300–500ms，可配置）、请求序列/取消、按 `profile + markdown` 的有界缓存（建议 20 条）、`status`（idle/loading/ready/updating/error）与 `error`。
- 命中缓存立即应用、不发请求；迟到响应丢弃；失败保留上一次成功文档并暴露错误与重试。
- 既有导出保持兼容，新增能力不破坏 `ShortcodeInsertPanel`/`ShortcodeStudioView` 的现有调用。
- 新增/更新单测：连续输入只发一次、旧响应不覆盖、缓存命中、失败保留旧文档、卸载后无新请求（对调用次数做精确断言）。

### T2 侧栏插入面板与标签 Studio

- 输入/表单修改时保留旧预览，仅在画布上叠加轻量“更新中”状态（复用既有语义 token）；新文档就绪后替换。
- Studio 与插入面板的 iframe 在渲染期间不得卸载；高度不塌陷；选中插件时才重置为推荐高度。
- 清空/无效输入给出明确空态；失败显示错误并保留旧预览。
- 如确需给插入面板补 Fixture 标识，先与主 agent 确认，不擅自加视觉元素。
- 新增/更新单测与 E2E：输入过程中 iframe 元素身份不变、高度不低于阈值、最终预览更新。

### T3 正文标签 NodeView

- 区分 raw 与 context 的调度：raw 变更走短防抖（约 150ms），context 变更走较长合并（约 500–600ms）；使用 context 指纹参与缓存键。
- 同一 raw + context 指纹命中缓存不重发；刷新期间保持旧 iframe/文档，不白屏；成功后替换。
- 高度保留并平滑过渡，不在刷新时回退到 64；迟到响应丢弃；失败保留上次成功预览并显示可重试错误。
- 面板打开/关闭不得触发额外渲染；卸载后不得再发请求。
- 允许增加模块级并发上限（建议 2）避免首屏 N 个整站渲染同时打满，但不得让可见标签长期饥饿。
- 新增/更新单测：标题连续输入只发一次、缓存命中、旧响应不覆盖、iframe 不卸载、高度不回退。

### T4 顶栏插入菜单悬停预览

- 改为与插入面板一致的整文档预览（`component-library` fixture、`mode:"content"`），复用共享预览管线；请求静默（不弹全局错误 toast），带序列号与可失效缓存。
- 通过既有 `component-preview-resize` 协议做自适应高度并设最大高度；390px 视口菜单与气泡不得横向溢出（在既有布局内收窄/下移，不重做菜单）。
- 键盘聚焦与鼠标悬停都应触发预览；加载/错误有可辨识状态。
- 新增/更新单测或 E2E：动态标签渲染出内容、请求静默、快速切换不串台、390px 无横向溢出。

### T5 动态 fixture 数据与 Markdown 分屏

- 浏览器复核 Studio/插入面板/顶栏气泡中 `friends/sites/tabs` 是否因 sandbox + 缺 CORS 而失败；若复现，仅对 `/api/v1/preview/*` 这组公开、只读、无凭据 fixture 端点补 `Access-Control-Allow-Origin: *`（含必要的预检），并在任务记录写明安全依据。禁止放宽 iframe sandbox。
- 若 rating/vote 在 opaque origin 读取 localStorage 抛错，优先在预览 profile 下使用内存回退或禁用交互；如触及主题共享脚本，必须保持公开站行为不变，否则降级为记录问题、另立任务。
- `MarkdownEditor`：失败时保留上次预览并显示错误，不清空 iframe；preview-only 模式重渲染后尽量恢复滚动比例（若风险低）。
- 新增单测：错误保留旧文档；E2E/浏览器证据覆盖动态标签与 390px。

## 文件边界

允许修改（唯一写入者）：

- `modules/console/web/src/modules/plugins/preview/usePluginPreview.ts`
- `modules/console/web/src/modules/plugins/preview/preview-document.ts`、`render-gate.ts`（仅必要 helper）
- `modules/console/web/src/components/editor/StellarTagView.vue`
- `modules/console/web/src/components/editor/StellarTagPropertyPopover.vue`（仅加载/错误文案与状态）
- `modules/console/web/src/modules/plugins/ShortcodeInsertPanel.vue`
- `modules/console/web/src/modules/plugins/ShortcodeStudioView.vue`
- `modules/console/web/src/modules/posts/EditorToolbar.vue`（已获用户范围确认）
- `modules/console/web/src/components/MarkdownEditor.vue`（仅本任务相关的最小改动）
- `modules/console/api/router.go` / `preview_fixture.go`（仅 T5 的 CORS，若证据需要）
- `modules/console/tests/*`、`modules/console/api/preview_cors_test.go`、`tests/e2e/*`（本任务新增/更新测试）
  - 显式包含测试工程师新增的 `tests/e2e/tag-live-preview.spec.ts` 与 `modules/console/api/preview_cors_test.go`；修复轮次不修改前者断言。
- 本文件与 `docs/baselines/tag-live-preview/`

禁止：根工作树未提交的 `AGENTS.md`、`README.md`、`docs/**` WIP；`.pi/`；其他任务 worktree；生成物 `modules/stellar/web/assets/css/main.css`；公开站主题模板/CSS（除 T5 明确允许的最小脚本兼容修复且需说明）。

## 验收（可逐项勾选）

- [x] 连续修改标题/配置 10 次：同一标签最多 1–2 次预览请求（防抖合并），无逐字请求风暴。
- [x] 相同 raw+context 再次出现：命中缓存，0 新请求；主题/配置变化会失效重取。
- [x] 刷新期间：旧预览保持可见，iframe 不卸载、不白屏、高度不回退；新结果就绪后替换。
- [x] 迟到响应不覆盖新结果；失败保留上次成功预览并显示错误/重试。
- [x] 插入面板/Studio 输入过程中 iframe 不消失、高度不塌陷；最终预览正确。
- [x] 顶栏悬停预览加载完整运行时，动态标签出现内容；快速切换不串台；390px 无横向溢出。
- [x] Markdown 分屏失败不清空白屏，保留上次预览并提示错误。
- [x] 若动态 fixture 因 CORS 失败：修复后 Studio/插入面板/顶栏气泡可见 friends/sites 内容，且未放宽 sandbox。
- [x] `make test` 通过；相关 Console 单测、`pnpm exec vue-tsc -b`、`make build` 通过。
- [x] Playwright：`tests/e2e/shortcode-visual-editor.spec.ts`、`tests/e2e/tag-plugin-parity.spec.ts` 通过；新增本任务 E2E 覆盖不卸载/不塌陷/请求合并/390px。
- [x] 真实浏览器截图存 `docs/baselines/tag-live-preview/`，并在任务记录写明服务端口、构建产物、视口与未验证项。

## 验证命令（最小）

```bash
cd "/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261031-feat-blg-20261031-03-tag-live-preview"
make test
cd modules/console/web && pnpm exec vue-tsc -b
cd ../../.. && make build
pnpm e2e tests/e2e/shortcode-visual-editor.spec.ts tests/e2e/tag-plugin-parity.spec.ts
```

E2E 使用 `scripts/e2e/run-server.sh` 的隔离根目录与 8096 端口；不改用户真实数据。若改 CSS 才追加 `make css && make css-drift-check`。

## 状态与证据

状态：**待体验**（2026-11-01，Asia/Taipei）。T1–T5 已实现；测试工程师首轮验收通过，随后代码审查指出 H1/M1–M3 与 L1–L5 问题，本轮已修复并完成开发自测；测试工程师复验 H1/M1–M3/L1–L5 全部通过，唯一失败的 tabs 可见性已由 V1（预览表面）修复，最终独立验收通过（详见文末「最终验收」）；最终验收后复审发现跨文档身份预览回归与写入守卫边界，已于「收尾修复」修复并通过开发自测（详见文末「收尾修复」）。

### 实际改动

- T1 共享预览引擎（`preview/render-gate.ts`、`preview/usePluginPreview.ts`）
  - 新增 `PreviewScheduler`：合并防抖（默认 400ms，可覆盖）、请求序列号、按 `profile|markdown|context 指纹` 的 LRU 20 条共享缓存、`status`（idle/loading/ready/updating/error）；命中缓存同步应用且零请求；迟到响应丢弃；失败保留上次成功文档并暴露 `retry()`（跳过缓存）。
  - `scheduleRender` 不再清空文档；仅 `clearPreview()`（空输入/切换插件/关闭气泡）清空。保留原有 `previewHtml/rendering/previewError/frameHeight/srcdoc/scheduleRender/resetHeight/clearPreview/onMessage` 导出；新增 `status/hasDocument/retry/invalidateCache/previewToken/document`。
  - 新增 `PreviewQueue`（模块级 `sharedPreviewQueue`，上限 2），排队任务在执行前重新校验序列号，被取消的请求直接跳过，不长期占用执行位。
  - 新增 `previewFingerprint/previewCacheKey`：上下文键排序后哈希，标题/slug/配置变化才失效。
  - 旧 `RenderGate` 保留，兼容 `MarkdownEditor` 等未迁移路径与既有单测。
- T2 侧栏插入面板 / Studio
  - 输入或表单变更时保留旧 iframe 与文档，只在画布右上叠加“更新中…”（复用 `text-quaternary/card` token）；iframe 不再因 `rendering` 卸载，高度不回退。
  - 只有选中插件才 `clearPreview + resetHeight`；raw 为空才显示空态；解析/校验中不打断旧预览。
  - 失败时旧预览保留，底部显示可重试错误条（`state-error`）+“重试”；无文档时显示错误态。
  - 插入面板与 Studio 的调度改为“raw 变空才清空，canApply 后调度”，避免解析期间逐字清空。
- T3 正文 NodeView（`components/editor/StellarTagView.vue`）
  - raw 变更 150ms 防抖，context 变更 550ms 合并；`profile: null` 使用真实上下文，context 指纹参与缓存键。
  - 刷新期间 iframe/文档/高度保持；高度消息严格校验 `event.source`、当前 token 与数值类型（字符串/NaN/Infinity/负数忽略），保留 32px 最小点击区。
  - 失败保留上次成功预览：无文档时显示错误卡 + 重试；有文档时底部错误条 + 重试。面板打开/关闭不触发渲染，卸载由调度器 `dispose()` 取消待执行请求并丢弃迟到响应。
  - 使用模块级并发上限 2。
- T4 顶栏插入菜单悬停（`modules/posts/EditorToolbar.vue`）
  - 删除独立 CSS-only 组装，改用共享引擎的整文档预览（`component-library`、`mode:content`）；气泡加载真实运行时，friends/sites/tabs 不再空壳。
  - 300ms 悬停防抖 + 引擎序列号/缓存；键盘 `focus` 同样触发；加载/更新/错误状态与重试可辨识；请求经 `mode:content` 静默（不弹全局 toast）。
  - 通过 `component-preview-resize` 自适应高度，`minHeight 180 / maxHeight 360`；移除原来的 `allow-same-origin`，sandbox 收紧为 `allow-scripts`。
  - 390px：菜单与气泡改为上下排列，气泡 `max-width: 20rem` 且受视口约束；不再整体隐藏气泡。
- T5 动态 Fixture 与 Markdown 分屏
  - 浏览器复核确认 sandboxed srcdoc（opaque origin）读取 `/api/v1/preview/*` 会被 CORS 拦截：仅为 `rating`（GET/POST）、`vote/info`、`vote/update`、`friends`、`sites`、`md` 这组公开、只读、无凭据端点补 `Access-Control-Allow-Origin: *`、允许方法与 `Content-Type` 预检，并注册 OPTIONS；`/preview/markdown` 等需鉴权端点不返回这些头。未放宽 iframe sandbox，未加 `allow-credentials`。
  - opaque origin 下 `localStorage` 直接抛 SecurityError（rating/vote 初始化会读到）：在预览文档注入**预览专用内存 Storage 回退**（`injectPreviewStorageFallback`，幂等），但**仅 component-library profile**（Studio/插入面板/顶栏）；正文 NodeView 与 Markdown 真实上下文不注入，保持 rating/vote 不可交互。未改公开站主题脚本。
  - `MarkdownEditor`：失败不再清空 `previewSrcdoc`，保留上次文档并显示可重试错误条；仅预览模式记录子文档回传的滚动比例，重渲染加载后按 token 恢复。
- 单测更新：`plugin-preview-engine.test.ts`（新增）、`plugin-preview.test.ts`、`stellar-tag-preview.test.ts`、`stellar-tag.test.ts`；`markdown-editor-draft.test.ts`、`uc-consumer-lifecycle.test.ts` 仅补充新 SFC 依赖映射。后端新增 `modules/console/api/preview_cors_test.go`。

### 实际执行的检查

| 命令 | 结果 |
| --- | --- |
| `make test`（worktree 根） | 通过：console 397/397、stellar 资源测试、module-check、`go vet`、staticcheck、`go test ./...` 全绿 |
| `pnpm exec vue-tsc -b`（`modules/console/web`） | 通过，无输出 |
| `pnpm test`（`modules/console/web`） | 397 passed / 0 failed |
| `go test ./modules/console/api -run 'TestPreviewFixtureCORSHeaders...'` | 通过 |
| `make build` | 通过，产出 `./blog`、`dist`、`dist-uc` |
| `pnpm exec playwright test tests/e2e/tag-live-preview.spec.ts tests/e2e/shortcode-visual-editor.spec.ts tests/e2e/tag-plugin-parity.spec.ts --config .tmp/chrome/playwright.config.ts` | 修复轮次复跑 **33 passed / 0 failed**（系统 Chrome `channel: chrome` + `run-server.sh` 隔离根目录/8096；未修改仓库 E2E 配置与新增用例） |

浏览器自测（真实服务 8096，隔离根目录；截图在 `docs/baselines/tag-live-preview/`）：

- `studio-friends.png`：Studio friends（4 张 Fixture 卡片，0 条降级提示），iframe 元素身份在改 source 前后保持，高度 240→240。
- `insert-panel-friends.png`：共享插入面板 friends 动态卡片 4 张，iframe 身份保持，高度 240→240。
- `toolbar-bubble-390.png`：390×844 视口，气泡 320px 宽、右边界 359px，`scrollWidth - innerWidth = 0`，气泡内 4 张动态卡片；`focus` 到 note 项也会出预览。
- `nodeview-title-debounce.png`：连续输入标题 10 个字符，预览请求精确计数 **1 次**；打开/关闭属性面板新增预览请求 **0 次**；无 pageerror。
- `markdown-error-retained.png`：强制 500 后错误条出现、`srcdoc` 与失败前逐字一致（旧内容仍在 iframe 内），点击“重试”后恢复新文档；无 pageerror。

### 未覆盖与已知风险

- 本次自测只跑既有两条 E2E + 定向浏览器脚本，新增 E2E 用例与全量回归由测试工程师负责。
- Playwright 自带 Chromium 未能安装（下载超时），E2E 使用系统 Chrome；正式 CI 仍使用自带浏览器，需以 CI 结果为准。
- 预览文档的存储回退是 `Object.defineProperty(window, 'localStorage')` 的内存实现，且仅用于 component-library 预览：若未来浏览器禁止重定义，Studio 的 rating/vote 仍可能降级为不可交互（不会影响其它标签与公开站）。正文/Markdown 真实上下文按设计不注入，rating/vote 仍不可交互。
- 并发上限只作用于正文 NodeView；Studio/插入面板/顶栏仍按自身交互触发请求，单页同时打开多个面板仍可能并行请求。
- Markdown 分屏失败时 API 传输层仍可能显示全局错误提示（既有行为）；本次只保证不清空预览并提供局部重试。
- 滚动比例恢复依赖预览子文档回传的最后比例；若用户从未在 iframe 内滚动，恢复比例为 0。

### 需要用户裁决的设计点

- 无新增装饰性元素。新增的“更新中…”与错误条均复用既有语义 token/尺寸；若希望给插入面板增加常驻 “Fixture” 徽标（Studio 已有同类徽标），需要另行确认视觉方案，本次未擅自添加。

## 独立验收（测试工程师，2026-10-31）

### 环境与产物绑定

- 系统 Chrome `156.0.8078.74`（Playwright 1.62.1，临时配置 `.tmp/chrome/playwright.config.ts` 使用 `channel: chrome`）；`pnpm exec playwright install chromium` 单次尝试超时，未使用自带 Chromium。
- 隔离服务端口 `8096`，`scripts/e2e/run-server.sh` 的临时根目录；服务日志明确加载 `modules/console/web/dist (dev)`。
- E2E 构建 `assets/index-DjmV8VyV.js`；最终生产构建 `assets/index-CYhalqKO.js`。两者都经 `dist/index.html` 与 `/console/` 返回的脚本引用逐字比对一致。

### 新增/执行的验证

- 新增 `tests/e2e/tag-live-preview.spec.ts`（7 个用例）：NodeView 请求合并/面板零请求；NodeView 更新中保留 iframe 与高度；插入面板 friends；Studio friends+sites；390px 悬停/聚焦；Markdown 分屏 500 失败与重试；评分/投票预览不写真实端点。
- `make test` exit 0；`pnpm exec vue-tsc -b` exit 0；`make build` exit 0。
- 新增 E2E 7 passed（55.2s）；既有 `shortcode-visual-editor.spec.ts` + `tag-plugin-parity.spec.ts` 26 passed（1.2m）。
- `plugin-preview-engine.test.ts` 9 passed；相关 5 个单测文件 38 passed；`TestPreviewFixtureCORS*`/`TestPreviewFixturePreflight` 通过。

### 数值证据（浏览器实测）

| 检查 | 实测 |
| --- | --- |
| NodeView 标题连续 10 字符 | 预览请求 **1** 次；属性面板开关新增 **0** 次 |
| NodeView 更新中/成功后 | iframe 身份保持；高度 **118 → 118 → 118**；旧 srcdoc 保持，最终更新 |
| 插入面板 friends | **4** 张卡片；高度 **240 → 240 → 240**；`/api/v1/preview/friends` 200 + `ACAO: *` |
| Studio friends / sites | friends **4** 张、sites **3** 张；高度 240；sites Fixture `ACAO: *` |
| 顶栏 390×844 | `scrollWidth=390`、`innerWidth=390`（溢出 0）；气泡宽 **320**、右边界 **359**；friends 4 张；tabs 2 个 pane；切换后 friends 卡片 0；focus 同样出预览 |
| Markdown 500 | 错误条出现；旧 srcdoc 逐字保持；重试恢复新正文；仅 1 条预期 500 资源错误 |
| 真实写入 | `POST /api/rating`、`POST /api/vote` **0** 次；vote Fixture 200 + `ACAO: *` |

- 新截图：`docs/baselines/tag-live-preview/e2e-toolbar-bubble-390.png`、`e2e-toolbar-focus-390.png`、`e2e-toolbar-390.json`、`acceptance-e2e.json`；实现者原截图未覆盖。
- 浏览器无 pageerror；除被强制 500 的预期资源错误外，无未预期 console error。

### 非阻断缺陷（既有 Spec 63 遗留，未自行修复）

- **rating 预览仍被 CORS 阻断且数据形状不匹配**：`rating.js` 请求 `/api/v1/preview/rating/info`、`/api/v1/preview/rating/update`，路由只注册了 `/api/v1/preview/rating`；实测 `/api/v1/preview/rating/info` 为 404 且无 ACAO，Studio 星级均分显示 `(0.0)`，控制台报 CORS。Fixture 返回 `{avg,count}`，而 `rating.js` 期望 `{rating:{score:count}}`。
- 复现：Studio 搜索 rating → 选中 → 等待预览；控制台出现 `blocked by CORS policy`；`curl -i /api/v1/preview/rating/info?id=x` → 404。
- 影响：rating 预览无 Fixture 数据；不会写真实评分。本任务 CORS 只覆盖 `/preview/rating`，未覆盖 `/info`、`/update`。
- 建议：补 `/preview/rating/info` GET 与 `/preview/rating/update` POST（含 CORS/OPTIONS）并对齐响应形状；或统一改写 data-api。属既有问题，是否并入本任务由项目经理裁决。
- **修复轮次已按上述建议修复（L4）**，见下节。

## 修复轮次（2026-10-31，Asia/Taipei）

状态：实现中；H1/M1/M2/M3 与 L1–L5 已由测试工程师独立复验通过；复验同时发现 tabs 预览文本在深色容器中不可见（计划验收第 6 项失败），修复前不标「待体验」。

### 阻断项

- **H1 预览可向真实 `/api/vote/update` 写数据**
  - 新增 `injectPreviewWriteGuard`：在 charset 之后、站点脚本之前包装 `window.fetch`、`XMLHttpRequest.prototype.open/send`、`navigator.sendBeacon`。GET/HEAD/OPTIONS 放行；非幂等方法仅允许 `/api/v1/preview/*`（相对路径或与文档 base 同源）；其余直接拒绝——fetch 返回 rejected Promise，XHR/beacon 抛错/返回 false，并在 `window.__stellarPreviewWriteBlocked` 留诊断。
  - URL 判定抽为纯函数 `previewWriteAllowed`；`about:srcdoc` 等 opaque base 用固定 base 解析，解析失败退化为 pathname 比较且默认拒绝，不会因异常放行。
  - `preparePreviewDocument(document, profile)` 统一预处理：写守卫始终注入；内存 Storage 仅在 `profile === "component-library"` 注入。`usePluginPreview` 按请求 profile 调用；`MarkdownEditor` 传 `null`，因此 NodeView/Markdown 保持 rating/vote 不可交互。
  - 浏览器实测：NodeView/Markdown 的 srcdoc 有写守卫、无 Storage；iframe 内 `fetch('/api/vote/update',{method:'POST'})` 返回 `PreviewWriteBlockedError` 并写入诊断；`localStorage` 仍抛错；真实写请求 0 次。Studio/插入面板/顶栏仍能写 `/api/v1/preview/*`。
- **M1 顶栏气泡切换串台**：记录 `shownPluginName`，A→B 在调度前 `clearPreview()`（显示加载占位），同插件重复进入不清空；新增 SFC 单测精确断言 clear 次数与调度顺序。
- **M2 focus 绕过防抖/点击多请求**：hover 与 focus 共用同一个 300ms timer；`insert/hide` 前清 timer；菜单项增加 `@blur`；预览接入 `sharedPreviewQueue`。新增 SFC 单测：快速 focus 3 项只渲染最后一项；focus 后点击插入 0 补发请求；浏览器实测 3 项 rapid focus 仅 1 次请求。
- **M3 无效参数无提示**：新增纯函数 `previewInvalidNoticeVisible(canApply, parsing, hasDocument)`；插入面板/Studio 在 `!canApply && !parsing && 有旧文档` 时叠加“参数无效，预览未更新”（复用 `text-quaternary/card`），校验中不误报；旧 iframe/srcdoc 保持。浏览器实测 copy 清空必填参数后提示出现、iframe 与 srcdoc 未变。

### 低风险项

- **L1** `resetHeight()` 与初始高度统一受 `options.frame.minHeight/maxHeight` 钳制；顶栏首个文档前稳定在 180–360。
- **L2** NodeView 删除本地 `iframeH` 与第二套高度校验，改用 composable `frameHeight/onMessage`；`stellar-tag-preview.test.ts` 改为加载真实 `usePluginPreview`（仅注入 `@/api`）驱动，断言 token/source/边界不变。
- **L3** NodeView/Studio/插入面板/顶栏 iframe 高度增加 150ms 过渡（Studio CSS `0.15s`，其余 `transition-[height] duration-150`）。
- **L4（Markdown 文案）** 新增 `previewFailureMessage(hasDocument)`：首次失败显示“预览渲染失败”，有历史文档才显示“仍显示上一次结果”。
- **L4（rating 契约）** 新增 `GET /api/v1/preview/rating/info`（`{rating:{"5":86,"4":30,"3":8,"2":4,"1":2}}`，支持 normal/empty/error）与 `POST /api/v1/preview/rating/update`（`{ok:true}`），含 CORS/OPTIONS，保留旧 `/preview/rating`；vote 的 `{votes:{up,down}}` 形状已核对一致。
- **L5** `preview_cors_test.go` 增补：未鉴权 `POST /preview/markdown` 401 且无 ACAO；rating info/update 的 CORS/预检/形状；vote info/update 形状；确认鉴权端点无通配 CORS。该文件与 `tests/e2e/tag-live-preview.spec.ts` 已加入文件边界。

### 修复轮次实测

- `make test` exit 0（console 397/397；Go/Staticcheck/module-check 全绿）；`pnpm exec vue-tsc -b` exit 0；`make build` exit 0（产出 `./blog`、`dist`、`dist-uc`）。
- E2E 33 passed / 0 failed（`tag-live-preview` 7 + `shortcode-visual-editor` 15 + `tag-plugin-parity` 11），系统 Chrome + 隔离 8096。
- 新增单测：`use-plugin-preview.test.ts`（9 例，真实 composable：保留旧 doc/缓存 0 请求/失败 retry/frame 钳制/holdHeight/卸载/迟到）、`editor-toolbar-preview.test.ts`（4 例，切换清空/focus 防抖/点击取消/queue）、写入守卫与 profile 收窄/提示文案（`plugin-preview.test.ts`）。
- 浏览器证据：`docs/baselines/tag-live-preview/invalid-params-notice.png`；NodeView/Markdown 写守卫与 NodeView 真实写 0 次；顶栏 A→B 切换后旧 friends 卡片 0 张。
- 未覆盖：E2E 全量复验仍由测试工程师执行；系统 Chrome 与 CI 自带 Chromium 的最终结果以 CI 为准。复验前状态保持「实现中」。

## 复验轮次（测试工程师，2026-10-31 22:50 Asia/Taipei）

### 环境与命令

- 系统 Chrome `156.0.8078.74`（Playwright 1.62.1，`.tmp/chrome/playwright.config.ts`，`channel: chrome`）；隔离 root/DB/config/content/uploads + 8096；生产构建 `assets/index-Bj5s8NTO.js`，`/console/` 返回引用与 `dist/index.html` 一致。
- `make test` exit 0（console **397/397**）；`pnpm exec vue-tsc -b` exit 0；`make build` exit 0。
- 新增 E2E **15 例：14 passed / 1 failed**（失败为下述可见性）；既有 `shortcode-visual-editor` 15 + `tag-plugin-parity` 11 = **26 passed**。
- 新增/相关单测：`use-plugin-preview` + `editor-toolbar-preview` + `plugin-preview-engine` = **22 passed**；后端 CORS/rating 定向 go test 通过。

### H1 / M1–M3 / L1–L5 复验裁决

| 项 | 结论 | 实测 |
| --- | --- | --- |
| H1 真实上下文写防护 | 通过 | NodeView/Markdown srcdoc 有 `data-preview-write-guard`、无 `data-preview-storage`；fetch `/api/vote/update`、`/api/rating/update`、XHR、sendBeacon 全部被 `PreviewWriteBlockedError` 拒绝并写 `__stellarPreviewWriteBlocked`；真实写 **0**；pageerror **0** |
| Fixture 对照 | 通过 | Studio rating：`/info` 200 + `ACAO:*`，`{rating:{5:86,4:30,3:8,2:4,1:2}}`，均分 **4.5**、**130** 人；`/update` 200 + `ACAO:*`；vote 86→87、`/vote/update` 200 + `ACAO:*`；真实写 **0**；无 CORS console error |
| M1 串台 | 通过 | A→B 挂起 B 时 `held=1`，气泡内无旧 iframe（`mixedWhilePending=false`）；B 就绪后 2 个 tab pane，friends 卡片 **0** |
| M2 focus 防抖 | 通过 | 4 项 rapid focus → component-library 请求 **1**；点击插入前后均为 **1**（不补发） |
| M3 无效参数 | 通过 | Studio 与插入面板：`{% copy %}` 时“参数无效，预览未更新”可见、旧 srcdoc/iframe 身份保持；补全后提示消失并更新 |
| L1/L2/L3 | 通过 | resetHeight 钳制单测；NodeView 高度 118→118→118 且由 composable 驱动；四处 iframe 均有 150ms height 过渡 |
| L4 Markdown 文案 | 通过 | 首次失败显示“预览渲染失败”，无历史文档；历史失败仍显示“仍显示上一次结果”（既有用例）；重试恢复 |
| L4 rating 契约 | 通过 | `/preview/rating/info` 200 + `ACAO:*`，形状与 rating.js 一致；`/update` 200 + `ACAO:*`；旧 `/preview/rating` 保留 |
| L5 CORS 负向 | 通过 | 未鉴权 `POST /preview/markdown` 401 且无 ACAO；Fixture 端点 `ACAO:*` + OPTIONS 204 |

### 失败：计划验收第 6 项（tabs 预览可见内容）

- **现象**：tabs 预览的 `.nav-tabs` 文本为 `rgba(0,0,0,0.8)`，iframe/article/body/html 背景全透明；实心父容器为深色（Studio `rgb(16,16,18)`、插入面板/顶栏 `rgb(18,18,20)`）。三处对比度仅 **1.08 / 1.10 / 1.10 : 1**（期望 ≥4.5），截图预览区为深色空白。friends/sites 因卡片自带白底未暴露。
- **复现**：Studio 搜 tabs 选中；或插入面板选 tabs 并填源码；或顶栏插入菜单 focus tabs；等待 `.nav-tabs` 在 iframe DOM 中出现，观察预览区（或运行新增 E2E“可见性”用例）。
- **期望/实际**：动态标签内容应可见；实际文本与背景同色系，用户看到空壳。
- **证据**：`e2e-visibility-studio-tabs.png`、`e2e-visibility-insert-tabs.png`、`e2e-visibility-toolbar-tabs.png`、`acceptance-e2e-fix-round.json`。
- **建议**：给预览 iframe 设置与文档主题一致的不透明背景（浅色文档 → 白底），或为预览文档注入深色主题；覆盖 Studio/插入面板/顶栏，不改公开站、不放宽 sandbox。
- **影响**：这是本轮唯一失败项；H1/M1–M3/L1–L5 已通过。状态保持「实现中」。

### 未覆盖

- Playwright 自带 Chromium 未安装，CI 浏览器结果以 CI 为准。
- NodeView 中 tabs/纯文本标签的深色背景可见性未单独探针（与三处组件库预览同源，风险相同）。
- L3 高度过渡只做代码与数值核验，未逐帧录屏。

## 最终验收（测试工程师，2026-10-31 23:56 Asia/Taipei）

### 环境与产物

- worktree 与分支不变（`codex/feat/20261031-blg-20261031-03-tag-live-preview`，基于 `dev` @ `023d7cac`，全部未提交）。系统 Chrome `156.0.8078.74`（Playwright 1.62.1，`.tmp/chrome/playwright.config.ts`，`channel: chrome`）；自带 Chromium 仍未安装。
- 隔离 root/DB/config/content/uploads + 8096（`scripts/e2e/run-server.sh`）；服务日志明确 `console SPA loaded from …/modules/console/web/dist (dev)`。
- 产物一致性：`make build` 产出 `./blog` 与 dist 入口 `assets/index-Ds4cieaH.js`；E2E 用 `VITE_E2E=1` 重建后入口 `assets/index-sDz-y3X8.js`。验收期 `/console/` 返回 HTML 与 `modules/console/web/dist/index.html` **md5 完全一致**（`7a016fa417f1f209d00ffe4445d83e1a`），入口 js 返回 200。

### V1–V6 落地核对（独立阅读代码）

| 项 | 位置 | 结论 |
| --- | --- | --- |
| V1 预览表面 | `preview-document.ts` `injectPreviewSurface`/`previewSurfaceStyle`/`previewSurfaceScript`，由 `preparePreviewDocument` 统一调用（NodeView/Markdown/Studio/插入面板/顶栏） | 已落地 |
| V2 写守卫 | 精确 fixture 路径集合；协议相对/带 scheme 绝对 URL 拒绝；请求时读取实时 `baseURI` 并与安装快照比对 | 已落地 |
| V3 默认下限 | `usePluginPreview.frameLimits()` 缺省 `resolvePreviewHeight(getInfo())` | 已落地 |
| V4 跨文档残留 | `MarkdownEditor` watch `storageKey` → `clearPreviewIdentity()`（清 srcdoc/token/错误/滚动比例） | 已落地 |
| V5 失败态高度 | `onChange` 在 `ready/idle/error` 释放 `heightHold` | 已落地 |
| V6 确定性等待 | `stellar-tag-preview.test.ts` 20ms 墙钟等待已改为 nextTick+setImmediate 轮询 | 已落地 |

### 命令与结果

| 命令 | 结果 |
| --- | --- |
| `make test` | exit 0（Go/vet/staticcheck/module-check 全绿；console 402/402） |
| `pnpm exec vue-tsc -b`（console/web） | exit 0 |
| `make build` | exit 0（`./blog`、dist、dist-uc） |
| Playwright（系统 Chrome + 8096，3 个 spec） | **43 passed / 0 failed（2.9m）**：tag-live-preview 17（原 15 + 新增 2）、shortcode-visual-editor 15、tag-plugin-parity 11 |
| curl 复核 Fixture | friends 200 + `ACAO:*`；vote/update OPTIONS 204 + `ACAO:*`；rating/info 200 + `ACAO:*`；rating/update 200 + `ACAO:*`；未鉴权 `POST /preview/markdown` 401 且无 ACAO |

### 五处 tabs 可见性（真实 Chrome，计算样式 + 截图像素）

| 位置 | 浅色表面 / 对比度 | 深色表面 / 对比度 |
| --- | --- | --- |
| 正文 NodeView | `rgb(249,250,251)` / **12.27:1** | `rgb(28,31,33)` / **10.97:1** |
| Markdown 分屏 | 同上 / **12.27:1** | 同上 / **10.97:1** |
| 标签 Studio | 同上 / **12.27:1** | 同上 / **10.97:1** |
| 侧栏插入面板 | 同上 / **12.27:1** | 同上 / **10.97:1** |
| 顶栏气泡（桌面） | 同上 / **12.27:1** | 同上 / **10.97:1** |
| 顶栏气泡（390×844） | 同上 / **12.27:1**，`scrollWidth-innerWidth=0` | — |

- 表面不透明：五处 iframe `html`/`body` 背景 alpha 均为 1；深色主题背景亮度 **0.013**（深底），与站点主题（`--background`）一致；NodeView 文档稳定后 800ms 高度差 ≤2px（用例通过）。
- 证据：`final-visibility.json`、`final-visibility-dark.json`、`final-visibility-*.png`、`final-dark-*.png`；旧 `e2e-visibility-*.png` 由用例重跑重新生成（原失败数值保留在「复验轮次」文字中）。用 PIL 抽查像素确认不是纯色空白（如 studio 浅色 uniq=328、深色 uniq=441，含文本像素）。
- **非活动页签**锚点继承主题 `--text-p3`：浅色 **3.94:1**、深色 **5.11:1**；活动页签/容器 0.8 → 12.27/10.97。3.94 低于 4.5，属公开站同款主题 token、本任务未改主题 CSS，供 PM/用户裁决是否单独立项。

### 交互与写防护

- 真实 vote/rating 写 **0**（H1 两例：fetch/XHR/sendBeacon 全部 `PreviewWriteBlockedError`，`realWrites=[]`）；Fixture 端点仍可交互（rating 4.5/130、vote 86→87）。
- M1 切换：B 未就绪时气泡无旧 iframe，就绪后 friends 卡片 0；M2：4 项 rapid focus 仅 1 次请求，点击插入不补发；M3：Studio/插入面板“参数无效，预览未更新”出现、旧 iframe/srcdoc 不变，补全后消失。
- Markdown：首次失败显示“预览渲染失败”，历史失败“仍显示上一次结果”，重试恢复。
- V4/V5 由单测覆盖（`markdown-editor-draft.test.ts` 切换文档身份、`use-plugin-preview.test.ts` 失败释放 holdHeight）；E2E 未单独覆盖跨文档切换。

### 本轮新增/调整的测试（仅测试文件与证据）

- `tests/e2e/tag-live-preview.spec.ts` 新增 2 例：NodeView/Markdown/390 可见性；深色系统偏好下五处可见性。
- 可见性取证增加“等外部样式表结算 + 绘制预热”：本机 Chrome 窗口被遮挡时窗口帧被抑制，首张截图会空白；iframe 内 rAF 被节流，改用父页短等待。仅影响截图取证，不改任何断言口径。

### 未覆盖与残余风险

- CI 自带 Chromium 未安装，最终以 CI 浏览器结果为准。
- 深色以 `emulateMedia({colorScheme:'dark'})` 模拟系统偏好（站点 `appearance=auto`）；未另存站点 `appearance=dark` 配置实测。
- 站点文档引用的高亮样式表 `gcore.jsdelivr.net/...` 为 render-blocking，本机首绘延迟约 2–4s，期间预览区空白；属既有公开站行为（Stellar 默认 `highlight_stylesheet`），非本任务引入，建议另立任务改本地/非阻塞资源。
- L3 高度过渡仅代码/数值核验，未逐帧录屏；并发上限仅作用 NodeView（已知）。
- 结论：通过，状态「待体验」；非活动页签 3.94:1 作为非阻断项上报。

## 收尾修复（2026-11-01，Asia/Taipei）

状态：最终验收后复审发现一个 E2E 未覆盖的可见回归（M1）与一个写入守卫边界（L1）；本轮先把状态改回「实现中」，修复并完成开发自测后回到「待体验」。

### M1 MarkdownEditor 在 storageKey 变化但正文不变时清空预览且不重建（已修复）

- 触发路径：新建文章输入正文、等待预览完成 → 保存草稿；`PostEditor.adoptCreatedSlug` 把 `draftStorageKey` 从 `post:new` 改成 `post:<slug>`，正文不变、编辑器实例不重建。`clearPreviewIdentity` 旧逻辑只在 `contentChanged` 时调度，且 `clearTimeout(previewTimer)` 会取消同 tick 内 context watcher 排好的调度，导致 `previewSrcdoc=""` 长期空白。`NoteEditor`/`WikiDocsView` 在 id/slug 落定时同路径。
- 修复（`modules/console/web/src/components/MarkdownEditor.vue`）：
  - `clearPreviewIdentity()` 清空文档、token、错误与滚动比例后，只要 `mode !== "edit"`，统一在 `nextTick` 后 `schedulePreview(true)`；不再依赖 `contentChanged`，也不再让 `clearTimeout` 吞掉同 tick 的 watcher 调度。`nextTick` 合并同 tick 的 modelValue/context 变化，只发一次请求。
  - `refreshPreview()` 发起时记录 `props.storageKey` 身份；成功与失败落地前均要求序号与身份都未变化，A 的迟到正文不会渲染到 B 的身份下。
  - 保留“B 首次失败不残留 A”：身份切换先清空 srcdoc，失败只显示“预览渲染失败”，不保留旧文档。
  - 新增 `previewActive` 卸载守卫，避免卸载后 nextTick 里的补调度再发请求；移除已无读取方的 `previewContent`。
- 单测（`modules/console/tests/markdown-editor-draft.test.ts`，9 例全绿，既有断言未削弱）：
  - 新增「storageKey 变化但正文不变时，仍重新请求并重建非空预览」：断言清空后请求数 1→2、第二次正文仍是同一正文、srcdoc 含新身份文档且不含旧身份。
  - 新增「身份切换后迟到的 A 响应被丢弃，B 失败不残留 A」：A 挂起 → 切换身份 → B 500 → srcdoc 保持空并显示“预览渲染失败” → 再 resolve A，srcdoc 仍为空。
  - 变异检验：把补调度改成 `if (false)` 模拟旧行为后，上述 2 例与既有「切换文档身份…B 首次失败不残留 A」共 **3 例失败**；恢复后全绿。
  - 既有「切换文档身份时清空旧预览，B 首次失败不残留 A」保持通过。

### L1 反斜杠绝对 URL 被写入守卫放行（已修复）

- 修复（`modules/console/web/src/modules/plugins/preview/preview-document.ts` 的 `previewWriteAllowed`）：
  - 写请求的 raw URL 含反斜杠一律拒绝（WHATWG URL 把 `\` 当 `/`，`\\evil.example/...`、`/\evil.example/...` 会变成外域协议相对 URL）。
  - 不再用同一字符串对 trusted base 重解析做恒真比较；改为分别解析“实时 base”和“安装时 trusted base”，要求 `实际 base.origin === trusted base.origin`，再要求 `解析结果.origin === 实时 base.origin`。
  - GET/HEAD/OPTIONS 仍放行；相对 fixture 路径与既有精确路径集合仍放行。
- 单测（`modules/console/tests/plugin-preview.test.ts`）：
  - URL 判定新增 `\\evil.example/...`、`/\evil.example/...`、`\\localhost:8096/...` 全部拒绝；GET 反斜杠仍放行；端口变化的动态 base 拒绝；非法 base 拒绝；同源 fixture POST 仍放行。
  - 运行时守卫新增：`fetch` 对两种反斜杠变体均以 `PreviewWriteBlockedError` 拒绝，`sendBeacon` 返回 false；原有 XHR/fetch/beacon、协议相对与动态 `<base>` 断言保持通过。

### P3 FixtureStudio 切换保留旧文档（已最小处理）

- `modules/console/web/src/modules/plugins/fixtures/FixtureStudioView.vue` 的 `selectedId` watch 在 `resetHeight + scheduleRender` 前调用 `preview.clearPreview()`，恢复“切换即清空”，新 Fixture 就绪前不再短暂展示上一个 Fixture 的文档。
- 同一次改动只加一行；`clearPreview` 是既有 API，未改共享预览引擎，未扩大风险。该视图不在三条 E2E 的覆盖范围内，由 vue-tsc、单测与构建覆盖；切换瞬间 iframe 会先空一帧再显示新文档，属预期。

### 实际命令与结果

| 命令 | 结果 |
| --- | --- |
| `make test`（worktree 根） | exit 0；console **404/404**（原 402，新增 2 例 M1 单测）；Go/vet/staticcheck/module-check 全绿 |
| `pnpm exec vue-tsc -b`（`modules/console/web`） | exit 0（仅 pnpm 配置字段提示，无类型错误） |
| `make build` | exit 0；产出 `./blog`、`dist`、`dist-uc` |
| `pnpm exec playwright test tests/e2e/tag-live-preview.spec.ts tests/e2e/shortcode-visual-editor.spec.ts tests/e2e/tag-plugin-parity.spec.ts --config .tmp/chrome/playwright.config.ts` | **43 passed / 0 failed（3.0m）**：tag-live-preview **17**、shortcode-visual-editor **15**、tag-plugin-parity **11** |
| 定向单测 `tsx --test ../tests/markdown-editor-draft.test.ts ../tests/plugin-preview.test.ts` | **40 passed / 0 failed**（9 + 31） |

E2E 环境：系统 Chrome `156.0.8078.74`（Playwright 1.62.1，`.tmp/chrome/playwright.config.ts`，`channel: chrome`；自带 Chromium 仍未安装，未重试）；`scripts/e2e/run-server.sh` 隔离 root/DB/config/content/uploads + 8096；未修改测试工程师的 E2E spec。H1 用例仍得到 `PreviewWriteBlockedError`，真实 vote/rating 写 0 次；五处 tabs 对比度仍为 12.27 / 10.97，390px 溢出 0。E2E 运行会重建 `dist`（`VITE_E2E=1`）并重写 `docs/baselines/tag-plugin-parity/after-*-mobile.png`，后者已恢复为 HEAD，未混入本轮改动；E2E 结束后已再次 `make build`（exit 0）恢复生产产物。8096 端口已释放。

### 未覆盖与风险

- M1 的新场景目前由 SFC 单测（真实 Vue props/watch 顺序）覆盖；未在本轮用真实浏览器复现“新文章保存拿到 slug”路径，E2E 由测试工程师补充。身份记录分支是纵深防御：正常路径下身份切换会先 `previewSeq++`，因此该分支无法在不绕过 watcher 的情况下单独隔离验证。
- L1 仍按“相对 fixture 路径”设计，绝对同源 fixture URL 继续拒绝（既有语义，未放宽）；守卫依赖标准 `URL` 与 `document.baseURI`，未在无 URL 的极老浏览器上验证。
- P3 仅代码级与构建/单测验证，无 E2E；切换时一帧空 srcdoc 属预期行为。
- 系统 Chrome 与 CI 自带 Chromium 的最终结果仍以 CI 为准。

## 收尾复验（测试工程师，2026-11-01 00:18 Asia/Taipei）

状态：**待体验**（M1/L1/P3 独立复验通过；五处可见性无回归）。工作树、分支、未提交状态不变；本轮只改 `tests/e2e/tag-live-preview.spec.ts` 与 `docs/baselines/tag-live-preview/` 证据。

### 环境与产物

- 系统 Chrome `156.0.8078.74`（Playwright 1.62.1，`channel: chrome`）；隔离 root/DB/config/content/uploads + 8096。
- `make build` 生产入口 `assets/index-BsgTtX4d.js`；E2E 构建入口 `assets/index-2jl8TjP4.js`。验收期 `/console/` 返回 HTML 与 `modules/console/web/dist/index.html` md5 一致（`c84abb436c6c12b03c3ed78a2c11356a`）；E2E 后已重新 `make build` 恢复生产产物并释放 8096。

### 命令与结果

| 命令 | 结果 |
| --- | --- |
| `make test` | exit 0（console **404/404**；go vet/staticcheck/go test 全绿） |
| `pnpm exec vue-tsc -b` | exit 0 |
| `make build` | exit 0 |
| Playwright（3 spec） | **45 passed / 0 failed（3.4m）**：tag-live-preview **19**（原 17 + 新增 2）、shortcode-visual-editor 15、tag-plugin-parity 11 |

### M1 真实 UI 流程（新增 E2E）

- 步骤：登录 → `/console/posts/new` → 切 Markdown 编辑器 → 输入标题与 `note` 正文 → 等待预览成功 → 保存草稿 → URL 变为 `/console/posts/edit/m1-身份切换验收`。
- 断言：保存后编辑器 textarea 与预览 iframe 的实例标记都在（未重挂）；**保存后预览请求 `held=1`、`previewRequestsAfterSave=1`**；扣住请求期间 `srcdoc=""`（身份切换先清空）、iframe 仍可见；释放后 `srcdoc` 非空且 **97484 字节**、含“新建文章预览正文”；无 pageerror、真实写 0。
- 旧行为对照说明：旧逻辑在正文未变时不补调度，`held` 会一直为 0，20s 轮询超时即失败（未运行变异，产品代码不动）。
- 证据：`fix-m1-storagekey-flow.json`、`fix-m1-storagekey-flow.png`（截图含重建后的 note 预览）。

### L1 写守卫（新增 E2E）

- 在 Studio rating 预览 iframe 内执行 `fetch('\\\\evil.example/api/v1/preview/rating',{method:'POST'})` 与 `fetch('/\\evil.example/api/v1/preview/rating',{method:'POST'})`：两者均 `PreviewWriteBlockedError`，诊断 `__stellarPreviewWriteBlocked` 记录原始 URL；`page.on('request')` 对 `evil.example` **0 次**。
- 正常 fixture 写 `/api/v1/preview/rating/update?id=l1-e2e&value=5` 放行：`resolved`、200、`ACAO:*`，守卫未写诊断；真实写 0、无 pageerror。
- 证据：`fix-l1-backslash-guard.json`。

### P3 / 可见性回归

- P3：代码复核 `FixtureStudioView` 在切换 fixture 时先 `preview.clearPreview()` 再 `resetHeight()/scheduleRender`；无 E2E，切换瞬间一帧空 srcdoc 属预期。
- 可见性：五处 tabs 浅色 **12.27:1**、深色 **10.97:1**，表面 `rgb(249,250,251)` / `rgb(28,31,33)` 均不透明；390×844 溢出 0；用例全部通过，无回归。

### 未覆盖与风险（本轮）

- P3 仅代码级；CI 自带 Chromium 未安装，以 CI 为准；深色仍以 `emulateMedia` 模拟系统偏好。
- 站点外部高亮样式表（gcore.jsdelivr.net）render-blocking，首绘存在秒级延迟（既有公开站行为）；M1 证据截图前已等样式表结算并做绘制预热。
- `docs/baselines/tag-plugin-parity/after-*-mobile.png` 由 E2E 重写后已恢复为 HEAD，未混入本轮改动。
