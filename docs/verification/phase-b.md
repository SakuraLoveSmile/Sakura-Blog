# 阶段 B：实施与验证报告

日期：2026-09-08。交付工作树：`../Blog-phase-b-implementation`，分支 `codex/phase-b-implementation`。

## 范围与交付状态

B1、B2、B3 已实现，未提交、合并、推送或部署。该工作树包含阶段 A 的 37 个未提交依赖文件；下面文件清单按与 `../Blog-phase-a-implementation` 的内容差异统计，避免将 A 重复算作 B。根 dev 仍为 `2cc0371`（ahead 13），用户 `.gitignore` 改动保留。

## 实现决定

### B1：整站恢复

- 上传/快照先完整暂存；检查 ZIP 路径、精确重复及大小写冲突（含隐式父目录）、符号链接、扩展名、大小、YAML、目标权限、空间和数据库结构。大小写冲突跨平台保守拒绝，避免更换文件系统后覆盖。
- 一个维护协调器覆盖 HTTP 及后台写入；先拒绝新业务并排空存量，再制作 DB/uploads/site.yml 完整前像。保留 livez；readyz 和业务请求维护时 503，冲突提交 409。公共状态不含任务或用户细节。
- 版本 1 原子日志与文件/目录 fsync；所有内部路径由合法 task ID 派生。SQLite 用现有连接表级事务恢复；附件在目标文件系统切换；既有 site.yml 原位同步写入并保留权限。
- 提交/必要重建失败补偿完整前像；补偿失败保持维护。启动补偿先于 YAML 解析和数据库迁移。终态日志同步结果不确定时保持维护，不冒险再次回滚已可能成功记录的状态。
- 维护内完成运行配置、存储、模块状态、FTS、缓存失效；RAG 先清空持久化及内存派生内容，再于窗口外重建，防止旧私密内容继续检索。普通通知不决定恢复成败。
- 新异步任务接口需要 Idempotency-Key；重复键/相同来源返回原任务，不同来源冲突；旧同步接口保留。账号从备份恢复后重新认证，无临时管理员绕过。
- 保留最近三份成功任务完整前像；未完成/补偿失败前像不自动删除；旧 `.pre-restore-*.db` 仍可读。
- update.sh 与应用共享 no-follow 文件锁；root 首次创建时通过文件描述符设置 blog 属主。保留 NoNewPrivileges 和 ProtectSystem=strict，仅增加既有 site.yml 写权限。阶段 C 的 systemd.path 更新传输机制尚未实施。

### B2：设置一致性

完整读取、应用输入、密钥保留及候选验证在同一串行锁中；SQLite UPSERT 成功后发布已构造快照，失败不发布。所有配置重建入口使用相同协调。存储配置先准备后端再提交；JSON/YAML 设置上限 1 MiB，超限 413，保留显式空值/false/0 语义。

### B3：正文和修订

事务内读取旧正文，更新正文、关联、slug 并插入和裁剪修订（最多 20 个）；任一步失败整体回滚。事务成功后单次内容事件携带 PreviousSlug，清理旧 slug 的搜索/OG/RAG 派生状态。

## 实际验证

| 命令/验证 | 结果与证据 |
| --- | --- |
| `make test` | PASS：Console 173、Stellar 74，部署/发布来源/模块边界检查，Go vet、Staticcheck、Go 全套测试；最终日志 `/tmp/blog-phase-b-delivery-final-test.log` |
| `go test -race ./internal/backup ./internal/maintenance ./internal/config ./internal/rag ./internal/module/perf ./internal/app ./modules/console/api` | PASS，API 146.309s；`/tmp/blog-phase-b-final-race.log` |
| 更早的广域 race | PASS：包含 store、后台服务及 Stellar 路由，`/tmp/blog-phase-b-race.log`；最新改动的包以上一行复跑覆盖 |
| `go test ./modules/console/api -run TestAsyncRestore -count=1` | PASS：必需幂等键、缺失任务、JSON 413/多对象/来源歧义/空来源拒绝，未创建任务 |
| `go test ./internal/backup` | PASS：包含大小写文件和父目录冲突，业务三部分不变 |
| `bash deploy/tests/restore-systemd.sh` | PASS：独立 Debian/systemd Docker 容器，真实加固权限、root/blog 互斥锁、Linux backup/store/app 测试与跨文件系统 uploads；`/tmp/blog-phase-b-final-systemd.log` |
| `pnpm run e2e` | PASS 29/29，42.4s，实际重建 SPA 并启动 Go 服务；`/tmp/blog-phase-b-final-e2e.log` |
| `pnpm exec playwright test tests/e2e/restore-task.spec.ts` | PASS 2/2，19.4s，截图脚本修正后重新验证；`/tmp/blog-phase-b-final-ui2.log` |
| `pnpm run lint` | PASS |
| 变更前端文件 `prettier --check` | PASS：BackupView、settings API client、生成声明、恢复 E2E |
| `pnpm run format:check` | FAIL：仍为 49 个既有文件差异，未扩大范围批量格式化；`/tmp/blog-phase-b-full-format.log` |
| `go generate ./modules/console/api` | 已执行，生成 RestoreTaskResponse 等契约 |
| `make build` | PASS：真实 Console/UC 构建与 Go 二进制；主 SPA `index-DTLj0e1J.js`；`/tmp/blog-phase-b-delivery-build.log` |
| `git diff --check` | PASS |
| 两轮独立源码复审 | 实质发现已修正并复核：终态等待竞态、大小写路径、旧 perf 桶、密钥预填、root 锁属主、RAG 旧内容 |

