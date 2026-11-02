// 行为面：文章批量发布 / 批量草稿的纯规划逻辑（src/modules/posts/batch-status.ts）——
// 目标集合、跳过/回收站计数、按钮禁用原因、确认弹窗文案与 Toast 汇总。
// 机械校验入口：make test → console-test → tsx --test ../tests/*.test.ts
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import {
  batchStatusToast,
  planBatchStatus,
  type BatchStatusAction,
  type BatchStatusPost,
} from "../web/src/modules/posts/batch-status.ts";

// 当前页混合状态：已发布（含/不含草稿头）、草稿（含/不含草稿头）、回收站。
const page: BatchStatusPost[] = [
  { slug: "published", status: "published" },
  { slug: "published-with-draft", status: "published", has_draft: true },
  { slug: "draft", status: "draft" },
  { slug: "draft-with-draft", status: "draft", has_draft: true },
  { slug: "trashed", status: "trashed" },
];
const pageSlugs = page.map((post) => post.slug);

test("回收站文章不进入目标集合，只计入 trashed", () => {
  const publish = planBatchStatus(page, ["trashed", "draft"], "publish");
  assert.deepEqual(publish.targets, ["draft"]);
  assert.equal(publish.count, 1);
  assert.equal(publish.skipped, 0);
  assert.equal(publish.trashed, 1);
  assert.equal(publish.disabledReason, null);

  for (const action of ["publish", "draft"] as const) {
    const onlyTrashed = planBatchStatus(page, ["trashed"], action);
    assert.deepEqual(onlyTrashed.targets, []);
    assert.equal(onlyTrashed.count, 0);
    assert.equal(onlyTrashed.trashed, 1);
    assert.equal(onlyTrashed.disabledReason, "回收站文章需先恢复再设置状态");
  }
});

test("已是目标状态的文章进入 skipped，不再提交", () => {
  const publish = planBatchStatus(page, ["published", "published-with-draft", "draft"], "publish");
  assert.deepEqual(publish.targets, ["draft"]);
  assert.equal(publish.count, 1);
  assert.equal(publish.skipped, 2);
  assert.equal(publish.trashed, 0);

  const draft = planBatchStatus(page, ["draft", "draft-with-draft", "published"], "draft");
  assert.deepEqual(draft.targets, ["published"]);
  assert.equal(draft.count, 1);
  assert.equal(draft.skipped, 2);
});

test("混合选择计数：目标 / 跳过 / 回收站 / 草稿修改互不重叠", () => {
  const publish = planBatchStatus(page, pageSlugs, "publish");
  assert.equal(publish.targetStatus, "published");
  assert.deepEqual(publish.targets, ["draft", "draft-with-draft"]);
  assert.equal(publish.count, 2);
  assert.equal(publish.skipped, 2);
  assert.equal(publish.trashed, 1);
  assert.equal(publish.hasDraft, 1);
  assert.equal(publish.disabledReason, null);

  const draft = planBatchStatus(page, new Set(pageSlugs), "draft");
  assert.equal(draft.targetStatus, "draft");
  assert.deepEqual(draft.targets, ["published", "published-with-draft"]);
  assert.equal(draft.count, 2);
  assert.equal(draft.skipped, 2);
  assert.equal(draft.trashed, 1);
  assert.equal(draft.hasDraft, 1);
  assert.equal(draft.disabledReason, null);
});

test("目标集合保持当前页顺序，与选中顺序无关；不在当前页的 slug 被忽略", () => {
  const reversed = planBatchStatus(page, ["draft-with-draft", "draft"], "publish");
  assert.deepEqual(reversed.targets, ["draft", "draft-with-draft"]);

  const withGhost = planBatchStatus(page, ["ghost", "draft"], "publish");
  assert.deepEqual(withGhost.targets, ["draft"]);
  assert.equal(withGhost.count, 1);
});

test("空选择：按钮禁用且原因为「未选择文章」", () => {
  for (const action of ["publish", "draft"] as const) {
    const plan = planBatchStatus(page, [], action);
    assert.deepEqual(plan.targets, []);
    assert.equal(plan.count, 0);
    assert.equal(plan.disabledReason, "未选择文章");
  }
});

test("禁用原因覆盖已发布 / 已是草稿 / 回收站混合场景", () => {
  const allPublished = planBatchStatus(page, ["published", "published-with-draft"], "publish");
  assert.equal(allPublished.count, 0);
  assert.equal(allPublished.skipped, 2);
  assert.equal(allPublished.disabledReason, "所选文章均已发布");

  const allDraft = planBatchStatus(page, ["draft", "draft-with-draft"], "draft");
  assert.equal(allDraft.count, 0);
  assert.equal(allDraft.disabledReason, "所选文章均已是草稿");

  const mixed = planBatchStatus(page, ["published", "trashed"], "publish");
  assert.equal(mixed.count, 0);
  assert.equal(mixed.disabledReason, "所选文章均已发布，另有 1 篇在回收站需先恢复");
});

