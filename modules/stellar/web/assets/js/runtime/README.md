# Stellar 2.0.0 runtime

来源：https://github.com/xaoxuu/hexo-theme-stellar/tree/2.0.0/source/js/runtime
固定提交：`9d11978a24fe6610e3a7dbab741cf62634539d05`，许可证见 `LICENSE`。

`asset-loader.js`、`extension-registry.js`、`request-cache.js`、`legacy-request-adapter.js` 原样保留。`index.js` 仅移除了 legacy adapter 的导入与安装：Blog 的经典 Go 数据服务继续使用已有请求与缓存，避免切换其行为。manifest 使用上游 version=1 契约，入口 query 会传播至所有依赖模块和扩展。

`extensions/shell.js` 和 `extensions/search.js` 为 Blog 的适配扩展，采用上游 shell/region primitives，注册于 document scope，支持 pagehide 清理与 BFCache pageshow 重新挂载。搜索沿用 Blog 的 FTS5 优先、JSON 降级及 Wiki data-filter，不使用上游 Hexo 的索引 schema。尚未具备全部服务的区域卸载契约，因此不启用 partial_navigation。产品仍仅提供深色模式。
