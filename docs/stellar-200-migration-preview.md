# Stellar 2.0.0 存量迁移审查

状态：实现中。T1 真实源的只读候选已重新生成；真实迁移未执行、未获授权，尚无真实源 Apply 入口；结构工具仅能新建私密隔离库。本文替换第一轮预检，保留完整脱敏映射和逐记录清单。不能以候选通过生成、消费者单测或历史界面证据代替迁移后的实测。

## 本次证据与私密边界

2026-10-13 从根工作树 `Blog` / `dev` / `02ee112` 读取 `data/blog.db`；`ReadSnapshot` 使用 SQLite `mode=ro`、`query_only=1` 和只读事务，未使用 immutable、未 seed、未建表、未修改真实配置/内容/上传目录，也未删除侧车。根 `.pi/` 与 `docs/plans/maintenance/` 保留。

- SourceDigest：`6c7983b1b42d43b0456e786c3c98ca61fc16a84c2acabe76893e1febf39e8293`，与 review-6 历史源摘要一致。
- 主 DB SHA256：`40cd42cb5c704b457be8b6761d226c53ee14896211584dd25f0f9287d2454f0a`，读取前后及 review-6 一致。
- 本次 DB、WAL、SHM、journal 前后指纹相同；这是本次观察结果。`mode=ro` 并不保证整个目录零写，活跃 WAL 的锁侧车仍可能产生或变化，不应用 immutable 绕过活跃 WAL。
- 两份 tracked 文章逐字摘要前后相同。另一个 tracked Markdown `content/notes.md` 是系统 `/notes` 初次启动的正文来源，无 Front Matter；不伪造为第三条文章候选，已单独记录前后 hash，并保留文件。当前 `/notes` 内容以 DB pages 候选审查。
- 完整候选保留源设置、全文、身份及密钥，只存在私密输出；公开报告不含原值、正文、邮箱、密钥、密码哈希或私密 alternatives。

完整输出位于迁移 worktree 的 `.tmp/stellar-migration-review-20261013-consumer-1/`：目录 `0700`，`candidate.private.json`、`report.public.json`、`source-check.json`、`notebook-structure.private.json`、`notebook-structure.public.json` 均为 `0600`。不得提交、外发或复制私密候选。本文由公开报告生成，无私密源内容。

本次仍采用默认空 capabilities：32 条候选（30 DB + 2 tracked）、171 项配置映射、766 项逐记录映射、48 条阻断。分组为 consumer 34、policy 7、structure 4、schema 3；后续实际闭合消费者可由主 agent 显式声明重新生成，政策、结构、schema 不可用 capabilities 自动解除。

相较 review-6 的 34 阻断，本次解除仅完整匹配固定上游默认的 OKR 假阳性，并补回 15 条原先仅存储、漏报验证门槛的全局消费者诊断，因此默认总数为 48；这表示审查覆盖被校正，不能解释为真实数据已发生损失。

## 集中审查的真实数据决定

以下决定按具体来源处理；选择尚未写入候选，也没有真实执行授权。已批准“严格 V2 编辑器仅一个集合显示名 name，内部标识为现有 ID”，不等于批准旧记录 name/title 取哪一个值。值及完整两种候选仅在私密 candidate 中，本页仅列来源与行为，防止泄露原设置或正文。

