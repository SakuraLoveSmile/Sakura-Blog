# BLG-20261109-03：编辑器标签预览表面自适应与 v2.1.3 正式发布

<!-- 状态词只用 未开始 / 实现中 / 待体验 / 已验收；已验收须用户明确确认。 -->

- 项目 / 任务编号：Blog / BLG-20261109-03
- 状态：待体验（修复、两级 PR、签名 tag、镜像发布、生产升级与线上验收均已完成，等待用户确认编辑器视觉）
- 日期：2026-11-09（Asia/Taipei）
- 分支 / worktree：`codex/fix/20261109-BLG-20261109-03-editor-tag-surface`（修复，已合入 dev 与 main）；worktree `20261109-fix-BLG-20261109-03-editor-tag-surface`
- 发布分支：`release`；tag：`v2.1.3`（GPG 签名，key `979B6BCF…3F3AD989`，指向 `e5eb3c2c`）
- 版本依据：用户 2026-11-09 明确要求「走一遍发布流程、顺便更新线上」。当天 09:47Z 另一条发布流程已把 v2.1.2（GitHub 卡片渲染修复）发布并升级生产；本次修复只能取 `v2.1.3`，其内容包含 v2.1.2 的全部改动。
- 基线证据：`docs/baselines/editor-tag-surface/`（改动前后浏览器截图与度量）、`docs/baselines/release-2.1.3/`（发布校验 JSON、生产验收 JSON、生产卡片截图）

## 问题与修复

- 现象：文章编辑器里的 GitHub 仓库卡片周围出现一整块站点主题底色，卡片只占左侧约 400px，余下区域全部露色（用户 2026-11-09 16:41 截图）。
- 定位：编辑器里每个 shortcode 标签是服务端整页预览 iframe；非页面画布场景会注入站点不透明表面（`html, body { background-color: var(--background) !important }`），深色 Console 上即显示为整块主题背景。页面编辑器因为已有正式页画布，使用透明嵌入，所以没有该现象。
- 修复：
  - `modules/console/web/src/components/editor/StellarTagView.vue`：NodeView 读取编辑器实际露出的底色（`body` → `documentElement` 第一个不透明背景），传给预览；`layout=page` 仍使用 `embedded`。
  - `modules/console/web/src/modules/plugins/preview/preview-document.ts`：新增 `surfaceLuminance` / `surfaceSchemeOf` / `PREVIEW_SURFACE_MATCH_CONTRAST = 2`；首屏 CSS 只给与宿主明暗相反的站点主题兜底；运行时脚本比较站点表面色与宿主底色的对比度，同色系 `removeProperty` 保持透明，异色系落成不透明 `rgb`。
  - `modules/console/web/src/modules/plugins/preview/usePluginPreview.ts`：透传 `hostBackground`，并计入预览缓存键。
  - 测试：`modules/console/tests/plugin-preview.test.ts`、`modules/console/tests/stellar-tag-preview.test.ts`、`tests/e2e/tag-live-preview.spec.ts`。
- 边界：卡片自身 `max-width:400px`、`--card` 背景与边框不变（与线上同款）；独立预览（Studio/插入面板/Markdown 分屏/组件库/设置预览）行为不变。

## 发布前同步

- 起点 `origin/dev` = `8a96ec23`；修复提交 `4aa3d7c1`（`make merge-check` 通过）。
- 同步 `origin/main`（4 条合并历史，内容零差异）→ 合并提交 `1529867b`；推送 `origin/dev`。
- 同步 `origin/release`（v2.1.2 tag 提交 `f0268369`，内容零差异）→ 合并提交 `019d5665`；这样 dev 直接包含 release 历史，省去本轮单独的 sync PR。推送 `origin/dev`。
- 另一条发布流程随后在 dev 追加 v2.1.2 发布记录（`9f8f1f9d`），随本次 PR 一并晋级。

## 晋级与发布

