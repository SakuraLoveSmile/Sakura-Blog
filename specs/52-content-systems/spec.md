# Spec 52 · 内容系统增强（专栏 Topic + 圆角/渐变配置）

- **分支**：`feature/frontend-gap` · **Wave 10** · **隔离等级**：中
- **主模块（独占）**：`internal/store/topic.go` + `internal/api/topic.go` + `cmd/server/routes_taxonomy.go` + `internal/template/partial/topics.html` + `frontend/console/src/modules/topics/` + `config/site.go`（style 段扩展）
- **前置**：Spec 34 已合（圆角组的 card/block/image 键与 style 段注入机制复用）；现有 posts.topic 文本字段简易专栏（`/topic` + `/topic/:name` 路由）为本 spec 升级基线

## Why

差距清单剩余两项系统级能力：

1. **专栏系统（Topic）**：原版 Stellar 的专栏是独立内容组织层级（文章 front-matter 的 `topic`/`topic_cover`/`topic_description`/`topic_icon`，`/topic/` 索引页按时间线性排列、专栏页沉浸式阅读、文章页与专栏强关联）。现状仅为 posts 表一个 `topic` TEXT 字段 + 两个简易路由：无独立数据模型（图标/描述/封面只能写死占位图）、索引按名称排序（非时间线性）、文章页无专栏横幅与前后篇导航、Console 无管理入口。
2. **圆角/渐变系统配置**：main.css 的 `border-radius` 全部硬编码（16px/8px/100px 等 40+ 处）、按钮/搜索栏渐变写死；Spec 34 已落地 style 段的主题色/字体注入与 Console AppearanceTab，但圆角变量化与渐变组未覆盖。

## 功能需求

### Part A · 专栏系统

| 能力 | 现状 | 目标 |
|---|---|---|
| 数据模型 | posts.topic TEXT（无独立表） | `topics` 表（name 唯一、slug、description、icon、cover、sort、created_at、updated_at）+ posts 增 `topic_id` 列；存量 topic 文本启动时迁移回填 |
| 索引页 | `/topic` 卡片（名称排序、占位图） | `/topic` 专栏卡片按**时间线性连续排列**（最近创建的专栏在前），卡片含图标 + 名称 + 文章数 + 描述；空态文案 |
| 专栏页 | `/topic/:name` 复用首页卡片流 | `/topic/:slug` 沉浸式专栏页：专栏头（封面大图 + 名称 + 描述 + 文章数）+ 文章卡片流 |
| 文章页关联 | 无 | 文章头部专栏横幅（封面缩略图 + 专栏名链接）；正文后「上一篇 / 下一篇」按专栏内文章时间序导航；非专栏文章保持现状 |
| Console 管理 | 无 | Topics 模块：列表（名称/slug/文章数/排序）、新建/编辑/删除（软删除保护：有文章不可删）、slug 唯一性校验、文章关联（编辑文章时可选专栏） |
| API | 无 | `/api/v1/topics` CRUD + `/api/v1/posts/{id}` 增 topic_id 字段 |

### Part B · 圆角/渐变系统配置

| 配置键（settings 表 style 段） | 注入 CSS 变量 | 消费点（main.css 变量化） |
|---|---|---|
| `style.border-radius.card` | `--radius-card` | 卡片容器（post-card/article 等，现状 16px/8px 各处） |
| `style.border-radius.block` | `--radius-block` | 块级元素（note/block 等，现状 8px/4px 各处） |
| `style.border-radius.navbar` | `--radius-navbar` | 导航栏/菜单容器（现状 100px/8px） |
| `style.border-radius.image` | `--radius-image` | 图片（现状 8px/16px，含 `.img` 类） |
| `style.gradient.button` | `--gradient-button` | 按钮背景渐变（现状纯色/无渐变） |
| `style.gradient.searchbar` | `--gradient-searchbar` | 搜索栏渐变（现状写死） |

缺省值 = 现状硬编码值（零配置逐像素不变）；CSS 变量消费点一律 `var(--radius-card, 16px)` fallback 形式，未配置不注入（复用 Spec 34 head.html 注入段）。

## What Changes（逐文件）

### Part A · 专栏系统