| 审查项 | 涉及来源 | 保留和变化 / 可选处理 | 推荐与执行前条件 |
| --- | --- | --- | --- |
| PJAX 策略 | settings.plugins.pjax | 旧 selectors/timeout 没有上游字段。采用官方 manifest/shell-signature/同集合导航，旧设置仍留源快照；或另行决定保留为独立扩展，不能伪造 strict 字段。 | 建议接受官方策略；需要主 agent 提供新导航实测，用户确认差异后才能解除。 |
| data_cache | settings.data_cache | 旧独立自定义缓存策略不进入 theme；选择采用官方 runtime policy，或明确要求独立 Blog 扩展承担原策略。 | 建议采用官方 runtime policy；完整旧值可在本地私密候选审查，不静默丢弃。 |
| 主页 cover | settings.cover | 旧独立主页 cover 不在上游 theme。可迁为官方首页内容/Hero 等效入口，或独立 Blog 扩展保留原显示。 | 重要体验选择须附对应效果后集中确认；当前候选不删除旧设置。 |
| Wiki shelf 名称来源 | settings.wiki[0].name → wiki_projects/go-blog | 官方 shelf 只保存项目 ID。冗余 shelf 名称与项目表不同：保留项目表定义，或审核 shelf 覆盖集合 name；不能保留双真值。 | 与下一项一起选择；ID与成员关联保持。 |
| Wiki name/title | wiki_projects/go-blog.name/title | 一个官方 name，可选保留旧 name，或取旧 title 为显示 name；内部标识保持现有 ID，不重命名路由。 | 编辑器单 name 已批准；这条真实记录的取值尚未批准。 |
| Wiki 集合首页 | wiki_projects/go-blog.route.path/navigation.tree | 旧入口跳第一篇已发布文档。可采用官方 Collection 首页，或独立 Blog 扩展保留旧重定向；两候选均保留成员、顺序和本地 canonical。 | 建议采用官方 Collection 首页；保留重定向选项尚须消费实测，不能先解除。 |
| banner/cover 图源 | posts/10.banner/cover | 可保留旧 cover 用于列表，独立 blog.content.banner_image 仅覆盖详情；或明确选旧 banner 作官方 cover（同时改变列表图）。 | 建议独立 banner 保留双图源；消费已由实施者安排验证，真实记录仍须批准。 |
| Footer 三个占位链接 | settings.footer.social → leftbar.footer.actions[1..3].url | 三条 url="#" 被严格 safe navigable URL 校验拒绝。可移除这三条占位按钮，或分别提供有效外链/站内根相对路径；不能自动改成其它目的地。 | 建议移除仅占位项；必须用户确认或提供三个有效目标。 |
| 旧 Note 路由 / 状态 / owner | notes/2（父 notebook 原 ID保持） | 旧表无 slug、status、owner，当前未建立逐 Note公开路由。不能按时间或全部用户猜作者；须确认新 slug、draft/published 和拥有者身份，或先保留不公开且待补元数据。 | 建议保留原 ID/正文/标签/父本且不自动发布；正式实现不能把未定义 owner 当已解决。 |
| Notebook/Note 表结构 | schema.notebooks/notes | Notebook ID与Note父本 → TEXT；Note ID保持原 INTEGER。nb_order→sort、content→content_md、tags→tags_json；保持ID/关系/日期/正文。 | 隔离旧/新库及当前Store读取/strict重快照证明已通过；仍须批准notebooks/2.owner和updated_at来源（可明确沿用created_at）。真实结构改变后重新快照批准，不能沿用旧digest。 |
| Tracked 发布状态 | 两份 content/posts/stellar-14{3,4}-acceptance.md.status | 候选已修为 blog.status（不进入上游 schema，也不落旧 blog.host.status）。保留原发布状态；draft/published 序列化定向测试通过。 | 这是已确认宿主契约的接线项，不新增用户选择；仍保持 file_host_fields 阻断，直到真实导入/重开证明，不能隐式发布。 |

已解决的等效项：真实旧 OKR 完整对象精确匹配固定 9d11978 的 border=true、五色及 zh-CN 文案；原值留私密源快照，候选继承官方与请求 locale。该项不再要求用户接受视觉损失；自定义、缺键、额外字段仍阻断。

Notebook/Note 实时结构预览列 21 项已确定映射、5 个未批准字段：notebooks/2.author_id、notebooks/2.updated_at、notes/2.slug/status/author_id。结构工具只有全新系统临时库入口；证明成功不标 version 2，也不解除真实原候选诊断。

## 消费者与执行接线门槛

consumer 不是要求用户决定是否保留功能：独立 Blog 扩展已获批准，实施者应完成全部消费者。当前默认报告不假称闭合：

| 能力 | 本源阻断条数 | 当前边界 |
| --- | --- | --- |
| poster | 11（3 DB记录 + 两份文件各4字段） | 完整 headline/topic/caption/color已保留；主 agent 核对 route/embed及真实界面后，才可显式声明poster。 |
| collection_order | 7（topics/1–7） | 原排序保留；需主 agent 核对真正集合列表顺序与分页，才可声明collection_order。 |
| authors | 0 | catalog支持；本源没有触发该阻断，不可据此宣称所有多作者流程已验收。 |
| wiki_external_url | 0 | catalog支持；不改变local canonical/document route，本源没有触发该阻断。 |
| author_widget | 1（settings.widgets.library.author） | 完整定义保留于 Blog.data.widgets；尚无闭合证据，不声明、不问用户重复决定。 |
| avatar_background | 1 | 独立全局扩展原值保留；仅存储或仅代码接通不等于已实测，不自动声明消费者。 |
| rightbar_intro | 1 | 独立全局扩展原值保留；仅存储或仅代码接通不等于已实测，不自动声明消费者。 |
| welcome_linklist | 1 | 独立全局扩展原值保留；仅存储或仅代码接通不等于已实测，不自动声明消费者。 |
| nav_tabs | 1 | 独立全局扩展原值保留；仅存储或仅代码接通不等于已实测，不自动声明消费者。 |
| ai_label | 1 | 独立全局扩展原值保留；仅存储或仅代码接通不等于已实测，不自动声明消费者。 |
| category_icons | 1 | 独立全局扩展原值保留；仅存储或仅代码接通不等于已实测，不自动声明消费者。 |
| tag_icons | 1 | 独立全局扩展原值保留；仅存储或仅代码接通不等于已实测，不自动声明消费者。 |
| read_next | 1 | 独立全局扩展原值保留；仅存储或仅代码接通不等于已实测，不自动声明消费者。 |
| expire_days | 1 | 独立全局扩展原值保留；仅存储或仅代码接通不等于已实测，不自动声明消费者。 |
| banner_ratio | 1 | 独立全局扩展原值保留；仅存储或仅代码接通不等于已实测，不自动声明消费者。 |
| edit_this_page | 1 | 独立全局扩展原值保留；仅存储或仅代码接通不等于已实测，不自动声明消费者。 |
| notebook_license | 1 | 独立全局扩展原值保留；仅存储或仅代码接通不等于已实测，不自动声明消费者。 |
| notebook_share | 1 | 独立全局扩展原值保留；仅存储或仅代码接通不等于已实测，不自动声明消费者。 |
| pages | 1 | 独立全局扩展原值保留；仅存储或仅代码接通不等于已实测，不自动声明消费者。 |
| seo | 1 | 独立全局扩展原值保留；仅存储或仅代码接通不等于已实测，不自动声明消费者。 |

