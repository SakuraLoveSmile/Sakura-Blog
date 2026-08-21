<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from "vue";
import { Icon } from "@iconify/vue";
import type { Chart } from "chart.js";
import { loadChart } from "@/lib/chart-init";
import { VPageHeader, VCard, VLoading, VEmpty, VButton } from "@/components/ui";
import {
  perfApi,
  type PerfOverview,
  type PerfSamplePoint,
  type PerfRouteStat,
  type PerfMinuteStat,
} from "@/api";

const loading = ref(true);
const disabled = ref(false);
const hours = ref(24);

const overview = ref<PerfOverview | null>(null);
const samples = ref<PerfSamplePoint[]>([]);
const routes = ref<PerfRouteStat[]>([]);
const throughput = ref<PerfMinuteStat[]>([]);

const memCanvas = ref<HTMLCanvasElement | null>(null);
const tpCanvas = ref<HTMLCanvasElement | null>(null);
let memChart: Chart | null = null;
let tpChart: Chart | null = null;

async function load() {
  loading.value = true;
  disabled.value = false;
  try {
    const [ov, his, req] = await Promise.all([
      perfApi.overview(),
      perfApi.history(hours.value),
      perfApi.requests(hours.value),
    ]);
    overview.value = ov.data;
    samples.value = his.data.points || [];
    routes.value = req.data.routes || [];
    throughput.value = req.data.throughput || [];
    requestAnimationFrame(async () => {
      await initMemChart();
      await initTpChart();
    });
  } catch (e: any) {
    if (e?.response?.status === 403) disabled.value = true;
  } finally {
    loading.value = false;
  }
}

watch(hours, load);
onMounted(load);
onUnmounted(() => {
  memChart?.destroy();
  tpChart?.destroy();
  memChart = null;
  tpChart = null;
});

function fmtUptime(sec: number): string {
  const d = Math.floor(sec / 86400);
  const h = Math.floor((sec % 86400) / 3600);
  const m = Math.floor((sec % 3600) / 60);
  if (d > 0) return `${d} 天 ${h} 小时`;
  if (h > 0) return `${h} 小时 ${m} 分`;
  return `${m} 分钟`;
}

