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

/** 图表内容区 SVG 坐标高度；实际显示高度由 flex 撑满卡片，preserveAspectRatio="none" 负责纵向伸缩。 */
const PLOT_HEIGHT = 120;

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

const peakItem = computed(() =>
  data.value.reduce<DailyStat | null>((best, item) => (!best || item.pv > best.pv ? item : best), null),
);

const totals = computed(() => {
  const pv = data.value.reduce((s, d) => s + d.pv, 0);
  const uv = data.value.reduce((s, d) => s + d.uv, 0);
  const activeDays = data.value.filter((item) => item.pv > 0).length;
  // 日均只摊在有访问的日子上；把空档算进分母会让长期停更的站点读数失真。
  const avgPv = activeDays ? Math.round(pv / activeDays) : 0;
  const peak = peakItem.value ? peakItem.value.pv : 0;
  return { pv, uv, avgPv, peak, activeDays, span: data.value.length };
});

/** 峰值日与首日的对比：说明窗口内是走高还是走平，不再只给一个孤立峰值。 */
const trendShape = computed(() => {
  if (data.value.length < 2) return { text: "样本不足", direction: "flat" as const };
  const first = data.value.find((item) => item.pv > 0);
  const last = data.value[data.value.length - 1];
  if (!first || last.pv === 0) return { text: "近期无访问", direction: "flat" as const };
  const delta = last.pv - first.pv;
  const ratio = first.pv > 0 ? Math.round((delta / first.pv) * 100) : 0;
  if (ratio === 0) return { text: "与起始持平", direction: "flat" as const };
  return {
    text: `${ratio > 0 ? "+" : ""}${ratio}%`,
    direction: ratio > 0 ? ("up" as const) : ("down" as const),
  };
});

/** 图表布局：内容区（面板底）与轴标签各占 SVG 的一段高度，纵向拉伸不影响比例。 */
const plot = computed(() => {
  return {
    height: PLOT_HEIGHT,
    width: 720,
    gridRows: sizePreset.value === "small" ? 2 : 4,
    // 折线两端留出描边半径，末点记号与峰谷才不会被卡片边缘切掉。
    inset: 5,
    top: 6,
    bottom: PLOT_HEIGHT - 6,
  };
});

const xScale = computed(() => {
  const n = data.value.length;
  const { inset, width } = plot.value;
  if (n <= 1) return () => width - inset;
  const span = width - inset * 2;
  return (i: number) => inset + (i / (n - 1)) * span;
});

const maxVal = computed(() => {
  const peak = Math.max(...data.value.map((d) => d.pv), 0);
  if (peak === 0) return 1;
  // 以 0 为基线并留出顶部呼吸空间，保持数据比例诚实。
  return Math.max(peak * 1.12, peak + 1);
});

const yScale = computed(() => {
  const max = maxVal.value;
  const top = plot.value.top;
  const height = plot.value.bottom - top;
  return (v: number) => top + height - (v / max) * height;
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
  const pts = data.value.map((d, i) => ({ x: xScale.value(i), y: yScale.value(d.pv) }));
  const spline = getShapePreservingSpline(pts);
  const lastX = xScale.value(data.value.length - 1);
  const firstX = xScale.value(0);
  const bottomY = plot.value.bottom;
  return `${spline} L${lastX},${bottomY} L${firstX},${bottomY} Z`;
});

const linePath = computed(() => {
  if (data.value.length === 0) return "";
  return getShapePreservingSpline(data.value.map((d, i) => ({ x: xScale.value(i), y: yScale.value(d.pv) })));
});

function pathFor(key: "pv" | "uv"): string {
  if (data.value.length === 0) return "";
  return getShapePreservingSpline(data.value.map((d, i) => ({ x: xScale.value(i), y: yScale.value(d[key]) })));
}

const uvLinePath = computed(() => pathFor("uv"));

function formatNumber(n: number): string {
  if (n >= 10000) return (n / 10000).toFixed(1) + "w";
  if (n >= 1000) return (n / 1000).toFixed(1) + "k";
  return n.toString();
}

