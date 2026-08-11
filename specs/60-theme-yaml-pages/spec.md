# Spec 60 · 主题配置复刻 + 页面统一管理

## 目标

1. 后端 `StyleConfig` 对齐 Stellar 1.38 `_config.yml` style 面；前端样式由配置运行时驱动；设置页「主题 YAML」编辑器保存即热更新前台。
2. 控制台「页面」模块纳入 wiki 与友链两个特殊页面模块（数据仍存各自表）。

## Phase A · 后端

- [model_style.go](backend/internal/template/model_style.go)：`StyleFontSize` 增 `code`/`codeblock`；`StyleBorderRadius` 扩 1.38 七档（card-l/card-s/bar/image-l/image-s，保留四档）；`StyleCodeblock.highlightjs_theme`；新增 `StyleAnimatedAvatar`；`header_prefix` 值为 URL 时注入图标 background-image。`InjectCSS` 增对应注入（代码字号、七档圆角变量、prefix 图标）。
- [settings.go](backend/internal/api/settings.go) + [router.go](backend/internal/api/router.go)：`GET/PUT /api/v1/settings/style/yaml`（goccy/go-yaml 解析校验 → `SaveSection(style)` → `RebuildSite` 热更新；解析失败 400+行号）。

## Phase B · 控制台主题 YAML

- 新增 [ThemeYamlTab.vue](frontend/console/src/modules/settings/ThemeYamlTab.vue)；[SettingsView.vue](frontend/console/src/modules/settings/SettingsView.vue) 外观组增「主题 YAML」tab；[api](frontend/console/src/api/index.ts) 增 `getStyleYAML`/`updateStyleYAML`。与 AppearanceTab/高级·JSON 同源（style 段）。

## Phase C · 页面统一管理

- [PageList.vue](frontend/console/src/modules/pages/PageList.vue) 顶部 `apple-segmented` 三分段（普通页面/文库/友链），深链 `?module=wiki|friends`；文库内嵌 WikiDocsView、友链内嵌 FriendList；普通页面=原列表。数据不变（wikidocs/links 表）。

## 验证

- `make build`+`make test` 全绿；重启 dev。
- 浏览器：`GET/PUT style/yaml` 往返；PUT accent/#00cc66+body 18px+card 30px 后前台注入 `--accent/--fsp/--radius-card` 生效，回退 `{}` 恢复；「主题 YAML」textarea 显示 YAML；页面分段切换 wiki/友链/普通 + 深链正常。

## Out of Scope

- 不重编 Stylus 改编译期字面量；仅运行时变量/覆盖能驱动的键纳入。
- 不把 wiki/友链数据迁移进 pages 表。
