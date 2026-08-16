import axios from"axios";
import router from"@/router";

const http = axios.create({
 baseURL:"/api/v1",
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
 // 登录日志（Wave 3a）：分页查询，status 空=全部 / success / fail
 loginLogs: (params: { page?: number; size?: number; status?: string }) =>
 http.get("/auth/login-logs", { params }),
};

// --- Profile（Spec 45 用户资料） ---
export interface UserProfile {
 id?: number;
 username: string;
 display_name: string;
 email: string;
 avatar: string;
 has_2fa: boolean;
 role?: string;
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
 // --- API Token（Wave 3a：Bearer 认证凭证，明文仅生成时展示一次） ---
 apiTokenStatus() {
 return http.get<{ has_token: boolean }>("/profile/api-token");
 },
 generateApiToken() {
 return http.post<{ token: string; username: string }>("/profile/api-token");
 },
 revokeApiToken() {
 return http.delete("/profile/api-token");
 },
};

// --- Users（多用户体系，admin-only） ---
export interface UserItem {
 id: number;
 username: string;
 display_name: string;
 email: string;
 avatar: string;
 role:"admin" |"author";
 created_at: string;
}

export const usersApi = {
 list: (params: { page?: number; size?: number; keyword?: string }) =>
 http.get<{ items: UserItem[]; total: number; page: number; size: number }>("/users", { params }),
 create: (data: {
 username: string;
 display_name: string;
 email: string;
 avatar?: string;
 password: string;
 role:"admin" |"author";
 }) => http.post<UserItem>("/users", data),
 update: (
 id: number,
 data: { display_name: string; email: string; avatar?: string; role:"admin" |"author" }
 ) => http.put<UserItem>(`/users/${id}`, data),
 resetPassword: (id: number, password: string) => http.put(`/users/${id}/password`, { password }),
 remove: (id: number) => http.delete(`/users/${id}`),
};

// --- Dashboard ---
export interface DashboardLayoutItem {
 type: string;
}

export const dashboardApi = {
 stats: () => http.get("/dashboard/stats"),
 recentPosts: () => http.get("/dashboard/recent-posts"),
 recentComments: () => http.get("/dashboard/recent-comments"),
 categories: () => http.get("/dashboard/categories"),
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
 hourly: (days: number = 7) => http.get("/stats/hourly", { params: { days } }),
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
 // AI 文章摘要（Spec 53）：保存/清空预生成摘要（独立于主保存链路）
 updateAISummary: (slug: string, summary: string) =>
 http.put(`/posts/${slug}/ai-summary`, { summary }),
 // 草稿头（已保存未发布，列表小绿点）
 saveDraft: (slug: string, data: Record<string, unknown>) =>
 http.put(`/posts/${slug}/draft`, data),
 publish: (slug: string, data: Record<string, unknown>) =>
 http.post(`/posts/${slug}/publish`, data),
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
 list: (params?: Record<string, unknown>) =>
 http.get("/comments", { params }),
 approve: (id: number) => http.put(`/comments/${id}/approve`),
 setStatus: (id: number, status: CommentStatus) =>
 http.put(`/comments/${id}/status`, { status }),
 delete: (id: number) => http.delete(`/comments/${id}`),
 reply: (data: { parent_id: number; content: string }) =>
 http.post("/comments/reply", data),
 batch: (data: { ids: number[]; action: "approve" | "reject" | "spam" | "delete" }) =>
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
 headers: {"Content-Type":"multipart/form-data" },
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
 /** 渲染 Markdown。previewProfile=component-library（Spec 63）时后端进入本地 Fixture 模式 */
 render: (markdown: string, previewProfile?: string) =>
  http.post("/preview/markdown", { markdown, preview_profile: previewProfile }),
 /** 发布前预览：提交当前表单拿临时 token，新标签页打开 /preview/draft/:token */
 createPostPreview: (form: Record<string, unknown>) =>
 http.post("/preview/post", form),
 /** 站点配置预览（设置页所见即所得）：草稿分段 → 前台整页 HTML */
 site: (sections: Record<string, unknown>, page: string) =>
 http.post<{ html: string }>("/preview/site", { sections, page }),
};

// --- AI 属性生成（OpenAI 兼容代理，配置在 设置 → AI） ---
export interface AiGeneratePayload {
 fields: string[];
 title: string;
 content: string;
 categories: string[];
 tags: string[];
}
export const aiApi = {
 generate: (payload: AiGeneratePayload) => http.post("/ai/generate", payload),
};

// --- 备份与导出 ---
// 下载接口走 cookie 认证，直接 <a href> 触发浏览器下载，不走 axios。
export const backupExportUrl ="/api/v1/backup/export";
export const backupExportMarkdownUrl ="/api/v1/backup/export/markdown";
export const backupApi = {
 status: () => http.get("/backup/status"),
 importMarkdown: (file: File) => {
 const form = new FormData();
 form.append("file", file);
 return http.post("/backup/import/markdown", form, {
 headers: {"Content-Type":"multipart/form-data" },
 });
 },
 // 整站恢复（Spec 41）：上传全量备份 zip / 从服务器留存备份还原
 restore: (file: File, strict: boolean) => {
 const form = new FormData();
 form.append("file", file);
 form.append("strict", strict ?"true" :"false");
 return http.post("/backup/restore", form, {
 headers: {"Content-Type":"multipart/form-data" },
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

// --- Settings ---
// 段名与后端 KnownSections 对齐；Spec 42 新增 seo（站点级 SEO）与 inject（ 代码注入 inject_head/inject_footer）两段。
export const settingsApi = {
 all: () => http.get("/settings"),
 updateSection: (section: string, data: unknown) =>
 http.put(`/settings/${section}`, data),
 testNotify: () => http.post("/settings/notify/test"),
 // Spec 60 主题 YAML 编辑器：style 段 YAML 读写 + 热更新
 getStyleYAML: () => http.get<{ yaml: string }>("/settings/style/yaml"),
 updateStyleYAML: (yamlText: string) =>
 http.put("/settings/style/yaml", yamlText, { headers: { "Content-Type": "text/yaml" } }),
};

// --- Friends / Links ---
export const linkApi = {
 list: () => http.get("/links"),
 create: (data: Record<string, unknown>) => http.post("/links", data),
 update: (id: number, data: Record<string, unknown>) =>
 http.put(`/links/${id}`, data),
 delete: (id: number) => http.delete(`/links/${id}`),
 /** 并发检测各友链可达性，返回 {items: [{id, ok, status}], tagged, restored}（status 为 HTTP 状态码，0=无法访问；tagged/restored 为自动打/摘「失效」标签条数） */
	check: () => http.post("/links/check"),
	snapshot: (id: number) => http.post(`/links/${id}/snapshot`),
 /** 导入友链（Spec 54/56）：source=github 拉 Friends 仓库 issues，source=text 解析粘贴的 JSON/YAML；返回 {imported, skipped, failed[]} */
 importLinks: (data: { source:"github" |"text"; group?: string; content?: string }) =>
 http.post("/links/import", data),
};

// --- 友链 RSS 订阅阅读器 ---
export const subscriptionApi = {
 sources: () => http.get("/subscriptions"),
 articles: (params: { feed?: string; filter?: string; page?: number; size?: number }) =>
 http.get("/subscriptions/articles", { params }),
 markRead: (id: number, read: boolean) => http.post("/subscriptions/read", { id, read }),
 markAllRead: (feed?: string) =>
 http.post("/subscriptions/read-all", null, { params: feed ? { feed } : {} }),
 toggleFlag: (id: number, flag:"favorite" |"later", value: boolean) =>
 http.post("/subscriptions/flag", { id, flag, value }),
 refresh: () => http.post("/subscriptions/refresh"),
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

// --- Assets (素材速查：插件库图标画廊数据源) ---
export interface AssetIconItem {
  key: string;
  /** 内联 SVG 或图片 URL */
  value: string;
}

export const assetsApi = {
  icons: () => http.get<{ items: AssetIconItem[] }>("/assets/icons"),
};

// --- 管理端全局搜索（Spec 39，⌘K 弹窗）---
export interface SearchResultItem {
 id: number;
 type:"post" |"page";
 slug: string;
 title: string;
 status:"published" |"draft" |"trashed";
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

// --- Notebooks（Spec 35b 笔记本/笔记管理） ---
// 后端响应格式：列表 {items,total}；笔记列表 {notes,total,page,per_page}；note-tags {tags}。
// Notebook: id/name(唯一, URL slug)/description/cover/order/created_at；Note: id/notebook_id/title/content/content_html/tags(逗号分隔)/created_at/updated_at。
export const notebookApi = {
 listNotebooks: () => http.get("/notebooks"),
 getNotebook: (id: number) => http.get(`/notebooks/${id}`),
 createNotebook: (data: {
 name: string;
 description?: string;
 cover?: string;
 order?: number;
 }) => http.post("/notebooks", data),
 updateNotebook: (
 id: number,
 data: { name?: string; description?: string; cover?: string; order?: number }
 ) => http.put(`/notebooks/${id}`, data),
 deleteNotebook: (id: number) => http.delete(`/notebooks/${id}`),
 listNotes: (notebookId: number, params?: Record<string, unknown>) =>
 http.get(`/notebooks/${notebookId}/notes`, { params }),
 createNote: (notebookId: number, data: { title: string; content?: string; tags?: string }) =>
 http.post(`/notebooks/${notebookId}/notes`, data),
 getNote: (id: number) => http.get(`/notes/${id}`),
 updateNote: (id: number, data: Record<string, unknown>) =>
 http.put(`/notes/${id}`, data),
 deleteNote: (id: number) => http.delete(`/notes/${id}`),
 getTags: () => http.get("/note-tags"),
};

// --- 功能模块（manifest + 启停） ---
export const moduleApi = {
 /** 全部模块 meta + 启停状态 */
 list: () => http.get("/modules"),
 /** 启用/禁用模块 */
 toggle: (id: string, enabled: boolean) => http.put(`/modules/${id}`, { enabled }),
};

// --- SEO 工具集模块 ---
export const seoApi = {
 getSettings: () => http.get("/seo/settings"),
 saveSettings: (data: Record<string, unknown>) => http.put("/seo/settings", data),
 overview: () => http.get("/seo/overview"),
 analyze: (url: string) => http.post("/seo/analyze", { url }),
 crawlerAnalysis: (range: string) => http.get("/seo/crawlers/analysis", { params: { range } }),
 crawlerLogs: (params: { page?: number; size?: number; engine?: string }) =>
 http.get("/seo/crawlers/logs", { params }),
 trafficAnalysis: (range: string) => http.get("/seo/traffic/analysis", { params: { range } }),
 trafficLanding: (range: string) => http.get("/seo/traffic/landing", { params: { range } }),
 pushManual: (urls: string[]) => http.post("/seo/push", { urls }),
 pushLogs: (params?: { page?: number; size?: number; engine?: string; status?: string }) =>
 http.get("/seo/push/logs", { params }),
 clearPushLogs: () => http.delete("/seo/push/logs"),
};

// --- 性能监控模块 ---
export interface PerfOverview {
 start_time: string;
 uptime_sec: number;
 go_version: string;
 platform: string;
 goroutines: number;
 heap_alloc_mb: number;
 sys_mem_mb: number;
 gc_count: number;
 db_size_bytes: number;
 recent: { requests: number; avg_ms: number; errors: number; err_rate: number };
}

export interface PerfSamplePoint {
 ts: string;
 metric: string;
 value: number;
}

export interface PerfRouteStat {
 method: string;
 route: string;
 count: number;
 avg_ms: number;
 max_ms: number;
 err_count: number;
}

export interface PerfMinuteStat {
 minute: string;
 count: number;
 err_count: number;
}

export const perfApi = {
 overview: () => http.get<PerfOverview>("/perf/overview"),
 history: (hours: number) =>
 http.get<{ hours: number; points: PerfSamplePoint[] }>("/perf/history", { params: { hours } }),
 requests: (hours: number) =>
 http.get<{ hours: number; routes: PerfRouteStat[]; throughput: PerfMinuteStat[] }>("/perf/requests", { params: { hours } }),
};

// --- 站内通知中心（Wave 3b） ---
export interface NotificationItem {
 id: number;
 user_id: number;
 type: string;
 title: string;
 content: string;
 link: string;
 read: number;
 created_at: string;
}

export const notificationApi = {
 /** 分页查询（未读优先），响应含 unread 未读数 */
 list: (params: { page?: number; size?: number }) =>
 http.get<{ items: NotificationItem[]; total: number; unread: number; page: number; size: number }>(
"/notifications",
 { params }
 ),
 /** 未读数（铃铛角标轮询用） */
 unreadCount: () => http.get<{ count: number }>("/notifications/unread-count"),
 /** 标记单条已读 */
 markRead: (id: number) => http.put(`/notifications/${id}/read`),
 /** 全部标记已读 */
 markAllRead: () => http.put("/notifications/read-all"),
};
