# 阶段 A 实施与验证报告

日期：2026-09-08。工作树：`../Blog-phase-a-implementation`，分支：`codex/phase-a-implementation`，起点：`2cc0371`。

## 交付范围

开始时 `dev` 已含 `fef56cd`（第一阶段整改）。本次以实际源码为准核验 A1–A4，补齐剩余缺陷和回归；没有重复重写已正确实现的列表、详情及修订归属逻辑。

- A1：统一 canonical/OG/sitemap/feed 地址，修复 legacy RSS 协议、originalHost 分歧、可信代理协议和特殊路径编码；新配置校验、旧配置兼容。
- A2：统一现有 Console/UC 附件操作规则，引用查询保护文章归属，批量移动事务化；普通作者保留共同署名且不能伪造替换作者。
- A3：封禁轮换 Cookie 密钥、清除 API Token 的既有原子 SQL 保持；新增 2FA challenge 凭据版本校验、封禁范围回归和前端 401 清理。
- A4：图片 download 空值/true/false/URL 的 Markdown 与预览验证；真实布尔开关的缺省、不注入默认值、启用与关闭语义验证。

## 实际验证

命令均在交付工作树运行，数据写入使用隔离测试数据库。

| 命令 | 实际结果 |
|---|---|
| `go test -count=1 ./internal/settings ./internal/config ./internal/store ./internal/identity ./modules/stellar/routes ./modules/console/api` | PASS；整合过程中运行，后续变更由最终门禁和对应回归再验证 |
| `make test` | PASS；Console 170/170、Stellar 74/74；部署/发布来源/模块边界检查、Go vet、Staticcheck、全部 Go 测试通过 |
| `go test -race ./internal/store ./internal/identity ./internal/config ./modules/console/api ./modules/stellar/routes` | PASS；后续署名保存微调另跑下列 targeted race |
| `go test -race ./modules/console/api -run 'TestAuthorWritesPreserveNumericUsernameAndCoauthors|TestBearerAttachmentOwnership' -count=1` | PASS |
| `pnpm run e2e` | PASS，27/27；最终运行 36.6 秒；真实重建 SPA 并重启独立服务，禁用复用旧服务 |
| `pnpm run lint` | PASS，全仓前端 ESLint |
| `pnpm exec prettier --check`（本次 7 个 TS/E2E 文件） | PASS |
| `pnpm run format:check` | FAIL：49 个未涉及文件的既有格式差异；与本次修改文件交集为 0，未批量格式化 |
| `make build` | PASS；Console、UC 真实产物与 Go 二进制构建成功 |
| `git diff --check` | PASS |
| 独立源码复核 | 最后一轮无新增实质问题；发现的特殊 slug 和数字用户名 ID 歧义均已修复并补测试 |

E2E 具体证据：Console 390/768/1440 三视口收到 401 后用键盘返回登录；UC 401 提示及清空身份；真实封禁读者后升级请求不能继续；特殊字符文章 URL 的 canonical/OG/sitemap 一致且同一编码路径实际 HTTP 200。该验收针对阶段 A 行为，不代表阶段 D 全站视觉、移动菜单及可访问性验收。

新增失败场景测试：数据库触发器拒绝封禁 UPDATE 时状态/密钥/Token 都不改变；批量移动 UPDATE 触发失败时整批回滚；缺失附件、越权、无效目标分组整批拒绝；封禁后提权及解封不复活旧 Cookie、Token 或有效 2FA challenge；数字用户名与其他用户 ID 碰撞时署名不漂移。

## 逐文件变更说明

以下路径均相对本工作树。

