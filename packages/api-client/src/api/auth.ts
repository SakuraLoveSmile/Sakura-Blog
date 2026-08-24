import { http } from "../transport";

// --- Auth ---
// Spec 45：username 用户表认证 + TOTP 2FA；username 为空时后端回退旧版 admin 密码认证。
export const authApi = {
  login: (username: string, password: string) => http.post("/auth/login", { username, password }, { mute: true }),
  // 两步登录第二步：验证 TOTP code 后下发签名 cookie
  verify2FA: (token: string, code: string) => http.post("/auth/2fa", { token, code }, { mute: true }),
  logout: () => http.post("/auth/logout", undefined, { mute: true }),
  check: () => http.get("/auth/check", { mute: true }),
  changePassword: (current: string, newPassword: string) => http.put("/auth/password", { current, new: newPassword }),
  // 登录日志（Wave 3a）：分页查询，status 空=全部 / success / fail
  loginLogs: (params: { page?: number; size?: number; status?: string }) => http.get("/auth/login-logs", { params }),
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
  roles?: string[];
  permissions?: string[];
}

export const profileApi = {
  get: () => http.get<UserProfile>("/profile", { mute: true }),
  update: (data: { display_name: string; email: string; avatar: string }) => http.put("/profile", data),
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
  role: "admin" | "author" | "reader";
  roles?: string[];
  permissions?: string[];
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
    role: "admin" | "author" | "reader";
    roles?: Array<"admin" | "author" | "reader">;
  }) => http.post<UserItem>("/users", data),
  update: (id: number, data: { display_name: string; email: string; avatar?: string; role?: "admin" | "author" | "reader"; roles?: Array<"admin" | "author" | "reader"> }) =>
    http.put<UserItem>(`/users/${id}`, data),
  resetPassword: (id: number, password: string) => http.put(`/users/${id}/password`, { password }),
  remove: (id: number) => http.delete(`/users/${id}`),
};

// --- Comment Users（Spec 71 评论用户管理，admin-only） ---
export interface CommentUserItem {
  id: number;
  username: string;
  display_name: string;
  email: string;
  avatar: string;
  role: string;
  status: "active" | "banned";
  created_at: string;
  comment_count: number;
  last_comment_at: string; // 空串=从未评论
}

export const commentUsersApi = {
  list: (params: { page?: number; size?: number; keyword?: string; status?: string }) =>
    http.get<{
      items: CommentUserItem[];
      total: number;
      page: number;
      size: number;
      counts: { all: number; banned: number };
    }>("/comment-users", { params }),
  setStatus: (id: number, status: "active" | "banned") => http.put(`/comment-users/${id}/status`, { status }),
  remove: (id: number) => http.delete(`/comment-users/${id}`),
  bulkComments: (id: number, action: "approve" | "spam" | "delete") =>
    http.post<{ ok: boolean; affected: number }>(`/comment-users/${id}/comments/bulk`, { action }),
};
