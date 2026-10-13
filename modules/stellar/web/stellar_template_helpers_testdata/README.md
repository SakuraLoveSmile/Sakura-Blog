# Stellar 2 SSR 模板验证

锁定源：`xaoxuu/hexo-theme-stellar@2.0.0`，commit `9d11978a24fe6610e3a7dbab741cf62634539d05`。模板采用官方 `site-shell/site-region/brand/ui-collection` DOM，不另建视觉 token。

`layout/stellar` 消费请求内 `StellarRenderData`；`partial/stellar/region_body` 消费同一数据和 `Region`。宿主先渲染各区域，排除空正文，再装配 `Stellar.RegionBodies/RegionNames`。模板不从旧 `Site.Style/Plugins` 读取主题真值。

- `ListItems` 为完整官方 `{collection,item,render}` 或 `render.listing` 投影，必须包含 `href`；`Collections` 是官方 Wiki/Topic/Notebook listing 投影。
- `Data.index.terms` 用于分类/标签聚合，分类保留 `children/parentId`；归档与作者页从 `ListItems` 分组。旧宿主结构仅作为已有独立宿主数据的备用来源，不替代严格主题。
- `Data.tags` 必须是全部公开标签 `{name,path,count}`，不能预先截为旧 top20。标签云按 Hexo 先排序再截取、离散计数排名分字号，保留 RGB/HSLA/标准颜色名与 alpha 插值。
- `Data.pinnedItems` 为全部公开置顶 Post 模型；`allWikiItems/wikiTags/wikiFilter/wikiTagName` 用于 Wiki 索引。`paginationPath` 为当前聚合的稳定第一页 URL。
- `Data.blog` 仅为宿主筛选后的公开扩展：`content.share/reward`、`ai.summary/ask` 的开关与 BYOK 布尔值。全局内容扩展的公开投影固定为 `enabled/avatar_background/rightbar_intro/welcome_linklist/nav_tabs/ai_label/category_icons/tag_icons/read_next/expire_days/banner_ratio/edit_this_page/notebook.license/notebook.share`；嵌套对象也按真实消费者筛选，未知密钥不进入公开 Data。模型提供摘要文本；配置密钥不进入 DOM。`Data.nativeComments` 提供 `title/admin_badge`，宿主提供准确 `CommentPageKey`。
- `Data.dateFormat` 来自宿主 `date_format`，默认 `YYYY-MM-DD`。日期保持输入 offset，宿主须在模板前处理站点时区。
- `Page.CanonicalURL` 为展示 URL，客户端/navigation 使用实际请求 URL。展示 URL 通过一次 URL resolve 构造，子目录不重复。按宿主约定，分页首页的 JSON-LD 使用其展示 URL；官方 Hexo helper 会强制首页 URL，此处是明确的 Go 宿主适配。

Go 检查不执行 Node，也没有新增生产 Node 或 CGO 依赖：

```sh
go test ./modules/stellar/web ./modules/stellar/i18n
go test -race ./modules/stellar/web ./modules/stellar/i18n
```

开发再生成采用隔离依赖 `ejs@3.1.10`、`hexo@8.0.0`、`hexo-util@4.0.0`、`moment@2.31.0`，使用 `NODE_PATH` 指向临时 npm 目录。三个生成器分别输出 `collection.json`（64 官方 DOM 组合，先验证上游源 SHA）、`tagcloud.json`（66 个颜色/排序/字号组合）和 `dates.json`（147 个三语言格式与跨年边界组合）。输出只是数据规则与 DOM 结构证据；真实路由、四种外观、第三方服务、浏览器交互及视觉验收由主集成任务继续核验。

全局 Blog 扩展遵循独立规则：welcome intro/linklist 只作用于命名 welcome 组件，显式组件参数优先；nav_tabs 支持替换或追加，保留官方 listing-nav DOM。read_next=false 只控制文章/Topic，Wiki 导航保留。AI default/色彩/图标只覆盖四个官方标签，图片横幅仍使用自适应文字。banner_ratio 只覆盖详情横幅，category/tag 图标用于列表、索引与详情。expire_days 只用于文章/Topic，edit_this_page 只在官方 contributor 未命中时按最长来源前缀匹配。

Notebook license/share 作为仅 Note 的扩展默认值补入当前渲染副本，不改变保存原文；显式 Page/Collection footer 优先，null 保留官方继承语义，false 关闭。分享显式关闭同时关闭独立 Blog 分享按钮。SEO 的独立 no_index/default_description/default_keywords/verification 字段由宿主 typed bridge 提供；页面显式元信息优先，站点缺省次之。启用 OG 时，普通 description 与分享元信息按官方 head.ejs 抑制及 Hexo helper 回退链输出；独立 default_description 继续消费于 JSON-LD。canonical 和 JSON-LD 保留宿主展示 URL，显式 OG URL 只影响社交元信息。输出可由 partial navigation 替换。

`generate-opengraph.cjs` 通过 `git archive` 读取上述固定 commit 的 head.ejs 和模型，调用真正的 `hexo@8.0.0` open_graph helper。外层 strip_html 使用 `hexo-util@4.0.0`，Hexo helper 自身解析到的嵌套依赖为 `hexo-util@3.3.0`；产物记录两者版本及 head/helper SHA256。开发再生成：

```sh
NODE_PATH=/absolute/path/to/oracle/node_modules node modules/stellar/web/stellar_template_helpers_testdata/generate-opengraph.cjs /absolute/path/to/resource/hexo-theme-stellar
```

`opengraph.json` 包含四种 profile 的 96 个参数组合：76 个正常组合分别验证真实 embed HTTP SSR、离线快照和 opaque Markdown preview（228 个输出），另 20 个非法 URL／基址／图片／Twitter 图片／账号类型组合记录官方 TypeError（60 个错误边界），并验证 Go 的三个入口明确失败。它覆盖作者、Facebook、Twitter、图片别名与数组、相对路径解析、语言、日期、查询参数、HTML／实体描述与标题、null／空值回退。验证的是实际元信息 DOM 值；错误通过模板 helper 正式传播，不静默丢弃图片。
