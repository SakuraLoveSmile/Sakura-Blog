> **历史文档：旧目录结构（已废弃）**

> 路径说明：文中本机绝对路径已脱敏。执行历史命令前，将 `BLOG_ROOT` 设置为对应版本的仓库根目录（例如从根目录执行 `export BLOG_ROOT="$PWD"`）。
>
> 本文档记录的是模块化重构前的路径，仅作历史参考，不应据此修改现行代码。
>
> 路径映射：
> - `frontend/console/` → `modules/console/web/`
> - `frontend/assets/` → `modules/stellar/web/assets/`
> - `backend/cmd/server/` → `modules/stellar/routes/` + `internal/`
> - `backend/internal/` → `internal/`

# Console Apple Dark Mode 重新设计实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `frontend/console` Vue SPA 从 Halo 薄荷绿风格整体迁移到 Apple Dark Mode 设计语言（纯黑底 #000000、Apple 蓝 #0A84FF、大圆角、毛玻璃、彩色辉光），先重构设计令牌与基础组件，再逐页对齐设计稿。

**Architecture:** 在 Tailwind v4 `@theme` 中重定义设计令牌（颜色/圆角/字体/阴影），重构 20 个 `V*` 基础组件为 Apple 风格，清理深色-only 下冗余的 `dark:` 变体与浅色硬编码，最后逐页改造 10 个模块。保留现有 Tiptap/Markdown 编辑器逻辑，仅改外层视觉。不引入第三方 UI 库，不引入 CodeMirror/Monaco。

**Tech Stack:** Vue 3.5 + TypeScript + Vite + Tailwind CSS v4 (`@tailwindcss/vite`) + @iconify/vue (Remix Icon) + Tiptap + chart.js + Pinia + vue-router + @tanstack/vue-query。

**参照设计稿（唯一验收基准）：** `resource/design/设计稿/_extracted/<模块名>/pages/<英文名>.html`

**核心设计令牌（从设计稿提取）：**

```css
/* 背景层级 */
--color-background: #000000;
--color-card: #1c1c1e;
--color-secondary: #2c2c2e;
--color-accent: #3a3a3c;
/* 品牌色 */
--color-primary: #0A84FF;
--color-primary-hover: #409CFF;
--color-primary-active: #0062CC;
/* 系统色 */
--color-state-success: #30D158;
--color-state-warning: #FF9F0A;
--color-state-error: #FF453A;
/* 圆角 */
--radius-sm: 8px;
--radius-base: 10px;
--radius-lg: 14px;
--radius-xl: 18px;
--radius-full: 9999px;
/* 字体 */
--font-sans: -apple-system, BlinkMacSystemFont, "SF Pro Display", "PingFang SC", sans-serif;
--font-mono: "SF Mono", "JetBrains Mono", Menlo, monospace;
/* 阴影 */
--shadow-sm: 0 1px 2px rgba(0,0,0,0.4);
--shadow-md: 0 4px 12px -2px rgba(0,0,0,0.5);
--shadow-lg: 0 12px 32px -8px rgba(0,0,0,0.6);
--shadow-glow-blue: 0 0 24px rgba(10,132,255,0.25);
```

**全局验收清单（所有页面完成后逐项核对）：**
1. 页面背景纯黑 `#000000`，卡片 `#1c1c1e`，二级容器 `#2c2c2e`
2. 主色（按钮、激活态、链接）为 Apple 蓝 `#0A84FF`，无薄荷绿 `#4ccba0` 残留
3. 卡片圆角 14px，按钮胶囊形或 10px
4. 无 `bg-white`/`text-gray-900`/`dark:` 等浅色基础值残留
5. 无 `#1f2937`/`#374151`/`#4ccba0`/`#d71d1d`/`#0e1731` 等旧硬编码
6. 主按钮 hover 有上浮 + 蓝色辉光
7. iOS 风格开关（白色滑块 + 绿色开启）
8. 侧边栏毛玻璃 `backdrop-filter: saturate(180%) blur(24px)`
9. 字体栈使用 SF Pro / PingFang SC
10. `npm run test`（vue-tsc + node:test）全绿，`go vet ./backend/... ./frontend/...` 通过

