import type { Component } from "vue";
import StatCardWidget from "./StatCardWidget.vue";
import TrendChartWidget from "./TrendChartWidget.vue";
import HourlyChartWidget from "./HourlyChartWidget.vue";
import RecentCommentsWidget from "./RecentCommentsWidget.vue";
import RecentPostsWidget from "./RecentPostsWidget.vue";
import CategoryDonutWidget from "./CategoryDonutWidget.vue";
import QuickActionsWidget from "./QuickActionsWidget.vue";
import StellarVersionWidget from "./StellarVersionWidget.vue";
import SystemVersionWidget from "./SystemVersionWidget.vue";
import { themeColors } from "@/lib/theme-colors";

const chartColors = themeColors({
  primary: "#2E8DFF",
  success: "#30D158",
  chart5: "#BF5AF2",
  warning: "#FF9F0A",
  primaryHover: "#66ABFF",
  error: "#FF453A",
});

export interface WidgetDefinition {
  type: string;
  label: string;
  icon: string;
  component: Component;
  props: Record<string, unknown>;
}

export const widgetRegistry: WidgetDefinition[] = [
  {
    type: "stat-posts",
    label: "文章",
    icon: "ri:book-read-line",
    component: StatCardWidget,
    props: { statKey: "post_total", label: "文章", icon: "ri:book-read-line", color: chartColors.primary },
  },
  {
    type: "stat-published",
    label: "已发布",
    icon: "ri:checkbox-circle-line",
    component: StatCardWidget,
    props: { statKey: "post_published", label: "已发布", icon: "ri:checkbox-circle-line", color: chartColors.success },
  },
  {
    type: "stat-views",
    label: "浏览量",
    icon: "ri:eye-line",
    component: StatCardWidget,
    props: { statKey: "view_total", label: "浏览量", icon: "ri:eye-line", color: chartColors.chart5 },
  },
  {
    type: "stat-comments",
    label: "评论",
    icon: "ri:chat-3-line",
    component: StatCardWidget,
    props: { statKey: "comment_total", label: "评论", icon: "ri:chat-3-line", color: chartColors.warning },
  },
  {
    type: "stat-attachments",
    label: "附件",
    icon: "ri:attachment-2",
    component: StatCardWidget,
    props: { statKey: "attachment_total", label: "附件", icon: "ri:attachment-2", color: chartColors.primaryHover },
  },
  {
    type: "stat-users",
    label: "用户",
    icon: "ri:user-3-line",
    component: StatCardWidget,
    props: { statKey: "user_total", label: "用户", icon: "ri:user-3-line", color: chartColors.error },
  },
  {
    type: "trend-chart",
    label: "访问趋势",
    icon: "ri:line-chart-line",
    component: TrendChartWidget,
    props: {},
  },
  {
    type: "hourly-chart",
    label: "访问时段",
    icon: "ri:bar-chart-2-line",
    component: HourlyChartWidget,
    props: {},
  },
  {
    type: "category-donut",
    label: "分类分布",
    icon: "ri:pie-chart-line",
    component: CategoryDonutWidget,
    props: {},
  },
  {
    type: "recent-posts",
    label: "最近文章",
    icon: "ri:file-list-3-line",
    component: RecentPostsWidget,
    props: {},
  },
  {
    type: "recent-comments",
    label: "最近评论",
    icon: "ri:chat-3-line",
    component: RecentCommentsWidget,
    props: {},
  },
  {
    type: "quick-actions",
    label: "快捷操作",
    icon: "ri:flashlight-line",
    component: QuickActionsWidget,
    props: {},
  },
  {
    type: "stellar-version",
    label: "Stellar 版本",
    icon: "ri:git-branch-line",
    component: StellarVersionWidget,
    props: {},
  },
  {
    type: "system-version",
    label: "Blog 版本",
    icon: "ri:rocket-2-line",
    component: SystemVersionWidget,
    props: {},
  },
];
