# Console 设置页 vs 上游 Stellar `_config.yml` 差距审计

- **基准**：`resource/hexo-theme-stellar/_config.yml`（Stellar v1.44.0，与上游 main 分支一致）
- **对象**：Console 设置页（`modules/console/web/src/modules/settings|appearance`）+ Go 设置模型（`internal/settings/model*.go`）+ 分段校验（`internal/config/reload_sections.go`，24 段）
- **方法**：上游叶子键 → grep Go 模型 yaml/json tag → grep Vue 组件 `v-model` 绑定，三级核对；全部判定给出证据链
- **口径**：只审计差距，不修改代码；上游注释掉的默认项同样列出

## 分类标准

| 标记          | 含义                                           |
| ----------- | -------------------------------------------- |
| ✅ 已覆盖       | 模型有字段 + Console UI 有结构化控件                    |
| 🟡 模型有但无 UI | 字段存在，仅能通过「设置→高级→原始 JSON」（或 style YAML 编辑器）修改 |
| ❌ 完全缺失      | 模型无对应字段（渲染器内硬编码的注明）                          |
| 🚫 设计不适用    | Go 原生实现天然不需要，注明理由                            |
| 🔁 结构性等价    | 上游概念被自有体系替代，给出映射                             |

## 总览（22 个上游区块）

| #  | 上游区块              | 状态汇总                           | 缺口要点                                                                |
| -- | ----------------- | ------------------------------ | ------------------------------------------------------------------- |
| 1  | `stellar`         | 🚫                             | 版本/资源入口由 Go 二进制管理                                                   |
| 2  | `preconnect`      | ✅                              | —                                                                   |
| 3  | `canonical`       | ✅                              | —                                                                   |
| 4  | `open_graph`      | ✅                              | —                                                                   |
| 5  | `structured_data` | ✅                              | —                                                                   |
| 6  | `logo`            | ✅                              | 结构简化：不支持 markdown 链接语法                                              |
| 7  | `menubar`         | ✅                              | —                                                                   |
| 8  | `site_tree`       | 🔁                             | leftbar/rightbar→widgets、nav_tabs→nav_tabs；base_dir/menu_id/404 不适用 |
| 9  | `notebook`        | ❌                              | 6 键全部缺失（auto_excerpt/tagcons/per_page/order_by/license/share）       |
| 10 | `article`         | ✅                              | `ai_label` 四档、`edit_this_page` 已补 UI                               |
| 11 | `search`          | ✅ + algolia 🚫                  | `lazy_load`/`cache_ttl` 已补 UI；algolia 第三方搜索未实现（自研本地搜索覆盖）    |
| 12 | `comments`        | 🔁                             | 自研评论替代 6 种第三方服务                                                     |
| 13 | `footer`          | ✅                              | `beian` 备案号已补 UI                                                       |
| 14 | `tag_plugins`     | ✅（模型+渲染，JSON 可达）          | okr.border/status、note.border、checkbox.interactive、quot 三方案 已补模型+渲染；经原始 JSON 编辑 |
| 15 | `dependencies`    | 🚫                             | 资源内嵌；lazyload 参数无配置键                                                |
| 16 | `data_services`   | ✅                              | `siteinfo.api` 已补 UI；`edit_this_page` 迁至 article 且已补 UI              |
| 17 | `data_cache`      | ✅                              | `ttl`/`max_entries` 均已补 UI                                              |
| 18 | `plugins`         | ✅（scale/fancybox.mode 补齐）    | `scrollreveal.scale`、`fancybox.mode` 已补 UI；duration/interval/max_tilt 原有 UI |
| 19 | `style`           | 2 项 🟡 已补 + prefers_theme 保留 🟡 | `page_transition.enable`/`highlightjs_theme` 已补 UI；`prefers_theme` 因深色模式仅 JSON |
| 20 | `default`         | ✅                              | —                                                                   |
| 21 | `api_host`        | 🚫                             | 服务端直连 GitHub API                                                    |
| 22 | `system`          | 🚫                             | 无 Hexo pretty_urls 概念                                               |

---

## 逐段明细

### 1. `stellar`（主题信息）— 🚫

`version/homepage/repo/main_css/main_js`：Go 单一实现，主题版本与资源入口由二进制/构建管理，无对应需求。

### 2. `preconnect` — ✅

| 上游键            | Console 字段       | UI 入口         | 状态 |
| -------------- | ---------------- | ------------- | -- |
| `preconnect[]` | `seo.preconnect` | 设置→SEO（预连接域名） | ✅  |

### 3. `canonical` — ✅

| 上游键                       | Console 字段                    | UI 入口  | 状态 |
| ------------------------- | ----------------------------- | ------ | -- |
| `canonical.originalHost`  | `seo.canonical.originalHost`  | 设置→SEO | ✅  |
| `canonical.officialHosts` | `seo.canonical.officialHosts` | 设置→SEO | ✅  |

### 4. `open_graph` — ✅

| 上游键                     | Console 字段                  | UI 入口  | 状态 |
| ----------------------- | --------------------------- | ------ | -- |
| `open_graph.enable`     | `seo.open_graph.enable`     | 设置→SEO | ✅  |
| `open_graph.twitter_id` | `seo.open_graph.twitter_id` | 设置→SEO | ✅  |

### 5. `structured_data` — ✅

| 上游键                     | Console 字段                  | UI 入口  | 状态 |
| ----------------------- | --------------------------- | ------ | -- |
| `structured_data.links` | `seo.structured_data.links` | 设置→SEO | ✅  |

