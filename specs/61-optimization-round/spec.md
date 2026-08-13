# Spec 61 · 项目优化轮

- **分支**：`feature/opt-frontend` → `feature/opt-build` → `feature/opt-security` → `feature/opt-password` → `feature/opt-perf`（5 个 feature worktree **顺序推进**：一个合回 dev 后再开下一个，全程 rebase 最新 dev）
- **主模块**：见各阶段「改动点」
- **前置**：Spec 60（theme-yaml-pages）

## Why

项目功能已收敛，进入优化轮：前端入口 chunk 过大（ri 图标全量注册 1.36MB、静态 Chart.js、每页无条件加载 KaTeX/Mermaid 约 2.3MB、18MB TTF 暴露在 /assets 公网）、构建流程存在全量 npm ci 与无条件重建、后端安全面（上传大小、zip bomb、AI guest SSRF、site 热更新数据竞争、pending2FA 泄漏）、admin 密码仍明文存储、以及一组明确的性能与死代码问题。本轮按「收益最大、风险最低 → 风险最高」排序分 5 阶段推进，每阶段独立验证、独立合回。

## 执行模型

- 每阶段开工前查 `CLAUDE.md` 对应坑点章节（head.html 改动 → Stellar CSS/HTML 规范与静态资源版本戳 bump 规范；SQLite → 工具链/迁移坑点；frontend/assets 改动 → 版本戳 bump）。
- 每阶段 `make merge-check` 全绿才合回 dev，合完即删分支；验证产物写入各 worktree 的 `.tmp/`，合回后提示用户单独授权清理。
- 各阶段独立提交，中文提交信息，格式 `类型: 描述`。

---

## 阶段 1 · 前端体积（feat-opt-frontend，收益最大、零后端风险）

### 改动点

1. **ri 图标全量注册瘦身**（`console/src/main.ts`）：`@iconify/json/json/ri.json` 全量替换为实际用到的子集——脚本扫描 `console/src` 中全部 `ri:` 用法（含 IconPicker 内联图标），生成 `src/icons/ri-subset.json` 提交入库；新增 assets-test 机械校验（`frontend/scripts/test/` 下新 .mjs）：源码里出现的新 `ri:` 图标不在子集内即失败。预期入口 chunk 减 ~1MB。
2. **Chart.js 懒加载**：静态 `Chart.register` 移入新 `src/lib/chart-init.ts`（注册幂等）；`HourlyChartWidget.vue` 与 `PerfView.vue` 改动态 import，vite 自动分包，仅 Dashboard/性能页加载。
3. **KaTeX/Mermaid 按需加载**（保持 CDN 决策）：`backend/internal/template/partial/head.html` 移除无条件 CDN 注入，替换为小型 loader（`frontend/assets/js/math-mermaid-loader.js`）：扫描 `.md-text` 数学定界符 / `.mermaid` 元素存在时动态注入 CDN link/script；DOMContentLoaded 执行（PJAX 已随 Spec 59 移除，导航为整页刷新，无需重扫回调）。
4. **死资源清理**：删 `assets/img/demo-desktop*.png`、`demo-phone.png`、`assets/js/vendor/jquery.min.js`、`console/src/assets/vue.svg`、`hero.png`；`console/index.html` `lang="zh-CN"`；`manifest.webmanifest` name 改为站点名。
5. **18MB TTF 移出公开 embed**：`frontend/assets/embed.go` 的 `all:fonts` 不再嵌入 `LXGWWenKai-Regular.ttf`——移到 `backend/internal/og/fonts/` 新建独立 embed（仅 OG 渲染器使用），`cmd/server/main.go` 同步改读取路径（embed 与 dev 两分支），`assets/fonts/` 只留 lxgw 分片。
6. **console 字体去重**：核实 dist 中 14 个字体文件的 .woff 是否为 fontsource 冗余产物，是则在构建后剔除（只留 woff2）；顺带确认 7 个字重全部真实使用。

### 验收清单

