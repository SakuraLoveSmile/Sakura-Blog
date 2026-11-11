# BLG-20261111-04：设置页「个人资料」对齐上游，访客也能保存

- 状态：待体验（已发布 v2.1.8，待生产体验确认）
- 分支：`codex/feat/20261111-BLG-20261111-04-settings-profile-local-identity`
- 工作树：`/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261111-feat-BLG-20261111-04-settings-profile-local-identity`
- 基点：`dev` @ `143d2e93`
- 合入目标：`dev`
- 视觉/行为基准：<https://xaoxuu.com/settings/>（Stellar 2.0.0 示例站，2026-11-11 实测）

## 背景

BLG-20261111-03 把「个人资料」接到站点账号：读 `GET /api/auth/me`、写 `POST /api/auth/profile`。几何与示例站一致（头像 96px、输入框、按钮位置逐项相同），但身份语义不同：

- 示例站把资料写进浏览器本地评论身份（实测写 `localStorage.ArtalkUser`），未登录也能「操作成功。」，左栏「设置」挂件随即变成读者昵称 + 头像。
- 本站未登录时保存返回 401「请先登录后再保存个人资料」（截图中的红字），左栏挂件永远是「设置」。
- 更关键：前台登录接口拒绝管理员账号（`internal/identity/public.go` 的「管理员账号请从后台登录」），Console 的 `blog_admin` cookie 也不被 `/api/auth/me` 认，因此**站主在前台永远存不了这份资料**。

用户在 2026-11-11 明确选择方案 A：照参考站的行为改，访客也能存。

## 已确认决定

1. 采用方案 A：访客走浏览器本地身份，已登录读者仍写站点账号。
2. 本地身份与评论区匿名身份**共用同一份键**（`cmt_nick`/`cmt_email`/`cmt_link`），设置页与评论区读写同一份资料，不新增第二套本地存储。
3. 未登录时「退出登录」= 清空本地匿名资料，与上游一致；不显示「操作成功」以外的会话提示。
4. 左栏「设置」挂件跟随同一身份模型：原生评论（账号或本地资料）显示读者昵称/头像，无身份能力的第三方后端（beaudar/utterances/giscus）仍只显示「设置」。

## 范围

- `modules/stellar/web/assets/js/profile.js`：原生身份`账号优先 + 本地兜底`；本地资料纯文本读写（与评论区共用键）；写侧按 https 补全无协议网址；挂件渲染改为异步并防竞态。
- `modules/stellar/web/assets/js/runtime/extensions/settings.js`：本地身份的保存/退出提示语义。
- `modules/stellar/web/stellar_template_helpers_regions.go`：原生 provider 也启用挂件身份。
- 测试：`modules/stellar/tests/profile-icon-contract.test.mjs`（本地身份读写/校验/退出）、`modules/stellar/web/stellar_settings_host_test.go`（挂件身份按 provider 开关）。

## 验收场景

1. 未登录打开 `/settings/`，填入昵称/邮箱/网址，保存显示「操作成功。」，页面不报错。
2. 刷新后三个输入仍回填刚才的本地资料；左栏「设置」挂件显示该昵称（有邮箱时显示 gravatar 头像）。
3. 已登录读者保存时仍写账号接口，刷新与 `/api/auth/me` 同步。
4. 非法网址（`javascript:`）与非法邮箱返回字段级提示，值保留供修正。
5. 未登录点「退出登录」清空本地资料，挂件回到「设置」。
6. `example.com` 这类无协议输入保存为 `https://example.com/`。
7. beaudar 等无身份能力后端仍不渲染资料块、挂件不显示身份。

## 本次实测

- `make test` 退出 0（2026-11-11，本工作树）：console/Stellar 资源/部署与模块检查、`go vet`、Staticcheck、全部 Go 测试通过。
- 新增/更新测试：`modules/stellar/tests/profile-icon-contract.test.mjs`（未登录读本地资料、保存不请求账号接口、字段级校验、退出清空；已登录写账号接口）、`modules/stellar/web/stellar_settings_host_test.go`（挂件身份按 provider 开关）。
- 隔离服务实测（`127.0.0.1:8141`，Chrome，840×432@2x）：未登录保存 → 「操作成功。」+ 写 `cmt_*` + 头像变 gravatar + 挂件显示「上杉九月」；刷新回填；`javascript:alert(1)` → 「请输入 HTTP(S) URL，或留空。」；`example.com` → `https://example.com/`；未登录退出 → 本地键清空、挂件回「设置」；登录读者保存 → `/api/auth/me` 同步且本地键未被污染；登录读者退出 → `id=0`；无 `pageerror`。
- 证据见 `docs/baselines/BLG-20261111-04-settings-profile-local-identity/`。
- **发布阶段补测（本地全量 e2e）**：`pnpm exec playwright test` 跑出 7 个 `writing-assistant.spec.ts` 失败，全部是 `/api/auth/me` 在 Console 标签预览的不透明源 iframe 里被 CORS 拒绝。根因是挂件身份开启后每次重渲染都打会话接口。修复（`41fe9732`）：会话读取整页只请求一次；不透明源直接跳过网络。修复后 27 项用例全通过，并补了「只请求一次」「不透明源不发请求」两条回归用例。

## 合入与发布

- `make merge-check` 通过，合入 `dev`（`2ddd5638`）。
- PR #79 `dev` → `main` 合并（`afccb749`），CI 全绿（build/console/e2e/go/lint/security）。
- PR #80 `main` → `release` 合并（`9e573071`），含 `image` 预构建与 `release-source` 来源校验。
- 打 `v2.1.8` tag，`docker-release.yml` 成功；GHCR 已有 `2.1.8`/`2.1`/`2`/`latest`（manifest `sha256:26d0398…`），GitHub Release 已创建。
- 未做：服务器 `docker compose pull && docker compose up -d`（需在宿主执行）。

## 待办

- [x] `make test` 通过
- [x] 隔离服务浏览器验收（未登录保存往返、登录态、退出、非法输入）
- [x] `make merge-check` 合入 dev
- [x] 发布 v2.1.8（镜像已产出并校验）
- [ ] 服务器升级到 v2.1.8
- [ ] 生产环境用读者身份实测保存