迁移树 `.tmp/migration-review-helper/main.go` 是只读生成工具：默认 `{}`；`-capabilities` 必须由主 agent 提供显式 JSON，未知 capability 被拒绝。为不同能力报告选择一个新的输出目录，工具以 O_EXCL 写 0600 文件，不能覆盖现有私密候选。声明能力只把对应诊断保留为 info/nonblocking，不执行迁移，不解除其它政策。

Notebook footer 保留独立 blog.content.notebook.license/share；严格 profile 不允许 profiles.note.footer，不能伪造字段。Pages仅friends已冻结属性的消费者可声明；未消费Hero animation/start/preview/actions等保真并保留schema阻断。

SEO已冻结六项官方目标：preconnect→preconnect，canonical.originalHost→canonical.host，canonical.officialHosts→canonical.allowed_hosts，open_graph.enable→open_graph.enabled，open_graph.twitter_id→open_graph.twitter_id，structured_data.links→structured_data.same_as。空host转null、旧nil slice转[]、旧nil enable按旧Enabled为true；false/空数组保持。坏类型/未知字段不丢弃，同一目标来源冲突继续阻断。

SEO余七项default_description/default_keywords/robots_extra/no_index/google_verification/bing_verification/baidu_verification保留typed Blog.seo，不覆盖独立Blog.site；canonical已映射值不再重复塞入该袋。它们的head/robots及默认优先级消费者需另行实测声明。

实施者交付证据（尚待主组合和浏览器核验）：13项content消费者提交f45ad24/c7ab767已有真实embedded SSR定向测试、race及正常hook；pages/SEO typed桥提交9ad6a6e已有embedded SSR/preview、保存重建与清除、服务开关及robots回归。SEO residual head另有独立模板测试。这里仅记录源码与隔离测试交付，不将上述20项能力标成前台已验收，默认capabilities仍为空。

## 完整源表计数

所有表都被只读快照摘要覆盖，评论、用户、权限、草稿及非迁移业务表的变化也使候选过期。以下行数包含所有状态，不等于公开数量。

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

## 完整配置映射（171 项）

仅列路径和转换说明，源值保留于私密候选；含宿主设置、密钥的目标路径不代表密钥进入公开 theme。

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
| 127 | settings.data_cache | source.snapshot | 旧缓存策略完整保留在源快照 |
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
| 146 | settings.wiki[0].description | host.wiki_projects.description | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 147 | settings.wiki[0].icon | host.wiki_projects.icon | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 148 | settings.wiki[0].repo | host.wiki_projects.repo | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 149 | settings.wiki[0].sort | host.wiki_projects.sort | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 150 | settings.wiki[0].tags | host.wiki_project_tags | 已存在项目标签一致；原关联与 shelf 顺序不变 |
| 151 | settings.wiki[0].title | host.wiki_projects.title | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 152 | settings.wiki[0].url | host.wiki_projects.url | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 153 | settings.wiki[1].description | host.wiki_projects.description | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 154 | settings.wiki[1].icon | host.wiki_projects.icon | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 155 | settings.wiki[1].name | host.wiki_projects.name | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 156 | settings.wiki[1].repo | host.wiki_projects.repo | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 157 | settings.wiki[1].sort | host.wiki_projects.sort | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 158 | settings.wiki[1].tags | host.wiki_project_tags | 已存在项目标签一致；原关联与 shelf 顺序不变 |
| 159 | settings.wiki[1].title | host.wiki_projects.title | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 160 | settings.wiki[1].url | host.wiki_projects.url | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 161 | settings.wiki[2].description | host.wiki_projects.description | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 162 | settings.wiki[2].icon | host.wiki_projects.icon | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 163 | settings.wiki[2].name | host.wiki_projects.name | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 164 | settings.wiki[2].repo | host.wiki_projects.repo | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 165 | settings.wiki[2].sort | host.wiki_projects.sort | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 166 | settings.wiki[2].tags | host.wiki_project_tags | 已存在项目标签一致；原关联与 shelf 顺序不变 |
| 167 | settings.wiki[2].title | host.wiki_projects.title | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 168 | settings.wiki[2].url | host.wiki_projects.url | 旧 shelf 与现有集合列一致，值保留在集合候选 |
| 169 | settings.wiki | blog.data.wiki | 旧 shelf 顺序转官方项目 ID 字符串数组；冗余定义逐字段核对，冲突不能静默丢弃 |
| 170 | settings.article.share.enabled | blog.content.enabled | 旧本地分享或打赏显式启用，开启 Blog content 消费；不控制 blog_* 标签注册 |
| 171 | settings.article.reward.enabled | blog.content.enabled | 旧本地分享或打赏显式启用，开启 Blog content 消费；不控制 blog_* 标签注册 |

