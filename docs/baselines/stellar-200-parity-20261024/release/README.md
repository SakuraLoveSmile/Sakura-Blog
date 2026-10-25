# Stellar 2.0 正式发布与生产核验

同属 `BLOG20261024-STELLAR`，权威记录在 `docs/stellar-200-full-port.md`。用户已明确授权完整 dev 候选的推送、PR 晋级、tag/双产物发布及服务器升级。

升级前 Console 实测版本为 `2.0.0-local.e2f8c664`，截图见 `before-console-version.jpg`。发布流水线、产物校验、备份/部署和升级后浏览器结果分别记录；未执行的项目保持未完成。私有 Compose、配置、数据库和用户文件的原始备份只保留服务器私有目录，不放入 Git。

2026-10-25 已部署 `v2.0.0`，本批状态为待体验。`public-http-verification.json` 是本次通过公开域名重新执行的 40 项检查；28 份静态资源可访问，其中 17 份前台资源 SHA-256 与 release 提交逐字匹配。`production-browser-verification.json` 记录线上桌面、390px 搜索/分页/设置入口、浅深配色与参照站的计算值，以及恢复原生视口和系统配色的结果。

服务器激活与最终只读复核见 `server-activation-passed.png`、`server-final-verification.png` 和 `server-production-verification.json`。39 张业务表、392 个保护文件保持；schema、业务列及行增删继续严格检查。订阅刷新会更新 `feed_articles.fetched_at`，仅该列的值不参加内容摘要，schema 和其它字段仍检查；原始失败、恢复与重试截图保留。

用户明确选择暂时关闭两项后台任务。配置与停用前备份逐叶比较，仅 `blog_extensions.ai.embedding_model` 和 `blog_extensions.friends.check_interval` 不同，见 `server-background-config-scope.png`。RAG 原有错误状态未修复，不自动恢复配置或触发可能计费的重建。完整配置备份及数据库/运行目录前像保留于服务器 `/home/ubuntu/blog-release-v2.0.0/`，不下载私密原文。

`after-production-home-light.jpg`、`after-production-home-dark.jpg` 为桌面视觉证据；`after-production-mobile-drawer.png`、`after-production-search-mobile.png`、`after-production-mobile-settings-dark.png` 使用 CDP 原始 390×844 视口截图。IAB 原生接口曾将手机画面缩小，该批中间截图已移除；本次没有真机验收。参照站的个人字体与本博客系统字体仍按原决定保留。

以下预演文件保留当时的失败现场；最终生产结果以上述激活和只读复核证据为准。

- `release-ci.json`：PR 与 tag workflow 的逐作业结论。
- `public-assets-verification.json`：公开三个资产、checksum 与 SBOM 校验。
- `server-stage-network-diagnosis.*`：internal 网络端口映射问题的实际证据。
- `server-stage-data-protection.*`：预演变化字段、现有 RAG 状态与生产未切换回执。
