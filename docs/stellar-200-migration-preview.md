# Stellar 2.0.0 存量迁移审查

状态：实现中。T1 完整迁移预览已按本次实测消费者重新生成，仍等待真实数据选择与审查批准。真实迁移未执行，尚无真实源 Apply 入口；隔离执行证明只接受新建系统临时 root。用户已批准严格 V2 编辑器统一一个“集合显示名（name）”，不等于批准旧记录 name/title 的取值。

## 本次只读证据与私密边界

捕获时间：`2026-10-13T14:26:47Z`。真实来源为根工作树 `Blog` / `dev` / `02ee112` 的 `data/blog.db` 及两份 tracked 文章。`ReadSnapshot` 使用 SQLite `mode=ro`、`query_only=1` 和只读事务；未 bootstrap、未 seed、未改 schema、未改真实配置/正文/上传目录，未删除侧车，未使用 immutable 绕过活跃 WAL。根 `.pi/` 与 `docs/plans/maintenance/` 保留。

- 本次 SourceDigest：`d8ae856807f1edde4ade0ac58554d045645fb27bc9ef5edef10ff2084badedb9`；快照格式为 **3**，显式覆盖 typed BLOB、隐藏 rowid/无 rowid 形状和逐值 SQLite storage class。
- 历史 SourceDigest：`6c7983b1b42d43b0456e786c3c98ca61fc16a84c2acabe76893e1febf39e8293`。算法与快照内容编码已升级，两个摘要不能直接比较，也不能据差异声称真实源变化。
- 主 DB SHA256：`40cd42cb5c704b457be8b6761d226c53ee14896211584dd25f0f9287d2454f0a`，本次读取前后及历史 review-6/authors-1 一致；WAL SHA、全部源表计数和两份 tracked 文件 hash 也与历史一致，独立证明本次持久化源未变化。
- 本轮 DB/WAL/SHM/journal 前后指纹相同，这是本次观察事实；`mode=ro` 不保证整个目录零写，SQLite 锁侧车仍可能产生或变化。未删除侧车，也不使用 immutable。
- `content/notes.md` 是系统 `/notes` 首次启动正文来源，没有文章 Front Matter；其 hash 前后相同、文件保留，不伪造为第三条文章。当前 `/notes` 内容以 DB pages 候选审查。

初次格式 3 捕获位于 `.tmp/stellar-migration-review-20261013-final-format3/`；日期消费校验修复后，从**同一个已校验快照**重新生成于 `.tmp/stellar-migration-review-20261013-final-format3-validated/`，没有再次读取或转换真实源。两目录均 `0700`，五份 candidate/report/source-check/notebook-structure 文件均 `0600`，旧候选未覆盖。完整候选含密钥、全文和私密 alternatives，禁止提交、外发或输出；本文仅列路径、hash、脱敏行为和决策标签。

本次：**32 条候选（30 DB + 2 tracked）、172 项配置映射、766 项逐记录映射、12 条阻断**。分组：consumer 0、policy 5、structure 4、schema 3。原 171 项配置映射全部保留；第 172 项是未启用首页 cover 的源归档说明，不能删减以凑旧数量。

## 集中确认的真实数据选择

以下是实际未决数据选择；尚未将任何选择写入候选。消费者已实现的能力不再要求用户重复决定。全部政策/schema/结构诊断仍保留，capabilities 不能解除。

| 组 | 涉及来源与诊断 | 候选与实际变化 | 建议 / 需明确的批准 |
| --- | --- | --- | --- |
| 1. 官方导航策略 | settings.plugins.pjax · pjax_policy_removed | 接受官方 manifest/shell-signature/同集合 partial navigation；旧 selectors/timeout 只留私密源归档。02ee112 的 typed PluginsConfig、脚本注册和旧 base/loader 不消费 PJAX 存储参数，不能描述成正在失去旧超时体验。 | 推荐接受官方策略（use_official）。新导航已按本次隔离浏览器证据闭合；本项仍需具名批准，不能全局豁免其它来源。 |
| 2. 一个 Wiki 显示名 | settings.wiki[0].name · wiki_shelf_definition_conflict；wiki_projects/go-blog · wiki_name_title | 最终 name 在项目旧 name、项目旧 title、旧 shelf 显示名三种来源中确定一个。内部 ID、成员、本地文档路由保持。两条诊断必须给出一致决定，不能在同一字段写互相冲突的值。 | 建议采用旧卡片主标题 title 为唯一显示 name，并使 shelf 仅保留项目 ID；具体旧值只在本地私密候选审查。编辑器单 name 决定已确认，不重复询问。 |
| 3. Wiki 集合入口 | wiki_projects/go-blog · wiki_homepage | 采用官方 Collection 首页；或保留独立 Blog 旧入口跳第一篇已发布文档。两候选保留成员和顺序，本地 canonical/doc route 不被外链覆盖。 | 推荐采用官方 Collection 首页。旧重定向候选只供完整差异审查，若选择保留须完成对应独立消费验证，不先假称完成。 |
| 4. banner/cover 双图源 | posts/10 · banner_cover | 候选 A：旧 cover 继续用于列表，blog.content.banner_image 仅覆盖详情。候选 B：旧 banner 成为官方 cover，同时改变列表图。 | 推荐候选 A“保留独立 Blog banner 图源”。该独立消费者已有本次实测，真实记录的取值仍需具名批准。 |
| 5. 三个 Footer 占位目标 | leftbar.footer.actions[1..3].url · invalid_value | 只移除这三条 url="#" 按钮；或分别提供三个 safe navigable 外链/站内根相对路径。其它按钮、顺序和配置保持，不自动改目标。 | 推荐仅移除三条占位按钮；如需保留，请提供逐项有效目标。 |
| 6. Notebook/Note 结构与五个元数据 | schema.notebooks/notes · legacy_notebook_schema；notes/2 · note_missing_slug_status | 批准 ID/父本转 TEXT、nb_order→sort、content→content_md、tags→tags_json；Note INTEGER ID、所有关系、原日期/正文/标签保留。缺字段为 notebooks/2.author_id、updated_at，以及 notes/2.slug、status、author_id。 | 推荐 Note slug 为 note-2（可自定）、先 draft；明确 Notebook/Note 的现有拥有者 ID，Notebook updated_at 可明确沿用其原 created_at。不按时间或全部用户猜 owner，不自动发布。 |

结构预览有 21 项确定映射、5 个未批准元数据；原三条 Go time.String 形式的 TEXT 日期现已按正式 modernc.org/sqlite 既有格式通过严格消费校验，原字符串保留，不需要用户选择新日期。无效日期、非字符串及未知值仍阻断。

两份 tracked 文章各有一条 `file_host_fields` 结构门禁：源 status 已映射到独立 `blog.status`，不进入上游 schema、不隐式发布。这是已确认宿主契约的执行验证/逐记录 `preserve_status` 批准项，**不新增数据取值选择**。正式导入、重开与状态检查及完整批准摘要须在执行前闭合；原诊断保留，不用 consumer 能力参数掩盖。

### 私密候选的可审查分支

下列标签及摘要说明对应私密候选中的完整转换结果；不公开原值或正文。没有自动应用任何分支。

| 来源 | Code | 分支标签 | 作用来源 | strict 候选 | Blog 候选 | 正文候选 |
| --- | --- | --- | --- | --- | --- | --- |
| settings | wiki_shelf_definition_conflict | 保留当前项目表集合定义 | wiki_projects/go-blog | 是 | 是 | 否 |
| settings | wiki_shelf_definition_conflict | 审核旧 shelf 字段覆盖（无等效字段需单独适配） | wiki_projects/go-blog | 是 | 是 | 否 |
| wiki_projects/go-blog | wiki_name_title | 保留旧 name 为 name | wiki_projects/go-blog | 是 | 否 | 否 |
| wiki_projects/go-blog | wiki_name_title | 保留旧 title 为显示 name | wiki_projects/go-blog | 是 | 否 | 否 |
| wiki_projects/go-blog | wiki_homepage | 独立 Blog 扩展保留旧重定向 | wiki_projects/go-blog | 是 | 是 | 否 |
| wiki_projects/go-blog | wiki_homepage | 采用官方 Collection 首页 | wiki_projects/go-blog | 是 | 是 | 否 |
| posts/10 | banner_cover | 保留独立 Blog banner 图源 | posts/10 | 是 | 是 | 否 |
| posts/10 | banner_cover | 用旧 banner 作为官方 cover | posts/10 | 是 | 是 | 否 |
| notes/2 | note_missing_slug_status | 保留旧 ID 与当前未公开状态 | notes/2 | 是 | 是 | 否 |
| notes/2 | note_missing_slug_status | 另行确认 slug/状态/ownership 迁移 | notes/2 | 是 | 是 | 否 |

## 本次消费者声明与证据

主 agent 已明确声明全部 20 个 catalog 能力实现且本次实测，因此本预览将相应 34 条原 consumer 诊断保留为 info/nonblocking，consumer 阻断为 0。没有删除诊断。证据清单及文件 hash 见 [stellar-200-migration-capabilities.json](stellar-200-migration-capabilities.json)。这些是隔离 fixture 的本次实际浏览器/HTTP 证据，不代表真实迁移或用户整体体验确认。

| 能力 | 本源 info 诊断数 | 本次证据 |
| --- | --- | --- |
| ai_label | 1 | docs/baselines/stellar-200-full/resume-global-consumers.json |
| author_widget | 1 | docs/baselines/stellar-200-full/resume-global-consumers.json<br>docs/baselines/stellar-200-full/after-resume-author-widget.png |
| authors | 0 | docs/baselines/stellar-200-full/resume-post-authors.json<br>docs/baselines/stellar-200-full/resume-page-author.json |
| avatar_background | 1 | docs/baselines/stellar-200-full/resume-global-consumers.json |
| banner_ratio | 1 | docs/baselines/stellar-200-full/resume-global-consumers.json |
| category_icons | 1 | docs/baselines/stellar-200-full/resume-global-consumers.json |
| collection_order | 7 | docs/baselines/stellar-200-full/resume-pages-seo-wiki.json |
| edit_this_page | 1 | docs/baselines/stellar-200-full/resume-global-consumers.json |
| expire_days | 1 | docs/baselines/stellar-200-full/resume-global-consumers.json |
| nav_tabs | 1 | docs/baselines/stellar-200-full/resume-global-consumers.json |
| notebook_license | 1 | docs/baselines/stellar-200-full/resume-global-consumers.json |
| notebook_share | 1 | docs/baselines/stellar-200-full/resume-global-consumers.json |
| pages | 1 | docs/baselines/stellar-200-full/resume-pages-seo-wiki.json |
| poster | 11 | docs/baselines/stellar-200-full/resume-global-consumers.json |
| read_next | 1 | docs/baselines/stellar-200-full/resume-global-consumers.json |
| rightbar_intro | 1 | docs/baselines/stellar-200-full/resume-global-consumers.json |
| seo | 1 | docs/baselines/stellar-200-full/resume-pages-seo-wiki.json<br>docs/baselines/stellar-200-full/after-resume-page-explicit-seo.png |
| tag_icons | 1 | docs/baselines/stellar-200-full/resume-global-consumers.json |
| welcome_linklist | 1 | docs/baselines/stellar-200-full/resume-global-consumers.json |
| wiki_external_url | 0 | docs/baselines/stellar-200-full/resume-pages-seo-wiki.json |

SEO 最后闭合提交为 aa1fa93：显式 about description 优先、og:image 使用 cover；上游强制 og:title 保留 page.title。robots_extra 以实际 HTTP CLI 检查通过，浏览器客户端阻止 robots.txt，不能称浏览器请求已通过。公开作者是 catalog ID 的字符串数组；公开显示作者不授予编辑权限。

## 已确认无需新增选择的等效项

- 旧 OKR 完整默认对象与固定官方 border/五色/zh-CN 文案相同；保留原值，继承官方与请求 locale。
- data_cache 完整对象与固定官方 runtime policy 全树一致（enable/default_ttl/max_entries 和九项 TTL）；保留原值，无策略差异，自定义/null/缺键/未知值继续阻断。
- 首页 cover 仅已知 title/description 且 title 为空，旧首页模板不显示它；description 只留源归档，不造 Hero。非空 title、额外字段或坏类型不豁免。
- Wiki 编辑器统一 name 已由用户确认；旧值取哪个来源仍按第 2 组审查。

## 完整源表计数

覆盖所有状态及非迁移业务表，不等于公开内容数量。主 DB/WAL/全部行数与历史一致；快照格式 3 还使仅 rowid 或 storage class 改动也导致批准过期。