## 全局与配置诊断

| 类型 / 严重度 | Code | 来源 / 路径 | 阻断 | 说明 |
| --- | --- | --- | --- | --- |
| structure / error | legacy_notebook_schema | schema · notebooks/notes | 是 | 旧 INTEGER ID、nb_order、content/tags 表与新 TEXT ID、slug/status/content_md/tags_json 不兼容；必须单独审核 schema 转换，不能先假设 EnsureTable 会升级 |
| consumer / error | global_avatar_background_consumer | settings · site.avatar_bg | 是 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / error | global_rightbar_intro_consumer | settings · rightbar.intro | 是 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / error | global_welcome_linklist_consumer | settings · rightbar.welcome_linklist | 是 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / error | global_pages_consumer | settings · pages | 是 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / error | global_nav_tabs_consumer | settings · nav_tabs | 是 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / error | global_ai_label_consumer | settings · article.ai_label | 是 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / error | global_category_icons_consumer | settings · article.category_icon | 是 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / error | global_tag_icons_consumer | settings · article.tag_icon | 是 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / error | global_read_next_consumer | settings · article.read_next | 是 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / error | global_expire_days_consumer | settings · article.expire_days | 是 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / error | global_banner_ratio_consumer | settings · article.banner_ratio | 是 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / error | global_edit_this_page_consumer | settings · article.edit_this_page | 是 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / error | global_notebook_license_consumer | settings · notebook.license | 是 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / error | global_notebook_share_consumer | settings · notebook.share | 是 | 已批准的本地能力保留在独立 Blog 扩展；仅存储不代表已消费，须主执行者实测后显式声明消费者能力 |
| consumer / error | global_seo_consumer | settings · seo | 是 | 已批准的宿主 SEO 扩展仅存储不代表已消费；须实测 typed 默认文案、head 与 robots 后显式声明能力 |
| policy / error | pjax_policy_removed | settings · plugins.pjax | 是 | 旧 selectors/timeout 与官方 manifest 导航没有直接等效字段；审核后明确接受官方策略 |
| policy / error | data_cache_removed | settings · data_cache | 是 | 2.0 runtime policy 没有公开同等配置；禁止伪造 features.data_cache，需确认具体策略处理 |
| consumer / error | widget_author_removed | settings · widgets.library.author | 是 | author widget 没有同等上游布局，完整定义保留在 Blog.data.widgets；需决定独立扩展消费 |
| policy / error | home_cover_removed | settings · cover | 是 | 旧主页 cover 独立配置不在上游主题结构；需确认独立 Blog 展示或首页内容迁移 |
| policy / error | wiki_shelf_definition_conflict | settings · wiki[0].name | 是 | 旧 shelf 冗余定义与当前项目表不同或无目标；官方 shelf 仅 ID 数组，需明确集合值来源 |
| schema / error | invalid_value | migration.theme · leftbar.footer.actions[1].url | 是 | safe navigable URL or root-relative path |
| schema / error | invalid_value | migration.theme · leftbar.footer.actions[2].url | 是 | safe navigable URL or root-relative path |
| schema / error | invalid_value | migration.theme · leftbar.footer.actions[3].url | 是 | safe navigable URL or root-relative path |

## 全部逐记录候选（32 条）

下列每条均列全部来源/目标映射、hash和诊断；不省略无冲突记录。宿主原列映射为 host.* 表示保持既有列，不把这些值塞入上游 Front Matter。Markdown变化表示候选中的显式标签/缺省语义转换，不表示真实文件或DB已改。

### 1. `wiki_projects/frontend`

Profile：`wiki`；候选路径：`/wiki/frontend`；已version 2原文保留：否；候选Markdown变化：否。

