# Spec 49 · 标签插件六期（展示容器 6 件套）

- **分支**：`feature/frontend-gap` · **Wave 10** · **隔离等级**：中
- **主模块（独占）**：`internal/post/shortcode.go` + `assets/css/tag-plugins.css` + `assets/js/`
- **前置**：无（swiper 已有 JS 数据服务 + shortcode 渲染，本 spec 为其完善参数；quot 已有 shortcode 渲染，本 spec 延续）

## Why

Spec 06/16/20/30/31 已落地 40+ 标签插件，覆盖提示标注、布局容器、媒体播放、数据图表、GitHub 五大类。本期补齐展示容器类中频 6 件：**swiper 参数增强**（已有 shortcode 但缺 min/max 宽度与 coverflow 等效果）、**banner 横幅**（页面顶部/个人资料/文章摘要卡片的常见需求）、**quot 引用增强**（已有 shortcode 但缺 el:h5/h6 支持）、**chat 聊天增强**（已有 shortcode 但缺群聊模式与多媒体 cell 占位）、**navbar 文章内导航**（文章内嵌导航栏，独立于页面顶部 navbar）、**frame 设备框架**（iPhone/iPad/MacBook 设备壳套图）。本期完成后展示容器类覆盖率达原版 95%。

## 功能需求（语法对照原版 `scripts/tags/lib/*.js`）

| 插件 | 语法 | 说明 |
|---|---|---|
| swiper | `{% swiper [width:max/min] [effect:slide/coverflow/cards] %}` 图片列表 `{% endswiper %}` | 已有 shortcode + JS 服务，补 width 的 min/max 语义、effect 的 coverflow/cards 参数透传；swiper 库已在 vendor |
| banner | `{% banner [bg:背景图] [avatar:头像] [link:链接] [title:标题] [subtitle:副标题] %}` body `{% endbanner %}` | 页面顶部/个人资料/摘要卡片，body 渲染为 banner 正文内容 |
| quot | `{% quot [el:p/h2/h3/h4/h5/h6] [icon:图标] [prefix:前缀图标] [suffix:后缀图标] text %}` | 已有 shortcode，补 el:h5/h6 支持（当前仅 p/h2/h3/h4） |
| chat | `{% chat device [style:qq/wechat] [title:标题] [scene:group/single] [me:用户key] %}` YAML 用户定义 + `<!-- chattip -->` / `<!-- chatcell -->` `{% endchat %}` | 已有 shortcode（文字 + chattip 子集），补 scene:group 群聊模式（多用户左对齐排列）、`<!-- chatcell type:image -->` 等多媒体 cell 的 fallback 占位（不渲染实际内容，仅显示类型标签防止破版） |
| navbar | `{% navbar [active:文本] %}` 链接列表 `{% endnavbar %}` | 文章内导航栏，body 每行 `[text](url)` 渲染为导航项，active 匹配当前高亮 |
| frame | `{% frame [device:iphone/ipad/macbook] [width:宽度] %}` 图片/视频 `{% endframe %}` | 设备框架容器，将内容嵌入对应设备 SVG 壳中，仅图片/视频/iframe 内容 |

## What Changes

### 已有插件增强（swiper/quot/chat）

- `internal/post/shortcode.go`：
  - **swiper**：`renderSwiper()` 补 `width:min` 和 `width:max` 语义解析（min→`max-width` 百分比，max→`max-width` 像素值如 800px）；effect 参数透传现有 `effect` 属性（已有逻辑，补 coverflow/cards 向 Swiper 实例的兼容处理——当前 swiper.js 已将 effect 属性传给 Swiper 构造器，覆盖流程类参数如 `coverflowEffect` 需在 JS 端补全）
  - **quot**：`renderQuot()` 的 `el` 参数允许值追加 `h5`、`h6`（当前 if 判断 `strings.Contains(el, "h")` 已覆盖，仅需确认锚点 id 生成逻辑对 h5/h6 无副作用）
  - **chat**：`renderChat()` 补 `scene` 参数解析（缺省 `group`），scene=group 时参与者消息均左对齐（目前 `me` 用户靠右其他靠左，group 模式下全部靠左）；`<!-- chatcell type:image/voice/video/file/link -->` 解析时输出 `<div class="chat-cell unsupported">[type]</div>` 占位（当前静默跳过，破版风险低但占位更健壮）