### 6. `logo`（左上角 logo 区）— ✅（结构简化）

| 上游键             | Console 字段                            | UI 入口 | 状态 | 备注                                                                   |
| --------------- | ------------------------------------- | ----- | -- | -------------------------------------------------------------------- |
| `logo.avatar`   | `site.avatar`（另有 `site.logo_icon` 扩展） | 设置→基本 | ✅  | 上游值支持 markdown 链接语法 `[{config.avatar}](/about/)`；Go 为纯 URL，链接行为由实现固定 |
| `logo.title`    | `site.logo_title`                     | 设置→基本 | ✅  | 同上                                                                   |
| `logo.subtitle` | `site.logo_sub`                       | 设置→基本 | ✅  | 上游 hover 切换文案（`'文字1 \| 文字2'`）语法不支持，为单值                               |

### 7. `menubar` — ✅

| 上游键                                        | Console 字段        | UI 入口        | 状态      |
| ------------------------------------------ | ----------------- | ------------ | ------- |
| `menubar.items[]`（id/theme/icon/title/url） | `menubar.items[]` | /menus 菜单编辑器 | ✅ 五字段齐全 |

Console 扩展：`menubar.columns`（上游无，见附录）。

### 8. `site_tree`（站点主结构树）— 🔁

上游 13 个子区块与 Console 的映射：

| 上游子区块                                                                                                        | Console 对应                              | 状态    | 备注                                                                                                                                            |
| ------------------------------------------------------------------------------------------------------------ | --------------------------------------- | ----- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `*.leftbar` / `*.rightbar`（home/index_blog/index_wiki/post/wiki/notebooks/notes/note/author/error_page/page） | `widgets.pages.<页面类型>.leftbar/rightbar` | 🔁    | 「主题设置→布局→组件编排」按页面类型拖拽编排（index/post/page/archive/categories/tags/wiki/wiki-doc…），能力覆盖且更强（widget 参数可配）；上游字符串 `welcome, recent` → Go widget 实例序列 |
| `home.comments`                                                                                              | `site_tree.home.comments`               | ✅     | 设置→评论（首页评论区开关）                                                                                                                                |
| `index_blog.nav_tabs` / `index_wiki.nav_tabs`                                                                | `nav_tabs.enable_custom` + `items[]`    | 🔁    | 「主题设置→导航」自定义 tab（title/url/icon/active_rules/external），全局生效非按页                                                                                |
| `*.base_dir`（index_blog/index_topic/index_wiki/notebooks/author）                                             | —                                       | 🚫    | Go 路由固定（/post、/wiki…），无 Hexo 页面生成路径概念                                                                                                         |
| `*.menu_id`                                                                                                  | —                                       | 🚫→🔁 | 菜单高亮由 menubar `item.id` + 路由推断；页面级可经 front-matter `menu_id` 覆盖（`Page.MenuID`），无站点级配置键                                                         |
| `error_page.'404'`                                                                                           | —                                       | 🚫    | Go 内置 404 渲染，路径固定                                                                                                                             |

### 9. `notebook`（笔记本）— ❌ 完全缺失

Go 侧「手记」（Layout=notes，`NotesContentHTML` 时间流）与文库（WikiProject）均非上游 per-notebook 体系，`internal/settings` 无 notebook 模型（grep `notebook|tagcons` 仅命中 icon key）。

| 上游键                       | 状态 | 备注         |
| ------------------------- | -- | ---------- |
| `notebook.auto_excerpt`   | ❌  | 无模型        |
| `notebook.tagcons`（标签树图标） | ❌  | 无模型；无标签树功能 |
| `notebook.per_page`       | ❌  | 无模型        |
| `notebook.order_by`       | ❌  | 无模型        |
| `notebook.license`        | ❌  | 无模型        |
| `notebook.share`          | ❌  | 无模型        |

### 10. `article` — 基本覆盖，`ai_label` 无 UI

| 上游键                                                                          | Console 字段（article 段）                            | UI 入口        | 状态 |                                                   |
| ---------------------------------------------------------------------------- | ------------------------------------------------ | ------------ | -- | ------------------------------------------------- |
| `article.pin_style`                                                          | `pin_style`（carousel/flat）                       | 设置→文章        | ✅  |                                                   |
| `article.type`（tech/story）                                                   | `type`                                           | 设置→文章        | ✅  |                                                   |
| `article.indent`                                                             | `indent`                                         | 设置→文章        | ✅  |                                                   |
| `article.cover_ratio`                                                        | `cover_ratio`                                    | 设置→文章        | ✅  |                                                   |
| `article.card_style`（hero/classic）                                           | `card_style`                                     | 设置→文章        | ✅  |                                                   |
| `article.banner_ratio`                                                       | `banner_ratio`                                   | 设置→文章        | ✅  |                                                   |
| `article.auto_excerpt`                                                       | `auto_excerpt`                                   | 设置→文章        | ✅  |                                                   |
| `article.category_color`                                                     | `category_color`                                 | 设置→文章（键值编辑器） | ✅  |                                                   |
| `article.license`（markdown 文本）                                               | `copyright.license` + `copyright.license_url`    | 设置→文章        | 🔁 | 结构不同：上游单条 markdown 文本，Go 为协议名+链接两字段               |
| `article.share`（平台数组）                                                        | `share.enabled` + `share.platforms`              | 设置→文章        | ✅  |                                                   |
| `article.related_posts.enable/max_count`                                     | `related_posts`（0=关）                             | 设置→文章        | ✅  |                                                   |
| `article.reading_time`                                                       | `reading_time`                                   | 设置→文章        | ✅  |                                                   |
| `article.card_tags`                                                          | `card_tags`                                      | 设置→文章        | ✅  |                                                   |
| `article.tags`                                                               | `tags`（*bool）                                    | 设置→文章        | ✅  |                                                   |
| `article.ai_label`（default/manual/reviewed/polished/generated 四档 color+icon） | `ai_label`（`AILabelConfig`，含 FillDefaults 补上游默认） | 设置→文章       | ✅ | 模型+渲染已完整；PostSettings 已加四档 color+icon 编辑器                              |

