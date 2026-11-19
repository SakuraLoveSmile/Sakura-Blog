# BLG-20261119-03：整页 HTML 文档隔离渲染

- 项目：personal-blog
- 任务编号：BLG-20261119-03
- 状态：实现中
- 创建日期：2026-11-19
- 权威记录：本文件
- 工作目录：`/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261119-fix-BLG-20261119-03-raw-html-document`
- 分支：`codex/fix/20261119-BLG-20261119-03-raw-html-document`
- 基线：`dev` @ `b9d86dae`
- 合入目标：根工作树 `dev`

## 背景（线上反馈）

2.1.10 的「插入外部 HTML」面向**片段**。作者把完整的导出页面（`<!doctype html>…</html>`）粘进去后出现两个问题：

1. 预览里多出一行可见的 `<!doctype html>` 文本。
2. 预览版式错乱、内容显示不全。

### 根因（已在本地复现）

- **doctype 变文本**：goldmark 的 HTML 块只把 `<!` + **大写**字母识别为声明块。大写 `<!DOCTYPE html>` 原样通过，小写 `<!doctype html>` 被当普通段落转义成 `<p>&lt;!doctype html&gt;</p>`。
- **版式错乱**：整页文档里的 `html/head/body` 标签会被浏览器丢弃，剩下的 `<style>` 里的**页面级**规则（`body{padding:16px;background:var(--surface-1)}`、`*{margin:0;padding:0}`、`#vis-container{overflow:hidden}` 等）直接作用在预览页与文章页上，改写宿主版式。
- 这不是预览独有：发布走同一条渲染管线，问题一致。

## 已确认决定

1. 用户选择「整页隔离」：整页文档用 sandbox iframe 承载，片段仍然内联。
2. 判定与渲染放在**后端 Markdown 渲染**这一层，预览 API、编辑器节点、发布页共用同一结果。
3. sandbox 保持 `allow-scripts`（opaque origin）：外部脚本在隔离文档内执行，不能触碰宿主页面。
4. 片段（无 doctype / 非 `<html>` 开头）行为完全不变。

## 实现落点

新增 `internal/post/raw_html_document.go`：

- `isolateRawHTMLDocuments(body string) string`：把正文里的整页文档替换成隔离 iframe；文首或空行之后、且不在代码块/缩进块内的 `<!doctype html>`/`<html>` 才算整页文档；以 `</html>` 收尾，找不到结束标签时仅当整篇都是文档才隔离到底。
- `renderRawHTMLDocumentFrame`：生成**单行** `div + iframe(srcdoc) + 高度回传脚本`，单行是为了让 goldmark 把它当作一个 HTML 块。
- srcdoc 注入高度回传脚本（`body.scrollHeight` 优先，避免「iframe 视口高度回填」的正反馈环）；外层脚本按 `event.source` 只调整自己那一组 iframe，高度钳制在 120–5000px。

修改：

- `internal/post/post.go` `Render`：插件 shortcode hook 之后、shortcode/goldmark 之前调用隔离。
- `internal/post/shortcode_v2.go` `RenderStellarV2WithContext`：同上。

测试：`internal/post/raw_html_document_test.go`（6 例）。

## 验收场景

1. 整页文档 → 输出单个 `iframe[data-raw-html-frame]`，`sandbox="allow-scripts"`，srcdoc 逐字保留原文，不再出现 `<p>&lt;!doctype`。
2. 文档里的 `<style>` 不泄漏到宿主页面（宿主 body 背景/内边距不变）。
3. iframe 高度按内容回传（实测 342px），无反馈环。
4. 片段（`<div class="card">`）仍内联渲染，不生成 iframe。
5. 代码块内的整页文档不被隔离，仍按代码块展示。
6. 文档前后还有 Markdown（标题、段落）时，只隔离文档本身，其余正常渲染。
7. 编辑器预览、发布页与上面同一份输出。

## 实测与未验证项

- 单元测试：`go test ./internal/post/...` 通过（7 个新用例 + 既有用例）。
- 浏览器实测（真实导出文件，走 `RenderStellarV2WithContext`）：宿主 `body` 背景保持 `rgb(11,11,11)`、内边距 24px 未被改写；iframe 内文档正常渲染（`#mA` 等节点存在）；高度回传 342px；无 pageerror。
- e2e：`tests/e2e/raw-html-insert.spec.ts` 10/10 通过，含新增「整页 HTML 文档走隔离 iframe，站点样式不被改写」（弹窗插入 → 保存 → 发布 → 文章页 iframe 内 `h1` 渲染、正文无可见 doctype、宿主背景不被 `rgb(1,2,3)` 改写、高度不停在初始 520px）。
- `make test` 全量门禁：见提交前的 merge-check 记录。

## 实测中发现

- 编辑器保存时**会丢掉小写 `<!doctype html>` 这一行**（markdown-it 的声明块规则只认 `<!` + 大写字母），`content_md` 里只剩 `<html>...`。这不影响隔离判定（`<html>` 开头同样命中），但 srcdoc 缺 doctype 会退化成怪异模式，因此在 `ensureStandardsMode` 里统一补一行 doctype。
- 编辑器侧的逐字无损仍只覆盖大写 doctype；小写 doctype 的 round-trip 缺口是既有行为，本次不改编辑器解析，记录在案。

## 边界与遗留

- 整页文档的脚本在 opaque origin 里运行：`localStorage`/`sessionStorage` 访问会抛 `SecurityError`（与编辑器预览的既有 sandbox 语义一致），本次不注入存储回退。
- 同一篇正文里多个整页文档各自独立成帧，互不影响。
