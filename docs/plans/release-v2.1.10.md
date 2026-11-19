# Release v2.1.10：编辑器直插外部 HTML 并直接渲染

- 状态：待体验（代码已发布并完成生产升级，等待线上体验确认）
- 日期：2026-11-19（Asia/Taipei）
- tag：`v2.1.10` → `5a1c28b757863ce629f95a53c908d8acb894c9d2`（`release` 的 PR 合并提交）
- 仓库：`SakuraLoveSmile/Sakura-Blog`

## 发布内容

| 类型 | 内容 | 任务 |
| --- | --- | --- |
| feat | 编辑器支持直插外部 HTML 并在富文本中直接渲染：工具栏「插入 HTML」+ 源码弹窗、富文本可视化 atom 节点、外部网页粘贴保真、Markdown 往返无损 | BLG-20261117-02 |

详细实现、已知边界与验收清单见 `docs/plans/BLG-20261117-02-raw-html-insert.md`。

## 流程与证据

1. **推送 `dev`**：`d522a2c2..e273495e`。
2. **PR #85 `dev` → `main`**：run `44487173806` 全部通过（go 3m1s、lint 2m3s、security 3m29s、console 1m21s、build 1m31s、e2e 25m58s），合并提交 `cd8fc284`。
3. **PR #86 `main` → `release`**：run `44491051888` 全部通过，其中 `release-source` pass、`image` pass（1m40s，不推送）、e2e pass（27m13s），合并提交 `5a1c28b7`。
4. **tag `v2.1.10`**：GPG 签名的 annotated tag 推到 `5a1c28b7`；`Docker Release` run `44495333690` success。
5. **GitHub Release `v2.1.10`**：工作流自动创建，无资产、非 prerelease（`2026-11-19T09:24:53Z`）。
6. **GHCR**：`2.1.10` / `2.1` / `2` / `latest` 指向同一 manifest `sha256:fc7c2d006ee516af3adfce68d6a05ef063ef432ca4fd81e1376af5c54d613fd0`。
7. **镜像冒烟**：本机未装 Docker，改在服务器执行 `docker run --rm ghcr.io/sakuralovesmile/sakura-blog:2.1.10 -h`，退出码 0（打印 usage）。

## 生产升级

- compose 备份：`docker-compose.yml.before-2.1.10`；`image` 由 `…:2.1.10-beta01` 改为 `…:2.1.10`，`pull` 后 `up -d` 重建容器。
- 完成判定：容器 `Up` 且运行 `ghcr.io/sakuralovesmile/sakura-blog:2.1.10`；宿主 `http://127.0.0.1:8081/readyz` = `{"ok":true}`；公网 `https://blog.上杉九月.cn/readyz` 200；日志无 panic、无迁移错误，启动自动备份 `blog-backup-20261119-092643.zip`。
- 产物核对：生产 `/console/assets/ShortcodeInsertPanel-BbQN3Xol.js` 含「插入外部 HTML」，确认线上 Console 是新构建。
- 回退：把 compose 的 `image` 改回 `…:2.1.10-beta01` 后 `up -d`。本版本后端零改动、无 schema 变化，回退不需要恢复数据。

## 未验证与限制

- 线上功能体验未做（本次交付即交给用户在线上确认，确认前状态保持「待体验」）。
- Console 页面内的版本号未在浏览器核对（无管理员会话）；版本证据取自运行镜像 tag 与容器内二进制。
- 编辑器内插入效果依赖作者信任模型（`html.WithUnsafe()` 原样直出），与既有正文管线一致。
