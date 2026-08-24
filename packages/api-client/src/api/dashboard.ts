import { http } from "../transport";
import type { DashboardResponsiveLayout } from "../types";

// --- Dashboard ---
export interface StellarVersionResponse {
  local_version: string;
  upstream_version: string;
  in_sync: boolean;
  release_url: string;
  checked_at: string;
  error?: string;
}

export interface DashboardLayoutResponse {
  version: number;
  layouts: DashboardResponsiveLayout | null;
  updated_at?: string;
}

export interface DashboardLayoutDocument {
  version: 10;
  layouts: DashboardResponsiveLayout;
}

export const dashboardApi = {
  stats: () => http.get("/dashboard/stats"),
  layout: () => http.get<DashboardLayoutResponse>("/dashboard/layout"),
  saveLayout: (layouts: DashboardResponsiveLayout) =>
    http.put<DashboardLayoutResponse>("/dashboard/layout", { version: 10, layouts }),
  stellarVersion: () => http.get<StellarVersionResponse>("/dashboard/stellar-version"),
  recentPosts: () => http.get("/dashboard/recent-posts"),
  recentComments: () => http.get("/dashboard/recent-comments"),
  categories: () => http.get("/dashboard/categories"),
  quickActions: () => http.get("/dashboard/quick-actions"),
  saveQuickActions: (items: string[]) => http.put("/dashboard/quick-actions", { items }),
};

// --- Stats (Spec 24) ---
export const statsApi = {
  trend: (days: number = 7) => http.get("/stats/trend", { params: { days } }),
  popular: (days: number = 7, limit: number = 10) => http.get("/stats/popular", { params: { days, limit } }),
  referrers: (days: number = 7) => http.get("/stats/referrers", { params: { days } }),
  hourly: (days: number = 7) => http.get("/stats/hourly", { params: { days } }),
};
