# Worktree 汇总执行记录

日期：2026-09-12。起点 dev：01202263cafe329d5a26f7ac5da55c309eb9abe9。
本次只涉及本地整合、提交与归档清理，不推送或部署。

## T1 保存工作

归档：`../Blog-worktree-archives/20260912-130425-consolidation/`（仅当前用户访问）。
14 个旧 worktree 均保存 metadata、暂存/未暂存/完整二进制补丁、文件归档和逐文件 manifest。
排除 node_modules、Go 二进制和 Console 构建产物；保留 .tmp、data、配置、源码、截图和符号链接本身。
已逐一解包比对 SHA-256/模式/链接，并从原 HEAD 应用补丁比对跟踪文件；SHA256SUMS 全部通过。
归档时 lsof 未发现旧 worktree 文件引用。根目录 .gitignore 校验值保存在归档中，未改动。

## T2 整合 A/B 和 C/D/E

来源：B implementation 相对 2cc0371 的未提交差异和新增文件；不覆盖 dev 整树。
A auth 的 9 文件与 A 汇总版相同；A 的 37 文件中 35 个与 B 相同，另外两处是恢复路由与事务修订扩展。
A ownership/url 以及 B background/console/restore/revisions/settings 为分工中间版本：采用汇总版的真实作者 ID、特殊 slug 编码、附件错误传播、恢复前像/输入校验、后台世代、配置原子发布和事务修订；原副本完整归档。
保留 dev 的 C 无 sudo 更新与状态、D Console/YAML 契约、E Wiki/相册及 shortcode 能力。

### UI 固定验收

唯一参照：`docs/baselines/phase-b-restore/reference-phase-a-1440.png`（迁入 A/B 历史基线）。
新截图放 `docs/baselines/worktree-consolidation/`，历史截图不覆盖。
已查阅共享设计库的 SKILL、README、tokens、组件索引、button/card/input 定义及预览、CSS 和网站 kit；仅沿用既有深色语义组件。
- [x] 导出/导入与恢复确认入口保留。
- [x] 恢复处理中禁止重复提交，刷新继续查询，结束后可重新登录查询结果。
- [x] 390/768/1440 页面可操作、无横向溢出。
- [x] 当前 SPA 重新构建和测试服务重新启动；记录实际加载资源。

## T3 验证与合入

本次结果在下方记录；phase-a/phase-b 文档是历史证据，不是本次通过记录。

## T4 归档清理

仅在 T3 通过、归档复核和无活动写入后移除明确的旧 worktree，再删对应分支；不跟随或删除 resource 链接目标。

## 本次整合决定与证据（整合分支）

- 11 个重叠文件逐项三方合并；service、settings handler、settings API client、tygo 四处文本冲突手工合并。生成声明由 Go 重新生成。
- 更新锁顺序：maintenance.lock -> update.lock -> 恢复日志校验 -> 请求消费；锁等待时保持同一进程和原请求，采用 O_NOFOLLOW/普通文件/单链接校验与 fd 继承。恢复日志阻塞以 75 退出。
- 独立审查指出普通失败自动重试会重复部署坏版本，已修正。systemd 252 实测不允许 oneshot + RestartForceExitStatus；最终采用 Type=exec、Restart=no、RestartForceExitStatus=75、RestartSec=30s，隔离验证通过。
- 配置保存保留串行快照发布、密钥与 YAML 未知字段；新增 unknown YAML 和恢复预约阻止更新入队的组合回归。
- 768px 统计卡片由四列挤压改为两列（四列起点从 sm 改为 lg），新增标签高度断言；保留 390/1440 布局。已查看三档新截图，768 修正后再次查看。

