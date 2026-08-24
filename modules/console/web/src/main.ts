import { createApp } from "vue";
import { createPinia } from "pinia";
import { VueQueryPlugin, type VueQueryPluginOptions } from "@tanstack/vue-query";
import { addCollection } from "@iconify/vue";
import riIcons from "./icons/ri-subset.json";
import { queryClient } from "@/api/query-client";
import { configureApiTransport } from "@blog/api-client";
import { Dialog, Toast } from "@/components/ui";
import App from "./App.vue";
import router from "./router";
import "@fontsource/dm-sans/latin-400.css";
import "@fontsource/dm-sans/latin-500.css";
import "@fontsource/dm-sans/latin-600.css";
import "@fontsource/dm-sans/latin-700.css";
import "@fontsource/jetbrains-mono/latin-400.css";
import "@fontsource/jetbrains-mono/latin-600.css";
import "./styles/main.css";

let unauthorizedDialogOpen = false;

function handleUnauthorized() {
  // A burst of parallel queries can all receive 401 at once. Keep one dialog
  // and let the user choose when to leave the current page.
  if (router.currentRoute.value.path === "/login" || unauthorizedDialogOpen) return;
  unauthorizedDialogOpen = true;
  const redirect = router.currentRoute.value.fullPath;
  Dialog.warning({
    title: "登录已过期",
    description: "请重新登录后继续操作。",
    confirmText: "去登录",
    cancelText: "稍后",
    onConfirm: async () => {
      unauthorizedDialogOpen = false;
      await router.replace({ path: "/login", query: redirect !== "/login" ? { redirect } : {} });
    },
    onCancel: () => {
      unauthorizedDialogOpen = false;
    },
  });
}

configureApiTransport({
  onUnauthorized: handleUnauthorized,
  onError: (_error, message) => Toast.error(message),
});

addCollection(riIcons);

const app = createApp(App);
app.use(createPinia());
app.use(router);

// 注册 TanStack Query：统一管理服务端状态（缓存/重取/乐观更新）
const vueQueryOptions: VueQueryPluginOptions = { queryClient };
app.use(VueQueryPlugin, vueQueryOptions);

// Chart.js 注册已随 Spec 61 移入 src/lib/chart-init.ts（懒加载，仅 Dashboard/性能页调用）
app.mount("#app");
