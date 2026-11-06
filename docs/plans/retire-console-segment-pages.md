# BLG-20261106-02：Console 旧分段页面退役与侧栏入口清理

- 项目 / 任务编号：Blog / BLG-20261106-02
- 状态：实现中
- 创建日期（Asia/Taipei）：2026-11-06
- 权威记录：本文件
- 主仓库 / 当前工作目录：`/Users/sakurasep/Documents/Code/Project/Personal Project/Blog`（根 `dev` 工作树，保留用户原有未提交的文档改动）/ `/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261106-fix-blg-20261106-02-retire-segment-pages`
- 基线 / 分支 / 合入目标：`dev` / `2ad23a62`（= `origin/dev`）/ `codex/fix/20261106-blg-20261106-02-retire-segment-pages` / 本地 `dev`
- 当前 HEAD / 未提交差异或快照：`2ad23a62` + 本任务未提交改动（见「提交与集成」）
- 运行资源：不需要常驻服务；验证只用 `make test` / `make merge-check` 与本机 pnpm/npm 测试。

## 目标与范围

用户在 Console 里不再遇到“死胡同”式的旧配置页：侧栏不再挂「主题设置」「菜单」「作者」三个空壳入口；旧地址（含旧书签、旧通知里的链接）仍能打开，并直接落到系统设置对应页。

- 覆盖：Console 侧栏 manifest（Go `internal/module/core` 与前端 core plugin）、`/console/appearance`、`/console/menus`、`/console/authors` 三个路由、对应三个空壳页面组件、受影响的 Console 行为测试与 Playwright 用例、`CLAUDE.md` Console 章节。
- 明确排除：系统设置页的内部信息架构与字段（保持 settings-ia 已验收的 5 组 12 页）；`/console/friends` 页内嵌的友链设置提示（不是侧栏入口，属友链页内部切换）；`modules/console/web/src/modules/appearance/` 下未被引用的旧子表单组件（本次不顺手删，避免扩大改动面）；后端旧分段接口与迁移读取路径（保持只读兼容）。

## 已确认决定

- 用户 2026-11-06 在诊断确认后指示：“确认改进 改进完成按照项目工作流发布新的测试版本，然后再更新线上”。据此授权：本地实现 → `dev` 合入 → 发布测试版本（预发布 tag）→ 生产升级。
- 保留旧地址为**重定向兼容层**而不是 404：旧书签与旧通知里的链接仍可用，重定向目标沿用设置页的页面键（`appearance`、`navigation`、`community`），旧 `/console/appearance?tab=feedback` 仍落到访客反馈页。
- 不保留空壳页面组件：页面已无任何可保存内容，保留组件等于继续保留死胡同；重定向映射集中到 `modules/console/web/src/router/retired-pages.ts`，并有单测锁定目标必须是已注册的设置页。
- 不改后端旧分段门禁（`CheckLegacySectionWrite` 继续只读兼容历史迁移预览）；本次只改 Console 入口与路由。

## 实现与验收

| 步骤 | 用户效果 / 实现范围 | 验收情形与预期 | 当前结果及证据 | 状态 |
| --- | --- | --- | --- | --- |
| T1 | 侧栏不再出现「主题设置」「菜单」（外观分组随之消失）与「作者」入口 | 登录后侧栏无这三个 href；Go manifest 与前端 core plugin 一致 | 未运行 | 实现中 |
| T2 | 旧地址 `/console/{appearance,menus,authors}` 直接落到系统设置的对应页 | 打开旧地址落到 `/console/settings?tab=appearance/navigation/community`；`?tab=feedback` 落到访客反馈 | 未运行 | 实现中 |
| T3 | 删除三个空壳页面组件，旧页面样式不再参与构建 | 仓库内无 `AppearanceView.vue`、`MenuView.vue`、`AuthorsView.vue`；`make test` 的类型检查与结构测试通过 | 未运行 | 实现中 |
| T4 | 测试反映新契约：旧入口必须不存在、旧地址必须重定向、重定向目标必须是已注册设置页 | `make test` 全绿（含新增 `retired-pages.test.ts`）；相关 Playwright 用例改断言重定向与侧栏缺席 | 未运行 | 实现中 |
| T5 | 发布测试版本 | 预发布 tag 触发 Docker Release success；镜像可启动；`latest`/`2.0`/`2.0.8` 等稳定标签 digest 不变 | 未运行 | 未开始 |
| T6 | 生产升级到测试版本 | compose 固定到新测试版本；容器 running、`/readyz` 200、Console 版本一致；记录备份与回退 | 未运行 | 未开始 |

## 执行与验证记录

| 日期 | 代码版本 / 候选差异 | 环境与运行对象 | 命令或实际操作 | 结果 / 证据位置 |
| --- | --- | --- | --- | --- |
| 2026-11-06 | `2ad23a62` + 本任务改动 | 本机 feature worktree | 代码、测试与文档改动（未提交） | 待 `make test` |

## 提交与集成

| 提交 | 行为变化与原因 | 验证 | 合并 / 发布情况 |
| --- | --- | --- | --- |
| 待补 | Console 旧分段页面退役、旧地址重定向到系统设置 | 待补 | 待补 |

## 交付与恢复

- 体验入口和操作步骤：登录 Console，确认侧栏无「主题设置」「菜单」「作者」；地址栏打开 `/console/menus` 应落到「导航与页脚」。
- 尚未完成 / 尚未验证：本地测试、`dev` 合入、测试版本发布与生产升级。
- 用户确认（明确确认后才改为已验收）：待用户体验确认。
- 下一步和恢复任务时首先核对的内容：`git worktree list`、本分支 HEAD、`docs/plans/retire-console-segment-pages.md`、`make test` 结果。
- 需要时的回退 / 数据保留方式：代码回退即恢复被删的三个组件与路由；本次无数据库 schema 变化，生产回退只需把 compose 镜像改回上一版本。
- 本任务 worktree 收尾：合并并验证完成后按规范清理（等发布与上线记录补齐）。