- PR [#67](https://github.com/SakuraLoveSmile/Sakura-Blog/pull/67)（dev→main，"发布 v2.1.3：编辑器标签预览表面按配色自适应"）：go / lint / security / console / e2e / build 全绿，合并 `b44d073f`。
- PR [#68](https://github.com/SakuraLoveSmile/Sakura-Blog/pull/68)（main→release，"release: 发布 v2.1.3"）：go / lint / security / console / e2e / build / release-source / image 全绿，合并 `e5eb3c2c`；合并后 `origin/main` 与 `origin/release` 内容零差异。
- tag `v2.1.3`（GPG 签名，`git tag -v` 通过）打在 `e5eb3c2c` 并推送。
- [Docker Release 42605031138](https://github.com/SakuraLoveSmile/Sakura-Blog/actions/runs/42605031138) success（10:57:02Z→11:02:36Z，约 5.5 分钟），创建 GitHub Release `v2.1.3`：`prerelease=false`、`draft=false`、`target=main`、0 资产、更新日志 315 字符。

## 镜像与标签核对

- 镜像：`ghcr.io/sakuralovesmile/sakura-blog:2.1.3`
- manifest list digest：`sha256:552447be8720fdb030b9a1419966230c398fdd354559fbda706511efbc305256`
- amd64 镜像 config digest（服务器 `docker image inspect`）：`sha256:963b9d7eefa4bac1df49f53777d31c39bb88186515eefda93cc41d3fd680cf02`
- label：`org.opencontainers.image.revision=e5eb3c2cea023d3e0fcb508752a6b30c125e360b`、`org.opencontainers.image.created=2026-11-09T10:57:25.080Z`
- 别名 `2.1.3` / `2.1` / `2` / `latest` 指向同一 manifest；`2.1.2` 镜像仍保留在服务器用于回退
- 服务器 `docker run --rm ghcr.io/sakuralovesmile/sakura-blog:2.1.3 -h` 退出码 0，输出 usage

## 生产升级

- 服务器：`blog-prod`（Ubuntu，compose 目录 `/opt/1panel/docker/compose/blog`，宿主端口 8081）。
- 升级前：compose 固定 `2.1.2`，容器 `14352c0a9faf`，image `sha256:ad5c0840…`（2.1.2），Up 13 分钟，restarts 0；`/readyz` 200；根盘剩余 3.1G；Console 入口 `assets/index-tof-zf5V.js`。
- 备份：`/home/ubuntu/blog-pre-v2.1.3/`（0700，ubuntu:ubuntu）
  - `site-state-pre-2.1.3.tar.gz`（3,012,925 字节，SHA256 `8e421b9138c897699733cae50b73e8cd3e0cbcbe4409e70d3e5cb70b73a7ea47`，含 `config/`、`content/`、`data/blog.db`，`sudo tar`）
  - `docker-compose.yml.before-2.1.3`（image 行仍为 2.1.2）
- 变更：compose `image` 由 `2.1.2` 改为 `2.1.3`（先 `docker pull` 并验证二进制），`docker compose up -d` 重建容器。
- 停机窗口：11:04:57Z 开始重建 → 11:04:58Z 新容器 Started，约 1 秒；本次无 schema 迁移。
- 升级后：容器 `3401ffd19577`，镜像 `2.1.3`，image `sha256:963b9d7e…`（与 GHCR digest 一致），restarts 0，running；宿主 `/readyz` 200。

## 生产验收（2026-11-09T11:05Z 后）

- 公网 `https://blog.xn--fhqths51enha.cn`：`/readyz`、`/settings`、`/console/` 均 200。
- Console 入口从 `assets/index-tof-zf5V.js` 变为 `assets/index-Dz1HIoGi.js`；遍历入口引用的 83 个 chunk，`assets/usePluginPreview-Dv_6PJrz.js` 含新逻辑标记 `preview-surface:host=`，证明生产 SPA 已加载本次修复。
- 文章 `/post/pi-desktop-custom-dev-log/`（Playwright 1280×900 DPR2 dark，networkidle）：`.ghrepo-card` 424.2×110、背景 `rgb(39,37,32)`、12px 圆角，console error / page error 均为 0；截图 `docs/baselines/release-2.1.3/production-ghrepo-card.png`。
- 未登录生产 Console 复测编辑器视觉（没有生产账号），编辑器里「卡片周围不再出现整块站点背景」需要用户实际确认。

## 回退

- 镜像回退：compose `image` 改回 `ghcr.io/sakuralovesmile/sakura-blog:2.1.2`（镜像仍在服务器），`docker compose up -d`；本次无 schema 迁移，只切镜像即可。
- 数据回退（如需）：停容器后以 `/home/ubuntu/blog-pre-v2.1.3/site-state-pre-2.1.3.tar.gz` 恢复 `config/`、`content/`、`data/blog.db` 再启动。

## 已知限制

- 深色主题的「同色系透明」判定只在隔离 e2e 实例验证；生产 Console 未登录复测编辑器视觉。
- 本次发布与另一条 v2.1.2 发布流程并发：v2.1.2 先发布并升级生产（10:00Z），v2.1.3 在其之上覆盖；两条流程都改动了 dev，发布记录按提交顺序共存。
- `dev` 比 `main` 少 release 合并提交（`e5eb3c2c`），下一次发布前仍需按既有流程同步。
- 本机 Docker daemon 未启动，镜像 manifest / 二进制启动检查在生产机完成。

## 收尾

- 本记录与 `docs/baselines/release-2.1.3/` 提交到 `dev` 并推送；修复 worktree 与本地任务分支在核验改动已进入 release 后清理。
