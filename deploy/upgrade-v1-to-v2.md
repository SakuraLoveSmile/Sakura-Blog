# Docker / Compose：从 1.x 升级到 2.0 并导入 Markdown

这是服务器操作步骤。本文命令中的镜像、Compose 服务名和目录必须对应实际部署；本地测试通过不表示服务器已经升级。不要先启动新版本去尝试读取旧库，也不要用 `-seed` 代替迁移。

## 1. 找到旧站与数据

在服务器执行以下只读命令，找到 Blog 的容器名：

```bash
docker ps -a --format '{{.Names}}\t{{.Image}}\t{{.Status}}'
docker inspect blog --format '{{range .Mounts}}{{println .Source "->" .Destination}}{{end}}'
```

第二条中的 `blog` 换成真实容器名。记录 config、data、content、uploads 的实际宿主目录与容器内目录。已运行过的 1.x 站点，配置真值是旧 SQLite 的 `settings` 表；`app.yml` 与 `BLOG_*` 环境变量决定数据库、上传目录和监听端口。迁移会保留配置文件、附件和其它资源，但不会将任意独立 `theme.yml/widgets.yml` 自动当作数据库配置导入。只有 YAML、没有旧数据库时，先确认来源，不要新建空库代替旧站。

确认原 Compose 文件与服务名，并检查有无额外容器/宿主进程共用同一份数据库。不要把含密钥的 `docker inspect` 全文或 `docker compose config` 全文发到聊天、提交到 Git。

## 2. 备份并固定镜像

保留旧镜像的准确版本/摘要及原 Compose 文件。先下载已经验证、包含本文新迁移命令的 2.0 镜像；不要以 `latest` 是否变化作为版本证据。如果采用宿主挂载二进制的旧部署方式，先按该方式准备新二进制，下面的官方完整镜像步骤不能直接套用。

停止旧服务及所有共用数据的实例，再完整备份已确认的 config、data、content 和外置 uploads。停止之后复制 SQLite 所在目录，包含仍存在的 WAL/journal，不单独复制一个活跃的 `blog.db`。备份放在运行目录之外，保留原权限，验证可以读取；不要使用 `docker compose down -v`。

先盘点 `data/backups` 等历史备份数量与容量。迁移会捕获运行目录资源，历史备份很多时可能超过小内存服务器的容量。完整备份验证后，可将历史备份完整归档到运行目录外，保存逐文件摘要和恢复位置，再生成新的来源预览；不要删除历史备份，也不要在来源预览后移动文件。可先在副本上使用 `--memory` 和 `GOMEMLIMIT` 验证内存上限，不能以退出 137 的失败预览继续批准。

以下示例从原 Compose 所在目录执行。示例服务名是 `blog`，官方镜像运行根是 `/app`；按第一步的实际结果修改。原文件中将服务的 `image` 更新为已验证的固定新版本/摘要，并保留原数据挂载和环境配置。此时不要执行 `up`。

```bash
docker compose stop blog
# 在这里完成并验证停服后的完整备份，再修改 Compose 的 image。
docker compose pull blog

umask 077
UPGRADE_DIR="$PWD/upgrade-v2-$(date +%Y%m%d-%H%M%S)"
mkdir -m 700 "$UPGRADE_DIR"
RUNTIME_ROOT=/app

migrate() {
  docker compose run --rm --no-deps \
    --entrypoint /usr/local/bin/blog \
    -v "$UPGRADE_DIR:/migration" \
    -e BLOG_SEED=false -e BLOG_AUTO_BACKUP=false \
    blog -root "$RUNTIME_ROOT" stellar migrate "$@"
}
```

`run --no-deps` 只创建一次性迁移容器，沿用 Blog 的数据卷与环境，不启动站点或依赖服务。额外 `-v` 把审查目录放在 `/app` 之外，避免报告改变捕获来源。若用多个 Compose 文件，所有 `docker compose` 命令必须带同一组 `-f` 参数。若数据库/上传路径在外部，原卷挂载必须完整带入。

## 3. 预览并处理具名选择

```bash
migrate preview --out /migration/source
```

命令默认声明当前宿主已实现的 2.0 消费能力，不需要手工制作能力文件；显式传入 `--capabilities` 仍完全覆盖默认声明。它只读取数据库并生成私密文件；SQLite 只读连接仍可能建立 WAL/SHM 侧车，不承诺整个目录零写。不会自动修改文章、配置版本或发布状态。

在服务器本地查看 `$UPGRADE_DIR/source/report.public.json` 和 `decisions-guide.private.json`。`blocking` 大于 0 表示尚有未决事项，退出 0 只表示预览已生成。按指南填写 `approval-template.private.json` 中的具名 resolutions，以及旧 Notebook/Note 所缺的归属、slug、状态等信息。没有可推断的作者时必须指定真实账户，不能按显示名或时间猜测。

官方镜像默认以 root 运行。在普通 rootful Docker 中，审查子目录和文件归 root 所有，权限分别为 `0700`、`0600`。以 `ubuntu` 等普通账户操作时，在服务器本地使用 `sudo` 查看、`sudoedit` 编辑，保留私密权限：

```bash
sudo cat "$UPGRADE_DIR/source/report.public.json"
sudo cat "$UPGRADE_DIR/source/decisions-guide.private.json"
sudoedit "$UPGRADE_DIR/source/approval-template.private.json"
```