---

## 文件结构总览

### 将修改的文件

**设计系统层：**
- `frontend/console/src/styles/main.css` — 重写 @theme 令牌
- `frontend/console/src/styles/custom-theme.css` — 调整过渡/骨架屏为深色值
- `frontend/console/index.html` — 更新 theme-color meta

**新建：**
- `frontend/console/src/styles/apple-base.css` — 全局基础样式（滚动条、选区、字体、光晕、渐变头像、分段控件、表单类）

**基础组件层（src/components/ui/）：** VButton、VCard、VTag、VPageHeader、VPagination、VSwitch、VModal、VTabbar、VStatusDot、VEmpty、VLoading、VSpace、VEntity 体系、FilterDropdown、FilterTag、dialog.ts、toast.ts

**布局层：** `frontend/console/src/layouts/DefaultLayout.vue`

**页面层（src/modules/）：** taxonomy、notebooks、users、moments、comments、notifications、attachments、wiki、settings、posts 编辑器外层

### 不修改
- MarkdownEditor/RichTextEditor/ContentEditor 内核
- 所有 Go 后端文件
- `admin-console-prototype/`

---

## Task 1: 重写设计令牌

**Files:**
- Modify: `frontend/console/src/styles/main.css`
- Create: `frontend/console/src/styles/apple-base.css`
- Modify: `frontend/console/src/styles/custom-theme.css`
- Modify: `frontend/console/index.html`

- [ ] **Step 1:** 用本计划「核心设计令牌」+ 完整 Apple 色板（含 purple/pink/indigo/teal/cyan/yellow、sidebar、border-strong、text-secondary/tertiary/quaternary、chart、radius-xs/2xl、shadow-xl/glow-green）重写 `main.css` 的 `@theme` 块，并在 `@layer base` 中设置 body 背景/字体。`@import` 顺序为 tailwindcss → custom-theme.css → apple-base.css。
- [ ] **Step 2:** 创建 `apple-base.css`，包含：选区蓝、Apple 滚动条（8px、rgba(84,84,88,0.5)）、`color-scheme: dark`、焦点环、`.apple-card-highlight`（顶部 1px 高光）、`.apple-ambient-glow`（右上蓝光+左下紫光 fixed 伪元素）、`.avatar-gradient-1..8`、`.notebook-cover-blue/purple/green/orange/pink/teal`、`.apple-segmented`（胶囊分段）、`.apple-segmented-rect`（矩形分段）、`.apple-input`/`.apple-textarea`/`.apple-select`/`.apple-search`（统一表单控件，focus 蓝色光环）。
- [ ] **Step 3:** 更新 `index.html` 的 `theme-color` 为 `#000000`。
- [ ] **Step 4:** 更新 `custom-theme.css`：`.hover-lift:hover` 阴影改为 `var(--shadow-lg)`；`.skeleton` 默认值改为深色渐变（`#1c1c1e`/`#2c2c2e`），删除 `.dark .skeleton` 规则。保留所有过渡动画。
- [ ] **Step 5:** 验证：`cd frontend/console && npm run test`（若 dist 缺失先 `make dist-bootstrap`）。
- [ ] **Step 6:** 提交：`feat(console): 切换到 Apple Dark Mode 设计令牌`

---

## Task 2: 重构 VButton

**Files:** Modify `frontend/console/src/components/ui/VButton.vue`

- [ ] **Step 1:** 保留 script/template，重写 `<style scoped>`：去 `!important`；`.btn` 高 36px、胶囊圆角、font-weight 500；`.btn-primary` 蓝底，hover `--color-primary-hover` + `translateY(-1px)` + `--shadow-glow-blue`；`.btn-secondary` 深灰底（`--color-secondary`）；`.btn-danger` 红底 + 红色辉光；`.btn-default` 描边卡片风；尺寸 lg 44px / sm 28px / xs 24px；circle 变体保持圆形。
- [ ] **Step 2:** 验证 `npm run test`。
- [ ] **Step 3:** 提交：`feat(console): VButton 切换 Apple 风格胶囊按钮`

