# BLG-20261111-03：站点设置页补齐「个人资料」（原生账号）

- 状态：待体验（本地已合入 dev，未推送、未发布）
- 分支：`codex/feat/20261111-BLG-20261111-03-settings-profile`
- 工作树：`/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261111-feat-BLG-20261111-03-settings-profile`
- 基点：`dev` @ `8af5aec7`
- 合入目标：`dev`
- 视觉基准：<https://xaoxuu.com/settings/>（Stellar 2.0.0 示例站设置页）

## 背景与目标

用户对比线上 `/settings/` 与示例站设置页，确认差异是缺「个人资料」整块：示例站有头像 + 昵称/Email/URL + 保存/退出登录，我们的页面直接从「缓存」开始。

上游只在评论后端是 artalk/waline/twikoo 时渲染该块（`layout/settings.ejs` 的 `providerSupported`）。我们站点没有第三方评论后端，读者身份来自站点自己的账号（`/api/auth/*`），因此该块被整体隐藏。本次把原生账号接入同一套设置页资料块，并支持示例站同款 URL 字段。

## 已确认决定

1. 目标页是**站点前台 `/settings/`**，不是 Console 设置页。
2. 补上「个人资料」，含 URL 字段，和示例站一致（用户在 2026-11-11 明确选择）。
3. 未配置 `comments.provider` 时视为原生评论（`data-provider="native"`）；artalk/waline/twikoo 仍走原本地存储逻辑，beaudar 等无身份能力的后端仍不渲染该块。
4. 资料落到站点账号：`users` 新增 `website` 列（老库 ALTER 补列），`POST /api/auth/profile` 保存昵称/邮箱/网址，`GET /api/auth/me` 增加 `website`；退出登录复用 `POST /api/auth/logout`。
5. 昵称上限沿用上游表单的 80 字符（页面 maxlength=80），网址只接受 http(s)、缺协议按 https 补全、与输入框 placeholder 一致。
6. 退出登录成功后整页重载：评论区登录态与身份栏同源，需要按新状态重绘。

## 范围

- `internal/store/user.go`、`internal/store/schema.go`：`website` 列、迁移、`UpdateUserProfile`、`CurrentUser.Website`。
- `internal/identity/public.go`：`/api/auth/me` 增加 `website`、新增 `POST /api/auth/profile` 与字段级校验。
- `modules/stellar/api/module.go` + `module_test.go`：公开路由挂载与所有权清单。
- `modules/stellar/web/stellar_template_helpers_settings.go`：原生评论也渲染「个人资料」。
- `modules/stellar/web/assets/js/profile.js`：原生账号按需读/写/退出（异步），第三方后端逻辑不变。
- `modules/stellar/web/assets/js/runtime/extensions/settings.js`：资料块支持 Promise 结果（原生账号异步），其余缓存逻辑不变。
- 测试与本文档；不改生成 CSS、Console 源码与其他页面。

## 验收场景

1. `/settings/` 在未配置第三方评论后端时显示「个人资料」：头像、昵称、Email、URL 三个输入，保存与退出登录。
2. 已登录读者打开页面看到账号现有昵称/邮箱/网址；保存后重读仍是新值，`/api/auth/me` 同步。
3. 未登录打开页面时输入为空，保存返回「请先登录后再保存个人资料」，页面不报错。
4. 邮箱留空、网址留空可保存；非法邮箱/非法网址返回字段级提示；`example.com` 保存为 `https://example.com/`。
5. 退出登录后回到未登录状态（整页重载，评论区同步）。
6. artalk/waline/twikoo 仍渲染该块并继续用本地身份；beaudar 不渲染。
7. 1280 与 390 宽度下与示例站结构一致（头像左、表单右、两枚按钮一行）。

## 本次实测

- `make test` 退出 0（2026-11-11，本工作树）：console 类型/行为测试、Stellar 资源测试、部署/来源/模块检查、`go vet`、Staticcheck、全部 Go 测试通过。
- 新增测试：`internal/identity` 个人资料接口（401/保存/清空/四个字段级校验）、`internal/store` 老库补 `website` 列与 `UpdateUserProfile`、`modules/stellar/web` 设置页个人资料按评论身份渲染、`modules/stellar/tests/profile-icon-contract.test.mjs` 原生账号读写。
- 隔离服务实测（`scripts/e2e/prepare.go` 空站点 + 读者账号，`127.0.0.1:8131`，Chrome，2x）：未登录 → 输入为空；登录 → 回填账号资料；保存 → 状态行「操作成功。」，`/api/auth/me` 与刷新后的输入都是新值；`javascript:alert(1)` → 「请输入 HTTP(S) 网址，或留空」（`data-state=failure`）；退出登录 → 整页重载后 `id=0, verified=false`、输入清空；无 `pageerror`。
- 与参照站几何逐项一致：1280 下 `.settings-profile` 624×364 @x328 y179、头像 96×96、输入框 464×37 @x488 y299、按钮 55×35 / 81×35；390 下 356×376 @x17 y169、输入框 258×38 @x115、按钮 56×36 / 83×36。截图与测量见 `docs/baselines/BLG-20261111-03-settings-profile/`。
- `example.org/me` 这类无协议输入会被浏览器 `input[type=url]` 拦在提交前（与上游一致）；服务端仍会把 API 直连的同类输入补成 `https://`。

## 验证计划

- [x] `make test`（含 assets-test 的 node 行为用例、Go 全量测试）。
- [x] 真实服务（隔离数据库）浏览器验收：登录/未登录两态，1280 与 390 宽度截图，保存往返与退出登录实测。

## 未做（本次范围外）

- 侧栏「设置」挂件对原生账号显示读者头像/昵称：仍只对第三方评论后端生效。
- 评论区登录身份的个人网址沿用评论内输入，不自动取账号 `website`（保持既有「防冒名只覆盖昵称/邮箱」语义）。
- 示例站「关于」多出的「隐私政策」「备用站」两行属于主题配置内容（`profiles.settings.about.items`），不是代码差异。
