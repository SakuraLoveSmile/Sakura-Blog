# Stellar 2.0 HTML 后处理

锁定上游 `9d11978a24fe6610e3a7dbab741cf62634539d05`。本包仅使用 Go 标准库及项目已有的 `golang.org/x/net`，不要求生产 Node、Hexo、CGO 或文件系统。

## 装配顺序

1. 调用者完成 Markdown/shortcode 渲染及作者 HTML 净化。
2. `ProcessContent(content, Context)`：`md_table` 包裹普通表格，再执行 `md_link`。随后构造页面模型，供模板引用。普通链接使用 `Context.Icon("default:link")` 的**内联**结果；不要传延迟占位回调。`Theme` 为严格归一化的 camelCase 主题，链接服务使用 `services.siteInfo[provider].endpoint`。
3. 模板输出整页，统一给托管脚本添加 `data-stellar-script`；本包不修改这些标记。
4. `ProcessSite(html, Context)`：逐字扫描真实图片标签，执行 lazyload → onerror。保留 script/style/comment 及其他内容原文；`data-src`、`srcset`、`no-lazy` 按上游规则处理，保留作者自定义 onerror。
5. 调用 `internal/stellarimages` 的 metadata/enrich（同一整页快照），然后调用 clientconfig 的 partial-navigation 后处理/签名，再发布。

`Context` 为 `{Theme map[string]any, SiteURL string, Icon func(string)string, IconData func(string)string, Language string}`。`SiteURL` 必须是站点根 URL，包含实际子路径并以 `/` 结尾，例如 `https://example.com/docs/`，不能传文章永久链接。整页 fallback SVG 的公开路径默认是这个 root 下的 `images/stellar-error.<sha16>.svg`。自定义普通 fallback URL 保留相对语义。

`ProcessContent`/`ProcessSite` 返回 `(string, error)`。需要增强的链接缺少 `default:link`，或需要失败处理的图片缺少 `image:onerror` 时明确报错。它们不会执行网络请求。作者已有不支持的 URL 按上游保留原样；严格配置入口由 `stellarv2` 负责，本包不重复解析配置。

## 图标与资产发布

`RenderIcon(key,args,inline,icons)` 对应 `utils.icon`：URL 输出 `<img>`，`inline=true` 返回内联值，其他 SVG 输出 `svg.icon[data-icon]`。`args` 必须来自可信模板，不能传作者原始属性。未知键显示为转义文本。注册表 SVG 属于可信生成资料，自定义 SVG 必须在进入注册表前完成既有校验。

`IconAssets(icons)` 输出 `[]Asset`；`ImageFallbackAsset(IconData("image:onerror"))` 输出 `*Asset`。`Asset` 含 `Path`、`ContentType`、`Body []byte`，由 root 发布。SVG fallback 保留注释/署名并按解码字节计算 SHA256 前 16 位，图标 JSON 则按上游删除 SVG 注释。

| 资产 | `Asset.Path` | stock 消费路径 | 推荐本地映射 |
| --- | --- | --- | --- |
| 图标命名空间 | `js/icons/<ns>.json` | `/js/icons/<ns>.json`，经 `context.assets.resolve` | 若 resolver 加 `/assets`，发布 `/assets/js/icons/<ns>.json` |
| SVG 失败图 | `images/stellar-error.<sha16>.svg` | 站点 root 下的同名文件 | 默认发布 `<site-root>/images/...`；若统一 `/assets`，将 `IconData("image:onerror")` 回调改为该资产最终 URL，同时发布生成的 SVG |

不要改官方 `deferred-icons.js` 的硬编码 stock 路径；发布地址必须与现有 assets resolver 一致。图标未知 namespace 的请求由 runtime 显式警告；生成器拒绝会逃逸路径的 namespace。错误 SVG data URL 与上游一致保留原 URL，不伪造生成成功。

## 上游链路归属

本包覆盖 `filters/lib/md_table.js`、`md_link.js`、`img_lazyload.js`、`img_onerror.js`、`lib/html-images.js`、`image-fallback.js`、`events/lib/utils.js` 的图标语义以及 `generators/stellar-icons.js` 的图标命名空间/失败图资产。

`img.js` 的 image-metadata/enrich 归图片模块；page-view-model 的 projection 归页面模型；partial-navigation 归 clientconfig；`stellar_icons` 的全局 client assets 归 clientconfig。`runtime-minify.js` 是构建时排除原生 ESM 的策略，构建链应确保 runtime 模块不经 classic-script 压缩器。`pretty_urls.js` 是页面生成器路由映射，归生成器模块。Markdown inline 标签在 tags 注册入口，不另行重复过滤。

## 验证

`go test -race ./internal/stellarhtml` 使用提交的官方 oracle，不依赖 Node。样例包括 342 个正文、276 个整页图片、36 个图标渲染及 8 个 fallback 资产。正文按 DOM 比较（HTML 序列化的引号与 void-element 表示允许不同）；图片另检查 inert 区域逐字保留和幂等。

开发再生：在忽略目录安装 `cheerio@1.1.0`、`hexo-util@4.0.0`，设置 `NODE_PATH` 后执行 `node internal/stellarhtml/generate/oracle.cjs <locked-upstream-root> internal/stellarhtml/testdata/official.json`。不要更改项目依赖或生产运行时。

两处安全修正保留产品边界：图片单引号/无引号原始属性改为双引号时，转义值防止产生额外作者事件属性；未知 icon key 只显示转义文本。扫描到未闭合 script/style/comment/tag 时保留剩余原文，避免上游扫描器截断页面。这些不扩展作者 HTML 净化权限。第三方 siteInfo 服务请求与真实浏览器生命周期须由集成方验收，包内差分不是外部服务证据。
