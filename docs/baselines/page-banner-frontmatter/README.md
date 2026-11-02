# 页面编辑「横幅与元信息」接入 Stellar 2.0 front matter

## 参照

- 用户提供的线上 `/friends` 截图（2026-11-02，2692×1558，sha256 见本次请求附件）：页首横幅包含「主页 / 友链」面包屑、发布日期与「友链」标题。
- Console 页面编辑实测基线（真实构建、隔离站点）：`before-console-banner.png`、`before-friends-banner.png`，2026-11-02 采集。基线页面同时带迁移后 front matter 与历史旧列，用于确认表单回填来源。

## 目标效果

- Console 页面编辑「横幅与元信息」显示值与前台实际生效值一致：
  - 横幅背景图：读取 `blog_extensions.content.banner_image`（渲染优先级更高），缺失时回退 front matter `cover`。
  - 侧栏品牌图片：读取 `blog_extensions.content.banner_icon`，缺失时回退 front matter `leftbar.brand.image.src`。
  - 横幅标题/副标题/头像：`banner.headline` / `banner.tagline` / `banner.avatar`（保持现有行为）。
  - 显示面包屑和日期：`breadcrumb`；发布日期：`date`；作者：`article.author`（保持现有行为）。
- 修改后保存：写入上述官方 front matter 字段；背景图与侧栏品牌图同时删除同义的历史 Blog 扩展字段，避免旧值继续覆盖；旧页面列仍按现有逻辑写入以兼容旧读取路径。
- 未编辑字段、未知字段与原文格式保留；不因回填补写默认值。
- 前台 `/friends` 页面横幅随保存变化：背景图、标题、副标题、头像、面包屑/日期开关生效。
- 文章编辑器与文章页横幅不受影响；页面编辑 UI 布局与样式不变。

## 验收项

- [x] 加载带迁移后 front matter 的页面：背景图回退 front matter `cover`，侧栏品牌图片优先 Blog 扩展旧值（`content.banner_icon`）；头像、标题、副标题、面包屑、日期正确回填。
- [x] 页面只配置 front matter `cover`、未配置 `content.banner_image` 时，表单显示该背景图。
- [x] 修改横幅背景图并保存：`stellar_config.cover` 更新，`blog_extensions.content.banner_image` 被删除，前台背景图变化。
- [x] 修改侧栏品牌图片并保存：`leftbar.brand.image.src` 更新，`content.banner_icon` 被删除，前台左栏品牌图变化。
- [x] 修改横幅标题/副标题/面包屑/日期后保存：front matter 对应字段更新，前台横幅跟随，无需手改 JSON。
- [x] 只改一个字段时，`stellar_config` / `blog_extensions` 中未编辑字段与未知字段保持（单元测试覆盖：`unknown`、`content.keep`、`title`、`date` 等）。
- [x] 清空背景图/品牌图：删除对应键而不是写空串。
- [x] 文章编辑器与文章页横幅不受影响（`postFields` 仍用 `content.banner_image`；文章页对照横幅正常）。
- [x] Console 页面编辑布局与样式无回归（桌面 1440 与移动 390 抽查，无横向溢出）。

## 验收记录（2026-11-02）

环境：本 worktree 的 Console 真实构建（`pnpm --filter console run build`）+ `cmd/blog` 隔离站点（`:18083`，`.tmp/console-banner-check/runtime`，独立 SQLite 与 upload 夹具）；页面 `friends` 同时带迁移后 front matter 与历史旧列；背景图/品牌图使用本地 `/uploads/*.png` 夹具，避免外链加载失败干扰截图。

- 表单回填（Playwright 真实 Console，读取附件链接字段）：
  - 横幅背景图 = `/uploads/banner-before.png`（front matter `cover`，Blog 扩展无 `banner_image`）。
  - 侧栏品牌图片 = `/uploads/icon-before.png`（Blog 扩展 `content.banner_icon` 优先）。
  - 头像 = `/uploads/avatar.png`；横幅标题/副标题 = 迁移横幅标题/迁移副标题；面包屑勾选；日期 = `2026-08-21`。
- 保存后数据库（`pages.friends`）：
  - `stellar_config`：`cover=/uploads/banner-after.png`、`banner.headline=Console 修改的横幅标题`、`banner.tagline=Console 修改的副标题`、`breadcrumb=false`、`date=2025-03-04`、`leftbar.brand.image.src=/uploads/icon-after.png`。
  - `blog_extensions`：`content.banner_icon` 已删除（夹具清理后 `content={}`）。
  - 旧列同步：`banner`、`banner_info`、`breadcrumb=0`、`published_at=2025-03-04`（保持旧读取路径兼容）。
- 前台 `/friends` 服务端渲染（修改后）：新背景图、新标题、新副标题、新左栏品牌图出现；面包屑与日期因 `breadcrumb=false` 隐藏；旧背景图不再出现。
- 文章页 `/post/post-banner` 对照：横幅标题保持「文章横幅标题」，无背景图（未被页面表单影响）。
- 移动端 390×844：Console 页面编辑无横向溢出。
- 截图：`before-console-banner.png`、`before-friends-banner.png`、`after-console-banner.png`、`after-friends-banner.png`、`after-article-banner.png`、`after-console-banner-mobile.png`。
- 机械门禁：`make console-test`（vue-tsc + 425 项 node 行为测试，含新增 `page-banner-form.test.ts`）通过；`make test` 见任务记录。
- 未验证：生产部署；真实外链图片显示（使用本地 upload 夹具规避 onerror）；Stellar 前台四种外观预设未逐一切换（本次只改 Console 表单数据来源与保存行为，无视觉样式改动）。
