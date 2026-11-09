# BLG-RAWSURF01：收敛内容编辑器里的「Stellar 2 原文配置」入口

<!-- 状态词只用 未开始 / 实现中 / 待体验 / 已验收；已验收须用户明确确认。 -->

- 项目 / 任务编号：Blog / BLG-RAWSURF01
- 状态：待体验
- 创建日期（Asia/Taipei）：2026-11-10
- 权威记录：本文件；视觉与验收依据见 [docs/baselines/raw-config-entry/README.md](../baselines/raw-config-entry/README.md)
- 主仓库 / 当前工作目录：`/Users/sakurasep/Documents/Code/Project/Personal Project/Blog`（根 `dev`，保留用户 139 个未提交改动，不触碰）/ `/Users/sakurasep/Documents/Code/Project/Project Worktree/personal-blog/20261110-refactor-BLG-RAWSURF01-stellar-raw-config-entry`
- 基线 / 分支 / 合入目标：`d184fc41`（dev）/ `codex/refactor/20261110-BLG-RAWSURF01-stellar-raw-config-entry` / 本地 `dev`（未获授权前不合并、不推送）
- 共享 `resource/`：以绝对路径符号链接到根工作树，只读；本任务不涉及 CSS 重建。

## 目标与范围

内容编辑器的 7 个页面各挂一个 `<details>`「Stellar 2 原文配置」入口，文案三套、位置两套。本任务只收敛**呈现形态**，不动**字段覆盖**和 **UC 编辑能力**。

### 背景（根因，2026-11-10 核查）

1. `22f5c7fb`（2026-10-13）引入 `StellarContentConfig.vue`，同时接入 Post / Page / Topic / WikiDocs / WikiProjects / UC 六处。当时仓库里**没有** `useStellarForm` 与 `stellar-form-fields.ts`（`git ls-tree 22f5c7fb` 确认），该 textarea 是内容级配置的**唯一入口**，放进每个表单是合理的。
2. 同日 `849a9199` 落地基本表单同步层，常用字段改由表单接管；但 schema 覆盖不全，撤掉会丢编辑能力，于是组件从「主入口」降级为「逃生口」而**界面呈现未改**。
3. `9bbd8da2`（2026-10-21）才给组件加可选 `summary` prop，各调用点各写各的，默认值停在「Stellar 2 原文配置」→ 形成三套文案。
4. `b6c10929`（2026-11-07）新增笔记/笔记本时照抄两处，扩散到 7 处。
5. `1333f216`（2026-11-06）在 PageEditor 摘掉过，但该任务边界明写「其他编辑器共用组件不删除」，剩余入口无人处置。

### 已核实、本任务不改的事实

- schema 覆盖率低，是该入口暂时无法删除的客观原因：`page` 内容级 29 条路径表单覆盖 5 条（17%）；`collection` 内容级 40 条覆盖 18 条（45%）（`internal/stellarv2/schema.json` 对照 `stellar-form-fields.ts`；已排除 topbar/leftbar/rightbar/footer/comments 等结构继承类路径）。
- `UCPostEditorView` 未接 `useStellarForm`，只有 title / excerpt / tags 三个手写 input；该处入口仍是主入口，**彻底隐藏会真丢功能**，因此本任务不做隐藏。
- `TopicsView` 用 `collectionFields`（5 字段），`WikiProjectsView` 用 `wikiFields`（17 字段），同 profile 覆盖差 3 倍多。

## 已冻结的实现与边界

