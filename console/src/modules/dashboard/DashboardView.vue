<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { dashboardApi, statsApi } from "@/api";
import { Icon } from "@iconify/vue";
import { VPageHeader, VCard } from "@/components/ui";
import { Chart } from "chart.js";

const stats = ref({
  post_total: 0,
  post_published: 0,
  comment_total: 0,
  comment_pending: 0,
  view_total: 0,
  attachment_total: 0,
});
const recentComments = ref<any[]>([]);

const cards = [
  { key: "post_total", label: "文章", icon: "ri:book-read-line" },
  { key: "comment_total", label: "评论", icon: "ri:chat-3-line" },
  { key: "view_total", label: "浏览量", icon: "ri:eye-line" },
  { key: "attachment_total", label: "附件", icon: "ri:folder-2-line" },
];

const quickActions = [
  { label: "写文章", icon: "ri:quill-pen-line", to: "/posts/new" },
  { label: "审核评论", icon: "ri:chat-check-line", to: "/comments" },
  { label: "上传附件", icon: "ri:upload-cloud-line", to: "/attachments" },
  { label: "系统设置", icon: "ri:settings-3-line", to: "/settings" },
];

// --- 访问统计 ---
const trendDays = ref(7);
const trend = ref<{ date: string; pv: number; uv: number }[]>([]);

// Chart.js refs
const trendCanvas = ref<HTMLCanvasElement | null>(null);
const hourlyCanvas = ref<HTMLCanvasElement | null>(null);
const metricTab = ref<'pv' | 'uv' | 'both'>('both');

/** 趋势图日期范围文字（如 "2026-06-27 至 2026-07-26"） */
const trendDateRange = computed(() => {
  if (!trend.value.length) return '';
  return `${trend.value[0].date} 至 ${trend.value[trend.value.length - 1].date}`;
});
const hourlyDist = ref<number[]>(Array(24).fill(0));
let trendChartInstance: Chart | null = null;

async function loadStats() {
  try {
    const { data } = await statsApi.trend(trendDays.value);
    trend.value = data.trend || [];
    console.log('[Dashboard] Trend data loaded:', trend.value.length, 'days');
  } catch (err) {
    console.error('[Dashboard] Failed to load trend data:', err);
    trend.value = Array(7).fill({ date: '', pv: 0, uv: 0 }); // 兜底数据
  }
  
  // 加载时段分布数据（如果后端已实现）
  try {
    const res = await fetch(`/api/v1/stats/hourly?days=${trendDays.value}`);
    if (!res.ok) throw new Error(`HTTP ${res.status}: ${res.statusText}`);
    const response = await res.json();
    console.log('[Dashboard] Hourly response:', response);
    hourlyDist.value = Array.isArray(response) ? 
      response : 
      (response.distribution?.map((d: any) => d.count) || Array(24).fill(0));
  } catch (err) {
    console.warn('[Dashboard] Failed to load hourly data, using defaults:', err);
    hourlyDist.value = Array(24).fill(0); // 失败则使用默认数据
  }
  
  console.log('[Dashboard] Initializing charts...');
  console.log('  - trend.value length:', trend.value.length);
  console.log('  - hourlyDist length:', hourlyDist.value.length);
  
  // 数据加载成功后再初始化图表
  initTrendChart();
  initHourlyChart();
}

