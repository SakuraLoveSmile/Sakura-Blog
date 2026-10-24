# 编辑器 Stellar 标签预览修复

唯一视觉基线为用户提供的 `before-inline-preview.png`；`before-toasts.png` 记录重复弹窗。`reproduced-production.png` 为线上 EmbyBot 草稿的本次复现：24 个失败标签、24 条 `stellar page not found` 提示。

本次保持现有编辑器、标签卡片与布局，只修复候选正文的预览链路。

## 验收项

- [x] 已保存草稿重新打开默认编辑器后，Stellar 标签正常预览。
- [x] 新建文章和已发布文章的候选标签正常预览。
- [x] 标签请求失败保留卡片内错误，不逐个触发全局弹窗；登录过期处理继续有效。
- [x] 预览只使用提交的候选正文和公开站点快照，不读取隐藏文章正文、不写库。
- [x] 公开访问仍隔离草稿、加密、已删除和未来发布的文章。
- [ ] 真实构建、浏览器核验及仓库合并门禁通过。

## 本次验证

- 真实 renderer 回归先红：草稿、加密和软删除路径在 content/document 两种模式均返回 `stellar page not found`。修复后，新建、公开、草稿、加密、软删除、未来发布共 12 个候选组合通过；隐藏标记未泄露，数据库 `query_only=ON`，无网络回调。
- API 客户端回归先红：24 个失败的 content 请求产生 24 条全局提示。修复后 7 项 transport 测试通过，content 错误仍 reject，默认与显式 document 错误仍通知，401 仍交给宿主。
- `go test ./modules/stellar/routes -run 'TestStellar(ContentPreview|PublicVersionGate|ToolSnapshots)' -count=1` 通过。
- `make build` 通过，随后启动独立 config/DB/content/uploads 的 8098 站点。6 个 Console 入口资源与当前构建逐字相同，SHA256 见 `local-assets.json`。
- 主 agent 真实浏览器核验：已保存草稿重开 4 个 iframe、已发布文章 4 个、新文章 2 个，均无失败卡片和全局提示。故意提交未知配置字段时 4 个标签显示局部错误、全局提示为 0；恢复配置、刷新后 4 个预览恢复。截图为 `local-saved-draft.png`、`local-published.png`、`local-new.png`、`local-inline-error.png`。
- 独立只读审查未发现阻断问题。

合并门禁与生产结果在交付记录中分别报告；上述勾选表示本次实测，不代表用户已验收。
