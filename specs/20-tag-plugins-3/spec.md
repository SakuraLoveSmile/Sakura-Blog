# Spec 20 · Stellar 标签插件三期（媒体与展示 8 件套）

- **分支**：`feature/tag-plugins-3` · **Wave 5** · **隔离等级**：高
- **主模块（独占）**：`internal/post/shortcode.go` + `assets/css/tag-plugins.css`
- **前置**：Spec 16 已合（同文件追加式扩展）；ghcard 的代理 URL 依赖 **Spec 21 先合并**（合并前临时热链外部 API，21 合并后切本地代理，见技术决策）

## Why

Spec 06 + 16 已落地 19 个标签插件，剩余中频一批：**video/audio 媒体嵌入**（B 站/网易云/YouTube 是中文博客刚需）、**copy 复制框**（git clone 场景）、**poetry 诗词排版**、**rating 评分**（数据服务类首件）、**swiper 轮播**、**chat 聊天记录**、**ghcard GitHub 卡片**。本期补齐后标签插件覆盖率达原版 90%+，仅剩 sites/friends/albums 等数据页插件（已有专页实现）与低频项。

## 功能需求（语法对照原版 `scripts/tags/lib/*.js`）

| 插件 | 语法 | 说明 |
|---|---|---|
| video | `{% video src [type] [bilibili:bvid] [youtube:id] [ratio:16/9] [width:100%] [autoplay] %}` | 三分支：bilibili iframe / youtube iframe / 原生 `<video>` |
| audio | `{% audio src [type] [netease:id] [autoplay] %}` | 双分支：网易云外链 iframe / 原生 `<audio>` |
| copy | `{% copy text [git:ssh/gh/https] [prefix:$] %}` | 复制框：git 参数展开为 clone 命令，按钮 JS 复制 + toast |
| poetry | `{% poetry [title] [author:x] [date:x] [footer:x] %}` body `{% endpoetry %}` | 居中诗词排版，body 走 renderMD |
| rating | `{% rating id [icon] [title] %}` | 五星评分，`data-api` 指向本地 `/api/rating`（见 What Changes） |
| swiper | `{% swiper [width:max] [effect:cards] %}` 图片列表 `{% endswiper %}` | 轮播：提取 body 中 `<img>` 转 slide；swiper 库本地化 vendored |
| chat | `{% chat device [style:qq/wechat] [title] [me:user] %}` YAML 用户定义 + `<!-- chattip -->` / `<!-- chatcell -->` `{% endchat %}` | **仅实现子集**：文字消息（md:）+ 时间提示（chattip），见技术决策 |
| ghcard | `{% ghcard user[/repo] [theme:x] ... %}` | GitHub 统计卡片 `<img>`，URL 走 Spec 21 本地代理 |

## What Changes

- `internal/post/shortcode.go`：+8 正则 + render 函数，接入 `processShortcodes`。
  - 块级（poetry/swiper/chat）先处理，行内（video/audio/copy/rating/ghcard）后处理。
  - copy 需要自增 id（对齐 `copy_index` 模式，参考现有 `hashtagColorIdx`）。
- `assets/css/tag-plugins.css`：仅补 main.css 确无的类 —— 已核实 **audio、swiper 缺失**，video-player/copy/poetry/ds-rating/chat/ghcard 均已在 main.css，**不得重复自定义**。swiper 容器样式对照原版 `source/css/_plugins/swiper.styl`。
- `assets/vendor/swiper/`：vendored swiper-bundle.min.css/js（v10.3，对照原版 `_config.yml` CDN 版本），`partial/head.html` 或按需注入（有 swiper 插件的页面才加载，对齐现有 mermaid 按需加载模式；若无此模式则全局引入）。
- `assets/js/`：copy 按钮的复制 + toast 逻辑、rating 的打分请求逻辑（对照原版 `source/js/services/rating.js` 行为）、swiper 初始化调用。放现有 js 文件追加或新建 `services.js`，勿动 reading.js 主体。
- **rating 本地数据服务**（小而独立）：
  - `internal/store/rating.go`（新文件）：`ratings` 表（id, target_id, score, voter_hash, created_at），`VoteRating`/`GetRating`（avg + count），voter_hash = IP hash 防重复刷（同 target 同 hash 覆盖更新）。
  - `internal/comment/` 同级思路：公开路由 `GET/POST /api/rating`（query: id），放 `cmd/server/main.go` 注册（+2 行）。响应格式对照原版 rating.js 期望的字段（读原版 js 确认）。

