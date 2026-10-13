import type { NotebookRequest, NotebookNoteRequest } from "@blog/api-client";

export interface NotebookForm {
  id: string;
  config: string;
  blog: string;
}
export interface NoteForm {
  slug: string;
  content: string;
  tags: string;
  config: string;
  blog: string;
}

export function newNotebookForm(): NotebookForm {
  return { id: "", config: '{\n  "name": "新笔记本",\n  "description": ""\n}', blog: "{}" };
}
export function newNoteForm(notebook: string): NoteForm {
  return {
    slug: "",
    content: "",
    tags: "",
    blog: "{}",
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
export function validateNotebookID(id: string): string {
  const result = id.trim();
  if (!result || result === "." || result === ".." || /[<>:"/\\|?*\x00-\x1f]/.test(result))
    throw new Error("笔记本 ID 必须是不含路径分隔符的有效名称");
  return result;
}
export function validateNoteSlug(slug: string): string {
  const result = slug.trim();
  if (
    !result ||
    /[<>:"\\|?*\x00-\x1f]/.test(result) ||
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
export function notebookRoute(notebook: string, suffix = "notes"): string {
  return `/notebooks/${encodeURIComponent(notebook)}/${suffix}`;
}
