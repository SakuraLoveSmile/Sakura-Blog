<script setup lang="ts">
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { Icon } from "@iconify/vue";
import { VPageHeader } from "@/components/ui";

const route = useRoute();
const router = useRouter();

const sub = [
  { path: "/seo/overview", label: "概览", icon: "ri:dashboard-line" },
  { path: "/seo/analysis", label: "页面分析", icon: "ri:search-eye-line" },
  { path: "/seo/push", label: "站点推送", icon: "ri:send-plane-line" },
  { path: "/seo/crawlers", label: "爬虫记录", icon: "ri:bug-line" },
  { path: "/seo/traffic", label: "搜索流量", icon: "ri:line-chart-line" },
  { path: "/seo/settings", label: "设置", icon: "ri:settings-3-line" },
];

const active = computed(() => route.path);
</script>

<template>
  <div>
    <VPageHeader title="SEO 工具集">
      <template #icon><Icon icon="ri:search-eye-line" class="text-primary" /></template>
    </VPageHeader>

    <div class="m-0 flex gap-4 p-4 md:m-4 md:p-0">
      <!-- 二级导航 -->
      <nav class="w-40 flex-shrink-0">
        <ul class="space-y-1">
          <li v-for="s in sub" :key="s.path">
            <button
              class="flex w-full items-center gap-2 rounded-base px-3 py-2 text-sm transition-colors"
              :class="active === s.path
                ? 'bg-primary/10 font-medium text-primary'
                : 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-700'"
              @click="router.push(s.path)"
            >
              <Icon :icon="s.icon" class="text-base" />
              {{ s.label }}
            </button>
          </li>
        </ul>
      </nav>

      <!-- 内容区 -->
      <div class="min-w-0 flex-1">
        <router-view />
      </div>
    </div>
  </div>
</template>
