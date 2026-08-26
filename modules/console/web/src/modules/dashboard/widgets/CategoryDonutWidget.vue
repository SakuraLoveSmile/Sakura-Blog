<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import { useRouter } from "vue-router";
import { useBreakpoint } from "../useBreakpoint";
import { dashboardApi } from "@/api";
import { themeColors } from "@/lib/theme-colors";
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

const { breakpoint } = useBreakpoint();
const router = useRouter();

interface CatItem {
  name: string;
  count: number;
}

interface CatResp {
  total: number;
  items: CatItem[];
}

const chartColors = themeColors({
  chart2: "#2E8DFF",
  primaryHover: "#66ABFF",
  chart5: "#BF5AF2",
  error: "#FF453A",
  success: "#30D158",
  warning: "#FF9F0A",
  chart4: "#5E5CE6",
  primaryActive: "#007AFF",
});
const COLORS = [
  chartColors.chart2,
  chartColors.primaryHover,
  chartColors.chart5,
  chartColors.error,
  chartColors.success,
  chartColors.warning,
  chartColors.chart4,
  chartColors.primaryActive,
];

const data = ref<CatItem[]>([]);
const loading = ref(true);
const error = ref("");
const hoveredIdx = ref<number | null>(null);

function loadCategories() {
  loading.value = true;
  error.value = "";
  dashboardApi.categories()
    .then((r: { data: CatResp }) => {
      data.value = r.data.items.sort((a, b) => b.count - a.count);
    })
    .catch((e) => (error.value = e instanceof Error ? e.message : "分类加载失败"))
    .finally(() => (loading.value = false));
}

onMounted(() => {
  loadCategories();
  window.addEventListener(DASHBOARD_REFRESH_EVENT, loadCategories);
});
onUnmounted(() => window.removeEventListener(DASHBOARD_REFRESH_EVENT, loadCategories));

const total = computed(() => data.value.reduce((s, d) => s + d.count, 0));

const displayData = computed(() => {
  if (sizePreset.value === "small") return data.value.slice(0, 5);
  if (sizePreset.value === "medium") return data.value.slice(0, 5);
  if (data.value.length > 8) {
    const visible = data.value.slice(0, 7);
    const otherCount = data.value.slice(7).reduce((sum, item) => sum + item.count, 0);
    if (otherCount > 0) visible.push({ name: "其他分类", count: otherCount });
    return visible;
  }
  return data.value;
});

const arcs = computed(() => {
  if (total.value === 0) return [];
  const cx = 90;
  const cy = 90;
  const r = 66;
  const innerR = 46;
  const n = displayData.value.length;
  const padAngle = n > 1 ? 0.04 : 0;
  let startAngle = -Math.PI / 2;
  function makePath(start: number, angle: number): string {
    const end = start + angle;
    const x1 = cx + r * Math.cos(start);
    const y1 = cy + r * Math.sin(start);
    const x2 = cx + r * Math.cos(end);
    const y2 = cy + r * Math.sin(end);
    const ix1 = cx + innerR * Math.cos(end);
    const iy1 = cy + innerR * Math.sin(end);
    const ix2 = cx + innerR * Math.cos(start);
    const iy2 = cy + innerR * Math.sin(start);
    const largeArc = angle > Math.PI ? 1 : 0;
    return `M ${x1} ${y1} A ${r} ${r} 0 ${largeArc} 1 ${x2} ${y2} L ${ix1} ${iy1} A ${innerR} ${innerR} 0 ${largeArc} 0 ${ix2} ${iy2} Z`;
  }

  return displayData.value.map((d, i) => {
    const rawAngle = (d.count / total.value) * Math.PI * 2;
    const angle = Math.max(rawAngle - padAngle, 0.001);
    const path = makePath(startAngle, angle);
    startAngle += rawAngle;
    return {
      path,
      sourceIndex: i,
      color: COLORS[i % COLORS.length],
      pct: Math.round((d.count / total.value) * 100),
      ...d,
    };
  });
});