- [ ] 入口 chunk 体积对比（前后留档 `.tmp/`），ri 子集后减 ~1MB
- [ ] `make test` 全绿（含新 ri 子集机械校验：故意加未注册 `ri:` 图标时失败、正常时通过）
- [ ] console 验证：登录/图标选择器（ri 图标可选可用）/仪表盘图表/性能页（Chart.js 分包后仍渲染）
- [ ] 前台抽 3 类页面：非数学页网络面板 katex/mermaid **零请求**；含公式文章仍渲染；含 mermaid 图表文章仍渲染
- [ ] `/assets/` 下无 18MB TTF；OG 渲染路径（embed 与 dev 两分支）正常
- [ ] console dist 无 .woff 冗余产物；manifest 名称/lang 修正生效

---

## 阶段 2 · 构建流程（feat-opt-build）

### 改动点

1. **去 npm ci 全量重装**：`Makefile` `console` target 改用 `console-deps`（升级为 package-lock hash 判断：`.node_modules-stamp` 与 lock 内容比对，不匹配才 `npm ci`）。
2. **dev 模式不再全量生产构建**：`dev` target 仅当 `console/dist` 缺失或 `console/src` 较新时才重建（时间戳判断），否则跳过。
3. **stylus 编译接入 Makefile**：新增 `make css` target 调 `build/stylus/compile.js` 再生成 `assets/css/main.css`；因 `resource/` 主题源不入 git、CI 无法编译，该 target 为本地/agent 维护入口，不进 test 门禁；README/CLAUDE.md 补说明（解决 287KB 生成物无法从源再生问题）。
4. 不做：vue-tsc/vite 并行化（明确排除）。

### 验收清单

- [ ] node_modules 已存在时 `make build` 确认跳过 npm ci
- [ ] touch `console/src` 后 `make dev` 触发重建；未 touch 时跳过
- [ ] 本机 `make css` 产物与 git 内 main.css diff 无漂移
- [ ] `make test` / `make merge-check` 全绿

---

## 阶段 3 · 后端安全加固（feat-opt-security，不含密码迁移）

### 改动点

1. **上传大小限制前置**：`internal/api/attachment.go` 与 `internal/comment/handler.go` 在解析前用 `http.MaxBytesReader` 按 `max_upload_mb` 限制请求体（现有解析后校验保留为第二道）。
2. **zip 导入防 zip-bomb**：`internal/backup/backup.go` 限制条目数与单条解压后大小（`io.LimitReader`），现有 slug 白名单保留。
3. **AI guest SSRF 加固**：`internal/api/ai.go` 校验后按解析所得 IP 固定拨号（自定义 Transport `DialContext` + 保留 SNI），并 `CheckRedirect` 禁止重定向（封堵 TOCTOU 重绑定与 302 内网跳转）。
4. **site 热更新数据竞争**：`internal/config/reload.go` —— `RebuildSite` 改为「构建全新 *tmpl.Site → atomic.Pointer 复制-交换发布」；`webServer.site` / `api.Handler.site` 两处字段改 `atomic.Pointer[tmpl.Site]`，读取点机械替换为 `.Load()`（与 `preview_site.go` 浅拷贝草稿机制兼容）。
5. **pending2FA 内存泄漏**：`internal/api/auth.go` 每次 `generate2FAToken` 顺带清理全表过期条目（沿用 loginBlocked 惰性清理模式）。

### 验收清单

- [ ] `go test -race ./backend/...` + `make test` 全绿
- [ ] 上传超大文件在解析前被拒（观察无临时文件落盘）
- [ ] zip bomb 样本拒绝；正常导入不受影响
- [ ] AI guest 传 `http://` 内网地址 / 重定向样本均被拒；正常外网请求不受影响
- [ ] 热更新保存设置后前台渲染无竞态（-race 无报）；pending2FA 过期条目被惰性清理

---

## 阶段 4 · 密码哈希迁移（feat-opt-password，独立阶段，风险最高）

现状：users 表 bcrypt 已存在（Spec 45），但 admin 密码仍明文存 settings admin 段，旧格式 cookie 用明文密码当 HMAC key。

### 改动点