| 文件 | 修改原因 |
|---|---|
| `internal/settings/url.go` | 按路径边界规范化 URL，兼容旧 query/fragment；新地址拒绝 userinfo 和非站点基址。 |
| `internal/settings/url_test.go` | 为相邻实现补充或同步阶段 A 针对性回归测试。 |
| `internal/store/attachment.go` | 目标分组、全批对象授权与移动在同一事务；失败返回可区分错误。 |
| `internal/store/attachment_authorization_test.go` | 为相邻实现补充或同步阶段 A 针对性回归测试。 |
| `internal/store/user_test.go` | 为相邻实现补充或同步阶段 A 针对性回归测试。 |
| `modules/console/api/2fa_test.go` | 为相邻实现补充或同步阶段 A 针对性回归测试。 |
| `modules/console/api/attachment.go` | 附件引用按文章归属过滤，修正正文字段并传播查询错误；调用事务移动。 |
| `modules/console/api/attachment_bearer_ownership_test.go` | 为相邻实现补充或同步阶段 A 针对性回归测试。 |
| `modules/console/api/auth.go` | 2FA 临时挑战绑定签发时 auth_secret，凭据轮换后不可复活。 |
| `modules/console/api/auth_enhance_test.go` | 为相邻实现补充或同步阶段 A 针对性回归测试。 |
| `modules/console/api/author_ownership_test.go` | 为相邻实现补充或同步阶段 A 针对性回归测试。 |
| `modules/console/api/ban_invalidation_test.go` | 为相邻实现补充或同步阶段 A 针对性回归测试。 |
| `modules/console/api/coauthor_preservation_test.go` | 为相邻实现补充或同步阶段 A 针对性回归测试。 |
| `modules/console/api/comment_users_test.go` | 为相邻实现补充或同步阶段 A 针对性回归测试。 |
| `modules/console/api/module.go` | SEO 推送复用 URL helper，并按路径段编码 slug。 |
| `modules/console/api/post.go` | 普通作者保留真实作者 ID；管理员未修改署名时也保留稳定 ID。 |
| `modules/console/api/router.go` | 同步 2FA 随机数测试注入函数签名，不改公开路由。 |
| `modules/console/api/uc.go` | 复用附件权限检查；编辑器合并请求不再回填有歧义的作者名。 |
| `modules/console/api/uc_test.go` | 为相邻实现补充或同步阶段 A 针对性回归测试。 |
| `modules/console/api/users.go` | 移除一般用户编辑接口新增的封禁能力，恢复仅评论读者入口管理状态的范围。 |
| `modules/console/api/users_test.go` | 为相邻实现补充或同步阶段 A 针对性回归测试。 |
| `modules/console/web/src/main.ts` | 401 提示前清空登录身份、权限和查询缓存，避免守卫跳回后台。 |
| `modules/console/web/src/stores/auth.ts` | 提供明确的会话清理动作，清除身份和权限状态。 |
| `modules/console/web/src/uc/main.ts` | UC 401 清空两个共享身份 store 与缓存，提示重新登录并跳转。 |
| `modules/console/web/src/uc/stores/auth.ts` | 清空 UC 会话；fetch 请求收到 401 时同样清理状态。 |
| `modules/stellar/routes/content_cache.go` | sitemap 按段编码文章、Wiki 项目和文档路径。 |
| `modules/stellar/routes/feed.go` | Atom、JSON Feed 和其他订阅复用 canonical 基址及路径拼接。 |
| `modules/stellar/routes/middleware.go` | 只接受可信直接代理的协议头，向请求上下文注入确定协议。 |
| `modules/stellar/routes/middleware_test.go` | 为相邻实现补充或同步阶段 A 针对性回归测试。 |
| `modules/stellar/routes/routes_feed.go` | RSS、sitemap 与 canonical 的 originalHost 规则一致；编码文章路径。 |
| `modules/stellar/routes/routes_feed_test.go` | 为相邻实现补充或同步阶段 A 针对性回归测试。 |
| `modules/stellar/routes/web.go` | canonical 使用 EscapedPath，统一基址策略和可信协议读取。 |
| `modules/stellar/routes/web_test.go` | 为相邻实现补充或同步阶段 A 针对性回归测试。 |
| `tests/e2e/canonical-url.spec.ts` | 真实公开页面 canonical/OG/sitemap 与特殊 URL 可访问性验收。 |
| `tests/e2e/session-invalidation.spec.ts` | 三视口键盘登录返回、UC 会话清理及真实读者封禁验收。 |
| `tests/e2e/shortcode-visual-editor.spec.ts` | 补齐 download 四值与布尔开关回写、预览契约。 |

## 兼容性与未验证范围

- 保留 API 路径和响应主体；普通作者不能再通过共同署名请求替换现有作者，历史 owner_id=0 附件仅管理员操作。
- 不新增数据库 schema、迁移、依赖或持久化格式。2FA challenge 为现有进程内临时状态。
- 新保存站点根地址拒绝 userinfo/query/fragment；旧已存配置继续读取并兼容拼接。不改生产菜单、文章或公开附件 URL。
- 保留 originalHost 的既有 HTTPS 语义；只有 `trusted_proxy` 中的直接代理能影响未配置基址时的协议。
- UC 继续保留现有简化接口，不新增无调用方的 refs/move 路由。页面、Wiki、评论和瞬间的权限模型不在 A2；引用中的页面和瞬间仍沿用既有范围。
- 本次未运行生产写入、部署或 Linux/systemd 更新/恢复演练；后者属于 B/C 阶段。不能据此声称生产 canonical 已更新。
- 未提交、未合入 dev、未推送。`make merge-check` 会执行集成，因此本次未调用；授权集成时应按仓库要求执行。
- 根 dev 原有 `.gitignore` 改动、既有 `Blog-phase-a-security` 工作树及共享主题的两处修改和 `.mimosa/` 均保留。

## 回退与后续集成

本次仅在独立工作树中留下未提交变更，dev 与运行中的服务未被替换，无需生产回退。需要放弃本次结果时可保留该工作树作为审阅记录；不要重置 dev 或共享 resource。后续授权集成应先核对最新 dev 与共享状态，提交中文说明并运行 `make merge-check` 及合并后的组合态门禁。生产部署和回退继续走独立授权流程。