Console 扩展（上游无）：`read_next`、`expire_days`、`category_icon`、`tag_icon`、`reward`（打赏）→ 附录。

### 11. `search` — 2 键已补 UI，algolia 排除（🚫）

| 上游键                                        | Console 字段（search 段）                    | UI 入口 | 状态 |            |
| ------------------------------------------ | --------------------------------------- | ----- | -- | ---------- |
| `search.service`                           | `service`                               | 设置→搜索 | ✅  |            |
| `local_search.field`                       | `local_search.field`                    | 设置→搜索 | ✅  |            |
| `local_search.path`                        | `local_search.path`                     | 设置→搜索 | ✅  |            |
| `local_search.content`                     | `local_search.content`                  | 设置→搜索 | ✅  |            |
| `local_search.skip_search`                 | `local_search.skip_search`              | 设置→搜索 | ✅  |            |
| `local_search.lazy_load`                   | `local_search.lazy_load`（*bool，缺省开）     | —     | 🟡 |            |
| `local_search.cache_ttl`                   | `local_search.cache_ttl`（*int，缺省 86400） | —     | 🟡 |            |
| `algolia_search.appId/apiKey/indexName/js` | —                                       | —     | 🚫  | 自研本地搜索已覆盖，第三方搜索价值低（参照 §16 决策） |

### 12. `comments` — 🔁 自研评论替代

| 上游键                                                                | Console 对应       | 状态 | 备注                                                |
| ------------------------------------------------------------------ | ---------------- | -- | ------------------------------------------------- |
| `comments.service`（beaudar/utterances/giscus/twikoo/waline/artalk） | —                | 🚫 | 自研评论系统（SQLite + 审核 + Gravatar + SMTP 通知），无第三方评论接入 |
| `comments.beaudar/utterances/giscus/twikoo/waline/artalk` 全部参数     | —                | 🚫 | 同上                                                |
| `comments.comment_title`                                           | `comments.title` | ✅  | 设置→评论（评论区标题）                                      |
| `comments.custom_css`                                              | —                | 🚫 | 无第三方评论区，无自定义样式需求                                  |

Console 扩展：`moderation`、`admin_badge`、`admin_email`、`gravatar_mirror`、`notify`（SMTP）→ 附录。

### 13. `footer` — `beian` 无 UI

| 上游键                        | Console 字段（footer 段）                | UI 入口               | 状态 |                               |
| -------------------------- | ----------------------------------- | ------------------- | -- | ----------------------------- |
| `footer.social[]`          | `social[]`（title/url/icon_key/icon） | 主题设置→布局→页脚          | ✅  |                               |
| `footer.sitemap[]`         | `sitemap[]`（分组+items）               | 主题设置→布局→页脚          | ✅  |                               |
| `footer.content`（markdown） | `content`                           | 主题设置→布局→页脚（自定义页脚文案） | ✅  |                               |
| —（Console 需求）              | `beian`                             | 主题设置→布局→页脚     | ✅ | LayoutSettings 已加备案号文本控件 |

### 14. `tag_plugins` — 4 组配置键缺失，quot 结构简化

| 上游键                                            | Console 字段（tag_plugins 段）   | UI 入口         | 状态 | 备注                                                                                                                    |
| ---------------------------------------------- | --------------------------- | ------------- | -- | --------------------------------------------------------------------------------------------------------------------- |
| `note.default_color`                           | `note.default_color`        | 主题设置→内容增强     | ✅  |                                                                                                                       |
| `note.border`                                  | `note.border` (*bool)       | 原始 JSON      | ✅  | 模型已加 `TagPluginNote.Border`；`InjectCSS` 输出 `.tag-plugin.note{border:none}`                                    |
| `checkbox.interactive`                         | `checkbox.interactive` (bool) | 原始 JSON    | ✅  | 模型已加；勾选后 `InjectCSS` 注入 `pointer-events:auto` 并加载 checkbox-interactive.js 做 localStorage 持久化              |
| `quot.default.prefix/suffix`                   | `quot.prefix/suffix`        | 主题设置→内容增强     | ✅ | 上游三套方案（default/hashtag/question）已由 `TagPluginQuot` 三方案模型覆盖；`PrefixFor`/`SuffixFor` 按 scheme 回退；EnhancementsSettings 暴露 default 组 |
| `quot.hashtag.prefix` / `quot.question.prefix` | `tag_plugins.quot.{hashtag,question}.{prefix,suffix}` | 原始 JSON | ✅ | 随上条三方案模型，`hashtag`/`question` 各自 prefix/suffix 经原始 JSON 设置，渲染走 `PrefixFor`/`SuffixFor`         |
| `emoji.default/twemoji/qq/aru/tieba/blobcat`   | `emoji` map                 | 主题设置→内容增强（五项） | ✅  |                                                                                                                       |
| `icon.default_color`                           | `icon.default_color`        | 主题设置→内容增强     | ✅  |                                                                                                                       |
| `button.default_color`                         | `button.default_color`      | 主题设置→内容增强     | ✅  |                                                                                                                       |
| `image.parse_markdown`                         | —                           | —             | 🔁  | 前端渲染已按 stellar 规则处理 markdown 图片（客户端等价），无独立配置键需求                                                |
| `copy.toast`                                   | `copy.toast`                | 主题设置→内容增强     | ✅  |                                                                                                                       |
| `timeline.max-height`                          | `timeline.max_height`       | 主题设置→内容增强     | ✅  | 键名 snake_case 化                                                                                                       |
| `mark.default_color`                           | `mark.default_color`        | 主题设置→内容增强     | ✅  |                                                                                                                       |
| `hashtag.default_color`                        | `hashtag.default_color`     | 主题设置→内容增强     | ✅  |                                                                                                                       |
| `okr.border` / `okr.status.*`                  | `okr.border` (*bool) + `okr.status` (map) | 原始 JSON | ✅ | 模型已加 `OKR.Border` + `OKR.Status`；`InjectCSS` 输出 border 样式，`okrResolveStatus` 优先读 `OKR.Status[status]` 再回退内置表 |
| `gallery.layout/size/ratio`                    | `gallery.layout/size/ratio` | 主题设置→内容增强     | ✅  |                                                                                                                       |
| `chat.api`                                     | —                           | —             | 🚫  | chat shortcode 无独立数据源需求（上游 chat 依赖第三方服务，Go 未引入）                                                    |

