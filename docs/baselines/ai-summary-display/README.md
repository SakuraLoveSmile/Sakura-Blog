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

## 截图

- `after-desktop.png`：canonical + 列均写入后，隔离服务 1280×900，摘要正文正常显示。
- `after-legacy-column.png`：模拟线上同形数据（仅 `posts.ai_summary` 列），隔离服务 1280×900，回退显示摘要正文。

## 边界

- 截图来自隔离合成站点（`scripts/e2e/prepare.go` 空站 + 合成文章），不连接生产数据库；生产只做了只读查询。
- 未部署生产、未发布版本；发布与生产数据写入均需用户明确授权。
- 未逐项检查四种 appearance 与移动端：本次不改 CSS 或模板结构，仅补全既有摘要卡的正文输出。
