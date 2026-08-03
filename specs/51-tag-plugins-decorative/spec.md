# Spec 51 · 装饰标签插件（poetry / paper / reel / 行内装饰 / folders / albums / posters）

- **分支**：`feature/frontend-gap` · **Wave 10** · **隔离等级**：低
- **主模块**：`internal/post/shortcode.go` + `assets/css/tag-plugins.css`（追加式）
- **前置**：Spec 31（albums/posters/paper/reel 基础版）已合——本期为其补齐参数与装饰细节；Spec 48/49 建议先合（同文件 shortcode.go 追加，减少 rebase）

## Why

Spec 06/16/20/30/31 已覆盖 42/42 原版标签插件清单，但对照原版 `scripts/tags/lib/` 仍有**装饰/文艺类缺口**：**poetry 诗歌卡**（文艺类写作高频）、**paper 纸张卡参数补齐**（31 已落地基础版，缺原版 title/subtitle 装饰参数）、**reel 胶片卷轴参数补齐**（同上）、**行内装饰标签 5 件**（psw 密码遮罩 / emp 强调 / wavy 波浪线 / blur 模糊 / sub 下标——原版 `scripts/tags/inline-labels.js` 的独立注册，本项目尚未实现）、**folders 折叠文件夹**（归档类内容分组）、**albums/posters 参数补齐**（31 已实现为纯正文卡片，本期对齐原版 size 尺寸参数）。本期完成后装饰类覆盖率达原版 100%。

## 功能需求（语法对照原版 `scripts/tags/lib/*.js` + `scripts/tags/inline-labels.js`）

| 插件 | 语法 | 说明 |
|---|---|---|
| poetry | `{% poetry [title] [author:作者] [date:日期] [footer:页脚] %}` body `{% endpoetry %}` | 诗歌卡片：title 标题 + meta 行（author/date）+ body 正文 + footer 页脚，正文走 renderMD |
| paper | `{% paper ... %}` body `{% endpaper %}` | 31 已落地基础版，本期对齐原版参数（对照 31 落地形态补缺，见 What Changes） |
| reel | `{% reel ... %}` 图片列表 `{% endreel %}` | 31 已落地基础版，本期补齐原版参数（见 What Changes） |
| psw | `{% psw 密码内容 %}` | 行内密码遮罩：默认显示 `******` 遮罩，点击/悬停显示明文 |
| emp | `{% emp 强调文本 %}` | 行内强调标签（`<emp>`，下划线+粗体样式） |
| wavy | `{% wavy 波浪文本 %}` | 行内波浪线文本（`<wavy>`，底部波浪线装饰） |
| blur | `{% blur 模糊文本 %}` | 行内模糊文本（`<blur>`，模糊遮罩，悬停清晰） |
| sub | `{% sub text [color:颜色] %}` | 行内下标（`<sub class="tag-plugin colorful sub">`，可选彩色） |
| folders | `{% folders [color:white] %}` `<!-- folder 标题 -->` body `{% endfolders %}` | 折叠文件夹组：每个 `<!-- folder 标题 -->` 开一个可折叠文件夹（header 标题 + body），color 可选 |
| albums | `{% albums [group] [size:s/m/l/xl/mix] %}` | 31 已实现纯正文卡片版（内联图片列表），本期补 size 参数：s/m/l/xl/mix 控制卡片尺寸网格 |
| posters | `{% posters [group] [size:xs/s/m/l/xl/mix] %}` | 同上，31 已实现纯正文卡片版，本期补 size 参数（竖版海报卡片） |

## What Changes

### 新插件（poetry）

- `internal/post/shortcode.go`：
  - +`rePoetry` 正则（块级，`(?s){%\s*poetry\s+(.*?)\s*%}(.*?){%\s*endpoetry\s*%}`）
  - +`renderPoetry(argsStr, body, renderMD)` 函数，输出 HTML 结构：
    ```html
    <div class="tag-plugin poetry">
      <div class="content">
        {% if title %}<div class="title">{{title}}</div>{% endif %}
        {% if author or date %}<div class="meta"><span>{{author}}</span><span>{{date}}</span></div>{% endif %}
        <div class="body">{{renderMD(body)}}</div>
        {% if footer %}<div class="footer">{{footer}}</div>{% endif %}
      </div>
    </div>
    ```
  - 在 `processShortcodes()` 中注册（块级，按现有顺序约定）
- `frontend/assets/css/tag-plugins.css`：补 `.tag-plugin.poetry` 系列样式——居中诗歌排版（正文行间距宽松、meta 弱化、footer 右对齐小字），对齐原版 `source/css/_plugins/poetry.styl` 编译产物（先 grep main.css 确认）

### 新插件（行内装饰 5 件：psw/emp/wavy/blur/sub）

- `internal/post/shortcode.go`：+5 个行内正则（统一格式 `{%\s*psw\s+(.*?)\s*%}` 等）+ 5 个 render 函数，输出：
  ```html
  <psw>文本</psw>
  <emp>文本</emp>
  <wavy>文本</wavy>
  <blur>文本</blur>
  <sub class="tag-plugin colorful sub"{% if color %} color="{{color}}"{% endif %}>文本</sub>
  ```
  （sub 与原版一致支持 color 参数；psw/emp/wavy/blur 为纯标签包裹，原版无参数）
  - 在 `processShortcodes()` 中注册（行内区，与 mark/hashtag 等并列）
