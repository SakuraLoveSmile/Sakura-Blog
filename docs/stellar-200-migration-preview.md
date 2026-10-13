# Stellar 2.0.0 存量迁移预检

状态：实现中。2026-10-13 读取根工作树 `data/blog.db`（SQLite mode=ro；补充核查 query_only）及 tracked content。仅统计结构、字段和数量，不输出密钥、个人资料或正文，不执行迁移。

本文件是第一轮可审查的影响预检，不是已经可执行的逐记录迁移补丁。用户已确认保留 Go 架构、完整实现等效能力；本地非上游功能保留为独立 Blog 扩展。下一步生成完整候选迁移及校验结果，先预览再执行。

## 数据规模

| 数据 | 当前原始行数 |
| --- | ---: |
| settings 段 | 29 |
| posts | 12 |
| pages | 4 |
| wiki_projects | 3 |
| wikidocs | 2 |
| topics | 7 |
| notes | 1 |
| native comments | 13 |
| users | 3 |
| moments | 5 |

行数包含数据库中所有状态，不等同公开发布数。`config/site.yml` 当前不存在，真实配置主要来自 DB；不能只迁移 `config/site.example.yml` 就声称真实站点迁移完成。

## 可直接进入候选迁移的结构映射

| 旧来源 | 上游目标 | 候选行为／验证 |
| --- | --- | --- |
| style | appearance | 配色、字体、字号、对齐、形状和背景逐字段映射；旧专用字段没有等价时报告，不整段透传 |
| plugins | features | 每个开关按上游实际扩展与参数迁移；不将旧 pjax 自动当成兼容局部导航 |
| site_tree | profiles | 将页面规则转换为全局/profile/collection/page 级联；空数组清空、省略继承 |
| menubar | leftbar.menu/menu_columns | 菜单标题、图标、链接及列数保持；上游 1–5 列语义生效 |
| search.service/local_search | search.provider/local/algolia | 显式 Provider、索引 scope/include_content/TTL；RAG 独立能力不能假装 Algolia |
| tag_plugins | tags | 按每个 renderer 参数迁移，并校验默认值变化 |
| 11 条 posts.topic | collection.profile/id | Topic 成员迁移到真实 collection 归属；7 个 Topic 定义与顺序一起检查 |
| 2 条 posts.pinned | listing.priority | 布尔 true 候选转 1，多级整数保持；按目标 schema 校验 |
| posts.ai_label（3 条非空） | article.ai_label | 校验四种语义 manual/reviewed/polished/generated，不猜未知值 |
| Wiki 项目 sort | listing.order | 两端排序方向一致，直接数值映射，避免反转 |
| 作者、references、license、share 等旧根字段 | article/footer 相应字段 | 来源与覆盖层级逐项检查，旧有空值和缺省值不随意填充 |

上游拒绝的旧根字段见 `scripts/schema/config-schema.js`；Collection/Front Matter 映射见 `scripts/schema/content-config-schema.js`，最终候选须通过真正 parser，而不是仅依据本表改名。

## 不能静默处理的真实冲突

| 冲突 | 当前影响 | 为什么需选择 |
| --- | --- | --- |
| 原生评论与第三方 Provider | 13 评论、3 用户 | 上游 Provider 模型不覆盖本地原生登录、上传、点赞和用户中心。新增第三方不能自动迁移身份或评论 |
| 旧 data_cache 配置 | 当前存在独立设置段 | 2.0 已移除该公开配置根，缓存策略由 runtime policy 承载；没有 features.data_cache 字段。旧自定义策略不能伪造字段迁移或静默删除 |
| 旧菜单 item.theme | 旧菜单结构可包含此字段 | 新菜单有 accent 等声明字段，需核实值语义并校验，不能直接透传未知 theme |
| 旧分享 wechat/link | article.share 已启用，当前含 wechat/weibo/email/link | 上游只含 qrcode/weibo/x/telegram/whatsapp/email。wechat 可候选转 qrcode；复制链接没有等价，不能静默删掉 |
| 旧 banner 图片 URL | 1 篇非空 banner | 上游 banner.background 是开关且图片来自 cover；旧 banner/cover 是独立图源。必须对照候选结果，不能把 URL 写入 bool 字段或覆盖 cover |
| 旧 poster | 3 篇非空 poster | headline/topic/caption/color 组合没有完全等价的 2.0 Front Matter。直接丢弃会改变内容展示 |
| 旧块级 tip | 4 次引用 | 本地实际也是 tooltip，可候选转 inline tip；正文 Markdown 与 pop 注解必须保留。不能可靠 tokenize 的内容列人工项 |
| 本地 moments/links 标签 | 页面中各有 1 次引用 | 不在上游标签注册全集，不能在严格规则下继续悄悄接受，亦不能删正文 |
| image 默认 fancybox | 本地默认 false，上游默认 true | 必须在候选中说明是否改变旧图片点击行为；若保留旧体验需显式参数 |
| 额外 about/ghrepo/ghuser 标签及多作者 | 本地已有实现，使用量仍需最终候选逐项核对 | 上游 ghcard 不是同一资料卡，article.author 是单字符串；不能无声替换组件或截断多作者 |

`tip` 候选例：`{% tip text:注解 %}正文{% endtip %}` → `{% tip 正文 pop:注解 %}`，仅是可解析形态示例，尚未改任何正文。

tracked 两份验收文章含旧 topic、poster_* 字段，也必须进入严格 schema 迁移；不能仅改真实 DB 或仅改 fixture。没有密码文章、定时发布、AI 摘要及草稿头的非空记录，不意味着可以删除这些现有产品能力。

## 执行门槛

1. 架构和没有上游等价的本地能力处理已经确认：Go 架构＋独立 Blog 扩展。逐记录不能无损确定的选择仍需在候选预览中确认。
2. 逐段配置、逐记录内容生成候选：旧来源→新目标、展示/行为变化、未映射值、诊断及处理结果；敏感值只保留在本地受保护的候选/备份中，不写 tracked 报告。
3. 在独立旧库副本和新库验证候选；不得覆盖根数据。验证幂等、失败不写版本标记、ID/关联/原 Markdown/草稿/评论归属不丢失。
4. 用户审查可执行的迁移预览后才执行真实迁移；备份、候选校验和失败回退路径必须已具备。
5. 迁移后通过真实后台编辑/重开、前台展示、搜索、路由、评论等结果核验。未迁移旧配置和内容显示可定位的上游规则诊断，不默默回退兼容。
