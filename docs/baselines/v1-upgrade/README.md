# 1.x 升级与 Markdown 导入验收

状态：T1、T2 待体验；服务器升级实现中。T1 为旧配置迁移命令，T2 为旧 Markdown 显式转换；真实服务器升级依赖 SSH 认证、线上 Compose 与数据目录确认。尚未操作生产数据。

## 界面参照与冻结差异

唯一视觉基线为本目录 `before-post-editor.jpg`，来自 `dev@9616908e` 的真实构建和隔离站点（非真实文章）。保留编辑器工具栏、正文、侧栏、主题 token 与备份页结构。按本地 `resource/apple-design/library-consumption.json` 阅读设计库后，复用现有 `VModal`、`VButton`，只修改导入流程：

- T2 工具栏原导入按钮先打开格式选择，可选择严格 2.0 或明确的 1.x 转换。
- T2 复用现有弹窗结构显示可滚动正文、配置、转换说明，取消不回填；确认只回填，不保存、发布或修改已有文章 URL。
- T2 备份页只增加“从 1.x 格式转换”复选框；确认说明状态与覆盖影响，显示后端具体错误。
- 桌面与 390px 宽度检查弹窗边界、滚动、操作按钮及键盘关闭。

## 联合验收

- [x] T1 默认旧库预览、显式能力覆盖、具名决策、摘要批准、完整演练与隔离 apply/recover。
- [x] T1 未确认正文、来源变化或篡改的报告不得通过批准；初次预览不得自动批准。
- [x] T2 严格导入继续拒绝旧字段；显式转换支持旧状态、分类、置顶、日期与 slug。
- [x] T2 预览和取消均不写库；保存草稿、重开、发布及公开页形成闭环。
- [x] T2 单篇与 ZIP、同 slug、未知字段、整批失败不写入；严格模式保留 2.0 语义，旧模式统一转换旧 image 默认行为。
- [x] 构建真实 Console 并重启隔离 Go 服务，核对实际服务的入口与资源 hash。

上述勾选表示适用技术验证已完成，用户尚未确认验收。所有内容和账户均为隔离合成夹具，不代表用户旧库与文章已迁移。

## 本次证据（2026-10-19）

- `make test` 通过：Console 273、Stellar 220 项，零失败、零跳过；含部署、来源、模块检查及 Go vet、Staticcheck、全部 Go 测试。
- `TestStellarMigrationActualBinary` 在测试内重新编译单二进制，覆盖批准、离线门禁、演练、Apply、服务启动、搜索与恢复；不是 Docker 容器实测。
- `make build` 通过：真实 Console/User Center 构建与 Go 编译。修复独立审查发现的旧 image 转换不一致后，相关转换测试通过。
- `pnpm exec playwright test tests/e2e/editor-md-import.spec.ts tests/e2e/backup-import.spec.ts`：6 项通过。覆盖预览取消、确认前无写入、草稿重开和不公开、显式发布后前台展示、手机页面取消、严格入口拒绝旧字段、批量旧文件缺省草稿。
- 主 agent 亲自用真实浏览器检查 1280×900 与 390×844：长文章可滚动，手机预览边界为 x=16..374、y=72..772；两个底部按钮为 y=711..755；Escape 关闭且不回填。旧页面回填后保存为草稿，重开保留标题、Slug、2020-02-03 与正文。
- 界面截图：`after-import-desktop.jpg`、`after-import-mobile.jpg`、`after-page-saved-mobile.jpg`、`after-backup-desktop.jpg`。最终服务入口和主资源均与当前 dist 逐字匹配，SHA-256 见本目录 `served-assets.json`。
- 独立只读审查发现 2 个 P2，均已修正：仅含 image 的旧文档保持旧 fancybox 默认行为；升级指南明确 rootful Docker 私密审查文件的 `sudo` / `sudoedit` 操作。

## 剩余问题

- `ubuntu@124.220.55.80` 的 SSH 身份验证被拒绝；尚未取得容器、镜像摘要、挂载、旧版本、文章目录或用户样本。
- 本机 Docker daemon 未运行，Docker/Compose 命令仅按仓库 Dockerfile 和迁移 CLI 契约核对，没有完成容器实测。
- 生产停服、完整备份、实际旧库预览与选择、迁移、启动及真实文章导入均未执行。指南位于 `deploy/upgrade-v1-to-v2.md`，必须按实际服务器挂载调整。

并行归属：迁移 CLI 专家独占 sibling `Blog-v1-upgrade-migration` 的 CLI 与其测试；导入后端专家独占 sibling `Blog-v1-upgrade-import` 的转换、backup 与 API；主 agent 独占 `Blog-v1-upgrade` 的 API client、界面、E2E、升级指南与最终集成。主 agent 核对补丁、执行组合门禁并亲自检查真实浏览器。