---

## Task 3: 重构 VCard、VPageHeader、VTag

**Files:** VCard.vue、VPageHeader.vue、VTag.vue

- [ ] **Step 1:** VCard style 令牌化：bg `var(--color-card)`、border `var(--color-border)`、radius `var(--radius-lg)`、shadow `var(--shadow-sm)`；加 `::before` 顶部 1px 高光渐变；header/body/footer padding 调整为 14px 18px / 16px 18px。
- [ ] **Step 2:** VPageHeader 改为 sticky 顶栏：`backdrop-blur-xl` + `background: rgba(0,0,0,0.72)` + 底边框；标题 22px bold，副标题 13px tertiary；actions 区加 gap-2。
- [ ] **Step 3:** VTag style 令牌化：高 22px、12px、padding 0 8px、radius `var(--radius-sm)`；primary 蓝、secondary accent、danger 红、default secondary 底 + border-strong；保留 rounded 胶囊选项。
- [ ] **Step 4:** 验证 `npm run test`。
- [ ] **Step 5:** 提交：`feat(console): VCard/VPageHeader/VTag 切换 Apple 风格`

---

## Task 4: 重构 VSwitch、VModal、VPagination、VStatusDot

**Files:** VSwitch.vue、VModal.vue、VPagination.vue、VStatusDot.vue

- [ ] **Step 1:** VSwitch 改为 51×31 iOS 开关：开启 `bg-state-success`，关闭 `bg-accent`，白色 27px 圆形滑块，translate-x-5。
- [ ] **Step 2:** VModal 令牌化：遮罩 `rgba(0,0,0,0.6)` + `backdrop-blur-sm`；容器 `bg-card border-border rounded-xl shadow-xl`；header/footer 边框令牌化；关闭按钮 hover `bg-secondary`；footer 右对齐 gap-2。
- [ ] **Step 3:** VPagination 模板令牌化：页码按钮 h-8 min-w-8 rounded-lg，激活 `bg-primary text-white shadow-glow-blue`，hover `bg-secondary`；总数文字 tertiary；select 用 `apple-select`。
- [ ] **Step 4:** VStatusDot 硬编码色（#4ccba0/#f59e0b/#d71d1d/#9ca3af）替换为 `var(--color-state-success/warning/error)` 和 `var(--color-text-tertiary)`。
- [ ] **Step 5:** 验证 `npm run test`。
- [ ] **Step 6:** 提交：`feat(console): VSwitch/VModal/VPagination/VStatusDot Apple 风格`

---

## Task 5: 重构 VTabbar、VEntity 体系、Filter 组件、反馈组件

**Files:** VTabbar.vue、VEntity.vue、VEntityContainer.vue、VEntityField.vue、FilterDropdown.vue、FilterTag.vue、dialog.ts、toast.ts、VEmpty.vue、VLoading.vue

- [ ] **Step 1:** VTabbar：default variant 高亮条改 `var(--color-primary)`；outline variant 改胶囊分段（radius-full，激活 `bg-accent` + shadow-sm）；清除 `#0e1731`/`#4ccba0`。
- [ ] **Step 2:** VEntity 体系：divide 边框令牌化，行 hover `bg-[rgba(255,255,255,0.03)]`，文字色改 text-secondary/tertiary；选中指示条保留 `bg-primary`。
- [ ] **Step 3:** FilterDropdown 弹层 `bg-card border-border shadow-lg rounded-lg`；FilterTag `bg-secondary text-text-secondary`，关闭 hover `bg-accent`。
- [ ] **Step 4:** dialog.ts/toast.ts：info 图标 `var(--color-primary)`，warning `var(--color-state-warning)`，error `var(--color-state-error)`，success `var(--color-state-success)`；容器 bg-card、border-border、radius-lg（dialog）。
- [ ] **Step 5:** VEmpty 文字 tertiary，SVG 用 currentColor；VLoading spin 用 `var(--color-primary)`。
- [ ] **Step 6:** 验证 `npm run test`。
- [ ] **Step 7:** 提交：`feat(console): VTabbar/VEntity/Filter/反馈组件令牌化`

