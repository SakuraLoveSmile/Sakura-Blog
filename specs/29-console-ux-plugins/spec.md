# Spec 29 · Console UX 打磨 + 标签插件写作面板

- **分支**：`feature/console-ux-plugins` · **Wave 7** · **隔离等级**：高
- **主模块**：Console SPA（`console/src/`）—— 文章编辑器 `PostEditor.vue` + 插件库 `PluginLibrary.vue` + `MarkdownEditor.vue` + 若干列表/布局组件
- **状态**：📋 待开发。基于前台实测 6 条批注 +「组件控制台设置和预览」需求落地。

## 背景

Console 后台功能完备（文章/评论/附件/页面/瞬间/分类标签/友链/插件库/设置/备份），但在实际写作与管理时暴露若干 UX 细节问题，且写作时查阅/插入 Stellar 标签插件仍需切到独立插件库页手动复制。本 spec 做两类事：**(1) 6 处 UX 打磨**（表格标签换行、侧栏标题、Slug 汉化、分类标签可选项、友链卡片对齐）；**(2) 核心特性**：在文章编辑器内嵌可折叠「标签插件」写作面板，提供结构化参数表单 + 实时预览 + 一键插入正文，并把「设置和预览」抽为共享组件供独立插件库页复用。

**关键前置事实（代码审计确认）**：
- 分类/标签 API 已存在：`taxonomyApi.categories()` / `taxonomyApi.tags()` 返回 `{ items: [{name, count}] }`（`console/src/api/index.ts:152-165`、`TaxonomyList.vue:26-40` 消费）。**无 topics 端点**，故专栏（topic）不做可选项。
- `MarkdownEditor.vue` 已有 `wrapSelection()` / `insertBlock()`（`:183`、`:211`），但仅 `defineExpose({ clearDraft })`（`:168`），需追加暴露插入方法。
- `PluginLibrary.vue`（Spec 23）已实现 29 个标签插件的分类速查 + 可编辑示例 + iframe 实时预览（复用 `previewApi.render` 注入前台 `main.css`/`tag-plugins.css`），但**只有可编辑示例文本，无结构化参数表单**。
- `plugins-data.ts` 的 `PluginMeta.params` 是松散的 `{ name, desc, default }`，签名混用位置参数/命名参数/正文，**无法直接可靠拼装 shortcode**，需扩展可选 `form` schema。
- 全部改动落在 `console/src/`，**后端零改动**，与进行中的 Spec 28（Go/模板/Docker）无文件交集，可并行。

## P0 · UX 打磨（6 条批注）

### P0-1 文章列表状态标签换行
- **现状**：`PostList.vue:102-104` 状态 `<span>`（"已发布"）在 `w-20`（80px）列内被挤成两行——列内容区约 48px，"已发布"3 个 CJK 字 + span `px-2` 内边距超宽即折行。
- **改动**：状态 `<span>` 加 `whitespace-nowrap inline-block`；表头分类列（`:86`）与状态列（`:87`）`w-20`→`w-24`，给标签留足单行宽度。
- **文件**：`console/src/modules/posts/PostList.vue`。

### P0-2 侧栏标题改「管理后台」
- **现状**：`DefaultLayout.vue:43` 侧栏标题为 `Stellar Console`。
- **改动**：改为「管理后台」；`Login.vue` 若有同名标题一并对齐，保持登录页与后台一致。
- **文件**：`console/src/layouts/DefaultLayout.vue`、`console/src/modules/Login.vue`。

### P0-3 Slug 字段汉化
- **现状**：`PostEditor.vue:152` 发布设置里 label 为英文「Slug」，非技术用户不解其意。
- **改动**：label 改「别名 (Slug)」；输入框下方加灰字提示「用于文章 URL，留空自动生成」（`text-xs text-gray-400`）。
- **文件**：`console/src/modules/posts/PostEditor.vue`。

### P0-4 分类/标签可选项
- **现状**：`PostEditor.vue:172-186` 分类/专栏/标签均为纯输入框，用户不知已有哪些分类/标签，易拼写不一致产生碎片。
- **改动**：`onMounted` 并行拉 `taxonomyApi.categories()` / `taxonomyApi.tags()`；在分类、标签输入框下方渲染可点 chips：
  - 点分类 chip → 写入 `form.category`（单选，覆盖）。
  - 点标签 chip → 按逗号拆分去重后追加到 `form.tags`（已含则不重复加）。
  - chips 样式对齐 Halo 风（`text-xs px-2 py-0.5 rounded-full bg-gray-100 hover:bg-indigo-50`，深色适配）。
  - 专栏（topic）**无端点**，保持纯输入，不做 chips。
- **文件**：`console/src/modules/posts/PostEditor.vue`。

