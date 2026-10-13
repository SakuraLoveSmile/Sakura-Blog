# Stellar 2 SSR 模板验证

锁定源：`xaoxuu/hexo-theme-stellar@2.0.0`，commit `9d11978a24fe6610e3a7dbab741cf62634539d05`。模板采用官方 `site-shell/site-region/brand/ui-collection` DOM，不另建视觉 token。

`layout/stellar` 消费请求内 `StellarRenderData`；`partial/stellar/region_body` 消费同一数据和 `Region`。宿主先渲染各区域，排除空正文，再装配 `Stellar.RegionBodies/RegionNames`。模板不从旧 `Site.Style/Plugins` 读取主题真值。

- `ListItems` 为完整官方 `{collection,item,render}` 或 `render.listing` 投影，必须包含 `href`；`Collections` 是官方 Wiki/Topic/Notebook listing 投影。
- `Data.index.terms` 用于分类/标签聚合，分类保留 `children/parentId`；归档与作者页从 `ListItems` 分组。旧宿主结构仅作为已有独立宿主数据的备用来源，不替代严格主题。
- `Data.tags` 必须是全部公开标签 `{name,path,count}`，不能预先截为旧 top20。标签云按 Hexo 先排序再截取、离散计数排名分字号，保留 RGB/HSLA/标准颜色名与 alpha 插值。
- `Data.pinnedItems` 为全部公开置顶 Post 模型；`allWikiItems/wikiTags/wikiFilter/wikiTagName` 用于 Wiki 索引。`paginationPath` 为当前聚合的稳定第一页 URL。
- `Data.blog` 仅为宿主筛选后的公开扩展：`content.share/reward`、`ai.summary/ask` 的开关与 BYOK 布尔值。模型提供摘要文本；配置密钥不进入 DOM。`Data.nativeComments` 提供 `title/admin_badge`，宿主提供准确 `CommentPageKey`。
- `Data.dateFormat` 来自宿主 `date_format`，默认 `YYYY-MM-DD`。日期保持输入 offset，宿主须在模板前处理站点时区。
- `Page.CanonicalURL` 为展示 URL，客户端/navigation 使用实际请求 URL。展示 URL 通过一次 URL resolve 构造，子目录不重复。按宿主约定，分页首页的 JSON-LD 使用其展示 URL；官方 Hexo helper 会强制首页 URL，此处是明确的 Go 宿主适配。

Go 检查不执行 Node，也没有新增生产 Node 或 CGO 依赖：

```sh
go test ./modules/stellar/web ./modules/stellar/i18n
go test -race ./modules/stellar/web ./modules/stellar/i18n
```

开发再生成采用隔离依赖 `ejs@3.1.10`、`hexo@8.0.0`、`hexo-util@4.0.0`、`moment@2.31.0`，使用 `NODE_PATH` 指向临时 npm 目录。三个生成器分别输出 `collection.json`（64 官方 DOM 组合，先验证上游源 SHA）、`tagcloud.json`（66 个颜色/排序/字号组合）和 `dates.json`（147 个三语言格式与跨年边界组合）。输出只是数据规则与 DOM 结构证据；真实路由、四种外观、第三方服务、浏览器交互及视觉验收由主集成任务继续核验。
