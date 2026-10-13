# 隔离迁移执行证明

`BuildMigrationFixture(ctx, reviewedCandidate, approval, notebookDecisions)` 仅从私密候选快照创建全新的系统临时 root，不接受目标路径、源路径或现有可写数据库。返回路径只指向本次新建目录；失败及取消撤回本次新目录，不能覆盖现有文件。它不是生产 Apply，也不授权真实迁移。

批准必须显式 `Approved=true`，并绑定 `SourceDigest` 与 `CandidateDigest(candidate)`。执行前重新生成同源、同 capabilities 的完整 Preview，并逐字摘要比较，拒绝删除诊断、缺记录或篡改字段。消费者只能由主 agent 已实测的能力声明闭合；policy/schema/structure 不可由 capabilities 闭合。

每项待决诊断使用 `MigrationResolution{Source,Code,Path,Action}` 唯一具名。当前支持：

- PJAX/data_cache 的 `use_official`：明确批准官方策略，源设置留私密快照。
- Wiki name/title、集合首页、banner/cover、已知 shelf name/title 的 `choose_alternative`：`AlternativeName` 必须来自该诊断的已审候选，且只应用相应 canonical 字段；冲突决议失败。
- 三类占位 Footer URL 的 `remove_placeholder` / `replace_url`：只修原值 `#` 的具名项，随后重新 strict 校验。
- 旧 Notebook/Note 的 `use_notebook_decisions`：必须先通过完整 `NotebookStructureDecisions` 校验。
- tracked `status` 的 `preserve_status`：确认原状态与 `blog.status` 一致，并实际序列化、解析验证正文与状态。

未知诊断或尚无正式消费者契约的 home cover 继续阻断。原诊断保存在私密批准归档的 resolution 记录中；解决后的候选仍运行完整 strict/schema/profile/membership/source 完整性校验。只改变 canonical v2 字段；任何候选正文与源正文不同，须有逐记录 `BodyDigests[source]` 精确批准，不自动接受正文转换。

SQLite 全表恢复、Notebook 结构转换、canonical 配置与内容写入共享一个事务，外键延迟到提交时检查。不调用分散的 Update/SaveStellarV2 方法，不改状态、用户、评论、权限关系或已有日期。完整原行与 Notebook 未知列保留在 0600 私密源归档。tracked 文档只写新 root。新 root 包含隔离的 DB/config/content/uploads/cache/runtime 位置，目录 0700、文件 0600，seed 和自动备份关闭。

快照格式 3 区分 table/virtual/shadow 对象，BLOB 使用明确 base64 编码；每个值另外捕获 SQLite storage class，避免同数值 INTEGER/REAL 相混。普通 rowid 表捕获可访问的隐藏 rowid 并显式恢复；WITHOUT ROWID 表使用原主键。所有 rowid 别名均被列遮蔽时拒绝捕获，不能猜测；缺 rowid/类型元数据及历史格式拒绝执行，不隐式兼容。JSON 往返及 SQL 绑定保持字节、大整数与原存储类型。旧 string BLOB、孤立 shadow 与附加 SQL 语句拒绝执行。FTS5 roots 先由 SQLite 建立，校验 shadow 类型后恢复其捕获行，从 shadow 保留 rowid 与搜索索引；external-content 的普通内容表同时保留隐藏 rowid，使关联不变。未支持 virtual module 失败。

`SnapshotCoverage` / `ExcludedResources` 明确说明：当前快照没有索引、视图、触发器、上传文件字节、原 app/site 文件、运行时缓存或未跟踪内容。只创建必要的新 Notebook 索引；不得把完整运行位置隔离说成复制了全部资源，也不得用隔离 Store/embedded SSR 通过宣称真实前台、媒体或外部服务已验收。

本模块测试全用系统临时夹具，覆盖批准缺失/摘要变化/诊断篡改/缺记录、未知政策与 strict URL 拒绝、Notebook 逐记录字段、正文批准、事务中途失败与取消、BLOB/大整数、稀疏 rowid、external-content FTS MATCH、同数值双 storage class、BOOLEAN/TIMESTAMP 原值与正式 Store 读取。真实源仍须重新只读捕获和完整用户审查；编码升级会改变 SourceDigest，不能把旧摘要沿用为新批准。DB 原文件哈希是独立事实，应分别核验。
