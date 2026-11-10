# BLG-20261110-01：仪表盘尺寸档位统一足迹

- 状态：待体验（本地实现与验证完成，未提交、未合并、未发布）
- 分支：`codex/feat/20261110-BLG-20261110-01-dashboard-unified-widget-size`
- 工作树：`/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261110-feat-BLG-20261110-01-dashboard-unified-widget-size`
- 基点：`dev` @ `b24bc214`
- 合入目标：`dev`
- 验收依据：`docs/baselines/dashboard-unified-size/README.md`（唯一参照、足迹表与逐项清单）

## 目标与范围

“小 / 中 / 大”在任一断点下只对应一个足迹，16 个已注册组件共用；选同一档的任何两个组件必须一样大。

范围限定在 Console 仪表盘的尺寸体系与组件密度层：

- `modules/console/web/src/modules/dashboard/widgets/layout.ts`（预设矩阵、约束、识别、默认布局、迁移）
- `modules/console/web/src/modules/dashboard/widgets/*.vue`（各组件按新盒子适配内容）
- `modules/console/web/src/modules/dashboard/WidgetSizeModal.vue`、`WidgetCenter.vue`、`DashboardGrid.vue`、`DashboardDesignerView.vue`、`dashboard.css`
- `modules/console/tests/*` 中固定旧档位数值的断言
- 新增 `tests/e2e/dashboard-widget-size.spec.ts`

不改后端 API、布局存储格式、Stellar 前台与设置页。

## 已确认决定

- 方向 A：统一足迹。档位是盒子，不再是每组件一套内容尺寸。
- 已确认（2026-11-10）：lg「大」为整行 `12×40`，两张「大」卡改为上下排；统计卡「大」用 hero 居中排版，不设例外。
- 既有 4 个档位异常随统一足迹消失，不再单独修：md 趋势大＝中、lg 分类大比中矮、xs/xxs 快捷中比小矮、xs/xxs 统计同尺寸换标签。
- 各组件 `minW`/`minH` 下调到不高于所属断点最小档位，避免 `getGridPresetSize` 钳制破坏统一。

## 实施阶段

| 阶段 | 内容 | 出口 |
| --- | --- | --- |
| T1 | 重写 `WIDGET_PRESET_SPECS` 为断点级三档；约束、识别、去重、默认布局与迁移同步 | 单元测试证明任意组件三档与表一致，无钳制 |
| T2 | 组件适配：统计卡 hero、三个图表 mini 档、列表/版本/健康/快捷按档收放 | 每个组件三档不裁切、不空荡 |
| T3 | 弹窗文案与预览、设计器新增默认尺寸、保存链路 | 三档恒在、可保存可刷新 |
| T4 | 机械门禁与真实浏览器量测（144 组） | 清单 A–G 全绿、量测 JSON 与截图入库 |

## 关键实现约定

- 足迹表：lg/md 12 列 `3×16 / 6×28 / 12×40`；sm 6 列 `3×16 / 6×28 / 6×40`；xs/xxs 4 列 `2×16 / 4×26 / 4×36`。
- `getGridPresetSize` 不再按组件查矩阵；矩阵按断点唯一，组件只决定内容密度。
- 旧布局靠现有 `normalizeBreakpointWidgets` 的“非声明尺寸落到最近档位”路径迁移，不新增存储版本号；不自动写回服务器。
- 图表画布高度改为按盒子剩余空间计算（CSS flex + 量测），去掉 `140 / 220` 这类写死高度，否则小档与整宽大档会溢出或留白。
- 统计卡大档为居中 hero 排版，填充度按 45% 验收（其余 60%）。

## 验收场景

