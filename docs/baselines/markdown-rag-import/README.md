# Markdown 导入与 RAG 全文索引恢复

问题来源：用户上传 `Redirectstorage.md` 时，在 Console「备份与导出」报 `restore database preflight: copy table rag_chunks_fts_content: UNIQUE constraint failed`。本次只修复恢复表枚举与对应回归，不改变 Markdown 的发布状态、转换或覆盖规则。

## 修复与回归

原实现从 `sqlite_master` 枚举表，仅排除 `posts_fts` 前缀，导致先复制 RAG 虚表，再次复制 SQLite 自动维护的影子表。新实现使用 `pragma_table_list` 的真实表类型，复制普通表及虚表的逻辑行，排除影子表；站内搜索 `posts_fts` 继续按现有流程重建。未知普通表、未知虚表和未知列仍拒绝恢复。

RAG 逻辑行与向量、hash、元数据在同一事务恢复。这样原生 Store 在启动前补偿时也保持一致，不依赖应用恢复回调；在线内容导入仍按既有流程使知识库失效。

修复前新增回归实际失败：

- 非空索引恢复到非空库：`rag_chunks_fts_content.id` 重复。
- 非空索引恢复到空库：`fts5: corruption found reading blob 1`。
- 空索引恢复到非空库：`rag_chunks_fts_data.id` 重复。

修复后 `go test ./internal/rag ./internal/store ./internal/backup -count=1` 通过。覆盖源/目标空与非空的四种组合、预检不改 live、正文与向量/hash/元数据恢复、旧关键词消失、MATCH 查询、SQLite/FTS5 完整性、恢复后继续写入、索引前缀业务表与未知虚表拒绝，以及已有 RAG 时无 front matter 的严格/1.x 导入路径。

## 实际文件浏览器验证

`make build` 后启动完全隔离的 `127.0.0.1:8102`，数据库、内容、配置、上传和维护锁均使用独立测试根目录。先写入非空 RAG 索引，再在真实浏览器中从「选择文件导入」上传用户提供的原文件（严格模式）。

- 页面显示「成功导入 1 篇文章 Redirectstorage」，见 `local-import-success.png`。
- 文件无 front matter，按现有规则使用文件名作为标题和 Slug；严格模式按确认框说明在隔离库默认发布。生产没有为验证而导入或发布此文。
- 原文 17443 字节，持久化 Markdown 与文件 SHA-256 完全一致：`f990f69802fef320e3155c0557b469bf5d87ec0ea0059bf9ab5b062591a3bba1`。
- 原文 34 个图片引用全部保留为渲染图片节点；此结论不等于外部图床全部可访问。
- 编辑器显示从「作者」至「多平台发布」的完整章节，数据库 `integrity_check=ok`。在线恢复回调已正常清空派生 RAG。
- 具体结果见 `local-verification.json`。

独立源码审查未发现实质问题。仓库门禁、集成与服务器更新结果在完成后补记。

状态：实现中。