- `frontend/assets/css/tag-plugins.css`（或 main.css——先 grep 确认）：补 `psw/emp/wavy/blur/sub` 样式：
  - `psw`：`background: 半透明遮罩色` + `user-select: none` + hover/点击显示明文（纯 CSS `:hover` 显示，或小 JS 点击切换，对齐原版 `source/css/_plugins/psw.styl`）
  - `emp`：下划线 + 粗体强调
  - `wavy`：`text-decoration: wavy underline`（或背景波浪图，对齐原版）
  - `blur`：`filter: blur(4px)` + `transition` + hover 清晰
  - `sub`：`.tag-plugin.colorful.sub` 彩色下标（复用既有 colorful 色系）

### 新插件（folders 折叠文件夹）

- `internal/post/shortcode.go`：
  - +`reFolders` 正则（块级，`(?s){%\s*folders\s*(.*?)\s*%}(.*?){%\s*endfolders\s*%}`）
  - +`reFolder` 内部正则：`<!--\s*folder (.*?)\s*-->` 分割标题与 body
  - +`renderFolders(argsStr, body, renderMD)` 函数，输出 HTML 结构：
    ```html
    <div class="tag-plugin colorful folders"{% if color %} color="{{color}}"{% endif %}>
      <details class="folder"{% if open %} open{% endif %}>
        <summary class="folder-header">{{header}}</summary>
        <div class="folder-body">{{renderMD(body)}}</div>
      </details>
      ...
    </div>
    ```
  - 使用原生 `<details>/<summary>` 实现折叠（零 JS），对齐原版 folders.js 的折叠行为
  - 在 `processShortcodes()` 中注册（块级，poetry 之后）
- `frontend/assets/css/tag-plugins.css`：补 `.tag-plugin.folders` 系列样式——文件夹行样式（文件夹图标 + 标题 + 展开箭头）、body 缩进卡片（对齐原版 `source/css/_plugins/folders.styl`）

### 已有插件参数补齐（paper/reel/albums/posters）

- `internal/post/shortcode.go` + `tag-plugins.css`：对照原版参数表逐项核对 31 落地形态，补齐以下（先读 31 落地代码确认现状，**仅补缺**，不重写）：
  - **albums**：补 `size` 参数（s/m/l/xl/mix）→ 输出 grid 卡片尺寸类（`size:s` 等映射到既有 gallery 尺寸类，先 grep main.css 确认 `.gallery` 尺寸类是否已存在）
  - **posters**：补 `size` 参数（xs 起，竖版海报卡片比例）
  - **paper**：核对原版 paper.js 参数（title/subtitle 等装饰参数）是否已覆盖，缺则补
  - **reel**：核对原版 reel.js 参数（高度/速度等）是否已覆盖，缺则补

### 共享改动

- `frontend/console/src/modules/plugins/plugins-data.ts`：+poetry/folders/psw/emp/wavy/blur/sub 七条新条目（category="文艺装饰"）；albums/posters/paper/reel 已有条目，仅更新参数表（size 等）
- 测试文 `content/posts/tag-plugins-5-test.md` 追加测试用例（poetry 全参数 / 5 件行内装饰 / folders 多组折叠 / albums/posters size 各档）

## 技术决策（无需再问）

- **folders 用原生 details/summary**：零 JS 依赖实现折叠（对齐原版行为，浏览器原生支持），不引入新脚本
- **psw 显示明文交互**：优先纯 CSS `:hover` 显示明文（零 JS）；若原版为点击切换则加 3 行内联 JS（页面级事件委托，不新增模块）
- **albums/posters 不引入数据服务**：延续 31 技术决策（纯正文标签，参数内联图片列表），size 仅控制网格卡片尺寸，不接 repo/api 数据源
- **行内装饰标签不注册 goldmark 扩展**：统一走 shortcode 正则体系（与 40+ 现有插件一致），不改 goldmark 装配
- **sub 复用 colorful 色系**：color 参数映射既有 colorful 类（先 grep main.css），无对应色类时仅输出无彩色样式
- **语法容错**：与历期同标准，未闭合/参数缺失原样输出

## Impact

- **新增**：`shortcode.go` 的 6 个新正则 + 6 个 render 函数、测试文测试用例
- **修改**：`shortcode.go`（albums/posters/paper/reel 参数补齐）、`tag-plugins.css`（新样式 + 参数类）、`plugins-data.ts`（7 条新条目 + 4 条参数更新）、测试文
- **不影响**：post/page store、api 管理端、console 非插件库页面、widget 系统、site.yml 配置、前端 JS 模块

## 验证

1. `go build ./... && go vet ./...` 通过
2. poetry：`{% poetry 静夜思 author:李白 date:唐 %}床前明月光...{% endpoetry %}` 渲染标题/meta/正文/footer 完整卡片，正文 Markdown 生效
3. psw：`{% psw 密码 %}` 显示遮罩，悬停/点击显示明文
4. emp/wavy/blur/sub：分别渲染下划线强调/波浪线/模糊/彩色下标，样式在亮/暗模式下均清晰
5. folders：`{% folders %}` 内 2 组 `<!-- folder 标题 -->` 均折叠，点标题展开对应 body，body 内 Markdown 生效
6. albums/posters：31 已有卡片渲染正常，`size:l` / `size:mix` 切换网格尺寸生效
7. paper/reel：31 已有渲染回归不破，参数表与原版核对无缺口
8. 亮/暗模式目检全部插件
9. Console 插件库新条目显示正常，示例文本可复制
10. `make test` 全绿

## Out of Scope

- albums/posters 的 repo/api 数据服务模式（延续 31 决策，不接远端数据）
- folders 的多级嵌套（原版仅单层）
- psw 的点击复制/双击查看等高级交互（原版无）
- 行内装饰标签的 site.yml 配色配置（原版无配置项，样式走 CSS 类）
- poetry 的背景图/音乐参数（原版无，不扩展）
