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
export const moduleApi = {
  /** 全部模块 meta + 启停状态 */
  list: () => http.get("/modules"),
  /** 启用/禁用模块 */
  toggle: (id: string, enabled: boolean) => http.put(`/modules/${id}`, { enabled }),
};