const arcSegments = computed(() => {
  if (arcs.value.length !== 1) return arcs.value;
  const arc = arcs.value[0];
  const cx = 90;
  const cy = 90;
  const r = 66;
  const innerR = 46;
  const makeHalf = (start: number) => {
    const end = start + Math.PI;
    const x1 = cx + r * Math.cos(start);
    const y1 = cy + r * Math.sin(start);
    const x2 = cx + r * Math.cos(end);
    const y2 = cy + r * Math.sin(end);
    const ix1 = cx + innerR * Math.cos(end);
    const iy1 = cy + innerR * Math.sin(end);
    const ix2 = cx + innerR * Math.cos(start);
    const iy2 = cy + innerR * Math.sin(start);
    return {
      ...arc,
      path: `M ${x1} ${y1} A ${r} ${r} 0 1 1 ${x2} ${y2} L ${ix1} ${iy1} A ${innerR} ${innerR} 0 1 0 ${ix2} ${iy2} Z`,
    };
  };
  return [makeHalf(-Math.PI / 2), makeHalf(Math.PI / 2)];
});

const hoveredArc = computed(() => {
  if (hoveredIdx.value === null || !arcs.value[hoveredIdx.value]) return null;
  return arcs.value[hoveredIdx.value];
});

function openCategory(name: string) {
  router.push({ path: "/posts", query: { category: name } });
}
</script>