后续候选正文和报告也按相同方式查看。不要为方便访问而把审查目录或文件改成公开可读。

- 多来源取值、旧专栏、失效 URL、未知配置等不会自动选值。
- 旧示例专栏不存在于项目库时，可逐项选择 `remove_missing_shelf`，或用 `choose_existing_project` 和 `replacement_id` 明确列出库中真实专栏。指南会列出现有项目 ID；删除全部示例不会自动把其它真实专栏加入列表，也不会删除项目正文。
- `legacy_official_policy` 只允许已列出的具体行为采用新版策略，原值归档。旧页脚正文与备案号都非空时继续阻断，因为拼接后渲染可能吞掉备案链接；应审查来源并重新预览。
- 不要删除诊断、修改 `candidate.private.json` 或随意把阻断标记改成 false。
- 没有冲突时也继续执行选值预览，生成完整审查摘要。
- 私密文件含配置、密钥和全文，只留在服务器权限受限目录中；不要发往外部服务。

```bash
migrate preview \
  --review /migration/source \
  --approval /migration/source/approval-template.private.json \
  --out /migration/selected
```

检查选值后的报告、原始/转换后正文，以及 `body-review.public.json`。来源在预览后发生变化，命令会拒绝继续；先确认变化并重新预览、审查，不能直接替换摘要。

## 4. 批准候选并完整演练

无正文变化时：

```bash
migrate approve --review /migration/selected --out /migration/approved-rehearsal
```

有正文变化时，逐条检查选值候选中的正文后，再显式加 `--accept-body-changes`。该标志绑定当前全部转换后正文摘要，不会绕过未决配置或结构冲突。

```bash
migrate rehearse \
  --review /migration/selected \
  --approval /migration/approved-rehearsal/approval.private.json
```

必须退出 0：它在隔离副本中迁移，启动相同二进制的生产 Bootstrap，重建搜索并检查公开 SSR；不改真实源。一次性容器删除后，临时演练副本也会消失；保留命令的脱敏结果与退出码即可。

## 5. 停服迁移与启动 2.0

确认旧服务器与 workers 全部停止、所有共用数据的旧实例均已检查，演练通过且真实来源没有变化，再生成正式离线批准。若有正文变化，同样加 `--accept-body-changes`：

```bash
migrate approve --review /migration/selected \
  --out /migration/approved-offline \
  --server-stopped --legacy-instances-checked

migrate apply \
  --review /migration/selected \
  --approval /migration/approved-offline/approval.private.json
```

只有 Apply 成功且没有待恢复错误，才启动新镜像：

```bash
docker compose up -d --no-deps blog
docker compose logs --tail 100 blog
```

检查 `/readyz`、首页、文章详情、后台登录、Stellar 2 设置和搜索。检查草稿没有公开、原发布状态/日期/作者/评论/图片与预览一致。`/readyz` 通过不能代替上述业务检查。

若 Apply 失败或出现 pending journal，保持停服，使用同一新镜像和相同数据挂载执行：

```bash
migrate recover
```

根据恢复结果继续处理，保留完整备份与迁移 journal。不要靠启动旧二进制来恢复已经转换的数据库，也不要删除 WAL、锁或 journal。版本回退涉及恢复停服前的完整数据备份，不能只切回镜像。

## 6. 导入服务器上的 Markdown 文章

升级旧数据库已经保留库内文章，不必重复导入。额外 `.md` 可下载到操作电脑后从后台导入；原 `content/posts` 中的文章也已经包含在整站迁移清单中。

- **先预览单篇：** 新建文章 → 工具栏“导入 Markdown 文件” → “从 1.x 转换” → 选择文件 → 审查预览 → “确认回填” → “保存草稿”。重新打开检查正文、日期、分类、Slug 和图片，确认后点击“发布”。回填不会保存或发布；编辑已有文章不会改其 URL。
- **批量：** 备份与导出 → Markdown 文章导入 → 勾选“从 1.x 格式转换” → 选择 `.md` 或平铺 `.zip`。ZIP 内直接放 `.md`，不要带目录、系统文件或图片。同 Slug 覆盖当前文章，确认框会提示。整批预检成功才写入；任一文件失败不会部分导入。
- 旧 `status: draft/published` 会保留；旧模式未指定状态的新文章保存为草稿。已有文章缺省状态沿用当前状态。严格 2.0 导入沿用原行为，未指定状态的新文章默认发布。
- 转换旧置顶、分类、海报、独立横幅及已知 shortcode；代码块保持原样。`topic` 必须唯一匹配现有专栏。未知/冲突字段继续报错，不静默丢弃。
- “从 1.x 转换”按旧版正文语义处理，例如未声明 `fancybox` 的旧 `image` 会补为 `false`。已经是 2.0 的文件应选择“Stellar 2 / 普通 Markdown”，以保留 2.0 默认行为。
- 密码保护、定时发布、编辑归属和草稿版本字段在本入口没有完整导入契约时会明确拒绝；请从后台逐项设置，不能把它们删掉后误公开原私密文章。
- 图片仍按文件中的 URL 访问，不会随 Markdown 上传。保留旧 uploads 挂载；相对路径应在发布前实看图片是否可访问。

完成后先保留备份、旧镜像和私密审查目录，直到用户确认新站体验。
