# BLG-FRIEND02：友链页标题与本站资料

- 状态：待体验。日期：2026-11-06（Asia/Taipei）。已按用户体验反馈补齐标题居中与可复制资料块，适用验证完成，正式版 `v2.0.9` 已发布并更新线上；发布授权、线上核验及收尾证据统一记在 [BLG-EDIT02](page-editor-wysiwyg.md) 的 v2.0.9 发布与上线一节。
- 目标：友链页“友链列表”“失联友链”“如何交换友链”“本站信息”统一为居中的橙色双斜杠、衬线文字；交换说明下按 xaoxuu 参照提供本站资料 YAML 代码块（title、url、avatar、description、feed）；答复现有友链展示方式，不切换卡片布局。
- 起点：根 dev / `2ad23a6225708675a7917cd718d5f92e399cee3d`，根目录已有用户文档与证据改动，全部保留。分支 `codex/fix/20261106-blg-friend02-friends-page`；worktree `/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261106-fix-blg-friend02-friends-page`。集成目标 dev；本次未授权提交、推送或生产发布。
- 标题装饰与字体的唯一参照：`docs/baselines/friends-page/reference.png`；本站信息的布局参照为用户后补的 `reference-site-info.png`，来源 `https://xaoxuu.com/friends/`，采用居中分区标题、说明及整段 YAML 代码块，标题装饰仍沿已确认的橙色斜杠。实时参考页只读访问未返回截图中的资料块，因此以用户截图作为资料布局依据。`before-heading.png`、`before-exchange.png` 仅记录原有问题。颜色复用 Stellar `--accent` / `--text`，浅色使用对应可读文字；字号复用 `--fsh2`，移动端无页面横向溢出。复用 Stellar `.highlight > .code > pre` 及现有复制按钮，不另建复制运行时。
- 本站资料由当前公开站点设置提供，只读展示，不改保存的正文、站点资料或友链数据。仅在友链页交换说明所属分区补资料；公开站点现有名称为“上杉九月的博客”，网址为 `https://blog.上杉九月.cn/`，头像为 `/uploads/2026/08/afed6b10291dafbc.png`，简介沿当前站点配置。feed 使用本站既有 `/atom.xml` 路由，并保留站点子目录；公开 Atom 地址已只读验证为 HTTP 200、有效 Atom XML。
- T1：主 agent 独占页面宿主渲染、交换资料、标题 CSS、实现说明及记录；worker 独占回归文件 `modules/stellar/routes/stellar_friends_content_test.go`。只读 explorer 并行核查已实现展示模式，不写文件、不启服务、不访问生产、不派生；其结论由主 agent 对实际 2.0 入口复核。
- T2：在隔离数据目录、端口 `127.0.0.1:18086` 启动本次构建，主 agent 用真实浏览器检查 1280/390 宽度、浅色/深色/自动及四种 appearance，保留截图、当前资源 hash 与检查结果。运行 `make test` 和实际构建；不运行会合并 dev 的 `make merge-check`。
- 本次补正验收：[x] 四个标题同样式且居中；[x] 五项本站资料为可整段复制的有效 YAML、头像及订阅地址为公开绝对地址；[x] 其他页面不受影响且存储原文不变；[x] 失联卡片仍无跳转；[x] 手机无页面横向溢出；[x] 适用检查通过；[ ] 用户体验确认。站点没有专用网站截图设置，不虚构 screenshot 地址或引入外部截图服务。
- 收尾：保留本任务 worktree 与本地预览供用户体验。原验证进程已结束，运行资源现由同工作树的 [BLG-EDIT02](page-editor-wysiwyg.md) 接续，当前进程及实际资源版本以该记录为准。运行根目录仍为工作树 `.tmp/blg-friend02-runtime`，数据库、账号、页面及三条友链均为隔离夹具；本站资料来自只读公开站点核实。仅共享工作树自己的构建产物与根目录 resource 输入。集成且验证通过后，先结束本任务服务，再按 AGENTS.md 清理本任务 worktree 与本地分支。

## 实测与证据

- 实现：实际服务及仍维护的 Blog 服务将失联标题改为 `h2`；友链页四个分区标题共用居中的双斜杠、衬线字及主题语义配色。专属 CSS 不改变生成的主 CSS，因此无需主题重编。资料直接消费当前站点快照，中文域名与相对头像地址复用既有 HTTP 资源解析器；改用既有 `github.com/goccy/go-yaml` 编码五项资料，不增加依赖。
- 预览与正式页面统一；预览的公开资料来源保留，避免后台 origin 覆盖站点网址。相对头像按站点根目录解析，含子目录站点；恶意头像协议不渲染为执行 URL。新增 3 个实际 embed 回归场景验证设置更新、分区位置、作用域、原文保留、预览与转义。
- 代码版本为起点 HEAD 加本任务未提交修改；当前补正版的 `checks-centered-verification.json` 保存代码 SHA256 和检查结果。当前 `make go-build` 及 `make test` 完整通过（Console 456、Stellar assets 229，以及 Go vet、Staticcheck、全部 Go 测试），定向 Go 测试也通过。真实 Console/UC 产物已在上一版本地 `make build` 生成，此后 SPA 源码未修改。未运行会执行合并的 `make merge-check`。
- 上一版首轮完整检查曾遇到未修改的 Mermaid loader 用例固定 15ms 等待不足，单独及完整重跑通过；补正版当前 `make test` 首次完整运行通过。没有改测试预期、延长等待、跳过或降低检查。原来的资料测试按这次已确认的 YAML 展示替换 DOM 断言，并增加冒号、引号、多行值和 feed 子目录的还原验证；正文保留、公开预览源和 XSS 检查继续执行。
- 当前浏览器：4 外观 ×（浅色、深色、自动跟随系统浅色、自动跟随系统深色）× 1280/390 宽度，共 32 场景通过；四个标题的中心偏移均为 0px，字体、字号、颜色、斜杠与倾斜矩阵一致，浅色黑字、深色白字，无页面横向溢出。另实测普通页面不加载专属样式或资料，再通过侧栏局部导航回友链页正常加载，复制按钮仅挂载一次，共 33 条记录，见 `browser-centered-verification.json`。
- 复制按钮已复用现有 runtime，点击后浏览器提示“复制成功”；代码块原文及解析后的五项值验证通过。浏览器工具的虚拟剪贴板读回为空、粘贴报告无虚拟剪贴板数据，不能据此验证操作系统剪贴板的实际内容；没有将该读回当作通过证据，也未修改复制组件。
- 当前服务提供的 CSS、实际友链脚本及代码复制脚本 SHA256 均与源码相同，见 `runtime-assets-centered.json`；浏览器无 error 日志。临时视口与系统配色模拟已还原，额外参考/检查标签页已关闭，保留友链预览页。
- 当前截图与度量在 `docs/baselines/friends-page/`：`after-centered-desktop-dark.jpg`、`after-centered-mobile-dark.jpg`、`after-centered-desktop-light.jpg`、`after-centered-preview.jpg`、`browser-centered-verification.json`。不含 centered 的截图/检查文件保留为上一版历史证据。截图使用隔离示例友链，不能当成生产页面已更新的证据。
- 展示方式核查：2.0 现有头像卡片、带 RSS 动态卡片、网站封面卡片和按数据分组的卡片可复用；纯文字列表需要另做样式。第三点只提供选项，当前布局未切换。
- 下一步：用户体验线上 <https://blog.xn--fhqths51enha.cn/friends>；本地预览已按后续发布授权停止，夹具和运行记录已保全，详见 BLG-EDIT02 收尾记录。仅用户确认后改为“已验收”。
