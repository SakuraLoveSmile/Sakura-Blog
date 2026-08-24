import { createRouter, createWebHistory, type RouteRecordRaw } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { useModulesStore } from "@/stores/modules";
import { consolePlugins, getConsolePlugin } from "@/plugin-registry";

function attachModuleMeta(route: RouteRecordRaw, moduleId: string, defaultPermissions: string[] = []): RouteRecordRaw {
  const { children, ...rest } = route;
  const meta = { ...route.meta, moduleId } as Record<string, unknown>;
  if (!Array.isArray(meta.permissions) && defaultPermissions.length > 0) meta.permissions = defaultPermissions;
  return (
    children
      ? {
          ...rest,
          meta,
          children: children.map((child) => attachModuleMeta(child, moduleId, defaultPermissions)),
        }
      : { ...rest, meta }
  ) as RouteRecordRaw;
}

function buildRoutes(): RouteRecordRaw[] {
  const core = getConsolePlugin("core");
  if (!core?.shell) throw new Error("Console core plugin must provide the DefaultLayout shell");

  const rootRoutes = consolePlugins.flatMap((plugin) =>
    (plugin.rootRoutes ?? []).map((route) => attachModuleMeta(route, plugin.id, plugin.meta.permissions ?? [])),
  );
  const moduleChildren = consolePlugins
    .filter((plugin) => plugin.id !== "core")
    .flatMap((plugin) =>
      (plugin.routes ?? []).map((route) => attachModuleMeta(route, plugin.id, plugin.meta.permissions ?? [])),
    );
  const shell = attachModuleMeta(
    { ...core.shell, children: [...(core.shell.children ?? []), ...moduleChildren] },
    core.id,
  );
  return [...rootRoutes, shell];
}

const router = createRouter({
  history: createWebHistory("/console/"),
  routes: buildRoutes(),
});

function permissionsFor(to: { matched: readonly { meta: Record<string, unknown> }[] }): string[] {
  return to.matched.flatMap((record) =>
    Array.isArray(record.meta.permissions) ? (record.meta.permissions as string[]) : [],
  );
}

// 路由守卫：认证、后端 manifest 模块启停、角色均在进入页面前统一判定。
router.beforeEach(async (to) => {
  const auth = useAuthStore();
  const modules = useModulesStore();
  if (!auth.initialized) await auth.checkAuth();

  if (to.meta.requiresAuth && !auth.authenticated) {
    return { path: "/login", query: { from: to.fullPath } };
  }
  if (auth.authenticated) {
    if (!auth.hasPermission("console:access")) {
      // Users without the console permission work in the User Center. Use a
      // full navigation because /uc is a separate Vite entry.
      window.location.assign("/uc/");
      return false;
    }
    if (!modules.loaded) await modules.fetch();
    const moduleId = to.matched
      .map((record) => record.meta.moduleId)
      .find((value): value is string => typeof value === "string");
    if (moduleId && !modules.isEnabled(moduleId)) return { path: "/dashboard" };

    const permissions = permissionsFor(to);
    if (permissions.length > 0 && !permissions.some((permission) => auth.hasPermission(permission))) {
      return { path: "/dashboard" };
    }
  }
  if (to.path === "/login" && auth.authenticated) return { path: "/dashboard" };
});

export default router;
