# Spec 62 · 优化收尾轮（后端性能/正确性收尾 + Console 数据链路/错误处理/编辑器体验 + 测试补强）

- **分支**：`feature/opt2-backend` → `feature/opt2-console` → `feature/opt2-editor` → `feature/opt2-tests`（4 个 feature worktree **顺序推进**：一个合回 dev 后再开下一个，全程 rebase 最新 dev）
- **主模块**：
  - 后端：`backend/internal/{api,store,comment,notify,feed,backup,moment}`、`backend/cmd/server`
  - 前端：`frontend/console/src`（api、components、modules/dashboard、modules/posts、modules/settings、composables、lib、tests）、`frontend/assets/js/{services,plugins}`（死文件删除）
- **前置**：Spec 61（optimization-round，已合回 dev）
- **冲突**：四阶段文件面互不交叉（阶段 1 后端 + 死 assets，阶段 2/3/4 console），顺序推进，无需冲突矩阵

## Why

Spec 61 优化轮（前端体积 -90.5%、构建流程、安全加固、密码迁移、后端性能）之后，对全仓做了一轮「收益最大、风险最低」的审查（后端 205 个 Go 文件 + console 111 个源文件 + 前台 assets + 仓库卫生），发现三类问题：

1. **Spec 61 半成品**：笔记优化（`CountNotesByNotebooks`/`ListNotesByNotebookTag`/`GetPrevNote`/`GetNextNote`）只写了 store 层，路由未接线，优化实际未生效；
2. **console 真实缺陷**：Dashboard 每次进入 4 次（可至 6 次）重复请求 `/dashboard/stats` 且 6 个 widget 裸 fetch 绕过 axios（会话过期静默卡死）；确认弹窗 API 失败零反馈（dialog.ts 无 catch）；编辑器大文档卡顿（每键全量序列化、预览无 in-flight 守卫）；两处一行级 CSS bug；3 处图标走 Iconify CDN 运行时请求（内网/离线缺失）；保存后列表 10s 内显示旧数据；
3. **一批已定位的后端 N+1/全量拉取/数据竞争**：listPosts 全表+内存过滤+逐篇 GetViews、dashboardStats 全表数数、contentCache dirty 无锁读写、评论通知 goroutine 无上限、handleWiki N+1 等。

本轮按 4 阶段顺序推进收尾，每阶段独立验证、独立合回；外加一节「运营收尾」（非代码，需用户逐项授权）。

## 执行模型

- 每阶段开工前查 `CLAUDE.md` 对应坑点章节（阶段 1 动 frontend/assets 删除死文件 → 「静态资源版本戳 bump 规范」：**纯删除无引用变化，无需 bump**；若顺带修改任何仍被引用的 assets 文件，必须同步 bump 对应 `?v=`）；阶段 2/3 动 PageList/SettingsView → 「Spec 60」的 router TDZ 坑（分段/module 逻辑必须在 useRouter/useRoute 声明之后）。
- 每阶段 `make merge-check` 全绿才合回 dev，合完即删分支；验证产物写入各自 worktree 的 `.tmp/`，合回后提示用户单独授权清理。
- 各阶段独立提交，中文提交信息，格式 `类型: 描述`。

---

## 阶段 1 · 后端性能与正确性收尾（feature/opt2-backend）

### 改动点

