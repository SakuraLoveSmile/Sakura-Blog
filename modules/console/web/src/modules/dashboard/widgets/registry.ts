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
import TopPostsWidget from "./TopPostsWidget.vue";
import SystemHealthWidget from "./SystemHealthWidget.vue";
import StackWidget from "./StackWidget.vue";
import type { WidgetConfigOption } from "./widget-config";

// 配置工具放在不依赖 .vue 的独立模块，node:test 可直接 import。
export { configFieldOptions, resolveWidgetConfig, configInt } from "./widget-config";
export type { WidgetConfigOption } from "./widget-config";

export interface WidgetConfigField {
  key: string;
  label: string;
  /** 配置控件类型；select 的候选项由 options 提供。 */
  type: "text" | "number" | "select" | "boolean";
  options?: (string | WidgetConfigOption)[];
  description?: string;
  /** number 类型的取值范围，仅用于渲染钳制。 */
  min?: number;
  max?: number;
  /** 布尔项在开关打开时的含义说明。 */
  placeholder?: string;
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
    defaultSize: input.defaultSize ?? { w: 6, h: 28 },
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
    props: { statKey: "post_total", label: "文章" },
    defaultSize: { w: 3, h: 16 },
  }),
  defineWidget({
    id: "stat-published",
    label: "已发布",
    icon: "ri:checkbox-circle-line",
    group: "statistics",
    component: StatCardWidget,
    props: { statKey: "post_published", label: "已发布" },
    defaultSize: { w: 3, h: 16 },
  }),
  defineWidget({
    id: "stat-views",
    label: "浏览量",
    icon: "ri:eye-line",
    group: "statistics",
    component: StatCardWidget,
    props: { statKey: "view_total", label: "浏览量" },
    defaultSize: { w: 3, h: 16 },
  }),
  defineWidget({
    id: "stat-comments",
    label: "评论",
    icon: "ri:chat-3-line",
    group: "statistics",
    component: StatCardWidget,
    props: { statKey: "comment_total", label: "评论" },
    defaultSize: { w: 3, h: 16 },
  }),
  defineWidget({
    id: "stat-attachments",
    label: "附件",
    icon: "ri:attachment-2",
    group: "statistics",
    component: StatCardWidget,
    props: { statKey: "attachment_total", label: "附件" },
    defaultSize: { w: 3, h: 16 },
  }),
  defineWidget({
    id: "stat-users",
    label: "用户",
    icon: "ri:user-3-line",
    group: "statistics",
    component: StatCardWidget,
    props: { statKey: "user_total", label: "用户" },
    defaultSize: { w: 3, h: 16 },
  }),
  defineWidget({
    id: "trend-chart",
    label: "访问趋势",
    icon: "ri:line-chart-line",
    group: "statistics",
    component: TrendChartWidget,
    props: {},
    defaultSize: { w: 6, h: 28 },
    defaultConfig: { defaultRange: "7", showUv: false },
    configFields: [
      {
        key: "defaultRange",
        label: "默认区间",
        type: "select",
        options: [
          { value: "7", label: "7 天" },
          { value: "30", label: "30 天" },
        ],
        description: "打开仪表盘时默认展示的趋势区间；仍可在卡片上临时切换。",
      },
      {
        key: "showUv",
        label: "默认显示独立访客",
        type: "boolean",
        description: "打开时同时展示 UV 虚线。",
      },
    ],
  }),
  defineWidget({
    id: "hourly-chart",
    label: "访问时段",
    icon: "ri:bar-chart-2-line",
    group: "statistics",
    component: HourlyChartWidget,
    props: {},
    defaultSize: { w: 6, h: 28 },
  }),
  defineWidget({
    id: "category-donut",
    label: "分类分布",
    icon: "ri:pie-chart-line",
    group: "content",
    component: CategoryDonutWidget,
    props: {},
    defaultSize: { w: 6, h: 28 },
  }),
  defineWidget({
    id: "recent-posts",
    label: "最近文章",
    icon: "ri:file-list-3-line",
    group: "content",
    component: RecentPostsWidget,
    props: {},
    defaultSize: { w: 6, h: 28 },
    defaultConfig: { itemLimit: "0" },
    configFields: [
      {
        key: "itemLimit",
        label: "显示条数",
        type: "select",
        options: [
          { value: "0", label: "按卡片大小" },
          { value: "3", label: "3 条" },
          { value: "5", label: "5 条" },
          { value: "8", label: "8 条" },
        ],
        description: "受档位高度限制，超出卡片的条数会被裁掉。",
      },
    ],
  }),
  defineWidget({
    id: "recent-comments",
    label: "最近评论",
    icon: "ri:chat-3-line",
    group: "content",
    component: RecentCommentsWidget,
    props: {},
    defaultSize: { w: 6, h: 28 },
    defaultConfig: { itemLimit: "0" },
    configFields: [
      {
        key: "itemLimit",
        label: "显示条数",
        type: "select",
        options: [
          { value: "0", label: "按卡片大小" },
          { value: "3", label: "3 条" },
          { value: "5", label: "5 条" },
          { value: "8", label: "8 条" },
        ],
        description: "受档位高度限制，超出卡片的条数会被裁掉。",
      },
    ],
  }),
  defineWidget({
    id: "quick-actions",
    label: "快捷操作",
    icon: "ri:flashlight-line",
    group: "tools",
    component: QuickActionsWidget,
    props: {},
    defaultSize: { w: 6, h: 28 },
    defaultConfig: { entryCount: "0" },
    configFields: [
      {
        key: "entryCount",
        label: "显示入口数",
        type: "select",
        options: [
          { value: "0", label: "按卡片大小" },
          { value: "2", label: "2 个" },
          { value: "4", label: "4 个" },
          { value: "6", label: "6 个" },
          { value: "8", label: "8 个" },
        ],
        description: "小档最多 2 个、中档最多 4 个；大档按此处的数量展示。",
      },
    ],
  }),
  defineWidget({
    id: "top-posts",
    label: "热门文章",
    icon: "ri:fire-line",
    group: "statistics",
    component: TopPostsWidget,
    props: {},
    defaultSize: { w: 6, h: 28 },
    defaultConfig: { itemLimit: "0" },
    configFields: [
      {
        key: "itemLimit",
        label: "显示条数",
        type: "select",
        options: [
          { value: "0", label: "按卡片大小" },
          { value: "3", label: "3 条" },
          { value: "5", label: "5 条" },
          { value: "8", label: "8 条" },
        ],
        description: "受档位高度限制，超出卡片的条数会被裁掉。",
      },
    ],
  }),
  defineWidget({
    id: "system-health",
    label: "系统状态",
    icon: "ri:heart-pulse-line",
    group: "system",
    component: SystemHealthWidget,
    props: {},
    defaultSize: { w: 6, h: 28 },
  }),
  defineWidget({
    id: "stellar-version",
    label: "Stellar 版本",
    icon: "ri:git-branch-line",
    group: "system",
    component: StellarVersionWidget,
    props: {},
    defaultSize: { w: 6, h: 28 },
  }),
  defineWidget({
    id: "system-version",
    label: "Blog 版本",
    icon: "ri:rocket-2-line",
    group: "system",
    component: SystemVersionWidget,
    props: {},
    defaultSize: { w: 6, h: 28 },
  }),
  defineWidget({
    id: "stack",
    label: "叠放容器",
    icon: "ri:stack-line",
    group: "tools",
    component: StackWidget,
    props: {},
    defaultSize: { w: 6, h: 28 },
    defaultConfig: { stackWidgets: ["recent-posts", "recent-comments", "top-posts"], interval: "0" },
    configFields: [
      {
        key: "stackWidgets",
        label: "容器内组件",
        type: "select",
        options: [
          { value: "recent-posts", label: "最近文章" },
          { value: "recent-comments", label: "最近评论" },
          { value: "top-posts", label: "热门文章" },
          { value: "recent-moments", label: "最近说说" },
          { value: "system-health", label: "系统状态" },
        ],
      },
      {
        key: "interval",
        label: "自动轮播间隔",
        type: "select",
        options: [
          { value: "0", label: "不轮播（手动切换）" },
          { value: "8", label: "8 秒" },
          { value: "15", label: "15 秒" },
        ],
        description: "自动切换只在仪表盘可见时运行。",
      },
    ],
  }),
];

export function getWidgetDefinition(id: string): WidgetDefinition | undefined {
  return widgetRegistry.find((widget) => widget.id === id);
}
