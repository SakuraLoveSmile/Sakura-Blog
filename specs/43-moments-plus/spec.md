# Spec 43 · 瞬间增强（图片九宫格 + 可见性）

- **分支**：`feature/moments-plus` · **Wave 8** · **隔离等级**：中
- **主模块**：`internal/store/moment.go`（+列）+ `internal/moment/`（前台渲染）+ `console/src/modules/moments/MomentList.vue`
- **前置**：无

## Why

Halo Moments 插件对照：本项目瞬间为纯文本简化版，缺 (1) **图片九宫格**（说说带图是核心场景）；(2) **可见性控制**（公开/私密——记录仅自己可见的碎碎念）。

## 功能需求

1. **图片**：发布框支持选图（复用 `AttachmentPickerModal`）与粘贴直传（复用 MarkdownEditor 已有直传逻辑），单条最多 9 张；前台按 1/2-4/9 宫格自适应布局，点击进 fancybox 灯箱（与文章页同套）。
2. **可见性**：发布框 +公开/私密切换；私密瞬间前台 /moments 与 /notes 近期动态均不渲染，Console 列表可见（带锁徽标）并可切换状态。
3. Console 列表：带图瞬间显示缩略图行；编辑既有瞬间可增删图。

## What Changes

- `internal/store/moment.go`：+`images`（JSON 数组文本列）+`visibility`（public/private，缺省 public）两列 ALTER 迁移 + 读写方法扩展 + `_test.go`。
- `internal/moment/moment.go`：前台查询过滤 private；模板渲染九宫格（**样式对照原版 Stellar 的 timeline/moments 相关 styl 与 main.css 已有类，禁止自创**——若 main.css 无九宫格类，对照 gallery/albums 类结构复用）。
- `internal/api/moment.go`：字段透传 + 私密项不出现在任何公开端点。
- `console/src/modules/moments/MomentList.vue`：选图/粘贴/九宫格预览/可见性切换。
- `/notes` 近期动态段：同 private 过滤（web.go momentH 消费处）。

## 实现步骤

1. store 迁移 + 过滤逻辑 + 单测（private 不出公开查询）。
2. api 字段 + Console 发布框（图片选择/粘贴/预览）。
3. 前台九宫格模板 + fancybox 接线 + 懒加载（wrapLazyloadImages 既有管线）。
4. 可见性切换 + 列表锁徽标。

## 技术决策（无需再问）

- **图片存附件库引用**：images 列存 uploads 相对路径数组，不复制文件；附件 refs 检查扩展计入瞬间引用（attachment refs 查询 +moment 源）。
- **不做视频/链接卡片**：Halo Moments 的音视频/链接预览本期不做，图文先行。
- **不做瞬间评论**：现有评论系统绑定文章/页面，瞬间不开评（原版 Stellar moments 也无评论）。
- **RSS 不含私密**：feed 若后续纳入瞬间，过滤器同源。

## Impact

- **新增**：moment 表 2 列
- **修改**：store/moment.go、moment 包模板、api/moment.go、MomentList.vue、attachment refs 查询、notes 动态段
- **不影响**：文章/页面链路、评论

## 验证

- [ ] 发布 9 图瞬间前台九宫格 + 灯箱 + 懒加载正常（1/4/9 张三种布局目检）
- [ ] 私密瞬间前台完全不可见（/moments、/notes、api 公开端点三处验证），Console 带锁可管理
- [ ] 附件引用检查能列出瞬间对图片的占用
- [ ] 旧库迁移幂等，存量瞬间缺省 public
- [ ] `make test` 全绿
