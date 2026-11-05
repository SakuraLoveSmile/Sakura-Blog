# BLG-20261104-01：移除已被原生实现替代的第三方服务

<!-- 状态词只用 未开始 / 实现中 / 待体验 / 已验收；已验收须用户明确确认。 -->

- 项目 / 任务编号：Blog / BLG-20261104-01
- 状态：已验收（用户 2026-11-04 确认“可以了 先这样”，随后随 v2.0.7 发布）
- 创建日期（Asia/Taipei）：2026-11-04
- 叠加关系：本任务叠在未提交的 BLG-20261103-01（系统设置页 IA）工作树上做，沿用同一个 worktree 与分支；两项都未提交、未合并。
- 主仓库 / 当前工作目录：`/Users/sakurasep/Documents/Code/Project/Personal Project/Blog`（根 `dev`，保留用户原有未提交改动，不触碰）/ `/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261103-feat-blg-20261103-01-settings-ia`
- 分支 / 合入目标：`codex/feat/20261103-blg-20261103-01-settings-ia`（基线 `ce8d7e6c`）/ 本地 `dev`（未获授权前不合并、不推送）
- 权威记录：本文件；设置页视觉与 IA 验收仍见 `docs/plans/settings-ia.md` 与 `docs/baselines/settings-ia/README.md`

## 目标与范围

用户在评审系统设置页时确认：评论、搜索、评分/投票、站点统计都已原生实现，批准「先去掉可以直接去掉的」第三方服务，以简化设置页并停止对外部服务的依赖。

本任务去掉 5 项：

1. **第三方评论 6 家**（giscus / utterances / beaudar / twikoo / waline / artalk）
   - 设置页：删除「评论 → 第三方评论」段（`theme.comments` 不再出现在 12 页里）。
   - 运行时：删除 `sf_comments` 模板助手与 `comments()` 第三方容器渲染、`clientconfig` 的 `CommentsModel`/`resolveComments`/comments 扩展、`services.js` 里第三方评论容器的增强逻辑；删除第三方评论运行时资源（`assets/js/runtime/extensions/comments.js`、`assets/js/services/{giscus,waline,twikoo,artalk}_latest_comment.js`）。
   - 保留：原生评论（`internal/comments`，含审核、点赞/Boost、邮件通知、管理员徽章）、评论区邮件 SMTP 设置、原生「最新评论」widget（读本地数据）。
   - 说明：`assets/css/comments/{waline,twikoo,artalk}.css` 是上游 CSS 生成产物，`make css` / 漂移检查仍会编译它们；运行时不加载，故保留文件（详见「取舍」）。
2. **Algolia 搜索**
   - 设置页：搜索服务选项只剩「本地静态索引 / Go 服务搜索 / 关闭」，删除 Algolia 段与 `theme.search.algolia` 路径、guidance/效果文案。
   - 运行时：`clientconfig` 搜索只生成本地索引扩展；旧配置 `provider: algolia` 回退为本地索引；删除 `algolia-search.js`、`search.js` 的 algolia 分支、旧 `main.js` 的 algolia 过滤路径与 `regions.go` 的 `data-algolia-filter-path`。
3. **star_vote 评分 / 投票**
   - 设置页：外部服务不再展示 `rating` / `vote` 子项。
   - 运行时：v2 短代码 `{% rating %}` / `{% vote %}` 固定 `data-api="/api/rating"` / `/api/vote"`（站内原生 handler：`internal/store/rating_handler.go`、`vote_handler.go`）；`rating.js` 改为原生契约（GET `/api/rating?id=` → `{avg,count}`，POST `/api/rating` JSON `{id,score}`）；`vote.js` 原本就对齐 `/api/vote/info|update`，无需改动。
4. **站点信息 API（site_info）**
   - 设置页：外部服务不再展示 `site_info`。
   - 运行时：`base.html` 不再注入 `services.siteinfo.api`、不再按外部 API 加载 `siteinfo.js`；`clientconfig` 客户端数据中该字段固定为 null（兼容字段）；`services.js` 不再在渲染/外链增强后加载 siteinfo、不再写 `data-siteinfo-api`；`sites.js` 去掉 siteinfo 图标兜底；删除 `assets/js/services/siteinfo.js`（含 `blog/services` 变体）。
   - 保留：原生站点统计 widget（`SETTINGS.SiteStats`，服务端渲染文章/标签/评论/阅读量）。
   - 已知残留（惰性、无请求）：服务端短代码（`{% link %}`、`{% sites %}`、chat 链接卡）仍可能输出指向旧 siteInfo endpoint 的 `data-api` / `data-siteinfo-api` 属性；客户端已没有任何脚本消费它们，不会发起请求。彻底清理需同时改 oracle 对照与上游移植语义，留待后续。

