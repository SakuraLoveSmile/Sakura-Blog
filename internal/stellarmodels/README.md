# Stellar 2.0 内容与集合模型

本包将锁定 `xaoxuu/hexo-theme-stellar@2.0.0`（`9d11978a24fe6610e3a7dbab741cf62634539d05`）的 `scripts/lib/models/{shared,post,wiki,topic,notebook}.js` 转写为纯 Go。包括官方 collection、item、render 全部输出字段和分支，复用 `internal/stellarv2` 的区域、品牌与 Widget 规则；不维护另一套旧配置兼容层。上游版权及 MIT 授权保留于 `LICENSE.upstream`。

生产不调用 Node、Hexo、数据库、文件或网络。`Object`/`CollectionModel` 保留官方 JSON 形状，`PageModel{Collection,Item,Render}` 使用同名 JSON key。每次调用复制可变配置，页面或集合之间不共享输出状态。调用方可以读取输出并作模板适配，但不应修改集合模型后再跨请求复用。

## 输入与调用顺序

1. 宿主从 Settings 读取**迁移后的 canonical 主题**，通过 `stellarv2.ParseTheme` strict 模式得到 `Input.Theme`。`Theme` 必须是完整默认值合并后的 runtime camelCase 对象，不能传旧 `style/plugins/site_tree`、原始 snake_case YAML 或仅局部 theme override。
2. 宿主从 Post/Page/Topic/WikiProject/Notebook 的 canonical 内容配置解析 `FrontMatter`/`CollectionConfig`：先 strict `ParsePage`/`ParseCollection`，建立 Registry，再 `ResolveContentMembership`，检查 error/Issues；按 profile 调用 `ValidatePageProfile`/`ValidateCollectionProfile` 并记录 warnings。`BuildPage` 不替代校验与迁移版本门禁，显式集合 ID 不匹配、缺少集合、非法 collection brand 会报 `*ConfigError`。本包不会猜旧 `topic/wiki/notebook` 字段。
3. 宿主先按发布、私密、密码访问规则筛选公开记录，避免把未授权正文、标题和成员送入本包。`visibility.listed/searchable` 是上游呈现规则，**不是权限检查**。模型保留 searchable/listed，Topic 系列同时过滤成员 listed=false。
4. `Input.SiteConfig` 是 Go 宿主映射的 Hexo 站点配置：`title/url/author/avatar/email/language/keywords/index_generator.path/per_page`。这里的 `index_generator`、`per_page` 是站点宿主字段，保留上游名字；`Theme.canonical.host` 控制 canonical，`IsBackup` 必须映射实际备份构建状态。
5. `Input.RuntimeData` 来自公开的数据目录/作者/Widget 注册表：`authors`、`defaultAuthor`、`widgets`。`authors` 支持 license 中 `{author.name}/{author.url}`；`widgets` 是已公开的完整官方 catalog，不是旧 Go Widget 列表。
6. `PageModelInput(page, config)` 接受公开宿主 `Page` 与 strict `FrontMatter`，投影为序列化快照。原宿主字段包括 `_id/source/path/permalink/link/title/layout/content/excerpt/date/updated/tags/categories/prev/next/lang/language`，`content/excerpt` 已经过正文渲染与作者 HTML 净化。直接构造 `Input.Page` 时，使用快照名字 `previous/next/categoryLinks/tagLinks`；这些不能只从 Front Matter 的标签字符串反推。
7. `DiscoverContent(posts, pages)` 的 `Member` 使用 `Page/FrontMatter`，返回所有官方分类、快照、访问次数、profile/collection 索引。发现阶段只分类；归属错误由第 2 步阻止进入发现阶段。普通无 collection 页面不参与 Wiki/Notebook 发现。
8. 构建集合：`BuildPostCollection(theme)`、`BuildWikiCollection(input)`、`BuildTopicCollection(input)`、`BuildNotebookCollection(input)`。Wiki 的 `CollectionState` 必须由宿主文档树生成：`homepage.path`、`sections[].title`、`sections[].pages[]` 中 `_id/id/title/path/page_number/is_homepage`。Topic 的 `Members` 含公开完整成员快照和 FM，用于稳定日期排序。Notebook 的 `CollectionItems` 是 `{collection:{profile:"notebook",id},tags}`，用于所有层级标签导航。
9. `BuildPage(input)` 使用 FM `collection.profile` 调度上述四类；无 collection 对应官方 Post 核心模型。Notebook 必须显式提供 `CollectionID`。可以传已构建的 `CollectionModel`，但仍需原 `CollectionConfig` 与匹配 FM，以保留集合级 visibility、标签、原始配置规则。`RelatedItems` 是上游 Post 的已计算相关文章列表；`RelatedCollections/Related/Listing` 是 Wiki 两阶段聚合输出，后两者 nil 表示自动构造，显式空数组/空对象表示覆盖；`TagTree/RecentItems` 是 Notebook 两阶段聚合后的导航与最近条目。
10. `BuildTopicIndexRender`、`BuildWikiListingRender`、`BuildWikiRelated` 提供集合索引与关联卡片。`SelectListingItems` 按 listed/tag.itemIds 选择并接受宿主 adapter comparator，保持稳定排序；`PaginateItems` 保留空集合至少一页的官方行为。普通 home/index/page/404 等模板仍由宿主依据 Theme profiles 生成，官方 models/index 没有这些独立模型。