---

## Task 6: 重构 DefaultLayout 侧边栏

**Files:** `frontend/console/src/layouts/DefaultLayout.vue`

- [ ] **Step 1:** 根容器加 `apple-ambient-glow bg-background text-foreground`，去除 `bg-gray-50 dark:bg-gray-900`。
- [ ] **Step 2:** `<aside>` 加 `border-r border-border` + 内联 `background: var(--color-sidebar); backdrop-filter: saturate(180%) blur(24px)`。
- [ ] **Step 3:** 搜索框改 `rounded-full bg-secondary p-2.5 text-text-tertiary`。
- [ ] **Step 4:** style 中菜单激活态：`.menu-item-title` 默认 text-secondary，hover/active `background: rgba(58,58,60,0.6)` + text-foreground，圆角 8px；`.active::after` 竖条改 `var(--color-primary)`；subitem 同步；menu-label 改 text-quaternary。
- [ ] **Step 5:** 底部 current-profile 背景 `var(--color-sidebar)`；铃铛/更多按钮 hover `bg-accent`；角标 `bg-state-error`；下拉菜单 `bg-card border-border shadow-xl`，菜单项 hover `bg-secondary`。
- [ ] **Step 6:** 模板中所有 `bg-white`/`bg-gray-*`/`text-gray-*`/`border-gray-*`/`dark:*` 替换为令牌等价类（bg-card/bg-secondary/bg-accent/text-foreground/text-text-secondary/text-text-tertiary/border-border）。
- [ ] **Step 7:** 搜索弹窗、移动端抽屉同理令牌化；遮罩改 `rgba(0,0,0,0.6)` + blur；draft 徽章 `rgba(255,159,10,0.15)`+`#FF9F0A`，trashed `rgba(255,69,58,0.15)`+`#FF453A`；snippet 高亮改 `var(--color-primary)`。
- [ ] **Step 8:** 页脚令牌化 border-border/text-text-quaternary。
- [ ] **Step 9:** 移动端底部导航保留 bg-secondary/bg-black（已是深色）。
- [ ] **Step 10:** 验证 `npm run test`。
- [ ] **Step 11:** 提交：`feat(console): DefaultLayout 侧边栏切换毛玻璃 Apple 风格`

---

## Task 7: 全局清理深色-only 冗余与旧硬编码

**Files:** `frontend/console/src/` 下所有未改造的 .vue/.ts/.css

- [ ] **Step 1:** `grep -rn "#4ccba0\|#0e1731\|#1f2937\|#374151\|#d71d1d\|#1a2a5c" --include="*.vue" --include="*.ts" --include="*.css" .` 逐个替换为令牌。
- [ ] **Step 2:** `grep -rn "bg-white\|bg-gray-50\|bg-gray-100\|bg-gray-200\|text-gray-900\|text-gray-800\|text-gray-700\|border-gray-100\|border-gray-200" --include="*.vue" .` 替换为 bg-card/bg-secondary/bg-accent/text-foreground/text-text-secondary/border-border；配套 `dark:bg-gray-*`/`dark:text-gray-*` 删除。
- [ ] **Step 3:** 各模块 `inputCls` 内联常量统一替换为 `apple-input`/`apple-textarea`/`apple-select`。
- [ ] **Step 4:** `dashboard.css` 和 chart.js 颜色配置改为 Apple 系统色（蓝 #0A84FF、紫 #BF5AF2、绿 #30D158、橙 #FF9F0A）。
- [ ] **Step 5:** 验证：`cd frontend/console && npm run test` + `go vet ./backend/... ./frontend/...`（在 worktree 根执行）。
- [ ] **Step 6:** 提交：`refactor(console): 全局清理深色-only 冗余 dark 变体与旧硬编码色`

