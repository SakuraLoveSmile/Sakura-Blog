# 页面编辑「横幅与元信息」接入 Stellar 2.0 front matter

## 目标与范围

Console 页面编辑器的「横幅与元信息」表单在 Stellar 2.0 下大部分字段已经通过 `useStellarForm` + `pageFields` 与 `stellar_config`（front matter）双向同步，但背景图与侧栏品牌图片仍指向与 2.0 渲染不一致的来源：

- 「横幅背景图」读写 `blog_extensions.content.banner_image`，而迁移后的页面背景图在 front matter `cover`（`stellarBlogBannerImage` 会优先用 Blog 扩展值覆盖 `cover`）。表单因此不回显已有背景图，新写值又落在非官方字段。
- 「侧栏品牌图片」读写 `leftbar.brand.image.src`，但迁移把旧 `banner_info.icon` 写入 `blog_extensions.content.banner_icon`（2.0 左栏品牌图不消费该字段）。表单不回显旧值，也不清理同义旧字段。

本次把两处表单字段接到 2.0 官方来源，并保证表单显示值与前台实际生效值一致：

- 「横幅背景图」：读取顺序 `blog_extensions.content.banner_image` → front matter `cover`；写入 `cover`，同时删除同义的 `content.banner_image`。
- 「侧栏品牌图片」：读取顺序 `blog_extensions.content.banner_icon` → front matter `leftbar.brand.image.src`；写入 `leftbar.brand.image.src`，同时删除同义的 `content.banner_icon`。
- 其余字段（横幅标题、副标题、头像、面包屑、发布日期、作者、页面标题）保持现有 front matter 同步行为，不改语义。
- 扩展通用表单同步器 `useStellarForm`：支持字段级「读取来源链」与「写入后清理历史同义字段」，只重写被编辑字段，未知字段与未编辑字段原文保留。
- 不改 Console 布局与样式，不改文章编辑器（文章横幅仍用 `content.banner_image` + 文章 `cover` 语义）。

## 已确认决定

- 2026-11-02 用户确认「把页面编辑器的旧横幅表单接到 2.0 front matter（或移除）」，选择接入 front matter，不移除表单。
- 表单显示值必须与前台实际生效值一致：读取按渲染优先级取第一个存在的来源；写入官方字段并清理同义历史字段，避免旧值继续覆盖。

## 运行资源

- 基线：根工作区 `dev` / `4d4f9633`。
- worktree：`/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261102-feat-blg-20261102-05-page-banner-frontmatter`
- 分支：`codex/feat/20261102-blg-20261102-05-page-banner-frontmatter`；合入目标：根工作区 `dev`。
- 隔离夹具：`.tmp/console-banner-check/`（页面 `friends` 同时带迁移后 front matter 与历史旧列，含对照文章）。

## 验收场景

见 `docs/baselines/page-banner-frontmatter/README.md`：表单回填、保存后 front matter 更新、清理同义字段、前台横幅跟随、未编辑字段保留、文章横幅不受影响、无样式回归。

## 代码与验证

- 变更点：
  - `modules/console/web/src/components/editor/useStellarForm.ts`：字段级 `readFrom` 读取来源链与 `migrate` 写入后清理历史同义字段；未编辑字段与原文保持不动。
  - `modules/console/web/src/components/editor/stellar-form-fields.ts`：`pageFields` 的横幅背景图改为写入 front matter `cover`（读取回退 `content.banner_image`），侧栏品牌图片改为写入 `leftbar.brand.image.src`（读取回退 `content.banner_icon`）；清空时删除键。
  - `modules/console/tests/page-banner-form.test.ts`：8 项 node 行为测试（读取链、写入与清理、清空删键、未知字段保留、host 字段回填、文章字段不回归）。
- 实测：`make console-test` exit 0（vue-tsc + 425 项行为测试）；真实 Console 浏览器验收（Playwright + 隔离站点）通过，见 `docs/baselines/page-banner-frontmatter/`：回填、保存、DB 字段、前台横幅、文章对照、移动端无溢出。
- 实测：`make test`（见提交前与 merge-check 记录）；`make build`（真实 Console 产物）通过。
- 未验证：生产部署；真实外链图片加载（使用本地 upload 夹具）；前台四种外观预设未逐一切换（本次无样式改动）。
- 说明：上一轮排查曾判断「页面编辑横幅表单在 2.0 下不生效」，真实 Console 验收后确认多数字段（标题/副标题/头像/面包屑/日期/作者）已由 `useStellarForm` 接入 front matter；本次只修正背景图与侧栏品牌图两处与 2.0 渲染来源不一致的映射，并补充清洗历史同义字段。