| 表 / 文件集合 | 行数 |
| --- | --- |
| attachment_groups | 0 |
| attachments | 1 |
| categories | 6 |
| comment_boosts | 1 |
| comment_likes | 1 |
| comments | 13 |
| crawler_logs | 0 |
| feed_articles | 45 |
| links | 3 |
| login_logs | 110 |
| module_states | 1 |
| moments | 5 |
| notebooks | 1 |
| notes | 1 |
| notifications | 6 |
| page_views | 725 |
| pages | 4 |
| pat_tokens | 0 |
| perf_requests | 1418 |
| perf_samples | 74564 |
| permissions | 19 |
| post_authors | 0 |
| post_authors_legacy | 0 |
| post_categories | 11 |
| posts | 12 |
| posts_fts | 18 |
| posts_fts_config | 1 |
| posts_fts_content | 18 |
| posts_fts_data | 32 |
| posts_fts_docsize | 18 |
| posts_fts_idx | 30 |
| rag_chunks | 27 |
| rag_chunks_fts | 27 |
| rag_chunks_fts_config | 1 |
| rag_chunks_fts_content | 27 |
| rag_chunks_fts_data | 11 |
| rag_chunks_fts_docsize | 27 |
| rag_chunks_fts_idx | 9 |
| rag_meta | 1 |
| ratings | 0 |
| revisions | 0 |
| role_permissions | 13 |
| roles | 3 |
| schema_migrations | 1 |
| seo_push_logs | 0 |
| settings | 29 |
| shared_posts | 0 |
| sqlite_sequence | 21 |
| topics | 7 |
| tracked_documents | 2 |
| user_preferences | 1 |
| user_roles | 3 |
| users | 3 |
| votes | 1 |
| wiki_project_tags | 5 |
| wiki_projects | 3 |
| wikidocs | 2 |

## 完整配置映射（172 项）

完整保留旧 171 项并增加 cover 无显示差异说明。仅列路径和转换行为；宿主、密钥对应目标不表示原值进入公开 theme。

| 编号 | 来源 | 目标 | 行为 |
| --- | --- | --- | --- |
| 1 | settings.admin | settings.admin | 宿主设置原文保留，无主题转换 |
| 2 | settings.dashboard | settings.dashboard | 宿主设置原文保留，无主题转换 |
| 3 | settings.page_banner_friends_legacy_migrated | settings.page_banner_friends_legacy_migrated | 宿主设置原文保留，无主题转换 |
| 4 | settings.seo_tools | settings.seo_tools | 宿主设置原文保留，无主题转换 |
| 5 | settings.wiki_projects_migrated | settings.wiki_projects_migrated | 宿主设置原文保留，无主题转换 |
| 6 | settings.site.title | blog.site.title | 原值保留（包含 false/null/空列表） |
| 7 | settings.site.subtitle | blog.site.subtitle | 原值保留（包含 false/null/空列表） |
| 8 | settings.site.description | blog.site.description | 原值保留（包含 false/null/空列表） |
| 9 | settings.site.url | blog.site.url | 原值保留（包含 false/null/空列表） |
| 10 | settings.site.logo_title | theme.leftbar.brand.name | 原值保留（包含 false/null/空列表） |
| 11 | settings.site.logo_sub | theme.leftbar.brand.tagline | 原值保留（包含 false/null/空列表） |
| 12 | settings.site.avatar | theme.leftbar.brand.image.src | 原值保留（包含 false/null/空列表） |
| 13 | settings.site.avatar_bg | blog.content.avatar_background | 原值保留（包含 false/null/空列表） |
| 14 | settings.site.favicon_url | blog.site.favicon_url | 原值保留（包含 false/null/空列表） |
| 15 | settings.site.apple_touch_icon | blog.site.apple_touch_icon | 原值保留（包含 false/null/空列表） |
| 16 | settings.site.sw_cache_version | blog.site.sw_cache_version | 原值保留（包含 false/null/空列表） |
| 17 | settings.menubar.columns | theme.leftbar.menu_columns | 原值保留（包含 false/null/空列表） |
| 18 | settings.rightbar.intro | blog.content.rightbar_intro | 原值保留（包含 false/null/空列表） |
| 19 | settings.rightbar.welcome_linklist | blog.content.welcome_linklist | 原值保留（包含 false/null/空列表） |
| 20 | settings.comments | blog.native_comments | 原值保留（包含 false/null/空列表） |
| 21 | settings.authors | blog.data.authors | 原值保留（包含 false/null/空列表） |
| 22 | settings.friends | blog.friends | 原值保留（包含 false/null/空列表） |
| 23 | settings.pages | blog.pages | 原值保留（包含 false/null/空列表） |
| 24 | settings.nav_tabs | blog.content.nav_tabs | 原值保留（包含 false/null/空列表） |
| 25 | settings.article.reward | blog.content.reward | 原值保留（包含 false/null/空列表） |
| 26 | settings.article.ai_label | blog.content.ai_label | 原值保留（包含 false/null/空列表） |
| 27 | settings.article.category_icon | blog.content.category_icons | 原值保留（包含 false/null/空列表） |
| 28 | settings.article.tag_icon | blog.content.tag_icons | 原值保留（包含 false/null/空列表） |
| 29 | settings.article.read_next | blog.content.read_next | 原值保留（包含 false/null/空列表） |
| 30 | settings.article.expire_days | blog.content.expire_days | 原值保留（包含 false/null/空列表） |
| 31 | settings.article.banner_ratio | blog.content.banner_ratio | 原值保留（包含 false/null/空列表） |
| 32 | settings.article.edit_this_page | blog.content.edit_this_page | 原值保留（包含 false/null/空列表） |
| 33 | settings.article.reading_time | theme.article.show_reading_time | 原值保留（包含 false/null/空列表） |
| 34 | settings.article.card_tags | theme.article.listing.show_tags | 原值保留（包含 false/null/空列表） |
| 35 | settings.article.related_posts | theme.article.related_posts_limit | 原值保留（包含 false/null/空列表） |
| 36 | settings.article.category_color | theme.article.category_colors | 原值保留（包含 false/null/空列表） |
| 37 | settings.plugins.preload.enable | theme.features.link_prefetch.enabled | 原值保留（包含 false/null/空列表） |
| 38 | settings.plugins.heti.enable | theme.features.heti.enabled | 原值保留（包含 false/null/空列表） |
| 39 | settings.plugins.fancybox.selector | theme.features.lightbox.selector | 原值保留（包含 false/null/空列表） |
| 40 | settings.plugins.scrollreveal.enable | theme.features.reveal.enabled | 原值保留（包含 false/null/空列表） |
| 41 | settings.plugins.scrollreveal.duration | theme.features.reveal.duration | 原值保留（包含 false/null/空列表） |
| 42 | settings.plugins.scrollreveal.interval | theme.features.reveal.interval | 原值保留（包含 false/null/空列表） |
| 43 | settings.inject.inject_head | theme.inject.head_end | 原值保留（包含 false/null/空列表） |
| 44 | settings.inject.inject_footer | theme.inject.body_end | 原值保留（包含 false/null/空列表） |
| 45 | settings.ai.base_url | blog.ai.base_url | 原值保留（包含 false/null/空列表） |
| 46 | settings.ai.api_key | blog.ai.api_key | 原值保留（包含 false/null/空列表） |
| 47 | settings.ai.model | blog.ai.model | 原值保留（包含 false/null/空列表） |
| 48 | settings.ai.embedding_model | blog.ai.embedding_model | 原值保留（包含 false/null/空列表） |
| 49 | settings.ai.summary_enabled | blog.ai.summary.enabled | 原值保留（包含 false/null/空列表） |
| 50 | settings.ai.allow_user_api | blog.ai.summary.allow_user_api | 原值保留（包含 false/null/空列表） |
| 51 | settings.ai.qa_enabled | blog.ai.ask.enabled | 原值保留（包含 false/null/空列表） |
| 52 | settings.ai.qa_allow_user_llm | blog.ai.ask.allow_user_llm | 原值保留（包含 false/null/空列表） |
| 53 | settings.notebook.auto_excerpt | theme.profiles.notebook.listing.excerpt_length | 原值保留（包含 false/null/空列表） |
| 54 | settings.notebook.license | blog.content.notebook.license | 原值保留（包含 false/null/空列表） |
| 55 | settings.notebook.share | blog.content.notebook.share | 原值保留（包含 false/null/空列表） |
| 56 | settings.article.type | theme.article.style | 旧缺省占位保留在源快照；候选继承上游默认 |
| 57 | settings.article.pin_style | theme.article.listing.pinned_layout | 旧缺省占位保留在源快照；候选继承上游默认 |
| 58 | settings.style.prefers_theme | theme.appearance.color_scheme | 旧缺省占位保留在源快照；候选继承上游默认 |
| 59 | settings.style.text_align | theme.appearance.typography.content_align | 旧缺省占位保留在源快照；候选继承上游默认 |
| 60 | settings.style.color.theme | theme.appearance.colors.primary | 旧缺省占位保留在源快照；候选继承上游默认 |
| 61 | settings.style.color.accent | theme.appearance.colors.accent | 旧缺省占位保留在源快照；候选继承上游默认 |
| 62 | settings.style.color.link | theme.appearance.colors.link | 旧缺省占位保留在源快照；候选继承上游默认 |
| 63 | settings.style.font_size.root | theme.appearance.typography.font_size.root | 旧缺省占位保留在源快照；候选继承上游默认 |
| 64 | settings.style.font_size.code | theme.appearance.typography.font_size.inline_code | 旧缺省占位保留在源快照；候选继承上游默认 |
| 65 | settings.style.font_size.codeblock | theme.appearance.typography.font_size.code_block | 旧缺省占位保留在源快照；候选继承上游默认 |
| 66 | settings.style.font_family.body | theme.appearance.typography.font_family.body | 旧缺省占位保留在源快照；候选继承上游默认 |
| 67 | settings.style.font_family.code | theme.appearance.typography.font_family.code | 旧缺省占位保留在源快照；候选继承上游默认 |
| 68 | settings.style.border_radius.card-l | theme.appearance.shape.radius.card_large | 旧缺省占位保留在源快照；候选继承上游默认 |
| 69 | settings.style.border_radius.card | theme.appearance.shape.radius.card | 旧缺省占位保留在源快照；候选继承上游默认 |
| 70 | settings.style.border_radius.card-s | theme.appearance.shape.radius.card_small | 旧缺省占位保留在源快照；候选继承上游默认 |
| 71 | settings.style.border_radius.bar | theme.appearance.shape.radius.bar | 旧缺省占位保留在源快照；候选继承上游默认 |
| 72 | settings.style.border_radius.image-l | theme.appearance.shape.radius.image_large | 旧缺省占位保留在源快照；候选继承上游默认 |
| 73 | settings.style.border_radius.image | theme.appearance.shape.radius.image | 旧缺省占位保留在源快照；候选继承上游默认 |
| 74 | settings.style.border_radius.image-s | theme.appearance.shape.radius.image_small | 旧缺省占位保留在源快照；候选继承上游默认 |
| 75 | settings.style.gradient.start | theme.appearance.gradients.primary_action | 旧缺省占位保留在源快照；候选继承上游默认 |
| 76 | settings.style.gradient.searchbar | theme.appearance.gradients.search_bar | 旧缺省占位保留在源快照；候选继承上游默认 |
| 77 | settings.style.leftbar.background_image | theme.appearance.backgrounds.leftbar.image | 旧缺省占位保留在源快照；候选继承上游默认 |
| 78 | settings.style.leftbar.blur_px | theme.appearance.backgrounds.leftbar.backdrop.radius | 旧缺省占位保留在源快照；候选继承上游默认 |
| 79 | settings.style.site.background_image | theme.appearance.backgrounds.page.image | 旧缺省占位保留在源快照；候选继承上游默认 |
| 80 | settings.style.site.blur_px | theme.appearance.backgrounds.page.backdrop.radius | 旧缺省占位保留在源快照；候选继承上游默认 |
| 81 | settings.style.site.blur_sat | theme.appearance.backgrounds.page.backdrop.saturation | 旧缺省占位保留在源快照；候选继承上游默认 |
| 82 | settings.style.site.blur_bg | theme.appearance.backgrounds.page.backdrop.overlay | 旧缺省占位保留在源快照；候选继承上游默认 |
| 83 | settings.style.codeblock.scrollbar | theme.appearance.code_block.scrollbar_width | 旧缺省占位保留在源快照；候选继承上游默认 |
| 84 | settings.style.codeblock.highlightjs_theme | theme.appearance.code_block.highlight_stylesheet | 旧缺省占位保留在源快照；候选继承上游默认 |
| 85 | settings.tag_plugins.button.default_color | theme.tags.button.default_color | 旧缺省占位保留在源快照；候选继承上游默认 |
| 86 | settings.tag_plugins.checkbox.interactive | theme.tags.checkbox.interactive | 原值保留（包含 false/null/空列表） |
| 87 | settings.tag_plugins.note.border | theme.tags.note.border | 原值保留（包含 false/null/空列表） |
| 88 | settings.tag_plugins.note.default_color | theme.tags.note.default_color | 旧缺省占位保留在源快照；候选继承上游默认 |
| 89 | settings.tag_plugins.icon.default_color | theme.tags.icon.default_color | 旧缺省占位保留在源快照；候选继承上游默认 |
| 90 | settings.tag_plugins.mark.default_color | theme.tags.mark.default_color | 旧缺省占位保留在源快照；候选继承上游默认 |
| 91 | settings.tag_plugins.hashtag.default_color | theme.tags.hashtag.default_color | 旧缺省占位保留在源快照；候选继承上游默认 |
| 92 | settings.tag_plugins.gallery.size | theme.tags.gallery.size | 旧缺省占位保留在源快照；候选继承上游默认 |
| 93 | settings.tag_plugins.gallery.ratio | theme.tags.gallery.aspect_ratio | 旧缺省占位保留在源快照；候选继承上游默认 |
| 94 | settings.tag_plugins.quot.default | theme.tags.quot.default | 原值保留（包含 false/null/空列表） |
| 95 | settings.tag_plugins.quot.hashtag.prefix | theme.tags.quot.hashtag.prefix | 原值保留（包含 false/null/空列表） |
| 96 | settings.tag_plugins.quot.question.prefix | theme.tags.quot.question.prefix | 原值保留（包含 false/null/空列表） |
| 97 | settings.seo | blog.seo | 仅保留宿主 SEO 默认文案/robots/验证及未映射原字段；不覆盖 site，未知字段继续阻断 |
| 98 | settings.services.siteinfo.api | theme.services.site_info.site_info_api.endpoint | 旧缺省占位保留在源快照；候选继承上游默认 |
| 99 | settings.menubar.items[0].theme | theme.leftbar.menu[0].accent | 旧菜单 CSS 颜色改名；严格 CSS color 校验 |
| 100 | settings.menubar.items[1].theme | theme.leftbar.menu[1].accent | 旧菜单 CSS 颜色改名；严格 CSS color 校验 |
| 101 | settings.menubar.items[2].theme | theme.leftbar.menu[2].accent | 旧菜单 CSS 颜色改名；严格 CSS color 校验 |
| 102 | settings.menubar.items[3].theme | theme.leftbar.menu[3].accent | 旧菜单 CSS 颜色改名；严格 CSS color 校验 |
| 103 | settings.menubar.items | theme.leftbar.menu | 菜单顺序/ID/标题/图标/URL 保留；theme 改名 accent |
| 104 | settings.footer.social | theme.leftbar.footer.actions | 社会链接顺序保留；icon_key 优先，否则原 HTML 图标严格验证 |
| 105 | settings.footer.sitemap | theme.footer.sitemap | 链接对象转官方 Markdown 字符串，保留组及顺序 |
| 106 | settings.article.auto_excerpt | theme.article.listing.excerpt_length | 旧 0 代表默认；候选省略，继承上游默认，源原值保留 |
| 107 | settings.article.cover_ratio | theme.article.listing.cover_ratio | 旧 0 代表默认；候选省略，继承上游默认，源原值保留 |
| 108 | settings.article.card_style | theme.article.listing.card_layout | 旧未声明覆盖继承上游默认 |
| 109 | settings.article.indent | theme.article.paragraph_indent | 布尔缩进转 always/never |
| 110 | settings.article.tags | theme.article.footer.show_tags | null 为旧继承；候选省略，原值仍在源快照 |
| 111 | settings.article.copyright | theme.article.footer.license | 按旧 enabled 保留关闭/Markdown 许可文字 |
| 112 | settings.article.share | blog.content.share | 原值保留（包含 false/null/空列表） |
| 113 | settings.article.share.platforms | theme.article.footer.share + blog.content.share | wechat 候选转 qrcode；复制链接完整保留为 Blog 扩展 |
| 114 | settings.style.font_size.body | source.snapshot | 旧空默认占位原值保留；上游没有独立覆盖 |
| 115 | settings.style.font_family.codeblock | source.snapshot | 旧空默认占位原值保留；上游没有独立覆盖 |
| 116 | settings.style.leftbar.background_color_dark | source.snapshot | 旧空默认占位原值保留；上游没有独立覆盖 |
| 117 | settings.style.border_radius.block | source.snapshot | 旧空默认占位原值保留；上游没有独立覆盖 |
| 118 | settings.style.border_radius.navbar | source.snapshot | 旧空默认占位原值保留；上游没有独立覆盖 |
| 119 | settings.style.gradient.angle | source.snapshot | 旧空默认占位原值保留；上游没有独立覆盖 |
| 120 | settings.style.paginator | source.snapshot | 旧空默认占位原值保留；上游没有独立覆盖 |
| 121 | settings.style.header_prefix | source.snapshot | 旧空默认占位原值保留；上游没有独立覆盖 |
| 122 | settings.style.animated_avatar | source.snapshot | 旧空默认占位原值保留；上游没有独立覆盖 |
| 123 | settings.style.leftbar.background_opacity | theme.appearance.backgrounds.leftbar.opacity | 旧空值继承默认 |
| 124 | settings.plugins.scrollreveal.distance | theme.features.reveal.distance | 旧 px 长度转官方数字 px |
| 125 | settings.plugins.preload.service | theme.features.link_prefetch | flying_pages 使用官方实现 |
| 126 | settings.plugins.pjax | theme.features.partial_navigation.enabled | 开关保留；官方 manifest 导航替代旧 PJAX |
| 127 | settings.data_cache | source.snapshot | 完整旧默认对象与固定 Stellar 2.0.0 runtime 缓存策略完全一致；原值留源快照，候选继承官方等效策略 |
| 128 | settings.widgets.library | blog.data.widgets | 原值保留（包含 false/null/空列表） |
| 129 | settings.widgets.pages | theme.profiles | 旧空覆盖；继承官方默认区域 |
| 130 | settings.site_tree | theme.profiles | 逐 profile 映射；空列表明确清空，缺省继承 |
| 131 | settings.search.service | theme.search.provider / blog.search.provider | 空旧默认转 local；只有 go 显式选择 Go FTS |
| 132 | settings.search.local_search.cache_ttl | theme.search.local.cache_ttl_seconds | 原值保持 |
| 133 | settings.search.local_search.content | theme.search.local.include_content | 旧空占位继承官方默认 |
| 134 | settings.search.local_search.field | theme.search.local.scope | 旧空占位继承官方默认 |
| 135 | settings.search.local_search.path | source.snapshot | 官方索引和加载策略由 runtime policy 管理 |
| 136 | settings.search.local_search.lazy_load | source.snapshot | 官方索引和加载策略由 runtime policy 管理 |
| 137 | settings.search.local_search.skip_search | source.snapshot | 官方索引和加载策略由 runtime policy 管理 |
| 138 | settings.notebook.order_by | theme.profiles.notebook.listing.sort | 旧排序符号转 direction，字段由严格 schema 验证 |
| 139 | settings.tag_plugins.emoji | theme.tags.emoji | 所有 source 原值保留；默认 URL 转匹配的具名 source |
| 140 | settings.tag_plugins.quot.hashtag.suffix | theme.tags.quot.hashtag.suffix | 空后缀转官方 null |
| 141 | settings.tag_plugins.quot.question.suffix | theme.tags.quot.question.suffix | 空后缀转官方 null |
| 142 | settings.tag_plugins.copy | source.snapshot | 旧参数在源快照完整保留 |
| 143 | settings.tag_plugins.okr | source.snapshot | 旧 OKR 对象与固定 Stellar 2.0.0 边框、五状态颜色和 zh-CN 文案完全一致；原值留源快照，候选继承官方等效及请求 locale 文案 |
| 144 | settings.tag_plugins.timeline | source.snapshot | 旧参数在源快照完整保留 |
| 145 | settings.tag_plugins.gallery.layout | source.snapshot | 旧参数在源快照完整保留 |
| 146 | settings.cover | source.snapshot | 旧首页模板以非空 title 为显示门槛；已知 title/description 对象的 title 为空，无显示差异，原值留源快照且不转 Hero |
| 147 | settings.wiki[0].description | host.wiki_projects.description | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 148 | settings.wiki[0].icon | host.wiki_projects.icon | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 149 | settings.wiki[0].repo | host.wiki_projects.repo | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 150 | settings.wiki[0].sort | host.wiki_projects.sort | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 151 | settings.wiki[0].tags | host.wiki_project_tags | 已存在项目标签一致；原关联与 shelf 顺序不变 |
| 152 | settings.wiki[0].title | host.wiki_projects.title | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 153 | settings.wiki[0].url | host.wiki_projects.url | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 154 | settings.wiki[1].description | host.wiki_projects.description | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 155 | settings.wiki[1].icon | host.wiki_projects.icon | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 156 | settings.wiki[1].name | host.wiki_projects.name | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 157 | settings.wiki[1].repo | host.wiki_projects.repo | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 158 | settings.wiki[1].sort | host.wiki_projects.sort | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 159 | settings.wiki[1].tags | host.wiki_project_tags | 已存在项目标签一致；原关联与 shelf 顺序不变 |
| 160 | settings.wiki[1].title | host.wiki_projects.title | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 161 | settings.wiki[1].url | host.wiki_projects.url | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 162 | settings.wiki[2].description | host.wiki_projects.description | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 163 | settings.wiki[2].icon | host.wiki_projects.icon | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 164 | settings.wiki[2].name | host.wiki_projects.name | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 165 | settings.wiki[2].repo | host.wiki_projects.repo | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 166 | settings.wiki[2].sort | host.wiki_projects.sort | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 167 | settings.wiki[2].tags | host.wiki_project_tags | 已存在项目标签一致；原关联与 shelf 顺序不变 |
| 168 | settings.wiki[2].title | host.wiki_projects.title | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 169 | settings.wiki[2].url | host.wiki_projects.url | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 170 | settings.wiki | blog.data.wiki | 旧 shelf 顺序转官方项目 ID 字符串数组；冗余定义逐字段核对，冲突不能静默丢弃 |
| 171 | settings.article.share.enabled | blog.content.enabled | 旧本地分享或打赏显式启用，开启 Blog content 消费；不控制 blog_* 标签注册 |
| 172 | settings.article.reward.enabled | blog.content.enabled | 旧本地分享或打赏显式启用，开启 Blog content 消费；不控制 blog_* 标签注册 |