function formatDate(dateStr: string): string {
  const d = new Date(dateStr);
  return `${d.getMonth() + 1}/${d.getDate()}`;
}

/** 横轴标签在 SVG 内渲染：坐标与绘图区同源，窄卡片也不会因百分比换算错位。 */
function axisTicks(): { key: string; left: number; label: string }[] {
  const n = data.value.length;
  if (n === 0) return [];
  const maxTicks = sizePreset.value === "small" ? 3 : n <= 7 ? n : 6;
  const count = Math.min(n, maxTicks);
  const indices = new Set<number>();
  if (count === 1) indices.add(n - 1);
  else for (let k = 0; k < count; k++) indices.add(Math.round((k / (count - 1)) * (n - 1)));
  return [...indices]
    .sort((a, b) => a - b)
    .map((i) => ({
      key: data.value[i].date,
      // 内缩后的绘图区换算成百分比；首尾标签用 CSS 改锚点贴住两端。
      left: (xScale.value(i) / plot.value.width) * 100,
      label: formatDate(data.value[i].date),
    }));
}

const ticks = computed(() => axisTicks());

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
  const { inset, width } = plot.value;
  const span = Math.max(width - inset * 2, 1);
  const idx = Math.round(((p.x - inset) / span) * (n - 1));
  hovered.value = Math.min(Math.max(idx, 0), n - 1);
  updateTip();
}

function onChartLeave() {
  hovered.value = null;
  tip.value = null;
}

/** 接口会对无数据的日子补零，因此「有序列」不等于「有访问」；全零时按空态处理。 */
const hasTraffic = computed(() => data.value.some((item) => item.pv > 0));

const showUvLine = computed(() => sizePreset.value === "large" || (sizePreset.value === "medium" && showUv.value));
</script>

