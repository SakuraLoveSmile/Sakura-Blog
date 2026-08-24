import { http } from "../transport";

// --- Shortcode catalog（后端 parser 注册表真值） ---
export interface ShortcodeCatalogParam {
  key: string;
  label?: string;
  type: "text" | "select" | "number" | "switch" | "color" | "textarea";
  role: "positional" | "named" | "flag" | "content";
  default?: unknown;
  options?: string[];
  placeholder?: string;
}

export interface ShortcodeCatalogDoc {
  name: string;
  desc: string;
  default?: string;
}

export interface ShortcodeCatalogItem {
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
  params: ShortcodeCatalogParam[];
  param_docs: ShortcodeCatalogDoc[];
}

export const pluginApi = {
  catalog: () => http.get<{ items: ShortcodeCatalogItem[] }>("/plugins/catalog"),
};

// --- 功能模块（manifest + 启停） ---
export interface ModuleNav {
  path: string;
  label: string;
  icon: string;
  group: string;
  roles?: string[];
  mobile?: boolean;
  children?: ModuleNav[];
}

export interface ModuleManifestItem {
  id: string;
  title: string;
  description: string;
  category: string;
  nav: ModuleNav[];
  enabled: boolean;
  locked?: boolean;
  config_section?: string;
  config_schema?: ModuleConfigField[];
}

export interface ModuleConfigField {
  key: string;
  type: "text" | "password" | "select" | "number" | "switch" | "color" | "textarea";
  label?: string;
  default?: unknown;
  options?: string[];
  placeholder?: string;
  hint?: string;
  rows?: number;
}

export const moduleApi = {
  /** 全部模块 meta + 启停状态 */
  list: () => http.get<{ items: ModuleManifestItem[] }>("/modules"),
  /** 启用/禁用模块 */
  toggle: (id: string, enabled: boolean) => http.put<{ ok: boolean; enabled: boolean }>(`/modules/${id}`, { enabled }),
  getConfig: (section: string) => http.get<Record<string, unknown>>(`/settings/${section}`),
  putConfig: (section: string, value: Record<string, unknown>) =>
    http.put<{ ok: boolean }>(`/settings/${section}`, value),
};