- `internal/store/topic.go`（新）：
  - `EnsureTopicsTable()`：`CREATE TABLE IF NOT EXISTS topics (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE, slug TEXT NOT NULL UNIQUE, description TEXT NOT NULL DEFAULT '', icon TEXT NOT NULL DEFAULT '', cover TEXT NOT NULL DEFAULT '', sort INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`。
  - 迁移（`post.go` 现有 migrate 模式，对齐 L267 的 ALTER 写法）：posts 表 `ADD COLUMN topic_id INTEGER NOT NULL DEFAULT 0`；启动时把非空 `topic` 文本按 name 去重建 topics 行并回填 `topic_id`（幂等，仅在 topic_id 全为 0 时执行一次）。
  - CRUD：`ListTopics()`（按 sort DESC, created_at DESC —— 时间线性）、`CreateTopic`/`UpdateTopic`/`DeleteTopic`（有文章关联时拒绝）、`GetTopicBySlug`、`CountByTopicID`；`post.go` 增 `ListPostsByTopicID`（按 publish_at DESC）。
- `internal/api/topic.go`（新）：`/api/v1/topics` GET 列表 / POST 新建 / PUT `:id` 更新 / DELETE `:id`（403 当有关联文章）；slug 唯一性 409；对照 notebook.go 的既有 API 风格与权限中间件。
- `cmd/server/routes_taxonomy.go`：`/topic` 改用 `ListTopics()`（时间线性排序，替代现有 `sort.Slice(Name)` 与占位图标）；`/topic/:slug` 改用 `GetTopicBySlug` + `ListPostsByTopicID`；渲染专栏头数据（cover/description/count）。`/topic/:name` 兼容旧路径 301 到 slug 路径。
- `backend/internal/template/`：
  - `model.go`：`TopicItem` 扩展 `Slug/Description/Cover/CreatedAt`；`ViewData` 增 `TopicDetail`（专栏头）与 `AdjacentPosts`（前后篇）。
  - `partial/topics.html`：卡片含 cover 背景 + 描述；`partial/topic_header.html`（新）：专栏头（cover 大图 + 名称 + 描述 + 文章数）。
  - 文章页模板（`partial/content.html` 或 `post` 相关 partial）：文章头部专栏横幅（`topic_cover` 缩略 + 专栏名链接）；正文后前后篇导航（`AdjacentPosts`，仅在 `topic_id` 非空时渲染）。
- `frontend/console/src/`：
  - `api/topics.ts`（新）+ `modules/topics/TopicList.vue`、`TopicEditDialog.vue`（新）：列表（名称/slug/文章数/sort 排序）、编辑（name/slug/description/icon/cover/sort）、删除保护提示。
  - `router/index.ts`：注册 `/topics` 路由与侧栏菜单项（对齐 taxonomy 模块的注册方式）。
  - `modules/posts/PostEditor.vue`（或 EditorSidebar）：文章表单增「专栏」下拉选择（topic_id）。
- `content/posts/` 测试：为既有测试文之一补 topic 字段，`-seed` 重导验证迁移与页面。

### Part B · 圆角/渐变系统配置

- `config/site.go` + `site.yml`：style 段缺省值补充 `border-radius.{card,block,navbar,image}` 与 `gradient.{button,searchbar}`（缺省 = 现状硬编码值；settings 表 style 段 JSON 同步默认结构，配置键一致性测试扩充）。
- `partial/head.html`：注入段新增 6 个变量（`--radius-card`/`--radius-block`/`--radius-navbar`/`--radius-image`/`--gradient-button`/`--gradient-searchbar`），仅用户显式配置时输出（对齐 Spec 34 注入模式）。
- `assets/css/main.css`：**只做变量消费点改造，不重写样式** —— 将 40+ 处硬编码 `border-radius` 按所属元素归类替换为 `var(--radius-*, 原值)`；按钮渐变与搜索栏渐变消费点接 `var(--gradient-*, 原渐变)`。改动前产出「main.css 行号 → 变量」对照表（质量关键，对齐 Spec 34 步骤 1）。
- `frontend/console/src/modules/settings/AppearanceTab.vue`：新增「圆角」组（4 个数值滑杆/输入，px 输入对齐现有 pxNum 工具）与「渐变」组（2 个文本输入，CSS 渐变值校验）。

