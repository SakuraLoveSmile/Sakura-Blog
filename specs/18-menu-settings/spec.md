# Spec 18 · 菜单可视化管理增强

- **分支**：`feature/menu-settings` · **Wave 4** · **隔离等级**：低（仅 console 设置页 + 少量 api 校验）
- **主模块**：`console/src/modules/settings/SettingsView.vue`（菜单 tab 区块独占）

## Why

菜单后端链路已通：menubar 作为 settings 分段存 SQLite（`config.SectionMenubar`），`PUT /api/v1/settings/menubar` 保存即热更新前台。Console 设置页也已有「菜单」tab，但只是**裸网格表单**：无拖拽排序、无图标预览、无配色取色器、无 URL/必填校验，增删体验与 Halo 菜单管理差距明显。本 spec 把菜单 tab 升级为可视化编辑器，后端零表结构改动。

## 功能需求

1. **拖拽排序**：菜单项可上下拖拽调整顺序（或提供上移/下移按钮作为降级交互），保存后前台 menubar 顺序即时生效。
2. **图标实时预览**：icon 字段输入 Iconify key（如 `solar:documents-bold-duotone`）时行内实时渲染预览；`default:` 前缀 key 给出「内置图标」占位提示。
3. **主题色取色**：theme 字段升级为原生 `<input type="color">` + 十六进制文本框双向同步；空值代表跟随默认色。
4. **字段校验**：保存前校验——title/url 必填、id 必填且同列表内唯一、url 必须以 `/` 或 `http(s)://` 开头、theme 为空或合法 `#RRGGBB`；不合法时行内标红并阻止保存。
5. **前台预览提示**：保存成功后 toast 附「查看前台」链接（新开 `/`），确认热更新生效。
6. **列数联动提示**：columns 输入时提示原版语义（菜单折叠为几列网格）。

## What Changes

- `console/src/modules/settings/SettingsView.vue`：
  - 菜单 tab 区块重写为独立子组件 `console/src/modules/settings/MenubarEditor.vue`（SettingsView 引用，避免单文件继续膨胀）。
  - 拖拽用原生 HTML5 drag events 或轻量实现（**不引入**新的拖拽依赖库）；每行保留上移/下移按钮兜底。
  - 图标预览复用项目已有 `@iconify/vue` 的 `<Icon>` 组件（列表行内已在用 `ri:delete-bin-line`，同款引入方式）。
  - 校验逻辑纯前端函数，错误对象按行索引存储，行内红框 + 底部汇总。
- `internal/config/reload.go`（可选加固）：`SaveSection` 对 menubar 段做服务端兜底校验（items 内 title/url 非空），非法时返回 400 文案；防绕过 Console 直接调 API 写坏菜单。

## 实现步骤

1. 抽出 `MenubarEditor.vue`：props 接收 menubar 数据引用，emit save 事件回 SettingsView 统一走现有 `save('menubar')` 通道；样式沿用 SettingsView 现有 `inputCls`/`btnCls` 类。
2. 实现行拖拽（dragstart/dragover/drop 交换数组元素）+ 上移/下移按钮。
3. 实现图标预览列与取色器列。
4. 实现校验函数与行内错误展示；保存按钮在有错误时禁用。
5. 服务端 `SaveSection` menubar 分支加非空校验（+少量行）。
6. 手动回归其余 settings tab 不受影响（site/rightbar/footer/wiki/comments/article/security/advanced）。

## 技术决策（无需再问）

- **不做菜单多级/嵌套**：原版 Stellar menubar 就是单层网格，保持对齐。
- **不做独立「菜单管理」页面**：留在设置页 tab 内，符合现有信息架构；Halo 的独立菜单页面形态不照搬。
- **不引第三方拖拽库**（vuedraggable 等）：菜单项数量个位数，原生 drag 足够，避免依赖膨胀。
- **id 字段语义**：保留手填（前台 active 态判断依赖 id 与页面对应），校验唯一性即可，不自动生成。
- **theme 空值**：序列化时保持空串，前台模板已有默认色兜底逻辑，不强制填色。

## Impact

- **新增**：`console/src/modules/settings/MenubarEditor.vue`
- **修改**：`console/src/modules/settings/SettingsView.vue`（菜单 tab 区块替换为子组件引用）、`internal/config/reload.go`（menubar 校验，+少量行）
- **不影响**：store、模板、前台路由、其它 settings 分段、api 路由

## ADDED Requirements

### Requirement: 菜单可视化编辑
The system SHALL provide a visual menubar editor in Console settings with drag-sort, icon preview, color picker, and validation.

#### Scenario: 拖拽排序
- **WHEN** 拖动菜单项「友链」到「博客」之前并保存
- **THEN** 前台刷新后 menubar 顺序为友链在前，无需重启服务

#### Scenario: 图标预览
- **WHEN** icon 输入框填入合法 Iconify key
- **THEN** 行内即时渲染该图标；清空则显示占位

#### Scenario: 校验拦截
- **WHEN** 某菜单项 url 为空或 theme 填了非法色值
- **THEN** 该行标红、保存按钮禁用、底部提示具体错误，API 不被调用

#### Scenario: 服务端兜底
- **WHEN** 绕过 Console 直接 PUT 一个 title 为空的 menubar JSON
- **THEN** API 返回 400 且现有菜单不被破坏

## 验证

1. `go build ./... && go vet ./...` 通过；`cd console && npm run build` 通过
2. 增删改排序菜单各一轮，前台 menubar（含图标、配色、active 态）即时正确
3. 校验四条规则逐一触发并拦截；服务端 curl 直连非法 JSON 返回 400
4. Console 亮/暗模式下编辑器样式正常；其余 settings tab 保存功能回归无异常
5. 重启服务后菜单保持（SQLite 持久化生效）

## Out of Scope

- 多级/下拉菜单
- 独立菜单管理页面与多套菜单方案
- rightbar/footer 等其它分段的同类可视化增强（形态不同，另行评估）
