# Stellar 2 设置分组界面证据

权威任务记录：[stellar-settings.md](../../plans/stellar-settings.md)。

`reference.png` 为用户提供的唯一界面参照，沿用其 Console 风格、标签栏和单卡片布局。
`after-appearance/navigation/layout-{390,1280}.png` 是本轮新增分类的真实隔离服务截图；
`existing-{post,comments,seo,search,advanced}-{390,1280}.png` 为原有分类迁移后的回归截图。

环境由 `scripts/e2e/run-server.sh` 准备，数据库、附件、内容和站点配置均在任务工作树 `.tmp/e2e-server.*`，服务只读取本任务构建的 SPA 与前台资源。
本轮未连接生产环境，也未调用外部 AI、邮件或付费接口。截图使用固定的 E2E 管理员夹具。

验证结果与构建/服务资源哈希见任务记录及 `verification.json`；最终通过日志保留在本任务 `.tmp/` 中；修复依据见权威任务记录。