明确不做：不改 `internal/stellarv2/schema.json` 与 `modules/stellar/clientconfig/resources.json`（上游 2.0 快照）；不删除 schema 节点（旧配置仍可解析、raw 原文仍可编辑和往返）；不动 GitHub 卡片 / marked / Gravatar（属于另一半「有原生基础但要收尾」清单）；不重拍 `docs/baselines/settings-ia` 截图（IA 任务记录，另行处理）。

## 已确认决定

- 2026-11-04 用户答复「可以 先去掉可以直接去掉的」，授权本 5 项移除。
- 本任务不新增提交、合并、推送、发布授权。

## 兼容与行为变化

- 旧配置里的 `theme.comments`、`theme.search.algolia`、`theme.services.site_info/rating/vote` 仍能被 schema 解析并保存在 raw 原文中；设置页不再展示，运行时不消费（Algolia 回退本地搜索；评分/投票改走站内接口）。
- 第三方评论不再渲染：即使旧配置保留 provider，也不会加载对应脚本；原生评论行为不变。
- 评分/投票的 DOM 从「未配置时禁用」变为「始终可用、指向站内接口」；`rating.js` 的读写契约随之改为站内 handler 形状。
- 未改任何数据表或迁移。

## 验收场景

1. 设置页（12 页）：评论页只有「原生评论」「评论邮件通知」；SEO 与搜索页搜索服务无 Algolia 选项/段落；扩展与服务 → 外部服务只有 5 个结构化子项（Gravatar 头像、Markdown 渲染、GitHub 贡献者、GitHub 卡片、GitHub 接口）。
2. 前台文章页：原生评论区正常；文章中的 `{% rating %}` / `{% vote %}` 渲染为可用控件且 `data-api` 指向 `/api/rating`、`/api/vote`，点击评分/投票调用站内接口成功。
3. 前台不请求 siteinfo 外部 API、不加载 algolia/siteinfo/第三方评论脚本；搜索使用本地索引。
4. raw 完整配置仍可编辑、保存；旧 `theme.search.algolia` 等值原样保留但不生效。
5. `make test`、`make build`、Playwright 相关规格通过。

## 执行与验证记录

| 日期 | 代码版本 | 环境 | 命令 | 结果 / 证据 |
| --- | --- | --- | --- | --- |
| 2026-11-04 | `ce8d7e6c` + 本任务未提交改动 | 本工作树 | `make test` | 退出 0：Console 453 tests / 0 fail（`vue-tsc -b` 通过）、assets node:test 225 pass / 0 fail、Go vet/staticcheck/全部 Go 测试通过。日志 `.tmp/make-test-removal2.log` |
| 2026-11-04 | 同上 | 本工作树 | `make build` | 退出 0：Console（含 /uc/）与 Go 二进制构建成功；随后 `dist` 入口为 `index-DxFbxItf.js` / `index-xGzz64GR.css`。`.tmp/make-build-removal2.log` |
| 2026-11-04 | 同上 | Playwright（8096，隔离库） | `pnpm run e2e`（全量） | **205 passed（10.0 分钟）**，退出 0。首轮曾 2 项失败并已修复：①Q2b 点击区用例原用已移除的 Algolia 段「脚本地址」，改为扩展页 `theme.services.markdown` 的同一控件；②Studio 预览评分 Fixture 改为原生 `{avg,count}` 形状（4.5/130），`rating.js` 新契约对应的 GET/POST 端点断言同步更新。`.tmp/e2e-removal.log`、`.tmp/e2e-targeted-removal.log` |
| 2026-11-04 | 同上 | 8098 隔离预览（重启于 18:10:46，晚于 `dist` 18:09） | 入口资源对比 + Playwright 冒烟 | 入口 chunk 与磁盘 `dist` 一致（`index-DxFbxItf.js`）；`/readyz` 200。设置页实测：评论页只剩「原生评论 / 评论邮件通知」；SEO 搜索只有 local/go/none；扩展页外部服务只有 5 个结构化子项，DOM 中无 `theme.comments`、`theme.search.algolia`、`theme.services.site_info/rating/vote`；0 条 console error。`.tmp/smoke-removal.mjs`、`.tmp/smoke-removal-extensions.png` |
| 2026-11-04（最终） | 同上（含 e2e 修复后的 preview fixture） | 本工作树 | `make test`（最终门禁） | 退出 0；Console 453 / assets 225 / Go 全过。`.tmp/make-test-final-removal.log` |
| 2026-11-04 | 同上 | 8098 隔离预览，公开文章页（临时文章，验证后删除） | 浏览器实测 `{% rating %}` / `{% vote %}` | `data-api` 分别为 `/api/rating`、`/api/vote`；点 5 星后均分 `(5.0)`、人数 1；点赞后 up=1；请求只命中站内 `/api/rating`、`/api/rating?id=…`、`/api/vote/info`、`/api/vote/update`；无 siteinfo/algolia/第三方评论请求；`.tmp/smoke-public.mjs`、`.tmp/smoke-removal-post.png` |

