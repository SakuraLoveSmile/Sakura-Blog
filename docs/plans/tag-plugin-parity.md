# BLG-20261026-02：完整适配标签插件

- 状态：待体验；2026-10-26（Asia/Taipei）。本文件是本任务唯一权威记录。
- 目标：完整适配当前项目锁定的 Stellar 2.0.0 标签，从选择/配置、插入、再次编辑、保存到预览及公开页面交互都可用，原始 Markdown 不被编辑器损坏。
- 基线：`dev e05aec52fe9cb724f063648ce3132b321702047c`；上游参照 `9d11978a24fe6610e3a7dbab741cf62634539d05`，与 `modules/stellar/web/build/stylus/upstream.json` 一致。官方注册为 45 + 9 = 54 项，现有 V2 catalog 和渲染器已覆盖 54 项，保留并补齐实际缺口。
- 主工作树：`/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261026-feat-blg-20261026-02-tag-plugin-parity`；分支 `codex/feat/20261026-blg-20261026-02-tag-plugin-parity`，预期合入 `dev`。2026-10-26 用户已授权本地收尾；本地提交与合入 dev 由 BLG-20261026-03 统一执行，不含推送、发布或生产操作。
- 根目录用户文档、上一项 Console UI 工作树及 8130/8131/8132 服务保留。新浏览器服务用 8133、完整隔离 root/DB/config/content/uploads；Playwright 单独使用 8096，主 agent 独占运行。

## 固定规则与验收范围

1. 目录与参数以现有后端 catalog 为唯一真值，使用现有 parse/build/preview API；不另造前端参数表。54 项官方标签及已存在的 Blog 扩展不因修复而消失；官方正常 DOM、语义、默认值及 CSS 保持。
2. `content_md` 为保存真值。代码围栏/行内代码中的标签示例不激活；完整外层标签的原文包含原始内层标签，不能包含编辑器占位 HTML。同名/异名嵌套、标记分段、引号、空值、false、代码和 Unicode 保真。无法可靠解析的未知参数、未闭合结构保留源码编辑，不补默认值、不猜测重写。
3. 文章与页面都能从共享标签面板选插件、填写参数、看到完整运行时预览、插入正文。标签 Studio 的示例/直接源码编辑与表单同步；不可结构化的语法明确保持源码模式，不能用旧表单覆盖新原文。快速切换的迟到预览不得覆盖当前选择。
4. 预览消费服务端的完整 `data.document` 和现有 token/resize 协议；复用正式前台 ESM/manifest，而不是重新拼旧脚本。插件库和插入面板使用隔离 fixture，不写真实投票或评分；正文预览沿现有文档上下文。保留 iframe sandbox、安全校验与 SSRF 防护。
5. 公开页面的标签 runtime 需要成功、空、失败、取消及保留 DOM 重挂行为可核验。动态内容安全进入 DOM，文本、属性、URL、Markdown 各按实际上下文处理；不允许事件属性或危险 URL 执行。相对 API 按站点 base 解析；成功复挂无重复条目，取消/失败可重试，迟到响应不更新旧页面。
6. 官方 54 项/288 参数 oracle、完整文本渲染和编辑往返、代表类型真实 UI、动态交互分别作为证据。不能将目录数量或 fixture 成功冒充全部真实外部服务成功。真实第三方只做必要只读检查，不产生费用、消息或公开提交。

## 现状与视觉约束

- 唯一 UI 现状参照：`docs/baselines/tag-plugin-parity/before-editor.png`（主 agent 在开始面板 UI 修改前从本轮隔离服务保存）；Studio 与页面编辑器是辅助截图。
- 继承当前 EditorSidebar 插件分区、PluginParamForm 和 StellarTagView 的组件/语义 token；遵循本地 apple-design 消费入口，不更换全局 token 或组件库。
- 文章侧栏原有分类、字段和插入按钮顺序保持；将其标签内容提取成共享面板，修正预览实现。页面侧栏在“大纲/详情”后增加“标签”入口，复用同一面板；小屏沿现有辅助面板打开方式，控件/预览不超出容器。
- Studio 保留左目录、右参数/源码/预览的整体结构。必要时显式显示“源码编辑/参数编辑”状态与无法结构化的原因；不顺带重做整页。错误、加载、无效参数保留明确状态，插入/复制不可提交过期有效值。
- 真实浏览器核验文章/页面插入与重新打开、Studio 源码/表单转换、桌面和 390px 手机，截图放本任务 baseline 目录。

## 已核实缺口