1. 7 处共用同一个 `StellarContentConfig.vue`；统一为一条紧凑入口，**默认折叠**，三种情况自动展开并显示诊断：① `syncError` 非空；② `version !== 2`（旧内容未迁移）；③ 导入回填（父组件显式传入）。
2. 不改任何保存契约、JSON 解析、字段覆盖、`useStellarForm` 同步语义；入口隐藏/展开不影响 `stellar_config` 与 `blog_extensions` 的读写。
3. 位置统一到各表单底部：UC 文章编辑与 WikiProjects 从表单顶部/正文上方移到末尾，与笔记/笔记本既有做法对齐。WikiDocs 移到正文编辑器下方。
4. `PostThemeSettings` 中 3 处「请在配置原文中编辑」的指引改为指向统一入口名，不留死路。
5. 删除已无任何 import 的 `modules/settings/tabs/AdvancedSettings.vue` 与 `modules/settings/ThemeYamlTab.vue`，同步处理 3 个测试文件里的 5 处断言。
6. 系统设置「代码与原文 → 完整配置原文」是**站点级**配置，与内容级无关，不动。

## 验收

- [ ] 7 个页面均只剩一条「高级：配置原文」入口，默认折叠，不再占整块卡片面积。
- [ ] 构造 JSON 语法错误时，入口自动展开并显示错误；修好后恢复折叠可行。
- [ ] `stellar_version !== 2` 的旧内容打开时自动展开并显示迁移提示。
- [ ] 入口展开后，原有「只校验」「导出 Markdown」能力与结论不变；保存后重开，`stellar_config` / `blog_extensions` 逐字未变。
- [ ] UC 文章编辑、WikiProjects 的入口位于表单底部；WikiDocs 位于正文编辑器下方。
- [ ] 1280 与 390 两个宽度下无横向溢出，入口与说明不遮挡相邻控件；手机点击区 ≥44px。
- [ ] `AdvancedSettings.vue` / `ThemeYamlTab.vue` 已删除，仓库内无残留 import，相关测试改为断言文件不存在。
- [ ] `make test` 通过；真实构建后浏览器核验加载的是本次产物。

## 执行记录

- 2026-11-10：改造 `StellarContentConfig.vue` 为一行入口（`details` + 自定义 `summary`），`needsAttention = syncError || version !== 2` 时经 `watch(flush:"post")` 直接写 `el.open`，保留用户手动开合；文案固定「高级：配置原文」，移除 `summary` prop 与三套自定义文案。
- 位置统一：UC 与 WikiProjects 移到底部、WikiDocs 移到正文编辑器之后；`EditorSidebar` 的 `post-settings-content-config` 卡片皮肤覆盖随之删除（新样式已自带边界，两者会冲突）。
- `PostThemeSettings` 3 处「请在配置原文中编辑」改为指向「高级：配置原文」，不留死路。
- 删除 `AdvancedSettings.vue`（327 行）与 `ThemeYamlTab.vue`：二者已无任何模块 import，只被测试读源码。同步把 3 个测试文件的耦合改成「文件已退役」断言，保存按钮计数 17 → 14（AdvancedSettings 带走 3 个）。
- 验证：`make test` 通过（562 项前端行为测试 + Go 全量）；`make build` 真实构建后重启服务，用 Playwright 在 1280/390 实测 7 个页面与自动展开、只校验、校验不改原文。

### 过程中发现并纠正

- 首次格式化误用 `prettier --write` 覆盖了 5 个文件，其中 `UCPostEditorView.vue`、`WikiDocsView.vue`、`WikiProjectsView.vue`、`settings-center.test.ts`、`settings-save-race.test.ts` 在 HEAD 本来就不符合 Prettier。已全部 `git checkout` 还原后重做，最终差异只含实质改动（位置移动 3–6 行、测试断言替换）。
- 首轮验收脚本的 3 个 FAIL 均为脚本问题：390 下设置面板是抽屉需先打开；笔记本/笔记路由需要 `:notebook` 参数且夹具库中无笔记本。修正脚本并建库后全部通过。

### 未验证

- 文库文档编辑页真实截图（夹具无 Wiki 文档，无文档时不渲染表单）。
- `stellar_version !== 2` 旧内容的自动展开（夹具是全新 2.0 空站点，无 1.x 内容）。

## 遗留与下一步

- 第二步（另立任务，需另行确认）：`TopicsView` 换用完整 collection 字段集；`UCPostEditorView` 接入 `useStellarForm`。在此之前该入口仍是 UC 与专栏的主要配置途径。
