<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from "vue";
import { Icon } from "@iconify/vue";
import { statsApi } from "@/api";
import type { Chart } from "chart.js";
import { loadChart } from "@/lib/chart-init";
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

const trendDays = ref(7);
const hourlyDist = ref<number[]>(Array(24).fill(0));
const loading = ref(true);
const hourlyCanvas = ref<HTMLCanvasElement | null>(null);
let hourlyChartInstance: Chart | null = null;

// 计算高峰时段与时段占比
const statsSummary = computed(() => {
  const values = hourlyDist.value;
  const maxVal = Math.max(...values, 0);
  const peakHour = maxVal > 0 ? values.indexOf(maxVal) : -1;
  const total = values.reduce((a, b) => a + b, 0);
  const dayTimeTotal = values.slice(8, 20).reduce((a, b) => a + b, 0);
  const dayPct = total > 0 ? Math.round((dayTimeTotal / total) * 100) : 50;

  const peakStr = peakHour >= 0 ? `${peakHour.toString().padStart(2, "0")}:00 - ${(peakHour + 1).toString().padStart(2, "0")}:00` : "暂无峰值";
  return { peakStr, total, dayPct, maxVal };
});

const isEmpty = computed(() => hourlyDist.value.every((value) => value <= 0));

const canvasHeight = computed(() => {
  if (sizePreset.value === "small") return 140;
  if (sizePreset.value === "large") return 220;
  return 180;
});

async function loadHourly() {
  loading.value = true;
  try {
    const resp = await statsApi.hourly(trendDays.value);
    const response = resp.data;
    const source = Array.isArray(response)
      ? response
      : (response.distribution?.map((d: any) => d.count) || Array(24).fill(0));
    hourlyDist.value = Array.from({ length: 24 }, (_, index) => Number(source[index]) || 0);
  } catch (err) {
    hourlyDist.value = Array(24).fill(0);
  } finally {
    loading.value = false;
  }
  if (isEmpty.value) {
    hourlyChartInstance?.destroy();
    hourlyChartInstance = null;
  } else {
    initHourlyChart();
  }
}

