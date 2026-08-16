import { createRouter, createWebHistory } from "vue-router";
import { useAuthStore } from "@/stores/auth";

const router = createRouter({
  history: createWebHistory("/console/"),
  routes: [
    {
      path: "/login",
      name: "Login",
      component: () => import("@/modules/Login.vue"),
    },
    {
      path: "/",
      component: () => import("@/layouts/DefaultLayout.vue"),
      meta: { requiresAuth: true },
      children: [
        { path: "", redirect: "/dashboard" },
        {
          path: "dashboard",
          name: "Dashboard",
          component: () => import("@/modules/dashboard/DashboardView.vue"),
        },
        {
          path: "dashboard/widgets",
          redirect: "/dashboard",
        },
        {
          path: "posts",
          name: "Posts",
          component: () => import("@/modules/posts/PostList.vue"),
        },
        {
          path: "topics",
          name: "Topics",
          component: () => import("@/modules/topics/TopicsView.vue"),
        },
        {
          path: "posts/new",
          name: "PostNew",
          component: () => import("@/modules/posts/PostEditor.vue"),
          meta: { hideFooter: true },
        },
        {
          path: "posts/edit/:slug",
          name: "PostEdit",
          component: () => import("@/modules/posts/PostEditor.vue"),
          meta: { hideFooter: true },
        },
        {
          path: "posts/shares",
          name: "Shares",
          component: () => import("@/modules/shares/ShareList.vue"),
        },
        {
          path: "comments",
          name: "Comments",
          component: () => import("@/modules/comments/CommentList.vue"),
        },
        {
          path: "comment-users",
          name: "CommentUsers",
          component: () => import("@/modules/comments/CommentUsersView.vue"),
          meta: { requiresAuth: true, adminOnly: true },
        },
        {
          path: "attachments",
          name: "Attachments",
          component: () => import("@/modules/attachments/AttachmentList.vue"),
        },
        {
          path: "pages",
          name: "Pages",
          component: () => import("@/modules/pages/PageList.vue"),
        },
        {
          path: "pages/edit/:slug",
          name: "PageEdit",
          component: () => import("@/modules/pages/PageEditor.vue"),
          meta: { hideFooter: true },
        },
        {
          path: "wiki",
          name: "Wiki",
          component: () => import("@/modules/wiki/WikiDocsView.vue"),
          meta: { requiresAuth: true },
        },
        {
          path: "notebooks",
          name: "Notebooks",
          component: () => import("@/modules/notebooks/NotebooksView.vue"),
          meta: { requiresAuth: true },
        },
        {
          path: "moments",
          name: "Moments",
          component: () => import("@/modules/moments/MomentList.vue"),
        },
        {
          path: "taxonomy",
          name: "Taxonomy",
          component: () => import("@/modules/taxonomy/TaxonomyList.vue"),
        },
        {
          path: "friends",
          name: "Friends",
          component: () => import("@/modules/friends/FriendList.vue"),
        },
        {
          path: "subscriptions",
          name: "Subscriptions",
          component: () => import("@/modules/subscriptions/SubscriptionList.vue"),
        },
        {
          path: "plugins",
          name: "Plugins",
          meta: { adminOnly: true },
          component: () => import("@/modules/plugins/PluginLibrary.vue"),
        },
        {
          path: "modules",
          name: "Modules",
          meta: { adminOnly: true },
          component: () => import("@/modules/modules/ModulesView.vue"),
        },
        {
          path: "seo",
          component: () => import("@/modules/seo/SeoLayout.vue"),
          meta: { adminOnly: true },
          children: [
            { path: "", redirect: "/seo/overview" },
            { path: "overview", name: "SeoOverview", component: () => import("@/modules/seo/SeoOverview.vue") },
            { path: "analysis", name: "SeoAnalysis", component: () => import("@/modules/seo/SeoAnalysis.vue") },
            { path: "push", name: "SeoPush", component: () => import("@/modules/seo/SeoPush.vue") },
            { path: "crawlers", name: "SeoCrawlers", component: () => import("@/modules/seo/SeoCrawlers.vue") },
            { path: "traffic", name: "SeoTraffic", component: () => import("@/modules/seo/SeoTraffic.vue") },
            { path: "settings", name: "SeoSettings", component: () => import("@/modules/seo/SeoSettings.vue") },
          ],
        },
        {
          path: "perf",
          name: "Perf",
          meta: { adminOnly: true },
          component: () => import("@/modules/perf/PerfView.vue"),
        },
        {
          path: "settings",
          name: "Settings",
          meta: { adminOnly: true },
          component: () => import("@/modules/settings/SettingsView.vue"),
        },
        {
          path: "profile",
          name: "Profile",
          component: () => import("@/modules/profile/ProfileView.vue"),
          meta: { requiresAuth: true },
        },
        {
          path: "users",
          name: "Users",
          component: () => import("@/modules/users/UserListView.vue"),
          meta: { requiresAuth: true, adminOnly: true },
        },
        {
          path: "login-logs",
          name: "LoginLogs",
          component: () => import("@/modules/profile/LoginLogsView.vue"),
          meta: { requiresAuth: true, adminOnly: true },
        },
        {
          path: "notifications",
          name: "Notifications",
          component: () => import("@/modules/notifications/NotificationsView.vue"),
          meta: { requiresAuth: true },
        },
        {
          path: "backup",
          name: "Backup",
          meta: { adminOnly: true },
          component: () => import("@/modules/backup/BackupView.vue"),
        },
      ],
    },
  ],
});

// 路由守卫：未登录跳转 /login
router.beforeEach(async (to) => {
  const auth = useAuthStore();
  if (!auth.initialized) {
    await auth.checkAuth();
  }
  if (to.meta.requiresAuth && !auth.authenticated) {
    return { path: "/login", query: { from: to.fullPath } };
  }
  if (to.matched.some((r) => r.meta.adminOnly) && !auth.isAdmin) {
    return { path: "/dashboard" };
  }
  if (to.path === "/login" && auth.authenticated) {
    return { path: "/dashboard" };
  }
});

export default router;
