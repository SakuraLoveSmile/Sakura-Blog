// Chart.js 懒加载入口（Spec 61 阶段 1）：chart.js 全家桶不再进入口 chunk，
// 仅 Dashboard 时段分布图 / 性能页趋势图通过 loadChart() 动态加载，vite 自动分包。
// 注册幂等：多个组件并发调用共享同一 Promise，注册只执行一次。
let chartPromise: Promise<typeof import("chart.js")> | null = null;
let chartRegistered = false;

export function loadChart(): Promise<typeof import("chart.js")> {
  if (!chartPromise) {
    chartPromise = import("chart.js").then((mod) => {
      if (!chartRegistered) {
        mod.Chart.register(
          mod.CategoryScale,
          mod.LinearScale,
          mod.BarController,
          mod.LineController,
          mod.BarElement,
          mod.LineElement,
          mod.PointElement, // Chart.js v4: 点元素
          mod.Filler, // Chart.js v4: 面积填充插件（line 数据集 fill:true 依赖）
          mod.Title,
          mod.Tooltip,
          mod.Legend
        );
        chartRegistered = true;
      }
      return mod;
    });
  }
  return chartPromise;
}
