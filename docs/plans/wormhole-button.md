# 页脚虫洞按钮

任务编号：BLG-20261031-01。状态：实现中。

- 目标：在博客现有页脚添加十年之约虫洞入口，点击在新标签打开 `https://www.foreverblog.cn/go.html`；保留站点地图、自定义页脚与署名。
- 已确认：用户选择虫洞按钮；采用官方 32px 深浅色按钮，放在署名下方。沿用 Stellar 页脚结构、间距与语义 token，不添加脚本、挂件或配置字段。使用 `noopener` 隔离新窗口，同时保留来源站点信息供官方统计。
- 基线：`dev` / `84fe1baa13f89b5964ab06a1f40ccc5693066abe`；工作树 `/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261031-feat-blg-20261031-01-wormhole-button`；分支 `codex/feat/20261031-blg-20261031-01-wormhole-button`；本地合入目标 `dev`。根工作区已有文档与验收素材改动，均保留。
- 唯一布局参照：`docs/baselines/wormhole-button/reference-before.png`，2026-10-31 从线上首页页脚实拍；官方按钮来源：<https://www.foreverblog.cn/about.html#ab-8>。验收截图同目录。
- 验收：页脚仅新增一个可聚焦入口；官方按钮浅/深/自动配色清晰；card/glass/minimal/flat 四种外观在桌面及 390px 手机视口无横向溢出；新标签地址正确；现有页脚内容保留。
- 分工：主 agent 独占页脚 helper、辅助 CSS、记录和截图，负责浏览器与最终组合态核验；Luna explorer `footer_entry` 只读核查设置入口与现有行为，无文件写入，无独立服务/数据/端口。
- 验证资源：本任务工作树内隔离预览数据与配置；预览端口 `127.0.0.1:19831`。不使用根工作区数据库。适用检查：`make test`、实际构建、浏览器验收与加载资源指纹核对；`make merge-check` 会合入 `dev`，不作为只读检查。
- 发布边界：本次请求为本地添加与验证；推送、发布和生产部署分别记录，未经授权不执行。

## 执行与验证

- 页脚在自定义内容后追加按钮，不改已有 `theme.footer.content` 或数据库。样式通过现有客户端资源清单独立加载，不依赖文章扩展开关；保持四种外观最终样式层的位置。按钮使用浏览器原生懒加载，不依赖新增脚本。
- 初次 `make test` 暴露固定图片调用 nil 图标注册表的问题。保留原“无图标预览不输出图标 CSS”契约，仅为图片处理链加载局部内置失败图标；没有回写共享注册表。原失败和修复后定向检查均留有日志。
- 既有资源顺序测试的预期只新增 `site-footer.css`，没有删除/跳过测试或降低门槛。定向 `go test ./modules/stellar/web ./modules/stellar/routes ./modules/stellar/clientconfig` 和修复后的 `make test` 通过；原生懒加载 class 的最后调整已编译并通过以下浏览器核验，最终代码还会由提交 hook 和集成门禁再查。
- `make build` 生成真实 Console/User Center 产物；最终 Go 代码调整后再次 `make go-build`。服务以最终 embed 二进制启动，新增 CSS 的 HTTP SHA256 与源码完全相同，Console 入口引用的 6 个资源与构建目录 hash 一致。源码和二进制指纹见 [检查记录](../baselines/wormhole-button/checks-verification.json)。
- [浏览器记录](../baselines/wormhole-button/browser-verification.json)：真实本地服务，独立新建夹具库，card/glass/minimal/flat × 显式浅色/显式深色/自动随系统浅色/自动随系统深色 × 1280×800/390×844，共 32/32 通过；每组保留完整截图。外观通过隔离设置 API 保存；配色状态由开发工具临时切换 `html[data-theme]`，系统偏好由媒体模拟切换，不代表完整设置 UI 的用户旅程验收。已还原媒体和视口。
- 图片实际加载，始终只有一张对应配色图片显示，显示高 32px、点击区域不少于 44px、无横向溢出，署名与站点地图保留。键盘焦点为 2px 实线。回车在新标签打开官方虫洞，再实际跳转到 `https://jiangmiemie.com/`；原博客标签保留。官方“送出读者”计数未登录核验。
- 两轮独立只读 review 均无遗留实质问题。未修改共享主题源或生成主 CSS，因此 `make css` / `make css-drift-check` 不适用。

下一步：核对暂存范围后完成本地提交、`make merge-check` 与合并组合态检查，再收尾本任务工作树。远端 CI、推送、发布和线上部署均未执行。
