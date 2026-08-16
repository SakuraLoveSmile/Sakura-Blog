<script setup lang="ts">
import { ref } from"vue";
import { Icon } from"@iconify/vue";
import { VCard, VButton } from"@/components/ui";
import { seoApi } from"@/api";

const url = ref("");
const loading = ref(false);
const result = ref<any>(null);
const errMsg = ref("");

async function analyze() {
 loading.value = true;
 errMsg.value ="";
 result.value = null;
 try {
 const { data } = await seoApi.analyze(url.value.trim());
 if (data.error) {
 errMsg.value = data.error;
 } else {
 result.value = data;
 }
 } catch (e: any) {
 errMsg.value = e?.response?.data?.error ||"检查失败";
 } finally {
 loading.value = false;
 }
}

// 环形进度：dasharray 计算
const R = 52;
const C = 2 * Math.PI * R;
const dash = (score: number) => `${(score / 100) * C} ${C}`;
</script>

<template>
 <div class="space-y-4">
 <VCard>
 <div class="flex items-center gap-2">
 <input
 v-model="url"
 class="w-full rounded-base border border-border px-3 py-2 text-sm outline-none focus:border-primary"
 placeholder="输入要检查的页面 URL（留空则检查站点首页）"
 @keyup.enter="analyze"
 />
 <VButton type="secondary" :loading="loading" @click="analyze">检查</VButton>
 </div>
 </VCard>

 <VCard v-if="errMsg">
 <p class="flex items-center gap-2 text-sm text-state-error">
 <Icon icon="ri:error-warning-line" /> {{ errMsg }}
 </p>
 </VCard>

 <template v-if="result">
 <!-- 评分 -->
 <VCard>
 <div class="flex flex-col items-center py-4">
 <div class="relative h-32 w-32">
 <svg class="h-32 w-32 -rotate-90" viewBox="0 0 120 120">
 <circle cx="60" cy="60" :r="R" fill="none" stroke="currentColor" class="text-text-secondary" stroke-width="12" />
 <circle cx="60" cy="60" :r="R" fill="none" stroke="currentColor" class="text-primary" stroke-width="12"
 stroke-linecap="round" :stroke-dasharray="dash(result.score)" />
 </svg>
 <div class="absolute inset-0 flex items-center justify-center text-2xl font-bold text-foreground">
 {{ result.score }}%
 </div>
 </div>
 <p class="mt-2 truncate text-xs text-text-quaternary">{{ result.url }}</p>
 </div>
 </VCard>

 <!-- 可优化项 -->
 <VCard v-if="result.issues.length" title="可优化项">
 <ul class="space-y-2">
 <li v-for="it in result.issues" :key="it.key" class="flex items-center gap-2 text-sm text-text-secondary">
 <Icon icon="ri:error-warning-line" class="text-state-warning" /> {{ it.label }}
 </li>
 </ul>
 </VCard>

 <!-- 已通过的检查 -->
 <VCard v-if="result.passed.length" title="已通过的检查">
 <ul class="space-y-2">
 <li v-for="it in result.passed" :key="it.key" class="flex items-center gap-2 text-sm text-text-tertiary">
 <Icon icon="ri:checkbox-circle-line" class="text-state-success" /> {{ it.label }}
 </li>
 </ul>
 </VCard>
 </template>
 </div>
</template>
