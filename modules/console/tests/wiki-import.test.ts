// 行为面：文库「导入文章」的纯规划逻辑（src/modules/wiki/wiki-import.ts）——
// 单篇可选状态、选择集、提交载荷、结果摘要与失败原因展示。
// 机械校验入口：make test → console-test → tsx --test ../tests/*.test.ts
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  buildImportPayload,
  importIssueText,
  importSummary,
  normalizeSlugs,
  postImportState,
  selectableSlugs,
  toggleSelection,
  type ImportablePost,
} from "../web/src/modules/wiki/wiki-import.ts";

const existing = new Set(["already-imported"]);

test("可选状态：已导入与回收站文章不可选，其余可选", () => {
  assert.equal(postImportState({ slug: "fresh", status: "published" }, existing), "importable");
  assert.equal(postImportState({ slug: "fresh", status: "draft" }, existing), "importable");
  assert.equal(postImportState({ slug: "already-imported", status: "published" }, existing), "imported");
  assert.equal(postImportState({ slug: "deleted", status: "trashed" }, existing), "trashed");
});

test("全选本页只选可选文章，保持列表顺序", () => {
  const page: ImportablePost[] = [
    { slug: "b", status: "published" },
    { slug: "already-imported", status: "published" },
    { slug: "a", status: "draft" },
    { slug: "deleted", status: "trashed" },
  ];
  assert.deepEqual(selectableSlugs(page, existing), ["b", "a"]);
});

test("选择集切换不修改入参，可跨页累加与取消", () => {
  const first = toggleSelection(new Set<string>(), "b", true);
  const second = toggleSelection(first, "a", true);
  assert.deepEqual([...second].sort(), ["a", "b"]);
  assert.equal(first.size, 1, "原有选择集不应被就地修改");

  const removed = toggleSelection(second, "b", false);
  assert.deepEqual([...removed], ["a"]);
  assert.equal(second.size, 2);
});

test("提交载荷去空白、去重并保留首次出现顺序；父文档与状态透传", () => {
  assert.deepEqual(normalizeSlugs([" b ", "a", "", "b ", "  "]), ["b", "a"]);
  assert.deepEqual(buildImportPayload(["b", "b", " a "], 0, "draft"), {
    slugs: ["b", "a"],
    parent: 0,
    status: "draft",
  });
  assert.deepEqual(buildImportPayload(["b"], -1, "published"), {
    slugs: ["b"],
    parent: 0,
    status: "published",
  });
  assert.deepEqual(buildImportPayload(["b"], 12, "published").parent, 12);
});

test("结果摘要分别计数，空结果给出明确文案", () => {
  assert.equal(importSummary({ imported: [1, 2], skipped: [1], failed: [] }), "已导入 2 篇；跳过 1 篇");
  assert.equal(importSummary({ imported: [], skipped: [], failed: [1] }), "失败 1 篇");
  assert.equal(importSummary({ imported: [], skipped: [], failed: [] }), "没有导入任何文档");
});

test("跳过/失败条目优先展示标题，并附后端原因", () => {
  assert.equal(
    importIssueText({ slug: "a", title: "文章 A", reason: "项目内已存在同名 Slug 文档" }),
    "文章 A：项目内已存在同名 Slug 文档",
  );
  assert.equal(importIssueText({ slug: "a", title: "  ", reason: "" }), "a");
  assert.equal(importIssueText({ slug: "a", title: "A" }), "A");
});
