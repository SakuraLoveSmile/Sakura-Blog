<script setup lang="ts">
import { ref, onMounted, computed, watch, nextTick } from "vue";
import { dashboardApi, statsApi, settingsApi } from "@/api";
import { Icon } from "@iconify/vue";
import { VPageHeader, VCard, VModal, VSwitch, Toast } from "@/components/ui";
import { Chart } from "chart.js";

// --- 仪表盘组件清单定义 ---
/** 全部组件清单（id 与显示名称），顺序即默认顺序 */
const WIDGET_DEFS = [
  { id: "stats-cards", label: "统计卡片" },
  { id: "pending-alert", label: "待审核提醒" },
  { id: "quick-actions", label: "快捷操作" },
  { id: "recent-comments", label: "最近评论" },
  { id: "visit-trend", label: "访问趋势" },
  { id: "hourly-dist", label: "访问时段分布" },
];
/** 默认配置：全部启用，按 WIDGET_DEFS 顺序 */
const DEFAULT_WIDGETS = WIDGET_DEFS.map((w) => ({ id: w.id, enabled: true }));

// --- 当前 dashboard 配置 ---
const dashboardConfig = ref<{ id: string; enabled: boolean }[]>([]);

/** 启用的组件 id 列表（按配置顺序过滤 enabled） */
const enabledWidgets = computed(() =>
  dashboardConfig.value.filter((w) => w.enabled).map((w) => w.id)
);

/** 除 stats-cards 外的组件按顺序两两配对（用于双栏布局），落单单独一行。
 *  pending-alert 仅在有待审核评论时参与布局，避免空槽位导致双栏错位。 */
const enabledWidgetRows = computed(() => {
  const others = enabledWidgets.value.filter((id) => {
    if (id === "stats-cards") return false;
    if (id === "pending-alert" && stats.value.comment_pending <= 0) return false;
    return true;
  });
  const rows: string[][] = [];
  for (let i = 0; i < others.length; i += 2) {
    rows.push(others.slice(i, i + 2));
  }
  return rows;
});

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

// 可用快捷操作池：前端定义全部可选操作（key/label/icon/to），用户从中选择。
// key 与后端 settings 表存储的标识一致，to 为 console 路由路径。
const availableActions = [
  { key: "write_post", label: "写文章", icon: "ri:quill-pen-line", to: "/posts/new" },
  { key: "posts", label: "文章管理", icon: "ri:file-list-3-line", to: "/posts" },
  { key: "review_comments", label: "审核评论", icon: "ri:chat-check-line", to: "/comments" },
  { key: "upload_attachment", label: "上传附件", icon: "ri:upload-cloud-line", to: "/attachments" },
  { key: "pages", label: "页面管理", icon: "ri:pages-line", to: "/pages" },
  { key: "moments", label: "瞬间管理", icon: "ri:flashlight-line", to: "/moments" },
  { key: "notes", label: "笔记编辑", icon: "ri:sticky-note-line", to: "/notes-page" },
  { key: "taxonomy", label: "分类标签", icon: "ri:price-tag-3-line", to: "/taxonomy" },
  { key: "friends", label: "友链管理", icon: "ri:link", to: "/friends" },
  { key: "plugins", label: "插件库", icon: "ri:puzzle-line", to: "/plugins" },
  { key: "settings", label: "系统设置", icon: "ri:settings-3-line", to: "/settings" },
  { key: "backup", label: "备份管理", icon: "ri:archive-line", to: "/backup" },
];

// 用户选中的快捷操作 key 列表（从后端加载）
const selectedKeys = ref<string[]>([]);
// 编辑弹窗的临时选中列表（保存前不影哏展示）
const editKeys = ref<string[]>([]);
const showEditModal = ref(false);
const savingActions = ref(false);
const MAX_QUICK_ACTIONS = 8;

/** 根据选中 key 列表映射出完整的操作对象（供模板渲染） */
const quickActions = computed(() =>
  selectedKeys.value
    .map((k) => availableActions.find((a) => a.key === k))
    .filter((a) => a != null) as typeof availableActions
);

/** 从后端加载用户自定义的快捷操作 */
async function loadQuickActions() {
  try {
    const { data } = await dashboardApi.quickActions();
    selectedKeys.value = data.items || [];
  } catch {
    selectedKeys.value = ["write_post", "review_comments", "upload_attachment", "settings"];
  }
}

