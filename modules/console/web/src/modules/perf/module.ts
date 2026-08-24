import { definePlugin } from "../definePlugin";

const admin = ["admin"];

export default definePlugin({
  id: "perf",
  meta: { title: "性能监控", description: "服务端运行时指标与请求耗时统计", roles: admin },
  nav: [{ path: "/perf", label: "性能监控", icon: "ri:speed-line", group: "系统", roles: admin }],
  routes: [
    {
      path: "perf",
      name: "Perf",
      meta: { roles: admin },
      component: () => import("./PerfView.vue"),
    },
  ],
});
