# BLG-20261111-03 验收基线：站点设置页「个人资料」

参照：<https://xaoxuu.com/settings/>（Stellar 2.0.0 示例站，2026-11-11 抓取）。`ref-lg.png` 为同视口参照截图。

验收环境：隔离站点（`scripts/e2e/prepare.go` 播种的空站点 + 一个读者账号，`127.0.0.1:8131`，`-dev` 加载本次工作树源码），Chrome（Playwright `channel: chrome`），2x DPR。

- `logged-lg.png`：1280 宽度、读者已登录。
- `logged-sm.png`：390 宽度、读者已登录。
- `anon-lg.png`：1280 宽度、未登录。
- `measurements.json`：同一脚本采集的结构、输入值、头像、按钮几何、保存/退出实测结果与错误清单。

## 与参照站的几何对照（1280 / 390）

| 元素 | 本站 | 参照站 |
| --- | --- | --- |
| `.settings-profile`（1280） | x328 y179 624×364 | x328 y179 624×364 |
| 头像（1280） | 96×96 | 96×96 |
| 首个输入框（1280） | x488 y299 464×37 | x488 y299 464×37 |
| 保存 / 退出登录（1280） | 55×35 @x488 / 81×35 @x553 | 55×35 @x488 / 81×35 @x553 |
| 分区顺序 | 个人资料 → 缓存 → 关于 | 个人资料 → 缓存 → 关于 |
| `.settings-profile`（390） | x17 y169 356×376 | x17 y169 356×376 |
| 首个输入框（390） | x115 258×38 | x115 258×38 |
| 保存 / 退出登录（390） | 56×36 @x115 / 83×36 @x182 | 56×36 @x115 / 83×36 @x182 |

## 结论

- 未登录：三个输入为空，保存返回服务端提示「请先登录后再保存个人资料」（本次实测的是登录态保存与非法网址分支；未登录保存由 `internal/identity` 单元测试覆盖 401）。
- 登录态：回填账号昵称/邮箱/网址；保存后状态行显示「操作成功。」，`GET /api/auth/me` 与刷新后的输入值都是新值（`september+new@example.com` / `https://example.org/me`）。
- 非法网址（`javascript:alert(1)`）返回字段级提示「请输入 HTTP(S) 网址，或留空」，状态行 `data-state=failure`，值保留供修正。
- 退出登录：整页重载后 `/api/auth/me` 为 `id=0, verified=false`，三个输入清空。
- 页面无 `pageerror`；控制台 404 只有隔离站点缺失的 `/about/`、`/friends` 导航与 gravatar 无网络时的头像回退，与本次改动无关。

## 生产验证（v2.1.7，2026-11-11）

线上 `/settings/` 已带「个人资料」：`data-provider="native"`，分区为 个人资料 → 缓存 → 关于，资料区几何 624×364 @x328 y179 与本地验收一致；「关于」的版本行显示 `Blog v2.1.7`。

- `production-lg.png`、`production-sm.png`：生产 1280 / 390 截图。
- `production-anon-save.png`：未登录点保存后的服务端提示「请先登录后再保存个人资料」（`data-state=failure`），没有写入任何数据。
- `production-verification.json`：容器状态、`/readyz`、公开接口状态码、`/api/auth/me` 形状与页面错误清单。

未在生产登录读者账号，因此登录态保存往返只有本地隔离站点证据；线上只验证了公开页面与未登录分支。
