# BLG-20261110-06：友链分区标题悬停色块与链接下划线

- 状态：待体验
- 分支：`codex/fix/20261110-BLG-20261110-06-friends-heading-hover`
- 工作树：`/Users/sakurasep/Documents/Code/Project/Project Worktree/Blog/20261110-fix-BLG-20261110-06-friends-heading-hover`
- 基点：`dev` @ `11301e8e`
- 合入目标：`dev`
- 验收依据：`docs/baselines/friends-heading-hover/README.md`

## 目标与范围

修复 `/friends` 分区标题的悬停异常：鼠标移上去整段标题被强调色块盖住、文字消失，底部多出一条链接色 1px 细线。

范围只有 `modules/stellar/web/assets/css/friends-page.css` 的标题锚点悬停覆盖，以及本任务基线与记录；不改模板、Go 代码、主题生成 CSS（`main*.css`）或其他页面。

## 已确认事实

2026-11-10 在线上 `/friends` 实测（`data-article-style="tech"`，article 带 `heti`）：

- 标题锚点 `.headerlink` 被 `friends-page.css` 拉满整个标题框，锚点色块会盖住标题文字。
- 悬停色块来自主题 tech 标题规则 `body[data-article-style="tech"] article.content h2 > a:first-child:hover { background: var(--accent) }`；`friends-page.css` 原先只覆盖静态态 `background`，缺少 `:hover` 覆盖，悬停规则以更高特异性生效。
- 细线来自 Heti 的 `.heti a:hover { padding-block-end: 1px; border-block-end: 1px solid currentColor }`；锚点 `color` 是站点链接色（线上为绿色），1px 边框落在拉满的锚点底部，橙色背景不覆盖边框。

## 已确认决定

- 在 `friends-page.css` 内补 `.headerlink:hover { background: none; border: 0 }`，就地覆盖，不动生成的主样式。
- 分区标题不提供悬停视觉变化；默认态、锚点与目录目标保持不变。

## 验收场景

1. 默认态与修复前一致：双斜杠、宋体标题、无底色。
2. 悬停标题不再出现整块强调色，标题文字不被遮挡。
3. 悬停不再出现链接色 1px 底线（站点开启 Heti 时同样成立）。
4. 标题锚点保留，点击仍可跳转。

## 验证计划与实测

- `make test`：通过（2026-11-10 本工作树；提交前 hook 与 `make merge-check` 再复核）。
- 本地隔离站点（`.tmp/friends-hover-check`，:8125，tech 文章样式）headless Chrome：
  - HEAD 版 CSS（route 拦截替换）+ 注入 Heti 链接悬停规则：悬停锚点 `background: rgb(255,87,36)`、`border-bottom: 1px solid rgb(32,148,243)`，复现色块与细线。
  - 修复版 CSS：同一坐标悬停 `background: transparent`、`border: 0`，标题恢复；两次均 `hovered=true`、无 `pageerror`。
  - 截图：`docs/baselines/friends-heading-hover/before-hover.png`、`after-hover.png`。
- 线上 `/friends` 复核（Playwright 注入修复版 CSS，保留真实 DOM、Heti 与站点链接色）：悬停不再出现色块与细线。

## 未做

- 推送、发布、生产部署（需另行授权）。