1. **views 入 postCols + listPosts SQL 下推**（`internal/store/post.go:154`、`internal/api/post.go:143-188`）：`postCols` 增加 `views` 列（`Post` 补 Views 字段）；listPosts 的 status/category/tag 过滤与 LIMIT/OFFSET 下推 SQL（tag 用逗号包裹 LIKE，与现有查询口径一致），删除逐篇 `GetViews` 循环；连带 `post.go:203/295/392` 三处详情/更新路径各少一次查询。
2. **dashboardStats COUNT 化**（`internal/api/dashboard.go:25-29`）：`List()` 全量加载（含 content_md/content_html 大字段）只为数数，改 `CountPosts()`。
3. **notebook 三方法接线 + note_count 下放**（`cmd/server/routes_taxonomy.go:489/550/648` 与 notebook 列表 API）：路由改调 `CountNotesByNotebooks`（GROUP BY）、`ListNotesByNotebookTag`、`GetPrevNote/GetNextNote`（store 层 Spec 61 已实现、已有单测），消灭 2 处 N+1 与 2 处全量取回；console 笔记本列表 API 响应带 `note_count`（复用 CountNotesByNotebooks，前端消费见阶段 3）。
4. **contentCache dirty 数据竞争修复**（`cmd/server/content_cache.go:41-133`）：search 与 sitemap 两处缓存的 `dirty bool` 改 `atomic.Bool`（invalidate 无锁写、get 无锁读是真实 data race）。
5. **评论通知并发上限 + SMTP 超时**（`internal/comment/handler.go:301/314`、`internal/notify/service.go`、`internal/comment/notify.go:116-157`）：notify goroutine 加信号量/worker 池（并发 ≤5）；SMTP `tls.Dial` 与 `smtp.SendMail` 会话加整体 deadline（复用 webhook 5s 超时模式）。
6. **aiGuestHits 惰性清理**（`internal/api/ai.go:380-403`）：仿 loginBlocked/pending2FA 惰性清理模式，过期 IP 条目顺带清除（当前 map 无界增长）。
7. **查询收口一批**：listComments 父评论改 `IN` 批量查询（`internal/api/comment.go:84`）；fillComments 计数改 `len()` 派生（`cmd/server/web.go:548-553`）；handleWiki 每项目循环 ListWikidocs 改单次查询（`routes_taxonomy.go:314`）；listShares 与 adminSearch 改轻量列（`internal/api/share.go:66`、`internal/api/search.go:64-78`）；api/wiki 改 `BuildDocTreeFromDocs` 去双查（`internal/api/wiki.go:15-20`）。
8. **公共代码去重与语义统一**：`parsePublishAt` 两处实现合并并统一时区语义（`internal/store/post.go:174` 按 UTC、`internal/api/post.go:595` 按 Local——取其一并注释说明）；`stripHTML/truncateText` 三处复制抽公共包（`cmd/server/web.go:29-46`、`internal/api/helpers.go:8-26`、`internal/moment/moment.go`）。
9. **DB/备份文件权限收紧**（`internal/store/post.go:247-257`、`internal/backup/restore.go:163`）：创建后 `chmod 0600`（含 -wal/-shm 与备份文件）。
10. **死 JS 资产删除**（`frontend/assets/js/services/` 9 个 + `frontend/assets/js/plugins/` 4 个，共 53KB）：fcircle/giscus_latest_comment/memos/rss/siteinfo/twikoo_latest_comment/waline_latest_comment/weibo/artalk_latest_comment.js 与 copycode/download-file/video/voice.js——已核实无任何模板/注册表/动态加载引用（base.html 服务注册表仅 10 项、plugins 仅加载 heti-init/preload/scrollreveal-init），删除后二进制与 /assets 同步瘦身（`//go:embed all:js` 无需改）。
11. **写路径吞错补日志**：`internal/api/post.go:433/387/389/448-456`、`cmd/server/search_index.go:113/117`、`cmd/server/main.go:351` 的忽略错误至少记日志（版本快照失败为告警级）。
12. **测试补强**：feed 包（Atom/JSONFeed 结构、password 保护文章正文不泄漏、Cache-Control 头）；seed 包（Import 幂等跑两遍不重复、frontmatter draft 语义、Pinned 映射——曾修两次的 bug 点）；content_cache 置脏-重建并发/竞态断言；listPosts 筛选/分页行为；dashboardStats 不再全表加载（SQLite trace 或 mock 断言）。

### 验收清单

- [ ] `make test` + `go test -race ./backend/...` 全绿
- [ ] SQLite trace 验证 listPosts 单次查询（无逐篇 GetViews）；dashboardStats 无大字段加载
- [ ] 笔记三路由（notebooks 列表/tag 过滤/上下篇）行为与优化前一致（单测 + 页面比对）
- [ ] `-race` 下反复保存设置触发 invalidate 无竞态报错；评论风暴下 notify 并发受限（日志验证）
- [ ] 密码保护文章不出现在 feed 输出（新单测断言）
- [ ] 删除 13 个 JS 文件后 `make build` 成功、前台浏览 3 类页面（首页/文章/归档）控制台零报错
- [ ] 新建 DB 与备份文件权限 0600（ls -l 验证）

