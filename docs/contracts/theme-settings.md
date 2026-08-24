# Stellar 主题契约

Console 将 Stellar 视为可替换的主题模块，不直接依赖主题内部实现。主题模块负责自报 upstream 身份、挂载前台路由和资源，并提供失败回退；Console 只消费明确的 API/mount 契约。

## 模块边界

- Stellar 模块自报 upstream owner/repository；Console 不硬编码 `stellarOwner` 或 `stellarRepo`。
- `stellar.Mount` 负责 SSR 路由、公共 API、模板/资源和主题相关回调。
- Console 负责 `/console/*` SPA 与 `/api/v1/*` 管理 API，不从 Stellar 读取模板内部状态。
- 主题资源不可用时，Stellar 通过组合根提供的 `NoRoute` 回退；Console 不接管前台路径。

## 主题设置 API

主题设置的边界是 style YAML：

- `GET /api/v1/settings/style/yaml`：返回当前 style 段 YAML。
- `PUT /api/v1/settings/style/yaml`：接收 YAML，服务端解析和校验后保存，并触发主题配置重建。

该接口只允许系统管理员使用。Console 的主题设置页不直接写文件，也不新增 Stellar 专属设置 Tab；新增设置应优先进入数据驱动的 YAML/schema。

## 失败与回退

- API 预览失败返回明确错误，不修改已保存配置。
- 上游版本检查失败仍返回本地版本和错误状态，不阻断 Console 加载。
- 主题模块缺失模板/资源时启动失败应带出明确路径；开发模式可从磁盘读取，embed 模式只读取构建产物。

## 扩展点命名

官方扩展点使用 `<domain>:<capability>:<action>` 命名：

- `widget:registry:register`
- `editor:toolbar:item:register`

扩展点先采用编译期注册；运行时装载只留给后续钩子型插件能力。
