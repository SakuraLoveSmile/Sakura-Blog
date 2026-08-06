import { settingsApi } from"@/api";
import type { DashboardLayoutItem } from"@/api";
import { getWidget, widgetRegistry, defaultLayout } from"./registry";

/** settings 表 dashboard 段的部件条目（与后端 config.dashboardWidget 一致） */
interface DashboardWidgetEntry {
 id: string;
 enabled: boolean;
}

// 旧版仪表盘配置（开关+排序实现）的 id → 新部件注册表 type 迁移映射。
// stats-cards 拆为 4 张统计卡片；pending-alert 已改为固定横幅，不再是部件。
const legacyIdMap: Record<string, string[]> = {
"stats-cards": ["stat-posts","stat-comments","stat-views","stat-attachments"],
"visit-trend": ["trend-chart"],
"hourly-dist": ["hourly-chart"],
"pending-alert": [],
};

/** 单个条目 id 归一化为注册表 type 列表（未知 id 丢弃） */
function normalizeId(id: string): string[] {
 if (getWidget(id)) return [id];
 return legacyIdMap[id] ?? [];
}

/**
 * 读取仪表盘布局：settings dashboard 段 → 启用条目按序展开为部件类型数组。
 * 段缺失 / 数据损坏 / 展开后为空时回落默认布局。
 */
export async function loadDashboardLayout(): Promise<DashboardLayoutItem[]> {
 try {
 const { data } = await settingsApi.all();
 const widgets: DashboardWidgetEntry[] | undefined = data?.dashboard?.widgets;
 if (!Array.isArray(widgets) || !widgets.length) return [...defaultLayout];
 const types = widgets
 .filter((w) => w.enabled)
 .flatMap((w) => normalizeId(w.id));
 // 去重（legacy 展开可能与新 id 并存）
 const seen = new Set<string>();
 const items = types
 .filter((t) => (seen.has(t) ? false : (seen.add(t), true)))
 .map((type) => ({ type }));
 return items.length ? items : [...defaultLayout];
 } catch {
 return [...defaultLayout];
 }
}

/**
 * 保存仪表盘布局到 settings dashboard 段：
 * 布局中的部件按序写 enabled:true，未使用的注册表部件补 enabled:false（保留 dev 版配置段 schema）。
 */
export async function saveDashboardLayout(items: DashboardLayoutItem[]): Promise<void> {
 const used = new Set(items.map((it) => it.type));
 const widgets: DashboardWidgetEntry[] = [
 ...items.map((it) => ({ id: it.type, enabled: true })),
 ...widgetRegistry.filter((w) => !used.has(w.type)).map((w) => ({ id: w.type, enabled: false })),
 ];
 await settingsApi.updateSection("dashboard", { widgets });
}
