# BLG-20261109-02：卡片渲染修复与 v2.1.2 发布

<!-- 状态词只用 未开始 / 实现中 / 待体验 / 已验收；已验收须用户明确确认。 -->

- 项目 / 任务编号：Blog / BLG-20261109-02
- 状态：待体验（修复、验收、两级 PR、签名 tag、镜像发布、生产升级与线上验收均已完成，等待用户确认）
- 日期：2026-11-09（Asia/Taipei）
- 分支 / worktree：`codex/fix/20261109-BLG-20261109-02-card-render-fixes`（已合入 dev）；`codex/chore/20261109-BLG-20261109-02-sync-main`、`codex/chore/20261109-BLG-20261109-02-sync-release`（历史同步）
- 发布分支：`release`；tag：`v2.1.2`（GPG 签名，key `979B6BCF…3F3AD989`，指向 `f0268369`）
- 版本依据：用户 2026-11-09 要求按排查建议顺序修复并沿用上次发布流程；patch 版本 `v2.1.2`，无 schema 迁移
- 基线证据：`docs/baselines/card-render-fixes/`（修复 before/after）、`docs/baselines/release-2.1.2/`（发布校验、生产验收、线上截图）

## 排查背景（只读审计）

2026-11-09 对公网 31 个页面与组件库 40+ 标签做卡片渲染审计（1280/390、暗色；DOM 几何/溢出/裁切/图片/emoji/console 检查）：

- 未发现与 ghcard fork 图标同类的“图标被裁切”问题；
- 发现 3 类可修代码缺陷（本轮修复）与一组内容资源问题（未改，见遗留）。

## 修复内容

1. **banner / quot / tip / frame 旧命名参数兼容**：旧 catalog/旧内容把 `title:`/`subtitle:`/`text:`/`device:` 写成命名参数，而 Stellar v2 官方语法是位置参数，整段落入位置文本被原样显示（线上 `/about` 横幅显示 `title: 我的博客`）。`v2MapArgs` 增加旧命名键白名单，值落到同一字段；官方位置语法不变；编辑解析同步兼容。
2. **link 的 `desc:true/false` 开关语义**：官方文档定义为描述区开关，官方实现把值原样输出，线上 6 篇文章 19 张卡片显示字面量 `true`。v2/v1 renderer 改为 true=rich、false=plain，字面量不入卡；空描述不占行（未配置 siteinfo 时）；catalog 字段说明同步。
3. **rich link 卡片图标固定尺寸**：`.md-text .link-card >.top .img { flex: none }`，超长 URL（Notion 111 字符）不再把 16px 图标压成 0 宽。
4. 测试：新增 `shortcode_v2_card_fixes_test.go`；link desc 开关从严格 oracle 移到“明确上游缺陷偏差”用例，并补 `link icon` 用例覆盖。

## 发布前同步

- 起点：`origin/dev` = `f1fe60f6`，`origin/main` = `93dad705`（4 条 dev 缺少的合并历史提交）。
- 从 `origin/main` 建同步分支 `codex/chore/20261109-BLG-20261109-02-sync-main`，`make merge-check` 通过后合入 `dev`（`8cccbe77`），`dev..origin/main` = 0。
- 修复分支提交 `8bca9233` 经 `make merge-check` 合入 `dev`（`8a96ec23`），推送 `origin/dev`。

## 晋级与发布

