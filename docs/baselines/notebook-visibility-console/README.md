# BLG-20261107-01 验收基线：笔记本在前台可见与 Console 优化

对应计划记录：[docs/plans/notebook-visibility-console.md](../../plans/notebook-visibility-console.md)（状态：已验收）。

本目录的截图和 `verification.json` 由同目录的 `capture-notebook.mjs` 对两个隔离沙箱重新生成，生成时间见 `verification.json` 的 `generated_at`。

- 浏览器：本机 Chromium（Playwright），桌面 1280×860 浅色、手机 390×844 深色，设备像素比 2。Console 始终是深色主题，公开页面跟随 `prefers-color-scheme`，所以桌面截图里 Console 是深色、前台是浅色。
- 沙箱 A：已初始化 Stellar 2.0 的站点，里面有笔记本 `acceptance` 和已发布笔记 `responsive-check`（按用户旧库里那条测试笔记的内容，通过 Console 新界面创建）。
- 沙箱 B：旧版布局，旧 `notebooks`/`notes` 表仍在、没有 Stellar 2.0 快照，即用户真实数据库迁移前的状态。

## 截图索引

每张图都有 `-1280`（桌面）和 `-390`（手机）两个版本。

| 文件前缀 | 沙箱 | 内容 |
| --- | --- | --- |
| `console-notebooks-list` | A | 笔记本列表：每个笔记本一张卡片，显示“1 篇笔记 · 1 篇已发布”，有“管理笔记 / 编辑 / 在博客中查看” |
| `console-notes-list` | A | 笔记列表：标题、已发布状态、Slug、更新时间、标签、“在博客中查看 / 编辑”，工具栏在窄屏自动换行 |
| `console-notebook-editor` | A | 笔记本编辑：ID（只读）、名称、描述、排序、图标、封面都是结构化字段，JSON 原文收进折叠的“高级”面板 |
| `console-note-editor` | A | 笔记编辑：标题、Slug、标签（每行一个）、折叠的 Front matter 原文、富文本正文，页头有“在博客中查看” |
| `public-notebooks-index` | A | 博客前台 `/notebooks`：笔记本总索引 |
| `public-notebook` | A | 博客前台 `/notebooks/acceptance`：笔记本首页 |
| `public-note` | A | 博客前台 `/notebooks/acceptance/responsive-check`：这条测试笔记在博客里的样子（标题、“检查项”小节、三条列表、标签、许可协议、评论区、目录）。页头的“更新于 N 分钟前”随生成时间变化 |
| `legacy-public-409` | B | 旧版布局下浏览器访问前台：HTTP 409 的“站点内容正在升级”说明页，取代原来的裸 JSON |
| `legacy-console-notebooks` | B | 旧版布局下 Console 的笔记本页：用迁移说明代替红色后端原文，原始报错收在默认折叠的“技术详情”里 |

## 可复核的事实（`verification.json`）

15 项检查全部通过，0 项失败：

- 沙箱 A 里 Console 给出的每个“在博客中查看”链接（`/notebooks/acceptance`、`/notebooks/acceptance/responsive-check`，桌面与手机各取一次）都是站内路径，请求返回 HTTP 200。
- 前台笔记页能看到“检查项”标题（桌面、手机各一项）。
- 沙箱 B 的前台页面对浏览器返回 HTTP 409，标题是“站点内容正在升级”（桌面、手机各一项）；Console 显示迁移说明且原始后端文字默认不可见（桌面、手机各两项）；不带浏览器 `Accept` 的请求仍拿到 `stellar_migration_required` 的 JSON。
- 手机端本任务自己的控件高度都 ≥44px；共享组件（富文本工具栏、图标/附件选择器）沿用各自既有尺寸，在 `touch_targets` 的 `shared` 里单独列出：笔记本编辑 4 个、笔记编辑 15 个，不计入判定。
- 检查过的 10 个“页面 × 视口”组合都没有横向溢出。

## 复现

前提：工作树里已执行 `pnpm install`，并装好 Playwright 的 Chromium；先 `make build` 生成真实的 Console 与 User Center 产物（`-dev` 沙箱从磁盘读取它们）。沙箱根目录放在被忽略的 `.tmp/` 里，端口只监听 `127.0.0.1`。以下命令都在仓库根目录执行。

### 沙箱 A（已迁移站点）

