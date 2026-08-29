import {
  PLUGIN_CATEGORIES,
  type PluginCategory,
  type PluginFormField,
  type PluginMeta,
  type PluginType,
} from "./plugin-types";

export interface PluginCatalogParam {
  key: string;
  label?: string;
  label_key?: string;
  hint?: string;
  hint_key?: string;
  type: "text" | "select" | "number" | "switch" | "color" | "textarea";
  role: "positional" | "named" | "flag" | "content";
  default?: unknown;
  options?: string[];
  placeholder?: string;
}

export interface PluginCatalogItem {
  name: string;
  title: string;
  description: string;
  signature: string;
  example: string;
  type: "inline" | "block" | "hybrid";
  category: string;
  capability: "static" | "interactive" | "service" | "external";
  services?: string[];
  fixture?: string;
  main_js?: boolean;
  height?: number;
  fallback?: string;
  params: PluginCatalogParam[];
  param_docs: Array<{ name: string; desc: string; default?: string }>;
  edit?: {
    enabled: boolean;
    required?: string[];
    required_any?: string[][];
    legacy_args?: string[];
  };
}

function categoryOf(value: string): PluginCategory {
  return PLUGIN_CATEGORIES.includes(value as PluginCategory) ? (value as PluginCategory) : "布局容器";
}

// hybrid 必须原样保留：navbar/frame 有单行与块级两种写法，映射成 inline 会丢失块级包装。
function typeOf(value: string): PluginType {
  return value === "block" || value === "hybrid" ? value : "inline";
}

function stringDefault(value: unknown): string | undefined {
  if (value === undefined || value === null) return undefined;
  if (typeof value === "boolean") return value ? "true" : "false";
  return String(value);
}

function formTypeOf(type: PluginCatalogParam["type"]): PluginFormField["type"] {
  return type === "switch" ? "flag" : type;
}

function mergeForm(existing: PluginMeta | undefined, params: PluginCatalogParam[]): PluginFormField[] | undefined {
  if (!params.length) return existing?.form;
  return params.map((param) => {
    const old = existing?.form?.find((field) => field.key === param.key);
    return {
      key: param.key,
      label: param.label || old?.label || param.key,
      label_key: param.label_key || old?.label_key,
      type: formTypeOf(param.type),
      role: param.role,
      options: param.options?.slice(),
      default: stringDefault(param.default),
      placeholder: old?.placeholder || param.placeholder,
      hint: param.hint || old?.hint,
      hint_key: param.hint_key || old?.hint_key,
    };
  });
}

function fallbackExample(item: PluginCatalogItem): string {
  return item.type === "block" ? `{% ${item.name} %}\n{% end${item.name} %}` : `{% ${item.name} %}`;
}

/** Rebuild the UI list from the backend catalog while preserving display copy. */
export function mergePluginCatalog(base: PluginMeta[], catalog: PluginCatalogItem[]): PluginMeta[] {
  const baseByName = new Map(base.map((plugin) => [plugin.name, plugin]));
  return catalog.map((item) => {
    const existing = baseByName.get(item.name);
    return {
      name: item.name,
      title: item.title || existing?.title || item.name,
      category: categoryOf(item.category),
      type: existing?.type || typeOf(item.type),
      desc: item.description || existing?.desc || "由后端 shortcode catalog 提供。",
      signature: item.signature || existing?.signature || fallbackExample(item),
      params:
        item.param_docs?.map((param) => ({
          name: param.name,
          desc: param.desc,
          ...(param.default ? { default: param.default } : {}),
        })) ||
        existing?.params ||
        [],
      example: item.example || existing?.example || fallbackExample(item),
      form: mergeForm(existing, item.params),
      preview: {
        capability: item.capability,
        services: item.services?.slice(),
        fixture: item.fixture,
        mainJs: item.main_js,
        height: item.height,
        fallback: item.fallback,
      },
    };
  });
}
