import { onMounted, onUnmounted, ref } from "vue";
import { dashboardApi } from"@/api";

export interface DashboardStats {
 post_total: number;
 post_published: number;
 comment_total: number;
 comment_pending: number;
 view_total: number;
 attachment_total: number;
 user_total?: number;
 post_change?: number;
 view_change?: number;
 comment_change?: number;
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
let forcedRequest: Promise<void> | null = null;

export const DASHBOARD_REFRESH_EVENT = "dashboard:refresh";

/** 拉取统计；force 会绕过当前去重请求，确保手动刷新拿到新快照。 */
export function refreshStats(force = false): Promise<void> {
 if (force && forcedRequest) return forcedRequest;
 if (force) pending = null;
 if (pending) return pending;

 const request = dashboardApi
 .stats()
 .then(({ data }) => {
 stats.value = data;
 })
 .catch(() => {})
 .finally(() => {
 if (pending === request) pending = null;
 if (forcedRequest === request) forcedRequest = null;
 });
 pending = request;
 if (force) forcedRequest = request;
 return request;
}

/** 共享仪表盘统计数据（多个 StatCardWidget 只发一次请求） */
export function useDashboardStats() {
 refreshStats();
 const onRefresh = () => { void refreshStats(true); };
 onMounted(() => window.addEventListener(DASHBOARD_REFRESH_EVENT, onRefresh));
 onUnmounted(() => window.removeEventListener(DASHBOARD_REFRESH_EVENT, onRefresh));
 return { stats, refreshStats };
}
