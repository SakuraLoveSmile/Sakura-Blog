# Stellar 2.0 迁移候选

此包提供只读候选、逐项决策审核、完整资源演练，以及显式离线批准后的 `ApplyReviewedRootMigration` 和恢复入口。`preview`、`approve` 不迁移源数据；`rehearse` 在新的私密目录验证，只有用户另行执行 `apply` 才修改绑定的运行时资源。独立 Notebook 结构证明仍只能创建新的私密临时夹具库。全程使用本仓库 Stellar 2.0.0 strict parser，不以 recovery、默认初始化或旧列推断作为运行时兼容。

## 接口与私密边界

- `ReadSnapshot(ctx, dbPath, files)`：SQLite `mode=ro`、`query_only=1`、单事务读取实际 `sqlite_master` 与动态列，另用同样只读的 `Store.DetectLegacyNotebookSchema` 共用结构契约并比对捕获列。旧表缺少新版列不会触发建表／补列。`files` 是调用者明确提供的受跟踪 Markdown 原文。
- `Preview(snapshot)`：先构建全部集合，再建立上游 Registry，逐条校验成员。返回主题 document JSON、独立 Blog JSON，以及每条 Post／Page／WikiProject／WikiDoc／Topic／Notebook／Note／文件的原文摘要、version 2、Front Matter JSON、Blog JSON、Markdown 和映射／冲突。已 version 2 的 theme／内容 JSON 与 Markdown 逐字保持；无任何存储副作用。
- `PreviewWithCapabilities(snapshot, ConsumerCapabilities{ConsumerPoster: true})`：仅调用者显式声明已经实测闭合的消费者才降为提示。catalog 保留 `poster`、`collection_order`、`authors`、`author_widget`、`wiki_external_url`，并按字段独立登记以下全局能力；未知 capability 报错，独立包默认没有闭合能力。Blog CLI 宿主明确提供当前已实现的 20 项能力，保留所有消费者诊断为 info；显式 `--capabilities <private-json>` 完整替换宿主声明，`{}` / `null` 仍不启用任何能力。诊断全部保留，以 `kind=consumer`、`consumer=<能力>`、`severity=info`、`blocking=false` 标明调用者的声明，并不替代真实体验证明。政策／结构／strict schema 冲突不受该参数影响。
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

公开署名与编辑权限独立：`blog.content.authors` 只含已有且 `name` 有效的公开 catalog ID 字符串数组，按旧关联顺序去重；完整 `post_authors/post_authors_legacy/users` 仍在私密源快照。TEXT 引用只精确匹配已有 ID；INTEGER 权限引用只在唯一 `users.id → username` 且 username 已是有效 catalog ID 时确定映射，不按显示名匹配、不创建作者或用户、不写权限。任何无法确定的关联（包括未映射 legacy 引用）使整段公开数组不生成，并保留不可用 capability 解除的 `public_author_unmapped` policy 阻断。单作者旧字段也只接受现有公开 ID。新候选通过正式 Store 的公开作者校验，而不是仅比较 JSON 形状。

### 全局消费者及已证明等效的字段

原先 `Blog.content` 仅存储却未报未消费的 13 项，现在分别要求能力证据：`avatar_background/rightbar_intro/welcome_linklist/nav_tabs/ai_label/category_icons/tag_icons/read_next/expire_days/banner_ratio/edit_this_page/notebook_license/notebook_share`。`Blog.pages` 与剩余 typed `Blog.seo` 分别要求 `pages/seo`；false、null 和显式空覆盖同样保留并报消费者门槛，不当作无源值。未声明时全部阻断，声明后诊断保留为 info，候选值与映射不改。

Notebook 页脚仍为独立 `blog.content.notebook.license/share`，不捏造 strict theme 不允许的 `profiles.note.footer`。Pages 的已冻结消费边界为 friends `title/hero/title-description-background/show_posts/refresh_interval/posts_per_link`；其它分组或 Hero animation/start/preview/actions 保留并报 `unmapped_setting`，不能用 pages 能力解除。

Pages 的 `show_posts` 是 nullable 指针：显式 null 原文保留并继承启用，false 关闭，true 启用；不将 null 当错误、缺字段或 false。

