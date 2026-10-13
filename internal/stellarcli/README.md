# Stellar Go 工具

命令入口由 `cmd/blog` 经顶层 `modules/stellar` 装配，核心不依赖 HTTP 模块。生产只使用 Go 二进制；Node 仅用于开发期的官方规则 oracle。

```sh
blog -root /absolute/blog stellar doctor --format text
blog -root /absolute/blog stellar doctor --format json
blog -root /absolute/blog stellar new note --notebook journal --title 'Hello' --tags 'Go,SQLite' --dry-run
blog -root /absolute/blog stellar new note --notebook journal --title 'Hello' --author 3
blog -root /absolute/blog stellar images --page notebooks/journal/index.html --dry-run
blog -root /absolute/blog stellar images --refresh https://example.org/image.png
```

- Doctor 报告真实 Go 版本、固定 Stellar 源身份和二进制嵌入资源 SHA-256。SQLite 是 Go 宿主的配置与内容真值，故检查对象为 settings、集合及内容记录，缺表/列给出诊断，不假装检查 Node/Hexo 或读取另一套 Hexo 配置。所有内容（含草稿）均须 version 2；回收站记录除外。用户已选择严格拒绝旧字段，因此官方 Doctor 的 recovery 在本产品不启用；不适用 profile 参数和 Widget placement 仍按官方 warning 规则报告。失败退出 1，无效参数退出 2。
- Doctor、Note dry-run 和 images 使用 `store.OpenExisting(..., true)`，不运行 `app.Bootstrap`、seed、建表、迁移、chmod 或 journal 模式切换。不存在的库不会被创建。
- Note 使用当前严格 Notebook。已知 Notebook 作者可继承；未知作者须显式 `--author`，核验现有 active 作者的 `content:write` 权限，不猜账号、不创建账号。计划输出来源、SQLite 目标、完整原始 Front Matter 和正文导出。正式操作在一个事务内再次检查严格父配置、作者归属/权限并创建 draft；唯一键拒绝覆盖，失败整笔回滚。数据库是唯一持久化源，不同步生成另一份 Markdown。
- 图片回调必须提供同一公开快照的真实 SSR 详情及聚合页面，不能使用静态仓库文件或回环 HTTP。缓存唯一位置为 `<root>/content/_data/caches/images_metadata.json`。dry-run 不下载或写缓存。正式图片下载沿 `safefetch`；本地资源仅开放 uploads、嵌入资产及同一注册表的生成图标/fallback，escaped route 只解码一次。失败图片计入 report，并沿官方规则保存重试时间；处理器错误退出 1，单张图片失败保留官方正常报告退出行为。

`generate/oracle.cjs` 从锁定 commit 的实际 `new-note.js`、`doctor.js` 和 adapter capability 声明生成 fixture。Go 差分验证标题安全/trim、tag 去重及 Widget 诊断；SQLite 适配另用隔离库与真实子进程测试退出码、只读和事务回滚。