- 富文本 `stellar-tag.ts` 逐类全串正则预处理会把围栏中的标签替换成占位 div；不同类型嵌套会先改内层，再将被污染的内容编码进外层 raw。只读执行已复现，需真实保存重开回归。
- 文章 EditorSidebar 丢弃 `data.document`，只拼 CSS 和旧 tabs 脚本；页面没有主动插入标签入口。Studio 目前使用真实预览文档，不能因未使用的 legacy resources.ts 推断其预览失败。
- Studio 的 example/modelValue 与 PluginParamForm 初始化值不同步；手写源码可能被旧表单覆盖或仍被旧 required 状态禁用复制。
- 官方 friends/sites/timeline 数据路径未充分处理远端文本/URL/Markdown；同 DOM 重挂追加重复数据；timeline 对合法相对 API 调用 `new URL(api)` 抛错。全内存脚本夹具已复现，不代替真实浏览器证据。
- marker 型同类嵌套可能被全局 marker 分段切断；先用真实 Go 渲染红例确认，再修复已证实问题。不可仅凭推演改行为。

## 执行步骤与唯一写入者

| 步骤 | 结果与独占范围 | 实施/验证 | 状态 |
|---|---|---|---|
| T1 | 语法专家：`components/editor/stellar-tag.ts`、必要的相邻纯解析 helper、对应 Console 单测 | 一次区间扫描保护代码和完整嵌套原文；行为测试先红后绿；不写侧栏/Studio | 待体验 |
| T2 | 编辑器专家：EditorSidebar、PageEditorSidebar/PageEditor 接线、共享标签面板、PluginParamForm、ShortcodeStudioView、usePluginPreview 及对应单测 | 复用 catalog/parse/build/完整预览文档；表单与源码双向同步、页面可插入、迟到请求保护；先等待 UI 基线保存 | 待体验 |
| T3 | 运行时专家：`assets/js/services/`、`runtime/extensions/services.js`、必要的局部安全 helper、对应 Stellar 单测 | 动态数据安全、相对 URL、挂载幂等、取消/失败/重试；不改生成 CSS 或通用 utils 行为 | 待体验 |
| T4 | 渲染专家：`internal/post/shortcode_v2*` 中 marker/嵌套相关实现和 Go 回归 | 真实渲染确认红例；修复外层/内层 marker 边界，保持54项官方oracle语义；不改共享API schema，确需改动先交主agent协调 | 待体验 |
| T5 | 主 agent：本记录、证据、最终 E2E 与组合集成 | 54项/288参数、全部标签样例；文章/页面保存重开；Studio及公开交互；完整 make test/build；最终服务资产绑定 | 待体验 |

已执行并行只读核查：Sol analyst 分别核查渲染/runtime与编辑器，Luna explorer 核查测试/隔离。实施用 Sol worker，各自独立 worktree（主路径加 `-syntax`、`-preview`、`-runtime`、`-render`），分支同后缀，不共享写入。权威规则在本主树文件；主 agent 串行接入独占 diff，统一 E2E 与组合验收。原子子任务自行完成、不继续派生。

## 验证与完成判定

- 原实现已经包含四项修复与浏览器用例；此前 .tmp 中有测试、构建和 26 项 E2E 日志，但没有完整源码指纹，本次归档保留为历史证据。当前组合态验证由 BLG-20261026-03 重新执行，结果以收尾记录为准。
- 必需：`make test`，真实 `make build`；受影响的 Go/Console/Stellar 行为测试；`shortcode-visual-editor.spec.ts` 及本次完整用户路径 E2E。如改生成 CSS，才追加 `make css`/`make css-drift-check`。
- 保留每项缺陷的失败基线和修复结果；变更已有测试预期须说明依据，不降低原文保真、权限、隔离或运行时检查。
- 主 agent 用最终单一组合态检查所有要求，记录源码指纹、构建、服务/浏览器资产、实际使用流程、mock和外部服务边界。全部必要验证完成才“待体验”，只有用户确认才“已验收”。
- 未提交/未合入期间保留本任务成果和体验环境；合入后按 AGENTS 保全并清理本任务 worktree。不碰其他任务、共享 resource 或生产数据。

## 2026-10-26 本地收尾

用户已授权本地提交、集成与完成后清理。源码已按所属功能进入 BLG-20261026-03 集成分支，最终组合验证和来源保全见 [branch-closeout.md](branch-closeout.md) 及 `docs/baselines/branch-closeout/verification.json`；合入与清理的实际结果见其链接的 `cleanup.json`。前文“尚未提交”和旧体验端口属于初次交付历史，不代表本轮状态。旧工作树与体验资源在合入验证后由收尾任务归档清理；用户尚未明确确认体验，不标已验收。
