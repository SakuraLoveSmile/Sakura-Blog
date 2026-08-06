<script setup lang="ts">
import { onMounted, ref, computed } from"vue";
import { VCard, VTabbar, VEmpty, type TabbarItem } from"@/components/ui";
import { seoApi } from"@/api";

const tabs: TabbarItem[] = [
 { id:"analysis", label:"分析" },
 { id:"landing", label:"着陆页" },
];
const tab = ref("analysis");
const range = ref("7d");
const ranges = [
 { id:"today", label:"今日" },
 { id:"7d", label:"近 7 天" },
 { id:"30d", label:"近 30 天" },
 { id:"90d", label:"近 90 天" },
];

const analysis = ref<any>(null);
const landing = ref<any[]>([]);

async function loadAnalysis() {
 const { data } = await seoApi.trafficAnalysis(range.value);
 analysis.value = data;
}
async function loadLanding() {
 const { data } = await seoApi.trafficLanding(range.value);
 landing.value = data.items || [];
}
onMounted(loadAnalysis);

function switchTab(id: string) {
 tab.value = id;
 if (id ==="landing") loadLanding();
}
function setRange(r: string) {
 range.value = r;
 tab.value ==="analysis" ? loadAnalysis() : loadLanding();
}

const maxDist = computed(() =>
 Math.max(1, ...(analysis.value?.distribution || []).map((d: any) => d.count))
);
const total = computed(() =>
 (analysis.value?.distribution || []).reduce((s: number, d: any) => s + d.count, 0)
);
</script>

<template>
 <div class="space-y-4">
 <VTabbar :items="tabs" :active-id="tab" type="outline" @change="switchTab" />

 <div class="flex gap-1">
 <button
 v-for="r in ranges"
 :key="r.id"
 class="rounded-base px-3 py-1 text-xs transition-colors"
 :class="range === r.id ? 'bg-primary text-white' : 'bg-secondary text-text-tertiary '"
 @click="setRange(r.id)"
 >{{ r.label }}</button>
 </div>

 <template v-if="tab === 'analysis' && analysis">
 <div class="grid grid-cols-2 gap-4 md:grid-cols-4">
 <VCard><div class="p-1"><p class="text-xs text-text-quaternary">今日访问</p>
 <p class="mt-1 text-2xl font-bold text-foreground">{{ analysis.today }}</p></div></VCard>
 <VCard><div class="p-1"><p class="text-xs text-text-quaternary">昨日访问</p>
 <p class="mt-1 text-2xl font-bold text-foreground">{{ analysis.yesterday }}</p></div></VCard>
 <VCard><div class="p-1"><p class="text-xs text-text-quaternary">区间总访问</p>
 <p class="mt-1 text-2xl font-bold text-foreground">{{ analysis.range_total }}</p></div></VCard>
 <VCard><div class="p-1"><p class="text-xs text-text-quaternary">TOP 来源</p>
 <p class="mt-1 text-lg font-semibold text-primary">{{ analysis.top_source ||"—" }}</p></div></VCard>
 </div>

 <VCard title="搜索引擎分布">
 <VEmpty v-if="!analysis.distribution.length" title="选定范围内暂无分布数据" message="尝试切换到更长的日期范围，查看更多搜索引擎分布" />
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

 <template v-else-if="tab === 'landing'">
 <VCard :body-class="['!p-0']">
 <VEmpty v-if="!landing.length" title="选定范围内暂无着陆页数据" message="来自搜索引擎的访问会在此按着陆页汇总" />
 <table v-else class="w-full text-sm">
 <thead class="text-xs text-text-quaternary">
 <tr class="border-b border-border">
 <th class="px-4 py-2 text-left font-normal">着陆页</th>
 <th class="px-4 py-2 text-right font-normal">来自搜索的访问</th>
 </tr>
 </thead>
 <tbody>
 <tr v-for="l in landing" :key="l.path" class="border-b border-border /50">
 <td class="max-w-md truncate px-4 py-2 text-text-tertiary">{{ l.path }}</td>
 <td class="px-4 py-2 text-right tabular-nums text-text-tertiary">{{ l.count }}</td>
 </tr>
 </tbody>
 </table>
 </VCard>
 </template>
 </div>
</template>
