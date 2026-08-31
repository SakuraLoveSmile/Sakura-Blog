// Mock Fixture 数据管理类型定义（Spec 63 + Spec 64）
export type FixtureFormat = "json" | "markdown";
export type FixtureState = "normal" | "empty" | "error";

export interface FixtureItem {
  id: string;
  title: string;
  category: "数据服务" | "交互组件" | "外部文档";
  endpoint: string;
  format: FixtureFormat;
  description: string;
  icon: string;
  previewShortcode: string;
  previewHeight: number;
  defaultData: string;
  sampleShortcodes: string[];
}

export interface FixtureOverrideConfig {
  endpoint: string;
  format: FixtureFormat;
  state: FixtureState;
  customData?: string;
}
