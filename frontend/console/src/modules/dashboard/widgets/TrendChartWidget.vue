<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { Icon } from "@iconify/vue";
import { useBreakpoint } from "../useBreakpoint";

interface DailyStat {
  date: string;
  pv: number;
  uv: number;
}

const { breakpoint } = useBreakpoint();

const days = ref<7 | 30>(7);
const data = ref<DailyStat[]>([]);
const loading = ref(true);
const error = ref("");

function fetchTrend(d: number) {
  loading.value = true;
  fetch(`/api/v1/stats/trend?days=${d}`, { credentials: "include" })
    .then((r) => {
      if (!r.ok) throw new Error("加载趋势失败");
      return r.json();
    })
    .then((resp: { trend: DailyStat[] }) => {
      data.value = resp.trend;
    })
    .catch((e) => (error.value = e.message))
    .finally(() => (loading.value = false));
}

onMounted(() => fetchTrend(days.value));
watch(days, (d) => fetchTrend(d));

const totals = computed(() => {
  const pv = data.value.reduce((s, d) => s + d.pv, 0);
  const uv = data.value.reduce((s, d) => s + d.uv, 0);
  const avgPv = data.value.length ? Math.round(pv / data.value.length) : 0;
  const peak = data.value.reduce((m, d) => Math.max(m, d.pv), 0);
  return { pv, uv, avgPv, peak };
});

const chartWidth = 720;
const chartHeight = 220;
const padding = computed(() => {
  if (breakpoint.value === "s") return { top: 16, right: 8, bottom: 30, left: 8 };
  if (breakpoint.value === "m") return { top: 20, right: 12, bottom: 30, left: 12 };
  return { top: 20, right: 20, bottom: 30, left: 40 };
});

const xScale = computed(() => {
  const n = data.value.length;
  const p = padding.value;
  if (n <= 1) return () => p.left;
  const w = chartWidth - p.left - p.right;
  return (i: number) => p.left + (i / (n - 1)) * w;
});

const yScale = computed(() => {
  const max = Math.max(...data.value.map((d) => d.pv), 1);
  const p = padding.value;
  const h = chartHeight - p.top - p.bottom;
  return (v: number) => p.top + h - (v / max) * h;
});

const areaPath = computed(() => {
  if (data.value.length === 0) return "";
  const p = padding.value;
  const pts = data.value.map((d, i) => `${xScale.value(i)},${yScale.value(d.pv)}`);
  const lastX = xScale.value(data.value.length - 1);
  const firstX = xScale.value(0);
  const bottomY = chartHeight - p.bottom;
  return `M${pts.join(" L")} L${lastX},${bottomY} L${firstX},${bottomY} Z`;
});

const linePath = computed(() => {
  if (data.value.length === 0) return "";
  const pts = data.value.map((d, i) => `${xScale.value(i)},${yScale.value(d.pv)}`);
  return `M${pts.join(" L")}`;
});

function formatNumber(n: number): string {
  if (n >= 10000) return (n / 10000).toFixed(1) + "w";
  if (n >= 1000) return (n / 1000).toFixed(1) + "k";
  return n.toString();
}

function formatLabel(dateStr: string, idx: number, total: number): string {
  const d = new Date(dateStr);
  const m = d.getMonth() + 1;
  const day = d.getDate();
  if (total <= 7) return `${m}/${day}`;
  if (idx % Math.ceil(total / 6) === 0 || idx === total - 1) return `${m}/${day}`;
  return "";
}
</script>