| 本次命令/验证 | 实际结果 | 日志（归档 evidence 同名保留） |
| --- | --- | --- |
| go generate ./modules/console/api | PASS，新增恢复类型，保留更新/样式类型 | generated.d.ts |
| make test | PASS，Console/资源/部署/模块检查、vet、Staticcheck、Go 全套 | blog-consolidation-test.log |
| 计划列出的完整 go test -race 包集合 | PASS，API 153.172s | blog-consolidation-race.log |
| 新增 YAML/更新预约 targeted race | PASS | blog-consolidation-combination.log |
| make build | PASS，Console/UC/Go 真实构建 | blog-consolidation-build.log |
| pnpm run lint；改动文件 prettier --check | PASS | blog-consolidation-lint.log / blog-consolidation-prettier.log |
| pnpm run e2e | PASS 29/29，49.5s | blog-consolidation-e2e-browser.log |
| 恢复页面断点修改后针对性 E2E | PASS 2/2，23.3s | blog-consolidation-restore-final.log |
| bash deploy/tests/restore-systemd.sh 最终版本 | PASS，真实隔离 systemd 的等待/75退避/exit1不重试及 Linux 恢复测试 | blog-consolidation-systemd-verified.log |
| git diff --check | PASS | 命令退出码 0 |

首次 E2E 因 Chromium v1234 未安装而无法启动，安装匹配版本后重跑通过；不把首次失败统计成产品失败或通过。systemd 调整过程的失败原因和最终验证如上，不删除失败记录。

最终状态以归档内 `final-receipt.json` 为准：该回执在本文件末次提交、门禁和临时整合工作树移除后生成，包含最后 HEAD、worktree 列表和 .gitignore 校验。

### 合入前补齐测试隔离

现有 E2E 仅隔离 DB，根 worktree 的 app.yml/附件路径可能仍被测试使用，因此将测试运行 root 整体设为 `.tmp/e2e-server.*`，仅链接模块资源与内部只读资源；config、data、content 全部使用测试目录。首次隔离启动发现 icons/font 资源定位依赖 root，补齐内部资源链接后全量 E2E **29/29 PASS，48.2s**（`blog-consolidation-e2e-isolated-final.log`）。
新增 `BLOG_E2E_SCREENSHOT_DIR` 仅控制测试截图位置，供合入后复验使用，避免修改已提交基线。
合入前重新核对 14 个旧目录：HEAD、分支、补丁、逐文件 manifest 与归档一致，未发现 lsof 文件引用。

## 合入后核验与清理记录

- `f6618ae` 汇总 A/B 与 C/D/E；`54c5e9e` 隔离 E2E 配置与附件。通过 `make merge-check` 快进进入根 dev；feature 与合入后 dev 的 `make test` 均通过。
- 根 dev 的 `make build` PASS（`blog-consolidation-dev-build.log`）；随后以独立临时运行 root 重新启动测试服务，全量 E2E **29/29 PASS，50.9s**（`blog-consolidation-dev-e2e.log`）。SPA 入口包含 `index-D917ajRk.js`，测试不复用现有服务。
- 新截图位于 `docs/baselines/worktree-consolidation/`；维护提示截图使用受控任务响应，真实恢复由第二项恢复用例独立验证。合入后截图保存到 `.tmp/consolidation-postmerge-screenshots` 并归档，不覆盖受跟踪基线。
- 清理前再次验证原 SHA256SUMS、14 个目录的 HEAD/分支/补丁/完整文件 manifest 与无活动文件引用；已逐个 `git worktree remove --force` 后 `git branch -d` 删除这 14 个旧临时目录及分支（`blog-consolidation-cleanup.log`）。未跟随 resource 链接，主目录共享资源保留。
- 本记录末次提交前仅余 Blog/dev 与 Blog-worktree-consolidation；末次文档提交同样通过 hooks 和 merge-check 后移除整合工作树，最终结果写入归档 `final-receipt.json`，不再修改仓库。
- 根 `.gitignore` 内容校验与 T1 相同，未暂存或覆盖。无推送、发布或生产部署；生产服务和真实用户数据未用于测试。