旧 `tag_plugins.okr` 只有完整对象精确等于固定 `9d11978` 的 1px 边框、五状态颜色和 zh-CN 文案时，标记“原值留 source.snapshot、继承官方等效”；不生成被上游删除的 `tags.okr` 参数袋。任何自定义、缺键或额外字段继续 `tag_config_removed` 阻断，不能把这一条默认对象的豁免扩散到其它 removed 字段。

旧 SEO 六项按冻结契约映射官方：`preconnect→preconnect`、`canonical.originalHost→canonical.host`、`canonical.officialHosts→canonical.allowed_hosts`、`open_graph.enable→open_graph.enabled`、`open_graph.twitter_id→open_graph.twitter_id`、`structured_data.links→structured_data.same_as`。旧 nil slice 转显式空数组，空 host 转 null，旧 nil enable 按旧 `Enabled()` 为 true；false/空数组保持。坏类型或未知字段不丢弃，继续 schema 阻断；同一 canonical 目标已有不同来源时不覆盖并报 policy 冲突。

`default_description/default_keywords/robots_extra/no_index/google_verification/bing_verification/baidu_verification` 保留 typed `Blog.seo`；前两项不覆盖原 `Blog.site`。来源保真和 runtime 优先级分别核验，消费者由宿主负责。官方已映射部分不重复塞入 raw Blog SEO 袋；全部原值仍保留于私密 source snapshot。

## 验证

`go test -race ./internal/stellarmigration` 覆盖独立旧／新 SQLite、只读／不存在源、version 2 原文、大整数／false／null、所有条目完整性、源评论变化失效、未知字段保留、集合归属、Wiki 两候选、旧笔记 schema、Markdown 代码保护及 private artifact 权限。预览／批准／验证不修改源数据；离线 Apply 另有完整资源前像、摘要约束、运行时验证与 journal 恢复门禁，不得用只读测试代替 Apply 验证。

`TestReadOnlySourcePreviewEvidence` 默认跳过。只有显式设置 `STELLAR_MIGRATION_SOURCE` 时才读指定源；`STELLAR_MIGRATION_CONTENT_ROOT` 可提供本任务两份 tracked 验收文章；`STELLAR_MIGRATION_OUTPUT` 选择新私密目录。它比较数据库与 `-wal/-shm/-journal` 的前后 SHA256，只生成 `candidate.private.json`、`report.public.json`、`source-check.json`，不执行真实转换。

## Notebook/Note 结构证明

- `PreviewNotebookStructure(snapshot, decisions)` 是纯私密预览，输出全部目标列、映射与阻断。`decisions.SourceDigest` 绑定完整源快照；`Records` 以 `notebooks/<id>`／`notes/<id>` 为键。缺失的 `author_id`、Note `slug/status`、日期必须由对应记录的 `Approved=true` 和非 nil 值明确批准；`OwnerID=0` 也必须显式提供，不根据当前会话、时间或署名推断。已有宿主字段不能借结构转换覆写。正 owner 必须存在于源 users。
- notebook ID 转为原数值的 TEXT，Note 的 INTEGER ID 保持原整数，`notes.notebook_id` 转为父本原 ID 的 TEXT；**不是把 Note ID 改为 TEXT**。`nb_order→sort`、`content→content_md`；旧逗号标签按既有规则拆分，保留层级、顺序、重复、大小写及空片段，完整旧字符串另存私密归档。已有 `tags_json` 不重写；新旧正文、排序或标签两来源冲突继续阻断。未提供的纯可选展示字段用当前空默认，非空列中的旧 NULL 不隐式改值。
- 旧 Notebook 缺 `updated_at` 时，调用者可逐记录明确批准使用源 `created_at`，不能自动使用当前时间。现有日期逐值保留。slug 按当前宿主路径规则校验，状态只接受 draft/published，重复 slug、缺父本、无效 owner/整数/日期等阻断。
- `BuildNotebookStructureFixture(ctx, snapshot, decisions)` 仅生成系统临时目录中的全新库，没有任意目标 DB 路径、Apply 或修改源表的参数。目录 `0700`，DB 与 `source.private.json` 为 `0600`；后者保留两张完整源表的 schema、原行、未知列、大整数/NULL及批准元数据，属于私密材料。夹具是 notebook 结构证明，并非包含 users/权限/评论的整站恢复；源 owner ID 校验不代表夹具已具备完整用户系统。调用者负责处理自己新建的临时夹具。
- 当前 DDL 来自 Store 在空新库上的初始化，不复制 DDL、不对旧源调用 EnsureTable。全部集合与笔记插入同一事务，唯一约束等失败回滚；取消/失败不返回夹具并清理本次新建的私密目录。原 Markdown、HTML缓存、时间、ID和关联保持；只做结构转换，旧 `stellar_version=0` 保持 0，不提前标成 2。已有 version 2 的原 JSON 保持。
- `go test ./internal/stellarmigration -run TestNotebookStructure -count=1` 验证当前 Store 真实读取、draft不公开、严格重新快照、未知字段归档、64位大ID、标签保真、拒绝未批准/过期元数据/冲突、事务回滚和取消。独立证明夹具不解除原 `Candidate` 中的 `legacy_notebook_schema`／`note_missing_slug_status`。完整 root 迁移必须在选值预览中具名选择 `use_notebook_decisions`，并以 `RootReviewDigest` 同时绑定全部 Notebook 元数据、候选与来源；来源变化必须重新预览。

