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
  posts: (params?: Record<string, unknown>) => ["posts", params] as const,
  post: (slug: string) => ["post", slug] as const,
  pages: (params?: Record<string, unknown>) => ["pages", params] as const,
  comments: (params?: Record<string, unknown>) => ["comments", params] as const,
  attachments: (params?: Record<string, unknown>) => ["attachments", params] as const,
  moments: (params?: Record<string, unknown>) => ["moments", params] as const,
  friends: () => ["friends"] as const,
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
