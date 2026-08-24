import { createApp } from "vue";
import { createPinia } from "pinia";
import { VueQueryPlugin, type VueQueryPluginOptions } from "@tanstack/vue-query";
import { addCollection } from "@iconify/vue";
import riIcons from "../icons/ri-subset.json";
import { configureApiTransport } from "@blog/api-client";
import { Toast } from "@/components/ui";
import { queryClient } from "@/api/query-client";
import App from "./App.vue";
import router from "./router";
import "@fontsource/dm-sans/latin-400.css";
import "@fontsource/dm-sans/latin-500.css";
import "@fontsource/dm-sans/latin-600.css";
import "@fontsource/jetbrains-mono/latin-400.css";
import "../styles/main.css";

configureApiTransport({
  onUnauthorized: () => router.replace("/login"),
  onError: (_error, message) => Toast.error(message),
});

addCollection(riIcons);

const app = createApp(App);
app.use(createPinia());
app.use(router);
app.use(VueQueryPlugin, { queryClient } satisfies VueQueryPluginOptions);
app.mount("#app");