## 测试预期变更与依据

- `modules/console/tests`：provider 选项、去掉的页面分区与路径、外部服务 5 子项；schema 未变，raw 原文/大整数/通用 structured 编辑用例保留（见 Console 改动记录）。
- `modules/stellar/clientconfig/build_test.go`：官方 oracle 对已移除扩展做归一（comments 扩展、Algolia 搜索扩展、services 选择器中的评论容器/siteinfo 选择器、clientData 的 siteinfo API）；Algolia 旧配置断言回退本地；`TestStylesAndUseNumber` 用本地搜索大整数验证数值保留；评分/投票 oracle 上下文注入原生 endpoint。
- `internal/post`：评分/投票短代码固定原生接口；`rating.js` 契约变更；oracle 用原生 endpoint 对齐；并发隔离用例改用 `link` 的 siteInfo 地址作为每轮变化量（评分固定后不再承担该信号）。
- `modules/stellar/appearance`、`modules/stellar/web`、`modules/stellar/routes`、`modules/console/api`：删除为已移除服务服务的断言（Artalk 背景规则、giscus/siteinfo 注入、第三方评论 footer 链接、预览里的 rating 禁用态）。
- `modules/stellar/tests`：删除 `algolia-search.test.mjs`、`stellar-v2-siteinfo-content.test.mjs`；`css-runtime-v2`/`appearance-v2` 保留上游 CSS 产物检查；`stellar-v2-service-content.test.mjs` 去掉第三方最新评论用例，保留微博/RSS/动态服务边界；`data-cache.test.mjs` 去掉 siteinfo 服务。
- 没有降低强度的地方：原生评论、搜索、评分/投票、GitHub/OG/AI 等既有断言保留；被删断言对应的对象已不存在。

## 取舍与已知限制

1. **上游 schema 与资源快照不改**：`internal/stellarv2/schema.json` 是上游 2.0 生成物，`clientconfig/resources.json` 是客户端资源快照；产品层移除消费路径而不手改生成物，保持旧配置可解析。代价是 raw 原文仍能看到这些键、schema 仍接受它们。
2. **上游 CSS 生成物保留**：`css/comments/{waline,twikoo,artalk}.css` 仍由 `make css` 生成，漂移检查与 CSS 用例按 20 个产物校验；运行时已不加载。若将来彻底删上游评论支持，需同步改编译清单与测试。
3. **服务端惰性属性**：见「范围」第 4 条；不产生请求，属于兼容占位。
4. 评分/投票的 star_vote 配置值不会再影响渲染；旧配置保留在 raw 中作历史数据。
5. `docs/baselines/settings-ia/after-comments-*.png`、`after-extensions-*.png` 等 IA 截图与 `verification.json` 源码指纹早于本任务，已过期；本任务未重拍，IA 任务验收时需重拍/更新。
6. 未验证：Safari/Firefox、真机、读屏器（继承 IA 任务边界）；未在真实生产数据上运行（全部是隔离库与测试）。

## 运行资源与预览

- 8098 隔离预览（`-dev`，独立库/内容根 `.tmp/e2e-server.5oeGIY`）用于用户验收；重建 Console 后必须重启 Go 服务才会加载新入口，重启记录见下节。
- Playwright 使用 8096（独立库，`scripts/e2e/run-server.sh`），运行期间才存在。

## 下一步

1. 用户打开 8098 预览验收本任务与 IA 改动。
2. 提交 / 合并 / 重拍基线 / 清理 worktree 均需用户明确授权。

## 发布记录（v2.0.7，2026-11-04）

- 与设置页 IA 同一分支提交：`e8eb8a26`（feat，包含本任务的设置页与运行时移除）、`b0ae24cb`（fix，编辑器相关）；`make merge-check` 合入 `dev`，PR #46 → main `82611cfd`，PR #47 → release `69cad0f8`。
- tag `v2.0.7` 镜像 digest `sha256:839f243d…a615ed6`；生产升级后公网 `/post/who-am-i` 无 siteinfo/algolia/第三方评论请求、无第三方评论节点，原生评论区正常。
- 完整发布与生产验收见 `docs/plans/release-v2.0.7.md`。
