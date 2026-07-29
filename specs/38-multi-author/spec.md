# Spec 38 · 多作者体系（authors.yml 对齐）

- **分支**：`feature/multi-author` · **Wave 9**（建议 33 合后，author widget 走组件机制）· **隔离等级**：中
- **主模块**：新 `config/authors` 段 + `cmd/server/routes_taxonomy.go`（+作者页路由）+ `partial/widgets/author.html` + front matter author 字段
- **前置**：Spec 33（author widget 挂进组件编排）

## Why

原版支持 `authors.yml` 多作者数据源：文章 front matter 标 author → 文章页 author widget 显示该作者卡片 → 点击进入作者页（该作者全部文章列表）。本项目仅站长单人卡片（rightbar user-card），无作者维度。对偶尔有约稿/转载署名的场景是刚需前置。

## 功能需求（对照原版 `_data/authors.yml` + `layout/author.ejs`）

1. **作者数据源**（site.yml 新增 authors 段，形状对照原版 authors.yml）：
   ```yaml
   authors:
     sakura:            # author id
       name: xxx
       avatar: /uploads/xxx.png
       description: xxx
       url: /about/     # 可选：作者主页外链
       social: [...]    # 社交图标组，复用现有 social 结构
   ```
2. **front matter**：文章 `author: sakura`（缺省 = 站长，行为不变）。
3. **作者页**：`/author/:id`——作者卡片头 + 该作者文章列表（分页，复用首页列表卡片结构，对照原版 author.ejs）。
4. **author widget**：文章页 rightbar 作者卡按 front matter 切换显示对应作者（无 author 字段 = 站长卡，现状不变）。
5. **Console**：文章编辑器发布设置 + author 下拉（选项来自 settings 的 authors 段；空段时不显示该控件）。

## What Changes

- `config/site.go` + `site.yml`：+authors 段。
- `internal/post/parser.go`：front matter +author 字段；`internal/store/post.go` +author 列（ALTER 迁移，**读 CLAUDE.md「Step 1 新增坑点」ALTER 迁移章节**）。
- `cmd/server/routes_taxonomy.go`：+`/author/:id` 路由；store +按作者查询（新文件 `store/author_query.go`）。
- `internal/template/`：+`partial/author.html`（作者页）；author widget 模板消费作者对象。
- `internal/api/post.go` + Console PostEditor：author 字段透传 + 下拉。

## 实现步骤

1. 精读原版 authors.yml 形状 + author.ejs + author widget EJS。
2. 配置段 + front matter + store 列迁移（含迁移单测）。
3. 作者页路由/模板/分页。
4. widget 切换逻辑。
5. Console 下拉（小改动，PostEditor +1 控件）。

## 技术决策（无需再问）

- **作者存 site.yml 不入库**：作者是低频配置数据（对齐原版 _data 文件形态），文章表只存 author id 字符串。
- **未知 author id 容错**：front matter 引用不存在的 id 时回退站长卡，不报错。
- **登录体系不关联**：多作者是**展示层**概念，与 Console 单管理员登录无关（多用户管理见 Spec 45）。
- **feed 作者字段**：atom.xml 的 author 按文章作者输出（缺省站长）。

## Impact

- **新增**：authors 配置段、`partial/author.html`、`store/author_query.go`
- **修改**：parser.go、post.go（+列）、routes_taxonomy.go、author widget、api/post.go、Console PostEditor
- **不影响**：评论、备份主体（导出 front matter 自然带 author）

## 验证

- [ ] 配置 2 个作者 + 文章标 author 后：文章页作者卡切换、/author/:id 列表正确
- [ ] 未配置 authors 段时全站与现状一致（含 Console 无 author 控件）
- [ ] store 迁移幂等（旧库启动不炸，author 列缺省空）
- [ ] atom.xml author 字段随文章作者
- [ ] `make test` 全绿