---

## 阶段 2 · Console 数据链路与错误处理闭环（feature/opt2-console）

### 改动点

1. **Dashboard 请求去重 + 接入 axios**（`src/modules/dashboard/widgets/StatCardWidget.vue:30-38` 等 6 个 widget）：StatCardWidget 改共享 `useDashboardStats()`（现为零引用的死代码，转活）或统一 `useQuery({queryKey: queryKeys.dashboard.stats()})` 天然去重；其余 5 个 widget 裸 fetch 改走 `dashboardApi/statsApi`（`/stats/hourly` 补进 statsApi）；删除 `src/stores/dashboard-editor.ts`（整文件死代码）。
2. **错误处理闭环**：`src/components/ui/dialog.ts:44-54` 补 catch + Toast.error（失败不关弹窗、有提示）；`PostList.vue:194-200`、`ShareList.vue:52-58`、`NotificationsView.vue:124-138` 等 useMutation 补 onError；`SeoOverview/SeoTraffic/SeoCrawlers` 三页补 try/catch 空态；QueryClient 加默认 onError 兜底。
3. **query key 工厂统一 + 保存后失效**（`src/api/query-client.ts`）：工厂补 users key；28+ 处字面量 invalidate 统一走工厂；`PostEditor.vue:297-345` handleSave/handlePublish 后 `invalidateQueries(posts)`（修「保存后列表 10s 内显示旧标题/草稿绿点失效」）。
4. **死代码清理**：`src/components/CollapsibleSection.vue`（被 modules/pages 同名遮蔽）、`src/components/ui/VSpace.vue`（仅 re-export）、`src/modules/notes/` 空目录、`UserListView.vue:227-232` 无 @click 的导出按钮（删除或补逻辑）。
5. **一行级修复包**：VModal.vue:81 动效 `-out` 拼接 bug（`var(--ease-standard)-out` 非法值）改规范写法 + 补 `.modal-zoom-leave-active`；DefaultLayout.vue:490/540 遮罩 `bg-secondary0/75` → `bg-black/60`（修抽屉/⌘K 无遮罩压暗）；Login.vue:212/286 与 plugins-data.ts:210/212/754-755 共 3 处非 ri 图标换 ri 内图标或子集化（去 Iconify CDN 运行时请求）；删除 `main.ts:14` jetbrains-mono-500 import；Login.vue:34-36 文案去 emoji（符合 UI 规范）；`PageList.vue:102-104` useQuery 加 `enabled: module==='pages'`；4 处列表图片补 `loading="lazy"`（PostList:454/616、FriendList:279、CommentList:495、UserListView:492/587）；铃铛未读数改 useQuery + refetchInterval（DefaultLayout.vue:22-36 与 NotificationsView 双数据源统一，已读后角标即时更新）。

### UI 验收边界

- **参照图路径**：`baselines/phase2-pre-mask.png`、`baselines/phase2-pre-modal.png`、`baselines/phase2-pre-login.png`（修复前基线截图，开工时截取入库；与修复后截图对比）
- **验收清单**（每项一个可观察判据）：
  - [ ] ⌘K 搜索与「更多」抽屉打开时背景有半透明遮罩压暗（修复前截图对比：修复前无压暗）
  - [ ] 弹窗出现/关闭有缩放+淡出动画（DevTools 检查 computed transition 生效，非 fallback ease）
  - [ ] Login 第三方登录气泡文案无 emoji；断网（Network 离线）下登录 spinner 与 icon/vote 标签图标正常渲染、无第三方请求
  - [ ] 验收完成后截图存入 `baselines/` 并更新本节勾选状态

### 验收清单

- [ ] Network 面板：进 Dashboard 首屏 stats 请求 1 次（不再 4 次）；全页无裸 fetch 调用
- [ ] 会话过期进 Dashboard 正确跳登录（不再静默卡死）
- [ ] 删除确认弹窗在 API 失败时：弹窗保持打开 + 错误 Toast 出现 + 无 unhandled rejection（console 验证）
- [ ] 编辑文章保存/发布后返回列表立即显示新标题与草稿状态（无需等 10s）
- [ ] `npm run build` 通过；dist 无 jetbrains-mono-500 woff2
- [ ] `make test` 全绿