<template>
  <div class="apple-card trend-card h-full flex flex-col" :class="`trend-card-${sizePreset}`" :data-bp="breakpoint">
    <!-- Small：标题与总量压成一行，图表占满余下空间 -->
    <div v-if="sizePreset === 'small'" class="trend-small-head">
      <div class="flex items-baseline gap-2 min-w-0">
        <h3 class="apple-section-title">访问趋势</h3>
        <span class="trend-range">{{ days }} 天</span>
      </div>
      <div class="text-[12px] text-text-secondary tabular-nums truncate">
        PV {{ formatNumber(totals.pv) }}<span class="trend-head-divider">/</span>UV {{ formatNumber(totals.uv) }}
      </div>
    </div>

    <!-- Medium / Large：标题行（图例 + 时间范围）与一行事实指标 -->
    <template v-else>
      <div class="trend-header">
        <div class="trend-heading">
          <h3 class="apple-section-title">访问趋势</h3>
          <p class="trend-subtitle">浏览量与独立访客</p>
        </div>
        <div class="trend-actions">
          <div class="trend-legend">
            <button
              type="button"
              class="trend-legend-toggle"
              :class="{ active: !showUvLine }"
              :aria-pressed="!showUvLine"
              @click="showUv = false"
            >
              <span class="trend-legend-line pv" />PV
            </button>
            <button
              v-if="sizePreset === 'medium'"
              type="button"
              class="trend-legend-toggle"
              :class="{ active: showUv }"
              :aria-pressed="showUv"
              @click="showUv = !showUv"
            >
              <span class="trend-legend-line uv" />UV
            </button>
            <span v-else class="trend-legend-static"><span class="trend-legend-line uv" />UV</span>
          </div>
          <div class="apple-segmented">
            <button :class="['seg-btn', days === 7 && 'active']" @click="days = 7">7 天</button>
            <button :class="['seg-btn', days === 30 && 'active']" @click="days = 30">30 天</button>
          </div>
        </div>
      </div>

      <!-- 中档：一行紧凑总量，代替占两行的事实网格 -->
      <div v-if="sizePreset === 'medium'" class="trend-inline-totals">
        <span>PV <b class="tabular-nums">{{ formatNumber(totals.pv) }}</b></span>
        <span>UV <b class="tabular-nums">{{ formatNumber(totals.uv) }}</b></span>
        <span>日均 <b class="tabular-nums">{{ formatNumber(totals.avgPv) }}</b></span>
      </div>

      <!-- 事实行：数字与细线，不内嵌小卡 -->
      <div class="trend-facts" :class="{ 'is-wide': sizePreset === 'large' }">
        <div class="trend-fact">
          <span class="trend-fact-label">总访问量</span>
          <span class="trend-fact-value tabular-nums">{{ formatNumber(totals.pv) }}</span>
          <span class="trend-fact-note">PV</span>
        </div>
        <div class="trend-fact">
          <span class="trend-fact-label">独立访客</span>
          <span class="trend-fact-value tabular-nums">{{ formatNumber(totals.uv) }}</span>
          <span class="trend-fact-note">UV</span>
        </div>
        <div class="trend-fact">
          <span class="trend-fact-label">日均访问</span>
          <span class="trend-fact-value tabular-nums">{{ formatNumber(totals.avgPv) }}</span>
          <span class="trend-fact-note">活跃日</span>
        </div>
        <div v-if="sizePreset === 'large'" class="trend-fact">
          <span class="trend-fact-label">峰值单日</span>
          <span class="trend-fact-value tabular-nums">{{ formatNumber(totals.peak) }}</span>
          <span class="trend-fact-note">{{ peakItem ? formatDate(peakItem.date) : "--" }}</span>
        </div>
        <div class="trend-fact trend-fact-shape">
          <span class="trend-fact-label">活跃天数</span>
          <span class="trend-fact-value tabular-nums">{{ totals.activeDays }}</span>
          <span class="trend-fact-note" :class="`is-${trendShape.direction}`">{{ trendShape.text }}</span>
        </div>
      </div>
    </template>

    <div v-if="loading" class="trend-chart-wrap trend-chart-skeleton" aria-label="正在加载趋势">
      <span
        v-for="(height, i) in [42, 58, 35, 68, 52, 76, 46]"
        :key="i"
        class="trend-skeleton-bar"
        :style="{ height: `${height}%` }"
      />
    </div>
    <div v-else-if="error" class="trend-chart-wrap trend-chart-state text-state-error">{{ error }}</div>
    <div v-else-if="!hasTraffic" class="trend-chart-wrap trend-empty" aria-label="暂无趋势数据">
      <Icon icon="ri:line-chart-line" class="w-5 h-5" />
      <span>近 {{ days }} 天暂无访问</span>
    </div>
    <div v-else ref="wrapEl" class="trend-chart-wrap">
      <svg
        ref="chartEl"
        :viewBox="`0 0 ${plot.width} ${plot.height}`"
        class="trend-chart-svg"
        preserveAspectRatio="none"
        role="img"
        aria-label="访问趋势图"
        @pointermove="onChartMove"
        @pointerdown="onChartMove"
        @pointerleave="onChartLeave"
      >
        <defs>
          <linearGradient :id="`${chartUid}-area`" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stop-color="var(--color-primary)" stop-opacity="0.22" />
            <stop offset="70%" stop-color="var(--color-primary)" stop-opacity="0.05" />
            <stop offset="100%" stop-color="var(--color-primary)" stop-opacity="0" />
          </linearGradient>
          <linearGradient :id="`${chartUid}-line`" x1="0" y1="0" x2="1" y2="0">
            <stop offset="0%" stop-color="var(--color-primary-active)" />
            <stop offset="55%" stop-color="var(--color-primary)" />
            <stop offset="100%" stop-color="var(--color-primary-hover)" />
          </linearGradient>
        </defs>

        <!-- 横向参考线：只留最淡的一层，不与折线争视线 -->
        <line
          v-for="i in plot.gridRows"
          :key="`grid-${i}`"
          :x1="plot.inset"
          :x2="plot.width - plot.inset"
          :y1="plot.top + ((plot.bottom - plot.top) / plot.gridRows) * i"
          :y2="plot.top + ((plot.bottom - plot.top) / plot.gridRows) * i"
          stroke="var(--color-chart-grid)"
          stroke-width="1"
          stroke-dasharray="3 5"
          opacity="0.5"
        />

        <path :d="areaPath" :fill="`url(#${chartUid}-area)`" class="trend-area-path" />
        <path
          :d="linePath"
          fill="none"
          :stroke="`url(#${chartUid}-line)`"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          class="trend-line-path"
        />
        <path
          v-if="showUvLine"
          :d="uvLinePath"
          fill="none"
          stroke="var(--color-chart-1)"
          stroke-width="1.5"
          stroke-dasharray="4 4"
          stroke-linecap="round"
          opacity="0.85"
          class="trend-line-path"
        />

        <!-- 常态只给末点一个记号，其余靠折线本身表达 -->
        <circle
          v-if="data.length"
          :cx="xScale(data.length - 1)"
          :cy="yScale(data[data.length - 1].pv)"
          r="3"
          fill="var(--color-primary)"
          stroke="var(--color-background)"
          stroke-width="2"
        />

        <g v-if="hoveredPoint" pointer-events="none">
          <line
            :x1="hoveredPoint.x"
            :x2="hoveredPoint.x"
            :y1="hoveredPoint.y"
            :y2="plot.bottom"
            stroke="var(--color-text-tertiary)"
            stroke-width="1"
            stroke-dasharray="2 4"
          />
          <circle
            :cx="hoveredPoint.x"
            :cy="hoveredPoint.y"
            r="4"
            fill="var(--color-primary)"
            stroke="var(--color-background)"
            stroke-width="2"
          />
        </g>

        <!-- 横轴标签由 HTML 渲染：SVG 纵向拉伸会等比压扁文字，标签必须留在 DOM 层 -->
      </svg>

      <div class="trend-axis">
        <span
          v-for="(tick, i) in ticks"
          :key="tick.key"
          :class="{ 'is-first': i === 0, 'is-last': i === ticks.length - 1 }"
          :style="{ left: `${tick.left}%` }"
        >
          {{ tick.label }}
        </span>
      </div>

      <Transition name="tip">
        <div
          v-if="tip && hoveredPoint"
          :class="['trend-tooltip', tip.below ? 'below' : 'above']"
          :style="{ left: tip.left + 'px', top: tip.top + 'px' }"
        >
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

    <!-- Small：底部一行结论，替代原先三条等宽小项 -->
    <div v-if="sizePreset === 'small' && !loading && !error && hasTraffic" class="trend-footnote">
      <span>活跃 {{ totals.activeDays }}/{{ totals.span }} 天</span>
      <span class="trend-head-divider">·</span>
      <span>日均 {{ formatNumber(totals.avgPv) }}</span>
      <span v-if="peakItem" class="trend-footnote-peak">峰值 {{ formatDate(peakItem.date) }}</span>
    </div>

    <!-- 读屏器与无障碍数据表格摘要 -->
    <div class="sr-only">
      <h4>访问趋势数据明细</h4>
      <p>
        时间范围：近 {{ days }} 天。总浏览量：{{ totals.pv }}，独立访客：{{ totals.uv }}，日均访问：{{ totals.avgPv }}。
      </p>
      <table>
        <caption>
          每日访问数据
        </caption>
        <thead>
          <tr>
            <th scope="col">日期</th>
            <th scope="col">浏览量 (PV)</th>
            <th scope="col">独立访客 (UV)</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in data" :key="row.date">
            <td>{{ row.date }}</td>
            <td>{{ row.pv }}</td>
            <td>{{ row.uv }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
/* 单层节奏：标题行 → 事实行 → 图表 → （小尺寸）结论行，不再卡中卡。 */
.trend-card {
  gap: 0;
}

.trend-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}
.trend-heading {
  min-width: 0;
}
.trend-subtitle {
  margin-top: 2px;
  color: var(--color-muted-foreground);
  font-size: 12px;
}
.trend-actions {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-shrink: 0;
}
.trend-legend {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 11px;
  color: var(--color-muted-foreground);
}
.trend-legend-toggle,
.trend-legend-static {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: 11px;
  letter-spacing: 0;
  cursor: pointer;
  box-shadow: inset 0 -1px 0 transparent;
}
.trend-legend-static {
  cursor: default;
}
.trend-legend-toggle:hover,
.trend-legend-toggle.active {
  color: var(--color-foreground);
}
.trend-legend-toggle.active {
  box-shadow: inset 0 -1px 0 var(--color-foreground);
}
.trend-legend-toggle:focus-visible {
  outline: 2px solid var(--color-ring);
  outline-offset: 2px;
}
.trend-legend-line {
  display: inline-block;
  width: 12px;
  height: 2px;
  background: var(--color-primary);
}
.trend-legend-line.uv {
  background: repeating-linear-gradient(90deg, var(--color-chart-1) 0 4px, transparent 4px 7px);
}

