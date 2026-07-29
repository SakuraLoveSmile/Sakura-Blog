# Spec 31 · 标签插件五期（展示类 11 件收官）

- **分支**：`feature/tag-plugins-5` · **Wave 8** · **隔离等级**：高
- **主模块（独占）**：`internal/post/shortcode.go` + `assets/css/tag-plugins.css`（追加式）
- **前置**：建议 Spec 30 先合（同文件追加，30 体量小先落地减少 rebase）

## Why

补齐原版剩余的展示类标签，本期后标签插件覆盖率达 42/42（mermaid 实现方式差异除外）：**about / albums / posters / banner / blockquote / frame / md / navbar / toc（文内目录标签）/ reel / paper**。全部为纯渲染类（无数据服务依赖），适合单分支一次做完。

## 功能需求（语法逐个对照原版 `scripts/tags/lib/*.js`）

| 插件 | 形态 | 说明 |
|---|---|---|
| about | 块级 `{% about [avatar:url] [height:x] %}...{% endabout %}` | 关于我卡片，body 走 renderMD |
| albums | 块级 | 相册集卡片流（原版配合 gallery 页面使用，本项目输出独立卡片组） |
| posters | 块级 | 海报流卡片（大图卡片组） |
| banner | 块级/行内 | 横幅大图 + 标题副标题（含 onclick 跳转参数） |
| blockquote | 块级 `{% blockquote [from] ... %}` | 引用出处排版 |
| frame | 行内 `{% frame [device] [img:x] [url:x] %}` | 手机/浏览器设备边框内嵌图或 iframe |
| md | 行内 `{% md url %}` | 拉取远端 md 渲染（本项目改为服务端拉取或限制为站内路径，见技术决策） |
| navbar | 块级 | 文内导航条（锚点/链接组） |
| toc | 行内 `{% toc [wiki\|blog] %}` | 文内目录标签（复用 `internal/template/toc.go` 已有 TOC 生成器） |
| reel | 块级 | 胶片卷轴横滑图组 |
| paper | 块级 | 纸张风格文本卡 |

## What Changes

- `internal/post/shortcode.go`：+11 正则与 render 函数，接入 `processShortcodes`（块级先、行内后，与现有顺序约定一致）。
- `assets/css/tag-plugins.css`：逐个 grep `main.css` 确认类是否已编译在内（大概率 about/blockquote/paper 等已存在），**仅补缺失**，对照原版 `source/css/_tag-plugins/` 同名 styl。
- toc 标签：调用 `internal/template/toc.go` 的现有解析（注意 shortcode 阶段在 goldmark 前，需决策为「渲染后二次替换」或「占位符 + 模板层回填」，对照 08/15 现有 TOC 管线选后者）。
- 测试文 `content/posts/tag-plugins-5-test.md` 覆盖全部 11 件。

## 实现步骤

1. 逐个精读 11 个原版 js，记录输出 HTML/class/参数表（半天，产出对照表贴 PR 描述）。
2. 纯容器类先行：about / blockquote / paper / banner / navbar / posters / albums / reel。
3. frame（设备边框，核对 main.css `.tag-plugin.frame` 系列类）。
4. md 标签（见技术决策的安全边界）。
5. toc 标签（占位符方案，模板层拿到 TOC 树后回填）。
6. `-seed` 重导 + 前台逐个目检与原版文档站截图比对。

## 技术决策（无需再问）

- **md 标签安全边界**：仅允许站内相对路径（`/` 开头）或白名单域（site.yml 可配 `md_allow_hosts`），杜绝 SSRF；拉取在服务端渲染期完成并走 bluemonday 过滤。
- **albums/posters 不建数据页**：原版这两个依赖 gallery 类页面数据，本项目实现为纯正文标签（参数内联图片列表），不新增页面类型。
- **toc 标签与右栏 TOC 共存**：文内 toc 渲染独立实例，不影响 rightbar 的 TOC widget。
- **语法容错**：与历期同标准，未闭合/参数缺失原样输出。

## Impact

- **新增**：测试文
- **修改**：`internal/post/shortcode.go`、`tag-plugins.css`、（md 白名单时）`config/site.go` + `site.yml` +1 key
- **不影响**：store、api、console

## 验证

- [ ] 11 个标签全部渲染且 class 结构与原版 EJS/文档站一致
- [ ] md 标签拒绝白名单外域名（返回原样文本）
- [ ] toc 标签在长文中正确生成并可锚点跳转
- [ ] 既有 29 个标签回归不破（tag-plugins-test 系列三篇重渲染目检）
- [ ] `make test` 全绿