test("存在 has_draft 时发布确认说明「只切状态、草稿修改保留」", () => {
  const withDraft = planBatchStatus(page, ["draft-with-draft"], "publish");
  assert.equal(withDraft.hasDraft, 1);
  assert.equal(withDraft.dialogTitle, "将选中的 1 篇文章设为已发布？");
  assert.match(withDraft.dialogDescription, /这 1 篇文章将设为已发布/);
  assert.match(withDraft.dialogDescription, /其中 1 篇存在未发布的草稿修改/);
  assert.match(withDraft.dialogDescription, /本次只切换发布状态，草稿修改保留/);

  const withoutDraft = planBatchStatus(page, ["draft"], "publish");
  assert.equal(withoutDraft.hasDraft, 0);
  assert.doesNotMatch(withoutDraft.dialogDescription, /草稿修改保留/);
});

test("确认弹窗包含条数、前台影响与跳过 / 回收站说明", () => {
  const publish = planBatchStatus(page, pageSlugs, "publish");
  assert.equal(publish.dialogTitle, "将选中的 2 篇文章设为已发布？");
  assert.match(publish.dialogDescription, /这 2 篇文章将设为已发布。/);
  assert.match(publish.dialogDescription, /另有 2 篇已发布，将跳过处理。/);
  assert.match(publish.dialogDescription, /另有 1 篇在回收站，需先恢复后再操作。/);

  const draft = planBatchStatus(page, pageSlugs, "draft");
  assert.equal(draft.dialogTitle, "将选中的 2 篇文章设为草稿？");
  assert.match(draft.dialogDescription, /这 2 篇文章将设为草稿，前台将不再展示；已保存的内容仍会保留。/);
  assert.match(draft.dialogDescription, /另有 2 篇已是草稿，将跳过处理。/);
});

test("Toast 汇总 succeeded，failed>0 时追加未处理条数", () => {
  assert.deepEqual(batchStatusToast("publish", { succeeded: 2, skipped: 1, failed: 0 }), {
    tone: "success",
    message: "已发布 2 篇文章",
  });
  assert.deepEqual(batchStatusToast("draft", { succeeded: 3, skipped: 0, failed: 1 }), {
    tone: "success",
    message: "已设为草稿 3 篇，1 篇未处理",
  });
  // 全部失败（200 + failed）用错误态提示，全部跳过用信息态，避免误报成功。
  assert.deepEqual(batchStatusToast("publish", { succeeded: 0, skipped: 0, failed: 2 }), {
    tone: "error",
    message: "已发布 0 篇文章，2 篇未处理",
  });
  assert.deepEqual(batchStatusToast("draft", { succeeded: 0, skipped: 2, failed: 0 }), {
    tone: "info",
    message: "已设为草稿 0 篇",
  });
});

test("规划是纯函数：不修改输入文章与选中集合", () => {
  const posts: BatchStatusPost[] = [
    { slug: "a", status: "draft" },
    { slug: "b", status: "published", has_draft: true },
  ];
  const selection = new Set(["a", "b"]);
  const snapshot = JSON.stringify({ posts, selection: Array.from(selection) });

  const plan = planBatchStatus(posts, selection, "publish");
  plan.targets.push("mutated-by-caller");

  assert.equal(JSON.stringify({ posts, selection: Array.from(selection) }), snapshot);
  assert.deepEqual(plan.targets, ["a", "mutated-by-caller"]);
  // Action 类型用于校验调用侧字面量收窄。
  const action: BatchStatusAction = "draft";
  assert.equal(planBatchStatus(posts, selection, action).targets[0], "b");
});

test("两个状态按钮把禁用原因作为 title 交给 VButton，VButton 默认继承到原生 button", () => {
  const view = readFileSync(new URL("../web/src/modules/posts/PostList.vue", import.meta.url), "utf8");
  assert.match(view, /:disabled="batchPlans\.publish\.count === 0"/);
  assert.match(view, /:title="batchPlans\.publish\.disabledReason \?\? undefined"/);
  assert.match(view, /:disabled="batchPlans\.draft\.count === 0"/);
  assert.match(view, /:title="batchPlans\.draft\.disabledReason \?\? undefined"/);

  // VButton 是单根 <button> 且未关闭属性继承；title 会落到原生按钮上（已用 SSR 实测）。
  const button = readFileSync(new URL("../../../packages/components/src/ui/VButton.vue", import.meta.url), "utf8");
  assert.doesNotMatch(button, /inheritAttrs\s*:\s*false/);
  assert.match(button, /<template>\s*<button\b/);
});
