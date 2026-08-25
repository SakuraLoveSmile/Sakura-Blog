<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { Icon } from "@iconify/vue";
import { useBreakpoint } from "../useBreakpoint";
import { statsApi } from "@/api";
import type { WidgetSizePreset } from "./layout";
import { useWidgetSize } from "../useWidgetSize";

interface Props {
  sizePreset?: WidgetSizePreset;
}

const props = withDefaults(defineProps<Props>(), {
  sizePreset: "medium",
});
const sizePreset = useWidgetSize(props.sizePreset).preset;

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
  // Spec 62：裸 fetch 改走 statsApi（统一 axios 拦截器：401 跳登录/超时/错误结构）
  statsApi
    .trend(d)
    .then((resp: { data: { trend: DailyStat[] } }) => {
      data.value = resp.data.trend;
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
  const peakItem = data.value.find((d) => d.pv === peak);
  return { pv, uv, avgPv, peak, peakDate: peakItem ? peakItem.date : "" };
});

const chartWidth = 720;
const chartHeight = computed(() => {
  if (sizePreset.value === "small") return 160;
  if (sizePreset.value === "large") return 240;
  return 220;
});

const padding = computed(() => {
  if (sizePreset.value === "small") return { top: 12, right: 8, bottom: 22, left: 8 };
  if (breakpoint.value === "xxs") return { top: 16, right: 8, bottom: 30, left: 8 };
  if (breakpoint.value === "sm") return { top: 20, right: 12, bottom: 30, left: 12 };
  return { top: 20, right: 20, bottom: 30, left: 40 };
});

const xScale = computed(() => {
  const n = data.value.length;
  const p = padding.value;
  if (n <= 1) return () => p.left;
  const w = chartWidth - p.left - p.right;
  return (i: number) => p.left + (i / (n - 1)) * w;
});

const maxVal = computed(() => {
  const peak = Math.max(...data.value.map((d) => d.pv), 0);
  if (peak === 0) return 1;
  // 留出少量顶部呼吸空间，避免峰值贴住图表边界；仍以 0 为基线，保持数据比例诚实。
  return Math.max(peak * 1.12, peak + 1);
});

const yScale = computed(() => {
  const max = maxVal.value;
  const p = padding.value;
  const h = chartHeight.value - p.top - p.bottom;
  return (v: number) => p.top + h - (v / max) * h;
});

function getShapePreservingSpline(points: { x: number; y: number }[]): string {
  if (points.length === 0) return "";
  if (points.length === 1) return `M${points[0].x},${points[0].y}`;
  if (points.length === 2) return `M${points[0].x},${points[0].y} L${points[1].x},${points[1].y}`;

  // Catmull–Rom 会在峰谷附近制造额外波峰。用 Fritsch–Carlson 风格的
  // 单调切线，让曲线只经过真实数据的峰谷，视觉平滑但不虚构拐点。
  const secants = points.slice(0, -1).map((point, i) => {
    const next = points[i + 1];
    return (next.y - point.y) / (next.x - point.x);
  });
  const tangents = new Array<number>(points.length);
  tangents[0] = secants[0];
  tangents[points.length - 1] = secants[secants.length - 1];

  for (let i = 1; i < points.length - 1; i++) {
    const previous = secants[i - 1];
    const next = secants[i];
    if (previous === 0 || next === 0 || Math.sign(previous) !== Math.sign(next)) {
      tangents[i] = 0;
      continue;
    }
    // 相邻点等距时，调和平均可限制切线幅度，避免过冲。
    tangents[i] = (2 * previous * next) / (previous + next);
  }

  let d = `M${points[0].x},${points[0].y}`;
  for (let i = 0; i < points.length - 1; i++) {
    const point = points[i];
    const next = points[i + 1];
    const segment = next.x - point.x;
    const cp1x = point.x + segment / 3;
    const cp1y = point.y + (tangents[i] * segment) / 3;
    const cp2x = next.x - segment / 3;
    const cp2y = next.y - (tangents[i + 1] * segment) / 3;
    d += ` C${cp1x},${cp1y} ${cp2x},${cp2y} ${next.x},${next.y}`;
  }
  return d;
}