<template>
  <div class="apple-card h-full flex flex-col justify-between" :class="`donut-card-${sizePreset}`" :data-bp="breakpoint">
    <div class="donut-header">
      <div class="flex items-center gap-2">
        <h3 class="apple-section-title" :class="{ 'text-sm': sizePreset === 'small' }">分类分布</h3>
        <span class="apple-card-badge">Categories</span>
      </div>
      <span class="text-xs text-muted-foreground">{{ data.length }} 个分类</span>
    </div>

    <div v-if="loading" class="donut-placeholder flex items-center justify-center text-muted-foreground">
      <Icon icon="ri:loader-4-line" class="w-6 h-6 animate-spin" />
    </div>
    <div v-else-if="error" class="donut-placeholder flex items-center justify-center text-state-error">{{ error }}</div>
    <div v-else-if="data.length === 0" class="donut-placeholder flex items-center justify-center text-muted-foreground text-sm">
      暂无分类数据
    </div>

    <!-- Small 尺寸：居中纯环形图（Hover 交互切换中心文案） -->
    <div v-else-if="sizePreset === 'small'" class="flex-1 flex flex-col items-center justify-center">
      <svg viewBox="0 0 180 180" class="w-32 h-32 overflow-visible">
        <g @mouseleave="hoveredIdx = null">
          <path
            v-for="(a, i) in arcSegments"
            :key="`${a.sourceIndex}-${i}`"
            :d="a.path"
            :fill="a.color"
            :opacity="hoveredIdx === null || hoveredIdx === a.sourceIndex ? 1 : 0.35"
            :style="{
              filter: hoveredIdx === a.sourceIndex ? `drop-shadow(0 0 6px ${a.color})` : 'none',
              transform: hoveredIdx === a.sourceIndex ? 'scale(1.03)' : 'scale(1)',
              transformOrigin: '90px 90px',
              transition: 'all 0.2s ease',
            }"
            class="donut-segment"
            @mouseenter="hoveredIdx = a.sourceIndex"
          />
        </g>
        <template v-if="hoveredArc">
          <text x="90" y="84" text-anchor="middle" fill="var(--color-foreground)" class="text-xs font-bold">{{ hoveredArc.name }}</text>
          <text x="90" y="102" text-anchor="middle" :fill="hoveredArc.color" class="text-xs font-semibold tabular-nums">{{ hoveredArc.pct }}% ({{ hoveredArc.count }})</text>
        </template>
        <template v-else>
          <text x="90" y="85" text-anchor="middle" fill="var(--color-foreground)" class="donut-center-num tabular-nums">{{ total }}</text>
          <text x="90" y="105" text-anchor="middle" fill="var(--color-muted-foreground)" class="donut-center-label">篇文章</text>
        </template>
      </svg>
    </div>

    <!-- Medium 尺寸：经典左环右图例 -->
    <div v-else-if="sizePreset === 'medium'" class="donut-body">
      <svg viewBox="0 0 180 180" class="donut-svg overflow-visible">
        <g @mouseleave="hoveredIdx = null">
          <path
            v-for="(a, i) in arcSegments"
            :key="`${a.sourceIndex}-${i}`"
            :d="a.path"
            :fill="a.color"
            :opacity="hoveredIdx === null || hoveredIdx === a.sourceIndex ? 1 : 0.35"
            :style="{
              filter: hoveredIdx === a.sourceIndex ? `drop-shadow(0 0 6px ${a.color})` : 'none',
              transform: hoveredIdx === a.sourceIndex ? 'scale(1.03)' : 'scale(1)',
              transformOrigin: '90px 90px',
              transition: 'all 0.2s ease',
            }"
            class="donut-segment"
            @mouseenter="hoveredIdx = a.sourceIndex"
          />
        </g>
        <template v-if="hoveredArc">
          <text x="90" y="84" text-anchor="middle" fill="var(--color-foreground)" class="text-xs font-bold">{{ hoveredArc.name }}</text>
          <text x="90" y="102" text-anchor="middle" :fill="hoveredArc.color" class="text-xs font-semibold tabular-nums">{{ hoveredArc.pct }}%</text>
        </template>
        <template v-else>
          <text x="90" y="85" text-anchor="middle" fill="var(--color-foreground)" class="donut-center-num tabular-nums">{{ total }}</text>
          <text x="90" y="105" text-anchor="middle" fill="var(--color-muted-foreground)" class="donut-center-label">篇文章</text>
        </template>
      </svg>
      <div class="donut-legend">
        <div
          v-for="(a, i) in arcs"
          :key="i"
          class="donut-legend-item cursor-pointer"
          :class="hoveredIdx === i && 'hovered'"
          role="button"
          tabindex="0"
          :aria-label="`查看分类：${a.name}`"
          @mouseenter="hoveredIdx = i"
          @mouseleave="hoveredIdx = null"
          @click="a.name !== '其他分类' && openCategory(a.name)"
          @focus="hoveredIdx = i"
          @blur="hoveredIdx = null"
          @keydown.enter="a.name !== '其他分类' && openCategory(a.name)"
          @keydown.space.prevent="a.name !== '其他分类' && openCategory(a.name)"
        >
          <span class="donut-dot" :style="{ backgroundColor: a.color, boxShadow: hoveredIdx === i ? `0 0 6px ${a.color}` : 'none' }" />
          <span class="donut-name">{{ a.name }}</span>
          <span class="donut-count tabular-nums">{{ a.count }}</span>
          <span class="donut-pct tabular-nums">{{ a.pct }}%</span>
        </div>
      </div>
    </div>

    <!-- Large 尺寸：大环形图 + 带占比进度条的完整分类明细 -->
    <div v-else class="donut-body gap-6">
      <svg viewBox="0 0 180 180" class="w-36 h-36 flex-shrink-0 overflow-visible">
        <g @mouseleave="hoveredIdx = null">
          <path
            v-for="(a, i) in arcSegments"
            :key="`${a.sourceIndex}-${i}`"
            :d="a.path"
            :fill="a.color"
            :opacity="hoveredIdx === null || hoveredIdx === a.sourceIndex ? 1 : 0.35"
            :style="{
              filter: hoveredIdx === a.sourceIndex ? `drop-shadow(0 0 6px ${a.color})` : 'none',
              transform: hoveredIdx === a.sourceIndex ? 'scale(1.03)' : 'scale(1)',
              transformOrigin: '90px 90px',
              transition: 'all 0.2s ease',
            }"
            class="donut-segment"
            @mouseenter="hoveredIdx = a.sourceIndex"
          />
        </g>
        <template v-if="hoveredArc">
          <text x="90" y="84" text-anchor="middle" fill="var(--color-foreground)" class="text-xs font-bold">{{ hoveredArc.name }}</text>
          <text x="90" y="102" text-anchor="middle" :fill="hoveredArc.color" class="text-xs font-semibold tabular-nums">{{ hoveredArc.pct }}%</text>
        </template>
        <template v-else>
          <text x="90" y="85" text-anchor="middle" fill="var(--color-foreground)" class="donut-center-num tabular-nums">{{ total }}</text>
          <text x="90" y="105" text-anchor="middle" fill="var(--color-muted-foreground)" class="donut-center-label">总计文章</text>
        </template>
      </svg>
      <div class="flex-1 min-w-0 space-y-2 max-h-52 overflow-y-auto pr-1">
        <div
          v-for="(a, i) in arcs"
          :key="i"
          class="p-2 rounded-xl bg-white/[0.03] border border-white/[0.05] hover:bg-white/[0.07] transition-all cursor-pointer"
          :class="hoveredIdx === i && 'ring-1 ring-primary/50 border-primary/30'"
          role="button"
          tabindex="0"
          :aria-label="`查看分类：${a.name}`"
            @mouseenter="hoveredIdx = a.sourceIndex"
          @mouseleave="hoveredIdx = null"
          @click="a.name !== '其他分类' && openCategory(a.name)"
          @keydown.enter="a.name !== '其他分类' && openCategory(a.name)"
        >
          <div class="flex items-center justify-between text-xs mb-1.5">
            <span class="font-medium text-foreground flex items-center gap-1.5 truncate">
              <span class="w-2 h-2 rounded-full flex-shrink-0" :style="{ backgroundColor: a.color, boxShadow: `0 0 5px ${a.color}` }" />
              <span class="truncate">{{ a.name }}</span>
            </span>
            <span class="text-muted-foreground font-semibold tabular-nums flex-shrink-0">{{ a.count }} 篇 · {{ a.pct }}%</span>
          </div>
          <div class="w-full h-1.5 rounded-full bg-white/10 overflow-hidden">
            <div class="h-full rounded-full transition-all duration-300" :style="{ width: `${a.pct}%`, backgroundColor: a.color, boxShadow: `0 0 4px ${a.color}` }" />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.donut-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}
