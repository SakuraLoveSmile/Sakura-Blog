# BLG-20261117-02：编辑器直插外部 HTML 并直接渲染

- 项目：personal-blog
- 任务编号：BLG-20261117-02
- 状态：待体验
- 创建日期：2026-11-17
- 权威记录：本文件
- 工作目录：`/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261117-feat-BLG-20261117-02-raw-html-insert`
- 分支：`codex/feat/20261117-BLG-20261117-02-raw-html-insert`
- 基线：`dev` @ `d522a2c2`
- 合入目标：根工作树 `dev`
- 运行资源：`resource/` 以绝对路径软链到根工作树共享目录（未克隆、未重置）

## 目标与范围

允许作者在文章/页面编辑器里直接插入外部 HTML 片段，并在默认的富文本（TipTap）所见即所得模式中直接渲染真实效果，Markdown 往返逐字节无损。

覆盖：
- `PostEditor.vue` / `PageEditor.vue` 共用的富文本与 Markdown 双模式。
- 工具栏「插入 HTML」入口 + 源码弹窗 + 实时预览。
- 外部 HTML 的可视化 atom 节点（拖拽 / 选中编辑 / 删除）。
- 从外部网页粘贴 HTML 的保真。
- 行内 HTML 保真。

排除：
- 瞬间发布框、前台访客评论框、站点设置代码注入（本次不改）。
- 不新增 sanitizer，不改后端渲染信任模型。
- 不重设计 Console 其他界面，不改 Stellar 前台样式。

## 已确认决定

1. **落点**：仅文章/页面编辑器。
2. **安全策略**：原样直出，作者即可信。与既有正文管线一致（`internal/post/post.go:190-197` goldmark `html.WithUnsafe()`），script/iframe/style/on* 全部保留。后端零改动。
3. **富文本程度**：完整可视化节点 + 粘贴保真。
4. **实现路线**：复刻 `StellarTag` 先例（atom 节点 + base64 占位 div + markdown-it core rule + iframe NodeView），不另造体系。
5. **HTML 块边界**：用 markdown-it token 层 `after("block")` rule，边界即 CommonMark，与 goldmark 发布语义一致；不做文本层扫描。
6. **foreign 判定共享一份实现**：纯字符串扫描器，core rule 与粘贴插件共用，避免规则漂移；干净语义 HTML（TipTap 原生标签 + 原生属性）保持可编辑，不退化为不透明卡片。
7. **编辑器内预览边界**：沿用 `sandbox="allow-scripts"` opaque iframe（CLAUDE.md:122/124），外部脚本在隔离文档内执行；发布后正文才是同源 `template.HTML` 直出。

## 既有行为（非本次引入，如实记录）

- `internal/stellarhtml/content.go:35-118`：正文含 `<table` 或 `<a` 时用 x/net/html 重序列化，大小写/void/实体归一，完整 `<html>/<head>` 被拍平，table 包 `md-table-scroll`，img 改 lazyload。`content_md` 逐字节无损，但「发布逐字节原样」不成立。
- `content.go:71-105`：无 class/role、href 与文本非空的 `<a>` 会被注入 `<span class="md-link-icon ui-icon">`。既有逃逸口：作者给 `<a>` 加 `class` 或 `role` 即跳过。
- `internal/post/shortcode.go` 的 `processShortcodes` 只保护围栏与行内代码，外部 HTML 的 `<pre>/<script>/<style>` 内若含 `{%` 会被展开（V2 同）。HTML 注释不能全局保护（chat/tabs/paper 依赖注释 marker）。列为可选切片 S8。

## 验收场景

1. 工具栏「插入 HTML」→ 弹窗写源码 → 实时预览可见 → 插入后富文本内直接渲染真实效果。
2. 选中节点出气泡，可编辑（回写源码）、可删除、可拖拽排序。
3. 富文本 ↔ Markdown 双模式切换，`content_md` 逐字节不变。
4. 从外部网页粘贴含 `style`/`div`/`iframe` 的 HTML，内容保真为 atom；干净 `<p><strong>` 仍可原生编辑；图片粘贴上传不回归。
5. 保存重载后内容不变；发布页确认内嵌 `<script>` 执行。
6. 既有 Stellar 标签、代码块、表格 round-trip 不回归。
7. 1280px 与 390px 视口无溢出、无遮挡、控件可达。

## 实现落点

新增：
- `modules/console/web/src/components/editor/raw-html-syntax.ts`：纯字符串扫描器 `scanHtmlTags`、共享判定 `isForeignHtml`、块/行内占位编解码、`registerRawHtmlRule`（token 层 `after("block")`）与 `registerRawHtmlInlineRule`（`after("inline")`）。
- `modules/console/web/src/components/editor/raw-html.ts`：`RawHtml` block atom 节点。
- `modules/console/web/src/components/editor/raw-html-inline.ts`：`RawHtmlInline` inline atom + `MarkHighlight`/`Subscript`/`Superscript` 三个 Mark。
- `modules/console/web/src/components/editor/raw-html-paste.ts`：`transformPastedHTML` 逐顶层块保真。
- `modules/console/web/src/components/editor/RawHtmlView.vue`、`RawHtmlInlineView.vue`、`RawHtmlInsertModal.vue`。
- 测试：`modules/console/tests/raw-html-syntax.test.ts`、`raw-html.test.ts`、`tests/e2e/raw-html-insert.spec.ts`。