---

## Task 8: 分类标签页（TaxonomyList）

**参照:** `resource/design/设计稿/_extracted/分类标签/pages/categories-tags.html`
**Files:** `frontend/console/src/modules/taxonomy/TaxonomyList.vue`

**验收清单：**
- [ ] sticky 页头：标题 22px + 副标题 + 胶囊搜索框 + 主按钮
- [ ] 胶囊分段控件切换「分类」「标签」
- [ ] 分类为 2 列卡片网格，每卡含 48px 彩色渐变图标盒、名称、描述、文章数徽章、编辑/删除图标按钮
- [ ] 标签为流式标签云，标签带计数小圆角，颜色循环 7 色
- [ ] 卡片 hover translateY(-1px) + shadow-lg
- [ ] 保留所有现有 API/CRUD 逻辑；`prompt()` 重命名改为 VModal 或内联编辑
- [ ] 验证 `npm run test`，提交 `feat(console): 分类标签页对齐 Apple 设计稿`

---

## Task 9: 笔记本页（NotebooksView）

**参照:** `resource/design/设计稿/_extracted/笔记本/pages/notebooks.html`
**Files:** `frontend/console/src/modules/notebooks/NotebooksView.vue`

**验收清单：**
- [ ] 顶栏 240px 矩形搜索 + 网格/列表视图切换 + 新建按钮
- [ ] 3 列卡片网格，封面高 120px，使用 `.notebook-cover-*` 6 种渐变（按 id 取色）
- [ ] 封面含毛玻璃图标圆、更多按钮、篇数胶囊徽章
- [ ] 卡片 hover translateY(-4px) + shadow-lg
- [ ] 两级视图（笔记本→笔记编辑）功能保留
- [ ] 验证提交：`feat(console): 笔记本页对齐 Apple 设计稿`

---

## Task 10: 用户管理页（UserListView）

**参照:** `resource/design/设计稿/_extracted/用户管理/pages/users.html`
**Files:** `frontend/console/src/modules/users/UserListView.vue`

**验收清单：**
- [ ] 4 列统计卡（从现有 API 数据派生），顶部 1px 高光，hover 上浮 2px
- [ ] 统计卡 40px 图标盒、32px 等宽数字（font-variant-numeric: tabular-nums）
- [ ] 表格卡头部胶囊分段筛选 + 迷你搜索
- [ ] 无头像时用 `avatar-gradient-*` 渐变圆 + 首字母（按用户名 hash 取色）
- [ ] 角色徽章：管理员粉 #FF375F、编辑紫 #BF5AF2、订阅者蓝 #0A84FF（15% 透明底胶囊）
- [ ] 状态药丸：活跃绿点辉光、禁用红点辉光
- [ ] 验证提交：`feat(console): 用户管理页对齐 Apple 设计稿`

---

## Task 11: 瞬间页（MomentList）

**参照:** `resource/design/设计稿/_extracted/瞬间/pages/moments.html`
**Files:** `frontend/console/src/modules/moments/MomentList.vue`

**验收清单：**
- [ ] 发布框保留功能、视觉对齐卡片
- [ ] 列表居中最大宽 680px，左侧 1px 垂直渐变线 + 10px 彩色节点（带外环 + 辉光）
- [ ] 按日期分组（今天/昨天/更早），日期分隔符
- [ ] 瞬间卡片：渐变头像、作者、时间、正文（#标签蓝色）、3 列图片网格（gap 6px、rounded 10px）、链接卡片、操作栏
- [ ] 筛选 tabs 胶囊分段（全部/文字/图片/链接）
- [ ] 验证提交：`feat(console): 瞬间页对齐 Apple 设计稿`

