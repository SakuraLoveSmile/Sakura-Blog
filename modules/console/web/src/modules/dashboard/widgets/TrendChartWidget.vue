<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import { Icon } from "@iconify/vue";
import { useBreakpoint } from "../useBreakpoint";
import { statsApi } from "@/api";
import type { WidgetSizePreset } from "./layout";
import { useWidgetSize } from "../useWidgetSize";
import { DASHBOARD_REFRESH_EVENT } from "./useDashboardStats";

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
const chartUid = `trend-${Math.random().toString(36).slice(2, 10)}`;

const days = ref<7 | 30>(7);
const data = ref<DailyStat[]>([]);
const loading = ref(true);
const error = ref("");
const showUv = ref(false);

function fetchTrend(d: number) {
  loading.value = true;
  error.value = "";
  // Spec 62：裸 fetch 改走 statsApi（统一 axios 拦截器：401 跳登录/超时/错误结构）
  statsApi
    .trend(d)
    .then((resp: { data: { trend: DailyStat[] } }) => {
      data.value = resp.data.trend;
    })
    .catch((e) => (error.value = e instanceof Error ? e.message : "趋势数据加载失败"))
    .finally(() => (loading.value = false));
}

const handleRefresh = () => fetchTrend(days.value);

onMounted(() => {
  fetchTrend(days.value);
  window.addEventListener(DASHBOARD_REFRESH_EVENT, handleRefresh);
});
onUnmounted(() => window.removeEventListener(DASHBOARD_REFRESH_EVENT, handleRefresh));
watch(days, (d) => fetchTrend(d));

const totals = computed(() => {
  const pv = data.value.reduce((s, d) => s + d.pv, 0);
  const uv = data.value.reduce((s, d) => s + d.uv, 0);
  const avgPv = data.value.length ? Math.round(pv / data.value.length) : 0;
  const peak = data.value.reduce((m, d) => Math.max(m, d.pv), 0);
  const peakItem = data.value.find((d) => d.pv === peak);
  return { pv, uv, avgPv, peak, peakDate: peakItem ? peakItem.date : "" };
});

