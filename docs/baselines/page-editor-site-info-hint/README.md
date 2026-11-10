# BLG-EDIT03 本地验收

`before-user.png` 是用户反馈「本站信息无法编辑」时的编辑器截图，作为唯一参照。权威记录为 `docs/plans/page-editor-site-info-hint.md`。

- `after-desktop-{dark,light}.png`：本次真实构建的编辑画布，本站资料块上方多一行「只读 · 由「设置 → 站点信息」自动生成」；深色跟随系统深色、浅色跟随系统浅色（站点外观为自动），提示颜色由 `--text-p3` 取主题色。
- `after-mobile-{dark,light}.png`：390px 视口下的同一画布，提示单行且无横向溢出。
- `after-editor-desktop-{dark,light}.png`：整页编辑器，用于确认提示在真实界面里的位置与层级。
- `verify-dark.json`、`verify-light.json`：浏览器实测输出。包含提示文本/数量、几何（18px 高、位于资料块上方）、颜色、保存往返（保存 `200`，`content_md` 与原始 Markdown 逐字相同且不含提示）、390 宽度溢出检查，以及前台 `/friends` 不含提示。
- 截图与 JSON 均来自隔离服务 `127.0.0.1:8096`（`.tmp/hint-verify/root`，e2e 播种的空站点），站点资料为夹具值（`E2E Blog`），不是生产数据。

当前仅本地分支，未提交、未合并、未上线。