### P0-5 友链卡片标签对齐
- **现状**：`FriendList.vue:112-117` 名称 `<a>`（带 `truncate`）与 label `<span>` 同处 `flex items-center gap-2`，label 因内边距/行高与名称基线不齐，且 truncate 与标签争抢宽度时显拥挤。
- **改动**：label `<span>` 加 `shrink-0 whitespace-nowrap leading-none self-center`；名称 `<a>` 保留 `truncate` 且外层 `min-w-0`（已具备），消除垂直错位与挤压。
- **文件**：`console/src/modules/friends/FriendList.vue`。

## P1 · 核心特性：编辑器内嵌标签插件面板（批注 5 + 组件控制台）

编辑器内嵌可折叠面板 + 结构化参数表单；「设置和预览」抽为共享组件，独立插件库页与编辑器面板复用。

### P1-6 右侧设置栏可折叠
- **现状**：`PostEditor.vue:147-207` 右侧「发布设置 / 分类与标签 / 其他」三张卡片始终展开，纵向占位长；后续再加「标签插件」面板会更挤。
- **改动**：新建轻量 `components/CollapsibleSection.vue`（标题栏点击展开/收起，`v-model` 或内部 `ref` 记忆状态，箭头图标旋转）；三张设置卡改用之包裹，默认展开。
- **文件**：新增 `console/src/components/CollapsibleSection.vue`；改 `PostEditor.vue`。

### P1-7 结构化参数表单（共享组件）
- **改动**：`plugins-data.ts` 的 `PluginMeta` 扩展**可选** `form` schema：
  ```ts
  interface PluginFormField {
    key: string;                 // 参数名 / 占位标识
    label: string;               // 中文标签
    type: 'text' | 'select' | 'color' | 'textarea';
    role: 'positional' | 'named' | 'content'; // 拼装位置：位置参数 / key:value / 正文
    options?: string[];          // select 选项
    default?: string;
    placeholder?: string;
  }
  interface PluginMeta { /* ...原有字段... */ form?: PluginFormField[]; }
  ```
  新建 `modules/plugins/PluginParamForm.vue`：入参一个 `PluginMeta` → 按 `form` 渲染输入 → 依 `role` 拼装 shortcode（`named` 拼 `key:value`（值非空才拼）、`positional` 按序、`content` 作正文）→ 对外产出 shortcode 字符串（`v-model` 或 emit）。**无 `form` 的插件回退**到现有可编辑示例文本（`example`）。
- **schema 覆盖范围**：优先覆盖高频插件（note/quot/mark/button/tabs/timeline/gallery 等），其余回退示例文本，控制工作量。
- **文件**：新增 `console/src/modules/plugins/PluginParamForm.vue`；改 `console/src/modules/plugins/plugins-data.ts`。

### P1-8 编辑器内「标签插件」面板 + 插入
- **改动**：
  - `MarkdownEditor.vue` 追加 `defineExpose({ clearDraft, insertBlock, insertText })`（暴露已有插入能力）。
  - `PostEditor.vue` 右侧新增可折叠「标签插件」卡：紧凑分类筛选 + 插件列表 → 选中后嵌 `PluginParamForm`（表单 + 实时预览）→「插入到正文」按钮调用 `editorRef.insertBlock(shortcode)` 插入光标处。
  - 预览复用 `previewApi.render` + iframe srcdoc（注入 `/assets/css/main.css` + `tag-plugins.css`，对齐 `PluginLibrary.vue:85-103`）。
- **文件**：`console/src/components/MarkdownEditor.vue`、`console/src/modules/posts/PostEditor.vue`。

### P1-9 独立插件库页复用参数表单
- **现状**：`PluginLibrary.vue` 只有可编辑示例文本 + 预览。
- **改动**：在示例区上方或替换为 `PluginParamForm`（对有 `form` 的插件显示结构化表单，产出的 shortcode 同步驱动既有预览 + 复制）；无 `form` 插件保留可编辑示例。统一「设置和预览」体验。
- **文件**：`console/src/modules/plugins/PluginLibrary.vue`。

## 实现步骤（建议顺序）
1. **P0 打磨批次**（互不依赖，快速）：P0-1 → P0-2 → P0-3 → P0-4 → P0-5。每步 `npm run build` 验证类型 + 冒烟。
2. **P1 基础**：P1-6 CollapsibleSection → P1-7 plugins-data `form` schema + PluginParamForm（先覆盖高频插件）。
3. **P1 接入**：P1-8 MarkdownEditor 暴露插入 + PostEditor 标签插件面板 → P1-9 PluginLibrary 复用表单。
4. 全量 `npm run build` + 冒烟回归。