---

## 阶段 3 · 编辑器体验与列表页骨架（feature/opt2-editor）

### 改动点

1. **RichTextEditor 序列化防抖**（`src/components/RichTextEditor.vue:127-136`）：onUpdate 内 getMarkdown+emit 做 150-300ms 防抖或 rAF 合并（现有 baseline+lastMarkdown 防循环机制保留）；保存/草稿路径同步处理。
2. **MarkdownEditor 预览 in-flight 守卫**（`src/components/MarkdownEditor.vue:121-139,194-200`）：响应加序号守卫（丢弃过期响应，修慢响应覆盖新内容）；文档超阈值（如 50KB）提高防抖至 800ms。
3. **ContentEditor 编辑器偏好统一 + tiptap 按需挂载**（`src/components/ContentEditor.vue:36-37`、`WikiDocsView.vue:604`、`NotebooksView.vue:657`）：localStorage key 与 PostEditor 统一（或默认 markdown）；RichTextEditor 改 `defineAsyncComponent` 按编辑器类型挂载时加载（Markdown-only 用户不下载 190KB gzip tiptap chunk）。
4. **SettingsView tab 懒加载**（`src/modules/settings/SettingsView.vue:6-13`）：8 个 tab 组件改 `defineAsyncComponent`（注意 Spec 60 的 router TDZ 坑不影响本节）。
5. **模板内重计算改预计算**（`PostList.vue:497-499,655-657` 的 parseTags 每行每渲染 2 次、`:441-447,600-607` 的 checkSelection O(n)×3 等热列表）：computed Map 预格式化 + selectedSlugs 换 Set；CommentList/MomentList 的 avatarInitial/formatRelative 同法处理。
6. **index key 修复**（`MenubarEditor.vue:103`、`NavTabsTab.vue:45`、`SettingsView.vue:667/689/756`、`AuthorsTab.vue:81`、`CoverTab.vue:50`）：可增删/排序列表改行内唯一字段作 key（修输入焦点/值错位）。
7. **SettingsView deep watch 降级**（`SettingsView.vue:500`）：整站配置对象 deep watch 改按字段路径 watch 或预览降频，消除 AuthorsTab 每次按键级联触发整站 POST /preview/site（`AuthorsTab.vue:39-45,81` 增量写）。
8. **NotebooksView 计数聚合 + SubscriptionList 分页**（`NotebooksView.vue:37-46`、`SubscriptionList.vue:54-67`）：前端消费阶段 1 的 note_count（21 请求 → 1）；订阅文章传 page 参数。
9. **usePagedList 骨架抽取（示范 2 页）**：抽 `src/composables/usePagedList.ts` + `useConfirmDelete()`，迁移 PostList 与 ShareList 两页作示范（分页/300ms 防抖/刷新/删除确认样板收口），其余 6 页留后续轮次。
10. **顺手项**：CommentList 搜索改走后端 keyword 参数（`CommentList.vue:94-101`，修「只搜当前页」缺口，后端 LIKE 已支持）。

### UI 验收边界

- **参照图路径**：`baselines/phase3-pre-list.png`（迁移前 PostList 基线截图，含分页/搜索/删除确认三态）
- **验收清单**：
  - [ ] PostList 迁移 usePagedList 后：分页/搜索防抖/刷新/删除确认行为与基线一致（逐项对照）
  - [ ] 设置页切 tab 时对应 chunk 按需加载（Network 面板）；首次进入设置页体积下降（前后留档对比）
  - [ ] 长文档（≥50KB）编辑时输入不卡顿、预览不串文（本地可观察）
  - [ ] 验收完成后截图存入 `baselines/` 并更新本节勾选状态

### 验收清单

- [ ] `npm run build` 通过；tiptap chunk 仅在编辑器路由且富文本挂载时请求
- [ ] 设置编辑器偏好 markdown 后，进文库/笔记编辑不下载 tiptap chunk（Network 验证）
- [ ] 20 个笔记本页 Network 请求数 21→≤2
- [ ] 可排序列表（菜单/导航 tab）增删/拖动后输入值与顺序不错位
- [ ] `make test` 全绿

---

## 阶段 4 · 前端测试与格式化工具（feature/opt2-tests）

### 改动点