## 实现步骤

1. Part A 数据层先行：topics 表 + 迁移 + 单测（建表幂等、存量 topic 回填、slug 唯一性、删除保护）。
2. Part A API + Console：CRUD 接口 → Topics 模块页 → 文章编辑器下拉。
3. Part A 前台：`/topic` 索引改造 → `/topic/:slug` 专栏页 → 文章页横幅 + 前后篇导航。
4. Part B：产出 main.css 圆角/渐变消费点对照表 → site.go 缺省值 → head.html 注入 → main.css 变量化 → AppearanceTab 编辑组。
5. 全量回归：零配置逐像素一致（亮/暗双主题）。

## 技术决策（无需再问）

- **独立 topics 表 + topic_id 外键**：不再把专栏信息塞进 posts 文本字段（现状图标/描述无处存放）；存量 `topic` 文本字段保留（兼容查询与迁移源），新代码统一走 `topic_id`。
- **slug 与 name 双唯一**：URL 用 slug（可含 ASCII 别名），name 用于展示；`/topic/:name` 301 兼容旧链接。
- **删除保护**：有关联文章的专栏不可删除（返回 403 与提示），先解除关联才能删；不做级联删除。
- **索引按时间线性排列**：sort DESC 优先、created_at DESC 兜底（对齐任务要求「按时间线性连续排列」，替代现状名称排序）。
- **圆角/渐变与 Spec 34 的关系**：34 已落地 style 段注入管线与 card/block/image 圆角键骨架，本 spec 增量 = navbar 圆角键 + gradient 组 + main.css 全量变量化 + AppearanceTab 编辑 UI（34 明确把 Console UI 留给后续批次，本 spec 承接）；若 34 尚未合，按本 spec 全量实现。
- **CSS 变量 fallback 铁律**：全部消费点 `var(--x, 原硬编码值)`，未配置不注入任何变量（零配置零差异）。

## Impact

- **新增**：`internal/store/topic.go`、`internal/api/topic.go`、`partial/topic_header.html`、`console/src/api/topics.ts`、`console/src/modules/topics/`、测试数据
- **修改**：`store/post.go`（topic_id 列 + 迁移 + 查询）、`routes_taxonomy.go`、`template/model.go`、`partial/topics.html`、文章页模板、`console/src/router/index.ts`、`modules/posts/PostEditor.vue`、`config/site.go`/`site.yml`、`partial/head.html`、`assets/css/main.css`（仅变量消费点）、`modules/settings/AppearanceTab.vue`
- **不影响**：shortcode、comment、search、backup 等模块（迁移仅加列，不动既有数据）

## 验证（验收清单）

- [ ] `go build ./... && go vet ./...` 通过；`make test` 全绿（含 topics 单测与配置键一致性测试）
- [ ] 存量文章带 `topic` 文本：`-seed` 重导/启动迁移后自动建 topics 行并回填 topic_id（幂等，二次启动不重复）
- [ ] `/topic` 索引：按时间线性排列（sort 高优先），卡片显示图标/名称/描述/文章数；空态显示空文案
- [ ] `/topic/:slug`：专栏头（封面 + 描述 + 文章数）+ 文章卡片流按时间倒序；旧 `/topic/:name` 301 跳转
- [ ] 文章页：专栏文章显示头部横幅与前后篇导航（按专栏内时间序），非专栏文章无横幅
- [ ] Console：Topics CRUD 全流程（含 slug 冲突 409、删除保护 403）；文章编辑器可关联/解绑专栏
- [ ] 圆角/渐变：零配置时全站与现状逐像素一致（亮/暗）；四档圆角 + 两组渐变改配置可见生效；AppearanceTab 保存后立即生效（热更新）
- [ ] 前后篇导航在 PJAX 切页后正常渲染（复用现有文章页导航模式）

## Out of Scope

- 专栏的订阅/关注、评论聚合、RSS 按专栏分组
- 专栏封面图上传（复用既有 media 库，不新建上传通道）
- 圆角按组件细粒度配置（如 badge/paginator 单独键，维持 Spec 34 五档之外的统一变量）
- 渐变动画（如 hover 渐变过渡动效）