/** 打开编辑弹窗，用当前选中状态初始化临时列表 */
function openEditModal() {
  editKeys.value = [...selectedKeys.value];
  showEditModal.value = true;
}

/** 切换某项的选中状态（已选则取消，未选且未达上限则添加） */
function toggleAction(key: string) {
  const idx = editKeys.value.indexOf(key);
  if (idx >= 0) {
    editKeys.value.splice(idx, 1);
  } else if (editKeys.value.length < MAX_QUICK_ACTIONS) {
    editKeys.value.push(key);
  }
}

/** 保存编辑结果到后端，成功后更新展示列表并关闭弹窗 */
async function saveQuickActions() {
  savingActions.value = true;
  try {
    const { data } = await dashboardApi.saveQuickActions(editKeys.value);
    selectedKeys.value = data.items || [];
    showEditModal.value = false;
  } catch {
    // 保存失败保持弹窗打开，用户可重试
  } finally {
    savingActions.value = false;
  }
}

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
let hourlyChartInstance: Chart | null = null;

/** 数据是否已加载完成（避免配置变更时在数据未就绪状态下初始化图表） */
const dataLoaded = ref(false);

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
    hourlyDist.value = Array.isArray(response)
      ? response
      : (response.distribution?.map((d: any) => d.count) || Array(24).fill(0));
  } catch (err) {
    console.warn('[Dashboard] Failed to load hourly data, using defaults:', err);
    hourlyDist.value = Array(24).fill(0); // 失败则使用默认数据
  }

  console.log('[Dashboard] Initializing charts...');
  console.log('  - trend.value length:', trend.value.length);
  console.log('  - hourlyDist length:', hourlyDist.value.length);

  // 数据加载成功后再初始化图表
  dataLoaded.value = true;
  initCharts();
}

/** 销毁旧图表实例并根据启用状态重新初始化 */
function initCharts() {
  // 先销毁旧实例，避免 canvas 残留导致 Chart.js 重复挂载报错
  if (trendChartInstance) {
    trendChartInstance.destroy();
    trendChartInstance = null;
  }
  if (hourlyChartInstance) {
    hourlyChartInstance.destroy();
    hourlyChartInstance = null;
  }
  // 数据未就绪时只销毁不初始化
  if (!dataLoaded.value) return;
  // 仅在对应组件启用时初始化对应图表
  if (enabledWidgets.value.includes('visit-trend')) initTrendChart();
  if (enabledWidgets.value.includes('hourly-dist')) initHourlyChart();
}

// 监听启用组件变化，DOM 更新后重新初始化图表
watch(enabledWidgets, () => {
  nextTick(() => initCharts());
});

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

// 指标切换
function switchMetric(key: 'pv' | 'uv' | 'both') {
  metricTab.value = key;
  if (trendChartInstance) {
    trendChartInstance.data.datasets[0].hidden = key === 'uv';
    trendChartInstance.data.datasets[1].hidden = key === 'pv';
    trendChartInstance.update();
  }
}

// --- 自定义编辑面板 ---
const showCustomize = ref(false);
/** 编辑中的临时配置副本，取消/关闭不污染原配置 */
const editingConfig = ref<{ id: string; enabled: boolean }[]>([]);
const savingConfig = ref(false);

/** 打开自定义面板，深拷贝当前配置到编辑副本 */
function openCustomize() {
  editingConfig.value = dashboardConfig.value.map((w) => ({ ...w }));
  showCustomize.value = true;
}

/** 编辑面板内上移/下移组件 */
function moveWidget(i: number, dir: -1 | 1) {
  const j = i + dir;
  if (j < 0 || j >= editingConfig.value.length) return;
  [editingConfig.value[i], editingConfig.value[j]] = [
    editingConfig.value[j],
    editingConfig.value[i],
  ];
}

/** 保存自定义配置到后端，成功后更新当前配置并关闭面板 */
async function saveCustomize() {
  savingConfig.value = true;
  try {
    await settingsApi.updateSection('dashboard', { widgets: editingConfig.value });
    dashboardConfig.value = editingConfig.value.map((w) => ({ ...w }));
    showCustomize.value = false;
    Toast.success('已保存');
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || e?.message || '保存失败');
  } finally {
    savingConfig.value = false;
  }
}

