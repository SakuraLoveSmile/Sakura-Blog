# Spec 68 · Stellar 1.40.0 同步

- **基线**：`resource/hexo-theme-stellar` 已切换到 `1.40.0`；CSS 由同一份 Stylus 源重编。
- **分支**：dev 直接推进，作为任务 3 的独立中文提交。
- **目标**：跟随上游 1.40.0 的默认外观、置顶轮播、阅读信息、标签插件和 Wiki README 首页行为；保留本项目 Go/SQLite 数据模型与自建评论系统边界。

## 完成项

- [x] T0/T1：同步 1.40 配置形状、默认圆角/`corner-shape`、左栏 card、头像渐变、文章 pin/reading/card-tags 配置链路，重编 `main.css`。
- [x] T2：分页器 SVG、左栏 card、头像渐变、navbar、文章卡片标签与阅读信息挂载点完成模板对齐；`linklist`、`markdown`、`toc` 沿用已有结构化 widget 实现。
- [x] T3：文章/Wiki 置顶轮播接入首页、归档、分类、标签和 Wiki；flat 模式回退列表置顶；首页第二页及以后不重复渲染轮播。
- [x] T4：新增 `{% table style:scroll|wrap|compact %}`、`{% tip text:... %}...{% endtip %}`；`{% md wrap:false %}` 输出 `data-replace`。
- [x] T5：`mdrender.js` 支持原地替换、远程 Markdown 标题锚点和 GitHub raw 相对资源；Memos v1 不移植为死服务，当前项目没有 `ds-memos` 配置/模板入口；Artalk/Waline 继续由自建评论系统边界排除。
- [x] T6：中文按字符、英文/数字按词统计阅读信息；卡片标签去重保序并由配置控制，最多展示 5 项。
- [x] T7：Wiki 文档树保留最多三级递归；timeline 节点标题保持字面文本，不解析 Markdown；Wiki 正文为空时 SEO 描述回退项目描述。
- [x] T8：本文档记录同步边界与验证命令；资源版本由工作区外部主题源固定为 `1.40.0`。

## Wiki README

配置 `wiki[].repo` 后，项目第一篇已发布文档被视为首页。首页正文为空时输出 `raw.githubusercontent.com/<repo>/<branch>/README.md` 的 `ds-mdrender` 占位，`branch` 缺省使用 `HEAD`，由浏览器按需加载；服务端不抓取外部内容，也不会影响非首页或已有正文的文档。

当前 Wiki 项目模型只保存文章分类字符串，不存在 Hexo category 对象的 `parent` 关系，因此不伪造 `category_tree.js` 的层级数据；Wiki 文档树的真实父子关系由 `wikidocs.parent` 继续递归装配。

## 验证

- `make css`
- `make css-drift-check`
- `make build`
- `make test`
- `make pre-commit-check`

Go 定向回归覆盖 shortcode、reading stats、card tags、timeline 标题、Wiki README 占位与 SEO fallback；前端 `mdrender.js` 通过 `node --check`。
