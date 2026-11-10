# BLG-20261110-01：收敛原文配置入口与 v2.1.5 正式发布

<!-- 状态词只用 未开始 / 实现中 / 待体验 / 已验收；已验收须用户明确确认。 -->

- 项目 / 任务编号：Blog / BLG-20261110-01
- 状态：待体验（实现、PR、签名 tag、镜像发布、生产升级与线上核验均已完成，等待用户确认界面）
- 日期：2026-11-10（Asia/Taipei）
- 功能记录：`docs/plans/BLG-RAWSURF01-stellar-raw-config-entry.md`（根因、范围、验收详情）；视觉依据 `docs/baselines/raw-config-entry/`
- 发布记录：本文件；机器可读证据 `docs/baselines/release-2.1.5/release-verification.json`
- tag：`v2.1.5`（GPG 签名，key `979B6BCF…3F3AD989`，指向 `b1724611`）
- 版本依据：用户 2026-11-10 明确要求「发布新版本 更新线上」。上一版本 `v2.1.4`（`70ea2a7a`），本次只能取 `v2.1.5`。

## 发布内容

- `9154595f` 收敛内容编辑器里的 Stellar 2 原文配置入口（BLG-RAWSURF01）
- `d184fc41` 优化仪表盘访问趋势小组件展示样式
- `5168fda0` v2.1.4 发布记录（随本次一并晋级）

## 前置处理

发布前 `dev` 不包含 `origin/main` 与 `origin/release` 的祖先关系（两者各含 `dev` 未合并的 merge 提交，但 `git log --no-merges` 与 `git cherry` 均证明**内容零差异**）。按 v2.1.3 的做法，先把两者历史并入 `dev`：

- 合并 `origin/main` → `3f0a78d7`
- 合并 `origin/release` → `39048662`
- 两次均为纯历史合并，`git diff` 无内容变化；随后推送 `origin/dev`。

## 晋级与发布

- PR [#71](https://github.com/SakuraLoveSmile/Sakura-Blog/pull/71)（dev→main）：console / go / lint / security / e2e（22m45s）/ build 全绿；`image`、`release-source` 在 dev→main 阶段跳过属预期。合并 `99ea31af`。
- PR [#72](https://github.com/SakuraLoveSmile/Sakura-Blog/pull/72)（main→release）：go / lint / security / console / e2e（16m41s）/ build / image / release-source 八项全绿。合并 `b1724611`。
- tag `v2.1.5`（GPG 签名，`git tag -v` Good signature）打在 `b1724611` 并推送。

## 镜像与分发

- [Docker Release 42669581338](https://github.com/SakuraLoveSmile/Sakura-Blog/actions/runs/42669581338) success（17:57:13Z → 18:02:34Z，约 5.4 分钟）。
- 镜像 digest `sha256:df35a021…6d9a6bf4`；匿名 token 核对标签 `2.1.5`、`2.1`、`2`、`latest` 均 HTTP 200。
- `docker run --rm …:2.1.5 -h` 退出码 0，镜像内二进制可启动。
- GitHub Release `v2.1.5`：`prerelease=false`、`draft=false`、0 资产、更新日志 319 字符。

## 生产升级

- 宿主 `blog-prod`（124.220.55.80），compose `/opt/1panel/docker/compose/blog/docker-compose.yml`。
- 本次无数据迁移，只改镜像标签：`2.1.4` → `2.1.5`；改前备份 `docker-compose.yml.bak-2.1.4`。
- `docker compose up -d` 重建容器；`Restarts=0`、`Status=running`、启动于 18:04:11Z。
- 宿主映射端口 8081 `/readyz` 返回 200；前台 `/` 返回 200。

## 线上核验

- Console 入口产物 `assets/index-YmQ_3yXL.js`，与本地验收构建同一份。
- 通过宿主端口拉取线上 chunk 并同时检查直接与 `\uXXXX` 转义两种形式：`StellarContentConfig-tIRA3H9m.js`、`PostEditor-m1wCGwbU.js` 均**含新文案「高级：配置原文」、不含旧文案「Stellar 2 原文配置」**。
- 容器版本标签 `2.1.5`。

## 未验证 / 已知边界

- 未用真实浏览器登录生产 Console 逐页核对入口形态（登录凭据不在本次操作范围）；线上证据为产物内容核对与 HTTP 探测。
- 功能侧遗留同 `BLG-RAWSURF01`：文库文档页无真实截图、旧版本自动展开未用真实旧内容实测、page/collection schema 覆盖仍为 5/29 与 18/40。
- 回退：把 compose `image` 改回 `2.1.5` 的上一版 `2.1.4` 并 `up -d` 即可；本次无数据迁移，无需恢复备份。

## 待用户确认

界面效果（原文配置入口的一行化形态）需用户确认后才可标「已验收」。