/** 重置为默认全启用顺序并持久化到后端 */
async function resetCustomize() {
  savingConfig.value = true;
  try {
    const defaults = DEFAULT_WIDGETS.map((w) => ({ ...w }));
    await settingsApi.updateSection('dashboard', { widgets: defaults });
    editingConfig.value = defaults.map((w) => ({ ...w }));
    dashboardConfig.value = defaults.map((w) => ({ ...w }));
    Toast.success('已重置为默认配置');
  } catch (e: any) {
    Toast.error(e?.response?.data?.error || e?.message || '重置失败');
  } finally {
    savingConfig.value = false;
  }
}

/** 根据 id 查找组件显示名称 */
function widgetLabel(id: string) {
  return WIDGET_DEFS.find((d) => d.id === id)?.label || id;
}

onMounted(async () => {
  // 先读取 dashboard 配置段，缺失或 widgets 为空时回落默认全启用顺序
  try {
    const { data } = await settingsApi.all();
    const widgets = data?.dashboard?.widgets;
    if (Array.isArray(widgets) && widgets.length > 0) {
      dashboardConfig.value = widgets;
    } else {
      dashboardConfig.value = DEFAULT_WIDGETS.map((w) => ({ ...w }));
    }
  } catch {
    dashboardConfig.value = DEFAULT_WIDGETS.map((w) => ({ ...w }));
  }

  // 加载快捷操作（不阻塞统计渲染）
  loadQuickActions();
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
      <template #actions>
        <button
          @click="openCustomize"
          class="inline-flex items-center gap-1.5 rounded-lg border border-gray-200 px-3 py-1.5 text-sm text-gray-600 transition-colors hover:bg-gray-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
        >
          <Icon icon="ri:settings-3-line" class="text-base" />
          自定义
        </button>
      </template>
    </VPageHeader>

    <div class="m-0 space-y-4 p-4 md:m-4 md:p-0">
      <!-- 统计卡片网格（启用时全宽 4 列置顶渲染） -->
      <div v-if="enabledWidgets.includes('stats-cards')" class="grid grid-cols-2 gap-4 lg:grid-cols-4">
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

      <!-- 其余启用的组件按顺序两两配对双栏，落单单独全宽 -->
      <template v-for="(row, ri) in enabledWidgetRows" :key="ri">
        <div :class="row.length === 2 ? 'grid gap-4 lg:grid-cols-2' : ''">
          <template v-for="wid in row" :key="wid">
            <!-- 待审核提醒（仍受 comment_pending > 0 条件控制） -->
            <router-link
              v-if="wid === 'pending-alert' && stats.comment_pending > 0"
              to="/comments"
              class="flex items-center gap-2 rounded border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-700 transition-colors hover:bg-amber-100 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-400"
              style="border-radius: 4px"
            >
              <Icon icon="ri:error-warning-line" class="text-base" />
              有 {{ stats.comment_pending }} 条评论待审核
              <Icon icon="ri:arrow-right-line" class="ml-auto" />
            </router-link>

            <!-- 快捷操作 -->
            <VCard v-else-if="wid === 'quick-actions'" title="快捷操作">
              <template #actions>
                <button
                  class="rounded p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-gray-700"
                  title="自定义快捷操作"
                  @click="openEditModal"
                >
                  <Icon icon="ri:edit-line" class="text-base" />
                </button>
              </template>
              <div v-if="quickActions.length" class="grid grid-cols-2 gap-3 sm:grid-cols-4">
                <router-link
                  v-for="action in quickActions"
                  :key="action.key"
                  :to="action.to"
                  class="flex flex-col items-center gap-2 rounded border border-gray-100 py-4 text-gray-600 transition-all hover:border-gray-200 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-700/50"
                  style="border-radius: 4px"
                >
                  <Icon :icon="action.icon" class="text-2xl" />
                  <span class="text-xs">{{ action.label }}</span>
                </router-link>
              </div>
              <p v-else class="py-8 text-center text-xs text-gray-400">
                暂无快捷操作，点击右上角编辑按钮添加
              </p>
            </VCard>

            <!-- 最近评论 -->
            <VCard v-else-if="wid === 'recent-comments'" title="最近评论">
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
            <VCard v-else-if="wid === 'visit-trend'">
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
            <VCard v-else-if="wid === 'hourly-dist'">
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
        </div>
      </template>
    </div>

    <!-- 自定义仪表盘 Modal -->
    <VModal :visible="showCustomize" @update:visible="showCustomize = $event" title="自定义仪表盘" :width="480">
      <div class="space-y-2">
        <p class="text-xs text-gray-400 dark:text-gray-500 mb-3">勾选启用组件，上下箭头调整显示顺序。</p>
        <div
          v-for="(w, i) in editingConfig"
          :key="w.id"
          class="flex items-center gap-3 rounded-lg border border-gray-100 px-3 py-2.5 dark:border-gray-700"
        >
          <span class="flex-1 text-sm text-gray-700 dark:text-gray-300">{{ widgetLabel(w.id) }}</span>
          <VSwitch :modelValue="w.enabled" @update:modelValue="w.enabled = $event" />
          <div class="flex items-center gap-0.5">
            <button
              @click="moveWidget(i, -1)"
              :disabled="i === 0"
              class="text-gray-400 hover:text-gray-600 disabled:opacity-30 dark:text-gray-500 dark:hover:text-gray-300"
            >
              <Icon icon="ri:arrow-up-s-line" class="text-lg" />
            </button>
            <button
              @click="moveWidget(i, 1)"
              :disabled="i === editingConfig.length - 1"
              class="text-gray-400 hover:text-gray-600 disabled:opacity-30 dark:text-gray-500 dark:hover:text-gray-300"
            >
              <Icon icon="ri:arrow-down-s-line" class="text-lg" />
            </button>
          </div>
        </div>
      </div>
      <template #footer>
        <div class="flex items-center justify-between">
          <button
            @click="resetCustomize"
            :disabled="savingConfig"
            class="px-3 py-1.5 text-sm text-gray-500 transition-colors hover:text-gray-700 disabled:opacity-50 dark:text-gray-400 dark:hover:text-gray-200"
          >
            重置默认
          </button>
          <div class="flex items-center gap-2">
            <button
              @click="showCustomize = false"
              class="px-3 py-1.5 text-sm text-gray-600 transition-colors hover:text-gray-800 dark:text-gray-300 dark:hover:text-gray-100"
            >
              取消
            </button>
            <button
              @click="saveCustomize"
              :disabled="savingConfig"
              class="rounded-lg bg-primary px-4 py-1.5 text-sm font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-50"
            >
              {{ savingConfig ? '保存中...' : '保存' }}
            </button>
          </div>
        </div>
      </template>
    </VModal>

    <!-- 快捷操作编辑弹窗 -->
    <VModal :visible="showEditModal" title="自定义快捷操作" :width="520" @update:visible="showEditModal = $event">
      <p class="mb-3 text-xs text-gray-500 dark:text-gray-400">
        从以下操作中选择常用的快捷入口（最多 {{ MAX_QUICK_ACTIONS }} 个），已选 {{ editKeys.length }} / {{ MAX_QUICK_ACTIONS }}
      </p>
      <div class="grid grid-cols-2 gap-2 sm:grid-cols-3">
        <button
          v-for="action in availableActions"
          :key="action.key"
          :class="[
            'flex items-center gap-2 rounded border px-3 py-2.5 text-sm transition-all',
            editKeys.includes(action.key)
              ? 'border-primary bg-primary/10 text-primary'
              : 'border-gray-100 text-gray-600 hover:border-gray-200 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-700/50',
          ]"
          style="border-radius: 4px"
          @click="toggleAction(action.key)"
        >
          <Icon :icon="action.icon" class="text-lg" />
          <span class="flex-1 text-left text-xs">{{ action.label }}</span>
          <Icon
            v-if="editKeys.includes(action.key)"
            icon="ri:check-line"
            class="text-base"
          />
        </button>
      </div>
      <template #footer>
        <div class="flex items-center justify-between">
          <button
            class="rounded px-3 py-1.5 text-xs text-gray-500 transition-colors hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700"
            style="border-radius: 4px"
            @click="showEditModal = false"
          >
            取消
          </button>
          <button
            class="rounded bg-primary px-4 py-1.5 text-xs font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-50"
            style="border-radius: 4px"
            :disabled="savingActions"
            @click="saveQuickActions"
          >
            {{ savingActions ? "保存中..." : "保存" }}
          </button>
        </div>
      </template>
    </VModal>
  </div>
</template>