<template>
  <div class="apple-card h-full" :data-bp="breakpoint">
    <div class="flex items-center justify-between trend-header">
      <div>
        <h3 class="apple-section-title">访问趋势</h3>
        <p class="trend-subtitle">Traffic Overview</p>
      </div>
      <div class="apple-segment">
        <button :class="['apple-segment-btn', days === 7 && 'active']" @click="days = 7">7 天</button>
        <button :class="['apple-segment-btn', days === 30 && 'active']" @click="days = 30">30 天</button>
      </div>
    </div>

    <div class="trend-stats">
      <div>
        <div class="trend-stat-label">总访问量</div>
        <div class="trend-stat-value">{{ formatNumber(totals.pv) }}</div>
      </div>
      <div>
        <div class="trend-stat-label">独立访客</div>
        <div class="trend-stat-value">{{ formatNumber(totals.uv) }}</div>
      </div>
      <div>
        <div class="trend-stat-label">日均访问</div>
        <div class="trend-stat-value">{{ formatNumber(totals.avgPv) }}</div>
      </div>
    </div>

    <div v-if="loading" class="trend-chart-box flex items-center justify-center text-[#8E8E93]">
      <Icon icon="ri:loader-4-line" class="w-6 h-6 animate-spin" />
    </div>
    <div v-else-if="error" class="trend-chart-box flex items-center justify-center text-red-400">{{ error }}</div>
    <svg v-else :viewBox="`0 0 ${chartWidth} ${chartHeight}`" class="w-full trend-chart-svg">
      <defs>
        <linearGradient id="trendGrad" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stop-color="#0A84FF" stop-opacity="0.4" />
          <stop offset="100%" stop-color="#0A84FF" stop-opacity="0" />
        </linearGradient>
        <linearGradient id="trendLine" x1="0" y1="0" x2="1" y2="0">
          <stop offset="0%" stop-color="#5AC8FA" />
          <stop offset="50%" stop-color="#0A84FF" />
          <stop offset="100%" stop-color="#5856D6" />
        </linearGradient>
      </defs>
      <!-- grid lines -->
      <g stroke="#2C2C2E" stroke-width="1">
        <line v-for="i in 4" :key="i"
          :x1="padding.left"
          :x2="chartWidth - padding.right"
          :y1="padding.top + ((chartHeight - padding.top - padding.bottom) / 4) * i"
          :y2="padding.top + ((chartHeight - padding.top - padding.bottom) / 4) * i"
        />
      </g>
      <path :d="areaPath" fill="url(#trendGrad)" />
      <path :d="linePath" fill="none" stroke="url(#trendLine)" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" />
      <!-- data points -->
      <circle
        v-for="(d, i) in data"
        :key="i"
        :cx="xScale(i)"
        :cy="yScale(d.pv)"
        r="3"
        fill="#0A84FF"
        stroke="#1C1C1E"
        stroke-width="2"
      />
      <!-- x labels -->
      <text
        v-for="(d, i) in data"
        :key="'l'+i"
        :x="xScale(i)"
        :y="chartHeight - 8"
        text-anchor="middle"
        fill="#8E8E93"
        font-size="11"
      >{{ formatLabel(d.date, i, data.length) }}</text>
    </svg>
  </div>
</template>

<style scoped>
.trend-header { margin-bottom: 24px; }
.trend-subtitle { color: #8E8E93; font-size: 13px; margin-top: 4px; }

.trend-stats {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
  margin-bottom: 24px;
}
.trend-stat-label { color: #8E8E93; font-size: 12px; margin-bottom: 4px; }
.trend-stat-value { color: #fff; font-size: 20px; font-weight: 700; letter-spacing: -0.02em; }

.trend-chart-box { height: 220px; }
.trend-chart-svg { height: 220px; }

/* M 断点 */
.apple-card[data-bp="m"] .trend-header,
.apple-card[data-bp="s"] .trend-header { margin-bottom: 18px; }
.apple-card[data-bp="m"] .trend-subtitle,
.apple-card[data-bp="s"] .trend-subtitle { font-size: 12px; }
.apple-card[data-bp="m"] .trend-stats,
.apple-card[data-bp="s"] .trend-stats { gap: 12px; margin-bottom: 18px; }
.apple-card[data-bp="m"] .trend-stat-value,
.apple-card[data-bp="s"] .trend-stat-value { font-size: 18px; }
.apple-card[data-bp="m"] .trend-chart-box,
.apple-card[data-bp="s"] .trend-chart-box { height: 180px; }
.apple-card[data-bp="m"] .trend-chart-svg,
.apple-card[data-bp="s"] .trend-chart-svg { height: 180px; }

/* S 断点 */
.apple-card[data-bp="s"] .trend-header {
  flex-direction: column;
  align-items: stretch !important;
  gap: 10px;
  margin-bottom: 14px;
}
.apple-card[data-bp="s"] .trend-header .apple-segment {
  align-self: flex-start;
}
.apple-card[data-bp="s"] .trend-stats { gap: 8px; margin-bottom: 14px; }
.apple-card[data-bp="s"] .trend-stat-label { font-size: 10px; margin-bottom: 2px; }
.apple-card[data-bp="s"] .trend-stat-value { font-size: 15px; }
.apple-card[data-bp="s"] .trend-chart-box { height: 150px; }
.apple-card[data-bp="s"] .trend-chart-svg { height: 150px; }
</style>
