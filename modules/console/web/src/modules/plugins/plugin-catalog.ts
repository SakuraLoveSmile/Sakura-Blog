import {
  PLUGIN_CATEGORIES,
  type PluginCategory,
  type PluginFormField,
  type PluginMeta,
} from "./plugins-data";

export interface PluginCatalogParam {
  key: string;
  type: "text" | "select" | "number" | "switch" | "color" | "textarea";
  role: "positional" | "named" | "flag" | "content";
  default?: unknown;
  options?: string[];
  placeholder?: string;
}

export interface PluginCatalogItem {
  name: string;
  type: "inline" | "block" | "hybrid";
  category: string;
  capability: "static" | "interactive" | "service" | "external";
  params: PluginCatalogParam[];
}

function categoryOf(value: string): PluginCategory {
  return PLUGIN_CATEGORIES.includes(value as PluginCategory)
    ? (value as PluginCategory)
    : "布局容器";
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
      label: old?.label || param.key,
      type: formTypeOf(param.type),
      role: param.role,
      options: param.options?.slice(),
      default: stringDefault(param.default),
      placeholder: old?.placeholder || param.placeholder,
    };
  });
}

function fallbackExample(item: PluginCatalogItem): string {
  return item.type === "block"
    ? `{% ${item.name} %}\n{% end${item.name} %}`
    : `{% ${item.name} %}`;
}

/** Rebuild the UI list from the backend catalog while preserving display copy. */
export function mergePluginCatalog(base: PluginMeta[], catalog: PluginCatalogItem[]): PluginMeta[] {
  const baseByName = new Map(base.map((plugin) => [plugin.name, plugin]));
  return catalog.map((item) => {
    const existing = baseByName.get(item.name);
    return {
      name: item.name,
      title: existing?.title || item.name,
      category: categoryOf(item.category),
      type: existing?.type || (item.type === "block" ? "block" : "inline"),
      desc: existing?.desc || "由后端 shortcode catalog 提供。",
      signature: existing?.signature || fallbackExample(item),
      params: existing?.params || [],
      example: existing?.example || fallbackExample(item),
      form: mergeForm(existing, item.params),
    };
  });
}
