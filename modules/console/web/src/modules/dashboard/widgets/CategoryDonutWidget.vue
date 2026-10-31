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

// 图表色必须是功能 token；在 setup 时读取，确保主题变量已就绪且支持运行时主题变化。
const CHART_KEYS = ["chart-2", "chart-1", "chart-3", "chart-4", "chart-5", "chart-6", "chart-7", "chart-8"] as const;
const CHART_FALLBACKS = ["#2e8dff", "#30d158", "#ff9f0a", "#5e5ce6", "#bf5af2", "#32ade6", "#ff6482", "#64d2ff"];
const colors = computed(() => {
  const resolved = themeColors(Object.fromEntries(CHART_KEYS.map((key, index) => [key, CHART_FALLBACKS[index]])));
  return CHART_KEYS.map((key) => resolved[key]);
});

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
      color: colors.value[i % colors.value.length],
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
          <span class="donut-dot" :style="{ backgroundColor: a.color }" />
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
          class="donut-legend-item"
          :class="hoveredIdx === i && 'hovered'"
          role="button"
          tabindex="0"
          :aria-label="`查看分类：${a.name}`"
          @mouseenter="hoveredIdx = i"
          @mouseleave="hoveredIdx = null"
          @click="a.name !== '其他分类' && openCategory(a.name)"
          @keydown.enter="a.name !== '其他分类' && openCategory(a.name)"
        >
          <span class="donut-dot" :style="{ backgroundColor: a.color }" />
          <span class="donut-name">{{ a.name }}</span>
          <span class="donut-count tabular-nums">{{ a.count }}</span>
          <span class="donut-pct tabular-nums">{{ a.pct }}%</span>
        </div>
      </div>
    </div>

    <!-- 读屏器与无障碍数据表格摘要 -->
    <div class="sr-only">
      <h4>文章分类分布明细</h4>
      <p>共 {{ data.length }} 个分类，总计 {{ total }} 篇文章。</p>
      <table>
        <caption>分类分布数据</caption>
        <thead>
          <tr>
            <th scope="col">分类名称</th>
            <th scope="col">文章数</th>
            <th scope="col">占比</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in displayData" :key="item.name">
            <td>{{ item.name }}</td>
            <td>{{ item.count }} 篇</td>
            <td>{{ total > 0 ? Math.round((item.count / total) * 100) : 0 }}%</td>
          </tr>
        </tbody>
      </table>
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
.donut-center-num { font-size: 20px; font-weight: 600; }
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
.donut-legend-item.hovered { background: var(--color-accent); }
.donut-legend-item:focus-visible {
  outline: 2px solid var(--color-ring);
  outline-offset: 2px;
}
.donut-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
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
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .donut-pct { width: 30px; font-size: 11px; }

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
