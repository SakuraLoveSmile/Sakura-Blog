# BLG-20261109-03 编辑器标签预览表面按配色自适应

## 目标与范围

用户报「编辑器里的 GitHub 仓库卡片会渲染一整块背景，卡片不能自适应大小」。排查确认那块背景来自编辑器标签预览 iframe 注入的**站点不透明表面**（`--background`），不是卡片自身样式；页面编辑器有正式页画布所以是透明嵌入，文章/笔记/Wiki 编辑器统一保留不透明表面，导致深色 Console 上出现整块站点底色，且卡片只占左侧约 400px，余下区域全部露色。

本任务只改「文章/笔记/Wiki 等非页面画布编辑器」的标签预览表面判定：

- 站点配色与 Console 底色**同色系**时保持透明，不再铺站点底色；异色系仍铺不透明表面，保留原有文字对比度保护。
- 页面画布（`layout=page`）继续使用 `embedded` 透明嵌入，行为不变。
- 组件库、短代码 Studio、插入面板、Markdown 分屏、设置页前端预览等**独立预览**不传宿主底色，保持原不透明表面，行为不变。
- 不改卡片自身样式（`max-width: 400px`、`--card` 背景与边框沿用线上同款），不改 iframe 宽度策略。

## 已确认决定

- 判定口径：宿主底色（Console `body` 实际背景色）与站点解析后的表面色，按 WCAG 对比度比较；`< 2` 视为同色系、不铺表面，`>= 2` 铺站点表面。首屏 CSS 兜底按「宿主深色只给浅色站点铺色 / 宿主浅色只给深色站点铺色」生成，避免脚本执行前的闪烁。
- Console 目前全站深色（`--color-background: #0a0a0b`），但实现按宿主实际底色取值，不写死深色假设；后续如加浅色 Console 无需改预览逻辑。
- 站点 `color_scheme: auto` 时由文档 `data-theme`/`prefers-color-scheme` 在 iframe 内解析，脚本在系统偏好变化时重新判定。
- 缓存键补宿主底色：同一标签在不同宿主底色下不共用预览文档。

## 分支 / worktree / 合入目标

- 任务编号：`BLG-20261109-03`。
- 分支：`codex/fix/20261109-BLG-20261109-03-editor-tag-surface`。
- worktree：`/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261109-fix-BLG-20261109-03-editor-tag-surface/`。
- 基线：`dev`（`8a96ec23`）；合入目标：根目录 `dev`。
- 代码版本：`modules/console/web/src/modules/plugins/preview/preview-document.ts`、`modules/console/web/src/modules/plugins/preview/usePluginPreview.ts`、`modules/console/web/src/components/editor/StellarTagView.vue`。

## 实现

- `preview-document.ts`
  - 新增 `surfaceLuminance`（自包含，可 `.toString()` 注入 iframe）、`surfaceSchemeOf`、`PREVIEW_SURFACE_MATCH_CONTRAST = 2`。
  - `injectPreviewSurface(document, hostBackground?)`：给出可解析的宿主底色时，首屏 CSS 只给「与宿主明暗相反」的站点主题铺色；运行时脚本比较站点表面色与宿主底色的对比度，同色系用 `removeProperty` 保持透明，异色系落成不透明 `rgb`。未给出/非法宿主底色时保持旧行为。
  - `preparePreviewDocument` 透传 `hostBackground`。
- `usePluginPreview.ts`：`PluginPreviewOptions.hostBackground`，传给 `preparePreviewDocument`；`surfaceCacheKey` 把宿主底色计入缓存键。
- `StellarTagView.vue`：读取编辑器实际露出的底色（`body` → `documentElement` 第一个不透明背景色）；`layout=page` 仍 `embedded`，其余传 `hostBackground`。

## 验收场景

1. 深色站点 + 深色 Console（用户场景）：文章编辑器标签预览不再铺站点表面，卡片直接落在 Console 底色上；卡片名称、提示块、tabs 文字对实际底色对比度 ≥ 4.5。
2. 浅色站点 + 深色 Console：保持不透明浅色表面，文字对比度与改动前一致。
3. `color_scheme: auto`：系统深色偏好下透明，系统浅色偏好下铺浅色表面。
4. 页面编辑器画布：标签仍透明嵌入，画布本身保留正式页背景。
5. 独立预览（Studio/插入面板/组件库/Markdown 分屏/设置预览）：表面行为不变。
6. 注入幂等；不同宿主底色的预览文档不共用缓存。

## 实测

- 单测：`modules/console/tests/plugin-preview.test.ts` 新增宿主底色用例（CSS 只给相反明暗主题铺色、非法宿主色回退旧行为、脚本执行后同色系透明/异色系落色、亮度与配色归类）；`modules/console/tests/stellar-tag-preview.test.ts` 在真实 SFC + 真实 composable 上补「深色宿主下文章编辑器只给浅色站点铺首屏表面」。`pnpm --filter console run test`（含 `vue-tsc -b`）560 项全过。
- 浏览器：隔离 e2e 实例（独立临时根目录，Playwright Chromium 1.62.1，1440×900 DPR2），把站点 `theme.appearance.color_scheme` 设为 `dark`/`light`/`auto` 后打开文章编辑器实测：
  - 深色站点：改动前 iframe `html` 背景 `rgb(28,31,33)`；改动后 `rgba(0,0,0,0)`，且无内联背景。卡片 274×56.5（本地 GitHub 不可达，降级为仅名称）不变。
  - 文字对实际底色对比度：深色站点改动前 16.57/10.25/16.57，改动后 19.79/12.25/19.79（卡片/提示块/tabs）。
  - 浅色站点：改动前后一致（`rgb(249,250,251)` 不透明表面，对比度 20.1/11.52/20.1）。
  - `auto`：系统深色 → 透明；系统浅色 → `rgb(249,250,251)` 不透明表面。页面无 console/page error。
  - 证据：`docs/baselines/editor-tag-surface/verification.json` 与同目录截图。
- 回归：`tests/e2e/tag-live-preview.spec.ts`（见下方结果）。
- 门禁：`make test`（见下方结果）。

## 未验证 / 边界

- 用户生产站点的暖色深色主题（`--background: hsla(44 12% 12%)` ≈ `rgb(34,33,29)`）只用隔离实例的默认深色主题做了同构验证；未登录生产 Console 复测，需发布后由用户确认。
- `max-width: 400px` 的卡片宽度策略未改；用户如希望卡片铺满正文宽度，属于另一项样式决定。
- 浏览器只在 Chromium 上验证；未测 Safari/Firefox 的 iframe 与 `getComputedStyle` 差异。
- 仅改了非页面画布编辑器的 NodeView；独立预览表面未改。

## 遗留

- 生产发布与线上复测需用户另行授权。

## 结果

- `pnpm --filter console run test`（`vue-tsc -b` + `tsx --test`）：560 项全过。
- `tests/e2e/tag-live-preview.spec.ts`：19/19 passed（1.7m）。深色系统偏好用例改为「NodeView 允许透明并按 Console 实际底色验对比度；Studio/插入面板/顶栏气泡/Markdown 分屏仍必须不透明」：NodeView 12.58:1（底色 `rgb(10,10,11)`），其余 10.97:1（`rgb(28,31,33)`）。
- `make test`：见本次 worktree 执行结果（提交前门禁）。
- 尚未提交/合入/推送，等用户确认。