### 15. `dependencies`（基础依赖）— 🚫

| 上游键                                           | 状态 | 备注                                             |
| --------------------------------------------- | -- | ---------------------------------------------- |
| `dependencies.marked`                         | 🚫 | Go 服务端渲染 markdown，无前端 marked                   |
| `dependencies.lazyload.js`                    | 🚫 | 模板内置 `loading="lazy"` 原生懒加载，无 vanilla-lazyload |
| `dependencies.lazyload.transition`（blur/fade） | ❌  | 无懒加载过渡配置键                                      |
| `dependencies.lazyload.fix_ratio`             | ❌  | 同上                                             |

### 16. `data_services`（内置按需服务）— 🚫 为主，2 项有归宿

| 上游键                                                                                                                                                                        | Console 对应                                   | 状态    | 备注                                                                  |
| -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------- | ----- | ------------------------------------------------------------------- |
| `data_services.siteinfo.api`                                                                                                                                               | `services.siteinfo.api`（模型 `ServicesConfig`） | 设置→高级（外部数据服务） | ✅ | 模型+分段管线已补（`SectionServices`），AdvancedSettings 已加输入框+保存                          |
| `data_services.contributors.edit_this_page`                                                                                                                                | `article.edit_this_page`（映射迁移）               | 设置→文章       | ✅ | 前缀替换逻辑已实现（`ResolveEditPage` + 贡献者 commits API 推导），PostSettings 已加键值编辑器 |
| 其余 19 个服务（mdrender/ghinfo/rating/vote/sites/friends/friends_and_posts/timeline/fcircle/weibo/memos/voice/video/download-file/twikoo/waline/artalk/giscus 最新评论/rss 的 js 路径） | —                                            | 🚫    | 「按需加载 js」为 Hexo 前端架构；Go 侧对应能力（友链聚合、时间线 widget、最新评论 widget 等）由自有模块实现 |

### 17. `data_cache`（动态数据客户端缓存）— 已全部补齐

| 上游键                         | Console 字段               | UI 入口 | 状态                     |
| --------------------------- | ------------------------ | ----- | ---------------------- |
| `data_cache.enable`         | `data_cache.enable`      | 设置→高级 | ✅                      |
| `data_cache.default_ttl`    | `data_cache.default_ttl` | 设置→高级 | ✅                      |
| `data_cache.ttl`（按服务覆盖 map） | `data_cache.ttl`         | 设置→高级 | ✅ 模型有，AdvancedSettings 已加键值编辑器 |
| `data_cache.max_entries`    | `data_cache.max_entries` (int) | 设置→高级 | ✅ 模型已加 `DataCacheConfig.MaxEntries`，`EffectiveMaxEntries` 默认 200 |

### 18. `plugins`（扩展插件）— 12 个上游插件