- PR [#64](https://github.com/SakuraLoveSmile/Sakura-Blog/pull/64)（dev→main，"发布 v2.1.2：卡片渲染修复"）：go / lint / security / console / e2e / build 全绿，合并 `026ffb22`。
- PR [#65](https://github.com/SakuraLoveSmile/Sakura-Blog/pull/65)（同步 release 合并历史到 main）：`release` 上有 v2.1.1 的 `main→release` 合并提交 `038ffab8`，为满足 `strict` 保护从 `main` 建同步分支 `git merge origin/release`；`git diff --stat origin/main HEAD` 为空，合并 `d9ca4d42`。
- PR [#66](https://github.com/SakuraLoveSmile/Sakura-Blog/pull/66)（main→release，"release: 发布 v2.1.2"）：go / lint / security / console / e2e / build / release-source / image 全绿，合并 `f0268369`；合并后 `git diff origin/main origin/release` 为空。
- tag `v2.1.2`（GPG 签名，`git tag -v` 通过）打在 `f0268369` 并推送。
- [Docker Release 42596371051](https://github.com/SakuraLoveSmile/Sakura-Blog/actions/runs/42596371051) success（09:49:47Z→09:55:30Z，约 5m43s），创建 GitHub Release `v2.1.2`：`prerelease=false`、`draft=false`、`target=main`、更新日志 458 字符；`releases/latest` 变为 `v2.1.2`。

## 镜像与标签核对

- 镜像：`ghcr.io/sakuralovesmile/sakura-blog:2.1.2`
- digest：`sha256:ad5c0840a8f163ee030cd3f575b3b386f8fc529659f2a4c035a0ebc8967d6e42`
- label：`org.opencontainers.image.version=2.1.2`、`revision=f0268369a14fe592b932ced7815e2dad4ed77b1c`、`created=2026-11-09T09:50:10.370Z`
- 别名 `2.1` / `2` / `latest` 与 `2.1.2` 指向同一 manifest；`sha-f026836` 存在
- 稳定版基线：`2.1.1` digest `sha256:2e6dbe0a…436da4c1` 未移动
- 服务器 `docker run --rm ghcr.io/sakuralovesmile/sakura-blog:2.1.2 -h` 退出码 0

## 生产升级

- 服务器：`blog-prod`（Ubuntu，compose 目录 `/opt/1panel/docker/compose/blog`，宿主端口 8081）。
- 升级前：compose 固定 `2.1.1`，容器 `b1b5fc6ec21e`，image ID `sha256:2e6dbe0a…436da4c1`，Up 4 小时，restarts 0；`/readyz` 200；根盘剩余 3.1G；`/settings` 显示 `2.1.1`。
- 备份：`/home/ubuntu/blog-pre-v2.1.2/`（0700，ubuntu:ubuntu）
  - `site-state-pre-2.1.2.tar.gz`（2,988,874 bytes，SHA256 `bbb4e3c0c480a712019573d3b6069d7c43cde50eb695db15c767930679b20bef`，含 `config/`、`content/`、`data/blog.db`）
  - `docker-compose.yml.before-2.1.2`、`before.txt`
- 变更：compose `image` 改为 `ghcr.io/sakuralovesmile/sakura-blog:2.1.2`（先预拉取），`docker compose up -d` 重建容器。
- 停机窗口：10:00:30Z 冷停 → 10:00:34Z 新容器 running，约 4 秒，无 schema 迁移。
- 升级后：容器 `14352c0a9faf`，镜像 `2.1.2`，image ID `sha256:ad5c0840…c8967d6e42`（与 GHCR digest 一致），label `version=2.1.2`，restarts 0，running；宿主 `/readyz` 200。

## 生产验收（本机网络实测，2026-11-09T10:02Z）

- 公网 `https://blog.xn--fhqths51enha.cn`：`/`、`/readyz`、`/console/`、`/uc/`、`/settings` 均 200；`/settings` 显示 `v2.1.2`。
- `/about` 横幅（Playwright 1280×900 DPR2 dark）：标题「我的博客」、副标题「记录技术成长」，不再出现 `title:`/`subtitle:` 字面量；截图 `docs/baselines/release-2.1.2/production-about-banner.png`。
- `/post/anime-collect` 链接卡片：2 张均为 `link-card rich`，无 `true`、无空描述行；长 URL（Notion）与短 URL（B 站）图标均 16×16；console error 0；截图 `production-link-card.png`。
- 未用生产账号登录 Console 做写操作；未在手机真机复核（390px 视口由发布前验收覆盖）。

## 回退

- 镜像回退：compose `image` 改回 `ghcr.io/sakuralovesmile/sakura-blog:2.1.1`（本机仍有该镜像），`docker compose up -d`（本次无 schema 迁移）。
- 数据回退：停容器后以 `/home/ubuntu/blog-pre-v2.1.2/site-state-pre-2.1.2.tar.gz` 恢复 `config/`、`content/`、`data/blog.db` 再启动。

## 遗留：内容资源（未改代码，待确认后替换）

- 3 篇文章 `banner` 指向 `https://bu.dusays.com/2024/09/01/66d42682cd240.jpg`，被浏览器 ORB 拦截 → 横幅背景缺失：`teamspeak`、`stellarbat`、`linux-server`。
- 4 篇文章 `cover` 使用 picsum 随机占位图（首页/列表卡片封面空白或随机）：
  - `pi-desktop-custom-dev-log` → `https://picsum.photos/seed/wkb21ztc/1200/675`
  - `大模型执行能力测试第一弹` → `https://picsum.photos/seed/4h2pgina/1200/675`
  - `Redirectstorage` → `https://picsum.photos/seed/ib2yy77t/1200/675`
  - `hexo-blog-setup-guide` → `https://picsum.photos/seed/rbsnan8b/1200/675`
- `/about` 页面横幅背景与头像仍引用组件库预览素材（`/assets/img/preview/banner-bg.jpg`、`avatar-blog.svg`），建议换成本站上传图。
- 处理方式：需要生产 Console 账号或直接替换 DB 字段；建议用户提供图片后由我们替换上传，或用户自行在 Console 修改。

## 收尾

- 本记录与 `docs/baselines/card-render-fixes/`、`docs/baselines/release-2.1.2/` 证据提交到 `dev` 并推送。
- 修复与同步 worktree、本地任务分支在核验改动已进入目标分支后删除；远端同步/发布分支保留。
