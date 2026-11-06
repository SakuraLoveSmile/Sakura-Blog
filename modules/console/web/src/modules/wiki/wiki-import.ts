// 文库「导入文章」的纯规划逻辑：不依赖 Vue，供 WikiImportModal.vue 消费，并由 node:test 直接覆盖。
// 只计算单篇可选状态、选择集、提交载荷与结果摘要；不发请求、不改数据。

/** 文章列表里的最小字段（postApi.list 的返回子集）。 */
export interface ImportablePost {
  slug: string;
  title?: string;
  status: string;
}

/** 单篇文章在导入弹窗里的可选状态。 */
export type PostImportState = "importable" | "imported" | "trashed";

/** 目标项目已有同名 Slug（已导入过）或文章在回收站时不可再选。 */
export function postImportState(post: ImportablePost, existingSlugs: Iterable<string>): PostImportState {
  if (post.status === "trashed") return "trashed";
  const existing = existingSlugs instanceof Set ? existingSlugs : new Set(existingSlugs);
  return existing.has(post.slug) ? "imported" : "importable";
}

/** 返回新的选择集（不修改入参）；勾选/取消勾选都走这里，保证跨页多选可预测。 */
export function toggleSelection(selected: Iterable<string>, slug: string, checked: boolean): Set<string> {
  const next = new Set(selected);
  if (checked) next.add(slug);
  else next.delete(slug);
  return next;
}

/** 当前页可一次全选的 slug（跳过已导入与回收站）。 */
export function selectableSlugs(posts: ImportablePost[], existingSlugs: Iterable<string>): string[] {
  return posts.filter((post) => postImportState(post, existingSlugs) === "importable").map((post) => post.slug);
}

/** 提交给后端的 slug 列表：去空白、去空项、按首次出现顺序去重。 */
export function normalizeSlugs(input: Iterable<string>): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const raw of input) {
    const slug = raw.trim();
    if (!slug || seen.has(slug)) continue;
    seen.add(slug);
    out.push(slug);
  }
  return out;
}

/** POST /api/v1/wiki/:project/docs/import 的载荷（弹窗只允许 draft/published）。 */
export function buildImportPayload(
  selected: Iterable<string>,
  parent: number,
  status: "draft" | "published",
): { slugs: string[]; parent: number; status: "draft" | "published" } {
  return { slugs: normalizeSlugs(selected), parent: parent > 0 ? parent : 0, status };
}

interface ImportBuckets {
  imported: readonly unknown[];
  skipped: readonly unknown[];
  failed: readonly unknown[];
}

/** 结果面板首行摘要，例如「已导入 2 篇；跳过 1 篇；失败 1 篇」。 */
export function importSummary(result: ImportBuckets): string {
  const parts: string[] = [];
  if (result.imported.length) parts.push(`已导入 ${result.imported.length} 篇`);
  if (result.skipped.length) parts.push(`跳过 ${result.skipped.length} 篇`);
  if (result.failed.length) parts.push(`失败 ${result.failed.length} 篇`);
  return parts.length ? parts.join("；") : "没有导入任何文档";
}

/** 跳过/失败条目的展示文本：优先标题，附后端给出的原因。 */
export function importIssueText(item: { slug: string; title?: string; reason?: string }): string {
  const name = (item.title || "").trim() || item.slug;
  return item.reason ? `${name}：${item.reason}` : name;
}
