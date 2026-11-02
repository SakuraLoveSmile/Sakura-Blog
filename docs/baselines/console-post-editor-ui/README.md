# Console 文章编辑页 UI 优化基线

- 任务：`BLG-20261102-06`（分支 `codex/refactor/20261102-blg-20261102-06-console-post-editor-ui`）
- 日期：2026-11-02（Asia/Taipei）
- 状态：待体验（实现与本地组合验证完成；用户尚未确认验收）
- 范围：只调整文章编辑页 UI 结构与样式，不改变文章数据流、编辑器能力或设置字段。

## 视觉参照

唯一方向参照：`reference/article-editor-ui-preview-20261102-a1.html`（用户确认的「内容优先 · 少卡片 · 单色强调」桌面与移动预览）。

- 归档时仅修正移动发布按钮的局部级联冲突：`.mobile-publish` 提高一级选择器到 `.mobile-actions .mobile-publish`，避免通用 `.product button` 的 `color: inherit` 覆盖近白底上的深色文字。其余结构、间距和颜色未改。
- `reference/index.html` 是带 Console 语义 token 的本地查看壳，只用于复核预览，不向生产测试注入任何 `.product` 规则。
- 既有 `docs/baselines/editor-settings-panel/` 只继续作为字段完整性与行为约束的来源；其「保留卡片」的旧视觉约定由本次用户确认的预览取代。

## 已确认设计契约

1. 保存状态并入页头，移除独立 `.editor-status-bar`。
2. 标题、工具栏、正文形成一个开放写作面；取消三层独立卡片背景、圆角和重复边框，只保留必要 hairline。
3. 编辑器类型切换、桌面侧栏开关和移动「文章设置」入口整合到工具栏右侧；中间低频工具仍可内部横向滚动。
4. 右侧「大纲 / 设置」及全部设置分组改为单层结构，以留白和细分隔线代替卡片堆叠；输入、弹层、封面媒体等功能边界保留。
5. 沿用方向 A 的语义 token 和近白主按钮，不新建设计体系，不修改全局按钮。
6. 移动端保留现有底部设置抽屉、底部导航和完整保存/预览/发布能力；直接触控目标至少 44×44px。
7. 移动发布按钮必须清晰可读：近白底、`--color-primary-foreground` 深字、13px/500、至少 44×44px，WCAG 文本对比度至少 4.5:1。

## 证据目录

- `reference/`：确认预览及其查看壳。
- `before/`：生产改动前的真实服务截图、测量与控制台基线。
- `after/`：实施后的同视口截图、computed style、对比度、资产哈希与控制台检查。

## 实测记录

- before：`before/before-{1280,768,390}.png`、`before/before-390-drawer*.png`、`before/before-metrics.json`；采集于未修改代码，独立服务 `127.0.0.1:8136`，无控制台错误、无页面横向溢出。
- after：`after/after-{390,639,640,768,1023,1024,1280,1440}.png`、`after/after-390-{header,drawer,drawer-bottom}.png`、`after/reference-mobile-390.png`、`after/after-metrics.json`；使用最终普通生产构建并重启服务。
- 390 页头发布按钮：近白 `rgb(232,232,236)`、深字 `rgb(0,0,0)`、13px/500、72×44、对比度 17.19:1。
- 参考预览移动发布按钮：`rgb(232,232,236)` 底、`rgb(9,9,10)` 字、对比度 16.29:1，浅字浅底冲突已消失。
- 390 抽屉滚动到底后最后内容 bottom 779.7，底部导航 top 787，无遮挡；最新内容为「配置原文（JSON）」。
- 8 个目标视口均无页面横向溢出；每个视口一个 `.editor-save-status` live region，无 `.editor-status-bar`。
- 服务端 37 个 `/console/assets/*` 与 `modules/console/web/dist` 同名文件 SHA-256 一致，含 Rich/Markdown 异步 chunk。
- 控制台无新增 `console.error` / `pageerror`。

完整命令结果、竞态说明与未验证项见 [../../plans/console-post-editor-ui.md](../../plans/console-post-editor-ui.md)。
