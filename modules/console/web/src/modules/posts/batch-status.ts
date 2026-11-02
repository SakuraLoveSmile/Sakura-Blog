// 文章批量发布 / 批量草稿的纯规划逻辑：不依赖 Vue，供 PostList.vue 消费，并由 node:test 直接覆盖。
// 只计算目标集合、计数、按钮禁用原因与确认文案；不发请求、不改状态，回收站文章不参与目标集合。

export type BatchStatusAction = "publish" | "draft";

/** 批量状态操作对应的目标状态。 */
export type BatchStatusTargetStatus = "published" | "draft";

/** 规划输入：当前页文章的最小字段（列表接口已返回 has_draft）。 */
export interface BatchStatusPost {
  slug: string;
  status: string;
  has_draft?: boolean;
}

/** POST /api/v1/posts/batch 的响应计数。 */
export interface BatchStatusResult {
  succeeded: number;
  skipped: number;
  failed: number;
}

export interface BatchStatusPlan {
  action: BatchStatusAction;
  targetStatus: BatchStatusTargetStatus;
  /** 实际提交的 slug，保持当前页文章顺序。 */
  targets: string[];
  /** targets.length。 */
  count: number;
  /** 已是目标状态、本次跳过的条数（后端 skipped）。 */
  skipped: number;
  /** 在回收站、需先恢复的条数。 */
  trashed: number;
  /** 目标集合中带未发布草稿修改的条数，用于发布确认文案。 */
  hasDraft: number;
  /** 无法执行时的按钮 title 原因；可执行时为 null。 */
  disabledReason: string | null;
  dialogTitle: string;
  dialogDescription: string;
}

export interface BatchStatusToast {
  tone: "success" | "error" | "info";
  message: string;
}

// 按钮禁用原因（与 docs/plans/post-batch-status.md、baseline README 的文案对齐）。
const REASON_NONE = "未选择文章";
const REASON_TRASHED = "回收站文章需先恢复再设置状态";
const REASON_ALREADY: Record<BatchStatusAction, string> = {
  publish: "所选文章均已发布",
  draft: "所选文章均已是草稿",
};

function targetStatusOf(action: BatchStatusAction): BatchStatusTargetStatus {
  return action === "publish" ? "published" : "draft";
}

/** 无目标时给出禁用原因：区分「回收站需先恢复」与「已是目标状态」。 */
function emptyTargetReason(action: BatchStatusAction, skipped: number, trashed: number): string {
  const already = REASON_ALREADY[action];
  // 非回收站选择都已是目标状态时，跳过数与回收站数可能同时存在。
  if (skipped > 0) return trashed > 0 ? `${already}，另有 ${trashed} 篇在回收站需先恢复` : already;
  if (trashed > 0) return REASON_TRASHED;
  // 选中 slug 均不在当前页：视图的选择清理会移除失效 slug，这里只是兜底。
  return REASON_NONE;
}

/** 把当前页文章与选中 slug 归纳为某目标状态的目标集合、计数、禁用原因与确认文案。 */
export function planBatchStatus(
  posts: readonly BatchStatusPost[],
  selectedSlugs: Iterable<string>,
  action: BatchStatusAction,
): BatchStatusPlan {
  const selected = new Set(selectedSlugs);
  const targetStatus = targetStatusOf(action);
  const targets: string[] = [];
  let skipped = 0;
  let trashed = 0;
  let hasDraft = 0;

  for (const post of posts) {
    if (!selected.has(post.slug)) continue;
    if (post.status === "trashed") {
      trashed += 1;
      continue;
    }
    if (post.status === targetStatus) {
      skipped += 1;
      continue;
    }
    targets.push(post.slug);
    if (post.has_draft) hasDraft += 1;
  }

  const count = targets.length;
  let disabledReason: string | null = null;
  if (selected.size === 0) {
    disabledReason = REASON_NONE;
  } else if (count === 0) {
    disabledReason = emptyTargetReason(action, skipped, trashed);
  }

  const parts: string[] = [];
  if (action === "publish") {
    // 只陈述状态变更：保留 publish_at 的文章即使已发布，前台仍按定时时间生效。
    parts.push(`这 ${count} 篇文章将设为已发布。`);
    if (hasDraft > 0) {
      // 明确「只切状态」：不应用、不清空草稿头。
      parts.push(`其中 ${hasDraft} 篇存在未发布的草稿修改，本次只切换发布状态，草稿修改保留。`);
    }
  } else {
    parts.push(`这 ${count} 篇文章将设为草稿，前台将不再展示；已保存的内容仍会保留。`);
  }
  if (skipped > 0) {
    parts.push(`另有 ${skipped} 篇${action === "publish" ? "已发布" : "已是草稿"}，将跳过处理。`);
  }
  if (trashed > 0) {
    parts.push(`另有 ${trashed} 篇在回收站，需先恢复后再操作。`);
  }

  return {
    action,
    targetStatus,
    targets,
    count,
    skipped,
    trashed,
    hasDraft,
    disabledReason,
    dialogTitle: `将选中的 ${count} 篇文章设为${action === "publish" ? "已发布" : "草稿"}？`,
    dialogDescription: parts.join(""),
  };
}

/** 批量状态响应 → Toast 文案与语义；succeeded 才是实际变更条数。 */
export function batchStatusToast(action: BatchStatusAction, result: BatchStatusResult): BatchStatusToast {
  const failedSuffix = result.failed > 0 ? `，${result.failed} 篇未处理` : "";
  const message =
    action === "publish"
      ? `已发布 ${result.succeeded} 篇文章${failedSuffix}`
      : `已设为草稿 ${result.succeeded} 篇${failedSuffix}`;
  if (result.succeeded > 0) return { tone: "success", message };
  return result.failed > 0 ? { tone: "error", message } : { tone: "info", message };
}