## 全局与配置诊断

| 类型 / 严重度 | Code | 来源 / 路径 | 阻断 | 说明 |
| --- | --- | --- | --- | --- |
| structure / error | legacy_notebook_schema | schema · notebooks/notes | 是 | 旧 INTEGER ID、nb_order、content/tags 表与新 TEXT ID、slug/status/content_md/tags_json 不兼容；必须单独审核 schema 转换，不能先假设 EnsureTable 会升级 |
| consumer / info | global_avatar_background_consumer | settings · site.avatar_bg | 否 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / info | global_rightbar_intro_consumer | settings · rightbar.intro | 否 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / info | global_welcome_linklist_consumer | settings · rightbar.welcome_linklist | 否 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / info | global_pages_consumer | settings · pages | 否 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / info | global_nav_tabs_consumer | settings · nav_tabs | 否 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / info | global_ai_label_consumer | settings · article.ai_label | 否 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / info | global_category_icons_consumer | settings · article.category_icon | 否 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / info | global_tag_icons_consumer | settings · article.tag_icon | 否 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / info | global_read_next_consumer | settings · article.read_next | 否 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / info | global_expire_days_consumer | settings · article.expire_days | 否 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / info | global_banner_ratio_consumer | settings · article.banner_ratio | 否 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / info | global_edit_this_page_consumer | settings · article.edit_this_page | 否 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / info | global_notebook_license_consumer | settings · notebook.license | 否 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / info | global_notebook_share_consumer | settings · notebook.share | 否 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / info | global_seo_consumer | settings · seo | 否 | 已批准的宿主 SEO 扩展仅存储不代表已消费；须实测 typed 默认文案、head 与 robots 后显式声明能力 |
| policy / error | pjax_policy_removed | settings · plugins.pjax | 是 | 旧存储 selectors/timeout 与官方 manifest 导航没有直接等效字段；不据存储值推断旧 runtime 正在消费，审核后明确接受官方策略 |
| consumer / info | widget_author_removed | settings · widgets.library.author | 否 | author widget 没有同等上游布局，完整定义保留在 Blog.data.widgets；需决定独立扩展消费 |
| policy / error | wiki_shelf_definition_conflict | settings · wiki[0].name | 是 | 旧 shelf 冗余定义与当前项目表不同或无目标；官方 shelf 仅 ID 数组，需明确集合值来源 |
| schema / error | invalid_value | migration.theme · leftbar.footer.actions[1].url | 是 | safe navigable URL or root-relative path |
| schema / error | invalid_value | migration.theme · leftbar.footer.actions[2].url | 是 | safe navigable URL or root-relative path |
| schema / error | invalid_value | migration.theme · leftbar.footer.actions[3].url | 是 | safe navigable URL or root-relative path |

## 全部逐记录候选（32 条）

逐条列全部映射/hash/诊断，不省略无冲突记录。host.* 表示既有宿主列，不塞入上游 Front Matter。候选 Markdown 变化仅是拟议转换，真实文件/DB未改；任何正文变化仍须逐记录 BodyDigest 精确批准。

### 1. `wiki_projects/go-blog`

Profile：`wiki`；候选路径：`/wiki/go-blog`；已 version 2 原文保留：否；候选 Markdown 变化：否。

