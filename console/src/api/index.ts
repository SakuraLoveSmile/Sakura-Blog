import axios from "axios";
import router from "@/router";

const http = axios.create({
  baseURL: "/api/v1",
  timeout: 30000,
});

// 响应拦截器：401 时跳转登录页
http.interceptors.response.use(
  (res) => res,
  (err) => {
    if (err.response?.status === 401) {
      router.push("/login");
    }
    return Promise.reject(err);
  }
);

export default http;

// --- Auth ---
export const authApi = {
  login: (password: string) => http.post("/auth/login", { password }),
  logout: () => http.post("/auth/logout"),
  check: () => http.get("/auth/check"),
  changePassword: (current: string, newPassword: string) =>
    http.put("/auth/password", { current, new: newPassword }),
};

// --- Dashboard ---
export interface DashboardLayoutItem {
  type: string;
}

export const dashboardApi = {
  stats: () => http.get("/dashboard/stats"),
  recentComments: () => http.get("/dashboard/recent-comments"),
  quickActions: () => http.get("/dashboard/quick-actions"),
  saveQuickActions: (items: string[]) =>
    http.put("/dashboard/quick-actions", { items }),
};

// --- Stats (Spec 24) ---
export const statsApi = {
  trend: (days: number = 7) => http.get("/stats/trend", { params: { days } }),
  popular: (days: number = 7, limit: number = 10) =>
    http.get("/stats/popular", { params: { days, limit } }),
  referrers: (days: number = 7) => http.get("/stats/referrers", { params: { days } }),
};

// --- Posts ---
export const postApi = {
  list: (params?: Record<string, unknown>) => http.get("/posts", { params }),
  get: (slug: string) => http.get(`/posts/${slug}`),
  create: (data: Record<string, unknown>) => http.post("/posts", data),
  update: (slug: string, data: Record<string, unknown>) =>
    http.put(`/posts/${slug}`, data),
  delete: (slug: string) => http.delete(`/posts/${slug}`),
  restore: (slug: string) => http.post(`/posts/${slug}/restore`),
  purge: (slug: string) => http.delete(`/posts/${slug}/purge`),
  // 草稿头（已保存未发布，列表小绿点）
  saveDraft: (slug: string, data: Record<string, unknown>) =>
    http.put(`/posts/${slug}/draft`, data),
  publish: (slug: string, data: Record<string, unknown>) =>
    http.post(`/posts/${slug}/publish`, data),
  discardDraft: (slug: string) => http.delete(`/posts/${slug}/draft`),
};

// --- Post Revisions (Spec 22) ---
export const revisionApi = {
  list: (slug: string) => http.get(`/posts/${slug}/revisions`),
  get: (id: number) => http.get(`/revisions/${id}`),
  restore: (id: number) => http.post(`/revisions/${id}/restore`),
};

// --- Comments ---
export const commentApi = {
  list: (params?: Record<string, unknown>) =>
    http.get("/comments", { params }),
  approve: (id: number) => http.put(`/comments/${id}/approve`),
  delete: (id: number) => http.delete(`/comments/${id}`),
  reply: (data: { parent_id: number; content: string }) =>
    http.post("/comments/reply", data),
};

// --- Attachments ---
export const attachmentApi = {
  list: (params?: Record<string, unknown>) =>
    http.get("/attachments", { params }),
  upload: (file: File) => {
    const form = new FormData();
    form.append("file", file);
    return http.post("/attachments/upload", form, {
      headers: { "Content-Type": "multipart/form-data" },
    });
  },
  refs: (id: number) => http.get(`/attachments/${id}/refs`),
  delete: (id: number) => http.delete(`/attachments/${id}`),
};

// --- Markdown 预览 ---
export const previewApi = {
  render: (markdown: string) => http.post("/preview/markdown", { markdown }),
};

// --- 备份与导出 ---
// 下载接口走 cookie 认证，直接 <a href> 触发浏览器下载，不走 axios。
export const backupExportUrl = "/api/v1/backup/export";
export const backupExportMarkdownUrl = "/api/v1/backup/export/markdown";
export const backupApi = {
  status: () => http.get("/backup/status"),
  importMarkdown: (file: File) => {
    const form = new FormData();
    form.append("file", file);
    return http.post("/backup/import/markdown", form, {
      headers: { "Content-Type": "multipart/form-data" },
    });
  },
};

// --- Pages ---
export const pageApi = {
  list: (params?: Record<string, unknown>) => http.get("/pages", { params }),
  get: (slug: string) => http.get(`/pages/${slug}`),
  create: (data: Record<string, unknown>) => http.post("/pages", data),
  update: (slug: string, data: Record<string, unknown>) =>
    http.put(`/pages/${slug}`, data),
  delete: (slug: string) => http.delete(`/pages/${slug}`),
  restore: (slug: string) => http.post(`/pages/${slug}/restore`),
  purge: (slug: string) => http.delete(`/pages/${slug}/purge`),
};

// --- Moments ---
export const momentApi = {
  list: (params?: Record<string, unknown>) =>
    http.get("/moments", { params }),
  create: (data: Record<string, unknown>) => http.post("/moments", data),
  delete: (id: number) => http.delete(`/moments/${id}`),
};

// --- 笔记页（content/notes.md 原文读写，前台 /notes 上半部分） ---
export const notesApi = {
  get: () => http.get("/notes-content"),
  update: (content: string) => http.put("/notes-content", { content }),
};

// --- Settings ---
export const settingsApi = {
  all: () => http.get("/settings"),
  updateSection: (section: string, data: unknown) =>
    http.put(`/settings/${section}`, data),
  testNotify: () => http.post("/settings/notify/test"),
};

// --- Friends / Links ---
export const linkApi = {
  list: () => http.get("/links"),
  create: (data: Record<string, unknown>) => http.post("/links", data),
  update: (id: number, data: Record<string, unknown>) =>
    http.put(`/links/${id}`, data),
  delete: (id: number) => http.delete(`/links/${id}`),
};

// --- Taxonomy (categories / tags) ---
export const taxonomyApi = {
  categories: () => http.get("/categories"),
  renameCategory: (name: string, newName: string) =>
    http.put(`/categories/${encodeURIComponent(name)}`, { name: newName }),
  deleteCategory: (name: string) =>
    http.delete(`/categories/${encodeURIComponent(name)}`),
  tags: () => http.get("/tags"),
  renameTag: (name: string, newName: string) =>
    http.put(`/tags/${encodeURIComponent(name)}`, { name: newName }),
  deleteTag: (name: string) => http.delete(`/tags/${encodeURIComponent(name)}`),
  mergeTags: (sources: string[], target: string) =>
    http.post("/tags/merge", { sources, target }),
};

// --- Shares (文章分享链接) ---
export const shareApi = {
  list: (params?: Record<string, unknown>) => http.get("/shares", { params }),
  listByPost: (slug: string) => http.get(`/posts/${slug}/shares`),
  create: (slug: string, data: { share_type: string; expires_at: string }) =>
    http.post(`/posts/${slug}/shares`, data),
  delete: (id: number) => http.delete(`/shares/${id}`),
};