// 初始化主趋势折线图
function initTrendChart() {
  // 如果没有数据，不初始化图表
  if (!trend.value || trend.value.length === 0) {
    console.warn('[Chart.js] Trend chart skipped: no data');
    return;
  }
  const canvas = trendCanvas.value;
  if (!canvas) {
    console.error('[Chart.js] Trend chart skipped: canvas not found');
    return;
  }

  const ctx = canvas.getContext('2d');
  const cs = getComputedStyle(document.documentElement);
  const c1 = cs.getPropertyValue('--color-chart-1').trim() || '#4ccba0';
  const c2 = cs.getPropertyValue('--color-chart-2').trim() || '#5b8def';
  const gridColor = cs.getPropertyValue('--color-chart-grid').trim() || '#eef1f6';
  const mutedColor = cs.getPropertyValue('--color-muted-foreground').trim() || '#64748b';
  const cardBg = cs.getPropertyValue('--color-card').trim() || '#ffffff';
  const fgColor = cs.getPropertyValue('--color-foreground').trim() || '#0f172a';
  const borderColor = cs.getPropertyValue('--color-border').trim() || '#e6e9ef';
  
  // PV 渐变色填充 - hex alpha（1F = 12% 透明度）
  const gradient = ctx!.createLinearGradient(0, 0, 0, canvas.height);
  gradient.addColorStop(0, c1 + '1F');
  gradient.addColorStop(1, c1 + '00');

  trendChartInstance = new Chart(canvas, {
    type: 'line',
    data: {
      labels: trend.value.map(d => d.date.slice(5)),
      datasets: [
        {
          label: 'PV',
          data: trend.value.map(d => d.pv),
          borderColor: c1,
          backgroundColor: gradient,
          borderWidth: 2,
          fill: true,
          tension: 0.35,
          pointRadius: 0,
          pointHoverRadius: 4,
          pointHoverBackgroundColor: c1,
          pointHoverBorderColor: '#ffffff',
          pointHoverBorderWidth: 2
        },
        {
          label: 'UV',
          data: trend.value.map(d => d.uv),
          borderColor: c2,
          backgroundColor: 'transparent',
          borderWidth: 2,
          fill: false,
          tension: 0.35,
          pointRadius: 0,
          pointHoverRadius: 4,
          pointHoverBackgroundColor: c2,
          pointHoverBorderColor: '#ffffff',
          pointHoverBorderWidth: 2,
          hidden: metricTab.value !== 'both'
        }
      ]
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      animation: {
        duration: 0, // 禁用动画避免重绘
      },
      interaction: { mode: 'index', intersect: false },
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
          displayColors: true,
          boxPadding: 4
        }
      },
      scales: {
        x: {
          grid: { display: false },
          border: { color: gridColor },
          ticks: {
            color: mutedColor,
            font: { size: 11 },
            maxRotation: 0,
            maxTicksLimit: 7,
            callback: (_, index) => {
              const step = Math.max(1, Math.ceil(trend.value.length / 7));
              return index % step === 0 ? trend.value[index]?.date.slice(5) : '';
            }
          }
        },
        y: {
          beginAtZero: false,
          grid: { color: gridColor },
          border: { display: false },
          ticks: {
            color: mutedColor,
            font: { size: 11 }
          }
        }
      }
    }
  });
}

