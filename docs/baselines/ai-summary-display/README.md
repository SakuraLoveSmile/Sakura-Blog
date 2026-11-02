# AI 摘要已生成但前台不显示

## 参照

- `reference-before.png`：2026-11-02 23:22 抓取的线上真实页面 `https://blog.xn--fhqths51enha.cn/post/hexo-blog-setup-guide`（1280×900，系统配色为深色）。图中「AI 摘要」卡片只有 Reader API 自助入口，没有已生成摘要正文，即本次问题状态；该图只作问题参照，不是期望效果。
- 生产只读核对：该文章 `posts.ai_summary` 列有 299 字摘要，`blog_extensions` 为 `{"status":"draft"}`，canonical `content.ai_summary` 为空。

## 目标效果

- 文章已保存 AI 摘要时，公开文章页在既有「AI 摘要」卡片正文中渲染摘要文本：canonical `blog_extensions.content.ai_summary` 优先，缺失时回退 `posts.ai_summary` 列。
- 没有摘要、清空摘要或设置关闭 `ai.summary.enabled` 时，不输出摘要正文，行为与原来一致。
- 不改卡片结构、CSS、共享主题源或 Console 前端；独立页面（`layout=page`）仍不渲染摘要卡。

## 验收项

- [x] 通过 Console API 保存摘要后，列与 canonical 扩展同时写入，扩展中的其它键保留。
- [x] 公开文章页 HTTP 输出 `<div class="body"><p>摘要文本</p></div>`。
- [x] 模拟生产同形数据（仅列有摘要、canonical 扩展为空）时，页面回退显示摘要（`after-legacy-column.png`）。
- [x] canonical 扩展存在时优先于旧列（路由单测 `TestStellarAISummaryFallsBackToLegacyColumn`）。
- [x] 清空摘要后列与扩展均不残留，页面不再输出摘要文本；再次生成后恢复可见。
- [x] `make test` 全量通过；新增 stella store/route 回归用例。
- [x] 无摘要时页面结构与问题图一致（仅正文不存在），不影响其它页面。
- [x] 启用 HeTi 的页面摘要无词内空隙（`after-heti-desktop.png`）；HeTi 关闭时保留原打字机动画。
- [x] 生产部署后文章页摘要完整可见（`after-production.png`）。

## 截图

- `after-desktop.png`：canonical + 列均写入后，隔离服务（未启用 HeTi）1280×900，摘要正文正常显示。
- `after-legacy-column.png`：模拟线上同形数据（仅 `posts.ai_summary` 列），隔离服务 1280×900，回退显示摘要正文。
- `production-before-heti-fix.png`：beta03 生产页面，摘要已显示但 HeTi 造成 `He xo`、`Gi t` 等词内空隙（beta04 修复前状态）。
- `after-heti-desktop.png`：启用 HeTi 的隔离服务（beta04 代码）1280×900，`heti-spacing` 只包裹完整单词，摘要排版正常。
- `after-production.png`：beta04 生产页面 1280×900，摘要完整、排版正常（最终效果）。

## 发布与生产

- beta03（`v2.0.7-beta03`，release `0796ac76`，镜像 digest `sha256:fd65dba9…`）部署后文章摘要首次显示；beta04（`v2.0.7-beta04`，release `ef7d687c`，镜像 digest `sha256:1f6612a7…`）部署后 HeTi 词内空隙修复。
- 生产验收（公网真实页面，Playwright 1280×900）：`#ai-summary .body p` 文本完整；`heti-spacing` 仅包裹完整 token（Hexo、Git、Node.js v12、npm、cnpm、hexo-cli、`hexo clean && hexo g && hexo s`、github.io 等），无词内拆分；公网 `/settings` 显示 `Blog v2.0.7-beta04`，`/readyz` 200。
- 升级前备份 `/home/ubuntu/blog-release-v2.0.7-beta04/`（0700，db 三件套与 compose 副本，db md5 一致）；回退为 compose image 改回 `:2.0.7-beta03` 后 `up -d`，本次无 schema 迁移。

## 边界

- 隔离截图来自 `scripts/e2e/prepare.go` 空站 + 合成文章，不连接生产数据库；生产只做了只读查询与已授权的升级操作。
- HeTi 词内空隙的根因是页面加载打字机与 HeTi（CDN 脚本）的时序竞争；修复后 HeTi 页面不播放打字机，非 HeTi 页面行为不变。
- 未逐项检查四种 appearance 与移动端：本次不改 CSS 或模板结构，仅补全既有摘要卡的正文输出与 HeTi 时序处理。