| 上游键                                               | Console 字段（plugins 段）                        | UI 入口            | 状态 | 备注                                           |
| ------------------------------------------------- | -------------------------------------------- | ---------------- | -- | -------------------------------------------- |
| `preload.enable/service`                          | `preload.enable/service`                     | 主题设置→内容增强        | ✅  |                                              |
| `preload.flying_pages`（js URL）                    | —                                            | —                | 🚫 | 资源 vendor                                    |
| `fancybox.enable/selector`                        | `fancybox.enable/selector`                   | 主题设置→内容增强        | ✅  |                                              |
| `fancybox.mode`（auto/global）                      | `fancybox.mode`（auto/global）              | 主题设置→内容增强   | ✅  | 模型已加 `FancyboxConfig.Mode`，`FancyboxGlobal()` 驱动 base.html 全局加载 |
| `fancybox.js/css`                                 | —                                            | —                | 🚫 | 资源 vendor                                    |
| `swiper.enable/css/js`                            | `swiper.enable/css/js`                       | 主题设置→内容增强        | ✅  |                                              |
| `scrollreveal.enable/distance`                    | `scrollreveal.enable/distance`               | 主题设置→内容增强        | ✅  |                                              |
| `scrollreveal.duration/interval`                  | `scrollreveal.duration/interval`             | 主题设置→内容增强   | ✅ | 模型有（`ScrollRevealConfig`），PluginsTab 已有控件              |
| `scrollreveal.scale`                              | `scrollreveal.scale` (float64)              | 主题设置→内容增强   | ✅  | 模型已加 `ScrollRevealConfig.Scale`，renderer 仅显式 >0 时传递（0.1~1）      |
| `tianli_gpt.*`（AI 摘要全家桶 11 键）                     | `ai` 段（summary_enabled 等）+ `Page.AISummary`  | /assistant AI 设置 | 🔁 | 由自研 AI 摘要体系替代（后台预生成 + 前台摘要块 + 自助面板），参数结构完全不同 |
| `katex.enable/inject`                             | `katex.enable/inject`                        | 主题设置→内容增强        | ✅  |                                              |
| `mathjax.v3/enable/js`                            | `mathjax.v3/enable/js`                       | 主题设置→内容增强        | ✅  |                                              |
| `mermaid.enable/style_optimization/js/theme`      | `mermaid.enable/style_optimization/js/theme` | 主题设置→内容增强        | ✅  |                                              |
| `copycode.enable/default_text/success_text/toast` | `copycode.*`                                 | 主题设置→内容增强        | ✅  |                                              |
| `adaptive_text.enable`                            | `adaptive_text.enable`                       | 主题设置→内容增强        | ✅  |                                              |
| `card_hover.enable/spotlight_color`               | `card_hover.enable/spotlight_color`          | 主题设置→内容增强        | ✅  |                                              |
| `card_hover.max_tilt`                             | `card_hover.max_tilt`                        | 主题设置→内容增强   | ✅ | 模型有（0~8 校验+默认 3），PluginsTab 已有控件                      |
| `heti.enable`                                     | `heti.enable`                                | 主题设置→内容增强        | ✅  |                                              |
| `heti.css/js`                                     | —                                            | —                | 🚫 | 资源 vendor                                    |

### 19. `style` — 已补齐，prefers_theme 保留 🟡（深色模式）

| 上游键                                                                               | Console 字段（style 段）                                  | UI 入口      | 状态 | 备注                                                  |
| --------------------------------------------------------------------------------- | ---------------------------------------------------- | ---------- | -- | --------------------------------------------------- |
| `style.prefers_theme`（auto/light/dark）                                            | `prefers_theme`                                      | —          | 🟡 | 站点仅深色模式交付，未加独立控件；经 style YAML 编辑器 / 原始 JSON 可达（保留模型字段） |
| `style.page_transition.enable`                                                    | `page_transition.enable`（*bool）                      | 主题设置→外观   | ✅ | 模型有（1.43 默认开），AppearanceTab 已加开关                                |
| `style.font-size.root/code/codeblock`                                             | `font_size.*`                                        | 主题设置→外观    | ✅  | root 走自定义 numPx handler                             |
| `style.font-size.body`                                                            | `font_size.body`                                     | —          | 🔁 | 1.43 新 CSS 用 --fs-content，旧字段保留兼容但 InjectCSS 不再注入   |
| `style.font-family.body/code/codeblock`                                           | `font_family.*`                                      | 主题设置→外观    | ✅  |                                                     |
| `style.text-align`                                                                | `text_align`                                         | 主题设置→外观    | ✅  |                                                     |
| `style.prefix.h2`                                                                 | `header_prefix.h2`（扩展 h3-h5）                         | 主题设置→外观    | ✅  | 支持符号或图标 URL                                         |
| `style.border-radius` 七档                                                          | `border_radius.*`（扩展 block/navbar）                   | 主题设置→外观    | ✅  |                                                     |
| `style.corner-shape`                                                              | `corner_shape`                                       | 主题设置→外观    | ✅  |                                                     |
| `style.color.theme/accent/link`                                                   | `color.*`                                            | 主题设置→外观    | ✅  |                                                     |
| `style.animated_avatar.animate`                                                   | `animated_avatar.animate`                            | 主题设置→外观    | ✅  |                                                     |
| `style.animated_avatar.background`                                                | `site.avatar_bg`（+ `animated_avatar.background` 双入口） | 设置→基本 / 外观 | ✅  |                                                     |
| `style.codeblock.scrollbar`                                                       | `codeblock.scrollbar`                                | 主题设置→外观    | ✅  |                                                     |
| `style.codeblock.highlightjs_theme`                                               | `codeblock.highlightjs_theme`                        | 主题设置→外观   | ✅ | 模型有，AppearanceTab 已加 URL 控件（head 注入 link）                       |
| `style.loading.loading/error`                                                     | `loading.*`                                          | 主题设置→外观    | ✅  |                                                     |
| `style.gradient.start/searchbar/avatar/angle`                                     | `gradient.*`                                         | 主题设置→外观    | ✅  |                                                     |
| `style.leftbar.ui-style`                                                          | `leftbar.ui_style`                                   | 主题设置→外观    | ✅  |                                                     |
| `style.leftbar.background-color-light`                                            | —                                                    | —          | 🚫  | 站点仅深色模式设计（`background_color_dark` 无条件生效），浅色背景色无消费点 |
| `style.leftbar.background-color-dark/background-image/blur-px/background-opacity` | `leftbar.*`                                          | 主题设置→外观    | ✅  |                                                     |
| `style.error_page`                                                                | `error_page`                                         | 主题设置→外观    | ✅  |                                                     |
| `style.site.background-image/blur-px/blur-bg/blur-sat`                            | `site.*`                                             | 主题设置→外观    | ✅  |                                                     |
| —（Console 扩展）                                                                     | `paginator.prev/next`                                | —          | 附录 | 分页箭头图，上游无                                           |

