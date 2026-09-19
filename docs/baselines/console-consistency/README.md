# Console 交互统一与无障碍基线 (阶段 D)

本基线记录《博客稳定性、Console 一致性与 Stellar 适配整改计划》中**阶段 D：Console 交互统一**的设计与实现基线、唯一主视觉参照、验收项清单与视口规范。

---

## 1. 唯一参照图与设计规范

- **唯一参照图来源**：`docs/baselines/settings-gap-audit/appearance-final.png`
- **本地归档副本**：`docs/baselines/console-consistency/reference.png`
- **色彩与主题约束**：
  - 纯深色模式交付，禁止引入浅色模式切换、浅色专有 token 或浅色基线。
  - 所有颜色控件在未配置自定义值时绑定明确的回退默认色（主题色 `#2E8DFF`、强调色 `#ff9800`、链接色 `#2196f3`、左栏底色 `#1c1c1e`），避免 HTML `<input type="color">` 默认回退为 `#000000` 产生“默认黑”误导。
  - 颜色项明确展示「继承默认值」与「已覆盖」状态徽章，并提供一键「恢复默认」按钮。

---

## 2. 交互与无障碍验收清单

### D1. 登录行为与跳转安全
- [x] 移除未实现的 Apple / Google / GitHub 社交登录伪入口及多余装饰。
- [x] 移除未生效的「保持登录」复选框（JWT 统一由 HTTP-only Cookie 驱动）。
- [x] 抽离 `resolveSafeRedirect(redirectQuery, fromQuery, defaultPath)`：
  - 严格限定以单一 `/` 开头（拒绝 `//`、`/\`、包含 `:`、协议头、控制字符等开放重定向风险）。
  - 禁止重定向至自身 `/login`，避免无限重定向死循环。
  - 兼容 `redirect` 与旧版 `from` 查询参数。
- [x] 普通密码登录与 2FA 验证完成使用同一跳转收敛函数。
- [x] 路由守卫跳转参数统一使用 `redirect: to.fullPath`。

### D2. 设置工作流与样式 YAML
- [x] 设置中心各 Tab 建立统一的加载、保存和失败状态机与保存按钮防重提交锁（`savingKey` / `saving`）。
- [x] 快照隔离比对机制：保存成功后仅更新当前段的快照（`updateSectionSnapshot`），请求途中或跨 Tab 进行的新输入保留脏标记，防止未保存数据丢失。
- [x] 设置中心与主题外观页均接入 `onBeforeRouteLeave` 与 `beforeunload` 未保存修改防护守卫。
- [x] “自定义 CSS” 统一更名为“样式 YAML”：
  - 异常提示明确展示错误信息与行号，不再静默吞错。
  - 后端触及配置段改用具名结构体 `StyleYAMLResponse` 与 `StyleYAMLUpdateResponse`，并保留未知字段通道（`mergeUnknownFields`）。
  - `tygo.yaml` 同步生成前端 TypeScript 类型声明。

### D3. 公共组件与无障碍 (A11y)
- [x] **VModal**：
  - 声明 `role="dialog"`、`aria-modal="true"`、`:aria-labelledby`。
  - 支持 Esc 快捷键关闭。
  - 开启时捕获焦点（Focus Trap）在弹窗可交互元素间循环，关闭后焦点平稳恢复至打开前的触发元素。
- [x] **Dialog**：
  - 警告与错误弹窗赋予 `role="alertdialog"`，信息弹窗赋予 `role="dialog"`。
  - 声明 `aria-modal="true"`、`aria-labelledby`、`aria-describedby`。
  - 支持 Esc 键取消与 Tab 焦点限制，关闭后恢复原触发焦点。
- [x] **FilterDropdown**：
  - 触发器使用原生 `<button>`，声明 `aria-haspopup="listbox"`、`:aria-expanded="open"`。
  - 菜单标记 `role="listbox"`，选项标记 `role="option"` 与 `:aria-selected`。
  - 支持 `ArrowDown` / `ArrowUp` 键盘高亮切换、`Enter` / `Space` 选中与 `Escape` 关闭，关闭后焦点恢复至触发器。
- [x] **VSwitch**：
  - 底层原生 `<button>` 绑定 `:disabled`、`:aria-disabled`、`role="switch"` 与 `:aria-checked`。
  - 全站 20 余处 `VSwitch` 调用全面补齐显式 `aria-label`。
- [x] **VTabbar**：
  - 容器声明 `role="tablist"`，各标签声明 `role="tab"` 与 `:aria-controls="tabpanel-${item.id}"`。
  - 实行 Roving Tabindex（当前项 `tabindex="0"`，其余项 `tabindex="-1"`）。
  - 支持 `ArrowRight` / `ArrowLeft` / `ArrowDown` / `ArrowUp` / `Home` / `End` 键盘方向导航与自动焦点轮转。
- [x] **DefaultLayout**：
  - 侧边栏菜单项与子菜单项改用原生 `<router-link>`，搜索与通知改用原生 `<button>`，支持键盘聚焦高亮与回车访问。
  - 移动端底部导航根据启用的菜单数量动态分配网格列数（`gridTemplateColumns: repeat(...)`），并接入 iOS `safe-area-inset-bottom`。
  - 移动“更多”抽屉赋予 `role="dialog"` 与 `aria-modal="true"`，增加显式关闭按钮、Esc 监听与抽屉打开/关闭焦点管理。
- [x] **仪表盘与数据图表**：
  - `DashboardGrid` 处于编辑状态时，为每个组件浮层加入键盘可访问的「向前移动」（首项禁用）和「向后移动」（末项禁用）操作按钮。
  - `TrendChartWidget`、`HourlyChartWidget` 与 `CategoryDonutWidget` 均内置 `<div class="sr-only">` 数据表格与文字摘要，供屏幕阅读器与文本终端读取。

---

## 3. 视口规格

| 视口类型 | 宽度范围 | 导航布局 | 侧边栏/底部栏特征 |
| :--- | :--- | :--- | :--- |
| **桌面宽屏 (lg)** | $\ge 1024\text{px}$ | 侧边展开模式（`w-64`） | 展示图标、文字、角标、多级折叠菜单与底部用户信息 |
| **桌面紧凑 (md)** | $768\text{px} \sim 1023\text{px}$ | 侧边收起模式（`w-16`） | 仅展示图标，悬浮浮层提示文字，未读通知叠在头像角标 |
| **移动端 (sm/xs)** | $< 768\text{px}$ | 底部动态网格导航条 | 动态计算列数，适配 `safe-area-inset-bottom`，全屏抽屉承载更多菜单 |

---

## 4. 格式差异整理

1. **类型导出文件**：`packages/api-client/src/generated.d.ts` 由 `go generate ./modules/console/api` 统一由 tygo 生成，保留其固有的代码格式。
2. **样式隔离**：仪表盘移动按钮与组件焦点轮廓统一追加在 `modules/console/web/src/modules/dashboard/dashboard.css`，保持与 Apple Dark 令牌一致。