const areaPath = computed(() => {
  if (data.value.length === 0) return "";
  const p = padding.value;
  const pts = data.value.map((d, i) => ({ x: xScale.value(i), y: yScale.value(d.pv) }));
  const spline = getShapePreservingSpline(pts);
  const lastX = xScale.value(data.value.length - 1);
  const firstX = xScale.value(0);
  const bottomY = chartHeight.value - p.bottom;
  return `${spline} L${lastX},${bottomY} L${firstX},${bottomY} Z`;
});

const linePath = computed(() => {
  if (data.value.length === 0) return "";
  const pts = data.value.map((d, i) => ({ x: xScale.value(i), y: yScale.value(d.pv) }));
  return getShapePreservingSpline(pts);
});

// Large 尺寸下的 UV 折线
const uvLinePath = computed(() => {
  if (data.value.length === 0) return "";
  const pts = data.value.map((d, i) => ({ x: xScale.value(i), y: yScale.value(d.uv) }));
  return getShapePreservingSpline(pts);
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
  if (sizePreset.value === "small") {
    if (idx === 0 || idx === total - 1 || idx === Math.floor(total / 2)) return `${m}/${day}`;
    return "";
  }
  if (total <= 7) return `${m}/${day}`;
  if (idx % Math.ceil(total / 6) === 0 || idx === total - 1) return `${m}/${day}`;
  return "";
}

function formatDate(dateStr: string): string {
  const d = new Date(dateStr);
  return `${d.getMonth() + 1}/${d.getDate()}`;
}

/* ---- hover tooltip ---- */
const chartEl = ref<SVGSVGElement | null>(null);
const wrapEl = ref<HTMLDivElement | null>(null);
const hovered = ref<number | null>(null);

const hoveredPoint = computed(() => {
  const i = hovered.value;
  if (i == null || !data.value[i]) return null;
  const d = data.value[i];
  return { x: xScale.value(i), y: yScale.value(d.pv), uvY: yScale.value(d.uv), d };
});

const tip = ref<{ left: number; top: number; below: boolean } | null>(null);
const TIP_HALF = 62;

function viewToWrapPixels(vx: number, vy: number): { x: number; y: number } | null {
  const el = chartEl.value;
  const wrap = wrapEl.value;
  if (!el || !wrap) return null;
  const ctm = el.getScreenCTM();
  if (!ctm) return null;
  const pt = new DOMPoint(vx, vy).matrixTransform(ctm);
  const wr = wrap.getBoundingClientRect();
  return { x: pt.x - wr.left, y: pt.y - wr.top };
}

function updateTip() {
  const i = hovered.value;
  if (i == null || !data.value[i]) {
    tip.value = null;
    return;
  }
  const wrap = wrapEl.value;
  const px = viewToWrapPixels(xScale.value(i), yScale.value(data.value[i].pv));
  if (!wrap || !px) {
    tip.value = null;
    return;
  }
  const wr = wrap.getBoundingClientRect();
  const left = Math.min(Math.max(px.x, TIP_HALF + 4), wr.width - TIP_HALF - 4);
  const below = px.y < 84;
  const top = below ? px.y + 14 : px.y - 14;
  tip.value = { left, top, below };
}

function onChartMove(e: PointerEvent) {
  const el = chartEl.value;
  const n = data.value.length;
  if (!el || n === 0) return;
  const ctm = el.getScreenCTM();
  if (!ctm) return;
  const p = new DOMPoint(e.clientX, e.clientY).matrixTransform(ctm.inverse());
  const pad = padding.value;
  const w = chartWidth - pad.left - pad.right;
  const idx = Math.round(((p.x - pad.left) / w) * (n - 1));
  hovered.value = Math.min(Math.max(idx, 0), n - 1);
  updateTip();
}

function onChartLeave() {
  hovered.value = null;
  tip.value = null;
}
</script>

<template>
  <div class="apple-card h-full" :class="`trend-card-${sizePreset}`" :data-bp="breakpoint">
    <!-- Small 尺寸顶部精简条 -->
    <div v-if="sizePreset === 'small'" class="flex items-center justify-between mb-2">
      <div class="flex items-center gap-2">
        <h3 class="apple-section-title text-sm">访问趋势</h3>
        <span class="text-[11px] text-muted-foreground">近 7 天</span>
      </div>
      <div class="text-xs font-semibold text-primary">总浏览 {{ formatNumber(totals.pv) }}</div>
    </div>

    <!-- Medium / Large 尺寸顶部标准标题与分段器 -->
    <div v-else class="flex items-center justify-between trend-header">
      <div>
        <h3 class="apple-section-title">{{ sizePreset === "large" ? "流量深度分析与访问趋势" : "访问趋势" }}</h3>
        <p class="trend-subtitle">{{ sizePreset === "large" ? "PV / UV 双指标趋势与转化监测" : "Traffic Overview" }}</p>
      </div>
      <div class="flex items-center gap-2">
        <!-- Large 尺寸图例 -->
        <div v-if="sizePreset === 'large'" class="hidden sm:flex items-center gap-3 mr-2 text-xs text-muted-foreground">
          <span class="inline-flex items-center gap-1"
            ><span class="w-2.5 h-2.5 rounded-full bg-primary" /> PV (浏览量)</span
          >
          <span class="inline-flex items-center gap-1"
            ><span class="w-2.5 h-2.5 rounded-full bg-chart-1" /> UV (访客)</span
          >
        </div>
        <div class="apple-segmented">
          <button :class="['seg-btn', days === 7 && 'active']" @click="days = 7">7 天</button>
          <button :class="['seg-btn', days === 30 && 'active']" @click="days = 30">30 天</button>
        </div>
      </div>
    </div>

    <!-- 指标卡片：Small 隐藏；Medium 3项；Large 4项 -->
    <div
      v-if="sizePreset !== 'small'"
      class="trend-stats"
      :class="{ 'grid-cols-4': sizePreset === 'large', 'grid-cols-3': sizePreset === 'medium' }"
    >
      <div>
        <div class="trend-stat-label">总访问量 (PV)</div>
        <div class="trend-stat-value">{{ formatNumber(totals.pv) }}</div>
      </div>
      <div>
        <div class="trend-stat-label">独立访客 (UV)</div>
        <div class="trend-stat-value text-chart-1">{{ formatNumber(totals.uv) }}</div>
      </div>
      <div>
        <div class="trend-stat-label">日均访问</div>
        <div class="trend-stat-value">{{ formatNumber(totals.avgPv) }}</div>
      </div>
      <div v-if="sizePreset === 'large'">
        <div class="trend-stat-label">峰值单日</div>
        <div class="trend-stat-value text-chart-3">{{ formatNumber(totals.peak) }}</div>
      </div>
    </div>

    <div
      v-if="loading"
      class="trend-chart-box flex items-center justify-center text-muted-foreground"
      :style="{ height: `${chartHeight}px` }"
    >
      <Icon icon="ri:loader-4-line" class="w-6 h-6 animate-spin" />
    </div>
    <div
      v-else-if="error"
      class="trend-chart-box flex items-center justify-center text-state-error"
      :style="{ height: `${chartHeight}px` }"
    >
      {{ error }}
    </div>
    <div v-else ref="wrapEl" class="trend-chart-wrap">
      <svg
        ref="chartEl"
        :viewBox="`0 0 ${chartWidth} ${chartHeight}`"
        class="w-full trend-chart-svg"
        :style="{ height: `${chartHeight}px` }"
        @pointermove="onChartMove"
        @pointerdown="onChartMove"
        @pointerleave="onChartLeave"
      >
        <defs>
          <linearGradient id="trendGrad" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stop-color="var(--color-chart-2)" stop-opacity="0.4" />
            <stop offset="100%" stop-color="var(--color-chart-2)" stop-opacity="0" />
          </linearGradient>
          <linearGradient id="trendLine" x1="0" y1="0" x2="1" y2="0">
            <stop offset="0%" stop-color="var(--color-primary-hover)" />
            <stop offset="50%" stop-color="var(--color-chart-2)" />
            <stop offset="100%" stop-color="var(--color-primary-active)" />
          </linearGradient>
        </defs>
        <!-- grid lines -->
        <g stroke="var(--color-chart-grid)" stroke-width="1">
          <line
            v-for="i in sizePreset === 'small' ? 2 : 4"
            :key="i"
            :x1="padding.left"
            :x2="chartWidth - padding.right"
            :y1="padding.top + ((chartHeight - padding.top - padding.bottom) / (sizePreset === 'small' ? 2 : 4)) * i"
            :y2="padding.top + ((chartHeight - padding.top - padding.bottom) / (sizePreset === 'small' ? 2 : 4)) * i"
          />
        </g>
        <path :d="areaPath" fill="url(#trendGrad)" />
        <path
          :d="linePath"
          fill="none"
          stroke="url(#trendLine)"
          stroke-width="2.5"
          stroke-linecap="round"
          stroke-linejoin="round"
        />
        <!-- Large 尺寸渲染 UV 辅助线（chart-1 绿色） -->
        <path
          v-if="sizePreset === 'large'"
          :d="uvLinePath"
          fill="none"
          stroke="var(--color-chart-1)"
          stroke-width="2"
          stroke-dasharray="4 3"
          stroke-linecap="round"
          opacity="0.85"
        />
        <!-- data points -->
        <circle
          v-for="(d, i) in data"
          :key="i"
          :cx="xScale(i)"
          :cy="yScale(d.pv)"
          :r="sizePreset === 'small' ? 2 : 3"
          fill="var(--color-chart-2)"
          stroke="var(--color-card)"
          stroke-width="2"
        />
        <!-- x labels -->
        <text
          v-for="(d, i) in data"
          :key="'l' + i"
          :x="xScale(i)"
          :y="chartHeight - 6"
          text-anchor="middle"
          fill="var(--color-muted-foreground)"
          font-size="11"
        >
          {{ formatLabel(d.date, i, data.length) }}
        </text>
        <!-- hover highlight -->
        <g v-if="hoveredPoint" pointer-events="none">
          <line
            :x1="hoveredPoint.x"
            :x2="hoveredPoint.x"
            :y1="hoveredPoint.y"
            :y2="chartHeight - padding.bottom"
            stroke="var(--color-chart-2)"
            stroke-width="1"
            stroke-dasharray="3 3"
            opacity="0.45"
          />
          <circle :cx="hoveredPoint.x" :cy="hoveredPoint.y" r="9" fill="var(--color-chart-2)" opacity="0.18" />
          <circle
            :cx="hoveredPoint.x"
            :cy="hoveredPoint.y"
            r="5"
            fill="var(--color-chart-2)"
            stroke="var(--color-foreground)"
            stroke-width="2"
          />
        </g>
      </svg>
      <!-- hover tooltip -->
      <Transition name="tip">
        <div
          v-if="tip && hoveredPoint"
          :class="['trend-tooltip', tip.below ? 'below' : 'above']"
          :style="{ left: tip.left + 'px', top: tip.top + 'px' }"
        >
          <div class="trend-tooltip-date">{{ formatDate(hoveredPoint.d.date) }}</div>
          <div class="trend-tooltip-row">
            <span class="trend-tooltip-dot pv" />访问<b>{{ hoveredPoint.d.pv }}</b>
          </div>
          <div class="trend-tooltip-row">
            <span class="trend-tooltip-dot uv" />访客<b>{{ hoveredPoint.d.uv }}</b>
          </div>
        </div>
      </Transition>
    </div>
  </div>
</template>

<style scoped>
.trend-header {
  margin-bottom: 24px;
}
.trend-subtitle {
  color: var(--color-muted-foreground);
  font-size: 13px;
  margin-top: 4px;
}

.trend-stats {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
  margin-bottom: 24px;
}
.trend-stat-label {
  color: var(--color-muted-foreground);
  font-size: 12px;
  margin-bottom: 4px;
}
.trend-stat-value {
  color: var(--color-foreground);
  font-size: 20px;
  font-weight: 700;
  letter-spacing: -0.02em;
}

.trend-chart-box {
  height: 220px;
}
.trend-chart-svg {
  height: 220px;
  display: block;
  touch-action: pan-y;
}

/* tooltip 覆盖层 */
.trend-chart-wrap {
  position: relative;
}
.trend-tooltip {
  position: absolute;
  z-index: 5;
  pointer-events: none;
  min-width: 104px;
  padding: 8px 12px;
  border-radius: 12px;
  background: color-mix(in srgb, var(--color-card) 72%, transparent);
  -webkit-backdrop-filter: blur(20px) saturate(180%);
  backdrop-filter: blur(20px) saturate(180%);
  border: 1px solid color-mix(in srgb, #fff 12%, transparent);
  box-shadow: var(--shadow-lg);
}
.trend-tooltip.above {
  transform: translate(-50%, -100%);
}
.trend-tooltip.below {
  transform: translate(-50%, 0);
}
.trend-tooltip-date {
  color: var(--color-muted-foreground);
  font-size: 11px;
  margin-bottom: 4px;
  text-align: center;
}
.trend-tooltip-row {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--color-foreground);
  font-size: 12px;
  line-height: 18px;
  white-space: nowrap;
}
.trend-tooltip-row b {
  margin-left: auto;
  padding-left: 12px;
  font-weight: 600;
  color: var(--color-foreground);
}
.trend-tooltip-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}
.trend-tooltip-dot.pv {
  background: var(--color-chart-2);
}
.trend-tooltip-dot.uv {
  background: var(--color-chart-1);
}

