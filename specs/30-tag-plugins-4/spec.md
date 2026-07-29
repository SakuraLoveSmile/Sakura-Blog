# Spec 30 · 标签插件四期（数据服务类 4 件套 + 行内标签）

- **分支**：`feature/tag-plugins-4` · **Wave 8** · **隔离等级**：高
- **主模块（独占）**：`internal/post/shortcode.go` + `assets/css/tag-plugins.css`（追加式）
- **前置**：无（JS 服务与部分后端能力已就绪，见下）

## Why

差距调研确认原版 42 个标签插件中本项目已实现 29 个，缺失项里有 4 个属于「JS 服务已带、只差 shortcode 入口」的低成本高收益项：**vote**（`assets/js/services/vote.js` 与 `scripts/test/vote-service.test.mjs` 均已存在）、**friends/users**（`services/friends.js` 已带）、**sites**（`services/sites.js` 已带）、**radio**（checkbox 的变体，正则一行扩展）。另补 **inline-labels 行内标签语法**（原版 `scripts/tags/inline-labels.js`）。

## 功能需求（语法对照原版 `scripts/tags/lib/*.js` 与 `inline-labels.js`）

| 插件 | 语法 | 说明 |
|---|---|---|
| vote | `{% vote id [yes:icon] [no:icon] [title] %}` | 输出 `.tag-plugin.ds-vote`，`data-api` 指向本地投票端点，`data-id` 去重键；up/down 按钮 + 计数 |
| friends | `{% friends [api:url] %}` | 友链卡片流，`data-api` 缺省指向本地 `/api/friends`（复用 friends 聚合数据），由 friends.js 渲染 |
| sites | `{% sites [api:url] %}` | 站点卡片流，交给 sites.js 渲染；数据源配置对照原版 `data_services.sites` |
| radio | `{% radio [checked] [color:x] text %}` | reCheckbox 正则扩展为 `checkbox\|radio`，radio 输出圆形样式（main.css 已有类则直接用） |
| 行内标签 | `[[text]]`、`{color\|text}` 等（以原版 inline-labels.js 实际注册为准） | goldmark 渲染前的行内替换，与 shortcode 同管线 |

## What Changes

- `internal/post/shortcode.go`：+4 正则与 render 函数（vote/friends/sites 行内标签形态；radio 改造现有 reCheckbox）+ inline-labels 处理段。
- **vote 后端数据服务**（对照 Spec 20 的 rating 模式，小而独立）：
  - `internal/store/vote.go`（新文件）：`votes` 表（target_id, value up/down, voter_hash, created_at），同 hash 同 target 覆盖更新。
  - 公开路由 `GET /api/vote/info?id=` 与 `POST /api/vote/update?id=&value=`——**先精读 `assets/js/services/vote.js` L51-L88 已固化的 `${api}/info` 与 `${api}/update` 契约**，端点形状必须与之匹配（挂 `internal/api/router.go` 公开段，同 `/api/rating` 旁）。
- `assets/css/tag-plugins.css`：先 grep `main.css` 确认 `.ds-vote`/`.ds-friends`/`.ds-sites`/radio 类是否已存在，**已存在不得重复自定义**，仅补缺失类（对照原版 `source/css/_tag-plugins/`）。
- `partial/head.html` 或按需注入：vote/friends/sites 页面按需加载对应 services js（对齐现有 mermaid/swiper 按需模式）。

## 实现步骤

1. 精读原版 vote.js / friends.js / sites.js / checkbox.js / inline-labels.js，记录输出 HTML 与 class。
2. radio：改 reCheckbox 正则 + 渲染分支（半小时级）。
3. vote：store 表 + 公开 API + shortcode + 按需 js 注入；用现有 `scripts/test/vote-service.test.mjs` 验证前端行为不回归。
4. friends/sites：shortcode 输出容器 + data-api 缺省值决策（见技术决策）+ 按需 js。
5. inline-labels：确认原版实际语法集后实现，测试文覆盖。
6. 测试文 `content/posts/tag-plugins-4-test.md` 覆盖全部项，`-seed` 重导验证。

## 技术决策（无需再问）

- **vote 自建后端**：不接原版 star-vote 外部服务；voter_hash = IP hash（与 rating 同策略），端点契约以本项目 vote.js 已写死的 `/info`、`/update` 形状为准。
- **friends 标签数据源**：缺省 `data-api` 指向本地友链聚合端点；若 friends.js 期望的 JSON 形状与本地端点不一致，在 shortcode 层加 `data-adapter` 或改 js 适配（改 js 需保 `scripts/test` 通过）。
- **sites 无本地数据源**：`api:` 参数必填透传，不建本地站点库（原版即外部 API 驱动）。
- **语法容错**：参数缺失原样输出（与 06/16/20 同标准）。

## Impact

- **新增**：`internal/store/vote.go`、测试文
- **修改**：`internal/post/shortcode.go`、`tag-plugins.css`、`internal/api/router.go`（+2 行公开路由）、head 按需注入
- **不影响**：console、post/page store 主体

## 验证

- [ ] `{% vote %}` 渲染 + 投票 + 刷新后计数保持 + localStorage 去重生效
- [ ] `{% radio %}` 与 `{% checkbox %}` 并存渲染正确
- [ ] `{% friends %}`/`{% sites %}` 卡片流渲染（friends 走本地数据）
- [ ] 行内标签在正文中生效且不误伤代码块
- [ ] `make test` 全绿（含 vote-service.test.mjs）
