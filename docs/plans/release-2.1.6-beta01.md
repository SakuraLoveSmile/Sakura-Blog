# BLG-20261110-03：v2.1.6-beta01 预发布

- 状态：待体验（已发布 v2.1.6-beta01，未部署生产，未推送其它分支）
- 日期：2026-11-10（Asia/Taipei）
- 版本依据：最新稳定版 `v2.1.5`（2026-11-10），本次取下一个 patch 的预发布 `v2.1.6-beta01`
- 功能记录：`docs/plans/BLG-20261110-01-dashboard-unified-widget-size.md`（仪表盘尺寸档位统一足迹）
- 证据：`docs/plans/release-2.1.6-beta01.md`（本文件）

> 编号说明：`BLG-20261110-01` 已被同日创建的 `docs/plans/release-2.1.5.md` 使用，仪表盘任务也沿用了该编号，
> 两处记录重叠；本次发布取 `BLG-20261110-03`，未回改已合并的历史记录。

## 发布内容

- `591002f5` 仪表盘尺寸档位统一足迹：同一断点同一档位只有一个尺寸，16 个组件共用
- `96d5b323` 趋势卡按档位收放 + 既有 e2e 尺寸断言更新 + 量测夹具补真实 PV/UV
- `76c03153` 尺寸量测用例结束后还原默认布局，避免跨用例污染
- `2b4f472d` / `e866e8ce` 任务记录与 `origin/main` 历史同步

## 前置处理

- `dev` 缺少 `origin/main` 的 1 个 merge 提交：从 `origin/main` 建同步分支 `codex/chore/20261110-BLG-20261110-02-sync-main`，
  经 `make merge-check` 合回 `dev`（`e866e8ce`，`git diff` 内容零差异）。
- 首轮 PR CI 的 e2e 暴露两处遗留：`console-ui-consistency` 仍断言旧的行高与档位识别；趋势卡在有数据/窄屏时被固定
  最小高度撑破。修复见 `96d5b323`，并在同一轮把量测夹具补上真实 PV/UV（趋势、时段、热门按有数据状态量测）。
- 次轮 CI 只剩 `writing-assistant` 两个手机用例失败：根因是量测用例把含「系统状态」的布局留在账号里，后续用例
  经过仪表盘时请求 `/api/v1/perf/overview`，而 E2E 夹具未启用 perf 模块返回 403。改用例结束还原默认布局（`76c03153`）。

## 晋级与发布

- PR [#73](https://github.com/SakuraLoveSmile/Sakura-Blog/pull/73)（dev→main）：console / go / lint / security / build 通过，
  e2e 23m32s 通过；`image`、`release-source` 在 dev→main 阶段跳过属预期。合并 `820bb893`。
- PR [#74](https://github.com/SakuraLoveSmile/Sakura-Blog/pull/74)（main→release）：八项全绿（e2e 23m15s、image 2m1s、
  release-source 12s）。合并 `2c295fb4`；`git diff origin/main origin/release` 为空。
- tag `v2.1.6-beta01`（GPG 签名，key `979B6BCF…3F3AD989`，`git tag -v` Good signature）打在 `2c295fb4` 并推送。

## 镜像与分发

- [Docker Release 42769266213](https://github.com/SakuraLoveSmile/Sakura-Blog/actions/runs/42769266213) success（06:33:06Z → 06:38:07Z）。
- 镜像 `ghcr.io/sakuralovesmile/sakura-blog:2.1.6-beta01`，OCI index digest
  `sha256:c2bb3dd3d8bb13540b831dc5a2db2b8cd59f4d0f75909fc484f04465839cee21`；平台 `linux/amd64`（另有 unknown/unknown 的 attestation manifest）。
- 标签：新增 `2.1.6-beta01`、`sha-2c295fb`；**未**生成 `2.1.6`，`latest` / `2` / `2.1` / `2.1.5` digest 与发布前基线一致
  （均 `sha256:df35a021…6d9a6bf4`）。
- 镜像标签核对：`org.opencontainers.image.version=2.1.6-beta01`、`revision=2c295fb4…`、`created=2026-11-10T06:33:30Z`，
  Entrypoint `blog -root /app`。
- GitHub Release `v2.1.6-beta01`：`prerelease=true`、`draft=false`、0 资产，`isLatest=false`（`v2.1.5` 仍为 Latest）。

## 测试预期调整（均保持原检查强度）

| 位置 | 旧预期 | 新预期 | 依据 |
| --- | --- | --- | --- |
| `console-ui-consistency` 1280 尺寸 | 统计卡 h12（88–112px）、趋势卡 h44（330–380px） | h16（120–136px）、h40（310–330px） | 统一足迹后小/大档行数变化，仍断言具体行数与像素区间 |
| `console-ui-consistency` 平板/手机弹窗识别 | 五个组件一律识别为中档 | 趋势=大档，其余=中档 | 新默认布局给趋势卡 tall 档 |
| `dashboard-widget-size` 夹具 | 空数据库空态 | 6 篇文章 + 真实 PV/UV，用例后还原默认布局 | 空态无法覆盖图表与列表的真实高度 |

## 验证

- 本地：`make test`、`make build`、`pnpm e2e`（整套 237 passed / 2 failed，失败为夹具未启用 perf 的跨用例污染，已由 `76c03153` 修复并复跑通过）。
- `tests/e2e/dashboard-widget-size.spec.ts`：16 组件 × 3 档 × 3 断点 = 144 组，同档渲染尺寸唯一、最大溢出 0px。
- CI：两个 PR 的 e2e 均通过（23m32s / 23m15s）。

## 未验证 / 遗留

- **镜像内二进制启动未验证**：本机 Docker daemon 未运行（`docker info` 无法连接），本次未获授权操作生产服务器；
  未执行 `docker run --rm …:2.1.6-beta01 -h`。镜像标签、digest 与 OCI 配置已核对，但「容器可启动」缺证据。
- 未升级生产：compose 仍指向 `v2.1.5` 镜像。
- 未推送 `dev` 之外的其它本地内容；`main`、`release` 已推送。
- 仅 Chrome；Safari / Firefox 与真机触屏不在本次范围。

## 下一步

- 需要启动验证时：在有 Docker 的机器或服务器执行 `docker run --rm ghcr.io/sakuralovesmile/sakura-blog:2.1.6-beta01 -h`（退出码 0）。
- 生产升级需单独授权：compose 固定 `:2.1.6-beta01` → `docker compose pull && docker compose up -d` → 探测 `/readyz`。
- 体验确认后把状态改为「已验收」。
