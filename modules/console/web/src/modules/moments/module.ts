import { defineAsyncComponent } from "vue";
import { definePlugin } from "../definePlugin";

const RecentMomentsWidget = defineAsyncComponent(() => import("../dashboard/widgets/RecentMomentsWidget.vue"));

export default definePlugin({
  id: "moments",
  meta: {
    title: "瞬间",
    description: "发布和管理短内容动态",
  },
  nav: [{ path: "/moments", label: "瞬间", icon: "ri:quill-pen-line", group: "内容" }],
  routes: [{ path: "moments", name: "Moments", component: () => import("./MomentList.vue") }],
  widgets: [
    {
      id: "recent-moments",
      label: "最近说说",
      icon: "ri:quill-pen-line",
      group: "content",
      component: RecentMomentsWidget,
      props: {},
      defaultSize: { w: 6, h: 34 },
      defaultConfig: {},
    },
  ],
});
