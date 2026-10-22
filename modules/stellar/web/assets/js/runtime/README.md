# Stellar 2.0.0 runtime

来源：https://github.com/xaoxuu/hexo-theme-stellar/tree/2.0.0/source/js/runtime
固定提交：`9d11978a24fe6610e3a7dbab741cf62634539d05`，许可证见 `LICENSE`。

`asset-loader.js`、`extension-registry.js`、`request-cache.js`、`legacy-request-adapter.js` 保留上游接口。`index.js` 安装 legacy request adapter，并使用 document/region scope 管理挂载与清理；manifest 使用上游 version=1 契约，入口 query 传播至依赖模块和扩展。

实际 document shell 控制位于 `../main.js`。它消费上游 site-shell/region/drawer DOM，保留桌面左栏状态；移动抽屉隔离背景并循环焦点，关闭、断点变化及 pagehide 归还原交互状态，BFCache pageshow 重新同步。嵌套搜索采用原生模态 dialog。`extensions/shell.js` 是历史适配文件，当前严格 V2 manifest 不挂载它。

严格搜索按 manifest 排他选择：官方 local、Algolia 或关闭；Blog 的 Go 搜索为独立扩展，启用后由 `extensions/blog-search.js` 挂载，并向公开快照 API 传递 scope、include_content 与 domain，JSON 降级复用路径和 TTL 缓存。`extensions/blog-title-search.js` 支持关闭正文后的标题、标签和分类匹配。

partial navigation 由配置启用；服务端生成可比较的 collection/signature，客户端只在兼容时替换页面 region，并取消、卸载及重新挂载区域消费者。配色支持浅色、深色和自动，appearance 支持 card/glass/minimal/flat。真实界面与外部服务是否完成验收，以本次运行证据为准。