## 技术决策（无需再问）
- **纯前端实现，后端零改动**：复用已有端点（`/preview/markdown`、`/categories`、`/tags`），无新 API。
- **`PluginMeta.form` 为可选增量**：只给高频插件补 schema，其余回退现有示例文本，避免一次性覆盖 29 个插件的拼装规则，控制工作量与出错面。
- **共享 `PluginParamForm`**：编辑器面板与独立插件库页复用同一「设置 + 产出 shortcode」逻辑，DRY 且体验一致。
- **插入走编辑器既有能力**：`MarkdownEditor` 的 `insertBlock/wrapSelection` 已实现，仅 `defineExpose`，不重造。
- **隔离等级高**：所有改动在 `console/src/`，与 Spec 28（Go/模板/Docker/Cookie 等）无文件交集，Wave 7 可并行，无需协调合并顺序。
- **不做**：专栏（topic）可选项（无端点）、插件 `form` schema 全量覆盖 29 个、后端新增 API、富文本所见即所得编辑器、插件面板拖拽排序。

## Impact
- **新增**：`console/src/components/CollapsibleSection.vue`、`console/src/modules/plugins/PluginParamForm.vue`
- **修改**：`PostList.vue`、`DefaultLayout.vue`、`Login.vue`、`PostEditor.vue`、`FriendList.vue`、`MarkdownEditor.vue`、`PluginLibrary.vue`、`plugins-data.ts`
- **不影响**：后端全部（Go/模板/API）、前台渲染、其余 Console 模块（评论/附件/页面/瞬间/设置/备份等）

## ADDED Requirements

### Requirement: Console 列表与卡片视觉修正
The Console SHALL render status tags and friend-link labels on a single line without wrapping or misalignment.

#### Scenario: 文章状态标签
- **WHEN** 文章列表渲染"已发布"/"草稿"状态标签
- **THEN** 标签在状态列内单行显示，不折行

#### Scenario: 友链标签对齐
- **WHEN** 友链卡片名称旁带彩色 label
- **THEN** label 与名称垂直居中对齐，名称过长时 truncate 不挤压 label

### Requirement: 文章编辑元数据可发现性
The post editor SHALL localize the slug field and offer existing categories/tags as selectable options.

#### Scenario: Slug 汉化
- **WHEN** 打开新建/编辑文章
- **THEN** 别名字段显示中文标签「别名 (Slug)」与用途提示

#### Scenario: 分类/标签可选项
- **WHEN** 编辑文章的分类/标签区
- **THEN** 展示已有分类/标签 chips，点击写入分类或去重追加标签

### Requirement: 编辑器内标签插件写作面板
The post editor SHALL provide a collapsible tag-plugin panel with a structured parameter form, live preview, and one-click insertion into the article body.

#### Scenario: 结构化参数生成 shortcode
- **WHEN** 在标签插件面板选择一个具备参数 schema 的插件并填写参数
- **THEN** 自动拼出合法 shortcode 并实时预览渲染结果

#### Scenario: 插入正文
- **WHEN** 点击「插入到正文」
- **THEN** 生成的 shortcode 插入到 Markdown 编辑器当前光标处

#### Scenario: 无 schema 回退
- **WHEN** 选择的插件未定义参数 schema
- **THEN** 面板回退为可编辑示例文本 + 预览 + 插入

### Requirement: 设置与预览共享
The standalone plugin library SHALL reuse the same parameter-form and preview component as the editor panel.

#### Scenario: 插件库页参数表单
- **WHEN** 在插件库页选择具备参数 schema 的插件
- **THEN** 显示结构化参数表单，产出的 shortcode 同步驱动预览与复制

## 验证
1. `cd console && npm run build`（vue-tsc 类型检查 + vite 构建）通过
2. **P0-1**：文章列表"已发布"标签单行显示，不折行
3. **P0-2**：侧栏标题为「管理后台」，登录页标题一致
4. **P0-3**：编辑器别名字段中文标签 + 用途提示
5. **P0-4**：分类/标签 chips 可点，点分类写入、点标签去重追加；专栏保持纯输入
6. **P0-5**：友链卡片彩色 label 与名称垂直对齐，长名 truncate 不挤压
7. **P1-6**：右侧设置卡可折叠/展开，状态记忆
8. **P1-7/8**：标签插件面板选插件→填参数→预览正确→「插入到正文」得正确 shortcode 于光标处；无 schema 插件回退示例文本
9. **P1-9**：插件库页参数表单与预览、复制正常
10. 回归：既有 Console 模块（评论/附件/页面/瞬间/设置/备份/分类标签）功能不受影响

## Out of Scope
- 后端新增 API、topics 端点、标签规范化迁移
- 插件 `form` schema 全量覆盖 29 个（仅高频优先）
- 富文本所见即所得编辑器、插件面板拖拽排序/收藏
- 前台渲染逻辑改动（shortcode 解析在 `internal/post/shortcode.go`，本 spec 不碰）
