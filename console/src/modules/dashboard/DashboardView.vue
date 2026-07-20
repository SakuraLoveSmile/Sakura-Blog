<script setup lang="ts">
import { ref, onMounted } from "vue";
import { dashboardApi } from "@/api";
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

onMounted(async () => {
  try {
    const { data } = await dashboardApi.stats();
    stats.value = data;
  } catch {}
  try {
    const { data } = await dashboardApi.recentComments();
    recentComments.value = data.items || [];
  } catch {}
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
  </div>
</template>