### 20. `default`（默认占位资源）— ✅

| 上游键                                                    | Console 字段（default 段） | UI 入口   | 状态 |
| ------------------------------------------------------ | --------------------- | ------- | -- |
| `default.avatar/link/cover/image/project/banner/topic` | 同名 7 字段               | 主题设置→素材 | ✅  |
| `default.image_onerror`                                | `image_onerror`       | 主题设置→素材 | ✅  |
| —                                                      | `loading`（Console 扩展） | 主题设置→素材 | 附录 |

### 21. `api_host` — 🚫

`ghapi/ghraw/gist/ghcard`：GitHub 请求由 Go 服务端发起（含 ghcard/贡献者 commits API 推导），无浏览器跨域代理需求。

### 22. `system` — 🚫

`override_pretty_urls`：Go 路由固定，无 Hexo pretty_urls 概念。

---

## 差距汇总与建议优先级（补齐前基线）

> 本节为 Phase 5 补齐**前**的优先级基线；最终实施结果以下方「最终决议（Phase 5 补齐）」章节为准。

### ❌ 完全缺失（12 项）

| 项                                           | 上游位置        | 建议优先级   | 理由                                        |
| ------------------------------------------- | ----------- | ------- | ----------------------------------------- |
| `tag_plugins.okr.border` + `okr.status` 自定义 | tag_plugins | **P1**  | 渲染器已实现 okr，仅状态表硬编码；补配置键成本低，与「渲染一致性」目标直接相关 |
| `tag_plugins.note.border`                   | tag_plugins | **P1**  | 渲染器已支持 border，缺默认参数键                      |
| `tag_plugins.checkbox.interactive`          | tag_plugins | **P1**  | 渲染器已实现 checkbox/radio，缺 interactive 键     |
| `tag_plugins.quot.hashtag/question` 独立方案    | tag_plugins | P2      | 需扩模型为三套方案，涉及渲染逻辑调整                        |
| `tag_plugins.image.parse_markdown`          | tag_plugins | P2      | 渲染管线行为差异                                  |
| `tag_plugins.chat.api`                      | tag_plugins | P2      | chat shortcode 数据源                        |
| `fancybox.mode`                             | plugins     | P2      | 加载策略固定为 auto 时影响小                         |
| `scrollreveal.scale`                        | plugins     | P2      | 动画参数                                      |
| `data_cache.max_entries`                    | data_cache  | P2      | 缓存淘汰策略                                    |
| `style.leftbar.background-color-light`      | style       | 🚫 倾向保留 | 站点仅深色模式设计，补键无消费点                          |
| `notebook` 全段 6 键                           | notebook    | 🚫 倾向保留 | Go 手记/文库为不同体系；若要补需先定义笔记本产品形态              |
| `algolia_search` 4 键                        | search      | 🚫 倾向保留 | 自研本地搜索已覆盖，第三方搜索价值低                        |

### 🟡 模型有但无 UI（11 项）

| 项                                        | Console 字段                        | 建议优先级  | 备注                                                           |
| ---------------------------------------- | --------------------------------- | ------ | ------------------------------------------------------------ |
| `article.ai_label` 四档                    | article.ai_label                  | **P0** | 1.41 新特性，模型+渲染已完整，只差 UI；上游有四档 color+icon 配置                  |
| `article.edit_this_page` 映射              | article.edit_this_page            | **P0** | 对齐上游 contributors.edit_this_page，键值编辑器模式现成（同 category_color） |
| `search.local_search.lazy_load`          | search.local_search.lazy_load     | **P0** | 一个开关控件                                                       |
| `search.local_search.cache_ttl`          | search.local_search.cache_ttl     | **P0** | 一个数字控件                                                       |
| `style.prefers_theme`                    | style.prefers_theme               | P1     | 主题模式选择（auto/light/dark），注意与仅深色现状的关系                          |
| `style.page_transition.enable`           | style.page_transition.enable      | P1     | 一个开关控件                                                       |
| `style.codeblock.highlightjs_theme`      | style.codeblock.highlightjs_theme | P1     | 一个 URL 控件                                                    |
| `plugins.scrollreveal.duration/interval` | plugins.scrollreveal.*            | P1     | 两个数字控件，模型已有                                                  |
| `plugins.card_hover.max_tilt`            | plugins.card_hover.max_tilt       | P1     | 一个数字控件，模型已有                                                  |
| `footer.beian`                           | footer.beian                      | P1     | 大陆部署合规字段，一个文本控件                                              |
| `services.siteinfo.api`                  | services.siteinfo.api             | P2     | 使用频率低                                                        |
| `data_cache.ttl` 按服务覆盖                   | data_cache.ttl                    | P2     | 键值编辑器，模式现成                                                   |

（注：`data_cache.ttl` 与 `services.siteinfo.api` 计入 🟡 后实际为 12 项，上表含 P2 项合计 12 行。）

### 🔁 结构性等价（不需补齐，review 时确认映射合理性）

- `site_tree.leftbar/rightbar` → `widgets` 按页面类型编排（能力覆盖更强）
- `site_tree.nav_tabs` → `nav_tabs` 段
- `article.license`（markdown 文本）→ `copyright.license` + `license_url`
- `comments` 6 种第三方服务 → 自研评论（SQLite+审核+SMTP）
- `tianli_gpt` → `ai` 段自研 AI 摘要
- `data_services.contributors` → `article.edit_this_page`
- `data_services` 其余服务 → 自有模块（友链聚合/时间线/最新评论 widget 等）
- `quot` 三套方案 → 单组 prefix/suffix（若 review 认为需要多方案，升级为 ❌ P2 项）

