import type { NotebookRequest, NotebookNoteRequest, NotebookResponse } from "@blog/api-client";

export interface NotebookForm {
  id: string;
  config: string;
  blog: string;
  // 名称等基本字段与 config 里的对应项双向同步（useStellarForm），本身不单独提交。
  name: string;
  description: string;
  icon: string;
  cover: string;
  sort: number;
}
export interface NoteForm {
  slug: string;
  content: string;
  tags: string;
  config: string;
  blog: string;
  // 标题与 config 里的 title 双向同步（useStellarForm），本身不单独提交。
  title: string;
}

export function newNotebookForm(): NotebookForm {
  return {
    id: "",
    config: '{\n  "name": "新笔记本",\n  "description": ""\n}',
    blog: "{}",
    name: "",
    description: "",
    icon: "",
    cover: "",
    sort: 0,
  };
}
type NotebookFormSource = Pick<
  NotebookResponse,
  "id" | "stellar_config" | "blog_extensions" | "name" | "description" | "icon" | "cover" | "sort"
>;
export function notebookFormFrom(data: NotebookFormSource): NotebookForm {
  return {
    id: data.id,
    config: data.stellar_config,
    blog: data.blog_extensions,
    name: data.name,
    description: data.description,
    icon: data.icon,
    cover: data.cover,
    sort: data.sort,
  };
}
export function newNoteForm(notebook: string): NoteForm {
  return {
    slug: "",
    content: "",
    tags: "",
    blog: "{}",
    title: "",
    config: JSON.stringify({ title: "新笔记", collection: { profile: "notebook", id: notebook } }, null, 2),
  };
}

// 仅检查输入形状；原文字符串直接提交，规范化和完整上游校验由 Go 完成。
export function validateJSONText(raw: string, label: string): void {
  let value: unknown;
  try {
    value = JSON.parse(raw);
  } catch {
    throw new Error(`${label}必须是合法 JSON`);
  }
  if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error(`${label}必须是 JSON 对象`);
}
// 控制字符 U+0000–U+001F 逐字符判断，避免在正则字面量里书写它们（no-control-regex）。
function hasControlCharacter(value: string): boolean {
  return value.split("").some((char) => char.charCodeAt(0) < 0x20);
}
export function validateNotebookID(id: string): string {
  const result = id.trim();
  if (!result || result === "." || result === ".." || /[<>:"/\\|?*]/.test(result) || hasControlCharacter(result))
    throw new Error("笔记本 ID 必须是不含路径分隔符的有效名称");
  return result;
}
export function validateNoteSlug(slug: string): string {
  const result = slug.trim();
  if (
    !result ||
    /[<>:"\\|?*]/.test(result) ||
    hasControlCharacter(result) ||
    result.split("/").some((part) => !part || part === "." || part === "..")
  )
    throw new Error("Slug 必须是有效相对路径，不能包含空段、. 或 ..");
  return result;
}
export function notebookPayload(form: NotebookForm): NotebookRequest {
  const id = validateNotebookID(form.id);
  validateJSONText(form.config, "集合配置");
  validateJSONText(form.blog, "Blog 扩展");
  return { id, stellar_version: 2, stellar_config: form.config, blog_extensions: form.blog };
}
export function notePayload(notebook: string, form: NoteForm): NotebookNoteRequest {
  const slug = validateNoteSlug(form.slug);
  validateJSONText(form.config, "Front matter");
  validateJSONText(form.blog, "Blog 扩展");
  return {
    notebook_id: notebook,
    slug,
    content_md: form.content,
    tags: form.tags
      .split(/\r?\n/)
      .map((tag) => tag.trim())
      .filter(Boolean),
    stellar_version: 2,
    stellar_config: form.config,
    blog_extensions: form.blog,
  };
}
export function isMigrationError(error: unknown): boolean {
  const data = (error as { response?: { status?: number; data?: { error?: string; detail?: string; type?: string } } })
    ?.response;
  return (
    data?.status === 409 &&
    (data.data?.type === "stellar_migration_required" ||
      /migration|迁移/i.test(data.data?.error ?? data.data?.detail ?? ""))
  );
}
// 写入失败的简短 Toast：迁移说明已经在页面顶部完整展示，这里不再重复整段后端原文。
export function failureToast(message: string, migration: boolean): string {
  return migration ? "需要先迁移到 Stellar 2.0，详见页面顶部的说明" : message;
}
export function notebookRoute(notebook: string, suffix = "notes"): string {
  return `/notebooks/${encodeURIComponent(notebook)}/${suffix}`;
}
export function noteStatusLabel(status: string): string {
  return status === "published" ? "已发布" : "草稿";
}
export function notebookCountsLabel(item: { note_count: number; published_count: number }): string {
  if (!item.note_count) return "还没有笔记";
  return `${item.note_count} 篇笔记 · ${item.published_count} 篇已发布`;
}
// 列表里的更新时间：本地时区 YYYY-MM-DD HH:mm；无法解析时返回空串。
export function formatNoteTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}
