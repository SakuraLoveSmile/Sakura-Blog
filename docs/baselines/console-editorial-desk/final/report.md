# Console「方向 A 编辑台」最终返工报告

- 任务：BLG-20261031-01（`docs/plans/console-editorial-desk.md`）
- 分支：`codex/refactor/20261031-BLG-20261031-01-console-editorial-desk`
- 状态：第三轮收尾完成，等待独立复验；**未提交、未 push、未合并、未发布**；未改动根工作树用户 WIP。
- 运行方式：8140 隔离服务，独立 root `.tmp/rework-server.8140`（DB/config/content/upload），真实 `blog` 二进制 + 生产 dist；验收后服务已停止、临时 root/脚本已清理。
- 机器可读结果：`measurements.json`、`verification.json`。

## 1. 本轮 P0/P1 修复

| # | 问题 | 修复 | 证据 |
|---|---|---|---|
| P0-1 | 尺寸弹窗按列宽去重，识别按宽×高，当前中档不在选项里 | `size-preset-options.ts` 与 `getWidgetSizePreset` 共用 `w×h` 键；识别在同尺寸档位中按预设顺序返回被保留的档位 | sm 趋势 6×36 / 6×44 / 6×52；xxs 趋势 4×32 / 4×42 / 4×50；最近文章/评论、分类、版本同样三档且 medium 选中 |
| P0-2 | 旧测试把分叉写成预期 | 删除“trend sm 只保留 small”断言；新增默认布局遍历断言，以及“尺寸钳制撞档 + 任意宽高识别结果必须在选项内”的穷举断言 | `widget-size-presets.test.ts` 6/6 |
| P0-3 | 缺少真实 UI 验证 | 新增 Playwright 尺寸弹窗用例，逐项打开 sm/xxs 的 5 个组件 | `console-ui-consistency.spec.ts`；73/73 |
| P1-1 | 评论快捷键帮助入口被删 | 恢复紧凑入口 + 232px 弹层，保留 J/K/A/D/S/R/X；44px 命中区与 `aria-expanded`/Esc 关闭 | `CommentList.vue`；390 入口 44×44，弹层完整可见 |
| P1-2 | `.guide-save` 仍为 quaternary 3.7:1 | 改 `--color-text-tertiary`，加入 E2E 对比度断言与 final 度量 | 运行期 `rgb(126,126,133)` on `#0a0a0b`，对比度 **4.91:1** |
| P1-3 | `PostSettings.vue` 残留空 `@media` | 删除空块，并新增空 `@media` 机械门禁（扫描 Console 源码） | 门禁 0 违规 |
| P1-4 | J/K 焦点行操作不可见 | `.comment-row.is-focused .comment-actions` 恢复 opacity | E2E 按 J 后焦点行操作 opacity = 1 |
| P1-5 | VPageHeader `#icon` 死插槽 | 清理 assistant/comment-users/authors/wiki-projects/login-logs/profile/backup/seo/perf/shares/page-editor 直接图标插槽与无用 import；门禁改为扫描 Console 全部 `.vue` | `ui-restraint` 页面标题规则 0 违规 |
| P1-6 | 残留状态/分类胶囊 | FriendList 可达性、MomentList 可见性、UserListView 角色、CommentUsersView 主表状态改为圆点+文字；Shortcode/分类/角色/计数胶囊改 6px 工具胶囊；移除 UserList 内联状态背景 | `ui-restraint` 状态/圆角规则 0 违规；CommentUsersView 抽屉内旧状态标签为已知残留（final 截图不含该页） |
| P1-7 | Shortcode 重试 34px；评论 meta 孤立“·” | 手机重试 44px、圆角 6px；分隔符仅在文章与时间都存在时渲染 | E2E 390 重试宽高 ≥44；模板条件分隔 |
| 门禁 | 状态底只抓固定色；`rounded-full` 无条件放行；字号口径过大 | 状态规则抓任意 `bg-[#hex]` 与内联 `backgroundColor`（逐文件功能色白名单）；`rounded-full`/9999/50% 改为选择器上下文白名单；空 media 与 VPageHeader 全量扫描；字号运行时口径明确为六个主页面 | `ui-restraint` 15 条规则、定向单测 27/27；临时探针证明规则会真实失败后删除 |

尺寸弹窗的实际设备断点是 sm/xxs（`DESIGN_BREAKPOINTS` 不含 xs）；xs 作为兼容断点，由单测覆盖同一组五个组件的三档与 medium 识别。

## 2. 最终检查结果