1. lg 1280px：把访问时段与 Stellar 版本都设为“大”，两者宽度与高度完全一致（截图对照 `before-large-mismatch.webp`）。
2. lg / sm / xxs 三断点默认布局：只出现三档尺寸，无重叠、无横向溢出。
3. 16 组件逐个切三档：不裁切、不空荡、图表 mini 档可读。
4. 旧布局载入 → 落到最近档位 → 保存 → 刷新后保持。
5. 尺寸弹窗三档恒在、面积递增、当前档唯一高亮。

## 验证计划

- `make test`；`make build` + 重启服务核对新产物。
- 新增 e2e：144 组几何/溢出/填充度量测，`console.error`、`pageerror` 为 0。
- 证据：`docs/baselines/dashboard-unified-size/` 下的 before/after 截图与 `measurements.json`。

## 实施记录（2026-11-10）

| 文件 | 改动 |
| --- | --- |
| `widgets/layout.ts` | 删除按组件的尺寸矩阵与 `minW/minH`，改为断点级 `WIDGET_PRESET_FOOTPRINTS`；新增 `getDefaultPreset`；重写三套默认布局 |
| `DashboardGrid.vue` / `useWidgetSize.ts` | 未改协议；组件继续通过注入拿 preset |
| `dashboard.css` | 卡片按档位内边距；统计卡三档排版（小=单行、中=居中、大=hero 56px）；版本卡去掉 `min-height` |
| `StatCardWidget.vue` | 大档补「累计」说明行 |
| `TrendChartWidget.vue` | 小档隐藏坐标轴与结论行，图表吃剩余高度 |
| `HourlyChartWidget.vue` | 删除写死的 140/180/220 画布高度，改 flex 填充 |
| `CategoryDonutWidget.vue` | 小档改占比条 + 前三名；中/大档环形图按盒子缩放；手机档保持左右排布 |
| `RecentPosts/RecentComments/TopPosts` | 条数按档位（2/4/5-7）、小档单行化、空态与骨架屏不再撑高 |
| `QuickActionsWidget.vue` | 卡片改纵向 flex，中/大档行高按剩余空间均分 |
| `SystemHealthWidget.vue` | 小档去第三项指标与运行时长行；手机大档压缩存储区 |
| `WidgetSizeModal.vue` / `size-preset-options.ts` | 文案改为「同档同尺寸」；去重注释更新 |
| `registry.ts` / `moments/module.ts` | `defaultSize` 对齐新足迹（统计卡 3×16、其余 6×28） |
| `tests/widget-size-presets.test.ts` | 重写为足迹一致性、三档递增、三档恒在、默认布局、识别与 4 项旧缺陷回归 |
| `tests/grid-layout.test.ts` | 期望值改为新足迹；约束断言改为「不钳制档位」 |
| `tests/ui-restraint.test.ts` | 字号白名单新增 hero 56（附来源说明） |
| `tests/widget-registry.test.ts` | 矩阵断言改为足迹表与 `WIDGET_TYPES` |
| `tests/e2e/dashboard-widget-size.spec.ts` | 新增 144 组量测：同档同尺寸、不裁切、填充度、无横向溢出、无控制台错误 |

关键实测（详见 `docs/baselines/dashboard-unified-size/README.md`）：lg/sm/xxs 三个断点下
小/中/大三档各自只有一个渲染尺寸，144 组最大溢出 0px；有数据组件填充度 0.71–0.91。

## 未验证 / 遗留

- 设计器「保存布局→刷新」交互未在本次 e2e 覆盖（e2e 走布局 API 直写）。
- `console-mobile-a11y` 未单独重跑；本次未改按钮尺寸。
- 空态组件（趋势/时段/热门/最近评论）在夹具中没有数据，只验证了不裁切与空态居中。
- 仅 Chrome；Safari / Firefox 与真机触屏不在本次范围。
- 未提交、未合并、未发布。

## 下一步

体验确认后在 feature worktree 执行 `make merge-check` 合入 `dev`（会再跑一次 `make test` 并复验组合态），
随后按既有规则清理本任务 worktree 与已合并分支；推送与发布另需授权。