源记录摘要：`a3cdbb49ca83da647e435de8aa2a628b49c194f9ce971785905c15dbcf37f944`；候选 strict 配置摘要：`d5c4c8ef6a58e527e52653cb63eb66154ec1dd0557501ca9fa99839f6cd93afe`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| wiki_projects/go-blog.title | front_matter.title | 公开展示字段原值保留 |
| wiki_projects/go-blog.available | host.wiki_projects.available | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/go-blog.branch | host.wiki_projects.branch | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/go-blog.cover | host.wiki_projects.cover | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/go-blog.created_at | host.wiki_projects.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/go-blog.description | host.wiki_projects.description | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/go-blog.headline | host.wiki_projects.headline | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/go-blog.hero_json | host.wiki_projects.hero_json | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/go-blog.icon | host.wiki_projects.icon | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/go-blog.id | host.wiki_projects.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/go-blog.name | host.wiki_projects.name | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/go-blog.pin | host.wiki_projects.pin | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/go-blog.repo | host.wiki_projects.repo | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/go-blog.sort | host.wiki_projects.sort | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/go-blog.subtitle | host.wiki_projects.subtitle | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/go-blog.title | host.wiki_projects.title | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/go-blog.updated_at | host.wiki_projects.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/go-blog.url | host.wiki_projects.url | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/go-blog.visible | host.wiki_projects.visible | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/go-blog.name | front_matter.name | 公开展示字段原值保留 |
| wiki_projects/go-blog.description | front_matter.description | 公开展示字段原值保留 |
| wiki_projects/go-blog.icon | front_matter.icon | 公开展示字段原值保留 |
| wiki_projects/go-blog.cover | front_matter.cover | 公开展示字段原值保留 |
| wiki_projects/go-blog.id/slug | front_matter.route.path | 保持旧公开集合路径 |
| wiki_projects/go-blog.sort | front_matter.listing.order | 公开展示字段原值保留 |
| wiki_projects/go-blog.wikidocs.project_id/doc_order/slug | front_matter.navigation.tree | 完整成员与顺序保留，权限仍由宿主状态控制 |

| 类型 / 严重度 | Code | 来源 / 路径 | 阻断 | 说明 |
| --- | --- | --- | --- | --- |
| policy / error | wiki_name_title | wiki_projects/go-blog · name/title | 是 | 旧 name 与 title 同时存在且不同；官方 Collection 只有 name/headline，须审核显示标题与技术名的归属 |
| policy / error | wiki_homepage | wiki_projects/go-blog · route.path/navigation.tree | 是 | 旧项目入口重定向至第一篇已发布文档；上游会生成首页/Collection 模型。两个候选都保留成员身份，需审核入口行为 |

### 2. `wiki_projects/stellar`

Profile：`wiki`；候选路径：`/wiki/stellar`；已 version 2 原文保留：否；候选 Markdown 变化：否。

源记录摘要：`a08a0e67fec3cbfdc5a9471e0f46e75858669a04f754ce564a9250139d58c7b1`；候选 strict 配置摘要：`ab7c9efe8c1fc99630b16d38cb3927709c3f9490c29ceb9a5518ebedb6e8de2a`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| wiki_projects/stellar.title | front_matter.title | 公开展示字段原值保留 |
| wiki_projects/stellar.available | host.wiki_projects.available | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/stellar.branch | host.wiki_projects.branch | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/stellar.cover | host.wiki_projects.cover | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/stellar.created_at | host.wiki_projects.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/stellar.description | host.wiki_projects.description | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/stellar.headline | host.wiki_projects.headline | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/stellar.hero_json | host.wiki_projects.hero_json | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/stellar.icon | host.wiki_projects.icon | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/stellar.id | host.wiki_projects.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/stellar.name | host.wiki_projects.name | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/stellar.pin | host.wiki_projects.pin | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/stellar.repo | host.wiki_projects.repo | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/stellar.sort | host.wiki_projects.sort | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/stellar.subtitle | host.wiki_projects.subtitle | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/stellar.title | host.wiki_projects.title | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/stellar.updated_at | host.wiki_projects.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/stellar.url | host.wiki_projects.url | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/stellar.visible | host.wiki_projects.visible | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/stellar.description | front_matter.description | 公开展示字段原值保留 |
| wiki_projects/stellar.icon | front_matter.icon | 公开展示字段原值保留 |
| wiki_projects/stellar.repo | front_matter.source.repository | 公开展示字段原值保留 |
| wiki_projects/stellar.id/slug | front_matter.route.path | 保持旧公开集合路径 |
| wiki_projects/stellar.sort | front_matter.listing.order | 公开展示字段原值保留 |
| wiki_projects/stellar.title | front_matter.name | 旧 name 空，显示标题保留为官方 name |
| wiki_projects/stellar.wikidocs.project_id/doc_order/slug | front_matter.navigation.tree | 完整成员与顺序保留，权限仍由宿主状态控制 |

无诊断。

### 3. `wiki_projects/frontend`

Profile：`wiki`；候选路径：`/wiki/frontend`；已 version 2 原文保留：否；候选 Markdown 变化：否。

源记录摘要：`aac47a76d4994e3bdd0be0e9fa7faa87b3636e320504840bde14cee56640a6db`；候选 strict 配置摘要：`508373429a440d4f72041209d4006d7fb2ddd8a48bd09fbdccf422837929de74`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| wiki_projects/frontend.title | front_matter.title | 公开展示字段原值保留 |
| wiki_projects/frontend.available | host.wiki_projects.available | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/frontend.branch | host.wiki_projects.branch | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/frontend.cover | host.wiki_projects.cover | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/frontend.created_at | host.wiki_projects.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/frontend.description | host.wiki_projects.description | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/frontend.headline | host.wiki_projects.headline | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/frontend.hero_json | host.wiki_projects.hero_json | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/frontend.icon | host.wiki_projects.icon | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/frontend.id | host.wiki_projects.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/frontend.name | host.wiki_projects.name | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/frontend.pin | host.wiki_projects.pin | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/frontend.repo | host.wiki_projects.repo | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/frontend.sort | host.wiki_projects.sort | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/frontend.subtitle | host.wiki_projects.subtitle | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/frontend.title | host.wiki_projects.title | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/frontend.updated_at | host.wiki_projects.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/frontend.url | host.wiki_projects.url | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/frontend.visible | host.wiki_projects.visible | 源列保持；候选只新增严格元数据与明确正文转换 |
| wiki_projects/frontend.description | front_matter.description | 公开展示字段原值保留 |
| wiki_projects/frontend.icon | front_matter.icon | 公开展示字段原值保留 |
| wiki_projects/frontend.id/slug | front_matter.route.path | 保持旧公开集合路径 |
| wiki_projects/frontend.sort | front_matter.listing.order | 公开展示字段原值保留 |
| wiki_projects/frontend.title | front_matter.name | 旧 name 空，显示标题保留为官方 name |
| wiki_projects/frontend.wikidocs.project_id/doc_order/slug | front_matter.navigation.tree | 完整成员与顺序保留，权限仍由宿主状态控制 |

无诊断。

### 4. `topics/1`

Profile：`topic`；候选路径：`/topic/stellar-1400`；已 version 2 原文保留：否；候选 Markdown 变化：否。

源记录摘要：`8ccbd27011967c5e22bd775130314b3ff116cbcd68a618d8e6524cf0bdc95103`；候选 strict 配置摘要：`ae0c7074a8c8c13ac8e9502c0b9adef2bdcb2fc3a10266d34e67deefdcca0de0`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| topics/1.cover | host.topics.cover | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/1.created_at | host.topics.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/1.description | host.topics.description | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/1.icon | host.topics.icon | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/1.id | host.topics.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/1.name | host.topics.name | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/1.slug | host.topics.slug | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/1.sort | host.topics.sort | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/1.updated_at | host.topics.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/1.slug | collection.id | stellar-1400 |
| topics/1.name | front_matter.name | 公开展示字段原值保留 |
| topics/1.id/slug | front_matter.route.path | 保持旧公开集合路径 |
| topics/1.sort | blog.content.collection_order | 旧集合列表排序独立保留，不能错用上游成员排序 |

| 类型 / 严重度 | Code | 来源 / 路径 | 阻断 | 说明 |
| --- | --- | --- | --- | --- |
| consumer / info | topic_order_consumer | topics/1 · sort | 否 | 旧专栏集合排序保留于 Blog 扩展；宿主集合列表必须消费后才能执行迁移 |

### 5. `topics/2`

Profile：`topic`；候选路径：`/topic/发布流程`；已 version 2 原文保留：否；候选 Markdown 变化：否。

源记录摘要：`9321d5379961980ce8699f33eb25dee141224045f6de8326be1141b80ee74165`；候选 strict 配置摘要：`76d9583839a20608819036297f5e61e48e46fe0164c840f3e4052021346c1ab5`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| topics/2.cover | host.topics.cover | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/2.created_at | host.topics.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/2.description | host.topics.description | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/2.icon | host.topics.icon | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/2.id | host.topics.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/2.name | host.topics.name | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/2.slug | host.topics.slug | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/2.sort | host.topics.sort | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/2.updated_at | host.topics.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/2.slug | collection.id | 发布流程 |
| topics/2.name | front_matter.name | 公开展示字段原值保留 |
| topics/2.id/slug | front_matter.route.path | 保持旧公开集合路径 |
| topics/2.sort | blog.content.collection_order | 旧集合列表排序独立保留，不能错用上游成员排序 |

| 类型 / 严重度 | Code | 来源 / 路径 | 阻断 | 说明 |
| --- | --- | --- | --- | --- |
| consumer / info | topic_order_consumer | topics/2 · sort | 否 | 旧专栏集合排序保留于 Blog 扩展；宿主集合列表必须消费后才能执行迁移 |

### 6. `topics/3`

Profile：`topic`；候选路径：`/topic/标签插件`；已 version 2 原文保留：否；候选 Markdown 变化：否。

源记录摘要：`656b1e91b54e7896cd75f763837e3517ffc472e06f0aadcce356a736ad5375ec`；候选 strict 配置摘要：`7ac33ba4bd870f0bfed1e122922004c48fd2cc3912b4d572f4831858d97fbe3b`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| topics/3.cover | host.topics.cover | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/3.created_at | host.topics.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/3.description | host.topics.description | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/3.icon | host.topics.icon | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/3.id | host.topics.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/3.name | host.topics.name | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/3.slug | host.topics.slug | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/3.sort | host.topics.sort | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/3.updated_at | host.topics.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/3.slug | collection.id | 标签插件 |
| topics/3.name | front_matter.name | 公开展示字段原值保留 |
| topics/3.id/slug | front_matter.route.path | 保持旧公开集合路径 |
| topics/3.sort | blog.content.collection_order | 旧集合列表排序独立保留，不能错用上游成员排序 |

| 类型 / 严重度 | Code | 来源 / 路径 | 阻断 | 说明 |
| --- | --- | --- | --- | --- |
| consumer / info | topic_order_consumer | topics/3 · sort | 否 | 旧专栏集合排序保留于 Blog 扩展；宿主集合列表必须消费后才能执行迁移 |

### 7. `topics/4`

Profile：`topic`；候选路径：`/topic/内容组织`；已 version 2 原文保留：否；候选 Markdown 变化：否。

源记录摘要：`e05e264709b62fbbd58f8f70fd1eebf08486c63411df074a45ff6a615bc30996`；候选 strict 配置摘要：`ae9b7bfd39e053d7be3145aa0266f11855efd39814f54555b382e04bfbaca642`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| topics/4.cover | host.topics.cover | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/4.created_at | host.topics.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/4.description | host.topics.description | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/4.icon | host.topics.icon | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/4.id | host.topics.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/4.name | host.topics.name | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/4.slug | host.topics.slug | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/4.sort | host.topics.sort | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/4.updated_at | host.topics.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/4.slug | collection.id | 内容组织 |
| topics/4.name | front_matter.name | 公开展示字段原值保留 |
| topics/4.id/slug | front_matter.route.path | 保持旧公开集合路径 |
| topics/4.sort | blog.content.collection_order | 旧集合列表排序独立保留，不能错用上游成员排序 |

| 类型 / 严重度 | Code | 来源 / 路径 | 阻断 | 说明 |
| --- | --- | --- | --- | --- |
| consumer / info | topic_order_consumer | topics/4 · sort | 否 | 旧专栏集合排序保留于 Blog 扩展；宿主集合列表必须消费后才能执行迁移 |

### 8. `topics/5`

Profile：`topic`；候选路径：`/topic/stellar-1410`；已 version 2 原文保留：否；候选 Markdown 变化：否。

源记录摘要：`e1103eca9ea17dcb5e388c6f0f700aecbfd7203d381161f233fa1c99139ad39c`；候选 strict 配置摘要：`84d968ed10a81e0bd99060c276f26956501d2ece94c9d03c7e69463d196abe3c`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| topics/5.cover | host.topics.cover | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/5.created_at | host.topics.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/5.description | host.topics.description | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/5.icon | host.topics.icon | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/5.id | host.topics.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/5.name | host.topics.name | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/5.slug | host.topics.slug | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/5.sort | host.topics.sort | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/5.updated_at | host.topics.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/5.slug | collection.id | stellar-1410 |
| topics/5.name | front_matter.name | 公开展示字段原值保留 |
| topics/5.id/slug | front_matter.route.path | 保持旧公开集合路径 |
| topics/5.sort | blog.content.collection_order | 旧集合列表排序独立保留，不能错用上游成员排序 |

