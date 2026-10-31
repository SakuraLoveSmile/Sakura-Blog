# AI 摘要与标题卡间距修复

任务编号：BLG-20261031-02。状态：实现中。

- 目标：修复标题横幅与 AI 摘要贴边的问题，上下间距桌面 24px、768px 及以下 16px；保留卡片尺寸、字体、颜色、摘要生成行为与其他页面布局。
- 唯一视觉参照：`docs/baselines/ai-summary-spacing/reference-before.png`，用户提供的线上 `/friends` 截图。复用 Stellar 既有结构与 `--gap-base` 间距 token，不改生成 CSS 或共享主题源。
- 基线：`dev` / `07bf8aaa01ccaf9793a1b05dc95be1c7da281738`。worktree：`/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261031-fix-blg-20261031-02-ai-summary-spacing`；分支：`codex/fix/20261031-blg-20261031-02-ai-summary-spacing`；合入目标：根工作区 `dev`。根工作区 295 份已有改动与其他 worktree 保留，工作树内 AGENTS 同步当前约定但不纳入功能提交。
- 验收：标题卡 → 摘要 → 首段/标题的实际上下间距一致；HeTi 开关均有效；普通页面与文章页均适用；card/glass/minimal/flat 的浅、深、自动配色在桌面和手机无横向溢出；摘要为空、自助面板展开及已有摘要均正常；无 AI 摘要页面不受影响。
- 分工：主 agent 独占样式、记录、构建与浏览器验收；Luna explorer `summary_spacing_scope` 只读核查选择器/资源加载和 margin 合并边界，不写文件或启动服务。本任务验证使用独立合成夹具与 `127.0.0.1:19832`，不连接根工作区或生产数据库。
- 基线证据：真实线上 `/friends` 启用 HeTi 时，标题横幅与摘要实际距离 0px，摘要到首个 h2 为 24px；`#ai-summary` 声明的 10px 上距被 `.heti > *:first-child { margin-block-start: 0 !important; }` 清零。此前浏览器读取失败已通过重置连接恢复，本次取得了实际几何值。
- 验证：真实构建、受影响浏览器矩阵与资源指纹、提交 hook、`make merge-check` 的 feature/dev 完整门禁；集成后再次核验运行中的当前资源与页面。不因样式小改新增与实现逐字对应的机械用例。
- 发布：本次为修复、验证和本地集成；线上发布另按仓库授权边界记录，不修改已发布 v2.0.3 资产。

## 实现与实测

- 仅修改 `modules/stellar/web/assets/css/reading.css`：正文继承局部摘要间距变量，取 `--gap-base` 的 1.5 倍（桌面 24px）或 1 倍（<=768px 为 16px）。仅在横幅后首块为摘要时，以同优先级覆盖 HeTi 的首元素清零；摘要下距与紧邻正文上距统一，利用正常 margin 合并保持单份间隔。不改主 CSS、共享主题源、模板、脚本、配置 schema 或数据。
- `make build` 与 `make test` 退出 0；Console 352 项、Stellar assets 240 项均通过，Go vet、Staticcheck、全 Go 测试及部署/来源/模块检查通过。没有删除、跳过用例或改动预期。仅辅助 CSS 变化，`make css` / `css-drift-check` 不适用。
- 当前二进制嵌入的 `reading.css` HTTP SHA256 为 `9a790d8102de51abe66c37b1be499ec72b36992ee62364f464f62798589283de`，与源码一致；Console 引用的 6 个资源逐份对应真实构建产物，readyz 200。见 `checks-verification.json`。
- 四种 appearance × 显式浅/深、自动随系统浅/深 × 1280×800 / 390×844 共 32/32 通过；真实几何值上下分别为 24/24px 或 16/16px，无横向溢出。外观通过隔离服务 API 保存，配色/媒体/视口经 CDP 临时模拟，不冒充完整设置界面旅程；未修改线上设置。见 `browser-matrix.json`。
- 另有 10 项检查通过：已有摘要与首段正文两种宽度、读者面板展开两种宽度、HeTi 关闭的 1280/390/768/769 宽度、无摘要的桌面/手机。无摘要时临时禁用整个 reading.css 对比，原首块几何不变。已有摘要使用当前 canonical `blog_extensions.content.ai_summary` 合成夹具；未请求真实 AI 服务。见 `additional-browser-checks.json`。
- 手机切换时原样式 `transition: all .3s` 会暂时返回中间间距，首轮测量因此未通过；改为等待实际几何稳定后断言，未修改产品动画或降低目标间距。最终容差仅为 CSS 亚像素舍入 0.02px。
- 最初空站夹具缺少 links/pages 表，普通模式启动中止；用现有开发模式初始化该合成库后，切回本次真实 embed 二进制重新核验。最终桌面/手机预览为 `preview-1280.jpg`、`preview-390.jpg`，记录 `embedded-preview-verification.json`。首批 raw CDP 截图有裁剪/缩放异常，已移入忽略目录保留诊断；重绑当前预览标签后，以正确的原生视口重拍完整截图并检查。临时浏览器媒体、设备与视口模拟已清除。
- 下一步：按原门禁完成签名提交及 `make merge-check`，再在 dev 组合态核对当前 CSS、页面效果和数据隔离，归档夹具并收尾本任务临时工作树。
