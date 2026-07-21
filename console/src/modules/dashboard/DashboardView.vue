<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { dashboardApi, statsApi } from "@/api";
import { Icon } from "@iconify/vue";

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
  { key: "post_total", label: "文章总数", icon: "ri:article-line", color: "text-blue-600 bg-blue-50 dark:bg-blue-500/15 dark:text-blue-400" },
  { key: "comment_total", label: "评论总数", icon: "ri:chat-3-line", color: "text-green-600 bg-green-50 dark:bg-green-500/15 dark:text-green-400" },
  { key: "view_total", label: "总浏览量", icon: "ri:eye-line", color: "text-purple-600 bg-purple-50 dark:bg-purple-500/15 dark:text-purple-400" },
  { key: "attachment_total", label: "附件数", icon: "ri:image-2-line", color: "text-orange-600 bg-orange-50 dark:bg-orange-500/15 dark:text-orange-400" },
];

// --- Spec 24: 访问统计 ---
const trendDays = ref(7);
const trend = ref<{ date: string; pv: number; uv: number }[]>([]);
const popular = ref<{ path: string; title: string; pv: number; uv: number }[]>([]);

const chartW = 600;
const chartH = 160;
const padX = 40;
const padY = 20;

function polylinePoints(key: "pv" | "uv") {
  const data = trend.value;
  if (!data.length) return "";
  const maxVal = Math.max(...data.map((d) => Math.max(d.pv, d.uv)), 1);
  const stepX = (chartW - padX * 2) / Math.max(data.length - 1, 1);
  return data
    .map((d, i) => {
      const x = padX + i * stepX;
      const y = chartH - padY - ((d[key] / maxVal) * (chartH - padY * 2));
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");
}

const pvPoints = computed(() => polylinePoints("pv"));
const uvPoints = computed(() => polylinePoints("uv"));
const maxVal = computed(() => Math.max(...trend.value.map((d) => Math.max(d.pv, d.uv)), 1));

// X 轴日期标签（稀疏显示）
const xLabels = computed(() => {
  const data = trend.value;
  if (!data.length) return [];
  const step = Math.max(1, Math.floor(data.length / 6));
  const stepX = (chartW - padX * 2) / Math.max(data.length - 1, 1);
  return data
    .filter((_, i) => i % step === 0 || i === data.length - 1)
    .map((d) => ({
      x: padX + data.indexOf(d) * stepX,
      label: d.date.slice(5), // MM-DD
    }));
});

async function loadStats() {
  try {
    const { data } = await statsApi.trend(trendDays.value);
    trend.value = data.trend || [];
  } catch {}
  try {
    const { data } = await statsApi.popular(trendDays.value, 10);
    popular.value = data.items || [];
  } catch {}
}

function switchDays(d: number) {
  trendDays.value = d;
  loadStats();
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
  loadStats();
});
</script>

<template>
  <div class="p-6">
    <h2 class="text-xl font-semibold text-gray-800 mb-6 dark:text-gray-100">仪表盘</h2>

    <!-- Stats cards -->
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
      <div
        v-for="card in cards"
        :key="card.key"
        class="bg-white rounded-xl border border-gray-200 p-5 flex items-center gap-4 dark:bg-gray-800 dark:border-gray-700"
      >
        <div :class="['w-10 h-10 rounded-lg flex items-center justify-center', card.color]">
          <Icon :icon="card.icon" class="text-xl" />
        </div>
        <div>
          <p class="text-2xl font-bold text-gray-800 dark:text-gray-100">
            {{ (stats as any)[card.key] }}
          </p>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ card.label }}</p>
        </div>
      </div>
    </div>

    <!-- Pending comments alert -->
    <div
      v-if="stats.comment_pending > 0"
      class="bg-amber-50 border border-amber-200 rounded-lg px-4 py-3 mb-6 text-sm text-amber-700 dark:bg-amber-500/10 dark:border-amber-500/30 dark:text-amber-400"
    >
      有 {{ stats.comment_pending }} 条评论待审核
      <router-link to="/comments" class="underline ml-1">去审核</router-link>
    </div>

    <!-- Recent comments -->
    <div class="bg-white rounded-xl border border-gray-200 dark:bg-gray-800 dark:border-gray-700">
      <div class="px-5 py-4 border-b border-gray-100 dark:border-gray-700">
        <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300">最近评论</h3>
      </div>
      <div class="divide-y divide-gray-50 dark:divide-gray-700">
        <div v-for="cm in recentComments" :key="cm.id" class="px-5 py-3 flex items-start gap-3">
          <img :src="cm.avatar" class="w-8 h-8 rounded-full" />
          <div class="flex-1 min-w-0">
            <p class="text-sm">
              <span class="font-medium text-gray-800 dark:text-gray-100">{{ cm.nick }}</span>
              <span class="text-gray-400 ml-2 text-xs dark:text-gray-500">{{ cm.date }}</span>
            </p>
            <p class="text-sm text-gray-500 truncate dark:text-gray-400">{{ cm.content }}</p>
          </div>
        </div>
        <p v-if="recentComments.length === 0" class="px-5 py-8 text-center text-sm text-gray-400 dark:text-gray-500">
          暂无评论
        </p>
      </div>
    </div>

    <!-- PV/UV 趋势图 (Spec 24) -->
    <div class="bg-white rounded-xl border border-gray-200 mt-6 dark:bg-gray-800 dark:border-gray-700">
      <div class="px-5 py-4 border-b border-gray-100 dark:border-gray-700 flex items-center justify-between">
        <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300">访问趋势</h3>
        <div class="flex gap-1">
          <button
            @click="switchDays(7)"
            :class="['px-2.5 py-1 text-xs rounded-md transition', trendDays === 7 ? 'bg-blue-600 text-white' : 'text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-700']"
          >7 天</button>
          <button
            @click="switchDays(30)"
            :class="['px-2.5 py-1 text-xs rounded-md transition', trendDays === 30 ? 'bg-blue-600 text-white' : 'text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-700']"
          >30 天</button>
        </div>
      </div>
      <div class="px-5 py-4">
        <div class="flex items-center gap-4 mb-2 text-xs text-gray-500 dark:text-gray-400">
          <span class="flex items-center gap-1"><span class="w-3 h-0.5 bg-blue-500 inline-block rounded"></span>PV</span>
          <span class="flex items-center gap-1"><span class="w-3 h-0.5 bg-green-500 inline-block rounded"></span>UV</span>
        </div>
        <svg :viewBox="`0 0 ${chartW} ${chartH}`" class="w-full h-40" preserveAspectRatio="xMidYMid meet">
          <!-- Y 轴参考线 -->
          <line :x1="padX" :y1="padY" :x2="padX" :y2="chartH - padY" stroke="currentColor" class="text-gray-200 dark:text-gray-600" stroke-width="0.5" />
          <line :x1="padX" :y1="chartH - padY" :x2="chartW - padX" :y2="chartH - padY" stroke="currentColor" class="text-gray-200 dark:text-gray-600" stroke-width="0.5" />
          <!-- Y 轴最大值 -->
          <text :x="padX - 4" :y="padY + 4" text-anchor="end" class="fill-gray-400 text-[9px]">{{ maxVal }}</text>
          <text :x="padX - 4" :y="chartH - padY" text-anchor="end" class="fill-gray-400 text-[9px]">0</text>
          <!-- PV 折线 -->
          <polyline :points="pvPoints" fill="none" stroke="#3b82f6" stroke-width="2" stroke-linejoin="round" stroke-linecap="round" />
          <!-- UV 折线 -->
          <polyline :points="uvPoints" fill="none" stroke="#22c55e" stroke-width="2" stroke-linejoin="round" stroke-linecap="round" />
          <!-- X 轴日期 -->
          <text v-for="lb in xLabels" :key="lb.x" :x="lb.x" :y="chartH - 4" text-anchor="middle" class="fill-gray-400 text-[9px]">{{ lb.label }}</text>
        </svg>
      </div>
    </div>

    <!-- 热门文章榜 (Spec 24) -->
    <div class="bg-white rounded-xl border border-gray-200 mt-6 dark:bg-gray-800 dark:border-gray-700">
      <div class="px-5 py-4 border-b border-gray-100 dark:border-gray-700">
        <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300">热门文章（近 {{ trendDays }} 天）</h3>
      </div>
      <div class="divide-y divide-gray-50 dark:divide-gray-700">
        <div v-for="(item, idx) in popular" :key="item.path" class="px-5 py-3 flex items-center gap-3">
          <span class="w-5 text-center text-xs font-bold" :class="idx < 3 ? 'text-blue-600' : 'text-gray-400'">{{ idx + 1 }}</span>
          <span class="flex-1 text-sm text-gray-700 truncate dark:text-gray-300">{{ item.title }}</span>
          <span class="text-xs text-gray-400">PV {{ item.pv }} · UV {{ item.uv }}</span>
        </div>
        <p v-if="popular.length === 0" class="px-5 py-8 text-center text-sm text-gray-400 dark:text-gray-500">
          暂无访问数据
        </p>
      </div>
    </div>
  </div>
</template>