`BuildHero(HeroInput)` 返回与 Wiki `render.cover` 完全相同的源模型，仅集合首页且 hero.enabled=true 时启用。完整保留 image/video/parallax、四种 effect 的 options/runtime/presentation、图片预览、terminal commands 和 actions；video 与 effect 同时存在时，**模板依据官方规则优先展示 video**。`HeroBackground` 投影四 effect 的背景色，不自行补 effect options 默认值（这些属于客户端 effect runtime）。

`ReadmeHTML`、contributor、repositoryApi、releaseApi 只生成官方服务源/占位符。网络抓取、取消、缓存、SSRF 防护和公开数据权限由宿主服务/浏览器扩展负责。README 保留 `.data-service.ds-mdrender` 官方 HTML，宿主可加 `data-blog-service` 标记并使用独立 Blog 生命周期封装，不能删掉其 replace/base/heading 语义。

`ResolveComments` 保留 6 个第三方 provider 的资源拆分、级联、Artalk server 派生、Giscus 手动配色覆盖；原生 Blog 评论作为独立扩展，不能塞入严格 provider。`FilterShareServices/BuildShareAction` 只处理 6 个官方服务。`ResolveAILabel/BuildAILabelHTML` 保留官方 4 个标签；HTML helper 的 `iconHTML/extraClass` 必须来自可信图标/模板，用户文本经过官方等效转义。

## 验证与再生成

`testdata/official-oracle.json.gz` 的预期值由锁定官方 Node 函数生成，包含源文件 SHA-256，Go 测试不执行 Node。当前 780 条完整 JSON 差分覆盖四 profile、配色、默认与覆盖、null/false/空列表、日期、SEO、Hero/terminal、评论、分享、AI、发现/快照/分页、英文/中文/重音/标点层级标签及异常归属/品牌配置。额外并发测试验证独立输入输出和 race。`comments-defaults.json/model-fields.json` 由同一生成器提取官方默认值/字段，避免手写第二套白名单。

开发再生成只在临时开发依赖目录需要 Node 22+、hexo-util 4、js-yaml 4、cheerio 1.1；不要改生产依赖或 tracked JS 锁文件：

```sh
NODE_PATH="$PWD/.tmp/models-oracle/node_modules" node internal/stellarmodels/generate/oracle.cjs /absolute/path/to/locked/hexo-theme-stellar
go test ./internal/stellarmodels
go test -race ./internal/stellarmodels
```

生成器先比对 `generate/source-fingerprints.json` 中与完整上游 inventory 一致的 180 个相关源文件指纹，可使用未修改的 Git archive 导出，不以 HEAD 是否存在判断源码版本。

这些验证证明纯数据规则，不代表 SSR 已接入、浏览器第三方 provider 已运行、全站布局已验收或真实数据已迁移。