```bash
ROOT="$PWD/.tmp/sandbox-a"
rm -rf "$ROOT" && mkdir -p "$ROOT/config" "$ROOT/content/posts" "$ROOT/data"
ln -s "$PWD/modules" "$ROOT/modules" && ln -s "$PWD/internal" "$ROOT/internal"
go run ./scripts/e2e/prepare.go -db "$ROOT/data/blog.db"
BLOG_ROOT="$ROOT" BLOG_DB="$ROOT/data/blog.db" BLOG_LISTEN="127.0.0.1:8197" BLOG_AUTO_BACKUP=false \
  BLOG_MAINTENANCE_LOCK="$ROOT/data/maintenance.lock" \
  go run ./cmd/blog -root "$ROOT" -config "$ROOT/config/app.yml" -dev
```

用夹具管理员（账号 `admin`，密码见 `scripts/e2e/prepare.go`）登录 `http://127.0.0.1:8197/console/`，在 Console 里依次创建：

1. 笔记本：ID `acceptance`，名称“1.40.0 验收笔记本”，描述“Console 与前台功能验证数据”，排序 `10`。
2. 笔记：标题“响应式检查记录（已更新）”，Slug `responsive-check`，标签 `responsive`、`console`、`1.40.0`（每行一个），正文（切到 Markdown 模式输入）为 `## 检查项` 加 `- 375px`、`- 768px`、`- 1440px` 三条列表，点“发布”。

### 沙箱 B（旧版布局，尚未迁移）

```bash
ROOT="$PWD/.tmp/sandbox-b"
rm -rf "$ROOT" && mkdir -p "$ROOT/config" "$ROOT/content/posts" "$ROOT/data"
ln -s "$PWD/modules" "$ROOT/modules" && ln -s "$PWD/internal" "$ROOT/internal"
sqlite3 "$ROOT/data/blog.db" < docs/baselines/notebook-visibility-console/legacy-fixture.sql
go run ./scripts/e2e/prepare.go -db "$ROOT/data/blog.db"
sqlite3 "$ROOT/data/blog.db" "DELETE FROM settings WHERE section IN ('stellar_v2','blog_extensions');"
BLOG_ROOT="$ROOT" BLOG_DB="$ROOT/data/blog.db" BLOG_LISTEN="127.0.0.1:8198" BLOG_AUTO_BACKUP=false \
  BLOG_MAINTENANCE_LOCK="$ROOT/data/maintenance.lock" \
  go run ./cmd/blog -root "$ROOT" -config "$ROOT/config/app.yml" -dev
```

顺序不能换：旧表必须先于 `prepare.go` 灌进全新的库，否则新布局的表已经存在，灌入会失败。

### 生成截图与 `verification.json`

```bash
E2E_ADMIN_PASSWORD='<scripts/e2e/prepare.go 里夹具管理员的密码>' \
  node docs/baselines/notebook-visibility-console/capture-notebook.mjs docs/baselines/notebook-visibility-console
```

脚本只读沙箱；沙箱地址可用 `SANDBOX_A`、`SANDBOX_B` 环境变量改写。任何检查失败时退出码非 0，`verification.json` 仍会写出。

## 边界

- 交付时留给用户体验的沙箱 B 是用用户旧库里那条测试数据的拷贝建的；本目录的 `legacy-fixture.sql` 是合成的等价数据，只为让复现不依赖用户真实数据库。它已在一个临时沙箱上用上面的步骤验证过：浏览器得到 409 说明页、API 客户端得到 JSON、旧表原样保留、没有 `stellar_v2` 分区。`legacy-*` 截图只显示迁移说明，不渲染任何笔记内容，所以与具体行数据无关。
- 前台截图来自沙箱 A 里按旧数据内容重新创建的笔记（笔记本 ID `acceptance`，已发布），不是对用户真实数据库迁移后的结果。按 `docs/stellar-200-migration-preview.md` 里用户已确认的决定，真实迁移后这条笔记是草稿、slug 为 `note-2`，所属笔记本的公开路径是 `/notebooks/2`，要在 Console 里发布后前台才会显示。这是按文档推算的预期，迁移没有被运行过；旧标签 `responsive,console,1.40.0` 的转换也要等迁移预览与执行后才能确认。
- 沙箱 A 的搭建没有在清空环境里完整重放：它与沙箱 B 共用同一套 `prepare.go` 和启动命令（沙箱 B 已按上面的步骤重放验证过），创建笔记本与笔记那一步是手工界面操作，没有脚本化。
- 只在本机 Chromium 验证；没有 Safari、Firefox、真机触屏或读屏器的证据。
- 手机端 44px 的判定只覆盖本模块自己的控件，共享控件沿用既有尺寸（见上）。
- 用户真实的 `data/blog.db` 没有被写入，也没有对它启动过服务。
