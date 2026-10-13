export type PluginCategory = "提示标注" | "布局容器" | "文艺装饰" | "媒体播放" | "数据图表" | "GitHub";
// hybrid：同时存在单行与块级两种写法（navbar / frame），需按 variant 决定拼装方式。
export type PluginType = "inline" | "block" | "hybrid";
export type PluginVariant = "inline" | "block";

export const PLUGIN_CATEGORIES: PluginCategory[] = [
  "提示标注",
  "布局容器",
  "文艺装饰",
  "媒体播放",
  "数据图表",
  "GitHub",
];

export interface PluginParam {
  name: string;
  desc: string;
  default?: string;
}

export interface PluginFormField {
  named_alias?: boolean;
  key: string;
  label: string;
  label_key?: string;
  type: "text" | "select" | "number" | "color" | "textarea" | "flag";
  role: "positional" | "named" | "content" | "flag";
  options?: string[];
  default?: string;
  placeholder?: string;
  hint?: string;
  hint_key?: string;
}

export type PluginCapability = "static" | "interactive" | "service" | "external";

export interface PluginPreviewInfo {
  capability: PluginCapability;
  services?: string[];
  plugins?: string[];
  fixture?: string;
  mainJs?: boolean;
  height?: number;
  fallback?: string;
}

export interface PluginEditSchema {
  enabled: boolean;
  required?: string[];
  required_any?: string[][];
  legacy_args?: string[];
}

export interface PluginMeta {
  name: string;
  title: string;
  category: PluginCategory;
  type: PluginType;
  desc: string;
  signature: string;
  params: PluginParam[];
  example: string;
  form?: PluginFormField[];
  edit?: PluginEditSchema;
  preview?: PluginPreviewInfo;
}

export function previewInfoFor(plugin: { preview?: PluginPreviewInfo }): PluginPreviewInfo {
  return plugin.preview || { capability: "static" };
}
