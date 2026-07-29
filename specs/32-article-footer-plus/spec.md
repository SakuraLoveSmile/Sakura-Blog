# Spec 32 · 文章页脚与文首增强（contributors / read_next / related / dateinfo / banner）

- **分支**：`feature/article-footer-plus` · **Wave 8** · **隔离等级**：中
- **主模块**：`internal/template/partial/content.html` + `assets/css/post-footer.css` + `cmd/server/routes_post.go`（数据装配）
- **前置**：无

## Why

差距调研发现文章页首尾五处与原版不齐：(1) **contributors/edit_this_page 段**——`assets/js/services/contributors.js` 已带但页脚未挂载；(2) **read_next**（下一篇导航，原版 `main/article/read_next.ejs`）与 (3) **related_posts**（相关文章推荐）整体缺失；(4) 文首 navbar 区 **dateinfo 创建/更新时间徽标** 未完整对齐；(5) **article_banner 文内横幅**（banner_ratio/auto_banner）缺失。

## 功能需求（对照原版 EJS）

| 项 | 原版参照 | 行为 |
|---|---|---|
| contributors | `article_footer.ejs` contributors 段 + `services/contributors.js` | front matter `references.issues`/repo 配置时渲染贡献者头像列表；`edit_this_page` 按钮按 site.yml 配置的路径映射生成编辑链接 |
| read_next | `read_next.ejs` | 文末「下一篇」卡片：同分类/时间序取邻篇，site.yml 可关 |
| related_posts | `related_posts.ejs` | 按共同标签数排序取 top N（缺省 5），排除自身与加密文章 |
| dateinfo | `article_title.ejs` dateinfo 段 | 文首创建/更新时间徽标（含"x 天前"相对时间与过期提醒文案，文案取原版 zh-CN.yml） |
| article_banner | `_partial/main/navbar/` banner 相关 | front matter `banner:` 时文首渲染横幅图，`banner_ratio` 可配 |

## What Changes

- `partial/content.html`：文首 +dateinfo/banner 段；页脚 license/share 之后 +contributors、+read_next、+related_posts 段（结构 class 与原版 EJS 逐一对齐）。
- `cmd/server/routes_post.go` + `internal/template/model.go`（或新 `model_article.go`）：装配 NextPost/RelatedPosts/Contributors 视图模型；related 查询加 `internal/store/post.go` 新读方法（新文件 `store/related.go`，不碰 post.go 主体）。
- `config/site.go` + `site.yml`：`article.read_next`（bool）、`article.related_posts`（0=关）、`article.edit_this_page` 映射表、`article.banner_ratio`。
- `assets/css/post-footer.css`：先 grep main.css，仅补缺失类。
- `partial/head.html`：文章页按需挂 contributors.js（有 references 配置才注入）。

## 实现步骤

1. 精读原版 article_footer.ejs / read_next.ejs / related_posts.ejs / article_title.ejs，记录 class 树。
2. dateinfo（纯模板 + 现有字段，最快见效）。
3. read_next（store 邻篇查询 + 模板段）。
4. related_posts（标签交集查询 + 模板段 + 上限配置）。
5. contributors + edit_this_page（js 挂载 + site.yml 映射）。
6. article_banner（front matter 解析 + 模板段）。
7. 配套 `internal/template/model_test.go` 追加视图模型用例；store 新查询加 `_test.go`。

## 技术决策（无需再问）

- **related 算法**：共同标签计数 desc + 发布时间 desc，SQL 单查实现，不引入向量/分词。
- **read_next 取邻**：同分类内按发布时间取下一篇，无同分类邻篇时回退全局时间序。
- **过期提醒阈值**：dateinfo 的"内容可能过期"文案阈值走 site.yml（缺省 180 天），对齐原版行为。
- **加密/隐藏文章**：read_next/related 一律排除 password 保护与未发布文章。

## Impact

- **新增**：`internal/store/related.go`（+test）、`model_article.go`（可选）
- **修改**：`partial/content.html`、`routes_post.go`、`model.go`（+字段）、`site.go`/`site.yml`（+段）、`post-footer.css`、`head.html`
- **不影响**：console、api、shortcode

## 验证

- [ ] 文首显示创建/更新时间徽标，>180 天旧文出现过期提醒
- [ ] 文末出现下一篇卡片与相关文章列表（样式与原版一致），加密文章不泄漏
- [ ] 配置 references 的文章显示贡献者段与「编辑本文」按钮
- [ ] front matter banner 生效且比例可配
- [ ] site.yml 关闭开关后各段消失
- [ ] `make test` 全绿