源摘要：`3f3b94915d70b106b6faf3599a334a4e5dd9fb7de0e96031fee306440aac28c3`；候选 strict 配置摘要：`508373429a440d4f72041209d4006d7fb2ddd8a48bd09fbdccf422837929de74`。

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

### 2. `wiki_projects/stellar`

Profile：`wiki`；候选路径：`/wiki/stellar`；已version 2原文保留：否；候选Markdown变化：否。

源摘要：`882802a3d790ad066ac0f99710c80ae8ee8ccb7ff571f4c283fa208f38f50108`；候选 strict 配置摘要：`ab7c9efe8c1fc99630b16d38cb3927709c3f9490c29ceb9a5518ebedb6e8de2a`。

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

### 3. `wiki_projects/go-blog`

Profile：`wiki`；候选路径：`/wiki/go-blog`；已version 2原文保留：否；候选Markdown变化：否。

源摘要：`829dfd63a2f9d534c5d0a2e995acd0e983a1e1b87b207f03cb53ac69d9322ae1`；候选 strict 配置摘要：`d5c4c8ef6a58e527e52653cb63eb66154ec1dd0557501ca9fa99839f6cd93afe`。

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

### 4. `topics/1`

Profile：`topic`；候选路径：`/topic/stellar-1400`；已version 2原文保留：否；候选Markdown变化：否。

源摘要：`e9d773ea4cae7e1f62fb9f678243f7fdc35f4e24a25dbe8ba790031f0e8ec22c`；候选 strict 配置摘要：`ae0c7074a8c8c13ac8e9502c0b9adef2bdcb2fc3a10266d34e67deefdcca0de0`。

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
| consumer / error | topic_order_consumer | topics/1 · sort | 是 | 旧专栏集合排序保留于 Blog 扩展；宿主集合列表必须消费后才能执行迁移 |

### 5. `topics/2`

Profile：`topic`；候选路径：`/topic/发布流程`；已version 2原文保留：否；候选Markdown变化：否。

源摘要：`3a01998de9a2511b2e4125e70e079d37a88395729dc0e13557dfe5af1d2425b8`；候选 strict 配置摘要：`76d9583839a20608819036297f5e61e48e46fe0164c840f3e4052021346c1ab5`。

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
| consumer / error | topic_order_consumer | topics/2 · sort | 是 | 旧专栏集合排序保留于 Blog 扩展；宿主集合列表必须消费后才能执行迁移 |

### 6. `topics/3`

Profile：`topic`；候选路径：`/topic/标签插件`；已version 2原文保留：否；候选Markdown变化：否。

源摘要：`5960e54ba61f2f76270a452f9178acf1ad016f0baf8f6c6c4406ac903d2dbdab`；候选 strict 配置摘要：`7ac33ba4bd870f0bfed1e122922004c48fd2cc3912b4d572f4831858d97fbe3b`。

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
| consumer / error | topic_order_consumer | topics/3 · sort | 是 | 旧专栏集合排序保留于 Blog 扩展；宿主集合列表必须消费后才能执行迁移 |

### 7. `topics/4`

Profile：`topic`；候选路径：`/topic/内容组织`；已version 2原文保留：否；候选Markdown变化：否。

源摘要：`768b207f5d69868ba8173031c1efa414fa9aaea9a3baf6d34c1e028098445a4d`；候选 strict 配置摘要：`ae9b7bfd39e053d7be3145aa0266f11855efd39814f54555b382e04bfbaca642`。

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
| consumer / error | topic_order_consumer | topics/4 · sort | 是 | 旧专栏集合排序保留于 Blog 扩展；宿主集合列表必须消费后才能执行迁移 |

### 8. `topics/5`

Profile：`topic`；候选路径：`/topic/stellar-1410`；已version 2原文保留：否；候选Markdown变化：否。

源摘要：`53562a7308e1a1b08c0237d88080fe2cd06728d36dbaaa0b38ad06b82d603386`；候选 strict 配置摘要：`84d968ed10a81e0bd99060c276f26956501d2ece94c9d03c7e69463d196abe3c`。

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
| consumer / error | topic_order_consumer | topics/5 · sort | 是 | 旧专栏集合排序保留于 Blog 扩展；宿主集合列表必须消费后才能执行迁移 |

### 9. `topics/6`

Profile：`topic`；候选路径：`/topic/stellar-1421`；已version 2原文保留：否；候选Markdown变化：否。

源摘要：`463df498df9f1726f36078ae5a7fc9be440e3a82907d9dc723fef63bf2261ddf`；候选 strict 配置摘要：`e35367e0476d22dd1c4e98003fc1782c39cbdd2c9ebfbb29a1cca549f86266b3`。

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
| consumer / error | topic_order_consumer | topics/6 · sort | 是 | 旧专栏集合排序保留于 Blog 扩展；宿主集合列表必须消费后才能执行迁移 |

