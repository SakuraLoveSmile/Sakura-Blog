# Spec 46 · 文章版本行级 Diff 视图

- **分支**：`feature/revision-diff` · **Wave 8** · **隔离等级**：高（几乎纯 Console 前端）
- **主模块**：`console/src/modules/posts/RevisionPanel.vue`（Spec 22 产物扩展）
- **前置**：无

## Why

Halo 对照的部分实现项：版本历史（Spec 22）已有列表/查看/恢复/字数变化，但**无行级 diff 对比视图**——回看「这版到底改了哪几段」目前只能肉眼对照全文。补 diff 即补齐版本功能最后一块。

## 功能需求

1. RevisionPanel 版本列表任选两版（缺省：选中版 vs 当前正文）→「对比」视图：行级 diff，删除行红底、新增行绿底、上下文行灰色，支持逐块折叠未变化区域（>10 行未变化折叠显示「展开 n 行」）。
2. 并排/统一两种视图切换（缺省统一 unified）。
3. diff 视图内可直接「恢复到左侧版本」（复用既有恢复流程与确认弹窗）。
4. 移动端退化为统一视图。

## What Changes

- `console/src/modules/posts/`：+`RevisionDiff.vue`（diff 渲染组件）；RevisionPanel 加对比入口与两版选择态。
- **diff 算法前端实现**：+`console/src/modules/posts/diff.ts`——LCS 行级 diff（Myers 简化版，纯函数），**配套 `console/tests/diff.test.ts`**（node:test，遵循前端纯逻辑模块行为测试规范：空文本/全同/全异/交错增删/万行性能上限用例）。
- 后端零改动（`GET /api/v1/posts/:id/revisions/:rid` 已返回全文）。

## 实现步骤

1. diff.ts 纯函数 + 测试先行（TDD，规范要求）。
2. RevisionDiff.vue 渲染（行号/配色/折叠）。
3. Panel 集成（两版选择/视图切换/恢复按钮接线）。
4. 长文性能验证（5 千行文章 diff <200ms）。

## 技术决策（无需再问）

- **前端算 diff 不加后端端点**：两版全文已可获取，diff 是纯展示逻辑；后端加 diff 端点反而引入序列化格式约定成本。
- **行级不做字符级**：Markdown 写作场景行级已够语义；字符级高亮为后续增强（不承诺）。
- **不引 diff 库**：Myers 行级实现 <150 行，与项目「纯逻辑自实现 + node:test」惯例一致（参照 shortcode.ts 模式）。

## Impact

- **新增**：diff.ts（+test）、RevisionDiff.vue
- **修改**：RevisionPanel.vue
- **不影响**：后端全部、其他 Console 模块

## 验证

- [ ] 任选两版 diff 正确（构造增/删/改混合用例目检）
- [ ] 折叠/展开、并排/统一切换、diff 内恢复全可用
- [ ] diff.test.ts 全用例绿（含性能上限）
- [ ] `make test` 全绿