const insights = computed(() => ({
  peakDate: totals.value.peakDate ? formatDate(totals.value.peakDate) : "--",
  activeDays: `${data.value.filter((item) => item.pv > 0).length}/${data.value.length || 0}`,
  uvShare: totals.value.pv > 0 ? `${Math.round((totals.value.uv / totals.value.pv) * 100)}%` : "--",
}));

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
  <div class="apple-card h-full flex flex-col justify-between" :class="`trend-card-${sizePreset}`" :data-bp="breakpoint">
    <!-- Small 尺寸顶部精简条 -->
    <div v-if="sizePreset === 'small'" class="flex items-center justify-between mb-2">
      <div class="flex items-center gap-2">
        <h3 class="apple-section-title text-sm">访问趋势</h3>
        <span class="text-[11px] text-muted-foreground">近 7 天</span>
      </div>
      <div class="text-xs font-semibold text-primary tabular-nums">
        总浏览 {{ formatNumber(totals.pv) }}
      </div>
    </div>

    <!-- Medium / Large 尺寸顶部标准标题与分段器 -->
    <div v-else class="flex items-center justify-between trend-header">
      <div>
        <h3 class="apple-section-title">{{ sizePreset === 'large' ? '流量深度分析与访问趋势' : '访问趋势' }}</h3>
        <p class="trend-subtitle">{{ sizePreset === 'large' ? 'PV / UV 双指标趋势与转化监测' : 'Traffic Overview' }}</p>
      </div>
      <div class="flex items-center gap-2">
        <!-- Medium / Large 尺寸图例：Medium 可切换 UV 辅助线 -->
        <div class="hidden sm:flex items-center gap-2 mr-1 text-xs text-muted-foreground">
          <button type="button" class="trend-legend-toggle" :class="{ active: !showUv || sizePreset === 'large' }" @click="showUv = false">
            <span class="trend-legend-line pv" /> PV
          </button>
          <button v-if="sizePreset === 'medium'" type="button" class="trend-legend-toggle" :class="{ active: showUv }" :aria-pressed="showUv" @click="showUv = !showUv">
            <span class="trend-legend-line uv" /> UV
          </button>
          <span v-else class="inline-flex items-center gap-1"><span class="trend-legend-line uv" /> UV</span>
        </div>
        <div class="apple-segmented">
          <button :class="['seg-btn', days === 7 && 'active']" @click="days = 7">7 天</button>
          <button :class="['seg-btn', days === 30 && 'active']" @click="days = 30">30 天</button>
        </div>
      </div>
    </div>

    <!-- 指标卡片：Small 隐藏；Medium 3项；Large 4项 -->
    <div v-if="sizePreset !== 'small'" class="trend-stats" :class="{ 'grid-cols-4': sizePreset === 'large', 'grid-cols-3': sizePreset === 'medium' }">
      <div class="trend-stat-card">
        <div class="trend-stat-label">总访问量 (PV)</div>
        <div class="trend-stat-value tabular-nums">{{ formatNumber(totals.pv) }}</div>
      </div>
      <div class="trend-stat-card">
        <div class="trend-stat-label">独立访客 (UV)</div>
        <div class="trend-stat-value text-chart-1 tabular-nums">{{ formatNumber(totals.uv) }}</div>
      </div>
      <div class="trend-stat-card">
        <div class="trend-stat-label">日均访问</div>
        <div class="trend-stat-value tabular-nums">{{ formatNumber(totals.avgPv) }}</div>
      </div>
      <div v-if="sizePreset === 'large'" class="trend-stat-card">
        <div class="trend-stat-label">峰值单日</div>
        <div class="trend-stat-value text-chart-3 tabular-nums">{{ formatNumber(totals.peak) }}</div>
      </div>
    </div>

    <div v-if="loading" class="trend-chart-box trend-chart-skeleton" :style="{ height: `${chartHeight}px` }" aria-label="正在加载趋势">
      <span v-for="height in [42, 58, 35, 68, 52, 76, 46]" :key="height" class="trend-skeleton-bar" :style="{ height: `${height}%` }" />
      <span class="trend-skeleton-line" aria-hidden="true" />
    </div>
    <div v-else-if="error" class="trend-chart-box flex items-center justify-center text-state-error" :style="{ height: `${chartHeight}px` }">{{ error }}</div>
    <div v-else-if="data.length === 0" class="trend-empty flex-1" aria-label="暂无趋势数据">
      <Icon icon="ri:line-chart-line" class="w-6 h-6" />
      <span>暂无趋势数据</span>
    </div>
    <div v-else ref="wrapEl" class="trend-chart-wrap flex-1 min-h-0">
      <svg
        ref="chartEl"
        :viewBox="`0 0 ${chartWidth} ${chartHeight}`"
        class="w-full trend-chart-svg overflow-visible"
        :style="{ height: '100%' }"
        preserveAspectRatio="none"
        role="img"
        aria-label="访问趋势图"
        @pointermove="onChartMove"
        @pointerdown="onChartMove"
        @pointerleave="onChartLeave"
      >
        <defs>
          <linearGradient :id="`${chartUid}-area`" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stop-color="var(--color-primary)" stop-opacity="0.32" />
            <stop offset="60%" stop-color="var(--color-primary)" stop-opacity="0.08" />
            <stop offset="100%" stop-color="var(--color-primary)" stop-opacity="0" />
          </linearGradient>
          <linearGradient :id="`${chartUid}-line`" x1="0" y1="0" x2="1" y2="0">
            <stop offset="0%" stop-color="var(--color-primary-hover)" />
            <stop offset="50%" stop-color="var(--color-primary)" />
            <stop offset="100%" stop-color="var(--color-primary-active)" />
          </linearGradient>
          <filter :id="`${chartUid}-glow`" x="-20%" y="-20%" width="140%" height="140%">
            <feGaussianBlur stdDeviation="3" result="blur" />
            <feComposite in="SourceGraphic" in2="blur" operator="over" />
          </filter>
        </defs>

        <!-- grid lines -->
        <g stroke="var(--color-chart-grid)" stroke-width="1" stroke-dasharray="3 3" opacity="0.6">
          <line
            v-for="i in (sizePreset === 'small' ? 2 : 4)"
            :key="i"
            :x1="padding.left"
            :x2="chartWidth - padding.right"
            :y1="padding.top + ((chartHeight - padding.top - padding.bottom) / (sizePreset === 'small' ? 2 : 4)) * i"
            :y2="padding.top + ((chartHeight - padding.top - padding.bottom) / (sizePreset === 'small' ? 2 : 4)) * i"
          />
        </g>

        <!-- Area fill with smooth cubic spline -->
        <path :d="areaPath" :fill="`url(#${chartUid}-area)`" class="trend-area-path" />

        <!-- Line path with smooth cubic spline -->
        <path :d="linePath" fill="none" stroke="var(--color-primary)" stroke-width="8" stroke-linecap="round" stroke-linejoin="round" opacity="0.18" :filter="`url(#${chartUid}-glow)`" class="trend-line-path" />
        <path :d="linePath" fill="none" :stroke="`url(#${chartUid}-line)`" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" class="trend-line-path" />
        <!-- Large 尺寸渲染 UV 辅助线（chart-1 绿色） -->
        <path
          v-if="sizePreset === 'large' || (sizePreset === 'medium' && showUv)"
          :d="uvLinePath"
          fill="none"
          stroke="var(--color-chart-1)"
          stroke-width="2"
          stroke-dasharray="4 4"
          stroke-linecap="round"
          opacity="0.85"
          class="trend-line-path"
        />

        <!-- data points -->
        <circle
          v-for="(d, i) in data"
          :key="i"
          :cx="xScale(i)"
          :cy="yScale(d.pv)"
          :r="sizePreset === 'small' ? 2.5 : 3.5"
          fill="var(--color-primary)"
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
          font-family="var(--font-mono)"
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
            stroke="var(--color-primary)"
            stroke-width="1.5"
            stroke-dasharray="3 3"
            opacity="0.6"
          />
          <circle :cx="hoveredPoint.x" :cy="hoveredPoint.y" r="10" fill="var(--color-primary)" opacity="0.2" />
          <circle :cx="hoveredPoint.x" :cy="hoveredPoint.y" r="5" fill="var(--color-primary)" stroke="#ffffff" stroke-width="2" />
        </g>
      </svg>

      <!-- hover tooltip -->
      <Transition name="tip">
        <div v-if="tip && hoveredPoint" :class="['trend-tooltip', tip.below ? 'below' : 'above']" :style="{ left: tip.left + 'px', top: tip.top + 'px' }">
          <div class="trend-tooltip-date">{{ formatDate(hoveredPoint.d.date) }}</div>
          <div class="trend-tooltip-row">
            <span class="trend-tooltip-dot pv" />
            <span>访问 (PV)</span>
            <b class="tabular-nums">{{ hoveredPoint.d.pv.toLocaleString() }}</b>
          </div>
          <div class="trend-tooltip-row">
            <span class="trend-tooltip-dot uv" />
            <span>访客 (UV)</span>
            <b class="tabular-nums">{{ hoveredPoint.d.uv.toLocaleString() }}</b>
          </div>
        </div>
      </Transition>
    </div>

    <div v-if="sizePreset === 'small' && !loading && !error && data.length" class="trend-insight-strip">
      <div class="trend-insight-item">
        <span>峰值日</span>
        <strong class="tabular-nums">{{ insights.peakDate }}</strong>
      </div>
      <div class="trend-insight-item">
        <span>活跃天数</span>
        <strong class="tabular-nums">{{ insights.activeDays }}</strong>
      </div>
      <div class="trend-insight-item">
        <span>UV / PV</span>
        <strong class="tabular-nums">{{ insights.uvShare }}</strong>
      </div>
    </div>
  </div>
