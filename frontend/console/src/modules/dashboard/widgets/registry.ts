import type { Component } from "vue";
import StatCardWidget from "./StatCardWidget.vue";
import QuickActionsWidget from "./QuickActionsWidget.vue";
import RecentCommentsWidget from "./RecentCommentsWidget.vue";
import TrendChartWidget from "./TrendChartWidget.vue";
import HourlyChartWidget from "./HourlyChartWidget.vue";

/** 部件尺寸档位 → 12 列网格 col-span 类 */
export type WidgetSize = "small" | "medium" | "large";

export const sizeClass: Record<WidgetSize, string> = {
  small: "col-span-6 lg:col-span-3",
  medium: "col-span-12 lg:col-span-6",
  large: "col-span-12",
};

export const sizeLabel: Record<WidgetSize, string> = {
  small: "小",
  medium: "中",
  large: "大",
};

export interface WidgetDefinition {
  type: string;
  label: string;
  icon: string;
  size: WidgetSize;
  component: Component;
  props?: Record<string, unknown>;
}

/** 部件注册表（type → 定义），顺序即「添加部件」弹窗的陈列顺序 */
export const widgetRegistry: WidgetDefinition[] = [
  {
    type: "stat-posts",
    label: "文章",
    icon: "ri:book-read-line",
    size: "small",
    component: StatCardWidget,
    props: { statKey: "post_total", label: "文章", icon: "ri:book-read-line" },
  },
  {
    type: "stat-comments",
    label: "评论",
    icon: "ri:chat-3-line",
    size: "small",
    component: StatCardWidget,
    props: { statKey: "comment_total", label: "评论", icon: "ri:chat-3-line" },
  },
  {
    type: "stat-views",
    label: "浏览量",
    icon: "ri:eye-line",
    size: "small",
    component: StatCardWidget,
    props: { statKey: "view_total", label: "浏览量", icon: "ri:eye-line" },
  },
  {
    type: "stat-attachments",
    label: "附件",
    icon: "ri:folder-2-line",
    size: "small",
    component: StatCardWidget,
    props: { statKey: "attachment_total", label: "附件", icon: "ri:folder-2-line" },
  },
  {
    type: "quick-actions",
    label: "快捷操作",
    icon: "ri:flashlight-line",
    size: "large",
    component: QuickActionsWidget,
  },
  {
    type: "recent-comments",
    label: "最近评论",
    icon: "ri:chat-history-line",
    size: "medium",
    component: RecentCommentsWidget,
  },
  {
    type: "trend-chart",
    label: "访问趋势",
    icon: "ri:line-chart-line",
    size: "medium",
    component: TrendChartWidget,
  },
  {
    type: "hourly-chart",
    label: "访问时段分布",
    icon: "ri:bar-chart-2-line",
    size: "medium",
    component: HourlyChartWidget,
  },
];

export function getWidget(type: string): WidgetDefinition | undefined {
  return widgetRegistry.find((w) => w.type === type);
}

/** 默认布局（与后端 defaultDashboardLayout 保持一致） */
export const defaultLayout = widgetRegistry.map((w) => ({ type: w.type }));