| 类型 / 严重度 | Code | 来源 / 路径 | 阻断 | 说明 |
| --- | --- | --- | --- | --- |
| consumer / info | topic_order_consumer | topics/5 · sort | 否 | 旧专栏集合排序保留于 Blog 扩展；宿主集合列表必须消费后才能执行迁移 |

### 9. `topics/6`

Profile：`topic`；候选路径：`/topic/stellar-1421`；已 version 2 原文保留：否；候选 Markdown 变化：否。

源记录摘要：`e06a06e6602efb301d93b6d2b5047e09e4ec077241096fc7aac20d59e1f9e212`；候选 strict 配置摘要：`e35367e0476d22dd1c4e98003fc1782c39cbdd2c9ebfbb29a1cca549f86266b3`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| topics/6.cover | host.topics.cover | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/6.created_at | host.topics.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/6.description | host.topics.description | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/6.icon | host.topics.icon | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/6.id | host.topics.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/6.name | host.topics.name | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/6.slug | host.topics.slug | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/6.sort | host.topics.sort | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/6.updated_at | host.topics.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/6.slug | collection.id | stellar-1421 |
| topics/6.name | front_matter.name | 公开展示字段原值保留 |
| topics/6.id/slug | front_matter.route.path | 保持旧公开集合路径 |
| topics/6.sort | blog.content.collection_order | 旧集合列表排序独立保留，不能错用上游成员排序 |

| 类型 / 严重度 | Code | 来源 / 路径 | 阻断 | 说明 |
| --- | --- | --- | --- | --- |
| consumer / info | topic_order_consumer | topics/6 · sort | 否 | 旧专栏集合排序保留于 Blog 扩展；宿主集合列表必须消费后才能执行迁移 |

### 10. `topics/7`

Profile：`topic`；候选路径：`/topic/stellar-验收`；已 version 2 原文保留：否；候选 Markdown 变化：否。

源记录摘要：`4cd8f1bebe904eda6c961851d281f2588f1609a5f4784aa9fd57da72004eec68`；候选 strict 配置摘要：`4e6d97c60808b355b2c9dfeda4214771a8516d462708028019b4fb2a8abbfa92`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| topics/7.cover | host.topics.cover | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/7.created_at | host.topics.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/7.description | host.topics.description | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/7.icon | host.topics.icon | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/7.id | host.topics.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/7.name | host.topics.name | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/7.slug | host.topics.slug | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/7.sort | host.topics.sort | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/7.updated_at | host.topics.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| topics/7.slug | collection.id | stellar-验收 |
| topics/7.name | front_matter.name | 公开展示字段原值保留 |
| topics/7.cover | front_matter.cover | 公开展示字段原值保留 |
| topics/7.id/slug | front_matter.route.path | 保持旧公开集合路径 |
| topics/7.sort | blog.content.collection_order | 旧集合列表排序独立保留，不能错用上游成员排序 |

| 类型 / 严重度 | Code | 来源 / 路径 | 阻断 | 说明 |
| --- | --- | --- | --- | --- |
| consumer / info | topic_order_consumer | topics/7 · sort | 否 | 旧专栏集合排序保留于 Blog 扩展；宿主集合列表必须消费后才能执行迁移 |

### 11. `notebooks/2`

Profile：`notebook`；候选路径：`/notebooks/2`；已 version 2 原文保留：否；候选 Markdown 变化：否。

源记录摘要：`cce0e8e4828887887e68de7e15776017a50dc19f2b863ff7b60662678f3290ba`；候选 strict 配置摘要：`32242a0d27c53df98096e1cf82414ec96e5f5516238cea01440f2ef85817877e`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| notebooks/2.cover | host.notebooks.cover | 源列保持；候选只新增严格元数据与明确正文转换 |
| notebooks/2.created_at | host.notebooks.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| notebooks/2.description | host.notebooks.description | 源列保持；候选只新增严格元数据与明确正文转换 |
| notebooks/2.id | host.notebooks.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| notebooks/2.name | host.notebooks.name | 源列保持；候选只新增严格元数据与明确正文转换 |
| notebooks/2.nb_order | host.notebooks.nb_order | 源列保持；候选只新增严格元数据与明确正文转换 |
| notebooks/2.name | front_matter.name | 公开展示字段原值保留 |
| notebooks/2.description | front_matter.description | 公开展示字段原值保留 |
| notebooks/2.id/slug | front_matter.route.path | 保持旧公开集合路径 |
| notebooks/2.nb_order | front_matter.listing.order | 排序数值与方向保持 |

无诊断。

### 12. `posts/1`

Profile：`topic`；候选路径：`/post/140-card-tags`；已 version 2 原文保留：否；候选 Markdown 变化：是。

源记录摘要：`90b6a7c2fd467de88595ae0cc1f1840d2c0e9d7d2d0372cd81c049a8eb6e814e`；候选 strict 配置摘要：`c60c89373b6d76f1dbd1a79a96221382967928b44699d2840a4768c58f6d689a`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| posts/1.title | front_matter.title | 公开展示字段原值保留 |
| posts/1.ai_label | host.posts.ai_label | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.ai_summary | host.posts.ai_summary | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.author | host.posts.author | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.banner | host.posts.banner | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.category | host.posts.category | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.content_html | host.posts.content_html | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.content_md | host.posts.content_md | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.cover | host.posts.cover | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.created_at | host.posts.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.deleted_at | host.posts.deleted_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.draft_json | host.posts.draft_json | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.excerpt | host.posts.excerpt | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.id | host.posts.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.meta_description | host.posts.meta_description | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.meta_keywords | host.posts.meta_keywords | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.password | host.posts.password | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.pinned | host.posts.pinned | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.poster | host.posts.poster | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.publish_at | host.posts.publish_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.references | host.posts.references | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.slug | host.posts.slug | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.status | host.posts.status | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.tags | host.posts.tags | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.title | host.posts.title | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.topic | host.posts.topic | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.topic_id | host.posts.topic_id | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.updated_at | host.posts.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.views | host.posts.views | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/1.excerpt | front_matter.excerpt | 公开展示字段原值保留 |
| posts/1.cover | front_matter.cover | 公开展示字段原值保留 |
| posts/1.created_at | front_matter.date | 公开展示字段原值保留 |
| posts/1.updated_at | front_matter.updated | 公开展示字段原值保留 |
| posts/1.pinned | front_matter.listing.priority | 旧布尔 false/true 转 0/1 |
| posts/1.topic_id/topic | front_matter.collection | 迁移预览使用确切旧归属；strict runtime 不猜旧列 |
| posts/1.post_categories/categories | front_matter.categories | 所有完整层级路径与关联 position 顺序保留 |
| posts/1.markdown.byte[576] | blog_tip | 只改批准的 Blog 扩展名字；参数、正文及空白逐字保留 |
| posts/1.markdown.byte[740] | endblog_tip | 只改批准的 Blog 扩展名字；参数、正文及空白逐字保留 |

无诊断。

### 13. `posts/2`

Profile：`topic`；候选路径：`/post/140-draft`；已 version 2 原文保留：否；候选 Markdown 变化：是。

源记录摘要：`3c19496c457f3b8c2e5f9c27043b0917b9dab35f9496016012d5f6879cfa63f3`；候选 strict 配置摘要：`b96f5a83d3df89b60c9cd69f682a50d13eb5fb5428d542e0f3c0928880658f4b`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| posts/2.title | front_matter.title | 公开展示字段原值保留 |
| posts/2.ai_label | host.posts.ai_label | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.ai_summary | host.posts.ai_summary | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.author | host.posts.author | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.banner | host.posts.banner | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.category | host.posts.category | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.content_html | host.posts.content_html | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.content_md | host.posts.content_md | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.cover | host.posts.cover | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.created_at | host.posts.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.deleted_at | host.posts.deleted_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.draft_json | host.posts.draft_json | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.excerpt | host.posts.excerpt | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.id | host.posts.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.meta_description | host.posts.meta_description | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.meta_keywords | host.posts.meta_keywords | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.password | host.posts.password | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.pinned | host.posts.pinned | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.poster | host.posts.poster | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.publish_at | host.posts.publish_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.references | host.posts.references | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.slug | host.posts.slug | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.status | host.posts.status | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.tags | host.posts.tags | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.title | host.posts.title | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.topic | host.posts.topic | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.topic_id | host.posts.topic_id | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.updated_at | host.posts.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.views | host.posts.views | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/2.excerpt | front_matter.excerpt | 公开展示字段原值保留 |
| posts/2.created_at | front_matter.date | 公开展示字段原值保留 |
| posts/2.updated_at | front_matter.updated | 公开展示字段原值保留 |
| posts/2.pinned | front_matter.listing.priority | 旧布尔 false/true 转 0/1 |
| posts/2.topic_id/topic | front_matter.collection | 迁移预览使用确切旧归属；strict runtime 不猜旧列 |
| posts/2.post_categories/categories | front_matter.categories | 所有完整层级路径与关联 position 顺序保留 |
| posts/2.markdown.byte[115] | blog_tip | 只改批准的 Blog 扩展名字；参数、正文及空白逐字保留 |
| posts/2.markdown.byte[213] | endblog_tip | 只改批准的 Blog 扩展名字；参数、正文及空白逐字保留 |

无诊断。

### 14. `posts/3`

Profile：`topic`；候选路径：`/post/140-md-wrapper`；已 version 2 原文保留：否；候选 Markdown 变化：否。

源记录摘要：`38df19f4ad5847caade6d1bb75c9b66bdd7c2708854b33b8844264a21b142873`；候选 strict 配置摘要：`1f077e9a613de2c2d20c3eadae3fd215045fc50ea580c8a1ec7fa48cd6c9f8f1`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| posts/3.title | front_matter.title | 公开展示字段原值保留 |
| posts/3.ai_label | host.posts.ai_label | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.ai_summary | host.posts.ai_summary | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.author | host.posts.author | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.banner | host.posts.banner | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.category | host.posts.category | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.content_html | host.posts.content_html | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.content_md | host.posts.content_md | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.cover | host.posts.cover | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.created_at | host.posts.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.deleted_at | host.posts.deleted_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.draft_json | host.posts.draft_json | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.excerpt | host.posts.excerpt | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.id | host.posts.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.meta_description | host.posts.meta_description | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.meta_keywords | host.posts.meta_keywords | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.password | host.posts.password | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.pinned | host.posts.pinned | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.poster | host.posts.poster | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.publish_at | host.posts.publish_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.references | host.posts.references | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.slug | host.posts.slug | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.status | host.posts.status | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.tags | host.posts.tags | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.title | host.posts.title | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.topic | host.posts.topic | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.topic_id | host.posts.topic_id | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.updated_at | host.posts.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.views | host.posts.views | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/3.excerpt | front_matter.excerpt | 公开展示字段原值保留 |
| posts/3.created_at | front_matter.date | 公开展示字段原值保留 |
| posts/3.updated_at | front_matter.updated | 公开展示字段原值保留 |
| posts/3.pinned | front_matter.listing.priority | 旧布尔 false/true 转 0/1 |
| posts/3.topic_id/topic | front_matter.collection | 迁移预览使用确切旧归属；strict runtime 不猜旧列 |
| posts/3.post_categories/categories | front_matter.categories | 所有完整层级路径与关联 position 顺序保留 |

无诊断。

### 15. `posts/4`

Profile：`topic`；候选路径：`/post/140-navigation`；已 version 2 原文保留：否；候选 Markdown 变化：是。

源记录摘要：`456c0c58b3837d6a64081820025a7662d94f5d5240034809775a512090285052`；候选 strict 配置摘要：`f70e9b6d6f2fd8fdb7b6b34b91aa168a4fa47b4d3bbab216ab82ee98d1cb29cc`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| posts/4.title | front_matter.title | 公开展示字段原值保留 |
| posts/4.ai_label | host.posts.ai_label | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.ai_summary | host.posts.ai_summary | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.author | host.posts.author | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.banner | host.posts.banner | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.category | host.posts.category | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.content_html | host.posts.content_html | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.content_md | host.posts.content_md | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.cover | host.posts.cover | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.created_at | host.posts.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.deleted_at | host.posts.deleted_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.draft_json | host.posts.draft_json | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.excerpt | host.posts.excerpt | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.id | host.posts.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.meta_description | host.posts.meta_description | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.meta_keywords | host.posts.meta_keywords | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.password | host.posts.password | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.pinned | host.posts.pinned | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.poster | host.posts.poster | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.publish_at | host.posts.publish_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.references | host.posts.references | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.slug | host.posts.slug | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.status | host.posts.status | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.tags | host.posts.tags | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.title | host.posts.title | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.topic | host.posts.topic | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.topic_id | host.posts.topic_id | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.updated_at | host.posts.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.views | host.posts.views | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/4.excerpt | front_matter.excerpt | 公开展示字段原值保留 |
| posts/4.created_at | front_matter.date | 公开展示字段原值保留 |
| posts/4.updated_at | front_matter.updated | 公开展示字段原值保留 |
| posts/4.pinned | front_matter.listing.priority | 旧布尔 false/true 转 0/1 |
| posts/4.topic_id/topic | front_matter.collection | 迁移预览使用确切旧归属；strict runtime 不猜旧列 |
| posts/4.post_categories/categories | front_matter.categories | 所有完整层级路径与关联 position 顺序保留 |
| posts/4.markdown.byte[470] | blog_tip | 只改批准的 Blog 扩展名字；参数、正文及空白逐字保留 |
| posts/4.markdown.byte[611] | endblog_tip | 只改批准的 Blog 扩展名字；参数、正文及空白逐字保留 |