</template>

<style scoped>
.trend-header { margin-bottom: 20px; }
.trend-subtitle { color: var(--color-muted-foreground); font-size: 12px; margin-top: 2px; }

.trend-stats {
  display: grid;
  gap: 12px;
  margin-bottom: 20px;
}

.trend-stat-card {
  padding: 10px 14px;
  border-radius: 12px;
  background: color-mix(in srgb, #fff 3%, transparent);
  border: 1px solid color-mix(in srgb, #fff 5%, transparent);
}

.trend-stat-label { color: var(--color-muted-foreground); font-size: 11px; margin-bottom: 3px; }
.trend-stat-value { color: var(--color-foreground); font-size: 19px; font-weight: 700; letter-spacing: -0.02em; }

.trend-chart-box { height: 220px; }
.trend-chart-svg { height: 220px; display: block; touch-action: pan-y; }

.trend-chart-skeleton {
  position: relative;
  display: flex;
  align-items: flex-end;
  justify-content: space-around;
  gap: 12px;
  padding: 18px 8px 26px;
  overflow: hidden;
  border-radius: 14px;
  background: linear-gradient(180deg, color-mix(in srgb, var(--color-primary) 4%, transparent), transparent);
}

.trend-skeleton-bar {
  width: 8%;
  min-height: 12px;
  border-radius: 999px 999px 3px 3px;
  background: color-mix(in srgb, var(--color-primary) 16%, transparent);
}

.trend-skeleton-line {
  position: absolute;
  left: 5%;
  right: 5%;
  bottom: 24%;
  height: 1px;
  background: color-mix(in srgb, var(--color-primary) 25%, transparent);
  box-shadow: 0 0 14px color-mix(in srgb, var(--color-primary) 35%, transparent);
}

/* tooltip 覆盖层 */
.trend-chart-wrap {
  position: relative;
  display: flex;
  flex-direction: column;
  min-height: 0;
}
.trend-chart-wrap .trend-chart-svg {
  flex: 1 1 auto;
  min-height: 0;
}
.trend-area-path,
.trend-line-path {
  animation: trend-chart-reveal 380ms var(--ease-standard) both;
}

.trend-legend-toggle {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 5px;
  border: 1px solid transparent;
  border-radius: 6px;
  color: var(--color-muted-foreground);
  background: transparent;
  font: inherit;
  font-size: 11px;
  cursor: pointer;
}
.trend-legend-toggle:hover,
.trend-legend-toggle.active {
  color: var(--color-foreground);
  border-color: color-mix(in srgb, var(--color-primary) 22%, transparent);
  background: color-mix(in srgb, var(--color-primary) 8%, transparent);
}
.trend-legend-line {
  display: inline-block;
  width: 12px;
  height: 2px;
  border-radius: 99px;
  background: var(--color-primary);
}
.trend-legend-line.uv {
  background: repeating-linear-gradient(90deg, var(--color-chart-1) 0 4px, transparent 4px 7px);
}
.trend-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--color-muted-foreground);
  border: 1px dashed color-mix(in srgb, #fff 10%, transparent);
  border-radius: 14px;
  background: color-mix(in srgb, #fff 2%, transparent);
}
.trend-insight-strip {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid color-mix(in srgb, #fff 7%, transparent);
}
.trend-insight-item {
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
}
.trend-insight-item span {
  color: var(--color-muted-foreground);
  font-size: 10px;
}
.trend-insight-item strong {
  overflow: hidden;
  color: var(--color-foreground);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.trend-tooltip {
  position: absolute;
  z-index: 20;
  pointer-events: none;
  min-width: 120px;
  padding: 8px 12px;
  border-radius: 12px;
  background: color-mix(in srgb, var(--color-card) 85%, transparent);
  -webkit-backdrop-filter: blur(24px) saturate(180%);
  backdrop-filter: blur(24px) saturate(180%);
  border: 1px solid color-mix(in srgb, #fff 14%, transparent);
  box-shadow: 0 8px 24px -4px rgba(0, 0, 0, 0.6);
}
.trend-tooltip.above { transform: translate(-50%, -100%); }
.trend-tooltip.below { transform: translate(-50%, 0); }
.trend-tooltip-date { color: var(--color-muted-foreground); font-size: 11px; font-family: var(--font-mono); margin-bottom: 4px; border-bottom: 1px solid color-mix(in srgb, #fff 6%, transparent); padding-bottom: 4px; text-align: center; }
.trend-tooltip-row {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--color-foreground);
  font-size: 12px;
  line-height: 20px;
  white-space: nowrap;
}
.trend-tooltip-row b { margin-left: auto; padding-left: 12px; font-weight: 650; color: var(--color-foreground); }
.trend-tooltip-dot { width: 7px; height: 7px; border-radius: 50%; }
.trend-tooltip-dot.pv { background: var(--color-chart-2); }
.trend-tooltip-dot.uv { background: var(--color-chart-1); }

.tip-enter-active, .tip-leave-active { transition: opacity var(--duration-fast) var(--ease-standard); }
.tip-enter-from, .tip-leave-to { opacity: 0; }

@keyframes trend-chart-reveal {
  from { opacity: 0; transform: translateY(4px); }
  to { opacity: 1; transform: translateY(0); }
}

/* M 断点 */
.apple-card:is([data-bp="md"], [data-bp="sm"]) .trend-header,
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .trend-header { margin-bottom: 16px; }
.apple-card:is([data-bp="md"], [data-bp="sm"]) .trend-stats,
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .trend-stats { gap: 10px; margin-bottom: 16px; }
.apple-card:is([data-bp="md"], [data-bp="sm"]) .trend-stat-value,
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .trend-stat-value { font-size: 17px; }
.apple-card:is([data-bp="md"], [data-bp="sm"]) .trend-chart-box,
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .trend-chart-box { height: 180px; }
.apple-card:is([data-bp="md"], [data-bp="sm"]) .trend-chart-svg,
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .trend-chart-svg { height: 180px; }

/* S 断点 */
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .trend-header {
  flex-direction: column;
  align-items: stretch !important;
  gap: 10px;
  margin-bottom: 12px;
}
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .trend-header .apple-segmented {
  align-self: flex-start;
}
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .trend-stats { gap: 8px; margin-bottom: 12px; }
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .trend-stat-label { font-size: 10px; margin-bottom: 2px; }
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .trend-stat-value { font-size: 15px; }
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .trend-chart-box { height: 150px; }
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .trend-chart-svg { height: 150px; }
</style>
