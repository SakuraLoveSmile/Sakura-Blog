import { createApp } from "vue";
import { createPinia } from "pinia";
import App from "./App.vue";
import router from "./router";
import { useThemeStore } from "./stores/theme";
import "./styles/main.css";
import {
  Chart,
  CategoryScale,
  LinearScale,
  BarController,
  LineController,
  BarElement,
  LineElement,
  PointElement, // Chart.js v4: 点元素
  Title,
  Tooltip,
  Legend
} from "chart.js";

const app = createApp(App);
app.use(createPinia());
app.use(router);

// 注册 Chart.js 必要组件
Chart.register(
  CategoryScale,
  LinearScale,
  BarController,
  LineController,
  BarElement,
  LineElement,
  PointElement, // Chart.js v4: 点元素
  Title,
  Tooltip,
  Legend
);

// 初始化主题：与 index.html 内联防闪烁脚本保持一致，同步 store 状态并应用 .dark 类。
useThemeStore().apply();

app.mount("#app");