- `frontend/assets/js/services/swiper.js`：`initSwiper()` 补 `coverflowEffect` 参数——当 `effect=coverflow` 时透传 Swiper 的 `coverflowEffect: { rotate: 50, stretch: 0, depth: 100, modifier: 1, slideShadows: true }`；`effect=cards` 时透传 `cardsEffect: { ... }`（对齐 Swiper 10.3 文档）
- `frontend/assets/css/tag-plugins.css`：仅补 main.css 确无的类——**swiper coverflow/cards 效果容器样式**（Swiper 库自带效果样式，验证后确认是否需要额外覆盖）；chat `scene=group` 场景下消息左对齐调整（`.tag-plugin.chat[scene="group"] .chat-message` 对齐方式）

### 新插件（banner）

- `internal/post/shortcode.go`：
  - +`reBanner` 正则（块级，`(?s){%\s*banner\s+(.*?)\s*%}(.*?){%\s*endbanner\s*%}`）
  - +`reBannerPattern` 在 `processShortcodes()` 中注册（块级处理，在 swiper 之后、chat 之前）
  - +`renderBanner(argsStr, body, renderMD)` 函数，输出 HTML 结构：
    ```html
    <div class="tag-plugin banner"{% if bg %} style="background-image:url({{bg}})"{% endif %}>
      {% if link %}<a class="banner-link" href="{{link}}">{% endif %}
        <div class="banner-content">
          {% if avatar %}<div class="avatar"><img src="{{avatar}}"></div>{% endif %}
          {% if title %}<div class="title">{{title}}</div>{% endif %}
          {% if subtitle %}<div class="subtitle">{{subtitle}}</div>{% endif %}
          <div class="body fs14">{{renderMD(body)}}</div>
        </div>
      {% if link %}</a>{% endif %}
    </div>
    ```
- `frontend/assets/css/tag-plugins.css`：补 `.tag-plugin.banner` 系列样式——背景图铺满 + 居中内容 + 头像圆形裁剪 + 标题/副标题/正文层级间距（对齐原版 `source/css/_plugins/banner.styl` 编译产物，先 grep main.css 确认是否已存在）

### 新插件（navbar 文章内导航）

- `internal/post/shortcode.go`：
  - +`reNavbar` 正则（块级，`(?s){%\s*navbar\s*(.*?)\s*%}(.*?){%\s*endnavbar\s*%}`）
  - +`reNavbarLink` 内部正则：提取 `[text](url)` 格式链接
  - +`renderNavbar(argsStr, body)` 函数，body 解析为链接列表，输出 HTML 结构：
    ```html
    <div class="tag-plugin navbar">
      {% for each link in body %}
      <a class="nav-link{% if active == text %} active{% endif %}" href="{{url}}">{{text}}</a>
      {% endfor %}
    </div>
    ```
  - 在 `processShortcodes()` 中注册（块级，在 banner 之后）
- `frontend/assets/css/tag-plugins.css`：补 `.tag-plugin.navbar` 系列样式——水平内联导航、高亮 active 下划线/颜色、与页面顶部 navbar 视觉区分（更低调的字体/间距/圆角标签风格）

### 新插件（frame 设备框架）

- `internal/post/shortcode.go`：
  - +`reFrame` 正则（块级，`(?s){%\s*frame\s+(.*?)\s*%}(.*?){%\s*endframe\s*%}`）
  - +`renderFrame(argsStr, body)` 函数，body 提取 `<img>/<video>/<iframe>` 标签嵌入设备壳
  - 设备壳使用纯 CSS 实现（不依赖 SVG 图片资源），形状对照原版 `source/css/_plugins/frame.styl`：
    - iPhone：圆角矩形 + 刘海凹槽 + 圆角
    - iPad：更大圆角矩形
    - MacBook：屏幕区域 + 底部 body 区（键盘面）
  - 输出 HTML 结构：
    ```html
    <div class="tag-plugin frame" device="{{device}}" style="max-width:{{width}}">
      <div class="frame-device {{device}}">
        <div class="frame-notch"></div>
        <div class="frame-content">{{body 提取的内容}}</div>
      </div>
    </div>
    ```
  - 在 `processShortcodes()` 中注册（块级，在 navbar 之后）
