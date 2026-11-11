# BLG-20261111-02：友链支持最多 3 枚手填角标

- 项目 / 任务编号：personal-blog / BLG-20261111-02
- 状态：待体验
- 创建日期（Asia/Taipei）：2026-11-11
- 权威记录：本文件；实现细节以代码为准
- 主仓库 / 当前工作目录：`/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261111-feat-BLG-20261111-02-friends-multi-labels`
- 基线 / 分支 / 合入目标：`dev` / `codex/feat/20261111-BLG-20261111-02-friends-multi-labels` / `dev`
- 当前 HEAD：`15e8fd4e`（worktree 创建时）
- 运行资源：本地 `-dev` 服务（`:8097`、隔离 root 与 DB），验收后已停止并删除临时目录

## 目标与范围

让每条友链最多带 3 枚手动填写的角标（文案 + 颜色），后台可增删，`/friends` 卡片按顺序渲染成彩色标签；现有单枚角标行为和数据继续可用。

覆盖：
- `links` 表新增 `labels` 列（JSON 数组，存第 2、3 枚）。
- Console 友链编辑弹窗的角标区改为最多 3 行可增删。
- `/api/v1/links` 读写多角标；`/api/friends(-aggregated)` 的 `labels` 数组按序输出。
- 旧单角标数据、`linkwatch` 自动「失效」标签、旧客户端（只传 label/color）语义保持不变。

排除：全局可复用标签库、角标筛选/统计、`{% links %}` 服务端卡片补角标、友链导入按 issue 标签批量映射多枚。

## 已确认决定

1. **主角标仍存 `label`/`color`**：`linkwatch` 的自动「失效」标签、`linkUnavailable` 判定和旧 API 都依赖这两列，不动它们；第 2、3 枚存 `labels` JSON。展示顺序 = 主角标（若有）→ `labels` 顺序。
2. **上限 3 枚**：超过按 400 拒绝，前端按钮在 3 枚时禁用。
3. **空文案丢弃**：保存时去掉文案为空的行；颜色允许为空（沿用现状），存储为不带 `#` 的小写 6 位 hex，非法颜色落空串。
4. **请求语义**：`labels` 为完整有序列表（含首枚），省略表示沿用旧字段与既有附加角标；提供时整体替换，首枚同步写回 `label`/`color`。
5. **渲染沿用 `friends_and_posts.js` 现有规则**：浅色底配深字、中饱和配白字，无需新样式。
6. **Console 角标文字颜色按亮度切换**（亮度 > 0.62 用深字），否则浅色角标上的白字无法阅读；该内联文字色不改变 `ui-restraint` 对状态底色的限制。

## 实现与验收

参照图：`docs/baselines/BLG-20261111-02-friends-multi-labels/reference.webp`（用户提供，一条友链挂「社区先锋」「老友」两枚角标）。

| 步骤 | 用户效果 / 实现范围 | 验收情形与预期 | 当前结果及证据 | 状态 |
| --- | --- | --- | --- | --- |
| T1 | 数据层：`links.labels` 列 + 迁移 + 编解码 | 旧库补列后可读写；旧单角标数据读出来仍是 1 枚 | `internal/store/link_test.go` 迁移/归一化用例；`go test ./internal/store/` | 已验收（实现侧） |
| T2 | Console API：读写 0–3 枚、校验、附加角标保留 | 传 3 枚成功；传 4 枚 400；旧客户端只传 label/color 不清空附加角标 | `modules/console/api/link_labels_test.go`（全局 `make test` 通过） | 已验收（实现侧） |
| T3 | Console 弹窗：角标行可增删，最多 3 | 3 枚时“添加角标”禁用；删除后保存按序落库；手机不横向溢出 | 浏览器实拍 + 脚本日志；`after-console-editor-1280/390.webp` | 待体验 |
| T4 | 聚合与前台：`labels` 按序输出并渲染 | `/api/friends` 返回 2–3 枚；`/friends` 按序显示、颜色一致 | `after-friends-page-1280.webp`、`after-friends-page-390.webp`；`internal/friends/labels_test.go` | 待体验 |
| T5 | 回归：旧数据、失效标签、类型检查 | `make test` 全绿；检测自动打/摘「失效」仍工作 | `make test` 三次通过（含最后一次 Console 改版后） | 已验收（实现侧） |

## 执行与验证记录

| 日期 | 代码版本 / 候选差异 | 环境与运行对象 | 命令或实际操作 | 结果 / 证据位置 |
| --- | --- | --- | --- | --- |
| 2026-11-11 | 实现前 | 根 dev 工作树 | 用 `rg` 检索全仓与本地库，确认标签链路与现有单角标实现 | 见「已确认决定」 |
| 2026-11-11 | 候选改动 | worktree，`make test` | `make test`（首次） | 失败：`TestStellarMainImagesActualSSRAndReadOnlyDryRun` 报 `no such column: labels`——`schema.go` 基础建表未含新列；补进基础 DDL 后通过 |
| 2026-11-11 | 候选改动 | worktree，`console-test` | `pnpm run test` | 失败：`ui-restraint` 守卫只豁免 `normalizeColor(l.color)`；把豁免扩成 `(?:l\|item).color` 后 570 项全过（友链角标是数据驱动配色，不是状态色） |
| 2026-11-11 | 候选改动 | 隔离站点 `127.0.0.1:8097`（独立 DB，admin/e2e-password） | Playwright（本机 Chrome）登录 Console → 建 3 条友链（3/2/1 枚角标）→ 截图 | `after-console-list-1280.webp`：3 枚角标按序显示，浅色底深字 |
| 2026-11-11 | 同上 | 同上 | 打开编辑弹窗 | `after-console-editor-1280.webp` / `-bottom` / `-390`；日志：`label rows: 3`、`add button disabled at 3 labels: true`、手机 `scrollWidth == clientWidth == 356` |
| 2026-11-11 | 同上 | 同上 | 删掉第 3 枚 → 保存 → 读接口 | 日志：`labels after removing third: [["社区先锋","老友"], ...]` |
| 2026-11-11 | 同上 | 同上 | 访问 `/friends`（1280 / 390） | `after-friends-page-1280/390.webp`；日志：`public labels: ["社区先锋 老友 伙伴", "老友 推荐", "朋友"]` |
| 2026-11-11 | 同候选 | worktree | `make test`（二次、三次，最后一次含 Console 改动） | 全绿 |
| 2026-11-11 | 同上 | 同上 | `make console` | Console/UC 产物构建成功（浏览器验收用的就是这份产物） |

## 提交与集成

| 提交 | 行为变化与原因 | 验证 | 合并 / 发布情况 |
| --- | --- | --- | --- |
| 待提交 | 友链多角标（数据层 + API + Console + 前台渲染 + 文档） | `make test` + 浏览器实拍 | 未合并 |

## 交付与恢复

- 体验入口和操作步骤：Console → 友链 → 编辑 → 「角标（最多 3 枚）」；前台 `/friends`。
- 尚未完成 / 尚未验证：未在真实站点做深浅色外观各跑一遍（前台文字对比度沿用主题既有规则）；未验证导入友链是否应把 issue 多标签映射成多角标（本次明确不做）。
- 用户确认：未确认（待体验）。
- 下一步和恢复任务时首先核对的内容：worktree 路径、HEAD、未提交改动、`make test` 结果、`docs/baselines/BLG-20261111-02-friends-multi-labels/` 证据是否仍对应当前产物。
- 回退 / 数据保留：`labels` 为新增可空列，回退代码后 `label`/`color` 仍有效；`labels` 数据保留但不再渲染。
- 本任务 worktree 收尾：合入并验收后清理。
