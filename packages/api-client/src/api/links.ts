import { http } from "../transport";
import type { LinkPayload } from "./contracts";

// --- Friends / Links ---
export const linkApi = {
  list: () => http.get("/links"),
  create: (data: LinkPayload) => http.post("/links", data),
  update: (id: number, data: LinkPayload) => http.put(`/links/${id}`, data),
  delete: (id: number) => http.delete(`/links/${id}`),
  /** 并发检测各友链可达性，返回 {items: [{id, ok, status}], tagged, restored}（status 为 HTTP 状态码，0=无法访问；tagged/restored 为自动打/摘「失效」标签条数） */
  check: () => http.post("/links/check"),
  snapshot: (id: number) => http.post(`/links/${id}/snapshot`),
  /** 导入友链（Spec 54/56）：source=github 拉 Friends 仓库 issues，source=text 解析粘贴的 JSON/YAML；返回 {imported, skipped, failed[]} */
  importLinks: (data: { source: "github" | "text"; group?: string; content?: string }) =>
    http.post("/links/import", data),
};

// --- 友链 RSS 订阅阅读器 ---
export const subscriptionApi = {
  sources: () => http.get("/subscriptions"),
  articles: (params: { feed?: string; filter?: string; page?: number; size?: number }) =>
    http.get("/subscriptions/articles", { params }),
  markRead: (id: number, read: boolean) => http.post("/subscriptions/read", { id, read }),
  markAllRead: (feed?: string) => http.post("/subscriptions/read-all", null, { params: feed ? { feed } : {} }),
  toggleFlag: (id: number, flag: "favorite" | "later", value: boolean) =>
    http.post("/subscriptions/flag", { id, flag, value }),
  refresh: () => http.post("/subscriptions/refresh"),
};

// --- Taxonomy (categories / tags) ---
export const taxonomyApi = {
  categories: () => http.get("/categories"),
  createCategory: (name: string, parent_id = 0) => http.post("/categories", { name, parent_id }),
  updateCategory: (id: number, data: { name: string; parent_id?: number }) => http.put(`/categories/${id}`, data),
  deleteCategoryById: (id: number) => http.delete(`/categories/${id}`),
  renameCategory: (name: string, newName: string) =>
    http.put(`/categories/${encodeURIComponent(name)}`, { name: newName }),
  deleteCategory: (name: string) => http.delete(`/categories/${encodeURIComponent(name)}`),
  tags: () => http.get("/tags"),
  renameTag: (name: string, newName: string) => http.put(`/tags/${encodeURIComponent(name)}`, { name: newName }),
  deleteTag: (name: string) => http.delete(`/tags/${encodeURIComponent(name)}`),
  mergeTags: (sources: string[], target: string) => http.post("/tags/merge", { sources, target }),
};
