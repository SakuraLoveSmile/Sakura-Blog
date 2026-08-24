import type { RouteRecordRaw } from "vue-router";
import type { WidgetDefinition } from "./dashboard/widgets/registry";
import type { ModuleNav } from "@/stores/modules";

export interface ConsolePluginMeta {
  title: string;
  description?: string;
  permissions?: string[];
}

export interface ConsolePlugin {
  id: string;
  meta: ConsolePluginMeta;
  /** Routes mounted below the authenticated DefaultLayout shell. */
  routes?: RouteRecordRaw[];
  /** The core plugin owns the authenticated shell and its children. */
  shell?: RouteRecordRaw;
  /** Routes mounted outside the shell, such as the login page. */
  rootRoutes?: RouteRecordRaw[];
  nav?: ModuleNav[];
  widgets?: WidgetDefinition[];
}

export function definePlugin<T extends ConsolePlugin>(plugin: T): T {
  return plugin;
}
