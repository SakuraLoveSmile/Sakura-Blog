# Feedback 专属设置页验收基线（v1.7.1）

## 背景与目标

v1.7.0 起 Feedback 组件（`@feedback/web` v0.5.2 的 `<feedback-widget>`）接入配置埋在
「主题设置 → 增强」页底部一节。本任务把它升级为「系统设置」下的专属 tab「反馈」，
并补齐运维所需的连接自测能力，随 v1.7.1 发布。

## 参照

- 迁移前形态：`modules/console/web/src/modules/appearance/EnhancementsSettings.vue` 底部
  「反馈组件」一节（本目录 `before-enhancements-feedback.png`）。
- 视觉规范：`resource/apple-design/`（SKILL.md → css.json → components/index.json），
  复用现有 `settings-*` 类、`apple-input`/`apple-select`、VSwitch/VButton/VTabbar，
  不新增裸色值与平行组件。

## 验收项（逐项勾选）

- [x] `/settings` 出现「反馈」tab（icon `ri:feedback-line`），位置在「搜索」之后、「高级」之前
- [x] tab 内含完整表单：启用开关、api_base、app_id、app_name、app_version、
      launcher_mode（orb/tab）、side（left/right）、launcher_bottom、capture_mode（viewport/off）
- [x] 「连接测试」按钮：POST `/api/v1/settings/feedback/check` 提交**当前草稿** api_base，
      后端 GET 其 `/healthz`（4s 超时），返回 ok/status/latency_ms/error；
      前端展示可达性徽标（成功绿/失败红/检测中）；api_base 为空时按钮禁用。
      实测：生产 feedback 服务 200/86ms 绿徽标，127.0.0.1:9 不可达红徽标，改地址后旧结论转 stale
- [x] 入口位置示意：CSS-only 迷你视口，按 side/launcher_bottom/launcher_mode 实时渲染
      悬浮入口位置（orb 圆点 / tab 贴边条）；实测 left/right 切换位移、orb/tab 形态切换
- [x] 保存走既有 `emit("save","feedback")` → `PUT /api/v1/settings/feedback`（保存按钮
      `native-type="submit"` 挂 form submit），脏标记/离开拦截/JSON 兜底同步由
      SettingsView 既有机制覆盖；实测保存后前台渲染 `<feedback-widget api-base=...>`
- [x] `/settings?tab=feedback` 直达；legacyTabMap.feedback 指向本 tab；
      `/appearance?tab=feedback` 旧深链由 AppearanceView 改道 `/settings?tab=feedback`（实测）
- [x] ~~前端预览面板 PREVIEW_FOCUS~~ 不适用：widget 为 JS Custom Element，静态 srcdoc
      无可视内容；入口位置示意由 tab 内迷你视口承担
- [x] EnhancementsSettings.vue 不再含反馈组件表单节（迁移而非复制，保留一行迁移提示）
- [x] 后端 check 端点：仅允许 http/https scheme、禁 userinfo；不回传响应体、仅状态码与延迟；
      4s 超时、不跟随重定向；允许回环/内网（同机部署是标准形态，admin-only 端点）；
      具名 struct `FeedbackCheckResponse` 经 tygo 生成；Go 测试覆盖 200/500/不可达/非法 scheme/未鉴权
- [x] 移动端（390px）表单单列、示意与按钮不溢出（实测 scrollWidth 无溢出）
- [x] 仅深色模式；截图存本目录 `after-*.png`（桌面 check-ok/full + 移动）

## 非目标

- 不做反馈列表/收件箱（Feedback 服务自带管理端，本页只放跳转链接）
- 不改 site.yml 字段契约（反馈段 schema 不变，纯 UI 层迁移 + 一个只读 check 端点）
