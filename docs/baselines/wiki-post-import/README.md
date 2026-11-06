# 文库管理「从文章导入」基线（BLG-20261106-03）

本基线固定 Console「文库管理」新增「导入文章」入口与导入弹窗的唯一参照、验收清单和实测证据。

## 1. 唯一参照图

- **来源**：用户 2026-11-06 提供的线上 Console「文库管理」截图（原图 2544×1127，`image/webp`）。
- **本地归档副本**：`docs/baselines/wiki-post-import/reference-before.webp`（原图副本，未重编码）。
- 参照内容：项目 `Pi-Desktop` 下 0 篇文档，页头只有「项目设置 / 新建文档」；左侧树为空，右侧空态只有「新建文档」。
- **本次目标不是重设计**：只在页头、右侧空态和左侧空树提示里加入「导入文章」入口，并新增导入弹窗；弹窗沿用现有 `VModal`、`VButton`、`apple-select`、`apple-search` 与 Console 编辑台语义 token（主按钮近白底黑字、状态只用 5px 圆点 + 12px 文字），不新增裸色值、第二套按钮样式或平行组件。

## 2. 视口规范

| 视口 | 宽度 | 要求 |
| --- | --- | --- |
| 桌面宽屏 | 1280×800 | 弹窗单列；文章多选列表、父文档/状态选项、分页与页脚全部可见 |
| 移动 | 390×844 | 弹窗左右各留 16px、无横向滚动；搜索独占一行，多选列表与选项可滚动，页脚取消/导入按钮可点 |

## 3. 验收项

### 3.1 入口与弹窗
- [x] 文库页页头出现「导入文章」（未选项目时禁用），右侧空态与左侧空树提示同步给出导入入口。
- [x] 弹窗标题为「导入文章到「<项目名>」」；说明文案明确「只复制标题与 Markdown 正文，不改动原文章；同名 Slug 已存在时跳过，不覆盖」。
- [x] 搜索走服务端 `GET /api/v1/posts?search=`：实测输入夹具 token 后列表收敛为「共 2 条」，只显示 2 篇夹具文章。
- [x] 文章行展示标题、slug、发布状态（已发布/草稿圆点）与最近更新时间；支持「全选本页」「清空选择」与跨页多选，计数显示「已选 N 篇」。
- [x] 选项「归属父文档」（含根目录与当前项目全部文档）、「导入后状态」（默认「草稿（不公开）」，可切「已发布」）可用；分页显示「共 N 条」并可翻页。

### 3.2 导入结果（数据边界）
- [x] 默认草稿导入 2 篇：文档树出现 2 篇、统计为「2 篇文档 / 0 篇可在线查阅 / 2 篇暂未发布 / 2 篇更新记录」；第一篇自动打开，编辑器标题与正文来自文章。
- [x] 文档与文章逐字一致：API 实测文档 `title`、`content_md` 与文章相同；文档 `status = draft`、`parent = 0`。
- [x] 原文章只读：导入前后 `GET /api/v1/posts/:slug` 的 `content_md`、`status` 与文章一致，未被改写。
- [x] 草稿不公开：导入后 `GET /wiki/<project>/<slug>` 返回 404。
- [x] 「已发布 + 父文档」导入：文档 `status = published`、`parent = 所选父文档 ID`，公开页返回 200。
- [x] 重复导入：再次打开弹窗时该文章标记「已导入」（复选框禁用、提交按钮「导入 0 篇」禁用）；后端同 Slug 一律跳过，不覆盖已有文档（`internal/console/api/wiki_import_test.go` 断言）。
- [x] 回收站文章：不出现在导入候选（文章列表默认排除回收站）；绕过界面直接 `POST /api/v1/wiki/:project/docs/import` 只返回 `failed`（原因「文章在回收站，无法导入」），不写入。
- [x] 单篇失败（空 Markdown 原文 / 文章不存在 / 正文渲染失败）只记入 `failed`，不影响同批其它文章；父文档非法、空列表、超 100 篇、非法状态整单 400；项目不存在 404。

### 3.3 浏览器与产物证据
- [x] 浏览器实际入口 HTML 与磁盘 `modules/console/web/dist/index.html` SHA256 一致（`67fd518a…`），页面实际引用的 4 个入口资源与磁盘构建逐字一致；页面出现新入口「导入文章」。
- [x] 桌面 1280×800：搜索过滤 → 勾选 2 篇 → 默认草稿导入 → 结果面板「已导入 2 篇」→ 完成后文档树 2 篇、第一篇自动打开。
- [x] 移动 390×844：弹窗盒子 `x=16, width=358`，`scrollWidth = 390 = innerWidth`，无横向溢出；两篇文章可勾选，计数「已选 2 篇」。

## 4. 截图

| 文件 | 视口 | 内容 |
| --- | --- | --- |
| `modal-selected-2.png` | 1280×800 | 搜索收敛为 2 篇、勾选 2 篇、「取消本页/清空选择/已选 2 篇」、父文档与状态默认值、分页「共 2 条」、页脚「导入 2 篇」 |
| `result-panel.png` | 1280×800 | 导入完成结果面板「已导入 2 篇」+ Toast 汇总；页头统计已更新 |
| `docs-after-import.png` | 1280×800 | 导入后文档树 2 篇、第一篇自动打开，标题/正文来自文章 |
| `modal-imported-and-trashed.png` | 1280×800 | 重复导入时「已导入」禁用、「导入 0 篇」禁用；回收站文章不在候选 |
| `mobile-modal-390.png` | 390×844 | 手机弹窗：搜索独占一行、两篇已勾选、选项与分页完整、无横向溢出 |

## 5. 验证记录

- 运行：2026-11-06，Playwright 1.62.1 + Chromium（Playwright bundled）、8096 端口、隔离临时库与 `-dev` 服务；`tests/e2e/wiki-import.spec.ts` **4/4 通过**（26.1s）。
- 原始证据：`browser-evidence.json`（按用例实测值）；运行环境、通过数与产物指纹：`browser-verification.json`。
- 机械门禁：`make test`（Console `vue-tsc` + `node:test` 行为测试、Go vet/Staticcheck/全部 Go 测试、assets/module/release-source 检查）；本地合入走 `make merge-check`（feature 与 dev 组合态各跑一次 `make test`）。
- 未覆盖：生产实例与真实线上文章数据；Safari / Firefox / 真机；单次 100 篇上限的极端批量性能；正文渲染失败时的浏览器提示（Go 回归覆盖 `failed` 文案）。

## 6. 复现

```bash
# 在 feature worktree 根目录
pnpm exec playwright test tests/e2e/wiki-import.spec.ts
```

Playwright 会重建 Console（`VITE_E2E=1`）并用 `scripts/e2e/run-server.sh` 启动隔离服务；不读写本机 `data/blog.db`、`config/app.yml` 与上传目录。