无诊断。

### 16. `posts/5`

Profile：`topic`；候选路径：`/post/140-reading-time`；已 version 2 原文保留：否；候选 Markdown 变化：否。

源记录摘要：`55e2eabc1e1c146f8c765bc8da6971a6342378c9483f73fbd98cb00f946f3ca7`；候选 strict 配置摘要：`bfdc8a412a16c0f737d0d251b4ecf7e1d5b254e3f973279b57fecd6c51af743a`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| posts/5.title | front_matter.title | 公开展示字段原值保留 |
| posts/5.ai_label | host.posts.ai_label | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.ai_summary | host.posts.ai_summary | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.author | host.posts.author | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.banner | host.posts.banner | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.category | host.posts.category | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.content_html | host.posts.content_html | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.content_md | host.posts.content_md | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.cover | host.posts.cover | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.created_at | host.posts.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.deleted_at | host.posts.deleted_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.draft_json | host.posts.draft_json | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.excerpt | host.posts.excerpt | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.id | host.posts.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.meta_description | host.posts.meta_description | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.meta_keywords | host.posts.meta_keywords | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.password | host.posts.password | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.pinned | host.posts.pinned | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.poster | host.posts.poster | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.publish_at | host.posts.publish_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.references | host.posts.references | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.slug | host.posts.slug | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.status | host.posts.status | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.tags | host.posts.tags | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.title | host.posts.title | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.topic | host.posts.topic | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.topic_id | host.posts.topic_id | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.updated_at | host.posts.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.views | host.posts.views | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/5.excerpt | front_matter.excerpt | 公开展示字段原值保留 |
| posts/5.created_at | front_matter.date | 公开展示字段原值保留 |
| posts/5.updated_at | front_matter.updated | 公开展示字段原值保留 |
| posts/5.pinned | front_matter.listing.priority | 旧布尔 false/true 转 0/1 |
| posts/5.topic_id/topic | front_matter.collection | 迁移预览使用确切旧归属；strict runtime 不猜旧列 |
| posts/5.post_categories/categories | front_matter.categories | 所有完整层级路径与关联 position 顺序保留 |

| 类型 / 严重度 | Code | 来源 / 路径 | 阻断 | 说明 |
| --- | --- | --- | --- | --- |
| consumer / info | poster_consumer | posts/5 · poster | 否 | 旧 poster headline/topic/caption/color 完整保留为 Blog 扩展；尚需接通模板消费，不假称视觉保留 |

### 17. `posts/6`

Profile：`topic`；候选路径：`/post/140-table-styles`；已 version 2 原文保留：否；候选 Markdown 变化：是。

源记录摘要：`dcfd518707ad5ab6f2545804ace77dffe31ea7d0659e79e917515c0093afabf9`；候选 strict 配置摘要：`f7231acaf21197c8d85a95473e6efd77c5c1133543781ff1df59469900e3e7d4`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| posts/6.title | front_matter.title | 公开展示字段原值保留 |
| posts/6.ai_label | host.posts.ai_label | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.ai_summary | host.posts.ai_summary | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.author | host.posts.author | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.banner | host.posts.banner | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.category | host.posts.category | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.content_html | host.posts.content_html | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.content_md | host.posts.content_md | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.cover | host.posts.cover | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.created_at | host.posts.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.deleted_at | host.posts.deleted_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.draft_json | host.posts.draft_json | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.excerpt | host.posts.excerpt | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.id | host.posts.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.meta_description | host.posts.meta_description | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.meta_keywords | host.posts.meta_keywords | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.password | host.posts.password | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.pinned | host.posts.pinned | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.poster | host.posts.poster | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.publish_at | host.posts.publish_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.references | host.posts.references | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.slug | host.posts.slug | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.status | host.posts.status | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.tags | host.posts.tags | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.title | host.posts.title | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.topic | host.posts.topic | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.topic_id | host.posts.topic_id | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.updated_at | host.posts.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.views | host.posts.views | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/6.excerpt | front_matter.excerpt | 公开展示字段原值保留 |
| posts/6.created_at | front_matter.date | 公开展示字段原值保留 |
| posts/6.updated_at | front_matter.updated | 公开展示字段原值保留 |
| posts/6.pinned | front_matter.listing.priority | 旧布尔 false/true 转 0/1 |
| posts/6.topic_id/topic | front_matter.collection | 迁移预览使用确切旧归属；strict runtime 不猜旧列 |
| posts/6.post_categories/categories | front_matter.categories | 所有完整层级路径与关联 position 顺序保留 |
| posts/6.markdown.byte[651] | blog_tip | 只改批准的 Blog 扩展名字；参数、正文及空白逐字保留 |
| posts/6.markdown.byte[803] | endblog_tip | 只改批准的 Blog 扩展名字；参数、正文及空白逐字保留 |

无诊断。

### 18. `posts/8`

Profile：`topic`；候选路径：`/post/141-stellar-141`；已 version 2 原文保留：否；候选 Markdown 变化：否。

源记录摘要：`1f7d80c8c929fae82c3eaee03d69deeffc96af559abbeec72ddaac7eb1930380`；候选 strict 配置摘要：`0e041de5e510aaf5ccbe6a6938a8b8907f7b94beef76707d8f5c12eca12fdd72`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| posts/8.title | front_matter.title | 公开展示字段原值保留 |
| posts/8.ai_label | host.posts.ai_label | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.ai_summary | host.posts.ai_summary | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.author | host.posts.author | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.banner | host.posts.banner | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.category | host.posts.category | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.content_html | host.posts.content_html | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.content_md | host.posts.content_md | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.cover | host.posts.cover | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.created_at | host.posts.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.deleted_at | host.posts.deleted_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.draft_json | host.posts.draft_json | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.excerpt | host.posts.excerpt | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.id | host.posts.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.meta_description | host.posts.meta_description | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.meta_keywords | host.posts.meta_keywords | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.password | host.posts.password | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.pinned | host.posts.pinned | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.poster | host.posts.poster | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.publish_at | host.posts.publish_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.references | host.posts.references | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.slug | host.posts.slug | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.status | host.posts.status | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.tags | host.posts.tags | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.title | host.posts.title | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.topic | host.posts.topic | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.topic_id | host.posts.topic_id | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.updated_at | host.posts.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.views | host.posts.views | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/8.excerpt | front_matter.excerpt | 公开展示字段原值保留 |
| posts/8.cover | front_matter.cover | 公开展示字段原值保留 |
| posts/8.ai_label | front_matter.article.ai_label | 公开展示字段原值保留 |
| posts/8.created_at | front_matter.date | 公开展示字段原值保留 |
| posts/8.updated_at | front_matter.updated | 公开展示字段原值保留 |
| posts/8.pinned | front_matter.listing.priority | 旧布尔 false/true 转 0/1 |
| posts/8.topic_id/topic | front_matter.collection | 迁移预览使用确切旧归属；strict runtime 不猜旧列 |
| posts/8.post_categories/categories | front_matter.categories | 所有完整层级路径与关联 position 顺序保留 |

无诊断。

### 19. `posts/9`

Profile：`topic`；候选路径：`/post/141-feature-playground`；已 version 2 原文保留：否；候选 Markdown 变化：否。

源记录摘要：`b0947af3cf5e0145ed75c808cde5255c899442b59b358ddec6cf17ca4e030599`；候选 strict 配置摘要：`1e7831d305432c98962911d68ea91d72c7aae247ba65a66b39f1d8d010b46299`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| posts/9.title | front_matter.title | 公开展示字段原值保留 |
| posts/9.ai_label | host.posts.ai_label | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.ai_summary | host.posts.ai_summary | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.author | host.posts.author | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.banner | host.posts.banner | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.category | host.posts.category | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.content_html | host.posts.content_html | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.content_md | host.posts.content_md | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.cover | host.posts.cover | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.created_at | host.posts.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.deleted_at | host.posts.deleted_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.draft_json | host.posts.draft_json | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.excerpt | host.posts.excerpt | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.id | host.posts.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.meta_description | host.posts.meta_description | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.meta_keywords | host.posts.meta_keywords | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.password | host.posts.password | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.pinned | host.posts.pinned | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.poster | host.posts.poster | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.publish_at | host.posts.publish_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.references | host.posts.references | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.slug | host.posts.slug | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.status | host.posts.status | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.tags | host.posts.tags | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.title | host.posts.title | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.topic | host.posts.topic | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.topic_id | host.posts.topic_id | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.updated_at | host.posts.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.views | host.posts.views | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/9.excerpt | front_matter.excerpt | 公开展示字段原值保留 |
| posts/9.cover | front_matter.cover | 公开展示字段原值保留 |
| posts/9.ai_label | front_matter.article.ai_label | 公开展示字段原值保留 |
| posts/9.created_at | front_matter.date | 公开展示字段原值保留 |
| posts/9.updated_at | front_matter.updated | 公开展示字段原值保留 |
| posts/9.pinned | front_matter.listing.priority | 旧布尔 false/true 转 0/1 |
| posts/9.topic_id/topic | front_matter.collection | 迁移预览使用确切旧归属；strict runtime 不猜旧列 |
| posts/9.post_categories/categories | front_matter.categories | 所有完整层级路径与关联 position 顺序保留 |

无诊断。

### 20. `posts/10`

Profile：`topic`；候选路径：`/post/142-stellar-142`；已 version 2 原文保留：否；候选 Markdown 变化：否。

源记录摘要：`eb1f33d8273f7a253fe3421daaa053ae7b449c5e71f3e9026639d8c8e8db5aed`；候选 strict 配置摘要：`8f945339e5157df469513d9a8e3f3a99a2bfc7c43c6e9c2eea4eaab428b89137`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| posts/10.title | front_matter.title | 公开展示字段原值保留 |
| posts/10.ai_label | host.posts.ai_label | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.ai_summary | host.posts.ai_summary | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.author | host.posts.author | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.banner | host.posts.banner | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.category | host.posts.category | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.content_html | host.posts.content_html | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.content_md | host.posts.content_md | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.cover | host.posts.cover | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.created_at | host.posts.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.deleted_at | host.posts.deleted_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.draft_json | host.posts.draft_json | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.excerpt | host.posts.excerpt | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.id | host.posts.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.meta_description | host.posts.meta_description | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.meta_keywords | host.posts.meta_keywords | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.password | host.posts.password | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.pinned | host.posts.pinned | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.poster | host.posts.poster | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.publish_at | host.posts.publish_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.references | host.posts.references | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.slug | host.posts.slug | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.status | host.posts.status | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.tags | host.posts.tags | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.title | host.posts.title | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.topic | host.posts.topic | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.topic_id | host.posts.topic_id | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.updated_at | host.posts.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.views | host.posts.views | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/10.excerpt | front_matter.excerpt | 公开展示字段原值保留 |
| posts/10.cover | front_matter.cover | 公开展示字段原值保留 |
| posts/10.ai_label | front_matter.article.ai_label | 公开展示字段原值保留 |
| posts/10.created_at | front_matter.date | 公开展示字段原值保留 |
| posts/10.updated_at | front_matter.updated | 公开展示字段原值保留 |
| posts/10.pinned | front_matter.listing.priority | 旧布尔 false/true 转 0/1 |
| posts/10.topic_id/topic | front_matter.collection | 迁移预览使用确切旧归属；strict runtime 不猜旧列 |
| posts/10.post_categories/categories | front_matter.categories | 所有完整层级路径与关联 position 顺序保留 |

| 类型 / 严重度 | Code | 来源 / 路径 | 阻断 | 说明 |
| --- | --- | --- | --- | --- |
| policy / error | banner_cover | posts/10 · banner/cover | 是 | 旧 banner 与 cover 是独立图源，官方 banner.background 是布尔且图源来自 cover；禁止覆盖旧 cover |

### 21. `posts/11`

Profile：`topic`；候选路径：`/post/stellar-143-acceptance`；已 version 2 原文保留：否；候选 Markdown 变化：否。