修改：
- `components/RichTextEditor.vue`：注册六个扩展。
- `modules/posts/EditorToolbar.vue`：新增「插入外部 HTML」按钮与弹窗，提交走既有 `emit("insert")`，宿主零改动。
- `modules/console/tests/editor-toolbar-preview.test.ts`：模块桩白名单补入新依赖（工具栏确实新增了依赖，非放宽断言）。
- `package.json` / `pnpm-lock.yaml`：根 devDep 增 `markdown-it` 与 `@types/markdown-it`（行为测试在 `modules/console/tests/`，解析不到 `web/node_modules`）。
- `modules/console/web/src/icons/ri-subset.json`：`ri:html5-line` 重生成（211 → 212）。

后端零改动。

## 实现中发现并修复的缺陷

1. **空行切断 HTML 块**：CommonMark 在空行处结束 HTML 块，`<section>` 会被拆成 3 块。按标签配平把未闭合元素的后续块并回一个节点；已配平的块各自独立，干净语义 HTML 不被邻近外部块牵连。
2. **doctype 独立成块**：行号连续的 html_block 始终合并，避免 doctype 被丢弃。
3. **规则重复注册导致内容重复**（e2e 抓到、单元测试漏掉）：tiptap-markdown 对同一 markdown-it 实例多次调用 `parse.setup`，第二遍按 `token.map` 重新切片，把已并入占位的块再原子化一次。修复＝按实例 WeakSet 去重 + 基于 `token.content` 的幂等判定，并补重复注册回归测试。
4. **行内段放弃接管后闭合标签被单独芯片化**：会留下不配平输出。改为整段原样保留并跳过其余配平部分。
5. **mark/sub/sup 未纳入原生集合**：新增 Mark 后同步更新 `NATIVE_TAGS`，避免退化成 chip。
6. **TaskItem 自身生成的 checkbox 被原子化**（既有 tiptap-roundtrip golden 用例抓到）：`<input class="task-list-item-checkbox">` 的 class 不在原生属性集合内，任务列表往返退化成原始 HTML。放行该 class（其他 class 仍保真），并补回归测试。
7. **插入弹窗预览缺高度回传绑定**：iframe 会停在初始 120px 造成双层滚动。补 `frameRef` + `ownerDocument` 消息绑定与 `@load`，并加「高片段撑开 >400px」的可观察断言。
8. **UI 克制守卫拦截起始片段**（pre-commit 门禁抓到）：`ui-restraint.test.ts` 禁止未登记渐变与越界裸色。起始片段是作者插入的外部 HTML 内容示例、自带样式，不是 Console 界面样式；依守卫既有的「按文件 + 原因」功能性白名单机制，登记仅匹配 `html: '…` 样例行的窄范围例外（`FUNCTIONAL_EXTERNAL_HTML_STARTER`）。已做 mutation-check：在同文件界面 chrome 行临时加 `#ff00ff` 仍被守卫拦截，证明未放宽界面约束；探针已移除。

首屏可用性：弹窗提供三个自包含、离线可渲染的起始片段（渐变卡片 / 样式表格 / 折叠详情），一键填入即可看到真实效果，再按需改源码。

## 实测与未验证项

已运行并通过：
- `make test` 全量门禁：dist-bootstrap / console-test（vue-tsc + 654 项行为测试）/ assets-test / release-source-test / module-check / `go vet` / Staticcheck / `go test ./...` 全部通过。
- `pnpm lint`（eslint --max-warnings=0）：无错误无警告。
- `make build`：Console SPA + Go 二进制构建成功（exit 0）。
- `node modules/stellar/web/scripts/gen-ri-subset.mjs`：子集 212 图标。
- `pnpm run e2e tests/e2e/raw-html-insert.spec.ts`：9/9 通过（逐字 round-trip、插入闭环、起始片段、编辑回写与删除、粘贴保真、390px 弹窗、预览高度自适应、双模式切换、发布后脚本真实执行）。
- 回归 e2e（tiptap-roundtrip / shortcode-visual-editor / tag-live-preview / editor-md-import / console-post-editor-ui / editor-publish）：52 通过；唯一失败为本任务双模式用例的竞态写法，修正后复跑全绿。tiptap-roundtrip golden 基线零差异。
- 真实浏览器验收（隔离服务 :8097，当前 SPA 产物）：1280px 弹窗双栏实时渲染渐变卡片、插入后编辑器内节点真实渲染 + 选中环 + 操作气泡 + 拖拽把手、内联源码编辑 popover；390px 弹窗堆叠无横向溢出（scrollWidth-clientWidth=0）、控件可达；发布页逐字还原外部卡片。截图基线见 `docs/baselines/raw-html-insert/`（insert-modal-1280 / node-selected-1280 / node-edit-1280 / insert-modal-390 / node-390 / published-1280）。

未运行 / 待补：
- `make merge-check`（合入 dev 并复验组合态）。

## 遗留

- 回归运行 `tag-live-preview.spec.ts` 会重写它自己的 `docs/baselines/tag-live-preview/` 产物，其中 `effectiveBackground` 等字段属该 spec 与已提交基线之间的既有漂移，与本功能无关。已备份到 `.tmp/baseline-backup/` 后 `git restore` 恢复，保持本次改动范围只含本功能文件。
- 行内 HTML 若段内含 markdown 格式 token（如 `<span style>**粗**</span>`），不接管，沿用既有行为（span 丢失、粗体保留）。原因是这些 token 的 content 为空，重建会丢掉原文 `**` 标记。已在测试中显式断言该边界。
- 发布 HTML 经 `internal/stellarhtml` 归一化与链接图标增强，非逐字节原样（既有行为，见上）。
- 可选切片 S8（shortcode 对 `<pre>/<script>/<style>` 内 `{%` 的误伤掩码）未实施，属既有隐患，不阻塞本功能。