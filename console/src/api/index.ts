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
};

// --- Dashboard ---
export const dashboardApi = {
  stats: () => http.get("/dashboard/stats"),
  recentComments: () => http.get("/dashboard/recent-comments"),
};

// --- Posts ---
export const postApi = {
  list: (params?: Record<string, unknown>) => http.get("/posts", { params }),
  get: (slug: string) => http.get(`/posts/${slug}`),
  create: (data: Record<string, unknown>) => http.post("/posts", data),
  update: (slug: string, data: Record<string, unknown>) =>
    http.put(`/posts/${slug}`, data),
  delete: (slug: string) => http.delete(`/posts/${slug}`),
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
  delete: (id: number) => http.delete(`/attachments/${id}`),
};

// --- Pages ---
export const pageApi = {
  list: () => http.get("/pages"),
  get: (slug: string) => http.get(`/pages/${slug}`),
  create: (data: Record<string, unknown>) => http.post("/pages", data),
  update: (slug: string, data: Record<string, unknown>) =>
    http.put(`/pages/${slug}`, data),
  delete: (slug: string) => http.delete(`/pages/${slug}`),
};

// --- Moments ---
export const momentApi = {
  list: (params?: Record<string, unknown>) =>
    http.get("/moments", { params }),
  create: (data: Record<string, unknown>) => http.post("/moments", data),
  delete: (id: number) => http.delete(`/moments/${id}`),
};

// --- Settings ---
export const settingsApi = {
  all: () => http.get("/settings"),
  updateSection: (section: string, data: unknown) =>
    http.put(`/settings/${section}`, data),
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