async function initHourlyChart() {
  const canvas = hourlyCanvas.value;
  if (!canvas) return;
  const { Chart } = await loadChart();

  if (hourlyChartInstance) {
    hourlyChartInstance.destroy();
    hourlyChartInstance = null;
  }

  const cs = getComputedStyle(document.documentElement);
  const c1 = cs.getPropertyValue("--color-chart-1").trim() || "#2E8DFF";
  const primaryColor = cs.getPropertyValue("--color-primary").trim() || "#2E8DFF";
  const gridColor = cs.getPropertyValue("--color-chart-grid").trim() || "#2c2c2e";
  const mutedColor = cs.getPropertyValue("--color-muted-foreground").trim() || "#8e8e93";
  const fgColor = cs.getPropertyValue("--color-foreground").trim() || "#f5f5f7";

  const hours = Array.from({ length: 24 }, (_, i) => i);
  const values = hourlyDist.value;

  // 只高亮真实最大值；相同峰值的时段保持联动，避免用固定时段制造假峰值。
  const peakValue = Math.max(...values, 0);
  const bgColors = hours.map((h) =>
    peakValue > 0 && values[h] === peakValue ? primaryColor : c1,
  );

  const step = sizePreset.value === "small" ? 6 : 4;

  hourlyChartInstance = new Chart(canvas, {
    type: "bar",
    data: {
      labels: hours.map((h) => `${h.toString().padStart(2, "0")}:00`),
      datasets: [
        {
          label: "访问量",
          data: values,
          backgroundColor: bgColors,
          borderColor: bgColors,
          borderWidth: 1,
          borderRadius: { topLeft: 4, topRight: 4, bottomLeft: 1, bottomRight: 1 },
          borderSkipped: false,
          barPercentage: sizePreset.value === "small" ? 0.65 : 0.75,
          categoryPercentage: 0.85,
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: {
        duration: 0,
      },
      plugins: {
        legend: { display: false },
        tooltip: {
          backgroundColor: "rgba(28, 28, 30, 0.92)",
          titleColor: fgColor,
          bodyColor: mutedColor,
          borderColor: "rgba(255, 255, 255, 0.12)",
          borderWidth: 1,
          padding: 10,
          cornerRadius: 10,
          boxPadding: 4,
          callbacks: {
            title: (items: any[]) => items[0].label,
            label: (item: any) => ` 访问量: ${Number(item.parsed.y).toLocaleString()}`,
          },
        },
      },
      scales: {
        x: {
          grid: { display: false },
          border: { color: gridColor },
          ticks: {
            color: mutedColor,
            font: { size: 10, family: "var(--font-mono)" },
            callback: (_, index) => (index % step === 0 ? `${hours[index]}:00` : ""),
          },
        },
        y: {
          beginAtZero: true,
          grid: { color: gridColor, drawTicks: false },
          border: { display: false },
          ticks: {
            color: mutedColor,
            font: { size: 10, family: "var(--font-mono)" },
            display: sizePreset.value !== "small",
          },
        },
      },
    },
  });
}

watch(sizePreset, () => {
  if (!isEmpty.value) initHourlyChart();
});

watch(trendDays, loadHourly);

onMounted(() => {
  loadHourly();
  window.addEventListener(DASHBOARD_REFRESH_EVENT, loadHourly);
});

onUnmounted(() => {
  window.removeEventListener(DASHBOARD_REFRESH_EVENT, loadHourly);
  hourlyChartInstance?.destroy();
  hourlyChartInstance = null;
});
</script>

<template>
  <div class="apple-card h-full flex flex-col justify-between" :class="`hourly-card-${sizePreset}`">
    <div class="flex items-center justify-between mb-3">
      <div class="flex items-center gap-2">
        <h3 class="apple-section-title" :class="{ 'text-sm': sizePreset === 'small' }">访问时段分布</h3>
        <span v-if="sizePreset !== 'small'" class="apple-card-badge">24h Hours</span>
      </div>
      <div class="flex items-center gap-2">
        <div v-if="sizePreset !== 'small'" class="apple-segmented hourly-range-switch" aria-label="时段统计范围">
          <button type="button" :class="['seg-btn', trendDays === 7 && 'active']" :aria-pressed="trendDays === 7" @click="trendDays = 7">7 天</button>
          <button type="button" :class="['seg-btn', trendDays === 30 && 'active']" :aria-pressed="trendDays === 30" @click="trendDays = 30">30 天</button>
        </div>
        <span class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium bg-primary/10 text-primary border border-primary/15">
          <Icon icon="ri:fire-line" class="w-3.5 h-3.5" />
          {{ statsSummary.peakStr }}
        </span>
      </div>
    </div>

    <!-- Large 尺寸时段分析胶囊：表面层级与 apple-card 边框语言一致 -->
    <div v-if="sizePreset === 'large'" class="grid grid-cols-3 gap-2.5 mb-3 text-xs">
      <div class="p-2.5 rounded-xl bg-white/[0.03] border border-white/[0.06] flex items-center justify-between">
        <span class="text-muted-foreground flex items-center gap-1">
          <Icon icon="ri:sun-line" class="w-3.5 h-3.5 text-amber-400" />
          日间活跃
        </span>
        <strong class="text-foreground tabular-nums font-semibold">{{ statsSummary.dayPct }}%</strong>
      </div>
      <div class="p-2.5 rounded-xl bg-white/[0.03] border border-white/[0.06] flex items-center justify-between">
        <span class="text-muted-foreground flex items-center gap-1">
          <Icon icon="ri:moon-line" class="w-3.5 h-3.5 text-indigo-400" />
          夜间活跃
        </span>
        <strong class="text-foreground tabular-nums font-semibold">{{ 100 - statsSummary.dayPct }}%</strong>
      </div>
      <div class="p-2.5 rounded-xl bg-white/[0.03] border border-white/[0.06] flex items-center justify-between">
        <span class="text-muted-foreground flex items-center gap-1">
          <Icon icon="ri:flashlight-line" class="w-3.5 h-3.5 text-primary" />
          单时段峰值
        </span>
        <strong class="text-primary tabular-nums font-semibold">{{ statsSummary.maxVal.toLocaleString() }} 次</strong>
      </div>
    </div>

    <div class="hourly-chart-stage flex-1 min-h-0 w-full" :style="{ height: `${canvasHeight}px` }">
      <canvas v-if="!isEmpty || loading" ref="hourlyCanvas" :class="{ 'is-loading': loading }" aria-label="24 小时访问量分布图" />
      <div v-if="loading" class="hourly-chart-skeleton" aria-label="正在加载时段分布">
        <span v-for="height in [28, 42, 34, 62, 48, 72, 40, 55, 32, 66, 46, 78]" :key="height" :style="{ height: `${height}%` }" />
      </div>
      <div v-else-if="isEmpty" class="hourly-empty-state" aria-label="暂无时段访问数据">
        <div class="hourly-empty-lines" aria-hidden="true"><i /><i /><i /></div>
        <Icon icon="ri:bar-chart-2-line" class="w-5 h-5" />
        <span>暂无访问数据</span>
        <small>有新的访问后，这里会显示高峰时段</small>
      </div>
    </div>
  </div>
</template>

<style scoped>
.hourly-chart-stage {
  position: relative;
}

.hourly-chart-stage canvas {
  display: block;
  width: 100% !important;
  height: 100% !important;
  transition: opacity 180ms var(--ease-standard);
}

.hourly-chart-stage canvas.is-loading {
  opacity: 0.12;
}

.hourly-chart-skeleton {
  position: absolute;
  inset: 10px 4px 24px;
  display: flex;
  align-items: flex-end;
  justify-content: space-around;
  gap: 5px;
  overflow: hidden;
  border-radius: 12px;
}

.hourly-chart-skeleton span {
  position: relative;
  z-index: 0;
  width: 3.5%;
  min-height: 8px;
  border-radius: 999px 999px 2px 2px;
  background: color-mix(in srgb, var(--color-primary) 20%, transparent);
}

.hourly-chart-skeleton::after {
  z-index: 1;
}

.hourly-empty-state {
  position: absolute;
  inset: 8px 0 22px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  gap: 6px;
  border: 1px dashed color-mix(in srgb, var(--color-primary) 28%, var(--color-border));
  border-radius: 14px;
  color: var(--color-muted-foreground);
  background: color-mix(in srgb, var(--color-background) 35%, transparent);
}

.hourly-empty-state span {
  color: var(--color-foreground);
  font-size: 12px;
  font-weight: 600;
}

.hourly-empty-state small {
  font-size: 10px;
  color: var(--color-muted-foreground);
}

.hourly-empty-lines {
  display: flex;
  align-items: flex-end;
  gap: 5px;
  height: 30px;
  margin-bottom: 2px;
}

.hourly-empty-lines i {
  display: block;
  width: 5px;
  height: 12px;
  border: 1px dashed color-mix(in srgb, var(--color-primary) 45%, transparent);
  border-radius: 3px 3px 1px 1px;
}

.hourly-empty-lines i:nth-child(2) { height: 22px; }
.hourly-empty-lines i:nth-child(3) { height: 16px; }
</style>
