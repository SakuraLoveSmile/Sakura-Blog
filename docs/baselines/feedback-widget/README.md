# feedback-widget 接入验收基线

接入对象：自建 `Feedback` 服务（SakuraLoveSmile/Sakura-Feedback）的 `@feedback/web` 组件，
`<feedback-widget>` Custom Element（Shadow DOM，orb 悬浮入口 + 视口截图 + 文字/日志反馈）。

## 制品来源（固定为 GitHub Release，不读取/构建 Feedback 工作树）

| 项 | 值 |
| --- | --- |
| Tag / Release | `v0.5.2` · https://github.com/SakuraLoveSmile/Sakura-Feedback/releases/tag/v0.5.2 |
| 包制品 `feedback-web-0.5.2.tgz` | sha256 `f002372b652c766cbd6bea9e537072c2ab72bf575eb4d32b5c36de0cf00244e0` |
| vendor ESM 入口 `assets/vendor/feedback/feedback-web.js` | sha256 `1f42ae80e5ec9b00cea5fd9bacdb7e58d5face2bb62de458a0d1b79b2b74d787` |
| vendor 截图分块 `assets/vendor/feedback/html2canvas-pro.esm-CQ8baKsv.js` | sha256 `10a81b359dd16007e3144d4e81da15db3784c65fe7bf6314ba0c17e221101776` |

vendor 两个文件逐字节等于 tgz 内 `package/dist/` 同名文件，无任何本地补丁；组件内部以
`import("./html2canvas-pro.esm-CQ8baKsv.js")` 相对路径懒加载截图分块，同目录部署即可。
截图分块文件名自带内容哈希；ESM 入口经 `asset_ver` 注入 `?v=1f42ae80` 版本戳。

## 验收环境

Blog worktree 隔离 root + 独立 SQLite（`.tmp/feedback-accept/`），独立端口 `:8097`，`-dev` 启动；
Feedback 服务为生产实例 `https://feedback.xn--fhqths51enha.cn`（CORS 预检对
`Origin: http://127.0.0.1:8097` 返回 `access-control-allow-origin` 同源回显，跨域可用）。

## 验收项与证据（v0.5.2，桌面 1440×900 + 移动 390×844）

| 项 | 结果 | 证据 |
| --- | --- | --- |
| 属性透传（api-base/app-id/app-name/app-version/theme=dark/orb/viewport/right/25%） | ✅ | DOM 断言，`01-orb.png` |
| ESM 入口带 `?v=1f42ae80`（与 vendor 摘要前缀一致） | ✅ | HTML 断言 |
| orb 右侧悬浮可点击；桌面 x≈1372，移动端视口内 | ✅ | `01-orb.png`、`08-mobile-orb.png` |
| 点击 orb 打开深色面板（`rgb(28,28,30)`），移动端面板占满视口 | ✅ | `02-panel-open.png`、`09-mobile-panel.png` |
| 视口截图：懒加载 `html2canvas-pro.esm-CQ8baKsv.js` 无 404，缩略图有效 | ✅ | `02-panel-open.png` |
| 敏感区遮挡：`input[type=password]` 与 `data-feedback-capture-mask` 注入亮绿块，缩略图对应位置被灰块遮盖 | ✅ | `05-capture-mask.png` |
| 重新截图（retake）后缩略图再生成 | ✅ | `05-capture-mask.png` |
| 截图失败语义：abort chunk 请求后面板提示可重试、文字反馈仍可用 | ✅ | `06-capture-fail.png`（SW block 后拦截才生效） |
| 登录流程：提交展开登录面板，错误凭据经生产 `/api/auth/login` 返回 401 并渲染「用户名或密码错误」 | ✅ | `03-login-form.png`、`04-login-result.png` |
| 关闭面板重开后文字草稿保留 | ✅ | DOM 断言 |
| Console「内容增强 → 反馈组件」表单回填生产值并经既有 settings 管线保存 | ✅ | `07-console-settings.png` |
| `enabled=false` 或缺 `app_id`：不渲染元素也不加载 ESM；`api_base` 非 http(s) 被 400 拒绝 | ✅ | API + HTML 断言 |

合计 21 项断言全部通过。真实提交（登录有效账号后落库）不在本基线内，归 v1.7.0 生产验收。

## 已知边界

- `main.css` 中 `.logo-wrap a.avatar img.avatar` 的 `border-radius: calc(100% - 2 * 2px)` 会触发
  html2canvas-pro 2.4.2 的 calc 解析崩溃；`head.html` 在组件启用时输出等效 `border-radius: 50%` 覆盖。
  同类上游 calc 写法若再次出现需同样处理；长期修复应在 Feedback 仓库升级 html2canvas-pro 或加固截图解析。
- `page-label` 取 `Page.Path`（canonical path）：`/links` 输出 `page-label="/links"`；
  首页等未填充该字段的页面不输出此属性。
- 生产 Feedback 对登录做来源校验；本地验收证明 401 往返与错误渲染，真实提交以生产验收为准。

## 复现

```bash
# 验收 root（gitignored）：.tmp/feedback-accept/{config/site.yml,data/blog.db}
BLOG_LISTEN=:8097 go run ./cmd/blog -root .tmp/feedback-accept -dev
# 驱动脚本：.tmp/feedback-accept/accept-v052.mjs（pnpm exec node 运行）
```