// 初始化时段分布柱状图
function initHourlyChart() {
  const canvas = hourlyCanvas.value;
  if (!canvas) {
    console.error('[Chart.js] Hourly chart skipped: canvas not found');
    return;
  }

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

  new Chart(canvas, {
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

// 指标切换
function switchMetric(key: 'pv' | 'uv' | 'both') {
  metricTab.value = key;
  if (trendChartInstance) {
    trendChartInstance.data.datasets[0].hidden = key === 'uv';
    trendChartInstance.data.datasets[1].hidden = key === 'pv';
    trendChartInstance.update();
  }
}

onMounted(async () => {
  try {
    const { data } = await dashboardApi.stats();
    stats.value = data;
  } catch {}
  try {
    const { data } = await dashboardApi.recentComments();
    recentComments.value = data.items || [];
  } catch {}
  
  // 数据加载完成后自动初始化图表
  await loadStats();
});
</script>

<template>
  <div>
    <!-- VPageHeader -->
    <VPageHeader title="仪表盘">
      <template #icon>
        <Icon icon="ri:dashboard-3-line" class="mr-2 self-center text-xl" />
      </template>
    </VPageHeader>

    <div class="m-0 space-y-4 p-4 md:m-4 md:p-0">
      <!-- 统计卡片网格 -->
      <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <VCard v-for="card in cards" :key="card.key">
          <div class="flex items-center gap-3">
            <span class="flex h-10 w-10 items-center justify-center rounded bg-gray-100 dark:bg-gray-700" style="border-radius: 4px">
              <Icon :icon="card.icon" class="text-xl text-gray-600 dark:text-gray-300" />
            </span>
            <div>
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ card.label }}</p>
              <p class="text-xl font-bold text-gray-900 dark:text-gray-100">
                {{ (stats as any)[card.key] ?? 0 }}
              </p>
            </div>
          </div>
        </VCard>
      </div>

      <!-- 待审核提醒 -->
      <router-link
        v-if="stats.comment_pending > 0"
        to="/comments"
        class="flex items-center gap-2 rounded border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-700 transition-colors hover:bg-amber-100 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-400"
        style="border-radius: 4px"
      >
        <Icon icon="ri:error-warning-line" class="text-base" />
        有 {{ stats.comment_pending }} 条评论待审核
        <Icon icon="ri:arrow-right-line" class="ml-auto" />
      </router-link>

      <!-- 快捷操作 -->
      <VCard title="快捷操作">
        <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <router-link
            v-for="action in quickActions"
            :key="action.to"
            :to="action.to"
            class="flex flex-col items-center gap-2 rounded border border-gray-100 py-4 text-gray-600 transition-all hover:border-gray-200 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-700/50"
            style="border-radius: 4px"
          >
            <Icon :icon="action.icon" class="text-2xl" />
            <span class="text-xs">{{ action.label }}</span>
          </router-link>
        </div>
      </VCard>

      <!-- 双栏：评论 + 趋势 -->
      <div class="grid gap-4 lg:grid-cols-2">
        <!-- 最近评论 -->
        <VCard title="最近评论">
          <div v-if="recentComments.length" class="divide-y divide-gray-50 dark:divide-gray-700">
            <div v-for="c in recentComments.slice(0, 6)" :key="c.id" class="flex items-start gap-3 py-2.5">
              <span class="flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full bg-gray-100 text-xs font-medium text-gray-600 dark:bg-gray-700 dark:text-gray-300">
                {{ (c.nick || "?")[0] }}
              </span>
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-2">
                  <span class="text-xs font-medium text-gray-800 dark:text-gray-200">{{ c.nick }}</span>
                  <span class="text-[10px] text-gray-400">{{ c.date?.slice(0, 10) }}</span>
                </div>
                <p class="mt-0.5 truncate text-xs text-gray-500 dark:text-gray-400">{{ c.content }}</p>
              </div>
            </div>
          </div>
          <p v-else class="py-8 text-center text-xs text-gray-400">暂无评论</p>
        </VCard>

        <!-- 访问趋势 -->
        <VCard>
          <template #header>
            <div class="flex w-full flex-col gap-2 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <span class="flex items-center gap-2 text-base font-semibold text-foreground">
                  <Icon icon="ri:line-chart-line" class="text-lg" />
                  访问趋势
                </span>
                <p v-if="trendDateRange" class="mt-0.5 text-xs text-muted-foreground">{{ trendDateRange }}</p>
              </div>
              <div class="inline-flex rounded-md border border-border bg-muted/40 p-0.5">
                <button
                  v-for="{ key, label } in [
                    { key: 'pv', label: 'PV' },
                    { key: 'uv', label: 'UV' },
                    { key: 'both', label: '同时' }
                  ]"
                  :key="label"
                  :class="[
                    'rounded-[6px] px-3 py-1.5 text-xs font-medium transition-all',
                    metricTab === key 
                      ? 'bg-primary text-primary-foreground shadow-sm'
                      : 'text-muted-foreground hover:text-foreground',
                  ]"
                  @click="switchMetric(key)"
                >
                  {{ label }}
                </button>
              </div>
            </div>
          </template>
          <div class="p-4">
            <!-- 图例（图表上方） -->
            <div class="flex items-center justify-center gap-4 text-xs text-muted-foreground">
              <span class="inline-flex items-center gap-1.5">
                <span class="h-2.5 w-2.5 rounded-full bg-chart-1"></span>
                PV 页面浏览量
              </span>
              <span class="inline-flex items-center gap-1.5">
                <span class="h-2.5 w-2.5 rounded-full bg-chart-2"></span>
                UV 独立访客
              </span>
            </div>
            <div class="mt-3 h-[260px] w-full">
              <canvas ref="trendCanvas"></canvas>
            </div>
          </div>
        </VCard>

        <!-- 时段分布 -->
        <VCard>
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
      </div>
    </div>
  </div>
</template>
