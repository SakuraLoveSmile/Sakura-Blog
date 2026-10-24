# Stellar 2.0.0 配置与内容边界

固定上游 `xaoxuu/hexo-theme-stellar` tag `2.0.0`，commit `9d11978a24fe6610e3a7dbab741cf62634539d05`。`schema.json` 来自官方 `_config.yml` 和 schema/rules，不手写第二份字段白名单；其中保留源文件 SHA-256、字段顺序、默认值、类型、值域、边界与 4 种 Hero 参数声明。

生产路径全部使用 Go。生成器和官方 oracle 只在开发时需要 Node/js-yaml，不需要 Hexo 运行时；依赖已存在的 Go 标准库和 `golang.org/x/net/idna`。

## 调用

- `ParseTheme(raw, Options)`：完整默认配置深合并，然后校验并转换官方 runtime camelCase；`DefaultTheme()` 每次返回独立副本。
- `ParsePage(raw, Options)`、`ParseCollection(raw, Options)`：保留省略、null、false、0 与空列表的不同含义，不为未声明的覆盖补默认值。Front Matter 的官方宿主字段原样保留；Collection 缺少名称时生成名称 fallback 和 warning。
- `Options.Recover=false` 为 strict；`Recover=true` 使用官方按字段/列表项删除的 recovery，清空原本非空但全部无效的数组时回到上一层/default。Root 和 Page `collection.profile/id` 结构错误始终 fatal。错误可用 `errors.As(err, *SchemaError)` 获取 `Issues`；成功时 `Result.Issues` 是带 action 的 warning。
- Provider/Widget 参数袋按官方开放边界保留原始参数名和值，声明的 JS/CSS 资源字段仍执行安全校验。
- `Get(runtime, "appearance.colorScheme")` 使用点路径。含点的 Record key 通过父 map 精确访问。
- `ValidateCollectionProfile(config, source, profile)` 与 `ValidatePageProfile(config, source, profile)` 接受已经归一化的 runtime 对象，删除上游 profile 不支持的表现字段并产生 warning；不猜 profile。
- `CreateCollectionRegistry(map[sourceKey]parsedConfig)` 的 key 是 `wiki/<id>`、`topic/<id>`、`notebooks/<id>`，没有 `source/_data/` 前缀或 `.yml` 后缀。
- `ResolveContentMembership(MembershipInput{Kind: "posts"或"pages", Source, PagePath, Config, Registry})` 检查显式归属、关系/路径推导、冲突、歧义与缺失集合；`Config=nil` 且 `Issues` 非空代表结构错误，不可继续渲染/保存。

Recovery 不是迁移工具：产品必须在进入此库前检查迁移版本，strict 校验导入/保存候选，不能用 recovery 静默忽略未迁移旧配置。该库不读取 SQLite、配置文件或服务，不执行迁移、网络访问与渲染。URL 规则对应上游 WHATWG 校验边界；实际远程取数仍须由产品 `safefetch` 等权限边界控制。

## 重新生成与验证

在仓库根运行（`STELLAR_SOURCE` 指向锁定的独立官方导出，`NODE_PATH` 指向已有 js-yaml 所在 node_modules）：

```sh
node internal/stellarv2/generate/schema.cjs "$STELLAR_SOURCE"
node internal/stellarv2/generate/oracle.cjs "$STELLAR_SOURCE"
go test ./internal/stellarv2/...
```

官方 oracle 包括每个声明节点的类型/null/默认/枚举/值域、开放 Record key、严格与恢复、官方复杂例子、4 种 Hero 全部 options，以及导航/HTTP/Resource URL 边界。预期输出来自官方 parser，不由 Go 实现自行生成。Go 测试比较完整运行时对象的规范 JSON SHA-256 与全部诊断，另外对照全部 profile capability 和 membership 结果，并检查输入/默认对象不共享可变状态。测试夹具以 gzip 保存，测试不执行 Node。
