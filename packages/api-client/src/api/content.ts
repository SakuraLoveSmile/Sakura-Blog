import { http } from "../transport";

// --- Posts ---
// 请求/响应为弱类型透传；Spec 42 新增字段 meta_description / meta_keywords（文章级 SEO）。
export const postApi = {
  list: (params?: Record<string, unknown>) => http.get("/posts", { params }),
  get: (slug: string) => http.get(`/posts/${slug}`),
  create: (data: Record<string, unknown>) => http.post("/posts", data),
  update: (slug: string, data: Record<string, unknown>) => http.put(`/posts/${slug}`, data),
  delete: (slug: string) => http.delete(`/posts/${slug}`),
  restore: (slug: string) => http.post(`/posts/${slug}/restore`),
  purge: (slug: string) => http.delete(`/posts/${slug}/purge`),
  // AI 文章摘要（Spec 53）：保存/清空预生成摘要（独立于主保存链路）
  updateAISummary: (slug: string, summary: string) => http.put(`/posts/${slug}/ai-summary`, { summary }),
  // 草稿头（已保存未发布，列表小绿点）
  saveDraft: (slug: string, data: Record<string, unknown>) => http.put(`/posts/${slug}/draft`, data),
  publish: (slug: string, data: Record<string, unknown>) => http.post(`/posts/${slug}/publish`, data),
  discardDraft: (slug: string) => http.delete(`/posts/${slug}/draft`),
};

// --- Topics ---
export interface TopicItem {
  id: number;
  name: string;
  slug: string;
  description: string;
  icon: string;
  cover: string;
  sort: number;
  post_count: number;
  created_at: string;
  updated_at: string;
}

export interface TopicPayload {
  name: string;
  slug?: string;
  description?: string;
  icon?: string;
  cover?: string;
  sort?: number;
}

export const topicApi = {
  list: () => http.get<{ items: TopicItem[]; total: number }>("/topics"),
  create: (data: TopicPayload) => http.post("/topics", data),
  update: (id: number, data: Partial<TopicPayload>) => http.put(`/topics/${id}`, data),
  delete: (id: number) => http.delete(`/topics/${id}`),
};

// --- Post Revisions (Spec 22) ---
export const revisionApi = {
  list: (slug: string) => http.get(`/posts/${slug}/revisions`),
  get: (id: number) => http.get(`/revisions/${id}`),
  restore: (id: number) => http.post(`/revisions/${id}/restore`),
};

// --- Comments ---
export type CommentStatus = "pending" | "approved" | "rejected" | "spam";
export const commentApi = {
  list: (params?: Record<string, unknown>) => http.get("/comments", { params }),
  approve: (id: number) => http.put(`/comments/${id}/approve`),
  setStatus: (id: number, status: CommentStatus) => http.put(`/comments/${id}/status`, { status }),
  delete: (id: number) => http.delete(`/comments/${id}`),
  reply: (data: { parent_id: number; content: string }) => http.post("/comments/reply", data),
  batch: (data: { ids: number[]; action: "approve" | "reject" | "spam" | "delete" }) =>
    http.post("/comments/batch", data),
};

// --- Pages ---
export const pageApi = {
  list: (params?: Record<string, unknown>) => http.get("/pages", { params }),
  get: (slug: string) => http.get(`/pages/${slug}`),
  create: (data: Record<string, unknown>) => http.post("/pages", data),
  update: (slug: string, data: Record<string, unknown>) => http.put(`/pages/${slug}`, data),
  delete: (slug: string) => http.delete(`/pages/${slug}`),
  restore: (slug: string) => http.post(`/pages/${slug}/restore`),
  purge: (slug: string) => http.delete(`/pages/${slug}/purge`),
};

// --- Moments ---
export const momentApi = {
  list: (params?: Record<string, unknown>) => http.get("/moments", { params }),
  create: (data: Record<string, unknown>) => http.post("/moments", data),
  update: (id: number, data: Record<string, unknown>) => http.put(`/moments/${id}`, data),
  delete: (id: number) => http.delete(`/moments/${id}`),
};

// --- 管理端全局搜索（Spec 39，⌘K 弹窗）---
export interface SearchResultItem {
  id: number;
  type: "post" | "page";
  slug: string;
  title: string;
  status: "published" | "draft" | "trashed";
  snippet: string;
}

export const searchApi = {
  query: (q: string, limit: number = 20) =>
    http.get<{ items: SearchResultItem[] }>("/search", { params: { q, limit } }),
};

// --- Shares (文章分享链接) ---
export const shareApi = {
  list: (params?: Record<string, unknown>) => http.get("/shares", { params }),
  listByPost: (slug: string) => http.get(`/posts/${slug}/shares`),
  create: (slug: string, data: { share_type: string; expires_at: string }) => http.post(`/posts/${slug}/shares`, data),
  delete: (id: number) => http.delete(`/shares/${id}`),
};
