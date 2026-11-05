# BLG-RSS01：友链动态状态修正与上线

- 状态：实现中；日期：2026-11-05（Asia/Taipei）。用户授权：修改代码并更新线上，包含项目规定的提交、推送、PR 晋级、版本发布及部署。
- 目标：未配置 RSS 的友链只显示卡片与简介；抓取失败显示“动态暂不可用”；失联站点集中显示且不再误报未设置 RSS；正常的 HTTP 重定向不判失联。
- 起点：根工作树 `dev` / `a2399d1f2399b56840e2806aa95c413ad50d8df3`，用户已有文档、截图与未跟踪文件全部保留。线上镜像为 `2.0.7`。
- 分支：`codex/fix/20261105-blg-rss01-friend-feed-status`；工作树：`/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261105-fix-blg-rss01-friend-feed-status`；本地集成目标为根 `dev`。先同步 `origin/release` 已有的纯合并历史，源码无差异，确保后续晋级无需绕过分支保护。
- 执行约定以本次用户提供及根工作树的 AGENTS.md 为准；独立工作树中旧的位置/命名条款不适用，不把根工作树已有的规范修改夹带到本功能提交。

## 行为与视觉边界

- 唯一参照：`docs/baselines/friends-rss-status/before.png`，2026-11-05 当前线上 `/friends`。仅调整动态状态与失联分区，复用 `.user-post-card`、`.post-panel` 及现有 `.unavailable-*` 样式；不修改 CSS、全局 token、Console 或文章内容。已按本地设计库消费入口查阅卡片规则。
- `posts` 非空时保持文章链接、标题、日期；无文章且 `feed` 非空时显示暂不可用；两者为空时隐藏诊断提示。关闭动态开关时只保留卡片。
- `unavailable=true` 的友链置于组件末尾“失联友链”分区，保留名称、头像、描述、标签，移除跳转和文章动态。未知状态的外部源沿用正常卡片。
- 友链健康状态与巡检统一：200–399 为可达；未检测状态不直接判失联；手工“失联/失效”标签继续优先。保持探测不跟随重定向的出站安全边界。
- 本地友链 API 已由服务端聚合缓存，浏览器每次新挂载重新读取，避开原有 24h 持久缓存，保证后台补填 RSS 后新访问得到最新数据；其他数据源仍沿既有缓存策略。
- 不新增 RSS 自动发现、接口字段、依赖或数据库迁移。温锦瑜源站的 localhost 文章地址不是本次修改范围。
- 上线时备份后仅补填长街短梦的 `https://wangyunzi.com/rss.xml`，以及把 Pei你看雪站点网址规范为已验证的 `https://pnkx.top/`；其他友链字段与数据保持。

## 步骤与验收

1. T1（主 agent）：修正实际生效的 `services/friends_and_posts.js`，同步仍受维护的旧 Blog 服务的缺 RSS/失败文案；修正 `internal/config/reload_sections.go` 的 3xx 边界。只读 analyst 并行核查入口与回归范围，不写文件、不派生。
2. T2（主 agent）：扩充现有真实服务脚本测试与健康状态用例，覆盖无 RSS、失败、失联、关闭动态、外部源兼容及缓存边界。修改原“空源必须显示 no-post”的预期，是落实用户要求隐藏未设置提示，不降低验证门槛。
3. T3（主 agent）：`make build`，在隔离数据目录和专用端口启动候选服务；真实浏览器核验当前脚本、浅色/深色/自动与四种 appearance，以及手机页面。按项目执行 `make merge-check` 和组合态检查。
4. T4（主 agent）：推送 dev，按 `dev → main → release` PR 路径等待全部 CI，通过后在 release 合并提交打 `v2.0.8`；验证 Docker Release、无资产 GitHub Release、固定标签镜像与 `docker run … -h`。
5. T5（主 agent）：服务器只操作 blog 容器。拉镜像、停服冷备数据库及 compose、修改两项友链字段与固定镜像标签、启动；核验宿主/公网 `/readyz`、Console/公开设置版本、真实友链页面与 8 条数据。保存证据，停止隔离服务并清理本任务 worktree/本地分支。备份足以恢复两项数据及旧镜像。

## 验证记录

- T1/T2：实际服务已修正，未新增 schema/API 字段/CSS。定向 Node 21 个用例通过（含旧 Blog 入口 4 个）；Go `./internal/config ./internal/friends` 通过。`make test` 与 `make build` 首轮通过；随后独立 reviewer 发现 URL 分类抛错会阻断后续组件，已改回逐组件错误边界并补双组件回归，提交 hook 将核验最终组合。
- 本地运行：隔离根目录为工作树 `.tmp/blg-rss01-runtime`，端口 `127.0.0.1:18085`，独立 SQLite/内容/账号，不访问生产数据。真实浏览器已确认未订阅卡片、失败文案、失联分区和 301 卡片状态；外观矩阵继续核验。
- 本机 DNS 返回安全抓取器拒绝的地址，隔离服务无法抓取公网 RSS，未放宽安全策略。正常文章已由脚本行为测试覆盖，生产升级后再验证真实 RSS 抓取。
- 下一步：最终构建/提交 hook、`make merge-check`，随后 CI 晋级和上线。仅用户明确体验确认后改为“已验收”。
