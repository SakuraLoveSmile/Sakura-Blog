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

/**
 * Official widget extension point: `widget:registry:register`.
 *
 * Widgets are compile-time registrations in this registry. A future external
 * contributor may add a definition here or through a generated module registry;
 * runtime loading is intentionally not part of the current contract.
 */
export const WIDGET_REGISTRY_EXTENSION_POINT = "widget:registry:register";

/** Reserved editor extension point: `editor:toolbar:item:register`. */
export const EDITOR_TOOLBAR_EXTENSION_POINT = "editor:toolbar:item:register";

const chartColors = themeColors({
  primary: "#2E8DFF",
  success: "#30D158",
  chart5: "#BF5AF2",
  warning: "#FF9F0A",
  primaryHover: "#66ABFF",
  error: "#FF453A",
});

export interface WidgetConfigField {
  key: string;
  label: string;
  type: "text" | "number" | "select" | "boolean";
  options?: string[];
}

export interface WidgetDefinition {
  id: string;
  label: string;
  icon: string;
  group: string;
  component: Component;
  props: Record<string, unknown>;
  defaultSize: { w: number; h: number };
  defaultConfig: Record<string, unknown>;
  minW?: number;
  minH?: number;
  maxW?: number;
  maxH?: number;
  configFields?: WidgetConfigField[];
}

type WidgetDefinitionInput = Omit<WidgetDefinition, "group" | "defaultSize" | "defaultConfig"> & {
  group?: string;
  defaultSize?: { w: number; h: number };
  defaultConfig?: Record<string, unknown>;
};

function defineWidget(input: WidgetDefinitionInput): WidgetDefinition {
  return {
    ...input,
    group: input.group ?? "general",
    defaultSize: input.defaultSize ?? { w: 6, h: 24 },
    defaultConfig: input.defaultConfig ?? {},
  };
}

export const widgetRegistry: WidgetDefinition[] = [
  defineWidget({
    id: "stat-posts",
    label: "文章",
    icon: "ri:book-read-line",
    group: "statistics",
    component: StatCardWidget,
    props: { statKey: "post_total", label: "文章", icon: "ri:book-read-line", color: chartColors.primary },
    defaultSize: { w: 3, h: 19 },
  }),
  defineWidget({
    id: "stat-published",
    label: "已发布",
    icon: "ri:checkbox-circle-line",
    group: "statistics",
    component: StatCardWidget,
    props: { statKey: "post_published", label: "已发布", icon: "ri:checkbox-circle-line", color: chartColors.success },
    defaultSize: { w: 3, h: 19 },
  }),
  defineWidget({
    id: "stat-views",
    label: "浏览量",
    icon: "ri:eye-line",
    group: "statistics",
    component: StatCardWidget,
    props: { statKey: "view_total", label: "浏览量", icon: "ri:eye-line", color: chartColors.chart5 },
    defaultSize: { w: 3, h: 19 },
  }),
  defineWidget({
    id: "stat-comments",
    label: "评论",
    icon: "ri:chat-3-line",
    group: "statistics",
    component: StatCardWidget,
    props: { statKey: "comment_total", label: "评论", icon: "ri:chat-3-line", color: chartColors.warning },
    defaultSize: { w: 3, h: 19 },
  }),
  defineWidget({
    id: "stat-attachments",
    label: "附件",
    icon: "ri:attachment-2",
    group: "statistics",
    component: StatCardWidget,
    props: { statKey: "attachment_total", label: "附件", icon: "ri:attachment-2", color: chartColors.primaryHover },
    defaultSize: { w: 3, h: 19 },
  }),
  defineWidget({
    id: "stat-users",
    label: "用户",
    icon: "ri:user-3-line",
    group: "statistics",
    component: StatCardWidget,
    props: { statKey: "user_total", label: "用户", icon: "ri:user-3-line", color: chartColors.error },
    defaultSize: { w: 3, h: 19 },
  }),
  defineWidget({
    id: "trend-chart",
    label: "访问趋势",
    icon: "ri:line-chart-line",
    group: "statistics",
    component: TrendChartWidget,
    props: {},
    defaultSize: { w: 8, h: 52 },
  }),
  defineWidget({
    id: "hourly-chart",
    label: "访问时段",
    icon: "ri:bar-chart-2-line",
    group: "statistics",
    component: HourlyChartWidget,
    props: {},
    defaultSize: { w: 4, h: 40 },
  }),
  defineWidget({
    id: "category-donut",
    label: "分类分布",
    icon: "ri:pie-chart-line",
    group: "content",
    component: CategoryDonutWidget,
    props: {},
    defaultSize: { w: 4, h: 34 },
  }),
  defineWidget({
    id: "recent-posts",
    label: "最近文章",
    icon: "ri:file-list-3-line",
    group: "content",
    component: RecentPostsWidget,
    props: {},
    defaultSize: { w: 6, h: 36 },
  }),
  defineWidget({
    id: "recent-comments",
    label: "最近评论",
    icon: "ri:chat-3-line",
    group: "content",
    component: RecentCommentsWidget,
    props: {},
    defaultSize: { w: 6, h: 36 },
  }),
  defineWidget({
    id: "quick-actions",
    label: "快捷操作",
    icon: "ri:flashlight-line",
    group: "tools",
    component: QuickActionsWidget,
    props: {},
    defaultSize: { w: 4, h: 35 },
  }),
  defineWidget({
    id: "stellar-version",
    label: "Stellar 版本",
    icon: "ri:git-branch-line",
    group: "system",
    component: StellarVersionWidget,
    props: {},
    defaultSize: { w: 4, h: 25 },
  }),
  defineWidget({
    id: "system-version",
    label: "Blog 版本",
    icon: "ri:rocket-2-line",
    group: "system",
    component: SystemVersionWidget,
    props: {},
    defaultSize: { w: 4, h: 25 },
  }),
];

export function getWidgetDefinition(id: string): WidgetDefinition | undefined {
  return widgetRegistry.find((widget) => widget.id === id);
}
