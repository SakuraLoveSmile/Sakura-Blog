# BLG-EDIT03：页面编辑器本站资料只读提示

- 状态：已验收（2026-11-10，Asia/Taipei）。用户在 BLG-EDIT02 体验后把「本站信息」块当成 bug 反馈：同页其他内容可编辑，只有它点不进。确认这是后端按站点设置生成、刻意不入正文的只读装饰后，用户要求在该块上方加一行只读提示；实现并合入 `dev` 后用户明确确认验收。
- 目标：页面编辑器的友链「本站信息」只读装饰上方显示一行提示，说明内容来自站点设置；提示只属于编辑器界面，不进入 TipTap 文档、Markdown、前台 `/friends` 或 Console 前端预览。
- 工作区：`/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261110-fix-blg-edit03-site-info-hint`，分支 `codex/fix/20261110-blg-edit03-site-info-hint`，实现起点 `1af89ac8`（dev）。集成目标根目录 `dev`。根工作树有用户未提交改动，本次未触碰。
- 范围：`modules/console/web/src/components/editor/stellar-editor-decoration.ts`（widget 外壳与提示行）、`StellarEditingCanvas.vue`（提示行样式）、`modules/console/tests/stellar-editor-decoration.test.ts`（结构约定）。

## 已确认决定

1. 提示文案为「只读 · 由「设置 → 站点信息」自动生成」，单行、12px、主题语义色 `--text-p3`，只在编辑画布出现。
2. 提示 DOM 由编辑器宿主创建并包在同一个 `contenteditable="false"` widget 内，服务端返回的 `.friends-site-info` 片段保持原样，不做字符串改写。
3. 不改变只读语义：提示不可编辑、不写入正文，也不在宿主生成链路（`stellar_host_pages.go`）里加提示，因此前台与预览不受影响。
4. 不加裸 Hex 颜色；沿用 Console 既有「裸 Hex 守卫」，提示色走主题变量。

## 验收

- [x] 编辑器内本站资料块上方出现一行提示，文字与设计一致，块本身仍只读。
- [x] 提示不进入 Markdown/正文：编辑后保存，`content_md` 与原始内容逐字相同。
- [x] 前台 `/friends` 不出现提示，仍只有一个 `.friends-site-info`。
- [x] 1280/桌面与 390 手机宽度下提示单行、无横向溢出；深色（`rgb(28,31,33)` 底）与浅色（`rgb(249,250,251)` 底）均随主题取色。
- [x] `vue-tsc` 与 console 行为测试通过；`make test` 全量门禁通过。
- [x] 用户体验确认（2026-11-10 用户明确要求标记已验收）。

## 实测

- 隔离服务：`.tmp/hint-verify/root`（`prepare.go` 播种的 e2e 管理员与空站点），监听 `127.0.0.1:8096`，`-dev` 加载本次真实 `dist`。验收脚本 `.tmp/hint-verify/verify.mjs` 只在本机临时使用，不入库。
- 证据：`docs/baselines/page-editor-site-info-hint/` 的 `before-user.png`（用户问题截图）与 `after-{desktop,mobile}-{dark,light}.png`；`verify-dark.json`、`verify-light.json` 记录提示数量、几何、颜色与保存往返。
- 保存往返实测：编辑器内点击「保存」得到 `200`，随后 `GET /api/v1/pages/friends` 的 `content_md` 与原始 Markdown 完全一致，且不含「只读」「本站信息」；画布文本包含提示（证明只存在于装饰层）。
- 提示几何：桌面 18px 高、12px 字号、`letter-spacing 0.24px`，位于 `.friends-site-info` 上方（`aboveSection: true`）；390 宽度下宽 340px、单行、`overflowRight: -26`，文档 `scrollWidth` 等于 `clientWidth`。
- 编辑不受阻：重新打开编辑器后，在提示上方的段落末尾输入「临时」，画布文本立即包含该内容；`ControlOrMeta+z` 撤销后文本恢复，未产生保存（`editingAboveHint`）。其余正文的编辑、撤销与保存路径未改变。

## 未验证与限制

- 未推送、未发布：本次只合入本地 `dev`，`origin/dev` 仍未更新；服务器照旧运行既有镜像。
- 未跑完整 Playwright e2e 套件；本次改动是编辑器装饰层，已用真实构建 + 浏览器实测覆盖关键路径。
- 未覆盖「站点资料为空」等异常设置的截图，文案与只读行为不依赖具体取值。

## 合入

- 提交 `11301e8e`（`feat: 为页面编辑器本站资料加只读提示`，14 个文件）。在 feature worktree 执行 `make merge-check`：合并前 `make test` 全绿 → 合入根目录 `dev` → 合并后组合态 `make test` 全绿；`dev` HEAD 即 `11301e8e`。
- 用户原有的未提交改动（根工作树约 202 项）未被触碰；合并只带入本任务的 5 个路径。
- 合入后按仓库规则移除 worktree `.../20261110-fix-blg-edit03-site-info-hint` 并删除已合并分支 `codex/fix/20261110-blg-edit03-site-info-hint`，删除前确认工作树干净、改动已在 `dev`。

## 收尾

worktree 与分支已清理；`.tmp/hint-verify` 是临时验收资源，随工作树移除，不进入 Git。
