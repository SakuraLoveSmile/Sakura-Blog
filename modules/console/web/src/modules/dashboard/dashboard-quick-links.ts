export interface DashboardNavItem {
  path: string;
  permissions?: string[];
  children?: DashboardNavItem[];
}

export interface DashboardQuickLink {
  label: string;
  path: string;
}

function allowed(item: DashboardNavItem, hasPermission: (permission: string) => boolean): boolean {
  return !item.permissions?.length || item.permissions.some((permission) => hasPermission(permission));
}

function navAllowed(nav: DashboardNavItem[], path: string, hasPermission: (permission: string) => boolean): boolean {
  return nav.some((item) => {
    if (item.path === path) return allowed(item, hasPermission);
    return item.children?.some((child) => child.path === path && allowed(child, hasPermission)) ?? false;
  });
}

/**
 * 仪表盘快捷入口与侧栏使用同一套 manifest + 权限真值，避免普通用户看到
 * 自己无权访问的设置/评论/分类入口。
 */
export function buildDashboardQuickLinks(
  nav: DashboardNavItem[],
  hasPermission: (permission: string) => boolean,
): DashboardQuickLink[] {
  const canWritePost = navAllowed(nav, "/posts", hasPermission) && hasPermission("content:write");
  return [
    canWritePost ? { label: "写文章", path: "/posts/new" } : null,
    navAllowed(nav, "/comments", hasPermission) ? { label: "评论", path: "/comments" } : null,
    navAllowed(nav, "/taxonomy", hasPermission) ? { label: "分类", path: "/taxonomy" } : null,
    navAllowed(nav, "/settings", hasPermission) ? { label: "设置", path: "/settings" } : null,
  ].filter((item): item is DashboardQuickLink => Boolean(item));
}
