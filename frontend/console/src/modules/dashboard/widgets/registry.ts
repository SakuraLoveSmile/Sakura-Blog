import type { Component } from "vue";
import StatCardWidget from "./StatCardWidget.vue";
import TrendChartWidget from "./TrendChartWidget.vue";
import HourlyChartWidget from "./HourlyChartWidget.vue";
import RecentCommentsWidget from "./RecentCommentsWidget.vue";
import RecentPostsWidget from "./RecentPostsWidget.vue";
import CategoryDonutWidget from "./CategoryDonutWidget.vue";
import QuickActionsWidget from "./QuickActionsWidget.vue";

export type WidgetSize = "small" | "medium" | "large";

/** 组件在单个断点下的列跨度（12列网格） */
export type WidgetSpan = 3 | 4 | 6 | 8 | 12;

/** 响应式尺寸配置：L/M/S 三断点下各自的列跨度 */
export interface ResponsiveSize {
  l: WidgetSpan;
  m: WidgetSpan;
  s: WidgetSpan;
}

export interface WidgetDefinition {
  type: string;
  label: string;
  icon: string;
  defaultSize: ResponsiveSize;
  /** 推荐的区域（用于默认布局） */
  defaultSection: "stats" | "chartWide" | "chartNarrow" | "recents" | "others";
  component: Component;
  props: Record<string, unknown>;
}

/** 布局中的组件条目 */
export interface LayoutWidget {
  type: string;
  size: ResponsiveSize;
}

/** 预设响应式尺寸配置 */
export const SIZE_PRESETS: Record<WidgetSize, ResponsiveSize> = {
  small:  { l: 3, m: 6, s: 12 },
  medium: { l: 6, m: 6, s: 12 },
  large:  { l: 8, m: 12, s: 12 },
};

export const SPAN_LABELS: Record<WidgetSpan, string> = {
  3: "1/4",
  4: "1/3",
  6: "1/2",
  8: "2/3",
  12: "全宽",
};

export const widgetRegistry: WidgetDefinition[] = [
  {
    type: "stat-posts",
    label: "文章",
    icon: "ri:book-read-line",
    defaultSize: SIZE_PRESETS.small,
    defaultSection: "stats",
    component: StatCardWidget,
    props: { statKey: "post_total", label: "文章", icon: "ri:book-read-line", color: "#0A84FF" },
  },
  {
    type: "stat-published",
    label: "已发布",
    icon: "ri:checkbox-circle-line",
    defaultSize: SIZE_PRESETS.small,
    defaultSection: "stats",
    component: StatCardWidget,
    props: { statKey: "post_published", label: "已发布", icon: "ri:checkbox-circle-line", color: "#30D158" },
  },
  {
    type: "stat-views",
    label: "浏览量",
    icon: "ri:eye-line",
    defaultSize: SIZE_PRESETS.small,
    defaultSection: "stats",
    component: StatCardWidget,
    props: { statKey: "view_total", label: "浏览量", icon: "ri:eye-line", color: "#BF5AF2" },
  },
  {
    type: "stat-comments",
    label: "评论",
    icon: "ri:chat-3-line",
    defaultSize: SIZE_PRESETS.small,
    defaultSection: "stats",
    component: StatCardWidget,
    props: { statKey: "comment_total", label: "评论", icon: "ri:chat-3-line", color: "#FF9F0A" },
  },
  {
    type: "stat-attachments",
    label: "附件",
    icon: "ri:attachment-2",
    defaultSize: SIZE_PRESETS.small,
    defaultSection: "stats",
    component: StatCardWidget,
    props: { statKey: "attachment_total", label: "附件", icon: "ri:attachment-2", color: "#5AC8FA" },
  },
  {
    type: "stat-users",
    label: "用户",
    icon: "ri:user-3-line",
    defaultSize: SIZE_PRESETS.small,
    defaultSection: "stats",
    component: StatCardWidget,
    props: { statKey: "user_total", label: "用户", icon: "ri:user-3-line", color: "#FF453A" },
  },
  {
    type: "trend-chart",
    label: "访问趋势",
    icon: "ri:line-chart-line",
    defaultSize: { l: 8, m: 12, s: 12 },
    defaultSection: "chartWide",
    component: TrendChartWidget,
    props: {},
  },
  {
    type: "hourly-chart",
    label: "访问时段",
    icon: "ri:bar-chart-2-line",
    defaultSize: { l: 4, m: 6, s: 12 },
    defaultSection: "chartNarrow",
    component: HourlyChartWidget,
    props: {},
  },
  {
    type: "category-donut",
    label: "分类分布",
    icon: "ri:pie-chart-line",
    defaultSize: { l: 4, m: 6, s: 12 },
    defaultSection: "chartNarrow",
    component: CategoryDonutWidget,
    props: {},
  },
  {
    type: "recent-posts",
    label: "最近文章",
    icon: "ri:file-list-3-line",
    defaultSize: SIZE_PRESETS.medium,
    defaultSection: "recents",
    component: RecentPostsWidget,
    props: {},
  },
  {
    type: "recent-comments",
    label: "最近评论",
    icon: "ri:chat-3-line",
    defaultSize: SIZE_PRESETS.medium,
    defaultSection: "recents",
    component: RecentCommentsWidget,
    props: {},
  },
  {
    type: "quick-actions",
    label: "快捷操作",
    icon: "ri:flashlight-line",
    defaultSize: { l: 4, m: 6, s: 12 },
    defaultSection: "chartNarrow",
    component: QuickActionsWidget,
    props: {},
  },
];