## 实现步骤

1. 逐个精读 8 个原版 js，记录输出 HTML 与 class（chat 重点核对 fork 版结构与 main.css 中 `.tag-plugin.chat` 系列类的匹配度）。
2. 行内五件（video/audio/copy/rating/ghcard）先行——结构简单，纯字符串拼接。
3. rating 数据服务：store 表 + 公开 API + 前端 js 打分逻辑，`data-api` 指向 `/api/rating`。
4. swiper：vendor 库落地 + 块级正则（body 渲染后提取 img）+ 初始化 js + 缺失样式。
5. poetry 块级（最简单的容器类）。
6. chat 子集：YAML 头解析（用户名/头像/label）+ chattip/chatcell(md:) 切分渲染；不支持的 cell 类型静默跳过。
7. 测试文 `content/posts/tag-plugins-3-test.md` 覆盖全部 8 件，`-seed` 重导验证。

## 技术决策（无需再问）

- **swiper 本地 vendored**：不引 unpkg CDN（可用性差），版本锁 10.3 与原版一致。
- **rating 自建**：不接外部 star-vote 服务；voter 去重用 IP hash（单人博客够用），不做登录态。
- **chat 只做子集**：文字消息 + 时间提示 + me 右对齐 + label 徽标；image/voice/video/link/file cell 一律跳过（输出空），fork 版全量结构复杂度不值。
- **ghcard 分两步**：本 spec 输出 `<img src="/api/gh/card?...">` 指向 Spec 21 代理；若 21 未合，构建仍通过（img 404 而已），合并顺序上建议 **21 先合**。
- **autoplay 默认关**：video/audio 的 autoplay 缺省 false，对齐原版。
- **语法容错**：参数缺失/未闭合原样输出（与 06/16 同标准）。

## Impact

- **新增**：`internal/store/rating.go`、`assets/vendor/swiper/`、测试文
- **修改**：`internal/post/shortcode.go`、`assets/css/tag-plugins.css`、`assets/js/`（追加）、`cmd/server/main.go`（+2 行 rating 路由）、`partial/head.html`（+swiper 引入）
- **不影响**：post/page store 主体、api 管理端、console

## ADDED Requirements

### Requirement: 媒体与展示标签插件
The system SHALL render video/audio/copy/poetry/rating/swiper/chat/ghcard tag plugins from markdown into Stellar-aligned HTML, with a local rating data service.

#### Scenario: B 站视频嵌入
- **WHEN** 文章含 `{% video bilibili:BV1xx411c7mD %}`
- **THEN** 输出 `.tag-plugin.video-player` 内嵌 player.bilibili.com iframe，比例 16/9

#### Scenario: copy 复制框
- **WHEN** 文章含 `{% copy git:ssh xaoxuu/hexo-theme-stellar %}`
- **THEN** 显示 `git@github.com:xaoxuu/hexo-theme-stellar.git`，点击按钮复制成功并弹 toast

#### Scenario: rating 评分闭环
- **WHEN** 访客点击第 4 颗星
- **THEN** POST /api/rating 落库，页面即时刷新 avg 与 count；同 IP 重复打分为覆盖而非累加

#### Scenario: swiper 轮播
- **WHEN** swiper 块内含 3 张 markdown 图片
- **THEN** 渲染 3 个 slide + 分页器 + 前后按钮，本地 vendor 的 swiper 库初始化成功

#### Scenario: chat 子集降级
- **WHEN** chat 块含 1 条文字消息 + 1 条 image cell
- **THEN** 文字消息正常渲染，image cell 静默跳过，不报错不破版

## 验证

1. `go build ./... && go vet ./...` 通过
2. 测试文 8 插件逐个目检：结构对照原版渲染结果（原版 demo 站或本地 hexo）
3. rating：连续打分 → 覆盖逻辑正确；重启服务数据仍在
4. swiper/copy/rating 的 JS 在 PJAX 切页后仍能初始化（对齐现有 reading.js 的 pjax 钩子模式）
5. 亮/暗模式目检全部插件

## Out of Scope

- chat 的多媒体 cell（image/voice/video/link/file）
- video 的弹幕、播放列表；audio 的 APlayer 歌单
- rating 的管理端统计页
- sites/friends/albums 等数据页插件（已有专页替代）
