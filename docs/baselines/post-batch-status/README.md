# 文章批量发布 / 批量草稿基线（BLG-20261102-04）

本基线固定 Console「文章管理」批量浮条新增状态操作的唯一参照、视口规范与可逐项核对的验收清单。

## 1. 唯一参照图

- **来源**：用户 2026-11-02 提供的线上 Console「文章管理」截图（原图 2902×1445，`image/webp`）。
- **本地归档副本**：`docs/baselines/post-batch-status/reference-before.png`（PNG 转换，尺寸不变）。
- 参照内容：当前批量浮条为「2 已选文章 + 批量移入回收站 + 关闭」，深色 Console、圆角胶囊、`--color-card` 底色、`--shadow-lg` 阴影、`VButton size="sm"` 高度与间距。
- 本次目标不是重设计：新增按钮必须与既有浮条同一视觉体系（同尺寸、同圆角、同间距、同语义色），不得引入新组件、裸色值或第二套按钮样式。

## 2. 视口规范

| 视口 | 宽度 | 浮条要求 |
| --- | --- | --- |
| 桌面宽屏 | ≥ 1280px | 单行：计数药丸 · 批量发布 · 批量设为草稿 · 批量移入回收站 · 关闭 |
| 桌面紧凑 / 平板 | 768–1279px | 允许 `flex-wrap` 折行，按钮不裁切、不超出浮条 |
| 移动 | 390×844 | 浮条 `max-w-[calc(100%-2rem)]` 内折行；全部按钮可点击，无横向溢出 |

## 3. 验收项

### 3.1 浮条与按钮
- [x] 选中 ≥1 篇文章后浮条出现，顺序为「批量发布（primary）」「批量设为草稿（default）」「批量移入回收站（danger）」，关闭按钮位置不变。
- [x] 现有「批量移入回收站」的视觉与行为不变（仅新增按钮，未重排既有结构）。
- [x] 目标集合只含当前页选中且非回收站文章；已发布的文章不进入「批量发布」目标，草稿不进入「批量设为草稿」目标。
- [x] 无可操作目标时按钮禁用并带原因 `title`；全部选中项在回收站时两个状态按钮均禁用。实测 `title`：`所选文章均已发布`、`所选文章均已是草稿`、`回收站文章需先恢复再设置状态`。
- [x] 按钮使用既有 `VButton size="sm"` 胶囊（12px 字号），未新增裸色值、未新增组件。

### 3.2 确认与结果
- [x] 「批量发布」确认弹窗为信息型（`Dialog.info`，主色确认胶囊），标题/描述包含条数。Console 当前 `--color-primary` 是浅色 `#e8e8ec`（白底黑字），不是蓝色。
- [x] 存在未发布草稿修改（`has_draft`）时，发布确认说明「本次只切换发布状态，草稿修改保留」。文案由 `node:test` 覆盖；浏览器夹具未构造 `has_draft` 文章。
- [x] 「批量设为草稿」确认弹窗为警告型（`Dialog.warning` + 次级确认），说明前台将下线。
- [x] 确认后列表状态列立即反映，选择清空，浮条消失，Toast 汇总 `succeeded`；`failed>0` 时追加条数（仅 `node:test` 覆盖，浏览器未构造 failed 夹具）。
- [x] 取消确认不发请求、不清空选择（浏览器实测 0 次 batch 请求，浮条/计数/复选框保留）。

### 3.3 数据边界
- [x] 批量状态切换不改正文、slug、作者、分类、置顶、口令、`publish_at`、草稿头（`internal/store/post_batch_status_test.go` 逐字段断言）。
- [x] 已是目标状态的文章 `updated_at` 不变。
- [x] 回收站文章不会被批量状态接口改动，计入 `failed`。

### 3.4 浏览器证据
- [x] 1280×800：草稿 → 批量发布 → 状态「已发布」；再选中单篇批量设为草稿 → 状态「草稿」，另一篇保持「已发布」。
- [x] 390×844：同上路径中浮条折行（高 149.8px）不溢出（`scrollWidth = 390`），三个按钮可点且移动端真实点击发布成功。
- [x] 验收对象是当前构建产物：浏览器实际入口 HTML 与 `PostList`、`api` chunk 的 SHA256 与磁盘 `modules/console/web/dist` 同名文件一致（见 `browser-verification.json` 的 `consoleArtifacts`）。
- [x] 截图归档：`dock-desktop-1280.png`（1280×800）、`dock-mobile-390.png`（390×844）、`after-published.png`、`after-draft.png`。

## 4. 验证记录

- 运行：2026-11-02，Playwright 1.62.1 + Chromium 151，8096 隔离临时库与 `-dev` 服务；`post-batch-status.spec.ts` 6/6，回归 `console-ui-consistency.spec.ts` 38/38 与 `console-mobile-a11y.spec.ts` 16/16。
- 原始证据：`browser-evidence.json`（按用例实测值）与 `browser-verification.json`（命令、通过数、产物指纹、未覆盖项）。
- 未覆盖：中间断点（768–1279）、Firefox/WebKit/真机、`has_draft` 与 `failed>0` 的浏览器夹具、真实数据规模性能。
