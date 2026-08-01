<script setup lang="ts">
import { ref, onMounted } from "vue";
import { dashboardApi } from "@/api";
import { VCard } from "@/components/ui";

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
</template>
