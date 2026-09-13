import { createApp } from "vue";
import { createPinia } from "pinia";
import { VueQueryPlugin, type VueQueryPluginOptions } from "@tanstack/vue-query";
import { addCollection } from "@iconify/vue";
import riIcons from "../icons/ri-subset.json";
import { configureApiTransport } from "@blog/api-client";
import { permissionDirective } from "@/directives/permission";
import { i18n } from "@/i18n";
import { Toast } from "@/components/ui";
import { queryClient } from "@/api/query-client";
import { useUCAuthStore } from "./stores/auth";
import { useAuthStore } from "@/stores/auth";
import App from "./App.vue";
import router from "./router";
import "@fontsource/dm-sans/latin-400.css";
import "@fontsource/dm-sans/latin-500.css";
import "@fontsource/dm-sans/latin-600.css";
import "@fontsource/jetbrains-mono/latin-400.css";
import "../styles/main.css";

document.documentElement.setAttribute("data-app", "uc");

configureApiTransport({
  onUnauthorized: () => {
    useUCAuthStore().clearSession();
    useAuthStore().clearSession();
    queryClient.clear();
    if (router.currentRoute.value.path === "/login") return;
    Toast.warning("登录已过期，请重新登录。");
    void router.replace("/login");
  },
  onError: (_error, message) => Toast.error(message),
});

addCollection(riIcons);

const app = createApp(App);
app.use(createPinia());
app.use(router);
app.directive("permission", permissionDirective);
app.use(i18n);
app.use(VueQueryPlugin, { queryClient } satisfies VueQueryPluginOptions);
app.mount("#app");