2026-10-13 另以只读根快照生成默认未批准结构预览，位于迁移树 `.tmp/stellar-notebook-structure-review-20261013-1/`（目录 `0700`，五文件均 `0600`）。完整内容候选仍 32 条／171 配置映射／34 阻断；两张结构源表共两条记录、21 项已确定列映射，保留 5 个缺失批准字段：Notebook `author_id/updated_at`，Note `slug/status/author_id`。没有给真实记录分配值或生成真实内容夹具，未执行转换。SourceDigest 和 DB SHA 与 review-6 一致，本次数据库及 WAL/SHM/journal 指纹前后相同；仍不承诺目录零写。


## Blog CLI：从 1.x 审核到离线执行

始终使用同一新版本 binary、相同 `-root` / `-config` / `BLOG_*` 资源配置。Docker 中也必须挂载真实 DB、配置、uploads、content 及审核输出，且每一步看到的资源路径应一致。审核目录不能位于捕获的 runtime（含 root、外置 DB 父目录、uploads、app 配置父目录）之内；输出目录必须尚不存在，父目录须已存在。文件为 `0600`、目录为 `0700`，不覆盖原文件；手工保存 decisions 的直接父目录也须是 `0700`（示例中的 `/private-review` 需预先按此权限建立）。即使名称含 `public`，CLI 也将报告保存在私密目录，勿公开整份审核目录。

1. 用新 binary **预览旧来源**，不要先启动新版服务初始化旧库：

   ```sh
   blog -root /runtime stellar migrate preview --out /private-review/initial
   ```

   首次预览先使用 `mode=ro` 读取关闭的 WAL 旧库，稳定 SQLite 必要侧车，再执行双读完整资源捕获；不修改业务数据，但不承诺目录零写，不使用 immutable、不删侧车、不弱化摘要。默认发现 `content/posts/` 的 Markdown 和有效内容 manifest 的所有源文件；其它 `content/` 内已审查来源使用可重复的 `--tracked content/path.md`。预览不是文章导入或发布命令，不会替换已有内容。初次输出含完整 `root-capture.private.json`、`candidate.private.json`、脱敏 `report.public.json`、未批准的 `approval-template.private.json` 与 `decisions-guide.private.json`；stdout 给出摘要和下一步。

