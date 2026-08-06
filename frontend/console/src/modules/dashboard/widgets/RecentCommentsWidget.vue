<script setup lang="ts">
import { ref, onMounted } from"vue";
import { dashboardApi } from"@/api";
import { VCard } from"@/components/ui";

const recentComments = ref<any[]>([]);

onMounted(async () => {
 try {
 const { data } = await dashboardApi.recentComments();
 recentComments.value = data.items || [];
 } catch {}
});
</script>

<template>
 <VCard title="最近评论" class="h-full">
 <div v-if="recentComments.length" class="divide-y divide-[var(--color-border)]">
 <div v-for="c in recentComments.slice(0, 6)" :key="c.id" class="flex items-start gap-3 py-2.5">
 <span class="flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full bg-secondary text-xs font-medium text-text-secondary">
 {{ (c.nick ||"?")[0] }}
 </span>
 <div class="min-w-0 flex-1">
 <div class="flex items-center gap-2">
 <span class="text-xs font-medium text-foreground">{{ c.nick }}</span>
 <span class="text-[10px] text-text-quaternary">{{ c.date?.slice(0, 10) }}</span>
 </div>
 <p class="mt-0.5 truncate text-xs text-text-tertiary">{{ c.content }}</p>
 </div>
 </div>
 </div>
 <p v-else class="py-8 text-center text-xs text-text-tertiary">暂无评论</p>
 </VCard>
</template>