源记录摘要：`4933a09519053b23774f54124adf4d000829f1291134741449f7bfa062f68bc0`；候选 strict 配置摘要：`e314d57a3109ab94a873347f7a2894e2f166e585207091adbebe6214bc19e14a`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| posts/11.title | front_matter.title | 公开展示字段原值保留 |
| posts/11.ai_label | host.posts.ai_label | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.ai_summary | host.posts.ai_summary | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.author | host.posts.author | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.banner | host.posts.banner | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.category | host.posts.category | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.content_html | host.posts.content_html | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.content_md | host.posts.content_md | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.cover | host.posts.cover | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.created_at | host.posts.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.deleted_at | host.posts.deleted_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.draft_json | host.posts.draft_json | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.excerpt | host.posts.excerpt | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.id | host.posts.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.meta_description | host.posts.meta_description | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.meta_keywords | host.posts.meta_keywords | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.password | host.posts.password | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.pinned | host.posts.pinned | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.poster | host.posts.poster | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.publish_at | host.posts.publish_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.references | host.posts.references | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.slug | host.posts.slug | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.status | host.posts.status | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.tags | host.posts.tags | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.title | host.posts.title | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.topic | host.posts.topic | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.topic_id | host.posts.topic_id | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.updated_at | host.posts.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.views | host.posts.views | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/11.excerpt | front_matter.excerpt | 公开展示字段原值保留 |
| posts/11.created_at | front_matter.date | 公开展示字段原值保留 |
| posts/11.updated_at | front_matter.updated | 公开展示字段原值保留 |
| posts/11.pinned | front_matter.listing.priority | 旧布尔 false/true 转 0/1 |
| posts/11.topic_id/topic | front_matter.collection | 迁移预览使用确切旧归属；strict runtime 不猜旧列 |
| posts/11.post_categories/categories | front_matter.categories | 所有完整层级路径与关联 position 顺序保留 |

| 类型 / 严重度 | Code | 来源 / 路径 | 阻断 | 说明 |
| --- | --- | --- | --- | --- |
| consumer / info | poster_consumer | posts/11 · poster | 否 | 旧 poster headline/topic/caption/color 完整保留为 Blog 扩展；尚需接通模板消费，不假称视觉保留 |

### 22. `posts/12`

Profile：`post`；候选路径：`/post/shortcode-可视化编辑测试`；已 version 2 原文保留：否；候选 Markdown 变化：是。

源记录摘要：`4cef8d87e43bb746ff300a932696b629a2416514fa0ef1b553d3c205e8518567`；候选 strict 配置摘要：`39812c816018856aae5ece44baaeaf2934fc9334a1edacad00d58e21a15b1194`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| posts/12.title | front_matter.title | 公开展示字段原值保留 |
| posts/12.ai_label | host.posts.ai_label | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.ai_summary | host.posts.ai_summary | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.author | host.posts.author | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.banner | host.posts.banner | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.category | host.posts.category | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.content_html | host.posts.content_html | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.content_md | host.posts.content_md | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.cover | host.posts.cover | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.created_at | host.posts.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.deleted_at | host.posts.deleted_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.draft_json | host.posts.draft_json | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.excerpt | host.posts.excerpt | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.id | host.posts.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.meta_description | host.posts.meta_description | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.meta_keywords | host.posts.meta_keywords | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.password | host.posts.password | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.pinned | host.posts.pinned | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.poster | host.posts.poster | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.publish_at | host.posts.publish_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.references | host.posts.references | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.slug | host.posts.slug | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.status | host.posts.status | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.tags | host.posts.tags | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.title | host.posts.title | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.topic | host.posts.topic | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.topic_id | host.posts.topic_id | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.updated_at | host.posts.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.views | host.posts.views | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/12.cover | front_matter.cover | 公开展示字段原值保留 |
| posts/12.created_at | front_matter.date | 公开展示字段原值保留 |
| posts/12.updated_at | front_matter.updated | 公开展示字段原值保留 |
| posts/12.pinned | front_matter.listing.priority | 旧布尔 false/true 转 0/1 |
| posts/12.markdown.byte[674] | image.fancybox:false | 显式保留旧 image 默认禁用放大；避免官方 true 改变点击行为 |

无诊断。

### 23. `posts/13`

Profile：`topic`；候选路径：`/post/stellar-144-acceptance`；已 version 2 原文保留：否；候选 Markdown 变化：否。

源记录摘要：`54008206c1ca620fa4bb4236bcbbd5d99267a1a8ec289d590c430eb21f127186`；候选 strict 配置摘要：`4301d0e974df2993bdd0af3a4d2512b247d918088a702a9f6e86e2184f698b51`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| posts/13.title | front_matter.title | 公开展示字段原值保留 |
| posts/13.ai_label | host.posts.ai_label | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.ai_summary | host.posts.ai_summary | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.author | host.posts.author | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.banner | host.posts.banner | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.category | host.posts.category | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.content_html | host.posts.content_html | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.content_md | host.posts.content_md | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.cover | host.posts.cover | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.created_at | host.posts.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.deleted_at | host.posts.deleted_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.draft_json | host.posts.draft_json | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.excerpt | host.posts.excerpt | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.id | host.posts.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.meta_description | host.posts.meta_description | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.meta_keywords | host.posts.meta_keywords | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.password | host.posts.password | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.pinned | host.posts.pinned | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.poster | host.posts.poster | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.publish_at | host.posts.publish_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.references | host.posts.references | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.slug | host.posts.slug | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.status | host.posts.status | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.tags | host.posts.tags | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.title | host.posts.title | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.topic | host.posts.topic | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.topic_id | host.posts.topic_id | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.updated_at | host.posts.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.views | host.posts.views | 源列保持；候选只新增严格元数据与明确正文转换 |
| posts/13.excerpt | front_matter.excerpt | 公开展示字段原值保留 |
| posts/13.cover | front_matter.cover | 公开展示字段原值保留 |
| posts/13.created_at | front_matter.date | 公开展示字段原值保留 |
| posts/13.updated_at | front_matter.updated | 公开展示字段原值保留 |
| posts/13.pinned | front_matter.listing.priority | 旧布尔 false/true 转 0/1 |
| posts/13.topic_id/topic | front_matter.collection | 迁移预览使用确切旧归属；strict runtime 不猜旧列 |
| posts/13.post_categories/categories | front_matter.categories | 所有完整层级路径与关联 position 顺序保留 |

| 类型 / 严重度 | Code | 来源 / 路径 | 阻断 | 说明 |
| --- | --- | --- | --- | --- |
| consumer / info | poster_consumer | posts/13 · poster | 否 | 旧 poster headline/topic/caption/color 完整保留为 Blog 扩展；尚需接通模板消费，不假称视觉保留 |

### 24. `pages/1`

Profile：`page`；候选路径：`/friends`；已 version 2 原文保留：否；候选 Markdown 变化：否。

源记录摘要：`cf3e11c6b5415be6dcf70848d54dad35c2456de6fcc1d1db821bf143313b65de`；候选 strict 配置摘要：`6b2bfb78e22e80d5ff2cb63fa716dcf4ac5f8b5880782ba8885c613509be63ae`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| pages/1.title | front_matter.title | 公开展示字段原值保留 |
| pages/1.author | host.pages.author | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/1.banner | host.pages.banner | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/1.banner_info | host.pages.banner_info | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/1.breadcrumb | host.pages.breadcrumb | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/1.content_html | host.pages.content_html | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/1.content_md | host.pages.content_md | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/1.created_at | host.pages.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/1.deleted_at | host.pages.deleted_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/1.id | host.pages.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/1.is_system | host.pages.is_system | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/1.leftbar_widgets | host.pages.leftbar_widgets | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/1.published_at | host.pages.published_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/1.rightbar_widgets | host.pages.rightbar_widgets | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/1.slug | host.pages.slug | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/1.status | host.pages.status | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/1.title | host.pages.title | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/1.updated_at | host.pages.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/1.views | host.pages.views | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/1.created_at | front_matter.date | 公开展示字段原值保留 |
| pages/1.updated_at | front_matter.updated | 公开展示字段原值保留 |

无诊断。

### 25. `pages/2`

Profile：`page`；候选路径：`/notes`；已 version 2 原文保留：否；候选 Markdown 变化：是。

源记录摘要：`66a242dad1a96182b4edb0833717762288ea6a54dd978fde84c26318a5eb3f8c`；候选 strict 配置摘要：`7a50b5ba15a09a0d4cdfc1fe5b46beebb6e054209f03555852e786be245cbf04`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| pages/2.title | front_matter.title | 公开展示字段原值保留 |
| pages/2.author | host.pages.author | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/2.banner | host.pages.banner | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/2.banner_info | host.pages.banner_info | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/2.breadcrumb | host.pages.breadcrumb | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/2.content_html | host.pages.content_html | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/2.content_md | host.pages.content_md | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/2.created_at | host.pages.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/2.deleted_at | host.pages.deleted_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/2.id | host.pages.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/2.is_system | host.pages.is_system | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/2.leftbar_widgets | host.pages.leftbar_widgets | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/2.published_at | host.pages.published_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/2.rightbar_widgets | host.pages.rightbar_widgets | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/2.slug | host.pages.slug | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/2.status | host.pages.status | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/2.title | host.pages.title | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/2.updated_at | host.pages.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/2.views | host.pages.views | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/2.created_at | front_matter.date | 公开展示字段原值保留 |
| pages/2.updated_at | front_matter.updated | 公开展示字段原值保留 |
| pages/2.markdown.byte[1851] | blog_moments | 只改批准的 Blog 扩展名字；参数、正文及空白逐字保留 |

无诊断。

### 26. `pages/3`

Profile：`page`；候选路径：`/links`；已 version 2 原文保留：否；候选 Markdown 变化：是。

源记录摘要：`abbeb8c0d60235c4b58cfdc8f437b31f286b85f996cbbc11bcc686647bb2dcdc`；候选 strict 配置摘要：`21db8249023fa7cbc9be3836a6453ac7bf0404c263715795fbc752724f720753`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| pages/3.title | front_matter.title | 公开展示字段原值保留 |
| pages/3.author | host.pages.author | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/3.banner | host.pages.banner | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/3.banner_info | host.pages.banner_info | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/3.breadcrumb | host.pages.breadcrumb | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/3.content_html | host.pages.content_html | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/3.content_md | host.pages.content_md | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/3.created_at | host.pages.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/3.deleted_at | host.pages.deleted_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/3.id | host.pages.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/3.is_system | host.pages.is_system | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/3.leftbar_widgets | host.pages.leftbar_widgets | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/3.published_at | host.pages.published_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/3.rightbar_widgets | host.pages.rightbar_widgets | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/3.slug | host.pages.slug | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/3.status | host.pages.status | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/3.title | host.pages.title | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/3.updated_at | host.pages.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/3.views | host.pages.views | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/3.created_at | front_matter.date | 公开展示字段原值保留 |
| pages/3.updated_at | front_matter.updated | 公开展示字段原值保留 |
| pages/3.markdown.byte[0] | blog_links | 只改批准的 Blog 扩展名字；参数、正文及空白逐字保留 |

无诊断。

### 27. `pages/5`

Profile：`page`；候选路径：`/140-test-page`；已 version 2 原文保留：否；候选 Markdown 变化：否。

源记录摘要：`d3244b1bb2f26b6466d39810328f3b0f9f5ace50034728945d8ee62c4fe03ead`；候选 strict 配置摘要：`280624f3b71a535f7946b712929ba1537b007bc2265da3f483fc5e3594bcf517`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| pages/5.title | front_matter.title | 公开展示字段原值保留 |
| pages/5.author | host.pages.author | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/5.banner | host.pages.banner | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/5.banner_info | host.pages.banner_info | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/5.breadcrumb | host.pages.breadcrumb | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/5.content_html | host.pages.content_html | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/5.content_md | host.pages.content_md | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/5.created_at | host.pages.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/5.deleted_at | host.pages.deleted_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/5.id | host.pages.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/5.is_system | host.pages.is_system | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/5.leftbar_widgets | host.pages.leftbar_widgets | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/5.published_at | host.pages.published_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/5.rightbar_widgets | host.pages.rightbar_widgets | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/5.slug | host.pages.slug | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/5.status | host.pages.status | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/5.title | host.pages.title | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/5.updated_at | host.pages.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/5.views | host.pages.views | 源列保持；候选只新增严格元数据与明确正文转换 |
| pages/5.created_at | front_matter.date | 公开展示字段原值保留 |
| pages/5.updated_at | front_matter.updated | 公开展示字段原值保留 |

无诊断。

### 28. `wikidocs/1`

Profile：`wiki`；候选路径：`/wiki/go-blog/acceptance`；已 version 2 原文保留：否；候选 Markdown 变化：否。

源记录摘要：`2dc12423cce6e5de05eacd3e3a4856d173ba8e02c33f1e9da94c3870e871c72e`；候选 strict 配置摘要：`207ac2497fa4334750a98ffa62674d9dc8455355c17e51e0ab9e980a7e3ebd1c`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| wikidocs/1.title | front_matter.title | 公开展示字段原值保留 |
| wikidocs/1.content_html | host.wikidocs.content_html | 源列保持；候选只新增严格元数据与明确正文转换 |
| wikidocs/1.content_md | host.wikidocs.content_md | 源列保持；候选只新增严格元数据与明确正文转换 |
| wikidocs/1.created_at | host.wikidocs.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| wikidocs/1.doc_order | host.wikidocs.doc_order | 源列保持；候选只新增严格元数据与明确正文转换 |
| wikidocs/1.id | host.wikidocs.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| wikidocs/1.parent | host.wikidocs.parent | 源列保持；候选只新增严格元数据与明确正文转换 |
| wikidocs/1.project_id | host.wikidocs.project_id | 源列保持；候选只新增严格元数据与明确正文转换 |
| wikidocs/1.slug | host.wikidocs.slug | 源列保持；候选只新增严格元数据与明确正文转换 |
| wikidocs/1.status | host.wikidocs.status | 源列保持；候选只新增严格元数据与明确正文转换 |
| wikidocs/1.title | host.wikidocs.title | 源列保持；候选只新增严格元数据与明确正文转换 |
| wikidocs/1.updated_at | host.wikidocs.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| wikidocs/1.created_at | front_matter.date | 公开展示字段原值保留 |
| wikidocs/1.updated_at | front_matter.updated | 公开展示字段原值保留 |
| wikidocs/1.project_id | front_matter.collection | 原 Wiki 归属保持，不移动文档 |

