# Stellar 2.0 迁移候选

此包只读取真实输入、构建候选和做执行前校验，**没有修改既有数据库或真实迁移的入口**。结构证明可创建新的私密临时夹具库，不能把真实源路径传入写连接。它固定使用本仓库 Stellar 2.0.0 strict parser，不以 recovery、默认初始化或旧列推断作为运行时兼容。

## 接口与私密边界

- `ReadSnapshot(ctx, dbPath, files)`：SQLite `mode=ro`、`query_only=1`、单事务读取实际 `sqlite_master` 与动态列，另用同样只读的 `Store.DetectLegacyNotebookSchema` 共用结构契约并比对捕获列。旧表缺少新版列不会触发建表／补列。`files` 是调用者明确提供的受跟踪 Markdown 原文。
- `Preview(snapshot)`：先构建全部集合，再建立上游 Registry，逐条校验成员。返回主题 document JSON、独立 Blog JSON，以及每条 Post／Page／WikiProject／WikiDoc／Topic／Notebook／Note／文件的原文摘要、version 2、Front Matter JSON、Blog JSON、Markdown 和映射／冲突。已 version 2 的 theme／内容 JSON 与 Markdown 逐字保持；无任何存储副作用。
- `PreviewWithCapabilities(snapshot, ConsumerCapabilities{ConsumerPoster: true})`：仅调用者显式声明已经实测闭合的消费者才降为提示。catalog 固定为 `poster`、`collection_order`、`authors`、`author_widget`、`wiki_external_url`，未知 capability 报错；默认没有闭合能力。诊断全部保留，以 `kind=consumer`、`consumer=<能力>`、`severity=info`、`blocking=false` 标明调用者的声明，并不替代真实体验证明。政策／结构／strict schema 冲突不受该参数影响。
- `Candidate.PublicReport()`：只输出行数、公开源身份／路径、hash、源字段→目标字段、行为说明和无原值诊断。原始设置、正文、通知密钥、用户 email、密码哈希、AI key 和冲突值均不在报告中。
- `WritePrivate(path, candidate)`：以 `O_EXCL` 创建新的 `0600` 文件，不覆盖已有文件或跟随 symlink。候选含完整源数据，不能提交 Git、发给外部服务或复制进 public 目录。调用者应使用 `0700` 隔离输出目录。
- `ValidateCandidate(candidate, currentSnapshot)`：源 digest 必须一致，候选集合必须完整、没有未决冲突，且再次通过 theme／content／profile／membership strict 校验。源 digest 覆盖所有表及其 schema、所有关联、用户／权限、评论／草稿和明确输入文件；评论等不迁移记录的变化也使候选过期。此方法不写数据，不构成迁移授权。

主题候选保存作者 snake_case 覆盖，未覆盖部分由 `ParseTheme` 加载锁定上游的完整默认值。不会把 runtime camelCase 反写成作者配置。`json.Number`、false、null、空列表及旧未知值在源快照完整保留；未知／removed 配置需明确处理，不静默删除，也不捏造 `features.data_cache` 等字段。

## 数据与冲突规则

宿主身份、权限、slug、时间、发布／回收状态、口令、原作者关联、分类关联、草稿头、评论 `page_key`、缓存 HTML 均不在本候选修改范围。仅上游的公开展示字段进入严格 Front Matter。原生评论在 `Blog.native_comments`；文章 `Blog.native_comments.enabled=null` 表示继承，第三方评论不会接收原生身份。

公开 `_data` 只包含 `Blog.data.widgets/authors/links/wiki`；`wiki` 是官方的项目 ID 字符串 shelf 数组，旧对象的冗余字段逐条与集合表比较，相同值保持，不同来源报冲突。Hexo 宿主元数据在 `Blog.site`，原 `settings.site` 不改；显式 `search.service=go` 才转 `Blog.search.provider=go`，空旧默认转上游 local。AI 开关分别进入 `Blog.ai.summary.{enabled,allow_user_api}` 与 `Blog.ai.ask.{enabled,allow_user_llm}`，密钥保留在独立私密 Blog 配置。

`Blog.content.share/reward/poster/banner_image/ai_summary/authors/collection_order` 等保留获批准的本地能力。旧分享／打赏启用时显式 `Blog.content.enabled=true`，用于本地 content/JS 消费；它不控制 `blog_*` 标签注册，后者由 renderer 显式 `BlogExtensions:true` 注册。**保留候选值不代表模板／宿主已消费。** poster、集合排序、旧 author widget 等接通前有阻断诊断；主执行者须完成消费与体验验证后审查解除这些项。

Wiki 旧外链统一保留于 `Blog.content.wiki_external_url`，由独立模板消费；canonical `route.path/homepage` 保持本地路径，绝不塞 HTTP URL。shelf／项目表外链不同仍是来源冲突，其私密 alternatives 同样使用该扩展键。Wiki 的 name／title、项目入口重定向／官方首页、Hero／cover 的独立来源，以及文章 banner／cover 有具体私密候选 alternatives，禁止默选其中一项。旧笔记无 slug/status/author，不自行分配作者、发布或生成已存在的公开路由。INTEGER ID／`nb_order`／`content/tags` 到新版表结构是独立步骤；schema 变化后需重新生成源快照并审核内容候选，不能拿旧 digest 写入新 schema。

