<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from"vue";
import { statsApi } from"@/api";
import { Icon } from"@iconify/vue";
import { VCard } from"@/components/ui";
import { Chart } from"chart.js";

const trendDays = ref(7);
const trend = ref<{ date: string; pv: number; uv: number }[]>([]);
const trendCanvas = ref<HTMLCanvasElement | null>(null);
const metricTab = ref<'pv' | 'uv' | 'both'>('both');
let trendChartInstance: Chart | null = null;

/** 趋势图日期范围文字（如"2026-06-27 至 2026-07-26"） */
const trendDateRange = computed(() => {
 if (!trend.value.length) return '';
 return `${trend.value[0].date} 至 ${trend.value[trend.value.length - 1].date}`;
});

async function loadTrend() {
 try {
 const { data } = await statsApi.trend(trendDays.value);
 trend.value = data.trend || [];
 } catch (err) {
 console.error('[TrendChart] Failed to load trend data:', err);
 trend.value = [];
 }
 initTrendChart();
}

// 初始化主趋势折线图
function initTrendChart() {
 if (!trend.value || trend.value.length === 0) return;
 const canvas = trendCanvas.value;
 if (!canvas) return;

 const ctx = canvas.getContext('2d');
 const cs = getComputedStyle(document.documentElement);
 const c1 = cs.getPropertyValue('--color-chart-1').trim() || '#0A84FF';
 const c2 = cs.getPropertyValue('--color-chart-2').trim() || '#BF5AF2';
 const gridColor = cs.getPropertyValue('--color-chart-grid').trim() || '#2c2c2e';
 const mutedColor = cs.getPropertyValue('--color-muted-foreground').trim() || '#8e8e93';
 const cardBg = cs.getPropertyValue('--color-card').trim() || '#1c1c1e';
 const fgColor = cs.getPropertyValue('--color-foreground').trim() || '#ffffff';
 const borderColor = cs.getPropertyValue('--color-border').trim() || '#3a3a3c';

 // PV 渐变色填充 - hex alpha（1F = 12% 透明度）
 const gradient = ctx!.createLinearGradient(0, 0, 0, canvas.height);
 gradient.addColorStop(0, c1 + '1F');
 gradient.addColorStop(1, c1 + '00');

 trendChartInstance = new Chart(canvas, {
 type: 'line',
 data: {
 labels: trend.value.map(d => d.date.slice(5)),
 datasets: [
 {
 label: 'PV',
 data: trend.value.map(d => d.pv),
 borderColor: c1,
 backgroundColor: gradient,
 borderWidth: 2,
 fill: true,
 tension: 0.35,
 pointRadius: 0,
 pointHoverRadius: 4,
 pointHoverBackgroundColor: c1,
 pointHoverBorderColor: '#1c1c1e',
 pointHoverBorderWidth: 2,
 hidden: metricTab.value === 'uv'
 },
 {
 label: 'UV',
 data: trend.value.map(d => d.uv),
 borderColor: c2,
 backgroundColor: 'transparent',
 borderWidth: 2,
 fill: false,
 tension: 0.35,
 pointRadius: 0,
 pointHoverRadius: 4,
 pointHoverBackgroundColor: c2,
 pointHoverBorderColor: '#1c1c1e',
 pointHoverBorderWidth: 2,
 hidden: metricTab.value === 'pv'
 }
 ]
 },
 options: {
 responsive: true,
 maintainAspectRatio: false,
 animation: {
 duration: 0, // 禁用动画避免重绘
 },
 interaction: { mode: 'index', intersect: false },
 plugins: {
 legend: { display: false },
 tooltip: {
 backgroundColor: cardBg,
 titleColor: fgColor,
 bodyColor: mutedColor,
 borderColor: borderColor,
 borderWidth: 1,
 padding: 10,
 cornerRadius: 8,
 displayColors: true,
 boxPadding: 4
 }
 },
 scales: {
 x: {
 grid: { display: false },
 border: { color: gridColor },
 ticks: {
 color: mutedColor,
 font: { size: 11 },
 maxRotation: 0,
 maxTicksLimit: 7,
 callback: (_, index) => {
 const step = Math.max(1, Math.ceil(trend.value.length / 7));
 return index % step === 0 ? trend.value[index]?.date.slice(5) : '';
 }
 }
 },
 y: {
 beginAtZero: false,
 grid: { color: gridColor },
 border: { display: false },
 ticks: {
 color: mutedColor,
 font: { size: 11 }
 }
 }
 }
 }
 });
}

// 指标切换
function switchMetric(key: 'pv' | 'uv' | 'both') {
 metricTab.value = key;
 if (trendChartInstance) {
 trendChartInstance.data.datasets[0].hidden = key === 'uv';
 trendChartInstance.data.datasets[1].hidden = key === 'pv';
 trendChartInstance.update();
 }
}

onMounted(loadTrend);

onUnmounted(() => {
 trendChartInstance?.destroy();
 trendChartInstance = null;
});
</script>

<template>
 <VCard class="h-full">
 <template #header>
 <div class="flex w-full flex-col gap-2 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
 <div>
 <span class="flex items-center gap-2 text-base font-semibold text-foreground">
 <Icon icon="ri:line-chart-line" class="text-lg" />
 访问趋势
 </span>
 <p v-if="trendDateRange" class="mt-0.5 text-xs text-muted-foreground">{{ trendDateRange }}</p>
 </div>
 <div class="inline-flex rounded-md border border-border bg-muted/40 p-0.5">
 <button
 v-for="{ key, label } in [
 { key: 'pv', label: 'PV' },
 { key: 'uv', label: 'UV' },
 { key: 'both', label: '同时' }
 ]"
 :key="label"
 :class="[
 'rounded-[6px] px-3 py-1.5 text-xs font-medium transition-all',
 metricTab === key 
 ? 'bg-primary text-primary-foreground shadow-sm'
 : 'text-muted-foreground hover:text-foreground',
 ]"
 @click="switchMetric(key)"
 >
 {{ label }}
 </button>
 </div>
 </div>
 </template>
 <div class="p-4">
 <!-- 图例（图表上方） -->
 <div class="flex items-center justify-center gap-4 text-xs text-muted-foreground">
 <span class="inline-flex items-center gap-1.5">
 <span class="h-2.5 w-2.5 rounded-full bg-chart-1"></span>
 PV 页面浏览量
 </span>
 <span class="inline-flex items-center gap-1.5">
 <span class="h-2.5 w-2.5 rounded-full bg-chart-2"></span>
 UV 独立访客
 </span>
 </div>
 <div class="mt-3 h-[260px] w-full">
 <canvas ref="trendCanvas"></canvas>
 </div>
 </div>
 </VCard>
</template>
