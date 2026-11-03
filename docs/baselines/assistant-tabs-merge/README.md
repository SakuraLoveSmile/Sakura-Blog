# AI 助手：合并「助手」与「知识库」

## 参照与来源

- 改动前唯一参照：`before-desktop-1280.png`（同一隔离 e2e 服务 `admin` 登录后截取，代码为 `dev` @ `ce8d7e6c`）。
- 改动前标签栏为「助手」「知识库」「设置」，且「助手」与「知识库」两个标签渲染的 `.assistant-layout` 文本完全一致（采集脚本输出 `assistantTabEqualsKnowledgeTab: true`）；切换只改变 URL `?tab=`。
- 基线对照图：`before-knowledge-tab-1280.png` 证明「知识库」标签与「助手」是同屏同内容。

## 验收项

- [x] 桌面 1280×900：标签栏只有「对话」「设置」，对话工作区与「知识库状态／使用说明」同屏。证据：`after-desktop-1280.png`，采集脚本输出 `tabs: ["对话","设置"] active: 对话`。
- [x] 旧链接兼容：`?tab=knowledge`、`?tab=assistant` 均落在对话视图且右侧知识库面板可见（`legacy ?tab=knowledge: active=对话, knowledgePanel=true`、`legacy ?tab=assistant: active=对话, knowledgePanel=true`）。
- [x] 「设置」标签仍进入 AI 设置（`settingsVisible: true`），证据 `after-settings-1280.png`；`SettingsView` 的 `ai: { path: "/assistant", tab: "settings" }` 入口不受影响。
- [x] 页头「重建知识库」保留在对话视图（`after-desktop-1280.png`、`after-mobile-390.png`）。
- [x] 移动端 390×844 无横向滚动：`{"scrollWidth":390,"innerWidth":390}`，证据 `after-mobile-390.png`。
- 不适用 · 浅色模式：Console 现有全局样式为深色编辑台（`html { color-scheme: dark }`），AI 助手页无独立配色分支，本次未改配色。
- 未验证 · 真实 RAG 问答：e2e 隔离站点未配置对话/向量模型，本次只验证页面结构与标签合并，未发起真实问答；问答接口与渲染路径未改动。

## 采集方式

Playwright（仓库根 `node_modules` 的 1.62.1，Chromium）直连隔离 e2e 服务：`scripts/e2e/run-server.sh` 以独立临时 root 与 SQLite 启动 `-dev` 服务（端口 8096），用 `admin` / `e2e-password` 登录。脚本未入库（会话 scratchpad）。改动前服务使用 `dev` 根工作区同一 `dist`，改动后服务使用本分支 `make console` 产物；两次采集的入口 HTML 与 chunk 均由服务启动日志确认从对应 `modules/console/web/dist` 加载。

## 与设备证据的边界

- 90 分钟内的验证仅覆盖 Chromium 桌面 1280×900 与 390×844；未测 Safari/Firefox、真实触屏设备与真实 RAG 数据。
- 合并只删除重复的标签与路由分支，未改样式规则、对话渲染、引用卡片与重建接口。