---

## Task 12: 评论管理页（CommentList）

**参照:** `resource/design/设计稿/_extracted/评论管理/pages/comments.html`
**Files:** `frontend/console/src/modules/comments/CommentList.vue`

**验收清单：**
- [ ] 4 列统计卡（总评论/待审核/已批准/垃圾评论），数字 34px 等宽，底部彩色描述徽章
- [ ] 列表卡头部胶囊分段 tabs 带计数徽章（全部/待审核/已批准/已拒绝/垃圾评论）
- [ ] 评论项：42px 渐变头像、作者+邮箱、状态徽章（approved 绿/pending 橙/rejected 红/spam 灰，带圆点）、时间、正文（3 行截断）、上下文链接
- [ ] 操作按钮：通过（绿调底）、回复（蓝调底）、删除（红调底），低透明胶囊
- [ ] 保留批量选择、FilterDropdown、刷新功能
- [ ] 验证提交：`feat(console): 评论管理页对齐 Apple 设计稿`

---

## Task 13: 通知中心页（NotificationsView）

**参照:** `resource/design/设计稿/_extracted/通知中心/pages/notifications.html`
**Files:** `frontend/console/src/modules/notifications/NotificationsView.vue`

**验收清单：**
- [ ] 未读摘要横幅（显示未读数）
- [ ] 胶囊分段筛选（全部/系统/评论/互动/提醒）
- [ ] 按日期分组（今天/昨天/更早）
- [ ] 通知卡片：图标（4 种颜色变体）、未读圆点带辉光、标题、正文、时间
- [ ] 未读卡带蓝色左边框 `3px solid var(--color-primary)` + 略深背景
- [ ] 侧边栏铃铛角标保留
- [ ] 验证提交：`feat(console): 通知中心页对齐 Apple 设计稿`

---

## Task 14: 附件管理页（AttachmentList）

**参照:** `resource/design/设计稿/_extracted/附件管理/pages/attachments.html`
**Files:** `frontend/console/src/modules/attachments/AttachmentList.vue`

**验收清单：**
- [ ] 存储卡：已用/总容量 + 多色渐变进度条 + 存储明细（彩色圆点+类型+大小）
- [ ] 胶囊分段筛选（全部/图片/文档/视频/音频/其他）
- [ ] 网格/列表视图切换
- [ ] 4 列附件卡片网格，每卡缩略图/类型图标 + 文件名+大小+时间
- [ ] hover 显示半透明黑色毛玻璃遮罩 + 操作按钮（下载/预览/删除）
- [ ] 左侧分组列表保留，选中 `bg-primary/10 text-primary`
- [ ] 验证提交：`feat(console): 附件管理页对齐 Apple 设计稿`

---

## Task 15: 文库页（WikiDocsView）

**参照:** `resource/design/设计稿/_extracted/文库/pages/wiki.html`
**Files:** `frontend/console/src/modules/wiki/WikiDocsView.vue`

**验收清单：**
- [ ] 保留文档树+编辑器的功能结构
- [ ] 文档树卡片：选中项 `background: rgba(10,132,255,0.08)` + 3px 蓝色左边框
- [ ] 文件夹展开 chevron 旋转动画
- [ ] 顶部 3 列统计卡（总文档/分类/本月更新，数据从现有 API 派生）
- [ ] 面包屑导航
- [ ] 表单控件统一用 apple-input/apple-select
- [ ] 验证提交：`feat(console): 文库页对齐 Apple 设计稿`

---

## Task 16: 设置页（SettingsView）

**参照:** `resource/design/设计稿/_extracted/设置/pages/settings.html`
**Files:** `frontend/console/src/modules/settings/SettingsView.vue`

