import { http } from "../transport";
import type { SeoSettingsPayload } from "./contracts";

// --- SEO 工具集模块 ---
export const seoApi = {
  getSettings: () => http.get("/seo/settings"),
  saveSettings: (data: SeoSettingsPayload) => http.put("/seo/settings", data),
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
    http.get<{ hours: number; routes: PerfRouteStat[]; throughput: PerfMinuteStat[] }>("/perf/requests", {
      params: { hours },
    }),
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
      { params },
    ),
  /** 未读数（铃铛角标轮询用） */
  unreadCount: () => http.get<{ count: number }>("/notifications/unread-count", { mute: true }),
  /** 标记单条已读 */
  markRead: (id: number) => http.put(`/notifications/${id}/read`),
  /** 全部标记已读 */
  markAllRead: () => http.put("/notifications/read-all"),
};

// --- 系统更新（检查更新 / 一键更新 / 状态查询 / 版本识别） ---
// UpdateCheckResponse 字段参考：apply_note?: string
import type {
  SystemVersionResponse,
  UpdateCheckResponse,
  UpdateApplyResponse,
  UpdateStatusResponse,
} from "../generated";

export const systemApi = {
  /** 系统构建版本信息（管理员专属） */
  version: () => http.get<SystemVersionResponse>("/system/version"),
  /** 检查更新（refresh=1 绕过缓存直连 GitHub；token 为可选 GitHub Token，经 X-GitHub-Token 头携带以鉴权私有仓库） */
  check: (refresh?: boolean, token?: string) =>
    http.get<UpdateCheckResponse>("/system/update/check", {
      params: refresh ? { refresh: "1" } : {},
      ...(token ? { headers: { "X-GitHub-Token": token } } : {}),
    }),
  /** 一键更新（触发后台更新请求；token 同 check） */
  apply: (token?: string) =>
    http.post<UpdateApplyResponse>(
      "/system/update/apply",
      {},
      token ? { headers: { "X-GitHub-Token": token } } : {},
    ),
  /** 查询更新状态（idle/running/succeeded/failed） */
  status: () => http.get<UpdateStatusResponse>("/system/update/status", { mute: true }),
};