2. 复制并填写私密 `approval-template.private.json`，保留 `approval.approved=false`。`decisions-guide.private.json` 为每条阻断列出 `source/code/path`、支持的 action 与 alternative 名称；未知冲突的 `allowed_actions=[]`，需要修正来源或实现后重新初次预览，不能删诊断或发明 action。模板内已列出支持的 resolution，`action` 仍为空，不默选策略。`choose_alternative` 必填精确 `alternative_name`，`replace_url` 必填 `replacement_url`；不要在其它 action 保留这些字段。

   Notebook 以 `notebooks/<id>`、`notes/<id>` 具名列出待补记录。依据 guide 的 `notebook_required_fields`，仅添加实际缺失的 `owner_id`、`slug`、`status`、`created_at`、`updated_at`，逐记录设置 `approved=true`。不会自动选作者、发布状态、路由或日期。`owner_id=0` 也必须明确填写，正 ID 必须存在于源 users；日期可在审核后明确采用源日期，不能自动采用当前时间。不可覆盖已有宿主字段。

3. 生成**选值预览**，即便没有冲突也需要此步：

   ```sh
   blog -root /runtime stellar migrate preview --review /private-review/initial --approval /private-review/choices.private.json --out /private-review/selected
   ```

   这一步重新校验完整来源与具名决策，必须零阻断才成功。输出 `selected-candidate.private.json`、`reviewed-selections.private.json`、`body-review.public.json` 和完整审核摘要；新的 `approval-template.private.json` 仍为 `approved=false`，正文摘要未获批准。检查选值结果与正文变化，不要直接编辑 selected 目录内任何产物。

4. **显式批准已审查结果**：

   ```sh
   blog -root /runtime stellar migrate approve --review /private-review/selected --out /private-review/rehearsal-approval
   ```

   如 `body-review.public.json` 列出正文变化，必须先查看完整候选正文，再加 `--accept-body-changes`；该旗标仅批准重算出的精确正文摘要。approve 重捕获完整 Root/Source，重建 Candidate/Review 和每份选值审核产物，拒绝过期来源、被改过的候选／报告／正文摘要或未完成决策。只生成新 `approval.private.json`，不执行 Apply。不带两枚离线声明时 `offline_attestation.approved=false`，只能用于演练。

5. **演练正式 Bootstrap、搜索与 SSR**：

   ```sh
   blog -root /runtime stellar migrate rehearse --review /private-review/selected --approval /private-review/rehearsal-approval/approval.private.json
   ```

   演练复制完整资源至新私密目录，正式 binary 子进程校验当前内核。失败时 stdout 保留演练位置，查看该 root 的 `.stellar-migration-validation.private.json`。不要把通过演练当作线上已迁移。

6. 停止全部新旧服务器、workers 与其它写入者，检查旧实例；若源资源因此发生变化，需要重新执行预览与选值审核。明确准备执行后，生成**离线批准**：

   ```sh
   blog -root /runtime stellar migrate approve --review /private-review/selected --out /private-review/offline-approval --server-stopped --legacy-instances-checked
   ```

   有正文变化仍须加 `--accept-body-changes`。只有两枚离线旗标同时存在，才设置离线批准；单独一枚仍只授予演练批准。旗标是操作者声明，不是自动停机或远程进程探测。Apply 还会独立检查本机配置端口及共享 lifetime guard。新的离线批准不替代演练；确认审核与演练后，用户另行执行：

   ```sh
   blog -root /runtime stellar migrate apply --review /private-review/selected --approval /private-review/offline-approval/approval.private.json
   ```

   Apply 验证完整源资源和摘要，保存完整前像，校验 staged runtime 后离线替换。异常留下的 migration journal 会阻止普通启动／CLI，必须在保持离线与资源配置一致的条件下显式执行 `blog -root /runtime stellar migrate recover`；不要手动删除 journal、重建源库或覆盖备份。

stderr 只输出静态脱敏阶段错误码和下一步：`ARTIFACT_INVALID`（权限／JSON／缺文件）、`REVIEW_CHANGED`（审核文件或决策不匹配）、`SOURCE_CHANGED`（重新预览当前资源）、`SELECTIONS_BLOCKED`（补齐具名决策）、`BODY_APPROVAL_REQUIRED`（审查正文后加确认旗标）、`OUTPUT_INVALID`（改用 runtime 外的新目录）、`RUNTIME_VALIDATION_FAILED`（查看私密演练诊断）。底层 YAML、SQL、密钥及正文原值不会回显到终端。
