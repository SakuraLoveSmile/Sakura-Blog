# BLG-20261111-04 验收基线：设置页「个人资料」访客可保存

参照：<https://xaoxuu.com/settings/>（2026-11-11 实测：写入浏览器本地 `ArtalkUser`，未登录保存显示「操作成功。」，左栏挂件同步昵称与头像）。

验收环境：隔离站点（`scripts/e2e/prepare.go` 空站点 + `e2e-author` 读者账号），`127.0.0.1:8141`，`-dev` 加载本工作树源码，Chrome（Playwright），840×432 @2x。

## 截图

- `anon-saved.png`：未登录，填「上杉九月 / sakurasep@qq.com / https://example.org/me」保存后。「操作成功。」，头像变成 gravatar，左栏挂件显示「上杉九月」+ 头像。
- `anon-after-logout.png`：未登录点「退出登录」后，三个输入清空、挂件回到「设置」。
- `reader-saved.png`：`e2e-author` 登录态保存后，仍写站点账号。

## 实测结果

| 场景 | 结果 |
| --- | --- |
| 未登录保存 | 「操作成功。」，写 `cmt_nick`/`cmt_email`/`cmt_link`，不请求 `/api/auth/profile` |
| 刷新回填 | 三个输入保持「上杉九月 / sakurasep@qq.com / https://example.org/me」 |
| 非法网址 | 「请输入 HTTP(S) URL，或留空。」，`data-state=failure`，值保留 |
| 无协议网址 | `example.com` 保存为 `https://example.com/` |
| 未登录退出 | 本地键全删，输入清空，挂件回到「设置」，不弹错误提示 |
| 登录读者保存 | 返回「操作成功。」，`/api/auth/me` 变为 `{nickname:"九月",email:"september@example.com",website:"https://example.net/"}`，本地 `cmt_*` 键未被写入 |
| 登录读者退出 | 整页重载后 `id=0, verified=false`，挂件回到「设置」 |
| 页面错误 | 无 `pageerror` |

## 与参照站对照

- 身份模型一致：访客写浏览器本地资料，登录读者用站点账号（参照站用的是 artalk 本地身份，语义等价）。
- 几何不变：资料区、头像 96px、输入框、按钮尺寸与 BLG-20261111-03 记录的数值相同。
- 左栏挂件行为一致：保存后显示昵称与头像，退出后回到「设置」。