### 10. `topics/7`

Profile：`topic`；候选路径：`/topic/stellar-验收`；已version 2原文保留：否；候选Markdown变化：否。

源摘要：`537b086d280953cfce9fc8c9efb6669380f17ceb461cdedd657c5275834ec38d`；候选 strict 配置摘要：`4e6d97c60808b355b2c9dfeda4214771a8516d462708028019b4fb2a8abbfa92`。

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
| consumer / error | topic_order_consumer | topics/7 · sort | 是 | 旧专栏集合排序保留于 Blog 扩展；宿主集合列表必须消费后才能执行迁移 |

### 11. `notebooks/2`

Profile：`notebook`；候选路径：`/notebooks/2`；已version 2原文保留：否；候选Markdown变化：否。

源摘要：`3afc535feebe5bc495db3646e48e0698faa60742a0a46c920f1b0245685099f7`；候选 strict 配置摘要：`32242a0d27c53df98096e1cf82414ec96e5f5516238cea01440f2ef85817877e`。

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

### 12. `posts/12`

Profile：`post`；候选路径：`/post/shortcode-可视化编辑测试`；已version 2原文保留：否；候选Markdown变化：是。

源摘要：`07436bd974c0c8738cd47d2fd629c007da006e597163f555d07fcd4a9290f967`；候选 strict 配置摘要：`9049e4ace40acd83d525d0c22624f762e32f2601402ec6c116ba8caadc23c670`。

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

### 13. `posts/11`

Profile：`topic`；候选路径：`/post/stellar-143-acceptance`；已version 2原文保留：否；候选Markdown变化：否。

源摘要：`3846826e6630ca1a04a1eaba5b31585f451328b8f8857495165aab6f466f38cb`；候选 strict 配置摘要：`8174493524c2769704c890d3a3a663a94864b310115a08402c236db18f87372c`。

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
| consumer / error | poster_consumer | posts/11 · poster | 是 | 旧 poster headline/topic/caption/color 完整保留为 Blog 扩展；尚需接通模板消费，不假称视觉保留 |

### 14. `posts/13`

Profile：`topic`；候选路径：`/post/stellar-144-acceptance`；已version 2原文保留：否；候选Markdown变化：否。

源摘要：`99e5f54182d54ee55f68d1e6f752daee0c113e15623cede0608cf3e69a06ca93`；候选 strict 配置摘要：`ed70a4c15a90f0f3caa10818c105a52a662d6b07dcd44f80723bd6552ed68a56`。

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
| consumer / error | poster_consumer | posts/13 · poster | 是 | 旧 poster headline/topic/caption/color 完整保留为 Blog 扩展；尚需接通模板消费，不假称视觉保留 |

### 15. `posts/5`

Profile：`topic`；候选路径：`/post/140-reading-time`；已version 2原文保留：否；候选Markdown变化：否。

源摘要：`9456c67185bb93442afa04daa48ade508dcaeefdfcfb6896471c57695ae8a37b`；候选 strict 配置摘要：`511da41eb06a15f3aa4c9c0e9982d645edbc5f44e5f12165f4c2252e55ca3652`。

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
| consumer / error | poster_consumer | posts/5 · poster | 是 | 旧 poster headline/topic/caption/color 完整保留为 Blog 扩展；尚需接通模板消费，不假称视觉保留 |

### 16. `posts/1`

Profile：`topic`；候选路径：`/post/140-card-tags`；已version 2原文保留：否；候选Markdown变化：是。

源摘要：`7103d1978aa1e1481a765e9b5a838fbf7a83c0060f398577ce726b52b15063aa`；候选 strict 配置摘要：`8fc9a3958c5b1a1debeca7ebfe6262c274c6533d9b0c0e6b77499d24fabb1072`。

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

### 17. `posts/4`

Profile：`topic`；候选路径：`/post/140-navigation`；已version 2原文保留：否；候选Markdown变化：是。

源摘要：`cf59a4f84c6b33736ad1d2648925a93bfa17beac3a6071755847e64af3f88740`；候选 strict 配置摘要：`2cc99d0e7472230d85a31875c6066c749879a535ea577cffcdb98dbe63e069a8`。

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

### 18. `posts/6`

Profile：`topic`；候选路径：`/post/140-table-styles`；已version 2原文保留：否；候选Markdown变化：是。

源摘要：`f6d7e490d56c805caf9d77e4205bfb9228a2c75f93c3bdb9ee7626e625e98310`；候选 strict 配置摘要：`6c92fdf4704276a7cf45c4fb3eef36c2945c7383a20d7c0f3bdc446950badcf6`。

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

### 19. `posts/2`

Profile：`topic`；候选路径：`/post/140-draft`；已version 2原文保留：否；候选Markdown变化：是。

