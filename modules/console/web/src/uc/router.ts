import { createRouter, createWebHistory, type RouteRecordRaw } from "vue-router";
import { useUCAuthStore } from "./stores/auth";
import UCLayout from "./UCLayout.vue";
import UCLoginView from "./UCLoginView.vue";
import UCRegisterView from "./UCRegisterView.vue";
import UCUpgradeView from "./UCUpgradeView.vue";
import UCPostsView from "./UCPostsView.vue";
import UCPostEditorView from "./UCPostEditorView.vue";
import UCAttachmentsView from "./UCAttachmentsView.vue";

const routes: RouteRecordRaw[] = [
  { path: "/login", component: UCLoginView },
  { path: "/register", component: UCRegisterView },
  { path: "/upgrade", component: UCUpgradeView, meta: { requiresAuth: true } },
  {
    path: "/",
    component: UCLayout,
    meta: { requiresAuth: true },
    children: [
      { path: "", redirect: "/posts" },
      { path: "posts", component: UCPostsView, meta: { requiresAuthor: true } },
      { path: "posts/new", component: UCPostEditorView, meta: { requiresAuthor: true } },
      { path: "posts/:slug", component: UCPostEditorView, meta: { requiresAuthor: true } },
      { path: "attachments", component: UCAttachmentsView, meta: { requiresAuthor: true } },
      { path: "profile", component: () => import("@/modules/profile/ProfileView.vue") },
      { path: "notifications", component: () => import("@/modules/notifications/NotificationsView.vue") },
    ],
  },
];

const router = createRouter({ history: createWebHistory("/uc/"), routes });

router.beforeEach(async (to) => {
  const auth = useUCAuthStore();
  if (!auth.initialized) await auth.fetchMe();
  if (to.meta.requiresAuth && !auth.user) return { path: "/login", query: { from: to.fullPath } };
  if (to.meta.requiresAuthor && auth.user?.role !== "author" && auth.user?.role !== "admin")
    return { path: "/upgrade" };
  if ((to.path === "/login" || to.path === "/register") && auth.user) {
    return { path: auth.user.role === "reader" ? "/upgrade" : "/posts" };
  }
});

export default router;
