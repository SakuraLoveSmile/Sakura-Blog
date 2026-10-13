# Stellar 2.0.0 升级验收

状态：待体验（本地源码、构建与浏览器核验完成；尚未获得用户体验验收）。上游正式版本 2.0.0，commit `9d11978a24fe6610e3a7dbab741cf62634539d05`；起点为 Blog `bc0bc43` / Stellar 1.44.0。

唯一视觉参照：`upstream-reference.jpg`，来源 https://xaoxuu.com/wiki/stellar/#start ，2026-10-13 实际浏览器截图。旧版截图仅记录回归起点，不作为新设计目标。

冻结方向：采用 2.0 的 site-shell / site-region / brand / listing-nav 骨架和 card 外观；保留现有深色模式、中文字体、菜单、内容路由、配置编辑、搜索 API/静态索引、原生评论、短代码及 Go SSR。不上线、不迁移真实数据库、不改 Console 风格；Hexo doctor/schema/构建命令不套用于 Go 服务。

- T1：锁定上游干净源和 appearance/tags/features 构建配置，main.css 可重复生成，版本显示 2.0.0。
- T2：首页、文章、页面、分类/标签/归档、专栏、文库/文档、笔记、友链与瞬间采用新外壳；桌面和 390px 窄屏无横向溢出，导航、卡片、正文、侧栏可读。
- T3：侧栏开合、遮罩、Escape、目录锚点、Ctrl/Command+K、搜索结果跳转、评论和现有短代码交互保留；脚本不重复初始化，不新增浅色切换。
- T4：make css、css-drift-check、make build、make merge-check/组合态测试与真实浏览器核验。截图保存于本目录，记录实际服务资源 hash。

分工：样式专家独占 build/stylus、生成 main.css、version.go 和新增构建测试；模板专家独占 templates 与已有前台辅助 CSS及其 DOM 断言；运行时专家独占 JS/runtime 与浏览器回归；外观专家独占 settings 样式映射；预览专家独占静态预览。主 agent 负责资源版本、联调、验收记录与真实浏览器，接收各专家已释放的文件后完成集成修正。上游/本地核查 agent 只读且不派生。共享 DOM 契约采用上游 primitives/shell.ejs 与 region.ejs；既有 ctx 与 Go ViewData 保留。独立任务并行，最终页面验收由主 agent 亲自完成。

## 本次实测记录

2026-10-13，使用隔离的 SQLite、内容、site.yml 与 uploads；未用真实站点数据作为夹具。

- T1：2.0.0 干净源编译与 `make css-drift-check` 通过；完整 Console/UC 构建后重编 Go。`served-assets.json` 中开发模式和 embed 模式 12/12 资源内容与最终源码相符，CSS/ESM 的 MIME 正确。
- T2：主 agent 真实浏览器在 1280px 桌面、390px 窄屏检查 13 种公开路由（各 13 条，共 26 条），无横向溢出；见 `desktop-pages.json`、`mobile-pages.json`。截图为独立测试站点，并非用户生产内容。
- T3：实际操作品牌搜索、窄屏 Ctrl+K、单次 Escape 关闭与返回焦点、抽屉开合、桌面左栏收成 64px、目录跳转约 32px 留白、分享二维码展开（136px）、评论加载后的空态。Wiki 项目搜索经 JSON 索引降级，返回当前项目结果，跳转后正文命中词高亮。
- T3：原 radio 16px、checked 圆点 8px／unchecked 0；图库 square 比例为 1；旧 About 卡片与返回图标恢复；Galaxy 无图片背景的 canvas 采用 absolute 布局并真实绘制。搜索表单渐变实际匹配新 DOM，聚焦时 opacity=1。
- T4：9 个独立 Chromium 用例全部通过（ESM、两侧断点、互斥抽屉、焦点、搜索、BFCache 生命周期、Wiki FTS 空结果降级、普通空结果、迟到请求、索引刷新失败时沿用有效缓存）。新增真实缺陷用例先红后绿。Stellar 资源单测 83 项、Console 单测 229 项，以及 Go 定向检查通过；完整本地合入使用仓库 `make merge-check`，该门禁在 feature 和 dev 组合态分别执行 `make test`。

独立源码审查发现的旧内容与搜索兼容缺口均已修正；外部分享平台的实际发送、真实用户登录与 AI 付费请求未执行。Hexo 构建管线及其它未移植能力见 `docs/stellar-compatibility-matrix.md` 2.1。未推送、发布或部署。

体验入口：本次隔离预览 `http://127.0.0.1:8098/`。后续日常运行仍从根 dev 使用 `make dev`，其配置端口由本地 `config/app.yml` 决定。