源摘要：`bff622d34294bc7f52c68e6012b1bafcede06c341374b1da0260b04b5571a544`；候选 strict 配置摘要：`c08bbf6c891d4663ad78decdc631a7402e0f13f8a8230ce0ba3680affb59e9b6`。

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

### 20. `posts/3`

Profile：`topic`；候选路径：`/post/140-md-wrapper`；已version 2原文保留：否；候选Markdown变化：否。

源摘要：`675b73062193859773f7072597f3b551e1985e2622cb87bbb2b5e7deae2a3249`；候选 strict 配置摘要：`58c672db3e9865988e2a53dd716cb2880057a654b14723f3fdf88e52ab796bfe`。

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

### 21. `posts/8`

Profile：`topic`；候选路径：`/post/141-stellar-141`；已version 2原文保留：否；候选Markdown变化：否。

源摘要：`fc5658c8474405482178b860d0b314d6032b25df7cb952e7bcc0a373c8ae3c03`；候选 strict 配置摘要：`e4c42c0e37d73a9f93268cbe1c684ce38866a243f27641877408498766f9bf3d`。

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

### 22. `posts/9`

Profile：`topic`；候选路径：`/post/141-feature-playground`；已version 2原文保留：否；候选Markdown变化：否。

源摘要：`5aa082ea46766b9874b169052c2e73b4aa1c66899fc626ec6547f79348836f49`；候选 strict 配置摘要：`ed2c25f77ad00be88f42a74df1df35845ceb9834d2d2649f328f41ddd8b25519`。

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

### 23. `posts/10`

Profile：`topic`；候选路径：`/post/142-stellar-142`；已version 2原文保留：否；候选Markdown变化：否。

源摘要：`9ed8c5427f4ce2cb8de7308ff7b740d86cef18c277c45da65f8e5ff036ce972c`；候选 strict 配置摘要：`b775867d352d86bbf3ec9f31ab097e735035828cc1e47c4249d1776e0cbc9686`。

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

### 24. `pages/2`

Profile：`page`；候选路径：`/notes`；已version 2原文保留：否；候选Markdown变化：是。

源摘要：`b8d76bcea96500010975a170aff16399462be495d49621dc9f0ec736330cb081`；候选 strict 配置摘要：`429fb28747cf6131eb8aea474d77a6a097ed2679947d0ab96b6b4e9cb33867eb`。

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

### 25. `pages/5`

Profile：`page`；候选路径：`/140-test-page`；已version 2原文保留：否；候选Markdown变化：否。

源摘要：`ca23d17f67c616b88644a5a2947c681037efbd345f5da4844f3332c970efb784`；候选 strict 配置摘要：`dc0202b7dfdbd22581162a02754966e11866896fa6d15d1f87e92b8d390709db`。

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

### 26. `pages/1`

Profile：`page`；候选路径：`/friends`；已version 2原文保留：否；候选Markdown变化：否。

源摘要：`84e6fd38275ad32cb7425e4cf42fef2bd281a70a5b0928070b7dbb8e56c23d49`；候选 strict 配置摘要：`c4e40dd2d88ac4782d4f13fc7b4d89cfe6c6a2139a2250d5b6fa02df4f9ac9bd`。

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

### 27. `pages/3`

Profile：`page`；候选路径：`/links`；已version 2原文保留：否；候选Markdown变化：是。

源摘要：`7b3d84970bf1e903300e62935e09cc61514c48a662ec6f5877782f24c1db51e0`；候选 strict 配置摘要：`c0f5273d725dec311c2feed15cdcff52ed0b8f5b86cf1c3fce8afd1edc6b7682`。

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

### 28. `wikidocs/1`

Profile：`wiki`；候选路径：`/wiki/go-blog/acceptance`；已version 2原文保留：否；候选Markdown变化：否。

源摘要：`d9dcf26cbbf147f9a74a4a59b3fa649e49ea82480bc7fea40a732aac122b8247`；候选 strict 配置摘要：`05f8fb0d1b88870f6af8b4042ff2c8d92ae72da56081b9bd21c9c84ddb9e4063`。

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

Profile：`wiki`；候选路径：`/wiki/go-blog/responsive`；已version 2原文保留：否；候选Markdown变化：否。

源摘要：`b6470a61d45a43097befc00fead3f647b1eabbad7f96b5ec017bd6ac80008467`；候选 strict 配置摘要：`cc8eced065a52897f33032b1c035ad1a4b0ca75454f0434112aaba9a4fa7e427`。

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

Profile：`notebook`；候选路径：``；已version 2原文保留：否；候选Markdown变化：否。

源摘要：`20361ee253d7549202371e122a113dbc8056d450828a45a31830765a86c0e472`；候选 strict 配置摘要：`da0f493ad6b27cb38bbad1188ee113851a0805aa2c038c422392cbea561771d2`。

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

