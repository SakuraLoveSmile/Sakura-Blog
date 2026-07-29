# Spec 41 · 备份整站恢复（zip 一键还原）

- **分支**：`feature/backup-restore` · **Wave 8** · **隔离等级**：中（动 backup 包与启动逻辑边缘）
- **主模块**：`internal/backup/backup.go`（+Restore）+ `internal/api/backup.go`（+恢复端点）+ `console/src/modules/backup/BackupView.vue`
- **前置**：无

## Why

Halo 对照的半缺失项：现有备份能力齐（zip 全量导出 = DB 快照 + uploads + site.yml、Markdown 导入导出、每日自动快照），但**没有 zip 整站恢复端点**——换机迁移只能手工解包替换文件。补上恢复即闭环「备份 → 恢复」全链路。

## 功能需求

1. **上传恢复**：Console 备份页 +「从备份恢复」区——上传 blog-backup-*.zip → 后端校验包结构 → 确认弹窗（明示覆盖当前数据 + 显示包内元信息：备份时间/文章数）→ 执行恢复 → 提示完成。
2. **本地快照恢复**：快照列表（data/backups 下 .snapshot-*.db 与 zip）每项 +「恢复」按钮，同确认流程。
3. **恢复前自动兜底快照**：执行恢复前先对当前 DB 做一次 .pre-restore 快照，失败可回滚。
4. 恢复完成后：FTS 索引重建、侧边栏快照缓存失效、OG 缓存清空（复用 postWriteHook 既有失效路径或全量触发）。

## What Changes

- `internal/backup/backup.go`：+`Restore(zipPath)` / `RestoreSnapshot(dbPath)`——解包校验（拒绝 zip slip：路径穿越条目直接报错）、DB 文件替换（SQLite 连接策略见技术决策）、uploads 覆盖合并、site.yml 恢复 + 配置热更新（`internal/config/reload.go` 既有机制）。+`_test.go`（构造小 zip 全流程用例）。
- `internal/api/backup.go`：+`POST /api/v1/backups/restore`（multipart 上传）与 `POST /api/v1/backups/restore-snapshot`（body: 文件名，**白名单校验只允许 backups 目录内条目**）。
- `console/src/modules/backup/BackupView.vue`：上传区 + 快照恢复按钮 + 双重确认弹窗（输入 RESTORE 确认，参照危险操作惯例）。
- `cmd/server/`：恢复后索引重建复用 `setupSearchIndex` 的回填逻辑（抽公共函数或直接调用，改动最小化）。

## 实现步骤

1. Restore 核心：zip 校验/解包/DB 替换/回滚路径 + 单测（含损坏包、路径穿越包、超大包拒绝）。
2. 快照恢复（简单路径先通）。
3. api 端点（上传大小限制沿用 MaxUploadMB*4）+ 单测。
4. Console UI + 双重确认。
5. 恢复后失效链路（FTS 重建/缓存清理）验证。

## 技术决策（无需再问）

- **DB 替换策略**：恢复时以独立连接 `VACUUM INTO` 思路反向操作不可行，采用「关闭 store 连接 → 文件替换 → 重开连接」；`store.New` 已支持重建，恢复期间 API 短暂 503（单管理员场景可接受，不做在线热切）。
- **uploads 合并语义**：包内文件覆盖同名，现有多余文件**保留**（防误删未入包附件）；提供「严格模式」复选（先清空再解包）。
- **不做定时恢复演练/异地存储**：S3 等外部存储策略见差距清单附件分组项的后续决策，本 spec 不含。
- **版本兼容**：包内 DB schema 版本高于当前二进制时拒绝恢复并提示先升级程序。

## Impact

- **新增**：Restore 能力与两个端点
- **修改**：backup.go、api/backup.go、BackupView.vue、router.go（+2 行）
- **不影响**：自动备份、导出链路、markdown 导入

## 验证

- [ ] 导出 zip → 改数据 → 恢复 → 数据回到导出时点（文章/评论/附件/site.yml 全查）
- [ ] 恢复前自动生成 .pre-restore 快照；恢复失败自动回滚
- [ ] 路径穿越 zip 被拒；backups 目录外的快照路径被拒
- [ ] 恢复后 FTS 搜索/侧边栏/OG 图全部反映恢复后数据
- [ ] `make test` 全绿