1. **启动迁移（幂等）**：Bootstrap 检测 admin 段非空 → 确保 users 表存在 admin 用户且 bcrypt 匹配该密码（admin 用户缺失或哈希不匹配则创建/更新）→ 清空 settings admin 段。
2. **语义替换**：`site.Admin.Password` 改为派生布尔 `AuthEnabled`（RebuildSite 时查 users 表 admin 是否有密码哈希）；`apiAuth` / `authCheck` / `login` 的 `password == ""` 全放行分支改按 `AuthEnabled` 判定，无密码站点语义不变。
3. **旧格式登录路径**：无 username 登录 → 直接走 admin 用户 bcrypt 校验；`validAuthCookie` 旧格式（明文密码 HMAC）验证路径移除。**影响**：迁移后旧 cookie 一次性失效，需重新登录（在 spec 与 release 说明中明示）。
4. **changePassword userID=0 回退路径**改为写 users 表（bcrypt + 轮换 auth_secret）；`SaveAdminPassword` 及其明文回写链路删除。

### 验收清单

- [ ] 全新库（site.yml 带密码）启动 → users 表有 admin、settings admin 段已空、`AuthEnabled=true`
- [ ] console 全链路手动验证：登录/2FA/登出/改密/PAT/限流
- [ ] 旧 cookie 失效后重登正常；无密码配置站点仍全放行
- [ ] `make test` 全绿（auth 相关测试同步修正）

---

## 阶段 5 · 后端性能（feat-opt-perf，行为等价优先）

### 改动点

1. **PV 埋点异步批量**（`cmd/server/middleware.go` 同步写库串行化所有读）：buffered channel + flush goroutine（定时/批量阈值触发批量 INSERT），优雅退出时 flush，失败记日志。
2. **阅读量单语句**：`routes_post.go` IncrViews+GetViews 两次往返合并（`UPDATE ... RETURNING views`，先确认 modernc sqlite 版本支持，不支持则保双语句）。
3. **/search.json 内存缓存**：写路径 hook 已存在（post/page/moment hook + 需补 wikidoc 保存钩子）置脏后重建，atomic.Pointer 发布，去掉每请求全量重查。
4. **sitemap.xml 缓存**：同模式生成期缓存 + 写路径失效。
5. **索引失效修复**：确认 created_at 存储格式统一后去掉 `substr(created_at,1,19)` 排序包裹 + 建 `(status,pinned,created_at)` 复合索引；`pageview.go` 4 处 `date(created_at)>=?` 改范围比较；`ListByTag` 的 LIKE 逗号标签全表扫标注为遗留（本轮评估不动）。
6. **N+1**：wiki 计数/笔记本计数改 `GROUP BY` 一条查询、note 上下篇改两条 `LIMIT 1` 邻居查询、tag 过滤下推 SQL。
7. **LoadAI 内存缓存**：RebuildSite 时刷新，去掉每文章请求读 settings。
8. **死代码清理**：`handleTopicIndex/handleTopicDetail`（未注册）、`GetPrevPost/GetNextPost`、`ListRelatedByTags`、`CountByTopic`、`RenderBytes`、`_ = p.Topic`。
9. **小项**：friends `checkLinks` 并发信号量（10）、og/github 缓存 singleflight、FTS 重建包事务、SW 模板启动时读一次。

### 验收清单

- [ ] `make test` + `-race` 全绿
- [ ] `/search.json` 第二遍请求无 DB 查询（SQLite trace 验证）
- [ ] PV 落库延迟观察（批量 flush 后入库）；浏览量/统计接口数字与优化前一致
- [ ] 死代码删除后编译与测试全绿；sitemap 命中缓存

---



---

## 验收结果登记（2026-08-13 全部完成）

### 阶段 1 · 前端体积（feature/opt-frontend → `513bcf2` 起 4 笔提交）

