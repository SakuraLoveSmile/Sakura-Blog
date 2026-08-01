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
// Spec 45：username 用户表认证 + TOTP 2FA；username 为空时后端回退旧版 admin 密码认证。
export const authApi = {
  login: (username: string, password: string) =>
    http.post("/auth/login", { username, password }),
  // 两步登录第二步：验证 TOTP code 后下发签名 cookie
  verify2FA: (token: string, code: string) =>
    http.post("/auth/2fa", { token, code }),
  logout: () => http.post("/auth/logout"),
  check: () => http.get("/auth/check"),
  changePassword: (current: string, newPassword: string) =>
    http.put("/auth/password", { current, new: newPassword }),
};

// --- Profile（Spec 45 用户资料） ---
export interface UserProfile {
  id?: number;
  username: string;
  display_name: string;
  email: string;
  avatar: string;
  has_2fa: boolean;
}

export const profileApi = {
  get: () => http.get<UserProfile>("/profile"),
  update: (data: { display_name: string; email: string; avatar: string }) =>
    http.put("/profile", data),
  // --- 2FA 管理（Spec 45：TOTP 开启/关闭/恢复码） ---
  setup2FA() {
    return http.post("/profile/2fa/setup");
  },
  confirm2FA(data: { secret: string; code: string; recovery_codes: string[] }) {
    return http.post("/profile/2fa/confirm", data);
  },
  disable2FA(code: string) {
    return http.post("/profile/2fa/disable", { code });
  },
  getRecoveryCodes() {
    return http.get("/profile/2fa/recovery-codes");
  },
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
// 请求/响应为弱类型透传；Spec 42 新增字段 meta_description / meta_keywords（文章级 SEO）。
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
  batch: (data: { ids: number[]; action: "approve" | "delete" }) =>
    http.post("/comments/batch", data),
};

// --- Attachments ---
export const attachmentApi = {
  list: (params?: Record<string, unknown>) =>
    http.get("/attachments", { params }),
  upload: (file: File, groupId?: number) => {
    const form = new FormData();
    form.append("file", file);
    if (groupId != null && groupId > 0) form.append("group_id", String(groupId));
    return http.post("/attachments/upload", form, {
      headers: { "Content-Type": "multipart/form-data" },
    });
  },
  refs: (id: number) => http.get(`/attachments/${id}/refs`),
  delete: (id: number) => http.delete(`/attachments/${id}`),
  move: (ids: number[], groupId: number) =>
    http.post("/attachments/move", { ids, group_id: groupId }),
};

// --- Attachment Groups（Spec 44） ---
export const attachmentGroupApi = {
  list: () => http.get("/attachment-groups"),
  create: (data: { name: string; order?: number }) =>
    http.post("/attachment-groups", data),
  update: (id: number, data: { name: string; order?: number }) =>
    http.put(`/attachment-groups/${id}`, data),
  delete: (id: number) => http.delete(`/attachment-groups/${id}`),
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
  // 整站恢复（Spec 41）：上传全量备份 zip / 从服务器留存备份还原
  restore: (file: File, strict: boolean) => {
    const form = new FormData();
    form.append("file", file);
    form.append("strict", strict ? "true" : "false");
    return http.post("/backup/restore", form, {
      headers: { "Content-Type": "multipart/form-data" },
    });
  },
  restoreSnapshot: (name: string) => http.post("/backup/restore-snapshot", { name }),
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
  update: (id: number, data: Record<string, unknown>) => http.put(`/moments/${id}`, data),
  delete: (id: number) => http.delete(`/moments/${id}`),
};

// --- 笔记页（content/notes.md 原文读写，前台 /notes 上半部分） ---
export const notesApi = {
  get: () => http.get("/notes-content"),
  update: (content: string) => http.put("/notes-content", { content }),
};

// --- Settings ---
// 段名与后端 KnownSections 对齐；Spec 42 新增 seo（站点级 SEO）与 inject（ 代码注入 inject_head/inject_footer）两段。
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
  create: (slug: string, data: { share_type: string; expires_at: string }) =>
    http.post(`/posts/${slug}/shares`, data),
  delete: (id: number) => http.delete(`/shares/${id}`),
};

// --- Wiki Docs（Spec 35a 文库文档管理；项目来自 settings wiki 段） ---
// 文档字段：slug / title / order / parent(父文档 ID, 0=顶级) / content_md / status(published|draft)。
export const wikiApi = {
  listDocs: (project: string) =>
    http.get(`/wiki/${encodeURIComponent(project)}/docs`),
  getDoc: (project: string, slug: string) =>
    http.get(`/wiki/${encodeURIComponent(project)}/docs/${encodeURIComponent(slug)}`),
  createDoc: (project: string, data: Record<string, unknown>) =>
    http.post(`/wiki/${encodeURIComponent(project)}/docs`, data),
  updateDoc: (project: string, slug: string, data: Record<string, unknown>) =>
    http.put(`/wiki/${encodeURIComponent(project)}/docs/${encodeURIComponent(slug)}`, data),
  deleteDoc: (project: string, slug: string) =>
    http.delete(`/wiki/${encodeURIComponent(project)}/docs/${encodeURIComponent(slug)}`),
};
