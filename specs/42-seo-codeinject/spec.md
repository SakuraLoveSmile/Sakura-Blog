# Spec 42 · SEO 设置 + 代码注入

- **分支**：`feature/seo-codeinject` · **Wave 8** · **隔离等级**：中
- **主模块**：`console/src/modules/settings/SettingsView.vue`（+2 Tab）+ `internal/api/settings.go` + `partial/head.html` / `layout/base.html`（消费点）+ PostEditor（文章级 SEO）
- **前置**：无（与 39/40 无文件交集；同批次若有其他 settings 改动需协调 Tab 追加顺序）

## Why

Halo 对照的两个系统设置缺失项：(1) **SEO 设置**——站点级（默认 keywords/description、robots 附加规则、搜索引擎收录开关）与文章级（自定义 meta description/keywords，覆盖自动摘要）均无；(2) **代码注入**——无自定义 head/footer HTML 片段入口（站长挂统计脚本/自定义 CSS/验证 meta 的标配能力）。

## 功能需求

### A. 站点级 SEO（settings +「SEO」Tab）
| 项 | 行为 |
|---|---|
| 默认 description/keywords | head.html meta 输出（文章级未配置时的回退值；再回退现状 site.description） |
| robots 附加规则 | 追加到 /robots.txt 输出（feed 包 robots 生成处拼接） |
| 收录开关 | 关闭时全站输出 `<meta name="robots" content="noindex">`（建站期防收录） |
| 站长验证 meta | google/bing/baidu 验证码字段，输出对应 meta 标签 |

### B. 文章级 SEO（PostEditor 发布设置 + 折叠「SEO」区）
- meta_description、meta_keywords 两字段（store post 表 +2 列，ALTER 迁移）；head.html 文章页优先取用。

### C. 代码注入（settings +「代码注入」Tab）
- `inject_head` / `inject_footer` 两个多行文本框；分别原样输出在 head.html `</head>` 前与 base.html `</body>` 前。
- 仅登录管理员可写（既有鉴权覆盖）；输出**不经 bluemonday**（注入本意即执行脚本），Tab 内显著提示安全责任。

## What Changes

- `internal/config/`：site 配置 +seo/inject 段（入库热更新走 reload.go 既有机制，配置键一致性测试扩充）。
- `internal/api/settings.go`：+seo、inject 分段读写。
- `internal/store/post.go`：+meta_description/meta_keywords 列（ALTER 迁移，读 CLAUDE.md 迁移坑点）。
- `partial/head.html`：meta 输出优先级（文章级 → 站点级 → 现状回退）+ 验证 meta + noindex + inject_head。
- `layout/base.html`：+inject_footer 输出点。
- `internal/feed/`：robots 附加规则拼接。
- Console：SettingsView +2 Tab；PostEditor +SEO 折叠区；api/index.ts 字段透传。

## 实现步骤

1. 配置段 + settings API 分段 + 一致性测试。
2. head/base 消费点（含优先级逻辑单测——放 template model 测试）。
3. post 表迁移 + PostEditor SEO 区。
4. robots 拼接 + 验证 meta + noindex。
5. 代码注入 Tab + 输出点 + 冒烟（注入 `<script>console.log(1)</script>` 前台可见执行）。

## 技术决策（无需再问）

- **注入内容入 DB 不入 site.yml 文件**：走 settings 入库 + 热更新路径，备份随 DB 自然覆盖。
- **不做 per-page 注入**：只有全局两个注入点（Halo 基础形态）；页面级需求出现再扩展。
- **JSON-LD 不动**：Spec 28 已有的 JSON-LD 输出保持，SEO Tab 不重复管理。
- **keywords 尽管权重低仍实现**：对齐 Halo 功能面，成本一行 meta。

## Impact

- **新增**：seo/inject 配置段、post 表 2 列
- **修改**：settings.go、head.html、base.html、feed robots、SettingsView、PostEditor、store/post.go
- **不影响**：评论、备份、搜索

## 验证

- [ ] 文章级 description 覆盖站点级，未配置时回退链正确（curl 抽查 meta）
- [ ] noindex 开关生效；三家验证 meta 输出
- [ ] robots.txt 附加规则出现
- [ ] inject_head/footer 注入的脚本前台执行；清空后消失
- [ ] 旧库迁移幂等
- [ ] `make test` 全绿（含配置键一致性）