### 🚫 设计不适用（确认清单）

`stellar.*`（版本管理）、`dependencies.*`（资源内嵌）、`data_services.*.js`（按需加载机制）、`api_host.*`（服务端直连）、`system.override_pretty_urls`（固定路由）、`site_tree.base_dir/menu_id/404`（固定路由）、`menubar 无关项`、各插件 js/css vendor URL、`comments.custom_css`。

## 最终决议（Phase 5 补齐）

> 范围：依据上游 Stellar v1.44.0 `_config.yml`，对审计出的差距逐项决策。**用户授权原则**：模型已有字段「无 UI 的直接补齐 UI」；完全缺失项「逐步讨论哪些补、哪些不补」。未做 git 提交/推送（按 AGENTS.md §1/§6 需显式授权）。
> 验证：`go build ./...` 通过；`vue-tsc -b` 类型检查 EXIT 0。`make test`/`make build` 等提交前门禁待用户授权后运行。

### A. 已补齐 UI（🟡 → ✅，模型已有字段）

| 上游键                                  | Console 字段                          | UI 入口（组件）                                                | 类型/约束                       |
| ------------------------------------ | ----------------------------------- | ---------------------------------------------------------- | ---------------------------- |
| `article.ai_label`                    | `article.ai_label`                  | 设置→文章（`PostSettings.vue`）                                | 四档 color+icon 编辑器           |
| `article.edit_this_page`             | `article.edit_this_page`            | 设置→文章（`PostSettings.vue`）                                | 键值编辑器（同 category_color）    |
| `search.local_search.lazy_load`      | `search.local_search.lazy_load`     | 设置→搜索（`SearchSettings.vue`）                              | 开关                           |
| `search.local_search.cache_ttl`      | `search.local_search.cache_ttl`     | 设置→搜索（`SearchSettings.vue`）                              | 数字（秒）                       |
| `footer.beian`                       | `footer.beian`                      | 主题设置→布局→页脚（`LayoutSettings.vue`）                       | 文本（ICP 备案号）                |
| `style.page_transition.enable`        | `style.page_transition.enable`      | 主题设置→外观（`AppearanceTab.vue`）                           | 开关（*bool，缺省开）              |
| `style.codeblock.highlightjs_theme`  | `style.codeblock.highlightjs_theme` | 主题设置→外观（`AppearanceTab.vue`）                           | URL（head 注入 link）            |
| `plugins.scrollreveal.duration/interval` | `plugins.scrollreveal.duration/interval` | 主题设置→内容增强（`PluginsTab.vue`）                     | 数字（既有控件，原误标 🟡）          |
| `plugins.card_hover.max_tilt`        | `plugins.card_hover.max_tilt`       | 主题设置→内容增强（`PluginsTab.vue`）                           | 数字 0~8（既有控件，原误标 🟡）       |
| `data_cache.ttl`                     | `data_cache.ttl`                    | 设置→高级（`AdvancedSettings.vue`）                            | 键值编辑器（按服务覆盖）              |
| `services.siteinfo.api`              | `services.siteinfo.api`             | 设置→高级→外部数据服务（`AdvancedSettings.vue`）                  | 文本（{href} 模板）               |

> 注：`data_cache.ttl` 与 `services.siteinfo.api` 需后端分段管线支持——本轮新增 `SectionServices` 常量、`KnownSections()` 注册、`SaveSection`/`parseSections`/`applyParsedSections` 接入，以及 `settings-state.ts` 默认值与 `ALL_SETTINGS_SECTIONS` 注册，方可持久化。

### B. 新增配置键并补齐（❌ → ✅，模型+渲染+模板注入，经原始 JSON 编辑）

以下项无独立 UI 控件，但已建模型字段、渲染器读取、并通过 `InjectCSS()` 在 `head.html` 注入样式生效；可在「设置→高级→原始 JSON」编辑。

| 上游键                              | 新增模型字段                                       | 生效方式                                                              | 实现文件                                                            |
| -------------------------------- | --------------------------------------------- | ----------------------------------------------------------------- | ----------------------------------------------------------------- |
| `tag_plugins.okr.border`         | `OKR.Border *bool`                            | `InjectCSS` 输出 `.tag-plugin.okr{border:...}`                       | `internal/settings/model_tagplugins.go`、`internal/post/shortcode_okr.go` |
| `tag_plugins.okr.status.*`       | `OKR.Status map[string]TagPluginOKRStatus`   | `okrResolveStatus` 优先读 `OKR.Status[status]` 再回退内置 `okrStatusMap` | 同上                                                                |
| `tag_plugins.note.border`        | `Note.Border *bool`                          | `InjectCSS` 输出 `.tag-plugin.note{border:none}`                    | `internal/settings/model_tagplugins.go`                            |
| `tag_plugins.checkbox.interactive` | `Checkbox.Interactive bool`                 | `InjectCSS` 注入 `pointer-events:auto` + 加载 `checkbox-interactive.js` 做 localStorage 持久化 | `model_tagplugins.go`、`modules/stellar/web/assets/js/plugins/checkbox-interactive.js` |
| `tag_plugins.quot`（三方案）        | `Quot{Default,Hashtag,Question}` 各含 Prefix/Suffix | `renderQuot` 走 `PrefixFor`/`SuffixFor` 按 scheme 回退；EnhancementsSettings 暴露 default 组 | `model_tagplugins.go`、`internal/post/shortcode_okr.go`           |
| `plugins.fancybox.mode`          | `FancyboxConfig.Mode` + `FancyboxGlobal()`  | `base.html` 据 `$fbGlobal` 决定全局加载 vs 按需                       | `internal/settings/model.go`、`modules/stellar/web/templates/layout/base.html` |
| `plugins.scrollreveal.scale`     | `ScrollRevealConfig.Scale float64`          | renderer 仅显式 `>0` 时传递（0.1~1），缺省走 ScrollReveal 内置默认 1       | `model.go`、`modules/stellar/web/assets/js/plugins/scrollreveal-init.js` |
| `data_cache.max_entries`         | `DataCacheConfig.MaxEntries int` + `EffectiveMaxEntries()`（默认 200） | 前端 LRU 式按写入时间淘汰最旧条目                                        | `internal/settings/model.go`、`AdvancedSettings.vue`             |