- [x] 入口 chunk 1,360.26 kB → **129.32 kB（-90.5%）**，gzip 365.9→44.0 kB；留档 dev/.tmp 对比（随 worktree 清理已删，数字记录于此）
- [x] `make test` 全绿；ri 子集机械校验正/负例均验证（生成器还抓出 2 个不存在的图标并替换）
- [x] console 验证（headless）：登录/63 个 ri 图标渲染/图标选择器弹窗 33 图标/dashboard SVG 趋势图/性能页双 canvas，零控制台错误；Chart.js 分包后 dashboard 零请求、性能页按需加载 1 次
- [x] 前台：非公式页 katex/mermaid 零请求；公式文章（新增 mock-katex-math.md 夹具）KaTeX 按需渲染 6 处；mermaid 文章 svg 渲染；再访首页仍零请求
- [x] `/assets/fonts/` 无 18MB TTF；OG 渲染 dev 分支（68KB PNG）与 embed 分支（69KB PNG）均正常
- [x] console dist 14 字体 → 7 woff2（legacy woff 构建后剔除）；manifest name 已核实与站点名一致（"Stellar Demo"）；index.html lang=zh-CN
- 附注：`modulePreload` 预取问题根因是 main.ts 遗留静态导入（`513bcf2` 修复），非 rolldown 行为

### 阶段 2 · 构建流程（feature/opt-build → `fe9d3bc`）

- [x] node_modules 与 lock 一致时 `make build`/`make console-deps` 跳过 npm ci（stamp 哈希比对，篡改 stamp 触发重装验证）
- [x] `make dev` 三态验证：dist 最新跳过 / touch src 重建 / dist 刷新后跳过
- [x] `make css` 重编 main.css（280.5 KB）与 git 内产物 **diff 无漂移**；README/CLAUDE.md 补说明
- [x] `make test` / `make merge-check` 全绿

### 阶段 3 · 后端安全加固（feature/opt-security → `47229d9`）

- [x] `go test -race ./backend/...` + `make test` 全绿（15 包）
- [x] 7MB 上传在解析前被拒（0.007s，400）；1KB 对照正常走解析（429 限流证明路径完整）
- [x] zip-bomb：501 条目 → 400 硬拒；单条 40MB 解压放大 → 拒导（skipped）
- [x] AI guest：http:// 400、https://10.x/127.0.0.1 → 403；加固客户端单测覆盖固定拨号 + 302 拒绝（TLS httptest）；外网实测受限（本机无外网）
- [x] 热更新保存设置后前台渲染正常；-race 无报；pending2FA 惰性清理（代码级）
- 附注：site 原子化实际波及 page/moment/feed/comment/friends/module 全部持有者（~100 处 .Load()），非计划预估的 2 处字段

### 阶段 4 · 密码哈希迁移（feature/opt-password → `fc6dc24`）

- [x] 全新库（site.yml 带密码）启动：users 表 admin（bcrypt 60 位）、settings admin 段已清空、`passwordRequired:true`
- [x] console 链路：无 username 登录 → 新格式 cookie → profile 200；改密后旧 cookie 401、新密码重登 200；限流 5 次后 429（正确密码也被 IP 限流）
- [x] 旧格式 cookie（明文 HMAC 2 段）→ authCheck false / profile 401；无密码站点 authCheck `passwordRequired:false` + 管理接口全放行 + 登录"无需密码"
- [x] `make test` 全绿（auth 相关测试重写：迁移幂等/派生/旧路径移除）

### 阶段 5 · 后端性能（feature/opt-perf → `0e15461`）

- [x] `make test` + `-race` 全绿（15 包）
- [x] /search.json 缓存：单测验证重建一次/置脏可见新内容；运行时两次请求字节一致；sitemap 按 baseURL 分 key 缓存
- [x] PV 异步批量：立即查 0 → 4s 后批量落库 3 条；优雅退出 kill -INT 后 5 条全部 flush；浏览量数字与优化前一致（3 次访问 views=3）
- [x] 死代码删除后编译与测试全绿；FTS 重建包事务
- 附注：created_at 实测**格式不统一**（posts 带时区后缀/comments 纯 19 字符/updated_at 含纳秒），substr 排序包裹按计划条件保留，复合索引因无法服务 substr 表达式排序而不建（已在代码注释说明）；ListByTag LIKE 全表扫标注遗留

## Out of Scope

- 密码迁移之外的鉴权体系重构
- `ListByTag` 逗号标签改关联表（影响面大，本轮仅标注遗留）
- vue-tsc/vite 构建并行化
- dev→release→main 同步与推送（完成后由用户决定）