Markdown 仅把明确的旧本地 `about/links/moments/ghrepo/ghuser/tip` 标签改为 `blog_*`；批准的旧 tooltip 语义保留，不替换为不同的上游 inline tip。参数、正文和空白不重建。旧 image 未明确 fancybox 时添加 `fancybox:false` 保留默认点击行为。Goldmark 代码段（包括引用／列表内代码）不改；未知、跨行或引号不完整的标签原文保留并阻断。此包不假称已全面验证每个标签参数，也不净化作者 HTML。

tracked Markdown 的旧 `status` 候选映射为 `blog.status`，与宿主备份／目录导入契约一致，不进入上游 schema，也不放入 `blog.host.status`。候选保留原发布状态；`file_host_fields` 诊断仍阻断，直到宿主导入及重新打开证明不会误发布。其余权限／生命周期字段仍须单独接线，不因发布状态映射修正自动解除。

## 验证

`go test -race ./internal/stellarmigration` 覆盖独立旧／新 SQLite、只读／不存在源、version 2 原文、大整数／false／null、所有条目完整性、源评论变化失效、未知字段保留、集合归属、Wiki 两候选、旧笔记 schema、Markdown 代码保护及 private artifact 权限。预览／验证失败无既有数据库写入口，因此无真实源半批版本标记。

`TestReadOnlySourcePreviewEvidence` 默认跳过。只有显式设置 `STELLAR_MIGRATION_SOURCE` 时才读指定源；`STELLAR_MIGRATION_CONTENT_ROOT` 可提供本任务两份 tracked 验收文章；`STELLAR_MIGRATION_OUTPUT` 选择新私密目录。它比较数据库与 `-wal/-shm/-journal` 的前后 SHA256，只生成 `candidate.private.json`、`report.public.json`、`source-check.json`，不执行真实转换。

## Notebook/Note 结构证明

- `PreviewNotebookStructure(snapshot, decisions)` 是纯私密预览，输出全部目标列、映射与阻断。`decisions.SourceDigest` 绑定完整源快照；`Records` 以 `notebooks/<id>`／`notes/<id>` 为键。缺失的 `author_id`、Note `slug/status`、日期必须由对应记录的 `Approved=true` 和非 nil 值明确批准；`OwnerID=0` 也必须显式提供，不根据当前会话、时间或署名推断。已有宿主字段不能借结构转换覆写。正 owner 必须存在于源 users。
- notebook ID 转为原数值的 TEXT，Note 的 INTEGER ID 保持原整数，`notes.notebook_id` 转为父本原 ID 的 TEXT；**不是把 Note ID 改为 TEXT**。`nb_order→sort`、`content→content_md`；旧逗号标签按既有规则拆分，保留层级、顺序、重复、大小写及空片段，完整旧字符串另存私密归档。已有 `tags_json` 不重写；新旧正文、排序或标签两来源冲突继续阻断。未提供的纯可选展示字段用当前空默认，非空列中的旧 NULL 不隐式改值。
- 旧 Notebook 缺 `updated_at` 时，调用者可逐记录明确批准使用源 `created_at`，不能自动使用当前时间。现有日期逐值保留。slug 按当前宿主路径规则校验，状态只接受 draft/published，重复 slug、缺父本、无效 owner/整数/日期等阻断。
- `BuildNotebookStructureFixture(ctx, snapshot, decisions)` 仅生成系统临时目录中的全新库，没有任意目标 DB 路径、Apply 或修改源表的参数。目录 `0700`，DB 与 `source.private.json` 为 `0600`；后者保留两张完整源表的 schema、原行、未知列、大整数/NULL及批准元数据，属于私密材料。夹具是 notebook 结构证明，并非包含 users/权限/评论的整站恢复；源 owner ID 校验不代表夹具已具备完整用户系统。调用者负责处理自己新建的临时夹具。
- 当前 DDL 来自 Store 在空新库上的初始化，不复制 DDL、不对旧源调用 EnsureTable。全部集合与笔记插入同一事务，唯一约束等失败回滚；取消/失败不返回夹具并清理本次新建的私密目录。原 Markdown、HTML缓存、时间、ID和关联保持；只做结构转换，旧 `stellar_version=0` 保持 0，不提前标成 2。已有 version 2 的原 JSON 保持。
- `go test ./internal/stellarmigration -run TestNotebookStructure -count=1` 验证当前 Store 真实读取、draft不公开、严格重新快照、未知字段归档、64位大ID、标签保真、拒绝未批准/过期元数据/冲突、事务回滚和取消。证明夹具不解除原 `Candidate` 中的 `legacy_notebook_schema`／`note_missing_slug_status`；真实记录批准及真实转换后仍须重新生成完整内容候选并校验新源 digest。

2026-10-13 另以只读根快照生成默认未批准结构预览，位于迁移树 `.tmp/stellar-notebook-structure-review-20261013-1/`（目录 `0700`，五文件均 `0600`）。完整内容候选仍 32 条／171 配置映射／34 阻断；两张结构源表共两条记录、21 项已确定列映射，保留 5 个缺失批准字段：Notebook `author_id/updated_at`，Note `slug/status/author_id`。没有给真实记录分配值或生成真实内容夹具，未执行转换。SourceDigest 和 DB SHA 与 review-6 一致，本次数据库及 WAL/SHM/journal 指纹前后相同；仍不承诺目录零写。
