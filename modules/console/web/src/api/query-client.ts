import { QueryClient } from "@tanstack/vue-query";

/**
 * 全局 QueryClient 实例。
 * 配置合理的默认 staleTime 和 refetchOnWindowFocus，
 * 避免列表页频繁重复请求。
 */
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // 数据在 10 秒内视为新鲜，不触发自动重取
      staleTime: 10_000,
      // 窗口聚焦时不自动重取（后台管理页不需要此行为）
      refetchOnWindowFocus: false,
      // 失败重试 1 次
      retry: 1,
    },
  },
});

/**
 * Query Key 工厂函数，集中管理所有 query key，
 * 避免散落在各组件中的字符串拼接出错。
 */
export const queryKeys = {
  posts: (params?: Record<string, unknown>) =>
    params ? (["posts", params] as const) : (["posts"] as const),
  post: (slug: string) => ["post", slug] as const,
  pages: (params?: Record<string, unknown>) =>
    params ? (["pages", params] as const) : (["pages"] as const),
  comments: (params?: Record<string, unknown>) =>
    params ? (["comments", params] as const) : (["comments"] as const),
  attachments: (params?: Record<string, unknown>) =>
    params ? (["attachments", params] as const) : (["attachments"] as const),
  moments: (params?: Record<string, unknown>) =>
    params ? (["moments", params] as const) : (["moments"] as const),
  friends: () => ["friends"] as const,
  shares: (params?: Record<string, unknown>) =>
    params ? (["shares", params] as const) : (["shares"] as const),
  loginLogs: (params?: Record<string, unknown>) =>
    params ? (["login-logs", params] as const) : (["login-logs"] as const),
  notifications: (params?: Record<string, unknown>) =>
    params ? (["notifications", params] as const) : (["notifications"] as const),
  unread: () => ["notifications", "unread"] as const,
  users: (params?: Record<string, unknown>) =>
    params ? (["users", params] as const) : (["users"] as const),
  commentUsers: (params?: Record<string, unknown>) =>
    params ? (["comment-users", params] as const) : (["comment-users"] as const),
  categories: () => ["categories"] as const,
  tags: () => ["tags"] as const,
  dashboard: {
    stats: () => ["dashboard", "stats"] as const,
    recentComments: () => ["dashboard", "recent-comments"] as const,
  },
  stats: {
    trend: (days: number) => ["stats", "trend", days] as const,
    popular: (days: number, limit: number) => ["stats", "popular", days, limit] as const,
  },
  backup: {
    status: () => ["backup", "status"] as const,
  },
  settings: () => ["settings"] as const,
};
