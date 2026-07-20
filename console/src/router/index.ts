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
          path: "posts",
          name: "Posts",
          component: () => import("@/modules/posts/PostList.vue"),
        },
        {
          path: "posts/new",
          name: "PostNew",
          component: () => import("@/modules/posts/PostEditor.vue"),
        },
        {
          path: "posts/edit/:slug",
          name: "PostEdit",
          component: () => import("@/modules/posts/PostEditor.vue"),
        },
        {
          path: "comments",
          name: "Comments",
          component: () => import("@/modules/comments/CommentList.vue"),
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
          path: "settings",
          name: "Settings",
          component: () => import("@/modules/settings/SettingsView.vue"),
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
  if (to.path === "/login" && auth.authenticated) {
    return { path: "/dashboard" };
  }
});

export default router;