无诊断。

### 29. `wikidocs/2`

Profile：`wiki`；候选路径：`/wiki/go-blog/responsive`；已 version 2 原文保留：否；候选 Markdown 变化：否。

源记录摘要：`1a1050d6ff4c32f4564521e54720f90fba83dd219b1a91658e823875904b97fe`；候选 strict 配置摘要：`723ec7499379141d7222366b880131d9fede81a029741876fea9383e2b73a188`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| wikidocs/2.title | front_matter.title | 公开展示字段原值保留 |
| wikidocs/2.content_html | host.wikidocs.content_html | 源列保持；候选只新增严格元数据与明确正文转换 |
| wikidocs/2.content_md | host.wikidocs.content_md | 源列保持；候选只新增严格元数据与明确正文转换 |
| wikidocs/2.created_at | host.wikidocs.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| wikidocs/2.doc_order | host.wikidocs.doc_order | 源列保持；候选只新增严格元数据与明确正文转换 |
| wikidocs/2.id | host.wikidocs.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| wikidocs/2.parent | host.wikidocs.parent | 源列保持；候选只新增严格元数据与明确正文转换 |
| wikidocs/2.project_id | host.wikidocs.project_id | 源列保持；候选只新增严格元数据与明确正文转换 |
| wikidocs/2.slug | host.wikidocs.slug | 源列保持；候选只新增严格元数据与明确正文转换 |
| wikidocs/2.status | host.wikidocs.status | 源列保持；候选只新增严格元数据与明确正文转换 |
| wikidocs/2.title | host.wikidocs.title | 源列保持；候选只新增严格元数据与明确正文转换 |
| wikidocs/2.updated_at | host.wikidocs.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| wikidocs/2.created_at | front_matter.date | 公开展示字段原值保留 |
| wikidocs/2.updated_at | front_matter.updated | 公开展示字段原值保留 |
| wikidocs/2.project_id | front_matter.collection | 原 Wiki 归属保持，不移动文档 |

无诊断。

### 30. `notes/2`

Profile：`notebook`；候选路径：``；已 version 2 原文保留：否；候选 Markdown 变化：否。

源记录摘要：`d32335f452e5e90f81860e4531f5a8999c14f751e62a85fde11729e7e896e1a8`；候选 strict 配置摘要：`c117ac8b9208b9d81b66aaec953250dd233a1efdd6c59ee46219c551bc36f795`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| notes/2.title | front_matter.title | 公开展示字段原值保留 |
| notes/2.content | host.notes.content | 源列保持；候选只新增严格元数据与明确正文转换 |
| notes/2.content_html | host.notes.content_html | 源列保持；候选只新增严格元数据与明确正文转换 |
| notes/2.created_at | host.notes.created_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| notes/2.id | host.notes.id | 源列保持；候选只新增严格元数据与明确正文转换 |
| notes/2.notebook_id | host.notes.notebook_id | 源列保持；候选只新增严格元数据与明确正文转换 |
| notes/2.tags | host.notes.tags | 源列保持；候选只新增严格元数据与明确正文转换 |
| notes/2.title | host.notes.title | 源列保持；候选只新增严格元数据与明确正文转换 |
| notes/2.updated_at | host.notes.updated_at | 源列保持；候选只新增严格元数据与明确正文转换 |
| notes/2.created_at | front_matter.date | 公开展示字段原值保留 |
| notes/2.updated_at | front_matter.updated | 公开展示字段原值保留 |
| notes/2.notebook_id | front_matter.collection | 旧数字 ID 的路径文本保持；不换父本 |

| 类型 / 严重度 | Code | 来源 / 路径 | 阻断 | 说明 |
| --- | --- | --- | --- | --- |
| structure / error | note_missing_slug_status | notes/2 · slug/status | 是 | 旧笔记无 slug/status/owner，当前没有逐笔记公开路由；迁移须显式决定新公开路径与发布状态，不可假定作者 |

### 31. `content/posts/stellar-143-acceptance.md`

Profile：`topic`；候选路径：`/post/stellar-143-acceptance`；已 version 2 原文保留：否；候选 Markdown 变化：否。

源记录摘要：`260ee4414f652b8e10656c3968ad8a58895d34252d4486d4a55dd9a454dd255d`；候选 strict 配置摘要：`dee29cceffdd2f2456b8182ef91eb434c03ee9e0e663fbb19b8ca53acc2edbf7`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| /content/posts/stellar-143-acceptance.md.id | host..id | 源列保持；候选只新增严格元数据与明确正文转换 |
| content/posts/stellar-143-acceptance.md.category | candidate.front_matter/blog | 每字段候选保留；unknown 原值仍保留并由严格 schema 报错 |
| content/posts/stellar-143-acceptance.md.date | candidate.front_matter/blog | 每字段候选保留；unknown 原值仍保留并由严格 schema 报错 |
| content/posts/stellar-143-acceptance.md.excerpt | candidate.front_matter/blog | 每字段候选保留；unknown 原值仍保留并由严格 schema 报错 |
| content/posts/stellar-143-acceptance.md.poster_caption | candidate.front_matter/blog | 每字段候选保留；unknown 原值仍保留并由严格 schema 报错 |
| content/posts/stellar-143-acceptance.md.poster_color | candidate.front_matter/blog | 每字段候选保留；unknown 原值仍保留并由严格 schema 报错 |
| content/posts/stellar-143-acceptance.md.poster_headline | candidate.front_matter/blog | 每字段候选保留；unknown 原值仍保留并由严格 schema 报错 |
| content/posts/stellar-143-acceptance.md.poster_topic | candidate.front_matter/blog | 每字段候选保留；unknown 原值仍保留并由严格 schema 报错 |
| content/posts/stellar-143-acceptance.md.status | blog.status | 保留原发布状态，使用已定义的宿主 Markdown 导入契约；不进入上游 schema |
| content/posts/stellar-143-acceptance.md.tags | candidate.front_matter/blog | 每字段候选保留；unknown 原值仍保留并由严格 schema 报错 |
| content/posts/stellar-143-acceptance.md.title | candidate.front_matter/blog | 每字段候选保留；unknown 原值仍保留并由严格 schema 报错 |
| content/posts/stellar-143-acceptance.md.topic | candidate.front_matter/blog | 每字段候选保留；unknown 原值仍保留并由严格 schema 报错 |

| 类型 / 严重度 | Code | 来源 / 路径 | 阻断 | 说明 |
| --- | --- | --- | --- | --- |
| consumer / info | poster_consumer | content/posts/stellar-143-acceptance.md · poster_caption | 否 | 旧独立 poster 字段完整保留于 Blog 扩展，尚需接通消费 |
| consumer / info | poster_consumer | content/posts/stellar-143-acceptance.md · poster_color | 否 | 旧独立 poster 字段完整保留于 Blog 扩展，尚需接通消费 |
| consumer / info | poster_consumer | content/posts/stellar-143-acceptance.md · poster_headline | 否 | 旧独立 poster 字段完整保留于 Blog 扩展，尚需接通消费 |
| consumer / info | poster_consumer | content/posts/stellar-143-acceptance.md · poster_topic | 否 | 旧独立 poster 字段完整保留于 Blog 扩展，尚需接通消费 |
| structure / error | file_host_fields | content/posts/stellar-143-acceptance.md · status | 是 | 发布状态保留于 blog.status；须验证宿主导入保存该状态，不得把草稿隐式发布 |

### 32. `content/posts/stellar-144-acceptance.md`

Profile：`topic`；候选路径：`/post/stellar-144-acceptance`；已 version 2 原文保留：否；候选 Markdown 变化：否。

源记录摘要：`dc2a829ac7483466d6f0165c6be42339765bdb6f73f697ab5c05b79127b4205d`；候选 strict 配置摘要：`9be36cce609b6e378471e1b8075e30c3069696cc42c25520ed7674574e992f78`。

| 来源字段 | 候选目标 | 行为 |
| --- | --- | --- |
| /content/posts/stellar-144-acceptance.md.id | host..id | 源列保持；候选只新增严格元数据与明确正文转换 |
| content/posts/stellar-144-acceptance.md.category | candidate.front_matter/blog | 每字段候选保留；unknown 原值仍保留并由严格 schema 报错 |
| content/posts/stellar-144-acceptance.md.date | candidate.front_matter/blog | 每字段候选保留；unknown 原值仍保留并由严格 schema 报错 |
| content/posts/stellar-144-acceptance.md.excerpt | candidate.front_matter/blog | 每字段候选保留；unknown 原值仍保留并由严格 schema 报错 |
| content/posts/stellar-144-acceptance.md.poster_caption | candidate.front_matter/blog | 每字段候选保留；unknown 原值仍保留并由严格 schema 报错 |
| content/posts/stellar-144-acceptance.md.poster_color | candidate.front_matter/blog | 每字段候选保留；unknown 原值仍保留并由严格 schema 报错 |
| content/posts/stellar-144-acceptance.md.poster_headline | candidate.front_matter/blog | 每字段候选保留；unknown 原值仍保留并由严格 schema 报错 |
| content/posts/stellar-144-acceptance.md.poster_topic | candidate.front_matter/blog | 每字段候选保留；unknown 原值仍保留并由严格 schema 报错 |
| content/posts/stellar-144-acceptance.md.status | blog.status | 保留原发布状态，使用已定义的宿主 Markdown 导入契约；不进入上游 schema |
| content/posts/stellar-144-acceptance.md.tags | candidate.front_matter/blog | 每字段候选保留；unknown 原值仍保留并由严格 schema 报错 |
| content/posts/stellar-144-acceptance.md.title | candidate.front_matter/blog | 每字段候选保留；unknown 原值仍保留并由严格 schema 报错 |
| content/posts/stellar-144-acceptance.md.topic | candidate.front_matter/blog | 每字段候选保留；unknown 原值仍保留并由严格 schema 报错 |

| 类型 / 严重度 | Code | 来源 / 路径 | 阻断 | 说明 |
| --- | --- | --- | --- | --- |
| consumer / info | poster_consumer | content/posts/stellar-144-acceptance.md · poster_caption | 否 | 旧独立 poster 字段完整保留于 Blog 扩展，尚需接通消费 |
| consumer / info | poster_consumer | content/posts/stellar-144-acceptance.md · poster_color | 否 | 旧独立 poster 字段完整保留于 Blog 扩展，尚需接通消费 |
| consumer / info | poster_consumer | content/posts/stellar-144-acceptance.md · poster_headline | 否 | 旧独立 poster 字段完整保留于 Blog 扩展，尚需接通消费 |
| consumer / info | poster_consumer | content/posts/stellar-144-acceptance.md · poster_topic | 否 | 旧独立 poster 字段完整保留于 Blog 扩展，尚需接通消费 |
| structure / error | file_host_fields | content/posts/stellar-144-acceptance.md · status | 是 | 发布状态保留于 blog.status；须验证宿主导入保存该状态，不得把草稿隐式发布 |

## 本次验证与执行边界

- 格式 3 迁移包 race、真实 SQLite 稀疏 rowid/external-content FTS MATCH、同数值 INTEGER/REAL、BLOB/大整数、完整批准、防篡改、事务 rollback/取消及正常提交 hook 已通过；日期小修另以正式驱动读取 time.Time 和非法类型/日期反例验证。常规测试全部使用隔离夹具。
- 显式只读 helper 本次捕获真实源；DB/WAL/SHM/journal 与 tracked 文件前后指纹相同，DB/WAL/hash/源表计数与历史匹配。日期校验修复后只从同一私密 snapshot 重新生成，不再读取真实源。
- 公开报告完整性核对 32 records、172 settings mappings、766 record mappings；20 capabilities true，34 consumer 诊断均仍存在且 nonblocking；政策/schema/结构未消失。
- BuildMigrationFixture 没有真实目标参数；未对本次真实候选执行 BuildFixture，也没有真实 Apply、push、发布或生产部署。用户确认前不能转换真实数据。
- 隔离执行证明的 coverage 排除原索引/视图/触发器、上传字节/附件、原配置文件、运行缓存/untracked；完整运行位置隔离不能冒充完整资源复制或真实前台验收。
- 真实迁移执行前仍必须重新确认源一致性，并绑定 SourceDigest、完整 CandidateDigest、每项具名 decision 和逐记录正文批准；源变化会使批准过期。
