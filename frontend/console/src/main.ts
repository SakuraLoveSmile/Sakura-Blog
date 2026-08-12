import { createApp } from"vue";
import { createPinia } from"pinia";
import { VueQueryPlugin, type VueQueryPluginOptions } from"@tanstack/vue-query";
import { addCollection } from"@iconify/vue";
import riIcons from"./icons/ri-subset.json";
import { queryClient } from"@/api/query-client";
import App from"./App.vue";
import router from"./router";
import "@fontsource/dm-sans/latin-400.css";
import "@fontsource/dm-sans/latin-500.css";
import "@fontsource/dm-sans/latin-600.css";
import "@fontsource/dm-sans/latin-700.css";
import "@fontsource/jetbrains-mono/latin-400.css";
import "@fontsource/jetbrains-mono/latin-500.css";
import "@fontsource/jetbrains-mono/latin-600.css";
import "./styles/main.css";

addCollection(riIcons);

const app = createApp(App);
app.use(createPinia());
app.use(router);

// 注册 TanStack Query：统一管理服务端状态（缓存/重取/乐观更新）
const vueQueryOptions: VueQueryPluginOptions = { queryClient };
app.use(VueQueryPlugin, vueQueryOptions);

// Chart.js 注册已随 Spec 61 移入 src/lib/chart-init.ts（懒加载，仅 Dashboard/性能页调用）
app.mount("#app");