**验收清单：**
- [ ] 保留现有两级 Tab 功能，但视觉改为：左侧 200px 子导航 + 右侧表单卡片（最大宽 1100px 居中）
- [ ] 左侧导航激活项 3px 蓝色左边框 + 蓝字 + rgba(10,132,255,0.08) 背景
- [ ] 表单卡片：标题+描述+卡片，输入控件统一 apple-input/apple-textarea/apple-select
- [ ] iOS 开关（已用新 VSwitch）
- [ ] 矩形分段控件（主题模式/字号）用 `.apple-segmented-rect`
- [ ] 颜色选择器预设圆点
- [ ] 警告提示用橙色文字+图标
- [ ] 代码 textarea 用等宽字体 + `#0f0f10` 底
- [ ] 验证提交：`feat(console): 设置页对齐 Apple 设计稿`

---

## Task 17: 文章编辑器外层视觉（PostEditor 及子组件）

**参照:** `resource/design/设计稿/_extracted/文章编辑/pages/post-editor.html`
**Files:** `frontend/console/src/modules/posts/PostEditor.vue`、`EditorToolbar.vue`、`EditorSidebar.vue`

**注意：** 保留 Tiptap/RichTextEditor/MarkdownEditor 内核，不改编辑器技术栈，仅改外层布局/工具栏/侧栏面板视觉。

**验收清单：**
- [ ] 顶部 sticky 紧凑页头：返回 + 标题 + 预览/保存草稿/发布（ghost/secondary/primary 三按钮），背景 `rgba(0,0,0,0.75)` + blur(20px)
- [ ] 编辑区左右两栏：左自适应编辑区、右 320px 设置面板，gap 24px
- [ ] 工具栏分组按钮（32px、rounded 7px、hover 浅底、激活蓝底白字），分组间左边框分隔
- [ ] 右侧设置面板 4 张卡片：发布设置、分类标签、封面设置、高级选项
- [ ] 封面预览 16:9 深色渐变占位
- [ ] 草稿条幅保留但令牌化颜色
- [ ] 标题输入框 28px bold、无边框透明
- [ ] 验证提交：`feat(console): 文章编辑器外层对齐 Apple 设计稿`

---

## Task 18: 全量验证与收尾

**Files:** 无新增改动，仅验证。

- [ ] **Step 1:** 在 worktree 根执行完整测试：
  ```bash
  cd "$BLOG_ROOT"
  make test
  ```
  预期：console-test（vue-tsc + node:test）、assets-test、go vet、go test 全部通过。
- [ ] **Step 2:** 完整构建：`make build`，确认 go:embed 成功嵌入新 dist。
- [ ] **Step 3:** 启动 dev 服务目视验证所有 10 个页面：`make dev` 或 `go run ./backend/cmd/server/ -root $(pwd) -dev`，端口 8080。对照「全局验收清单」逐项核对。
- [ ] **Step 4:** 用浏览器 DevTools 确认无 `#4ccba0`、`#1f2937`、`bg-white` 等旧色残留（Computed 面板采样）。
- [ ] **Step 5:** 若发现问题，修复后回到 Step 1。
- [ ] **Step 6:** 最终提交（如有修复）：`fix(console): Apple Dark Mode 收尾修复`

---

## Self-Review 记录

**Spec 覆盖：** 10 个设计稿页面分别对应 Task 8-17（分类标签、笔记本、用户、瞬间、评论、通知、附件、文库、设置、文章编辑）。设计系统层 Task 1-7。全量验证 Task 18。全部覆盖。

**占位符扫描：** 无 TBD/TODO；每个 Task 有明确文件、验收清单、提交信息。具体代码在 Task 1-7 已给出关键令牌值，页面 Task 8-17 以验收清单形式描述（因每个页面需先读取现有代码再重构，保留实现弹性）。

**类型一致性：** 令牌命名 `--color-primary`/`--color-card`/`--color-secondary`/`--color-accent`/`--color-state-*`/`--color-text-*`/`--radius-*`/`--shadow-*` 在所有 Task 中一致。Tailwind v4 由 `@theme` 自动生成 `bg-primary`/`text-foreground`/`bg-card`/`rounded-lg`/`shadow-lg` 等工具类。
