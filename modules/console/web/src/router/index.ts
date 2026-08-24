import { createRouter, createWebHistory, type RouteRecordRaw } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { useModulesStore } from "@/stores/modules";
import { consolePlugins, getConsolePlugin } from "@/plugin-registry";

function attachModuleMeta(route: RouteRecordRaw, moduleId: string): RouteRecordRaw {
  const { children, ...rest } = route;
  return (
    children
      ? {
          ...rest,
          meta: { ...route.meta, moduleId },
          children: children.map((child) => attachModuleMeta(child, moduleId)),
        }
      : { ...rest, meta: { ...route.meta, moduleId } }
  ) as RouteRecordRaw;
}

function buildRoutes(): RouteRecordRaw[] {
  const core = getConsolePlugin("core");
  if (!core?.shell) throw new Error("Console core plugin must provide the DefaultLayout shell");

  const rootRoutes = consolePlugins.flatMap((plugin) =>
    (plugin.rootRoutes ?? []).map((route) => attachModuleMeta(route, plugin.id)),
  );
  const moduleChildren = consolePlugins
    .filter((plugin) => plugin.id !== "core")
    .flatMap((plugin) => (plugin.routes ?? []).map((route) => attachModuleMeta(route, plugin.id)));
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

function rolesFor(to: { matched: readonly { meta: Record<string, unknown> }[] }): string[] {
  return to.matched.flatMap((record) => (Array.isArray(record.meta.roles) ? (record.meta.roles as string[]) : []));
}

function hasRole(roles: string[], auth: ReturnType<typeof useAuthStore>): boolean {
  const current = auth.isAdmin ? "admin" : auth.user.role || auth.role;
  return roles.includes(current) || (auth.isAdmin && roles.includes("author"));
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
    if (!modules.loaded) await modules.fetch();
    const moduleId = to.matched
      .map((record) => record.meta.moduleId)
      .find((value): value is string => typeof value === "string");
    if (moduleId && !modules.isEnabled(moduleId)) return { path: "/dashboard" };

    const roles = rolesFor(to);
    if (roles.length > 0 && !hasRole(roles, auth)) {
      return { path: "/dashboard" };
    }
  }
  if (to.path === "/login" && auth.authenticated) return { path: "/dashboard" };
});

export default router;
