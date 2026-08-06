<script setup lang="ts">
import { onMounted, ref } from"vue";
import { useRouter } from"vue-router";
import { Icon } from"@iconify/vue";
import { VCard, VLoading } from"@/components/ui";
import { seoApi } from"@/api";

const router = useRouter();
const loading = ref(true);
const data = ref<any>(null);

async function load() {
 loading.value = true;
 try {
 const res = await seoApi.overview();
 data.value = res.data;
 } finally {
 loading.value = false;
 }
}
onMounted(load);

const dash = (v: any) => (v ==="" || v == null ?"—" : v);
</script>

<template>
 <div class="space-y-6">
 <VLoading v-if="loading" />
 <template v-else-if="data">
 <!-- 数据概览 -->
 <section>
 <h3 class="mb-3 text-sm font-semibold text-text-tertiary">数据概览</h3>
 <div class="grid grid-cols-2 gap-4 md:grid-cols-4">
 <VCard><div class="p-1">
 <p class="text-xs text-text-quaternary">今日搜索流量</p>
 <p class="mt-1 text-2xl font-bold text-foreground">{{ data.today_search }}</p>
 </div></VCard>
 <VCard><div class="p-1">
 <p class="flex items-center gap-1 text-xs text-text-quaternary">
 今日爬虫
 <span v-if="data.crawl_delta" :class="data.crawl_delta >= 0 ? 'text-green-600' : 'text-red-500'">
 {{ data.crawl_delta >= 0 ?"↑" :"↓" }}{{ Math.abs(data.crawl_delta) }}%
 </span>
 </p>
 <p class="mt-1 text-2xl font-bold text-foreground">{{ data.today_crawl }}</p>
 </div></VCard>
 <VCard><div class="p-1">
 <p class="text-xs text-text-quaternary">近 7 天 TOP 来源</p>
 <p class="mt-1 text-lg font-semibold text-foreground">{{ dash(data.top7_source) }}</p>
 </div></VCard>
 <VCard><div class="p-1">
 <p class="text-xs text-text-quaternary">近 7 天最活跃爬虫</p>
 <p class="mt-1 text-lg font-semibold text-primary">{{ dash(data.top7_crawler) }}</p>
 </div></VCard>
 </div>
 </section>

 <!-- SEO 健康 -->
 <section>
 <h3 class="mb-3 text-sm font-semibold text-text-tertiary">SEO 健康</h3>
 <VCard :body-class="['!p-0']">
 <ul class="divide-y divide-[var(--color-border)]">
 <li v-for="h in data.health" :key="h.key" class="flex items-center gap-3 px-4 py-3">
 <Icon
 :icon="h.status === 'ok' ? 'ri:checkbox-circle-fill' : 'ri:error-warning-fill'"
 :class="h.status === 'ok' ? 'text-green-500' : 'text-amber-500'"
 class="text-lg"
 />
 <div class="min-w-0 flex-1">
 <p class="text-sm font-medium text-foreground">{{ h.label }}</p>
 <p class="text-xs text-text-quaternary">{{ h.hint }}</p>
 </div>
 <span
 class="rounded-full px-2 py-0.5 text-xs"
 :class="h.status === 'ok'
 ? 'bg-green-100 text-green-700 bg-green-500/15 text-green-400'
 : 'bg-amber-100 text-amber-700 bg-amber-500/15 text-amber-400'"
 >{{ h.status ==="ok" ?"正常" :"待优化" }}</span>
 </li>
 </ul>
 </VCard>
 </section>

 <!-- 功能配置 -->
 <section>
 <h3 class="mb-3 text-sm font-semibold text-text-tertiary">功能配置</h3>
 <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
 <VCard><div class="flex items-start justify-between p-1">
 <div>
 <p class="text-sm font-medium text-foreground">站点推送</p>
 <p class="mt-1 text-xs text-text-quaternary">向 IndexNow、百度等搜索引擎推送新内容，加速收录</p>
 </div>
 <span class="text-xs" :class="data.feature_push ? 'text-green-600' : 'text-text-quaternary'">
 {{ data.feature_push ?"● 已开启" :"○ 未开启" }}
 </span>
 </div></VCard>
 <VCard><div class="flex items-start justify-between p-1">
 <div>
 <p class="text-sm font-medium text-foreground">爬虫记录</p>
 <p class="mt-1 text-xs text-text-quaternary">追踪搜索引擎爬虫访问行为，了解抓取频率与范围</p>
 </div>
 <span class="text-xs" :class="data.feature_crawl ? 'text-green-600' : 'text-text-quaternary'">
 {{ data.feature_crawl ?"● 记录已开启" :"○ 未开启" }}
 </span>
 </div></VCard>
 <VCard><div class="flex items-start justify-between p-1">
 <div>
 <p class="text-sm font-medium text-foreground">搜索流量</p>
 <p class="mt-1 text-xs text-text-quaternary">统计来自 Google、Bing 等搜索引擎的访问来源与着陆页</p>
 </div>
 <span class="text-xs" :class="data.feature_traffic ? 'text-green-600' : 'text-text-quaternary'">
 {{ data.feature_traffic ?"● 采集已开启" :"○ 未开启" }}
 </span>
 </div></VCard>
 <VCard><div class="flex items-start justify-between p-1">
 <div>
 <p class="text-sm font-medium text-foreground">搜索引擎验证</p>
 <p class="mt-1 text-xs text-text-quaternary">添加 Google、Bing、百度等验证码，解锁站长工具</p>
 </div>
 <button class="text-xs text-primary hover:underline" @click="router.push('/seo/settings')">
 {{ data.has_verify ?"已配置 →" :"去配置 →" }}
 </button>
 </div></VCard>
 </div>
 </section>
 </template>
 </div>
</template>
