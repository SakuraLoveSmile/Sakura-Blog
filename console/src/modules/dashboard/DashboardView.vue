<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { dashboardApi, statsApi } from "@/api";
import { Icon } from "@iconify/vue";
import { VPageHeader, VCard } from "@/components/ui";

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
const popular = ref<{ path: string; title: string; pv: number; uv: number }[]>([]);

const chartW = 600;
const chartH = 160;
const padX = 40;
const padY = 20;

function polylinePoints(key: "pv" | "uv") {
  const data = trend.value;
  if (!data.length) return "";
  const maxV = Math.max(...data.map((d) => Math.max(d.pv, d.uv)), 1);
  const stepX = (chartW - padX * 2) / Math.max(data.length - 1, 1);
  return data
    .map((d, i) => {
      const x = padX + i * stepX;
      const y = chartH - padY - ((d[key] / maxV) * (chartH - padY * 2));
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");
}

const pvPoints = computed(() => polylinePoints("pv"));
const uvPoints = computed(() => polylinePoints("uv"));
const maxVal = computed(() => Math.max(...trend.value.map((d) => Math.max(d.pv, d.uv)), 1));

const xLabels = computed(() => {
  const data = trend.value;
  if (!data.length) return [];
  const step = Math.max(1, Math.floor(data.length / 6));
  const stepX = (chartW - padX * 2) / Math.max(data.length - 1, 1);
  return data
    .filter((_, i) => i % step === 0 || i === data.length - 1)
    .map((d) => ({
      x: padX + data.indexOf(d) * stepX,
      label: d.date.slice(5),
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
                {{ (c.author || "?")[0] }}
              </span>
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-2">
                  <span class="text-xs font-medium text-gray-800 dark:text-gray-200">{{ c.author }}</span>
                  <span class="text-[10px] text-gray-400">{{ c.created_at?.slice(0, 10) }}</span>
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
            <div class="flex w-full items-center justify-between px-4 py-3">
              <span class="text-base font-bold text-gray-800 dark:text-gray-100">访问趋势</span>
              <div class="flex gap-1">
                <button
                  v-for="d in [7, 30]"
                  :key="d"
                  :class="[
                    'rounded px-2 py-1 text-xs transition-colors',
                    trendDays === d
                      ? 'bg-secondary text-white'
                      : 'text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700',
                  ]"
                  style="border-radius: 4px"
                  @click="switchDays(d)"
                >
                  {{ d }} 天
                </button>
              </div>
            </div>
          </template>
          <svg v-if="trend.length" :viewBox="`0 0 ${chartW} ${chartH}`" class="w-full">
            <line
              v-for="frac in [0, 0.5, 1]"
              :key="frac"
              :x1="padX"
              :x2="chartW - padX"
              :y1="chartH - padY - frac * (chartH - padY * 2)"
              :y2="chartH - padY - frac * (chartH - padY * 2)"
              stroke="#e5e7eb"
              stroke-dasharray="4 4"
              stroke-width="1"
            />
            <text
              v-for="frac in [0, 0.5, 1]"
              :key="'t' + frac"
              :x="padX - 6"
              :y="chartH - padY - frac * (chartH - padY * 2) + 3"
              fill="#9ca3af"
              font-size="9"
              text-anchor="end"
            >
              {{ Math.round(maxVal * frac) }}
            </text>
            <polyline :points="pvPoints" fill="none" stroke="#4ccba0" stroke-width="2" />
            <polyline :points="uvPoints" fill="none" stroke="#0ea5e9" stroke-width="2" />
            <text v-for="lb in xLabels" :key="lb.x" :x="lb.x" :y="chartH - 4" fill="#9ca3af" font-size="9" text-anchor="middle">
              {{ lb.label }}
            </text>
          </svg>
          <p v-else class="py-8 text-center text-xs text-gray-400">暂无数据</p>
          <div class="mt-2 flex items-center justify-center gap-4 text-xs text-gray-500 dark:text-gray-400">
            <span class="flex items-center gap-1.5"><span class="h-2 w-2 rounded-full" style="background: #4ccba0"></span>PV</span>
            <span class="flex items-center gap-1.5"><span class="h-2 w-2 rounded-full" style="background: #0ea5e9"></span>UV</span>
          </div>
        </VCard>
      </div>

      <!-- 热门文章 -->
      <VCard title="热门文章">
        <div v-if="popular.length" class="divide-y divide-gray-50 dark:divide-gray-700">
          <div v-for="(p, i) in popular" :key="p.path" class="flex items-center gap-3 py-2.5">
            <span
              :class="[
                'flex h-5 w-5 flex-shrink-0 items-center justify-center rounded text-[10px] font-bold',
                i < 3 ? 'bg-secondary text-white' : 'bg-gray-100 text-gray-500 dark:bg-gray-700 dark:text-gray-400',
              ]"
              style="border-radius: 4px"
            >
              {{ i + 1 }}
            </span>
            <span class="min-w-0 flex-1 truncate text-xs text-gray-700 dark:text-gray-300">{{ p.title || p.path }}</span>
            <span class="flex-shrink-0 text-[10px] tabular-nums text-gray-400">PV {{ p.pv }}</span>
          </div>
        </div>
        <p v-else class="py-8 text-center text-xs text-gray-400">暂无数据</p>
      </VCard>
    </div>
  </div>
</template>