/* 中档 28 行盒子只有 168px 内容高度：事实网格让位给一行总量 + 图表。 */
.trend-inline-totals {
  display: flex;
  align-items: baseline;
  gap: 16px;
  margin-top: 6px;
  color: var(--color-text-tertiary);
  font-size: 11px;
}
.trend-inline-totals span {
  white-space: nowrap;
}
.trend-inline-totals b {
  color: var(--color-foreground);
  font-size: 13px;
  font-weight: 600;
}
.trend-card-medium .trend-facts {
  display: none;
}
.trend-card-medium .trend-chart-svg {
  min-height: 64px;
}
.trend-card-medium .trend-chart-wrap {
  margin-top: 8px;
}

/* 事实行：小标签 + 数值，一行一组，靠留白而不是边框分组 */
.trend-facts {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 4px 24px;
  margin-top: 14px;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--color-border);
}
.trend-facts.is-wide {
  grid-template-columns: repeat(5, minmax(0, 1fr));
}
.trend-fact {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 2px 6px;
  min-width: 0;
}
.trend-fact-label {
  flex: 0 0 100%;
  color: var(--color-text-tertiary);
  font-size: 11px;
  line-height: 1.4;
  white-space: nowrap;
}
.trend-fact-value {
  color: var(--color-foreground);
  font-size: 20px;
  font-weight: 600;
  line-height: 1.2;
  letter-spacing: -0.02em;
}
.trend-fact-note {
  color: var(--color-text-quaternary);
  font-size: 11px;
  white-space: nowrap;
}
.trend-fact-note.is-up {
  color: var(--color-state-success);
}
.trend-fact-note.is-down {
  color: var(--color-state-warning);
}

