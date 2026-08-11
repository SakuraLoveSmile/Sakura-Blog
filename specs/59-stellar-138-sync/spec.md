# Spec 59 · Stellar 1.38.0 同步

- **基线**：`resource/hexo-theme-stellar` 原为 1.33.1+31（等效 1.34.0），本次切到 `1.38.0` tag；差 1.35/1.36/1.37/1.38（1.37 仅发版自动化，无功能影响）。
- **决策**：跟随上游移除 PJAX；新功能（首页评论区/mbti/data_cache）全量同步。
- **分支**：dev 直接推进；每阶段 `make build`+`make test` 全绿。

## 各阶段（T0–T7）

- **T0 资源与 CSS 管线**：checkout 1.38.0；`frontend/build/stylus/config.json` 增 `style.corner-shape`/`gradient.angle`、删 `smooth_scroll`；重编 `main.css`（含 corner-shape/菜单渐变/mbti 样式）；菜单结构对齐 1.38 列表化（`a.nav-item`+active-dot+grad-def，Go `gradsvg` 函数）。基线截图 `baselines/t0-*`。
- **T1 移除 PJAX**：删 `pjax.js`/`pjax-transitions.css`/其测试；移除全部 `pjax:complete` 监听（main/comments/local-search/tagcloud/reading/ai-summary/services/comments.html/content.html/base.html）；删 `plugins.pjax` 配置（Go/site.yml/console 卡片）；保留 flying-pages 预取。导航改整页刷新（`performance.navigation.type=navigate` 验证）。
- **T2 去 jQuery**：head 删 vendored jquery 与 `deps.jquery`；`utils` 增原生 `qs/qsa/dom`，`jq` 退化为直调，loading 处理改原生；services 以 1.38 为蓝本覆盖（ghinfo/timeline/memos/rss/contributors/friends/friends_and_posts/sites/mdrender），保留自建 rating/vote（本地 /api）；timeline 重放 srcdoc origin 降级修复。ghrepo 无 jquery 回填 stars/forks 验证。
- **T3 配置移除**：删 `style.smooth_scroll` 全链路（Go/测试/site.example/console AppearanceTab）。
- **T4 配置形状**：`style.gradient.button→start`（+`angle`）全链路（Go/测试/console/example）。
- **T5 data_cache**：Go `DataCacheConfig`+ctx 注入（`ptrBool` 函数）；`utils.request` 同 URL 在途去重 + localStorage TTL 缓存优先。
- **T6 新特性**：mbti 标签（shortcode.go `renderMbti`，zh 标签硬编码，结构对齐 mbti.styl）；首页评论区（`site_tree.home.comments`，routes_home fillComments "/" + base.html index 分支挂载，默认关）。
- **T7 收尾**：CLAUDE.md 增 Spec 59 章节（PJAX/jQuery 章节标记历史）；回归截图 `baselines/t7-*`；`make test` 全绿。

## 验证

- 每阶段 `make build`+`make test` exit 0。
- 浏览器：无 jquery/pjax 请求；导航整页刷新；gh 卡片/评论/TOC/搜索/标签云正常；1.38 列表化菜单渲染正确（`baselines/t7-post-dark.png`）。

## Out of Scope

- 上游 1.37 发版自动化；Hexo 专属命令（new-note）。
- TOC 平滑滚动/scrollreveal 侧栏 reveal 等纯视觉细项（默认值已随配置/重编生效，未逐项对齐 JS）。
