# Spec 26 · 评论邮件通知（已实现 → Console 配置化 + 测试补强）

- **分支**：`feature/comment-notify` · **Wave 6** · **隔离等级**：低
- **主模块**：`internal/comment/notify.go`（**已存在**）+ Console settings 新 SMTP tab（本期增量）
- **状态**：⚠️ **核心功能已实现并接线**。本 spec 先记录现状，再界定剩余增量，避免重复造轮子。

## 现状（已实现，审计确认）

经代码审计，评论邮件通知**已完整落地**，无需新建 `internal/notify/` 包：

- **`internal/comment/notify.go`**（117 行）：
  - `notify(cm, parent)`：新评论（含待审）→ 通知博主（`admin_email`，博主自发除外）；已发布回复 → 通知被回复者（有邮箱、非自回、非博主）。
  - `send()`：465 端口走隐式 TLS（`sendImplicitTLS`，手动 `tls.Dial`），其余端口走 `smtp.SendMail`（STARTTLS）；Subject 用 RFC 2047 `mime.QEncoding` 编码防中文乱码。
  - `NotifyReply()`：管理后台回复触发（`api` 回复路径调用），规则同 `notify`。
  - 异步 goroutine + `recover` 兜底，发送失败仅记日志，**不阻塞评论提交**。
- **配置**：`template/model.go` 的 `CommentsConfig.Notify NotifyConfig`（`smtp_host/smtp_port/username/password/from`）+ `AdminEmail`；`site.yml` 已有 `comments.notify` 段（`smtp_host:''` 默认关闭）。
- **接线**：`main.go:186` `comment.New(st, &site.Comments, site.Title)`；`smtp_host` 空则整体跳过。

## 剩余增量（本期真正要做的）

核心既已完成，本期只做**配置可视化 + 质量补强**两件小事：

1. **Console SMTP 配置 tab**：设置页新增「邮件通知」tab，可视化编辑 `admin_email` + `notify.{smtp_host,smtp_port,username,password,from}` +  moderaton 开关，复用 `settingsApi.updateSection`。免去手改 site.yml。
   - **前置确认**：settings 的 section 写回机制是否覆盖 `comments` 段（`api/settings.go` 的 section 白名单）——**动手前先读，若 comments 段不可写需补**。
   - password 字段在 GET 时脱敏（返回 `****` 或空），仅 PUT 非空时更新。
2. **测试覆盖**：`notify.go` 目前无单测。补 `notify_test.go`：
   - 规则矩阵：博主自发不通知博主 / 待审评论通知博主 / 已发布回复通知被回复者 / 自回不通知 / 回复博主不重复通知。
   - SMTP 未配置（`smtp_host==''`）整体跳过。
   - 用 `httptest`/假 `send` 注入验证收件人与主题（重构 `send` 为可注入的函数变量或接口 seam）。
3. **发信自检按钮（可选）**：Console 配置页「发送测试邮件」按钮 → 新端点 `POST /api/v1/settings/notify/test`，向 `admin_email` 发一封测试信，即时验证 SMTP 配置正确性。

## What Changes

- `console/src/modules/settings/SettingsView.vue`：+「邮件通知」tab（admin_email + SMTP 5 字段 + moderation + 测试按钮）。
- `internal/api/settings.go`：确认/补 `comments` section 可写；password 脱敏；（可选）`notify/test` 端点。
- `internal/comment/notify.go`：重构出可注入的发信 seam（`var sendMail = ...`）以便单测，**不改业务逻辑**。
- `internal/comment/notify_test.go`（新）：规则矩阵 + 跳过逻辑 + 收件人断言。

## 实现步骤

1. 审计 `api/settings.go` section 写回白名单，确认 comments 段可写（不可写则补）。
2. notify.go 抽出可注入 send seam（保持行为不变）。
3. notify_test.go 规则矩阵单测。
4. Console 邮件通知 tab + password 脱敏 + （可选）测试发信端点。
5. 回归：配真 SMTP 发测试信收到；评论/回复触发对应通知；smtp_host 空时静默。

## 技术决策（无需再问）

- **不新建 internal/notify 包**：逻辑已在 comment 包内聚（通知是评论的子行为），拆包反而割裂；原计划「新 notify 包」基于未实现的假设，审计后作废。
- **配置入 site.yml comments 段（已有）而非 app.yml**：通知是站点内容配置，与评论系统同段，语义一致。
- **password 脱敏**：GET 不回传明文，PUT 空值不覆盖，避免前端往返泄漏/误清。
- **不做**：邮件模板 HTML 化、多收件人/订阅、评论点赞通知、发送队列/重试（失败仅记日志，单人博客可接受）。

## Impact

- **新增**：`internal/comment/notify_test.go`、Console 邮件通知 tab、（可选）notify/test 端点
- **修改**：`SettingsView.vue`、`api/settings.go`（脱敏/section 白名单）、`notify.go`（抽 seam，逻辑不变）
- **不影响**：通知业务规则、前台评论区、其余 spec

## ADDED Requirements

### Requirement: 评论邮件通知配置化与测试
The system SHALL expose comment email-notification settings in the Console and provide test coverage for the existing notification rules.

#### Scenario: Console 配置 SMTP
- **WHEN** 在设置页填写 SMTP 并保存
- **THEN** 写入 site.yml comments.notify，password 脱敏回显、空值不覆盖

#### Scenario: 通知规则（既有，补测）
- **WHEN** 访客评论 / 回复他人 / 自回 / 回复博主
- **THEN** 按「博主收新评论、被回复者收回复、自回与回博主不重复」规则触发

#### Scenario: 未配置静默
- **WHEN** smtp_host 为空
- **THEN** 所有通知整体跳过，评论提交不受影响

#### Scenario: 测试发信（可选）
- **WHEN** 点击「发送测试邮件」
- **THEN** 向 admin_email 发送测试信并反馈结果

## 验证

1. `go build ./... && go vet ./... && go test ./internal/comment/` 通过（含新 notify_test）
2. Console 邮件通知 tab 保存生效，password 脱敏；测试发信收到
3. 真 SMTP 下：新评论/回复触发对应邮件；smtp_host 空时静默
4. 通知规则矩阵单测全绿

## Out of Scope

- 邮件 HTML 模板、多收件人/订阅机制、点赞通知
- 发送队列/失败重试/投递状态追踪
- 新建独立 notify 包（审计后确认不需要）
