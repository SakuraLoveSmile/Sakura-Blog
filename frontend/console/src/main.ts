import { createApp } from"vue";
import { createPinia } from"pinia";
import { VueQueryPlugin, type VueQueryPluginOptions } from"@tanstack/vue-query";
import { queryClient } from"@/api/query-client";
import App from"./App.vue";
import router from"./router";
import"./styles/main.css";
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
