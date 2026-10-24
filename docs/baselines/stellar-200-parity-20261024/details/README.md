# Stellar 2.0 前台细节复核

属于既有 `BLOG20261024-STELLAR`，权威记录是 `docs/stellar-200-full-port.md` 对应续查段。唯一视觉来源为 `https://xaoxuu.com/`；行为对照锁定的 Stellar 2.0.0 `9d11978a24fe6610e3a7dbab741cf62634539d05`，不是任意最新上游。

`reference-home.jpg`、`before-live-home.jpg`、`before-local-home.jpg` 和 `initial-contracts.json` 保存本轮初始现场；初始视口不同，不能作像素等同对比。`reference-home-aligned-light.jpg` 是后来与当前候选对齐 CSS 视口的参照截图。

`candidate-code.json` 绑定基线 `829959d3` 加本批源文件 SHA-256 与实际 embed 二进制；`runtime-binding.json` 核对本地公开页面、search JS 与 CSS 的实际 HTTP 字节。`browser-verification.json` 保存组合态真实 DOM/交互、四外观自动浅深色、minimal 显式配色及 1280/800/390px 的结果。这里的测试站是 8126 纯合成夹具，不是生产内容。

验收覆盖搜索链接与重绘 hover、Story/引用 SVG mask、默认及页面作者、页脚作者、updated 排序、跨集合 recent、linklist 高亮和 references/许可证链接差异。Go 定向回归同时覆盖无摘要、fallback 搜索、安全 CSS 字符串、整数作者 ID 顺序、带点作者 ID、私有作者字段过滤和未公开集合排除。

截图为原生 JPEG 1272×716，与 CSS 视口并不一致；390px 结论来自 DOM 几何及真实菜单、设置、搜索操作，未使用失真的窄屏截图宣称通过。持续 loading 动画、真实 SiteInfo 外部服务和完整原 T1–T7 能力清单未在这批局部修复中重新验收。参照站个人字体、内容与菜单保留为配置差异。

当前源码与本地浏览器结果为待体验；线上二进制未部署。发布、生产实测与用户验收分别记录。