恢复故障测试覆盖五个提交检查点、必要重建失败、补偿失败后维持维护及下次启动重放、终态 rename 后同步错误、六个子进程强制退出检查点、损坏日志、无权限和空间不足、幂等重放及三份前像保留。Linux 权限测试不改宿主机 systemd，不挂载生产数据。

## 验证边界与后续风险

- 进程中断和 I/O 故障注入不等同于物理断电；未实测磁盘控制器/文件系统掉电语义。
- 未进行生产恢复、生产部署、远端 CI 或数据迁移。部署前需按部署文档核对 DB 路径、共享锁路径与既有 site.yml 权限；不要对未完成日志降级运行旧版本。
- 响应式截图显示 768px 原有统计卡片挤压；记录于基线说明，留待阶段 D。未宣称完成全站键盘/响应式整改。
- 本任务使用本地文件系统的完整 uploads 前像；外部对象存储对象本身不包含在现有整站 ZIP 契约中。
- 每次完整前像需要额外磁盘空间；预检采用保守估算，提交阶段 I/O 仍可能失败并走补偿。
- 本次日志以安全任务状态对外，底层故障需查服务端日志；日志和前像目录需要持续保留以支持补偿。

## B 文件清单与职责

| 文件 | 修改原因 |
| --- | --- |
| `cmd/blog/main.go` | 启动前补偿、依赖重建及维护入口装配 |
| `deploy/README.md` | 单文件写权限、恢复更新互斥及部署说明 |
| `deploy/blog.service` | 单文件写权限、恢复更新互斥及部署说明 |
| `deploy/tests/restore-permissions.py` | 对应行为、失败路径或边界条件回归验证 |
| `deploy/tests/restore-systemd.Dockerfile` | 对应行为、失败路径或边界条件回归验证 |
| `deploy/tests/restore-systemd.sh` | 对应行为、失败路径或边界条件回归验证 |
| `deploy/update.sh` | 单文件写权限、恢复更新互斥及部署说明 |
| `docs/baselines/phase-b-restore/README.md` | 可复核的验收说明 |
| `docs/baselines/phase-b-restore/active-1440.png` | 固定参照或响应式验收截图 |
| `docs/baselines/phase-b-restore/active-390.png` | 固定参照或响应式验收截图 |
| `docs/baselines/phase-b-restore/active-768.png` | 固定参照或响应式验收截图 |
| `docs/baselines/phase-b-restore/reference-phase-a-1440.png` | 固定参照或响应式验收截图 |
| `internal/app/restore.go` | 启动前补偿、依赖重建及维护入口装配 |
| `internal/app/restore_test.go` | 对应行为、失败路径或边界条件回归验证 |
| `internal/app/services.go` | 启动前补偿、依赖重建及维护入口装配 |
| `internal/backup/backup.go` | 恢复暂存、预检、任务日志、完整前像及补偿 |
| `internal/backup/restore.go` | 恢复暂存、预检、任务日志、完整前像及补偿 |
| `internal/backup/restore_execute.go` | 恢复暂存、预检、任务日志、完整前像及补偿 |
| `internal/backup/restore_files.go` | 恢复暂存、预检、任务日志、完整前像及补偿 |
| `internal/backup/restore_prepare.go` | 恢复暂存、预检、任务日志、完整前像及补偿 |
| `internal/backup/restore_recovery_test.go` | 对应行为、失败路径或边界条件回归验证 |
| `internal/backup/restore_tasks.go` | 恢复暂存、预检、任务日志、完整前像及补偿 |
| `internal/backup/restore_test.go` | 对应行为、失败路径或边界条件回归验证 |
| `internal/config/reload_rebuild.go` | 串行构造候选、持久化后发布与密钥保留 |
| `internal/config/reload_sections.go` | 串行构造候选、持久化后发布与密钥保留 |
| `internal/config/reload_test.go` | 对应行为、失败路径或边界条件回归验证 |
| `internal/content/events.go` | 后台写入维护协调、旧缓存失效或设置体积限制 |
| `internal/friends/friends.go` | 后台写入维护协调、旧缓存失效或设置体积限制 |
| `internal/linkwatch/linkwatch.go` | 后台写入维护协调、旧缓存失效或设置体积限制 |
| `internal/maintenance/coordinator.go` | 统一维护许可、HTTP 拒绝与进程间互斥 |
| `internal/maintenance/coordinator_test.go` | 对应行为、失败路径或边界条件回归验证 |
| `internal/maintenance/filelock.go` | 统一维护许可、HTTP 拒绝与进程间互斥 |
| `internal/maintenance/filelock_test.go` | 对应行为、失败路径或边界条件回归验证 |
| `internal/maintenance/http.go` | 统一维护许可、HTTP 拒绝与进程间互斥 |
| `internal/module/manager_test.go` | 对应行为、失败路径或边界条件回归验证 |
| `internal/module/module.go` | 后台写入维护协调、旧缓存失效或设置体积限制 |
| `internal/module/perf/collect.go` | 后台写入维护协调、旧缓存失效或设置体积限制 |
| `internal/module/perf/perf_test.go` | 对应行为、失败路径或边界条件回归验证 |
| `internal/module/restore.go` | 恢复模块状态并可补偿生命周期变更 |
| `internal/module/seo/push.go` | 后台写入维护协调、旧缓存失效或设置体积限制 |
| `internal/module/seo/settings.go` | 后台写入维护协调、旧缓存失效或设置体积限制 |
| `internal/module/storage/storage.go` | 存储后端先准备、持久化成功后发布 |
| `internal/module/storage/storage_test.go` | 对应行为、失败路径或边界条件回归验证 |
| `internal/notify/service.go` | 后台写入维护协调、旧缓存失效或设置体积限制 |
| `internal/rag/rag.go` | 维护协调、旧队列失效及恢复前清空派生索引 |
| `internal/rag/rag_test.go` | 对应行为、失败路径或边界条件回归验证 |
| `internal/search/restore.go` | 必要索引重建错误传播 |
| `internal/storage/storage.go` | 存储后端先准备、持久化成功后发布 |
| `internal/store/maintenance.go` | 后台写入维护协调、旧缓存失效或设置体积限制 |
| `internal/store/post.go` | 正文和旧正文修订的原子事务 |
| `internal/store/post_test.go` | 对应行为、失败路径或边界条件回归验证 |
| `internal/store/restore.go` | SQLite 完整预检、兼容检查及表级事务恢复 |
| `internal/store/restore_validation_test.go` | 对应行为、失败路径或边界条件回归验证 |
| `internal/store/revision.go` | 正文和旧正文修订的原子事务 |
| `modules/console/api/backup.go` | 恢复任务兼容 API、设置校验或更新互斥 |
| `modules/console/api/backup_restores.go` | 恢复任务兼容 API、设置校验或更新互斥 |
| `modules/console/api/backup_restores_test.go` | 对应行为、失败路径或边界条件回归验证 |
| `modules/console/api/post.go` | 单次提交后事件及旧 slug 索引失效 |
| `modules/console/api/router.go` | 恢复任务兼容 API、设置校验或更新互斥 |
| `modules/console/api/settings.go` | 恢复任务兼容 API、设置校验或更新互斥 |
| `modules/console/api/settings_test.go` | 对应行为、失败路径或边界条件回归验证 |
| `modules/console/api/update.go` | 恢复任务兼容 API、设置校验或更新互斥 |
| `modules/console/tests/backup-restore.test.ts` | 对应行为、失败路径或边界条件回归验证 |
| `modules/console/web/src/modules/backup/BackupView.vue` | 异步恢复界面、客户端契约及生成类型 |
| `modules/stellar/routes/mount.go` | 单次提交后事件及旧 slug 索引失效 |
| `modules/stellar/routes/pageview_buffer.go` | 后台写入维护协调、旧缓存失效或设置体积限制 |
| `modules/stellar/routes/pageview_buffer_test.go` | 对应行为、失败路径或边界条件回归验证 |
| `modules/stellar/routes/search_index_test.go` | 对应行为、失败路径或边界条件回归验证 |
| `packages/api-client/src/api/settings.ts` | 异步恢复界面、客户端契约及生成类型 |
| `packages/api-client/src/generated.d.ts` | 异步恢复界面、客户端契约及生成类型 |
| `tests/e2e/restore-task.spec.ts` | 异步恢复界面、客户端契约及生成类型 |
| `tygo.yaml` | 异步恢复界面、客户端契约及生成类型 |