/* 图表区：吃掉卡片剩余高度 */
.trend-chart-wrap {
  position: relative;
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  min-height: 0;
  margin-top: 12px;
}
.trend-chart-svg {
  display: block;
  flex: 1 1 auto;
  width: 100%;
  /* 图表至少要撑住曲线形态；卡片高度富余时才继续拉伸。 */
  min-height: 96px;
  touch-action: pan-y;
}
/* 横轴留在 HTML 层：SVG 的 preserveAspectRatio="none" 会把内部文字压扁。 */
.trend-axis {
  position: relative;
  flex: 0 0 auto;
  height: 16px;
  margin-top: 6px;
  color: var(--color-text-quaternary);
  font-family: var(--font-mono);
  font-size: 11px;
  line-height: 16px;
}
.trend-axis span {
  position: absolute;
  top: 0;
  transform: translateX(-50%);
  white-space: nowrap;
}
/* 首尾标签贴住两端，避免半个字被卡片边缘裁掉。 */
.trend-axis span.is-first {
  transform: translateX(0);
}
.trend-axis span.is-last {
  transform: translateX(-100%);
}
.trend-chart-skeleton {
  display: flex;
  align-items: flex-end;
  justify-content: space-around;
  gap: 12px;
  padding: 18px 8px 0;
  overflow: hidden;
}
.trend-skeleton-bar {
  width: 8%;
  min-height: 12px;
  background: color-mix(in srgb, var(--color-primary) 10%, transparent);
}
.trend-chart-state {
  align-items: center;
  justify-content: center;
  color: var(--color-state-error);
  font-size: 12px;
}
.trend-empty {
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--color-muted-foreground);
  font-size: 12px;
}

.trend-area-path,
.trend-line-path {
  animation: trend-chart-reveal var(--duration-slow) var(--ease-standard) both;
}
@keyframes trend-chart-reveal {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}