1. **`src/lib/format.ts` 抽取与统一**：日期/相对时间/字节格式化 7-10 处组件私有函数收敛为单一实现，统一 4 套互相矛盾的相对时间行为（刚刚/今天/昨天/30 天/跨年/字节 1024 进位，边界单测），10+ 文件逐页替换并验收文案。
2. **布局迁移逻辑测试**（`src/modules/dashboard/widgets/layout.ts:115-176`）：导出 `migrateV6toV7` 补测（1280/768 边界、旧默认尺寸替换/自定义保留/缺段回退/损坏 JSON 回默认）。
3. **useLocalDraft 测试**（`src/composables/useLocalDraft.ts`）：stub localStorage + 假定时器（2s 防抖、基线比较清幻影草稿、空 key no-op）。
4. **数据不变量机械测试**（新增 `tests/registry.test.ts`）：widget-meta/registry 无重复 name/type、默认尺寸合法。
5. **diff/shortcode 边界补充**：尾换行、MAX_DP_CELLS 退化路径、特殊字符转义。
6. **friends 包解析测试**（backend，`internal/friends/friends.go:280-353` 纯函数）：fetchFeed 的 RSS/Atom 双格式（httptest 注入）、日期多格式、summarize/maxPostsPerFeed 截断。

### 验收清单

- [ ] `make test` 全绿（console tests 文件数 ≥ 6）
- [ ] format.ts 替换后：同一时间戳在通知/附件/动态/仪表盘四处显示一致（截图留档）
- [ ] layout 迁移测试覆盖损坏数据回退；`npm run test` 通过

---

## 运营收尾（非代码，需用户逐项授权后执行）

- **测试文章清理**：默认方案——mock-* 与 test 文章全部转 draft（保留作回归夹具，seed 草稿语义已验证），`stellar-138-test` 取消 pinned（当前置顶显示在首页首屏）；彻底删除需用户确认清单后执行。验收：首页无测试文、置顶为真实内容。
- **分支同步**：`main` ← `release` ← `dev` 纯 fast-forward（main 落后 161 提交、release 落后 10 提交，两分支均无 dev 缺失提交，零冲突）；推送由用户单独授权。
- **磁盘/仓库卫生**（清单待授权）：`dev/.tmp/` 2.4GB（gocache/gomodcache/ogcache/chrome-shot/旧二进制 blog-new、blog-server/过期截图）；qoder 残留 worktree `~/.qoder/worktree/dev/RIJZlJ`（711M，detached HEAD 提交已合入 dev，可安全移除）；`dev/backend/server`（69M 旧构建产物）；`dev/300`（curl cookie 游离文件）；`.bare/worktrees/dev/refs` 空目录（git garbage 警告，gc/rmdir）。

---

## 验证

1. 各阶段 `make merge-check` 全绿（含 `make test`：console 测试 + assets-test + `go vet` + `go test`）后合回 dev；阶段 1 额外跑 `go test -race ./backend/...`
2. UI 子任务按各阶段「UI 验收边界」逐项勾选，截图存入本 spec 目录 `baselines/`（入库，勿放 `.tmp/`）
3. 验证产物写入各自 worktree 的 `.tmp/`，合回后提示用户授权清理

## Out of Scope

- ListByTag 逗号标签改 post_tags 关联表（Spec 61 遗留，影响面大，继续遗留标注）
- created_at 时间格式统一 + 复合索引（Spec 61 遗留：需先统一格式才可建索引）
- vue-tsc/vite 构建并行化（Spec 61 已排除）
- console 巨型组件整体拆分（仅阶段 3 选定项）；`any` 199 处类型化；magic string 枚举化；44pt 触达合规与按钮/卡片/弹窗多套实现收敛（需独立 UI 验收轮，另立 spec）
- CSRF Origin 校验（Lax + JSON-only 已有缓解，属纵深防御）、User 鉴权轻量列、slog 结构化日志迁移
- main.css 拆分与版本戳自动化（与上游 Stellar 编译对齐优先）
- git 历史 72MB blob 重写（filter-repo 需强推，收益中等风险高）、OG 字体子集化、flying-pages 移动端省流策略
- 依赖面（mongo-driver/quic-go/sonic 为 gin 强制依赖，无法裁剪）