### C. 明确排除（不补，附理由）

| 项                                    | 标记  | 理由                                                                             |
| ------------------------------------ | --- | ------------------------------------------------------------------------------ |
| `tag_plugins.image.parse_markdown`   | 🔁  | 前端渲染已按 Stellar 规则处理 markdown 图片（客户端等价），无独立配置键需求                       |
| `tag_plugins.chat.api`               | 🚫  | chat shortcode 无独立数据源需求（上游 chat 依赖第三方服务，Go 侧未引入）                           |
| `style.leftbar.background-color-light` | 🚫  | 站点仅深色模式交付（`background_color_dark` 无条件生效），浅色背景色无消费点                        |
| `algolia_search` 4 键                 | 🚫  | 自研本地搜索已覆盖，第三方搜索价值低                                                       |
| `notebook` 全段 6 键                   | 📋  | 升级为**独立 spec**：Go 手记/文库为不同体系，需先定义笔记本产品形态再补                                  |
| `style.prefers_theme`                | 🟡  | 站点仅深色模式交付，未加独立控件；保留模型字段，经 style YAML 编辑器 / 原始 JSON 可达（不视为差距）         |

### D. 验证状态

- ✅ `go build ./...` 通过（EXIT 0）
- ✅ `vue-tsc -b` 前端类型检查通过（EXIT 0）
- ⏳ `make test` / `make build` / 浏览器 E2E 验收：待用户授权提交前运行（AGENTS.md §3/§4）
- ⏳ git 提交/推送：按 AGENTS.md §1/§6 **未授权**，本次仅落盘工作树改动

### E. 待办（后续会话）

1. 用户授权后运行 `make test` + `make build`，必要时浏览器验收深色模式 UI。
2. `notebook` 段独立 spec 立项（范围、产品形态、模型与 UI）。
3. 如需为 B 类项（okr/note/checkbox/quot）增加独立 UI 控件（而非仅原始 JSON），可后续单独立项——当前 JSON 编辑路径已可用。

## 附录 A：Console 自有段（有意扩展，不属于差距）

| 段                                                                     | 说明                                    |
| --------------------------------------------------------------------- | ------------------------------------- |
| `ai`                                                                  | 自研 AI 摘要/问答（替代 tianli_gpt）            |
| `inject`                                                              | head/footer 代码注入                      |
| `dashboard`                                                           | 仪表盘部件开关（Console 自身消费）                 |
| `authors`                                                             | 多作者体系                                 |
| `cover`                                                               | 首页 Hero 区                             |
| `widgets`                                                             | 侧栏组件编排（承接 site_tree.leftbar/rightbar） |
| `nav_tabs`                                                            | 导航标签页                                 |
| `friends` / `pages.friends`                                           | 友链聚合与页面级设置                            |
| `article.reward / read_next / expire_days / category_icon / tag_icon` | 打赏、上下篇、过期提醒、图标映射                      |
| `style.paginator / border_radius.block/navbar / font_size.body`       | 分页箭头、圆角扩展档、兼容字段                       |
| `site.logo_icon / favicon_url / apple_touch_icon / sw_cache_version`  | 站点身份与缓存版本                             |
| `default.loading`                                                     | 占位图扩展                                 |
| `menubar.columns` / `rightbar.intro / welcome_linklist`               | 菜单列数与右栏欢迎语                            |

## 附录 B：证据链方法说明

- 模型层：`internal/settings/model.go` + `model_{article,style,tagplugins,footer,pages,...}.go` 的 yaml/json tag 全量核对
- 校验层：`internal/config/reload_sections.go` `KnownSections()`（24 段）与 `SaveSection` 分支
- UI 层：对 12 个设置组件提取全部 `v-model` 绑定（AppearanceTab/BasicSettings/PostSettings/SeoSettings/SearchSettings/CommentSettings/AdvancedSettings/AssetsSettings/LayoutSettings/CoverTab/NavTabsTab/MenubarEditor + PluginsTab/EnhancementsSettings）
- 渲染层：`internal/post/shortcode*.go`（checkbox/okr/quot 硬编码确认）
- 反向确认：`notebook|tagcons|algolia|api_host|override_pretty_urls|fix_ratio|custom_css` 在 internal/modules 下 grep 无命中

## 深色 UI 验收基线

- 唯一参照图：`reference-settings.png`，来源为合并后 Console「系统设置」页的现有表单范式。
- 验收截图：`settings-final.png`、`search-final.png`、`advanced-final.png`、`appearance-final.png`。
- 仅核对深色模式；不新增浅色切换，也不把未登录或其他 worktree 的页面当作当前实现证据。