| 检查 | 命令/对象 | 结果 |
|---|---|---|
| 全量测试 | `make test` | exit 0；Console 369/369；Stellar 238 pass / 2 skipped；Go vet/staticcheck/test 全通过 |
| 生产构建 | `make build` | exit 0 |
| 定向门禁 | `ui-restraint` + `widget-size-presets` + `theme-colors` + `dashboard-quick-links` | 27/27 |
| 目标 Playwright | 4 个 spec，Chrome 通道 | **73/73**（3.3 分钟） |
| 实际产物 | `/console/` 6 个入口资源 SHA-256 vs `dist` | **6/6 MATCH** |
| 运行期异常 | 截图会话 console.error / pageerror | **0** |
| 横向溢出 | 1280 / 390 主要页面 | 0 |

实际加载入口（SHA-256 前 16 位）：`index-Bw6UwQ7A.js` `bae86c9d0ec1af45…`、`index-D-j-weAu.css` `6b69777047727a69…`、`api-C-H7IBEY.js` `28f3d58e9e625d42…`、`pinia-CXeiBUD1.js` `7e7c94b9658d5172…`、`preload-helper-l0U7FUPx.js` `dbd40ae429a13a3e…`、`runtime-core.esm-bundler-GmtFT3dp.js` `bcd39943a10cb760…`。

上述构建与 Playwright 完成后，仅追加了尺寸识别穷举单测（不改生产源码），产物哈希与 73/73 运行证据不受影响；随后复跑 `make test` 为 369/369、定向门禁 27/27。

## 3. 量化对比

| 指标 | 返工前 | 本轮后 |
|---|---|---|
| 尺寸弹窗选项 | sm 趋势仅 small；xxs 中档不可选 | 5 类组件 sm/xxs 均三档，medium `aria-pressed=true` 且 `is-selected` |
| 设置保存说明对比度 | quaternary，约 3.7:1 | tertiary，4.91:1 |
| 快捷键帮助 | 无入口 | 入口 44×44，弹层 232px，Esc 关闭 |
| J/K 焦点行操作 | opacity 0 | opacity 1 |
| 390 评论状态 / 邮箱 | 34×36 竖排 / 34px | 47×18 单行 / 130–144px |
| 评论勾选 | 16×16 | 44×44 |
| 仪表盘快捷链接（390） | 39×20 / 26×20 | 44×44 |
| 设置保存条 | 透明、横向 padding 重复 | `rgb(10,10,11)`、padding 16/0、容器 32/16 |
| 可见字号 | 仍有 16px 继承泄漏 | 六个主页面仅 11/12/13/14/20/24，0 违规 |
| 空 @media / 标题图标 | 有残留 | Console 源码机械扫描 0 违规 |
| 状态底 / 全圆角 | 规则覆盖面不足 | 任意 `bg-[#hex]`、内联状态底色、无上下文 full 圆角均会被拦截 |

## 4. 截图与夹具

- 文章 8 篇：5 已发布 / 2 草稿 / 1 回收站，1 篇置顶；列表默认显示 7 篇。
- 评论 6 条：approved 2（含管理员回复）、pending 2、rejected 1、spam 1。
- final 截图 14 张：仪表盘、文章列表、海报视图（含选中态）、设置、评论各 1280/390；评论快捷键 1280/390；尺寸弹窗 sm/xxs。
- 评论 390 与尺寸弹窗截图拍摄于 specs 运行前的干净夹具；specs 会向隔离库追加临时评论/文章，不影响截图。

## 5. 未覆盖与保留项

- 非管理员 Console 快捷入口仍只做单测：`RoleAuthor` 无 `console:access`，无法在真实控制台登录，不以伪造权限代替。
- 仅验证 Chrome；未验证 Safari/Firefox 与真机触屏。
- D04（390 外观页既有 6px 底栏贴近）、D05（置顶不参与客户端排序）为验收报告标注的既有问题，本次未改业务语义，仍保留。
- 字号运行时扫描口径是本次相关的**六个主页面**（仪表盘、设置基本/外观、外观、评论、文章）；字号/状态/圆角源码门禁扫描 `ui-restraint` 的 `DIRECTION_A_SCOPE` 白名单，空 `@media` 与 `VPageHeader #icon` 规则扫描 Console 全部 `.vue`；不声称“Console 整体”全量覆盖。
- `CommentUsersView` 抽屉内的旧状态标签（4px 圆角、10px 字号、secondary 背景）仍保留；final 截图不含 `/comment-users`，该项只有源码证据。
- 评论快捷键帮助在输入框/回复态聚焦时 Esc 不关闭弹层；E2E 只覆盖默认焦点场景。
- 本轮之后不再扩范围；如复验发现新问题，应另立切片。

## 6. 截图哈希

见 `measurements.json#files`；14 张 PNG 的 SHA-256 均在该文件记录。
