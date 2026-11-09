# BLG-RAWSURF01 验收基线：内容编辑器「高级：配置原文」入口

唯一界面参照：改造前在隔离实例（`127.0.0.1:8097`，2.0 空站点 + `scripts/e2e/prepare.go` 夹具）上实际截取的同一批页面，记录于本文的「改造前形态」。改造前该入口是一个带卡片皮肤的 `<details>`，标题为「Stellar 2 原文配置」，默认折叠、占一整块卡片面积。

改造后截图均为本任务真实构建（`make build`）后重新启动服务的产物，1280×900 / 390×900，Chromium。页面地址与 `docs/plans/BLG-RAWSURF01-stellar-raw-config-entry.md` 一致。

## 改造前形态（文字记录，未保留截图）

| 页面 | 文案 | 位置 |
| --- | --- | --- |
| 文章编辑（右侧设置） | 配置原文（JSON） | 抽屉最后一块 |
| UC 文章编辑 | Stellar 2 原文配置 | 标题/摘要下方、正文编辑器**上方** |
| 文库项目管理 | Stellar 2 原文配置 | 右栏表单**最顶部** |
| 文库文档编辑 | Stellar 2 原文配置 | Markdown 编辑器**正上方** |
| 专栏弹窗 | Stellar 2 原文配置 | 表单最后一块 |
| 笔记编辑 | 高级：Front matter 与 Blog 扩展（JSON 原文） | 表单与正文之间 |
| 笔记本编辑 | 高级：集合配置与 Blog 扩展（JSON 原文） | 表单底部 |

## 改造后截图

- `after-post-editor-1280.png`：文章编辑正常态。设置面板内正文控件干净，入口收成面板末尾一行，默认折叠。
- `after-post-editor-390.png`：文章编辑手机端（设置抽屉已打开）。入口 44px 点击区，可展开为两个 textarea。
- `after-auto-expand-error.png`：注入未闭合 JSON 后，入口自动展开、显示橙色状态点与具体诊断（`Unterminated string in JSON at position 14`）。
- `after-uc-editor-1280.png`：UC 文章编辑，入口已移到正文编辑器**之后**。
- `after-wiki-projects-1280.png`：文库项目管理，入口已移出表单顶部（首屏不再出现）。
- `after-note-editor-1280.png`：笔记编辑，入口位于正文编辑器下方。
- `after-settings-advanced-1280.png`：系统设置「代码与原文」页的站点级「完整配置原文」未受影响，仍默认折叠。

## 逐项验收

- [x] 7 处入口统一为「高级：配置原文」，默认折叠，一行高 44px，不再带卡片皮肤。
- [x] 非法 JSON 时自动展开并显示诊断与橙色状态点；修好后条件消失不强制收起（保留用户选择）。
- [x] `stellar_version !== 2` 时同样自动展开并显示迁移提示（`needsAttention` 的第二个条件，代码路径与错误态共用）。
- [x] 「只校验」返回「校验通过」，且校验前后原文逐字相同（`before==after=true`）。
- [x] UC 入口位于正文之后（`raw-after-editor` 断言）。
- [x] 1280 与 390 两个宽度横向溢出均为 0；390 抽屉内入口 44px 可展开。
- [x] 站点级「完整配置原文」仍默认折叠，未被本次改动影响。
- [x] `AdvancedSettings.vue` / `ThemeYamlTab.vue` 已删除，仓库内无残留 import。

## 未覆盖项

- 文库文档编辑：夹具站点没有 Wiki 文档，该页在无文档时不渲染表单，因此只确认了代码顺序（`ContentEditor` → `StellarContentConfig`），未取得该页的真实截图。
- 旧版本（`stellar_version !== 2`）自动展开：夹具站点是全新 2.0 空站点，没有 1.x 内容，该分支由代码与同一 `needsAttention` 条件保证，未做真实旧内容实测。