.tip-enter-active,
.tip-leave-active {
  transition: opacity var(--duration-fast) var(--ease-standard);
}
.tip-enter-from,
.tip-leave-to {
  opacity: 0;
}

/* M 断点 */
.apple-card:is([data-bp="md"], [data-bp="sm"]) .trend-header,
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .trend-header {
  margin-bottom: 18px;
}
.apple-card:is([data-bp="md"], [data-bp="sm"]) .trend-subtitle,
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .trend-subtitle {
  font-size: 12px;
}
.apple-card:is([data-bp="md"], [data-bp="sm"]) .trend-stats,
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .trend-stats {
  gap: 12px;
  margin-bottom: 18px;
}
.apple-card:is([data-bp="md"], [data-bp="sm"]) .trend-stat-value,
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .trend-stat-value {
  font-size: 18px;
}
.apple-card:is([data-bp="md"], [data-bp="sm"]) .trend-chart-box,
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .trend-chart-box {
  height: 180px;
}
.apple-card:is([data-bp="md"], [data-bp="sm"]) .trend-chart-svg,
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .trend-chart-svg {
  height: 180px;
}

/* S 断点 */
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .trend-header {
  flex-direction: column;
  align-items: stretch !important;
  gap: 10px;
  margin-bottom: 14px;
}
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .trend-header .apple-segmented {
  align-self: flex-start;
}
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .trend-stats {
  gap: 8px;
  margin-bottom: 14px;
}
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .trend-stat-label {
  font-size: 10px;
  margin-bottom: 2px;
}
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .trend-stat-value {
  font-size: 15px;
}
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .trend-chart-box {
  height: 150px;
}
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .trend-chart-svg {
  height: 150px;
}
</style>
