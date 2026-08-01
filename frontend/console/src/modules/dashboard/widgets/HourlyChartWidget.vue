<script setup lang="ts">
import { ref, onMounted, onUnmounted } from "vue";
import { VCard } from "@/components/ui";
import { Chart } from "chart.js";

const trendDays = ref(7);
const hourlyDist = ref<number[]>(Array(24).fill(0));
const hourlyCanvas = ref<HTMLCanvasElement | null>(null);
let hourlyChartInstance: Chart | null = null;

async function loadHourly() {
  // 加载时段分布数据（如果后端已实现）
  try {
    const res = await fetch(`/api/v1/stats/hourly?days=${trendDays.value}`);
    if (!res.ok) throw new Error(`HTTP ${res.status}: ${res.statusText}`);
    const response = await res.json();
    hourlyDist.value = Array.isArray(response) ?
      response :
      (response.distribution?.map((d: any) => d.count) || Array(24).fill(0));
  } catch (err) {
    console.warn('[HourlyChart] Failed to load hourly data, using defaults:', err);
    hourlyDist.value = Array(24).fill(0); // 失败则使用默认数据
  }
  initHourlyChart();
}

// 初始化时段分布柱状图
function initHourlyChart() {
  const canvas = hourlyCanvas.value;
  if (!canvas) return;

  const cs = getComputedStyle(document.documentElement);
  const c1 = cs.getPropertyValue('--color-chart-1').trim() || '#4ccba0';
  const primaryColor = cs.getPropertyValue('--color-primary').trim() || '#4ccba0';
  const gridColor = cs.getPropertyValue('--color-chart-grid').trim() || '#eef1f6';
  const mutedColor = cs.getPropertyValue('--color-muted-foreground').trim() || '#64748b';
  const cardBg = cs.getPropertyValue('--color-card').trim() || '#ffffff';
  const fgColor = cs.getPropertyValue('--color-foreground').trim() || '#0f172a';
  const borderColor = cs.getPropertyValue('--color-border').trim() || '#e6e9ef';

  const hours = Array.from({ length: 24 }, (_, i) => i);
  const values = hourlyDist.value;

  // 高峰时段高亮 (10-12 点、20-22 点) 用 primary 色，其余用 chart-1
  const bgColors = hours.map(h =>
    (h >= 10 && h <= 12) || (h >= 20 && h <= 22)
      ? primaryColor
      : c1
  );

  hourlyChartInstance = new Chart(canvas, {
    type: 'bar',
    data: {
      labels: hours.map(h => `${h.toString().padStart(2, '0')}:00`),
      datasets: [{
        label: '访问量',
        data: values,
        backgroundColor: bgColors,
        borderRadius: 3,
        barPercentage: 0.75,
        categoryPercentage: 0.85
      }]
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: {
        duration: 0, // 禁用动画避免重绘
      },
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
          callbacks: {
            title: (items: any[]) => items[0].label,
            label: (item: any) => '访问量: ' + item.parsed.y
          }
        }
      },
      scales: {
        x: {
          grid: { display: false },
          border: { color: gridColor },
          ticks: {
            color: mutedColor,
            font: { size: 10 },
            callback: (_, index) => index % 4 === 0 ? `${hours[index]}:00` : ''
          }
        },
        y: {
          beginAtZero: true,
          grid: { color: gridColor },
          border: { display: false },
          ticks: { color: mutedColor, font: { size: 10 } }
        }
      }
    }
  });
}

onMounted(loadHourly);

onUnmounted(() => {
  hourlyChartInstance?.destroy();
  hourlyChartInstance = null;
});
</script>

<template>
  <VCard class="h-full">
    <template #header>
      <div class="flex w-full items-center justify-between px-4 py-3">
        <span class="text-base font-bold text-gray-800 dark:text-gray-100 flex items-center gap-2">
          访问时段分布
        </span>
      </div>
    </template>
    <div class="p-4">
      <div class="h-[260px] w-full">
        <canvas ref="hourlyCanvas"></canvas>
      </div>
    </div>
  </VCard>
</template>
