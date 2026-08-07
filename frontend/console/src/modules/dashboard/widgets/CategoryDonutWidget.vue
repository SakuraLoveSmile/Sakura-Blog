<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import { useBreakpoint } from "../useBreakpoint";

const { breakpoint } = useBreakpoint();

interface CatItem {
  name: string;
  count: number;
}

interface CatResp {
  total: number;
  items: CatItem[];
}

const COLORS = ["#0A84FF", "#5AC8FA", "#BF5AF2", "#FF453A", "#30D158", "#FF9F0A", "#64D2FF", "#FFD60A"];

const data = ref<CatItem[]>([]);
const loading = ref(true);
const error = ref("");
const hoveredIdx = ref<number | null>(null);

onMounted(() => {
  fetch("/api/v1/dashboard/categories", { credentials: "include" })
    .then((r) => {
      if (!r.ok) throw new Error("加载分类失败");
      return r.json();
    })
    .then((resp: CatResp) => {
      data.value = resp.items.sort((a, b) => b.count - a.count).slice(0, 7);
    })
    .catch((e) => (error.value = e.message))
    .finally(() => (loading.value = false));
});

const total = computed(() => data.value.reduce((s, d) => s + d.count, 0));

const arcs = computed(() => {
  if (total.value === 0) return [];
  const cx = 90;
  const cy = 90;
  const r = 65;
  const innerR = 45;
  let startAngle = -Math.PI / 2;
  return data.value.map((d, i) => {
    const angle = (d.count / total.value) * Math.PI * 2;
    const endAngle = startAngle + angle;
    const x1 = cx + r * Math.cos(startAngle);
    const y1 = cy + r * Math.sin(startAngle);
    const x2 = cx + r * Math.cos(endAngle);
    const y2 = cy + r * Math.sin(endAngle);
    const ix1 = cx + innerR * Math.cos(endAngle);
    const iy1 = cy + innerR * Math.sin(endAngle);
    const ix2 = cx + innerR * Math.cos(startAngle);
    const iy2 = cy + innerR * Math.sin(startAngle);
    const largeArc = angle > Math.PI ? 1 : 0;
    const path = `M ${x1} ${y1} A ${r} ${r} 0 ${largeArc} 1 ${x2} ${y2} L ${ix1} ${iy1} A ${innerR} ${innerR} 0 ${largeArc} 0 ${ix2} ${iy2} Z`;
    startAngle = endAngle;
    return {
      path,
      color: COLORS[i % COLORS.length],
      pct: Math.round((d.count / total.value) * 100),
      ...d,
    };
  });
});
</script>

<template>
  <div class="apple-card h-full" :data-bp="breakpoint">
    <div class="donut-header">
      <h3 class="apple-section-title">分类分布</h3>
      <span class="apple-card-badge">Categories</span>
    </div>

    <div v-if="loading" class="donut-placeholder flex items-center justify-center text-[#8E8E93]">
      <Icon icon="ri:loader-4-line" class="w-6 h-6 animate-spin" />
    </div>
    <div v-else-if="error" class="donut-placeholder flex items-center justify-center text-red-400">{{ error }}</div>
    <div v-else-if="data.length === 0" class="donut-placeholder flex items-center justify-center text-[#8E8E93] text-sm">
      暂无分类数据
    </div>
    <div v-else class="donut-body">
      <svg viewBox="0 0 180 180" class="donut-svg">
        <g @mouseleave="hoveredIdx = null">
          <path
            v-for="(a, i) in arcs"
            :key="i"
            :d="a.path"
            :fill="a.color"
            :opacity="hoveredIdx === null || hoveredIdx === i ? 1 : 0.5"
            class="donut-segment"
            @mouseenter="hoveredIdx = i"
          />
        </g>
        <text x="90" y="85" text-anchor="middle" fill="#fff" class="donut-center-num">{{ total }}</text>
        <text x="90" y="105" text-anchor="middle" fill="#8E8E93" class="donut-center-label">篇文章</text>
      </svg>
      <div class="donut-legend">
        <div
          v-for="(a, i) in arcs"
          :key="i"
          class="donut-legend-item"
          :class="hoveredIdx === i && 'hovered'"
          @mouseenter="hoveredIdx = i"
          @mouseleave="hoveredIdx = null"
        >
          <span class="donut-dot" :style="{ backgroundColor: a.color }" />
          <span class="donut-name">{{ a.name }}</span>
          <span class="donut-count">{{ a.count }}</span>
          <span class="donut-pct">{{ a.pct }}%</span>
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
  transition: opacity 0.2s ease;
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
  transition: background 0.15s ease;
}
.donut-legend-item.hovered { background: rgba(255,255,255,0.05); }
.donut-dot { width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0; }
.donut-name { color: #EBEBF5; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.donut-count { color: #8E8E93; flex-shrink: 0; }
.donut-pct { color: #636366; font-size: 11px; width: 34px; text-align: right; flex-shrink: 0; }

/* M */
.apple-card[data-bp="m"] .donut-svg,
.apple-card[data-bp="s"] .donut-svg { width: 120px; height: 120px; }
.apple-card[data-bp="m"] .donut-body,
.apple-card[data-bp="s"] .donut-body { gap: 12px; }
.apple-card[data-bp="m"] .donut-legend-item,
.apple-card[data-bp="s"] .donut-legend-item { font-size: 12px; gap: 6px; padding: 3px 4px; }
.apple-card[data-bp="m"] .donut-center-num,
.apple-card[data-bp="s"] .donut-center-num { font-size: 20px; }
.apple-card[data-bp="m"] .donut-dot,
.apple-card[data-bp="s"] .donut-dot { width: 8px; height: 8px; }
.apple-card[data-bp="m"] .donut-pct,
.apple-card[data-bp="s"] .donut-pct { width: 30px; font-size: 10px; }

/* S: stack vertically, center donut */
.apple-card[data-bp="s"] .donut-body {
  flex-direction: column;
  align-items: stretch;
  gap: 12px;
}
.apple-card[data-bp="s"] .donut-svg {
  width: 130px;
  height: 130px;
  margin: 0 auto;
}
.apple-card[data-bp="s"] .donut-placeholder { height: 160px; }
.apple-card[data-bp="s"] .donut-legend {
  max-height: none;
}
</style>
