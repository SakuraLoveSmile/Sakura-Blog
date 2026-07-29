# Spec 45 · 用户账户体系（用户名密码 + 个人资料 + 2FA，大工程）

- **分支**：`feature/users-account` · **Wave 9**（涉及鉴权主链路，单独批次串行）· **隔离等级**：低（动 auth 全链路）
- **主模块**：新 `internal/store/user.go` + `internal/api/auth.go`（重构）+ `console/src/stores/auth.ts` + `Login.vue` + 新「个人资料」页
- **前置**：无文件级前置，但**建议 Wave 8 全部合完后单独进行**（鉴权改动影响所有 Console 模块的会话）

## Why

Halo 对照最大的系统性差距：本项目是「单管理员纯密码」（无用户名、无个人资料、无 2FA）。完整 RBAC 多用户对单人博客过度设计，本 spec 取**务实中间态**：正式用户模型（单主账户起步）+ 用户名/密码登录 + 个人资料 + TOTP 两步验证，表结构为未来多用户预留。**多用户 + 角色权限（RBAC）明确暂缓**（见技术决策）。

## 功能需求

1. **用户模型**：users 表（id/username/password_hash/display_name/email/avatar/totp_secret/created_at）；首次启动迁移：把现有 app.yml 管理密码迁为 admin 用户（用户名 admin，密码沿用），迁移后 app.yml 密码字段废弃（保留读取兼容 + 启动日志提示）。
2. **登录升级**：Login.vue 用户名 + 密码双字段；限流与 HMAC cookie 机制保留（cookie 载荷 +user id）。
3. **个人资料页**：Console 用户区下拉 +「个人资料」——display_name/email/avatar（附件选择器）编辑；侧栏底部用户区显示真实 display_name 与头像（替换静态「管理员」标签）。
4. **改密**：settings 安全 Tab 迁移到个人资料页（旧位置留跳转链接）。
5. **2FA（TOTP）**：个人资料页开启/关闭；开启流程：生成 secret → 二维码（otpauth://）→ 输入一次验证码确认；登录时若开启则第二步输 TOTP 码；恢复码 8 个一次性生成提示保存。

## What Changes

- `internal/store/user.go`（新）：users 表 + CRUD + 密码 hash（bcrypt——需新增 `golang.org/x/crypto` 依赖）+ TOTP secret/恢复码存取 + `_test.go`。
- `internal/api/auth.go`：登录改用户名查库 + bcrypt 校验 + TOTP 第二步；`/api/v1/profile` 读写端点；改密端点迁移。
- TOTP 实现：**不引第三方库**，RFC 6238 标准实现放 `internal/api/totp.go`（HMAC-SHA1 + 30s 窗口 ±1，代码 <100 行）+ 单测（RFC 测试向量）。
- `console/src/`：Login.vue 双字段 + 2FA 第二步；+`modules/profile/`；DefaultLayout 用户区真实资料；stores/auth.ts 用户对象。
- `config/app.go`：admin 密码字段标记 deprecated（迁移期兼容读）。

## 实现步骤

1. store/user.go + bcrypt + 迁移逻辑（app.yml 密码 → admin 用户）+ 单测。
2. 登录链路改造（用户名+密码，2FA 关闭态全通）+ api 单测 + Login.vue。
3. 个人资料页 + 用户区展示。
4. TOTP 实现 + 单测（RFC 向量）→ 开启流程 UI → 登录第二步 → 恢复码。
5. 全模块回归（鉴权中间件行为不变，仅登录入口变化）。

## 技术决策（无需再问）

- **多用户/RBAC 暂缓**：表结构支持多行，但本期不做用户管理界面/角色/邀请；多作者展示层已由 Spec 38 解决。需求真实出现（多人共写）时另立 spec 做 RBAC。此为差距清单「用户与角色权限」项的正式处置。
- **bcrypt 而非 argon2**：x/crypto/bcrypt 成熟且依赖面小；cost=10。
- **TOTP 自实现**：标准算法百行内，免引库；恢复码 hash 存储。
- **会话不做服务端存储**：沿用 HMAC 签名 cookie 无状态方案（改密/关 2FA 时轮换签名密钥使旧 cookie 失效）。
- **通知中心不随本 spec**：Halo 站内通知依赖用户体系，但价值待定，见 README「暂缓项」。

## Impact

- **新增**：store/user.go、api/totp.go、profile 模块、x/crypto 依赖
- **修改**：api/auth.go、Login.vue、auth.ts、DefaultLayout、settings 安全 Tab、config/app.go
- **不影响**：前台全部、内容管理模块业务逻辑

## 验证

- [ ] 旧库启动自动迁移出 admin 用户，旧密码可登录；app.yml 密码废弃提示
- [ ] 用户名+密码登录、错误限流、改密后旧会话失效
- [ ] 个人资料编辑后侧栏用户区即时更新
- [ ] 2FA 全流程：开启（扫码+确认）→ 登录两步 → 恢复码可用一次 → 关闭
- [ ] TOTP 单测过 RFC 6238 测试向量
- [ ] `make test` 全绿