/* 小档 116px：标题与总量一行，图表吃掉余下高度；坐标轴与结论行让位。 */
.trend-card-small .trend-chart-wrap {
  margin-top: 6px;
}
.trend-card-small .trend-chart-svg {
  min-height: 40px;
}
.trend-card-small .trend-axis,
.trend-card-small .trend-footnote {
  display: none;
}

/* Small：一行标题 + 一行结论 */
.trend-small-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
}
.trend-range {
  color: var(--color-text-tertiary);
  font-size: 11px;
  white-space: nowrap;
}
.trend-head-divider {
  margin: 0 4px;
  color: var(--color-text-quaternary);
}
.trend-footnote {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid var(--color-border);
  color: var(--color-text-tertiary);
  font-size: 11px;
  white-space: nowrap;
  overflow: hidden;
}
.trend-footnote-peak {
  margin-left: auto;
  color: var(--color-text-secondary);
}

/* tooltip 覆盖层 */
.trend-tooltip {
  position: absolute;
  z-index: 20;
  pointer-events: none;
  min-width: 120px;
  padding: 8px 10px;
  border-radius: var(--radius-2xl);
  background: var(--color-popover);
  border: 1px solid var(--color-border-strong);
  box-shadow: var(--shadow-lg);
}
.trend-tooltip.above {
  transform: translate(-50%, -100%);
}
.trend-tooltip.below {
  transform: translate(-50%, 0);
}
.trend-tooltip-date {
  margin-bottom: 4px;
  padding-bottom: 4px;
  border-bottom: 1px solid var(--color-border);
  color: var(--color-text-tertiary);
  font-family: var(--font-mono);
  font-size: 11px;
  text-align: center;
}
.trend-tooltip-row {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--color-foreground);
  font-size: 12px;
  line-height: 20px;
  white-space: nowrap;
}
.trend-tooltip-row b {
  margin-left: auto;
  padding-left: 12px;
  font-weight: 600;
}
.trend-tooltip-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
}
/* PV 折线使用 primary 渐变，提示点必须同色，避免图例与线色不一致。 */
.trend-tooltip-dot.pv {
  background: var(--color-primary);
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

/* sm / xxs 断点：标题与操作分行，事实行改行内紧凑排版，把高度留给图表 */
.apple-card:is([data-bp="sm"], [data-bp="xs"], [data-bp="xxs"]) .trend-header {
  flex-direction: column;
  align-items: stretch;
  gap: 8px;
}
.apple-card:is([data-bp="sm"], [data-bp="xs"], [data-bp="xxs"]) .trend-actions {
  justify-content: space-between;
  gap: 12px;
}
.apple-card:is([data-bp="sm"], [data-bp="xs"], [data-bp="xxs"]) .trend-facts,
.apple-card:is([data-bp="sm"], [data-bp="xs"], [data-bp="xxs"]) .trend-facts.is-wide {
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 2px 16px;
  margin-top: 10px;
  padding-bottom: 8px;
}
/* 窄屏把标签收回数值左边：两行而不是四行，读数仍然有名字。 */
.apple-card:is([data-bp="sm"], [data-bp="xs"], [data-bp="xxs"]) .trend-fact {
  gap: 6px;
  align-items: baseline;
}
.apple-card:is([data-bp="sm"], [data-bp="xs"], [data-bp="xxs"]) .trend-fact-label {
  flex: 0 1 auto;
  font-size: 11px;
}
.apple-card:is([data-bp="sm"], [data-bp="xs"], [data-bp="xxs"]) .trend-fact-value {
  font-size: 14px;
  letter-spacing: 0;
}
/* 平板/手机：标题与图例换行后，图表最小高度必须让位，否则会撑破盒子。 */
.apple-card:is([data-bp="sm"], [data-bp="xs"], [data-bp="xxs"]) .trend-chart-svg {
  min-height: 40px;
}
/* 最窄档省掉与标题重复的副标题，图表多出一行高度。 */
.apple-card[data-bp="xxs"] .trend-subtitle {
  display: none;
}
.apple-card[data-bp="xxs"] .trend-chart-wrap {
  margin-top: 10px;
}
</style>
