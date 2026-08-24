import { ref } from "vue";
import { dashboardApi } from "@/api";

export interface DashboardStats {
  post_total: number;
  post_published: number;
  comment_total: number;
  comment_pending: number;
  view_total: number;
  attachment_total: number;
  user_total?: number;
}

const stats = ref<DashboardStats>({
  post_total: 0,
  post_published: 0,
  comment_total: 0,
  comment_pending: 0,
  view_total: 0,
  attachment_total: 0,
});

// 模块级 promise 去重：同一次挂载周期内多个统计卡片部件共享同一次请求，
// 请求完成后重置，下次进入页面重新拉取最新数据。
let pending: Promise<void> | null = null;

/** 共享仪表盘统计数据（多个 StatCardWidget 只发一次请求） */
export function useDashboardStats() {
  if (!pending) {
    pending = dashboardApi
      .stats()
      .then(({ data }) => {
        stats.value = data;
      })
      .catch(() => {})
      .finally(() => {
        pending = null;
      }) as Promise<void>;
  }
  return { stats };
}
