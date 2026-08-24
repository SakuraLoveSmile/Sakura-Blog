<script setup lang="ts">
import { onMounted, ref, computed } from "vue";
import { VCard, VTabbar, VEmpty, type TabbarItem } from "@/components/ui";
import { seoApi } from "@/api";

const tabs: TabbarItem[] = [
  { id: "analysis", label: "分析" },
  { id: "logs", label: "记录" },
];
const tab = ref("analysis");
const range = ref("7d");
const ranges = [
  { id: "today", label: "今日" },
  { id: "7d", label: "近 7 天" },
  { id: "30d", label: "近 30 天" },
  { id: "90d", label: "近 90 天" },
];

const analysis = ref<any>(null);
const logs = ref<any[]>([]);
const logsTotal = ref(0);

async function loadAnalysis() {
  try {
    const { data } = await seoApi.crawlerAnalysis(range.value);
    analysis.value = data;
  } catch {
    // Spec 62：失败显示空态而非白屏
    analysis.value = null;
  }
}
async function loadLogs() {
  try {
    const { data } = await seoApi.crawlerLogs({ page: 1, size: 50 });
    logs.value = data.items || [];
    logsTotal.value = data.total || 0;
  } catch {
    logs.value = [];
    logsTotal.value = 0;
  }
}
onMounted(loadAnalysis);

function switchTab(id: string) {
  tab.value = id;
  if (id === "logs" && !logs.value.length) loadLogs();
}
function setRange(r: string) {
  range.value = r;
  loadAnalysis();
}

const maxDist = computed(() => Math.max(1, ...(analysis.value?.distribution || []).map((d: any) => d.count)));
const total = computed(() => (analysis.value?.distribution || []).reduce((s: number, d: any) => s + d.count, 0));
</script>

<template>
  <div class="space-y-4">
    <VTabbar :items="tabs" :active-id="tab" type="outline" @change="switchTab" />

    <template v-if="tab === 'analysis'">
      <!-- 时间范围 -->
      <div class="flex gap-1">
        <button
          v-for="r in ranges"
          :key="r.id"
          class="rounded-base px-3 py-1 text-xs transition-colors"
          :class="range === r.id ? 'bg-primary text-primary-foreground' : 'bg-secondary text-text-tertiary '"
          @click="setRange(r.id)"
        >
          {{ r.label }}
        </button>
      </div>

      <template v-if="analysis">
        <!-- 统计卡 -->
        <div class="grid grid-cols-2 gap-4 md:grid-cols-4">
          <VCard
            ><div class="p-1">
              <p class="text-xs text-text-quaternary">今日爬取</p>
              <p class="mt-1 text-2xl font-bold text-foreground">{{ analysis.today }}</p>
            </div></VCard
          >
          <VCard
            ><div class="p-1">
              <p class="text-xs text-text-quaternary">昨日爬取</p>
              <p class="mt-1 text-2xl font-bold text-foreground">{{ analysis.yesterday }}</p>
            </div></VCard
          >
          <VCard
            ><div class="p-1">
              <p class="text-xs text-text-quaternary">区间总爬取</p>
              <p class="mt-1 text-2xl font-bold text-foreground">{{ analysis.range_total }}</p>
            </div></VCard
          >
          <VCard
            ><div class="p-1">
              <p class="text-xs text-text-quaternary">最活跃爬虫</p>
              <p class="mt-1 text-lg font-semibold text-primary">{{ analysis.top_engine || "—" }}</p>
            </div></VCard
          >
        </div>

        <!-- 引擎分布 -->
        <VCard title="搜索引擎分布">
          <VEmpty
            v-if="!analysis.distribution.length"
            title="暂无爬虫数据"
            message="搜索引擎爬虫访问后这里会显示分布"
          />
          <ul v-else class="space-y-3">
            <li v-for="d in analysis.distribution" :key="d.engine">
              <div class="mb-1 flex justify-between text-xs text-text-tertiary">
                <span>{{ d.engine }}</span>
                <span>{{ d.count }} · {{ Math.round((d.count / total) * 100) }}%</span>
              </div>
              <div class="h-2 w-full overflow-hidden rounded-full bg-secondary">
                <div class="h-full rounded-full bg-primary" :style="{ width: (d.count / maxDist) * 100 + '%' }"></div>
              </div>
            </li>
          </ul>
        </VCard>
      </template>
    </template>

    <template v-else>
      <VCard :body-class="['!p-0']">
        <VEmpty v-if="!logs.length" title="暂无爬虫记录" message="搜索引擎爬虫访问后这里会显示明细" />
        <table v-else class="w-full text-sm">
          <thead class="text-xs text-text-quaternary">
            <tr class="border-b border-border">
              <th class="px-4 py-2 text-left font-normal">引擎</th>
              <th class="px-4 py-2 text-left font-normal">路径</th>
              <th class="px-4 py-2 text-left font-normal">时间</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="l in logs" :key="l.id" class="border-b border-border /50">
              <td class="px-4 py-2 text-text-tertiary">{{ l.engine }}</td>
              <td class="max-w-xs truncate px-4 py-2 text-text-tertiary">{{ l.path }}</td>
              <td class="px-4 py-2 text-xs text-text-quaternary">{{ l.created_at }}</td>
            </tr>
          </tbody>
        </table>
      </VCard>
    </template>
  </div>
</template>