- `frontend/assets/css/tag-plugins.css`：补 `.tag-plugin.frame` 系列样式——三种设备壳的 CSS 形状（圆角/刘海/边框/阴影），确保内容适配 frame-content 区域

### 共享改动

- `frontend/console/src/modules/plugins/plugins-data.ts`：+banner/navbar/frame 三个新插件的元数据条目（category="布局容器"）；swiper/quot/chat 已有条目，仅更新参数表（swiper 补 min/max width 说明、quot 补 h5/h6、chat 补 scene 和 scene:group）
- 测试文 `content/posts/tag-plugins-5-test.md` 追加 6 插件的测试用例（已有测试文，追加新节）

## 技术决策（无需再问）

- **swiper 效果参数**：effect=coverflow/cards 的附加参数（coverflowEffect/cardsEffect）在 JS 端硬编码合理值，不做 site.yml 配置（原版也是硬编码；过于低频的逻辑不增加配置复杂度）
- **banner 不依赖 CSS background-image 裁剪**：banner 背景图使用 `style="background-image:url(...)"` 内联，不额外引入 srcset/懒加载（banner 在首屏附近，懒加载需求低）
- **frame 纯 CSS 实现**：不引入 SVG 设备壳资源（减少打包体积），使用 border-radius + box-shadow + 伪元素模拟设备外形；原版 frame.styl 也是纯 CSS
- **chat 多媒体 cell 占位**：仅输出 `<div class="chat-cell unsupported">[type]</div>` 文本占位，不渲染实际媒体内容，不报错不破版（与现有 chat 子集决策一致）
- **navbar 文章内导航与页面顶部 navbar 无关联**：独立 shortcode，不共享样式/状态，不与 site.yml nav_tabs 交互
- **语法容错**：参数缺失/未闭合原样输出（与已有 40+ 插件同标准）

## Impact

- **新增**：`shortcode.go` 的 3 个新正则 + 3 个 render 函数、测试文测试用例
- **修改**：`shortcode.go`（swiper/quot/chat 增强）、`tag-plugins.css`（新样式）、`swiper.js`（coverflow/cards 支持）、`plugins-data.ts`（新条目）、测试文
- **不影响**：post/page store 主体、api 管理端、console 非插件库页面、widget 系统、site.yml 配置

## 验证

1. `go build ./... && go vet ./...` 通过
2. swiper：`{% swiper effect:coverflow %}` 渲染 3 张图片，轮播切换带 3D 旋转效果；`{% swiper width:min %}` 图片自适应最小宽度
3. banner：`{% banner bg:https://... avatar:https://... title:标题 %}` 渲染带背景图/头像/标题的卡片，link 参数可点击跳转
4. quot：`{% quot el:h5 text:五级标题 %}` 渲染为 `<h5 class="content">` 且锚点跳转正常
5. chat：`{% chat iphone style:qq scene:group %}` 消息全部左对齐；`<!-- chatcell type:image -->` 显示 `[image]` 占位文本
6. navbar：`{% navbar active:主页 %}` 内 `[主页](/)` 高亮，`[分类](/categories/)` 不高亮
7. frame：`{% frame device:iphone %}` 图片嵌入圆角 iPhone 壳中，刘海可见；`device:macbook` 显示屏幕+键盘区
8. 亮/暗模式目检全部 6 插件
9. Console 插件库 banner/navbar/frame 三条目显示正常，示例文本可复制

## Out of Scope

- swiper 的缩略图联动、自动播放间隔配置、响应式断点（Swiper 库支持但不做 shortcode 参数暴露）
- banner 的横幅动画、视差滚动效果
- frame 的 Android 设备、Apple Watch 等非主流设备壳
- chat 的多媒体 cell 实际渲染（image/voice/video/file/link——保持静默占位，与原版子集决策一致）
- navbar 的 dropdown 子菜单、图标支持