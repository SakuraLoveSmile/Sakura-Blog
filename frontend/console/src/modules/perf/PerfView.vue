<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from "vue";
import { useRouter } from "vue-router";
import { Icon } from "@iconify/vue";
import type { Chart } from "chart.js";
import { loadChart } from "@/lib/chart-init";
import { VPageHeader, VCard, VLoading, VEmpty } from "@/components/ui";
import {
 perfApi,
 type PerfOverview,
 type PerfSamplePoint,
 type PerfRouteStat,
 type PerfMinuteStat,
} from "@/api";

// 性能监控面板（perf 模块）：运行时指标卡 + 内存趋势/吞吐图表 + 路由耗时排行。
// 数据时间窗可选 1h/6h/24h（采样粒度：指标 30s、请求聚合 1min）。

const router = useRouter();
const loading = ref(true);
const disabled = ref(false); // API 403 = 模块未启用
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

// —— 格式化辅助 ——

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
const shortTs = (ts: string) => ts.slice(11, 16); // "2006-01-02 15:04:05" -> "15:04"
const shortMinute = (m: string) => m.length > 10 ? m.slice(5, 16) : m.slice(11, 16);

// —— 图表（样式对齐 Dashboard HourlyChartWidget：消费 CSS 变量 token） ——

function chartColors() {
 const cs = getComputedStyle(document.documentElement);
 return {
 c1: cs.getPropertyValue("--color-chart-1").trim() || "#2E8DFF",
 primary: cs.getPropertyValue("--color-primary").trim() || "#2E8DFF",
 grid: cs.getPropertyValue("--color-chart-grid").trim() || "#2c2c2e",
 muted: cs.getPropertyValue("--color-muted-foreground").trim() || "#8e8e93",
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
 const gorColor = getComputedStyle(document.documentElement).getPropertyValue("--color-state-success").trim() || "#30D158";
 memChart = new Chart(canvas, {
 type: "line",
 data: {
 labels: heap.map((p) => shortTs(p.ts)),
 datasets: [
 {
 label: "堆内存 (MB)",
 data: heap.map((p) => Number(p.value.toFixed(2))),
 borderColor: cc.primary,
 backgroundColor: cc.primary + "22",
 fill: true,
 pointRadius: 0,
 tension: 0.3,
 borderWidth: 1.5,
 },
 {
 label: "Sys (MB)",
 data: sys.map((p) => Number(p.value.toFixed(2))),
 borderColor: cc.c1,
 pointRadius: 0,
 tension: 0.3,
 borderWidth: 1.5,
 borderDash: [4, 3],
 },
 {
 label: "Goroutines",
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
 plugins: { legend: { labels: { color: cc.muted, boxWidth: 12, font: { size: 11 } } }, tooltip: tooltipOpts(cc) },
 scales: {
 x: { grid: { display: false }, border: { color: cc.grid }, ticks: { color: cc.muted, font: { size: 10 }, maxTicksLimit: 8 } },
 y: { beginAtZero: true, grid: { color: cc.grid }, border: { display: false }, ticks: { color: cc.muted, font: { size: 10 } } },
 y1: { position: "right", beginAtZero: true, grid: { drawOnChartArea: false }, border: { display: false }, ticks: { color: cc.muted, font: { size: 10 }, precision: 0 } },
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
 label: "请求数",
 data: throughput.value.map((p) => p.count),
 backgroundColor: cc.c1,
 borderRadius: 2,
 barPercentage: 0.9,
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
 x: { grid: { display: false }, border: { color: cc.grid }, ticks: { color: cc.muted, font: { size: 10 }, maxTicksLimit: 8 } },
 y: { beginAtZero: true, grid: { color: cc.grid }, border: { display: false }, ticks: { color: cc.muted, font: { size: 10 }, precision: 0 } },
 },
 },
 });
}
</script>

<template>
 <div>
 <VPageHeader title="性能监控">
 <template #icon><Icon icon="ri:speed-line" class="text-primary" /></template>
 <template #actions>
 <div class="flex items-center gap-1 rounded-lg border border-border bg-card p-0.5">
 <button
 v-for="h in [1, 6, 24]"
 :key="h"
 @click="hours = h"
 :class="[
 'px-2.5 py-1 text-xs rounded-md transition-colors',
 hours === h
 ? 'bg-secondary text-white'
 : 'text-text-tertiary hover:bg-accent',
 ]"
 >{{ h }}h</button>
 </div>
 </template>
 </VPageHeader>

 <div class="m-0 p-4 md:m-4 md:p-0 space-y-6">
 <VLoading v-if="loading" />

 <!-- 模块未启用提示 -->
 <VEmpty
 v-else-if="disabled"
 title="性能监控未启用"
 message="请前往「功能模块」页启用性能监控后查看数据"
 >
 <template #actions>
 <button
 class="px-4 py-2 bg-secondary text-white text-sm font-medium rounded-lg hover:opacity-90"
 @click="router.push('/modules')"
 >前往功能模块
 </button>
 </template>
 </VEmpty>

 <template v-else-if="overview">
 <!-- 指标卡 -->
 <section>
 <h3 class="mb-3 text-sm font-semibold text-text-tertiary">运行状态</h3>
 <div class="grid grid-cols-2 gap-4 md:grid-cols-3 xl:grid-cols-6">
 <VCard><div class="p-1">
 <p class="text-xs text-text-quaternary">运行时长</p>
 <p class="mt-1 text-xl font-bold text-foreground">{{ fmtUptime(overview.uptime_sec) }}</p>
 </div></VCard>
 <VCard><div class="p-1">
 <p class="text-xs text-text-quaternary">堆内存</p>
 <p class="mt-1 text-xl font-bold text-foreground">{{ overview.heap_alloc_mb.toFixed(1) }} MB</p>
 </div></VCard>
 <VCard><div class="p-1">
 <p class="text-xs text-text-quaternary">Goroutines</p>
 <p class="mt-1 text-xl font-bold text-foreground">{{ overview.goroutines }}</p>
 </div></VCard>
 <VCard><div class="p-1">
 <p class="text-xs text-text-quaternary">近 5 分钟请求</p>
 <p class="mt-1 text-xl font-bold text-foreground">{{ overview.recent.requests }}</p>
 </div></VCard>
 <VCard><div class="p-1">
 <p class="text-xs text-text-quaternary">平均耗时</p>
 <p class="mt-1 text-xl font-bold text-foreground">{{ fmtMs(overview.recent.avg_ms) }}</p>
 </div></VCard>
 <VCard><div class="p-1">
 <p class="text-xs text-text-quaternary">错误率（近 5 分钟）</p>
 <p class="mt-1 text-xl font-bold" :class="overview.recent.err_rate > 0.05 ? 'text-state-error' : 'text-foreground'">
 {{ fmtPct(overview.recent.err_rate) }}
 </p>
 </div></VCard>
 </div>
 <p class="mt-2 text-xs text-text-quaternary">
 {{ overview.go_version }} · {{ overview.platform }} · GC {{ overview.gc_count }} 次 ·
 数据库 {{ fmtBytes(overview.db_size_bytes) }}
 </p>
 </section>

 <!-- 趋势图 -->
 <section class="grid grid-cols-1 gap-4 lg:grid-cols-2">
 <VCard>
 <template #header>
 <div class="flex w-full items-center justify-between px-4 py-3">
 <span class="text-base font-bold text-foreground">内存 / Goroutine 趋势</span>
 </div>
 </template>
 <div class="p-4">
 <div class="h-[240px] w-full">
 <canvas ref="memCanvas"></canvas>
 </div>
 <VEmpty v-if="!samples.length" title="暂无采样数据" message="采样间隔 30 秒，稍后刷新查看" />
 </div>
 </VCard>
 <VCard>
 <template #header>
 <div class="flex w-full items-center justify-between px-4 py-3">
 <span class="text-base font-bold text-foreground">请求吞吐（每分钟）</span>
 </div>
 </template>
 <div class="p-4">
 <div class="h-[240px] w-full">
 <canvas ref="tpCanvas"></canvas>
 </div>
 <VEmpty v-if="!throughput.length" title="暂无请求数据" message="产生前台访问后自动聚合" />
 </div>
 </VCard>
 </section>

 <!-- 路由耗时排行 -->
 <section>
 <h3 class="mb-3 text-sm font-semibold text-text-tertiary">路由耗时排行（按平均耗时）</h3>
 <VCard :body-class="['!p-0']">
 <VEmpty v-if="!routes.length" title="暂无数据" message="时间窗内没有已记录的请求" />
 <div v-else class="overflow-x-auto">
 <table class="w-full text-sm">
 <thead>
 <tr class="text-left text-xs text-text-quaternary border-b border-border">
 <th class="px-4 py-2 font-medium">方法</th>
 <th class="px-4 py-2 font-medium">路由</th>
 <th class="px-4 py-2 font-medium text-right">请求数</th>
 <th class="px-4 py-2 font-medium text-right">平均耗时</th>
 <th class="px-4 py-2 font-medium text-right">最大耗时</th>
 <th class="px-4 py-2 font-medium text-right">错误数</th>
 </tr>
 </thead>
 <tbody>
 <tr v-for="r in routes" :key="r.method + r.route" class="border-b border-border last:border-0">
 <td class="px-4 py-2">
 <code class="text-xs font-mono text-secondary">{{ r.method }}</code>
 </td>
 <td class="px-4 py-2 font-mono text-xs text-foreground">{{ r.route }}</td>
 <td class="px-4 py-2 text-right text-text-secondary">{{ r.count }}</td>
 <td class="px-4 py-2 text-right font-mono text-xs text-foreground">{{ fmtMs(r.avg_ms) }}</td>
 <td class="px-4 py-2 text-right font-mono text-xs text-text-tertiary">{{ fmtMs(r.max_ms) }}</td>
 <td class="px-4 py-2 text-right" :class="r.err_count ? 'text-state-error' : 'text-text-quaternary'">
 {{ r.err_count }}
 </td>
 </tr>
 </tbody>
 </table>
 </div>
 </VCard>
 </section>
 </template>
 </div>
 </div>
</template>
