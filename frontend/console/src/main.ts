import { createApp } from"vue";
import { createPinia } from"pinia";
import { VueQueryPlugin, type VueQueryPluginOptions } from"@tanstack/vue-query";
import { addCollection } from"@iconify/vue";
import riIcons from"@iconify/json/json/ri.json";
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
import {
 Chart,
 CategoryScale,
 LinearScale,
 BarController,
 LineController,
 BarElement,
 LineElement,
 PointElement, // Chart.js v4: 点元素
 Filler, // Chart.js v4: 面积填充插件（line 数据集 fill:true 依赖）
 Title,
 Tooltip,
 Legend,
} from"chart.js";

const app = createApp(App);
app.use(createPinia());
app.use(router);

// 注册 TanStack Query：统一管理服务端状态（缓存/重取/乐观更新）
const vueQueryOptions: VueQueryPluginOptions = { queryClient };
app.use(VueQueryPlugin, vueQueryOptions);

// 注册 Chart.js 必要组件
Chart.register(
 CategoryScale,
 LinearScale,
 BarController,
 LineController,
 BarElement,
 LineElement,
 PointElement, // Chart.js v4: 点元素
 Filler, // Chart.js v4: 面积填充插件（趋势图 PV fill:true 依赖）
 Title,
 Tooltip,
 Legend
);

app.mount("#app");
