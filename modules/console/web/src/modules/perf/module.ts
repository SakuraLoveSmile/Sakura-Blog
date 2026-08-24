import { definePlugin } from "../definePlugin";

const admin = ["system:manage"];

export default definePlugin({
  id: "perf",
  meta: { title: "性能监控", description: "服务端运行时指标与请求耗时统计", permissions: admin },
  nav: [{ path: "/perf", label: "性能监控", icon: "ri:speed-line", group: "系统", permissions: admin }],
  routes: [
    {
      path: "perf",
      name: "Perf",
      meta: { permissions: admin },
      component: () => import("./PerfView.vue"),
    },
  ],
});
