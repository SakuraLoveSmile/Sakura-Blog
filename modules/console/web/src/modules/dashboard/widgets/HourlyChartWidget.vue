<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from "vue";
import { Icon } from "@iconify/vue";
import { statsApi } from "@/api";
import type { Chart } from "chart.js";
import { loadChart } from "@/lib/chart-init";
import type { WidgetSizePreset } from "./layout";
import { useWidgetSize } from "../useWidgetSize";

interface Props {
  sizePreset?: WidgetSizePreset;
}

const props = withDefaults(defineProps<Props>(), {
  sizePreset: "medium",
});
const sizePreset = useWidgetSize(props.sizePreset).preset;

const trendDays = ref(7);
const hourlyDist = ref<number[]>(Array(24).fill(0));
const hourlyCanvas = ref<HTMLCanvasElement | null>(null);
let hourlyChartInstance: Chart | null = null;

// 计算高峰时段与时段占比
const statsSummary = computed(() => {
  const values = hourlyDist.value;
  const maxVal = Math.max(...values, 0);
  const peakHour = values.indexOf(maxVal);
  const total = values.reduce((a, b) => a + b, 0);
  const dayTimeTotal = values.slice(8, 20).reduce((a, b) => a + b, 0);
  const dayPct = total > 0 ? Math.round((dayTimeTotal / total) * 100) : 50;

  const peakStr = peakHour >= 0 ? `${peakHour.toString().padStart(2, "0")}:00 - ${(peakHour + 1).toString().padStart(2, "0")}:00` : "--";
  return { peakStr, total, dayPct, maxVal };
});

const canvasHeight = computed(() => {
  if (sizePreset.value === "small") return 140;
  if (sizePreset.value === "large") return 220;
  return 180;
});

async function loadHourly() {
  try {
    const resp = await statsApi.hourly(trendDays.value);
    const response = resp.data;
    hourlyDist.value = Array.isArray(response)
      ? response
      : (response.distribution?.map((d: any) => d.count) || Array(24).fill(0));
  } catch (err) {
    hourlyDist.value = Array(24).fill(0);
  }
  initHourlyChart();
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
  const cardBg = cs.getPropertyValue("--color-card").trim() || "#1c1c1e";
  const fgColor = cs.getPropertyValue("--color-foreground").trim() || "#f5f5f7";
  const borderColor = cs.getPropertyValue("--color-border").trim() || "#3a3a3c";

  const hours = Array.from({ length: 24 }, (_, i) => i);
  const values = hourlyDist.value;

  const bgColors = hours.map((h) =>
    (h >= 10 && h <= 12) || (h >= 20 && h <= 22)
      ? primaryColor
      : c1,
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
          borderRadius: 3,
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
          backgroundColor: cardBg,
          titleColor: fgColor,
          bodyColor: mutedColor,
          borderColor: borderColor,
          borderWidth: 1,
          padding: 8,
          cornerRadius: 8,
          callbacks: {
            title: (items: any[]) => items[0].label,
            label: (item: any) => `访问量: ${item.parsed.y}`,
          },
        },
      },
      scales: {
        x: {
          grid: { display: false },
          border: { color: gridColor },
          ticks: {
            color: mutedColor,
            font: { size: 10 },
            callback: (_, index) => (index % step === 0 ? `${hours[index]}:00` : ""),
          },
        },
        y: {
          beginAtZero: true,
          grid: { color: gridColor },
          border: { display: false },
          ticks: {
            color: mutedColor,
            font: { size: 10 },
            display: sizePreset.value !== "small",
          },
        },
      },
    },
  });
}

watch(sizePreset, () => {
  initHourlyChart();
});

onMounted(loadHourly);

onUnmounted(() => {
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
        <span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium bg-primary/10 text-primary">
          <Icon icon="ri:fire-line" class="w-3 h-3" />
          {{ statsSummary.peakStr }}
        </span>
      </div>
    </div>

    <!-- Large 尺寸时段分析胶囊 -->
    <div v-if="sizePreset === 'large'" class="grid grid-cols-3 gap-2 mb-3 text-xs">
      <div class="p-2 rounded-xl bg-secondary/30 border border-border/40 flex items-center justify-between">
        <span class="text-muted-foreground">日间活跃 (08-20h)</span>
        <strong class="text-foreground">{{ statsSummary.dayPct }}%</strong>
      </div>
      <div class="p-2 rounded-xl bg-secondary/30 border border-border/40 flex items-center justify-between">
        <span class="text-muted-foreground">夜间活跃 (20-08h)</span>
        <strong class="text-foreground">{{ 100 - statsSummary.dayPct }}%</strong>
      </div>
      <div class="p-2 rounded-xl bg-secondary/30 border border-border/40 flex items-center justify-between">
        <span class="text-muted-foreground">单时段峰值</span>
        <strong class="text-primary">{{ statsSummary.maxVal }} 次</strong>
      </div>
    </div>

    <div class="flex-1 min-h-0 w-full" :style="{ height: `${canvasHeight}px` }">
      <canvas ref="hourlyCanvas" />
    </div>
  </div>
</template>