Profile：`topic`；候选路径：`/post/stellar-143-acceptance`；已version 2原文保留：否；候选Markdown变化：否。

源摘要：`260ee4414f652b8e10656c3968ad8a58895d34252d4486d4a55dd9a454dd255d`；候选 strict 配置摘要：`dee29cceffdd2f2456b8182ef91eb434c03ee9e0e663fbb19b8ca53acc2edbf7`。

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
| consumer / error | poster_consumer | content/posts/stellar-143-acceptance.md · poster_caption | 是 | 旧独立 poster 字段完整保留于 Blog 扩展，尚需接通消费 |
| consumer / error | poster_consumer | content/posts/stellar-143-acceptance.md · poster_color | 是 | 旧独立 poster 字段完整保留于 Blog 扩展，尚需接通消费 |
| consumer / error | poster_consumer | content/posts/stellar-143-acceptance.md · poster_headline | 是 | 旧独立 poster 字段完整保留于 Blog 扩展，尚需接通消费 |
| consumer / error | poster_consumer | content/posts/stellar-143-acceptance.md · poster_topic | 是 | 旧独立 poster 字段完整保留于 Blog 扩展，尚需接通消费 |
| structure / error | file_host_fields | content/posts/stellar-143-acceptance.md · status | 是 | 发布状态保留于 blog.status；须验证宿主导入保存该状态，不得把草稿隐式发布 |

### 32. `content/posts/stellar-144-acceptance.md`

Profile：`topic`；候选路径：`/post/stellar-144-acceptance`；已version 2原文保留：否；候选Markdown变化：否。

源摘要：`dc2a829ac7483466d6f0165c6be42339765bdb6f73f697ab5c05b79127b4205d`；候选 strict 配置摘要：`9be36cce609b6e378471e1b8075e30c3069696cc42c25520ed7674574e992f78`。

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
| consumer / error | poster_consumer | content/posts/stellar-144-acceptance.md · poster_caption | 是 | 旧独立 poster 字段完整保留于 Blog 扩展，尚需接通消费 |
| consumer / error | poster_consumer | content/posts/stellar-144-acceptance.md · poster_color | 是 | 旧独立 poster 字段完整保留于 Blog 扩展，尚需接通消费 |
| consumer / error | poster_consumer | content/posts/stellar-144-acceptance.md · poster_headline | 是 | 旧独立 poster 字段完整保留于 Blog 扩展，尚需接通消费 |
| consumer / error | poster_consumer | content/posts/stellar-144-acceptance.md · poster_topic | 是 | 旧独立 poster 字段完整保留于 Blog 扩展，尚需接通消费 |
| structure / error | file_host_fields | content/posts/stellar-144-acceptance.md · status | 是 | 发布状态保留于 blog.status；须验证宿主导入保存该状态，不得把草稿隐式发布 |

## 本次验证与尚未执行

- `go test ./internal/stellarmigration -run TestTrackedStatusUsesHostImportContract -count=1` 通过：draft/published 映射到 blog.status、序列化保持状态、上游 schema不含status，并保留执行阻断。
- `go test -race ./internal/stellarmigration -count=1` 通过：隔离夹具、只读源、strict校验、完整候选、评论变化源失效、正文/Markdown保护、私密权限、显式capability门槛。常规测试未访问真实数据。
- 本次另用显式只读helper读取真实源，生成新五份artifact并核对权限/完整性/历史一致性；没有运行真实迁移。
- OKR精确默认/自定义反例、15个消费者门槛、false/null/empty保真、SEO六项canonical与独立默认文案、未知schema/政策不能由capability解除均通过全包race测试；隔离Notebook/Note转换、rollback和取消证明已通过。
- 未执行：Apply、schema写入、真实数据回填、真实迁移后的Console编辑重开/前台/搜索/评论验收。实现者的消费者测试与主 agent的新浏览器验收仍须单列，不使用历史证据充当本次通过。

## 执行前不可绕过的检查

1. 实施者闭合所有已承诺消费者与导入/schema接线；主 agent 明确实测 capabilities，重新生成全部报告，不能手改报告诊断。
2. 将上表真正剩余选择与本地受保护的逐记录候选一并交用户审查；没有选择的行继续阻断。确认编辑器单name不等于确认真实取值或迁移。
3. 真实执行前重新比较源Snapshot digest、DB/活跃WAL及tracked/system源；任何变化重新生成并审核，不能只比较DB主文件hash。
4. 单独设计并验证备份、schema顺序、完整候选应用、幂等/失败原子性/回退；隔离旧库与新库证明ID/关系/正文/草稿/评论和权限不丢失，才能要求真实迁移授权。
5. 用户明确批准真实迁移后才写真实源；执行完成再核真实用户路径，不能以本只读报告称“已迁移”或“已验收”。
