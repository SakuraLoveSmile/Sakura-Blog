import { definePlugin } from "../definePlugin";

const admin = ["admin"];

export default definePlugin({
  id: "seo",
  meta: { title: "SEO 工具集", description: "站点体检、搜索引擎推送、爬虫与搜索流量分析", roles: admin },
  nav: [{ path: "/seo", label: "SEO 工具集", icon: "ri:search-eye-line", group: "系统", roles: admin }],
  routes: [
    {
      path: "seo",
      component: () => import("./SeoLayout.vue"),
      meta: { roles: admin },
      children: [
        { path: "", redirect: "/seo/overview" },
        { path: "overview", name: "SeoOverview", component: () => import("./SeoOverview.vue") },
        { path: "analysis", name: "SeoAnalysis", component: () => import("./SeoAnalysis.vue") },
        { path: "push", name: "SeoPush", component: () => import("./SeoPush.vue") },
        { path: "crawlers", name: "SeoCrawlers", component: () => import("./SeoCrawlers.vue") },
        { path: "traffic", name: "SeoTraffic", component: () => import("./SeoTraffic.vue") },
        { path: "settings", name: "SeoSettings", component: () => import("./SeoSettings.vue") },
      ],
    },
  ],
});
