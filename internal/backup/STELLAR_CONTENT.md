# Stellar 2 内容持久化与便携备份

生产 `WritePostsZip` 名称沿已有下载入口，输出完整 Stellar 内容 ZIP。`ImportContext` 接收该 ZIP、单篇 `.md` 或仅含平级文章 `.md` 的 ZIP；生产不调用旧轻量 `FileMeta` 导入器。SQLite 全量快照继续按真实表/列保存和恢复，不重建 JSON。

完整包包含 `blog-content-v2.json`，schema `version: 1`。`records` 每项包含白名单 `table`、对应 `source` 和完整 SQLite 列 `values`；`stellar_version` 必须为 2。`stellar_config`、`blog_extensions` 均为 JSON 字符串，连同 `content_md`、草稿头及其他列值保留原字节。`authors` 只含所引用身份的真实 ID 和用户名，不含认证数据。

| 内容 | 源文件 |
| --- | --- |
| Post | `posts/<slug>.md` |
| Page | `pages/<slug>.md` |
| Wiki 文档 | `wiki/<project>/<slug>.md` |
| Note | `notebooks/<notebook>/<slug>.md` |
| Wiki 集合 | `_data/wiki/<id>.json` |
| Topic 集合 | `_data/topic/<slug>.json` |
| Notebook 集合 | `_data/notebooks/<id>.json` |

Markdown 使用 Hexo 原生 JSON front matter `;;;`，主题部分保持上游 snake_case，独立扩展使用 `blog`。Go 发布状态在 Markdown 中镜像为 `blog.status`，避免单篇重新导入将草稿误发布；manifest 仍保存原扩展 JSON 与真实存储状态。目录 seed 使用相同 manifest/source 协议。手工修改源文件时，须同步生成相符的 manifest，不能让两份真值矛盾。

结构关联只允许 `categories`、`post_categories`、`post_authors`、`wiki_project_tags`。导入拒绝路径逃逸、符号链接、重复源/ID、未知表或列、缺父集合/成员、源与 manifest 不一致、身份错配及未迁移记录。不会新建账号、猜所有者、重映射 ID 或移动笔记到另一笔记本。派生 HTML 从原 Markdown 通过 v2 renderer 重新生成。

在线导入复用现有恢复任务、维护窗口、前像、原子数据库应用、重建和补偿。预检只改私有副本；维护排空后再从最新 DB 构建候选，保留已进入请求的最后一次写入。普通文章导入仅覆盖请求指定文章，保留目标库的既有集合定义和其他内容。

启动仅调用 `seed.ImportStellar`：显式 seed 对未迁移数据报错；自动发现只补缺失记录，绝不覆盖 Console 已编辑内容。普通启动遇到迁移门禁停止自动发现，仍允许 Console 打开迁移预览；不调用旧 seed 回退。旧 `seed.Import/ImportMissing` 仅供原历史单测，不构成生产严格路径的验证。

源文件及内容 ZIP 有体积上限，不支持非 UTF-8 文本；此类原始数据仍可使用 SQLite 原生快照原字节保存。所有本次往返、失败回滚、权限和并发验证使用隔离数据库，真实数据迁移需要先审核迁移预览。
