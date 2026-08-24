import { http } from "../transport";

// --- Wiki 项目与文档（Spec 35a） ---
export interface WikiHeroPreview {
  type?: string;
  commands?: { codes?: string[] }[];
  image?: string;
}

export interface WikiHeroAction {
  title?: string;
  url?: string;
  icon?: string;
}

export interface WikiHero {
  background?: string;
  title?: string;
  description?: string;
  start?: string;
  preview?: WikiHeroPreview;
  actions?: WikiHeroAction[];
}

export interface WikiProject {
  id: string;
  name: string;
  title: string;
  headline?: string;
  subtitle?: string;
  description?: string;
  icon?: string;
  cover?: string;
  available?: string;
  url?: string;
  repo?: string;
  branch?: string;
  sort: number;
  pin: boolean;
  visible: boolean;
  hero?: WikiHero;
  tags: string[];
  doc_count?: number;
  published_doc_count?: number;
  has_docs?: boolean;
}

// 文档字段：slug / title / order / parent(父文档 ID, 0=顶级) / content_md / status(published|draft)。
export const wikiApi = {
  listProjects: () => http.get<{ items: WikiProject[] }>("/wiki/projects"),
  getProject: (id: string) => http.get<WikiProject>(`/wiki/projects/${encodeURIComponent(id)}`),
  createProject: (data: Partial<WikiProject> & { id: string }) => http.post<WikiProject>("/wiki/projects", data),
  updateProject: (id: string, data: Partial<WikiProject>) =>
    http.put<WikiProject>(`/wiki/projects/${encodeURIComponent(id)}`, data),
  deleteProject: (id: string) => http.delete(`/wiki/projects/${encodeURIComponent(id)}`),
  listDocs: (project: string) => http.get(`/wiki/${encodeURIComponent(project)}/docs`),
  getDoc: (project: string, slug: string) =>
    http.get(`/wiki/${encodeURIComponent(project)}/docs/${encodeURIComponent(slug)}`),
  createDoc: (project: string, data: Record<string, unknown>) =>
    http.post(`/wiki/${encodeURIComponent(project)}/docs`, data),
  updateDoc: (project: string, slug: string, data: Record<string, unknown>) =>
    http.put(`/wiki/${encodeURIComponent(project)}/docs/${encodeURIComponent(slug)}`, data),
  deleteDoc: (project: string, slug: string) =>
    http.delete(`/wiki/${encodeURIComponent(project)}/docs/${encodeURIComponent(slug)}`),
};

// --- Notebooks（Spec 35b 笔记本/笔记管理） ---
// 后端响应格式：列表 {items,total}；笔记列表 {notes,total,page,per_page}；note-tags {tags}。
// Notebook: id/name(唯一, URL slug)/description/cover/order/created_at；Note: id/notebook_id/title/content/content_html/tags(逗号分隔)/created_at/updated_at。
export const notebookApi = {
  listNotebooks: () => http.get("/notebooks"),
  getNotebook: (id: number) => http.get(`/notebooks/${id}`),
  createNotebook: (data: { name: string; description?: string; cover?: string; order?: number }) =>
    http.post("/notebooks", data),
  updateNotebook: (id: number, data: { name?: string; description?: string; cover?: string; order?: number }) =>
    http.put(`/notebooks/${id}`, data),
  deleteNotebook: (id: number) => http.delete(`/notebooks/${id}`),
  listNotes: (notebookId: number, params?: Record<string, unknown>) =>
    http.get(`/notebooks/${notebookId}/notes`, { params }),
  createNote: (notebookId: number, data: { title: string; content?: string; tags?: string }) =>
    http.post(`/notebooks/${notebookId}/notes`, data),
  getNote: (id: number) => http.get(`/notes/${id}`),
  updateNote: (id: number, data: Record<string, unknown>) => http.put(`/notes/${id}`, data),
  deleteNote: (id: number) => http.delete(`/notes/${id}`),
  getTags: () => http.get("/note-tags"),
};