function fmtBytes(n: number): string {
  if (n <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  let i = 0;
  let v = n;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

function fmtMs(ms: number): string {
  if (ms >= 1000) return `${(ms / 1000).toFixed(2)} s`;
  return `${ms.toFixed(1)} ms`;
}

const fmtPct = (r: number) => `${(r * 100).toFixed(2)}%`;
const shortTs = (ts: string) => ts.slice(11, 16);
const shortMinute = (m: string) => (m.length > 10 ? m.slice(5, 16) : m.slice(11, 16));

function chartColors() {
  const cs = getComputedStyle(document.documentElement);
  return {
    c1: cs.getPropertyValue("--color-primary").trim() || "#2E8DFF",
    primary: cs.getPropertyValue("--color-primary").trim() || "#2E8DFF",
    grid: "color-mix(in srgb, var(--color-border) 40%, transparent)",
    muted: cs.getPropertyValue("--color-text-quaternary").trim() || "#8e8e93",
    card: cs.getPropertyValue("--color-card").trim() || "#1c1c1e",
    fg: cs.getPropertyValue("--color-foreground").trim() || "#f5f5f7",
    border: cs.getPropertyValue("--color-border").trim() || "#3a3a3c",
  };
}

function tooltipOpts(cc: ReturnType<typeof chartColors>) {
  return {
    backgroundColor: cc.card,
    titleColor: cc.fg,
    bodyColor: cc.muted,
    borderColor: cc.border,
    borderWidth: 1,
    padding: 10,
    cornerRadius: 8,
  };
}

async function initMemChart() {
  const canvas = memCanvas.value;
  if (!canvas) return;
  const { Chart } = await loadChart();
  memChart?.destroy();
  const cc = chartColors();
  const heap = samples.value.filter((p) => p.metric === "heap_alloc_mb");
  const sys = samples.value.filter((p) => p.metric === "sys_mem_mb");
  const gors = samples.value.filter((p) => p.metric === "goroutines");
  const gorColor = "#30D158";

  memChart = new Chart(canvas, {
    type: "line",
    data: {
      labels: heap.map((p) => shortTs(p.ts)),
      datasets: [
        {
          label: "堆内存 (MB)",
          data: heap.map((p) => Number(p.value.toFixed(2))),
          borderColor: cc.primary,
          backgroundColor: "rgba(0, 122, 255, 0.15)",
          fill: true,
          pointRadius: 0,
          tension: 0.3,
          borderWidth: 1.5,
        },
        {
          label: "系统内存 Sys (MB)",
          data: sys.map((p) => Number(p.value.toFixed(2))),
          borderColor: "#AF52DE",
          pointRadius: 0,
          tension: 0.3,
          borderWidth: 1.5,
          borderDash: [4, 3],
        },
        {
          label: "Goroutines 协程",
          data: gors.map((p) => Math.round(p.value)),
          borderColor: gorColor,
          yAxisID: "y1",
          pointRadius: 0,
          tension: 0.3,
          borderWidth: 1.5,
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: { duration: 0 },
      plugins: {
        legend: { labels: { color: cc.muted, boxWidth: 12, font: { size: 11 } } },
        tooltip: tooltipOpts(cc),
      },
      scales: {
        x: {
          grid: { display: false },
          border: { color: cc.grid },
          ticks: { color: cc.muted, font: { size: 10 }, maxTicksLimit: 8 },
        },
        y: {
          beginAtZero: true,
          grid: { color: cc.grid },
          border: { display: false },
          ticks: { color: cc.muted, font: { size: 10 } },
        },
        y1: {
          position: "right",
          beginAtZero: true,
          grid: { drawOnChartArea: false },
          border: { display: false },
          ticks: { color: cc.muted, font: { size: 10 }, precision: 0 },
        },
      },
    },
  });
}

async function initTpChart() {
  const canvas = tpCanvas.value;
  if (!canvas) return;
  const { Chart } = await loadChart();
  tpChart?.destroy();
  const cc = chartColors();

  tpChart = new Chart(canvas, {
    type: "bar",
    data: {
      labels: throughput.value.map((p) => shortMinute(p.minute)),
      datasets: [
        {
          label: "请求吞吐量",
          data: throughput.value.map((p) => p.count),
          backgroundColor: cc.c1,
          borderRadius: 3,
          barPercentage: 0.85,
          categoryPercentage: 0.9,
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: { duration: 0 },
      plugins: { legend: { display: false }, tooltip: tooltipOpts(cc) },
      scales: {
        x: {
          grid: { display: false },
          border: { color: cc.grid },
          ticks: { color: cc.muted, font: { size: 10 }, maxTicksLimit: 8 },
        },
        y: {
          beginAtZero: true,
          grid: { color: cc.grid },
          border: { display: false },
          ticks: { color: cc.muted, font: { size: 10 }, precision: 0 },
        },
      },
    },
  });
}

function methodBadge(method: string) {
  switch (method) {
    case "GET":
      return "bg-[#30D158]/10 text-[#30D158]";
    case "POST":
      return "bg-primary/10 text-primary";
    case "PUT":
      return "bg-[#FF9500]/10 text-[#FF9500]";
    case "DELETE":
      return "bg-[#FF453A]/10 text-[#FF453A]";
    default:
      return "bg-secondary text-text-tertiary";
  }
}
</script>

<template>
  <div class="min-h-full">
    <!-- 统一 Page Header -->
    <VPageHeader title="性能监控">
      <template #icon>
        <Icon icon="ri:speed-line" class="text-primary" />
      </template>
      <template #subtitle>实时监控 Go 运行时负载、内存分配趋势、Goroutines 并发与高耗时路由排行</template>
      <template #actions>
        <div class="apple-segmented">
          <button
            v-for="h in [1, 6, 24]"
            :key="h"
            type="button"
            :class="{ active: hours === h }"
            @click="hours = h"
          >
            最近 {{ h }}h
          </button>
        </div>

        <VButton @click="load">
          <template #icon><Icon icon="ri:refresh-line" /></template>
          刷新
        </VButton>
      </template>
    </VPageHeader>

    <div class="mx-auto max-w-[1400px] space-y-5 px-4 py-5 sm:px-6 md:px-8">
      <VLoading v-if="loading" />

      <!-- 模块未启用提示 -->
      <VEmpty
        v-else-if="disabled"
        title="性能监控模块未启用"
        message="请前往「功能模块」页启用性能监控模块后查看实时指标与趋势"
      >
        <template #actions>
          <VButton type="primary" :route="{ path: '/modules' }">
            前往功能模块
          </VButton>
        </template>
      </VEmpty>

      <template v-else-if="overview">
        <!-- 概览数据指标卡 -->
        <div class="grid grid-cols-2 gap-3 sm:grid-cols-4 sm:gap-4">
          <div class="stats-card">
            <div class="stats-icon text-[#30D158] bg-[#30D158]/10">
              <Icon icon="ri:timer-flash-line" />
            </div>
            <div>
              <div class="stats-label">运行时长 (Uptime)</div>
              <div class="stats-value text-[#30D158]">{{ fmtUptime(overview.uptime_sec) }}</div>
              <span class="stats-sub font-mono">{{ overview.go_version }} · {{ overview.platform }}</span>
            </div>
          </div>

          <div class="stats-card">
            <div class="stats-icon text-primary bg-primary/10">
              <Icon icon="ri:cpu-line" />
            </div>
            <div>
              <div class="stats-label">并发协程 (Goroutines)</div>
              <div class="stats-value">{{ overview.goroutines }} <span class="stats-sub">活跃协程</span></div>
            </div>
          </div>

          <div class="stats-card">
            <div class="stats-icon text-[#30B0C7] bg-[#30B0C7]/10">
              <Icon icon="ri:pie-chart-line" />
            </div>
            <div>
              <div class="stats-label">堆内存 / 系统内存</div>
              <div class="stats-value">{{ overview.heap_alloc_mb.toFixed(1) }} <span class="text-xs text-text-tertiary">/ {{ overview.sys_mem_mb.toFixed(1) }} MB</span></div>
              <span class="stats-sub">GC 执行 {{ overview.gc_count }} 次</span>
            </div>
          </div>

          <div class="stats-card">
            <div class="stats-icon text-[#AF52DE] bg-[#AF52DE]/10">
              <Icon icon="ri:database-2-line" />
            </div>
            <div>
              <div class="stats-label">数据库体积 (SQLite)</div>
              <div class="stats-value">{{ fmtBytes(overview.db_size_bytes) }}</div>
              <span class="stats-sub">近 {{ overview.recent.requests }} 次请求 · 均耗 {{ fmtMs(overview.recent.avg_ms) }}</span>
            </div>
          </div>
        </div>

        <!-- 图表区：内存趋势 + 吞吐分布 -->
        <div class="grid grid-cols-1 gap-5 lg:grid-cols-2">
          <!-- 内存趋势 -->
          <VCard title="内存占用与协程变化趋势">
            <template #header>
              <div class="flex items-center justify-between w-full">
                <span class="font-bold text-foreground text-sm flex items-center gap-2">
                  <Icon icon="ri:line-chart-line" class="text-primary text-base" />
                  内存与协程趋势
                </span>
                <span class="text-xs text-text-quaternary font-mono">采样粒度 30s</span>
              </div>
            </template>
            <div class="h-64 w-full">
              <canvas ref="memCanvas"></canvas>
            </div>
          </VCard>

          <!-- 吞吐分布 -->
          <VCard title="请求吞吐分布">
            <template #header>
              <div class="flex items-center justify-between w-full">
                <span class="font-bold text-foreground text-sm flex items-center gap-2">
                  <Icon icon="ri:bar-chart-grouped-line" class="text-primary text-base" />
                  分时请求吞吐量
                </span>
                <span class="text-xs text-text-quaternary font-mono">聚合粒度 1min</span>
              </div>
            </template>
            <div class="h-64 w-full">
              <canvas ref="tpCanvas"></canvas>
            </div>
          </VCard>
        </div>

        <!-- 路由耗时排行榜 -->
        <VCard title="路由访问与耗时排行 (Top Routes)">
          <template #header>
            <div class="flex items-center justify-between w-full">
              <span class="font-bold text-foreground text-sm flex items-center gap-2">
                <Icon icon="ri:fire-line" class="text-[#FF9500] text-base" />
                路由调用频次与平均耗时
              </span>
              <span class="text-xs text-text-quaternary">统计窗口: {{ hours }}h</span>
            </div>
          </template>

          <div v-if="!routes.length" class="py-8 text-center text-xs text-text-quaternary">
            当前时间窗口内暂无请求记录
          </div>

          <div v-else class="overflow-x-auto">
            <table class="w-full text-left text-xs min-w-[700px]">
              <thead>
                <tr class="border-b border-border text-text-quaternary font-medium">
                  <th class="py-2.5 px-3">请求方法</th>
                  <th class="py-2.5 px-3">路由路径</th>
                  <th class="py-2.5 px-3 text-right">总请求数</th>
                  <th class="py-2.5 px-3 text-right">平均耗时</th>
                  <th class="py-2.5 px-3 text-right">最大耗时</th>
                  <th class="py-2.5 px-3 text-right">错误率</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-border/60 font-mono">
                <tr v-for="r in routes" :key="r.method + r.route" class="hover:bg-secondary/40 transition-colors">
                  <td class="py-2.5 px-3">
                    <span class="rounded px-2 py-0.5 text-[10px] font-bold" :class="methodBadge(r.method)">
                      {{ r.method }}
                    </span>
                  </td>
                  <td class="py-2.5 px-3 font-semibold text-foreground truncate max-w-[280px]" :title="r.route">
                    {{ r.route }}
                  </td>
                  <td class="py-2.5 px-3 text-right text-text-secondary">
                    {{ r.count }}
                  </td>
                  <td class="py-2.5 px-3 text-right" :class="r.avg_ms > 100 ? 'text-[#FF9500]' : 'text-foreground'">
                    {{ fmtMs(r.avg_ms) }}
                  </td>
                  <td class="py-2.5 px-3 text-right text-text-tertiary">
                    {{ fmtMs(r.max_ms) }}
                  </td>
                  <td class="py-2.5 px-3 text-right" :class="r.err_count > 0 ? 'text-[#FF453A] font-bold' : 'text-text-quaternary'">
                    {{ fmtPct(r.err_count / (r.count || 1)) }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </VCard>
      </template>
    </div>
  </div>
</template>

<style scoped>
/* 统计卡片 */
.stats-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  background: var(--color-card);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  position: relative;
  overflow: hidden;
  box-shadow: var(--shadow-xs);
}
.stats-card::before {
  content: "";
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 1px;
  background: linear-gradient(90deg, transparent, color-mix(in srgb, #fff 8%, transparent), transparent);
  pointer-events: none;
}
.stats-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border-radius: var(--radius-md);
  font-size: 20px;
  flex-shrink: 0;
}
.stats-label {
  font-size: 11px;
  font-weight: 500;
  color: var(--color-text-quaternary);
  text-transform: uppercase;
  letter-spacing: 0.2px;
}
.stats-value {
  font-size: 16px;
  font-weight: 700;
  color: var(--color-foreground);
  line-height: 1.2;
}
.stats-sub {
  display: block;
  font-size: 11px;
  font-weight: 400;
  color: var(--color-text-tertiary);
  margin-top: 1px;
}

@media (max-width: 640px) {
  .stats-card {
    padding: 10px 12px;
    gap: 8px;
  }
  .stats-icon {
    width: 32px;
    height: 32px;
    font-size: 16px;
  }
  .stats-value {
    font-size: 14px;
  }
}
</style>
