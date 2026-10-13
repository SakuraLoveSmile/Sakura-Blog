# Stellar 2.0 动态外观

生产接口：`Build(theme map[string]any) (template.CSS, error)`，以及部署入口 `BuildWithOptions(theme, Options{RootURL, CSSURL, SkipHighlightStylesheet})`。输入必须是 `stellarv2.ParseTheme` 返回的完整 **camelCase** 运行时配置，不传旧 style、未经规范化的 snake_case 或局部配置。函数不改配置、不执行 Node、不读取文件、不请求网络；发生输出边界错误时返回空 CSS 和带字段路径的错误。

调用方先选 `main-{preset}.css`、选配 CSS 和对应 `main-appearance-{preset}.css`，再在最后的独立 `<style>` 中输出返回结果。`preset` 控制主样式选择；`colorScheme` 通过 SSR／官方运行时控制 `html[data-theme]`：light/dark 明确设置，auto 删除属性。此包同时输出浅色、深色与系统深色分支，不新增用户配色开关。

`RootURL` 是站点目录，`CSSURL` 是当前选中主样式的最终公开地址。background 相对图片按 CSSURL 解析，highlight 相对样式表按 RootURL 解析；根路径／绝对／data 资源保持标准 URL 含义，绝不按当前文章路径解析。便利 wrapper 的默认值等价 `/` 和 `/css/main.css`；仅指定站点目录时默认 CSS 路径随目录解析。主 agent 应传入部署 root 与实际 `/assets/css/main-{preset}.css` 地址，并承担旧相对路径的迁移候选和公开资源映射。

资源先通过输出边界检查，再用 `net/url.ResolveReference` 解析已校验的标准 HTTP／路径引用；不接受反斜线、控制字符或非标准 HTTP opaque 写法来模拟 WHATWG special URL 的额外容错。上游 schema／client builder 对 WHATWG special HTTP 资源可能采用浏览器兼容的解析与重写，例如将 `https:example.test/file.css` 规范化为标准 HTTP URL。本包不能把这种原始写法交给 Go 的 opaque URL 解析来冒充相同结果。

root 接线层必须先验证原始主题，再在**独立的已校验主题副本**中统一规范化 `backgrounds.leftbar.image`、`backgrounds.page.image` 和 `codeBlock.highlightStylesheet` 的资源输入；同一规范化结果提供给 client builder 与 appearance builder。不改 raw 保存值，不在本包偷偷猜 host／文章地址，也不因两条资源路径采用不同重写规则导致同一资源得到不同地址。

## 配置消费

- colors：传统 deg/无单位 HSL、RGB、十六进制和命名颜色按上游拆分 hue/saturation/lightness/alpha；主题 alpha 强制为 1/0.1/0.2/0.3，链接强制为 1/0.2，与上游相同。强调色按 Stylus 的 RGB／alpha 序列化规则输出。
- 现代色、`var()`、currentColor、非 deg 角度使用浏览器相对 HSL，原始来源保存在 `--stellar-source-color`；不把未定义变量替换成默认青色，也不伪造独立数值通道。Stylus 对 turn/rad/grad 数值会误当 deg，对现代／浏览器变量也无法计算；这些输入采用有效 CSS 的标准含义，不能宣称与错误的 Stylus 数值逐字一致。
- colorful：theme/accent 的全部边框、块、mark、代码块、文本 p1–p4、链接与透明链接按上游 light/dark 公式生成；保留命名色板与 dark tag 的原级联顺序。投票和 warning/error 保持固定来源色；light/dark 仅改变 `--theme`，保留原有来源继承。
- gradients、字体族、字号、100–900 字重映射、字体平滑、对齐、corner、七档 radius、代码滚动条全部写入已经进入编译 CSS 消费点的变量。
- leftbar：仅 glass 使用装饰背景；none 隐藏，image 使用安全引用与深色 0.75 opacity 因子，gradient 保留两套四色 radial/conic 材质。image 但图片 null 时按上游回落到 gradient。折叠桌面左栏取半 blur，移动端不改半径。
- page：图片、blur、saturation、overlay 实际覆盖对应背景节点；移除图片明确输出 none。Artalk 有页面图时边框透明，无图时恢复 block-border。
- checkbox/note：交互和边框写入既有动态变量。独立调用默认在返回样式开头以安全 `@import` 加载 highlightStylesheet，null 禁用。root 已通过 `clientconfig.Build` 的 `bundle.Styles` 按官方 head 方式消费该字段时，设置 `SkipHighlightStylesheet: true`，不再追加重复 import，也不冲破 preset 尾层的加载顺序；该标志仍保留资源校验。调用方不再强制旧高亮样式。

## 直接颜色消费者

全部原 `hsla(var(--hue) var(--sat) var(--light) / ...)` 派生 token 及下列直接消费者均被相对色覆盖，避免现代颜色仍保留旧数值通道：

| 选择器 | 属性／透明度 |
| --- | --- |
| `.related-wrap#comments .cmt-title p a` | `--link-a20` / 0.2 |
| `.md-text .tag-plugin.copy span` | background / 0.2 |
| story `.md-text.content h3:not([class])` before/after | background-color / 0.5 |
| story `.md-text.content h4:not([class])` before | background / 0.3；hover / 1 |
| story `.md-text.content>blockquote` 与 `.tag-plugin.blockquote` before/after | background-color / 0.25；hover / 0.5 |
| `.highlight .code .copy-btn:hover` | color / 1 |

story h2 和 topic 标题 before/after 的编译期 `$c-accent` 另覆盖为 `var(--accent)`。测试逐项扫描四份已提交主 CSS，不能只检查上述清单文字。

## 验证

`go test ./modules/stellar/appearance` 使用锁定 commit `9d11978a24fe6610e3a7dbab741cf62634539d05` 的开发 Stylus 生成夹具，比较传统颜色及背景的当前配置 token；另覆盖全部动态参数、拒绝逃逸／未闭合值、确定性和只读行为。

重新生成传统 oracle：

```sh
node modules/stellar/appearance/generate/oracle.cjs resource/hexo-theme-stellar
```

原生计算样式检查（开发用现有 Playwright；阻止全部网络请求，不启动服务）：

```sh
go run modules/stellar/appearance/generate/browser.go > /tmp/stellar-appearance-browser-fixtures.json
node modules/stellar/appearance/generate/browser.cjs /tmp/stellar-appearance-browser-fixtures.json
```

它检查四份主样式加载本包 CSS 后的真实颜色像素、colorful 文本派生、复制背景、字体、字号和字重，覆盖显式 light/dark 与 auto 的系统 light/dark。此证据不替代完整页面、交互和 Safari 验收。