.donut-placeholder { height: 180px; }

.donut-body {
  display: flex;
  align-items: center;
  gap: 16px;
}
.donut-svg {
  width: 144px;
  height: 144px;
  flex-shrink: 0;
}
.donut-segment {
  transition: opacity var(--duration-normal) var(--ease-standard);
  cursor: pointer;
}
.donut-center-num { font-size: 22px; font-weight: 700; }
.donut-center-label { font-size: 11px; }

.donut-legend {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
  overflow: hidden;
}
.donut-legend-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  padding: 4px 6px;
  border-radius: 6px;
  transition: background var(--duration-fast) var(--ease-standard);
}
.donut-legend-item.hovered { background: color-mix(in srgb, #fff 5%, transparent); }
.donut-dot { width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0; }
.donut-name { color: var(--color-foreground); flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.donut-count { color: var(--color-muted-foreground); flex-shrink: 0; }
.donut-pct { color: var(--color-text-quaternary); font-size: 11px; width: 34px; text-align: right; flex-shrink: 0; }

/* M */
.apple-card:is([data-bp="md"], [data-bp="sm"]) .donut-svg,
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .donut-svg { width: 120px; height: 120px; }
.apple-card:is([data-bp="md"], [data-bp="sm"]) .donut-body,
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .donut-body { gap: 12px; }
.apple-card:is([data-bp="md"], [data-bp="sm"]) .donut-legend-item,
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .donut-legend-item { font-size: 12px; gap: 6px; padding: 3px 4px; }
.apple-card:is([data-bp="md"], [data-bp="sm"]) .donut-center-num,
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .donut-center-num { font-size: 20px; }
.apple-card:is([data-bp="md"], [data-bp="sm"]) .donut-dot,
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .donut-dot { width: 8px; height: 8px; }
.apple-card:is([data-bp="md"], [data-bp="sm"]) .donut-pct,
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .donut-pct { width: 30px; font-size: 10px; }

/* S: stack vertically, center donut */
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .donut-body {
  flex-direction: column;
  align-items: stretch;
  gap: 12px;
}
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .donut-svg {
  width: 130px;
  height: 130px;
  margin: 0 auto;
}
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .donut-placeholder { height: 160px; }
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .donut-legend {
  max-height: none;
}
</style>
