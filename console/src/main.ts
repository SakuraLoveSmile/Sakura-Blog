import { createApp } from "vue";
import { createPinia } from "pinia";
import App from "./App.vue";
import router from "./router";
import { useThemeStore } from "./stores/theme";
import "./styles/main.css";

const app = createApp(App);
app.use(createPinia());
app.use(router);

// 初始化主题：与 index.html 内联防闪烁脚本保持一致，同步 store 状态并应用 .dark 类。
useThemeStore().apply();

app.mount("#app");
