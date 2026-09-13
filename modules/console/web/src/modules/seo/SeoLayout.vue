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

    <div class="m-0 flex flex-col gap-4 p-4 md:m-4 md:p-0 md:flex-row">
      <!-- 二级导航 -->
      <nav class="w-full flex-shrink-0 md:w-40">
        <ul class="flex gap-1 overflow-x-auto md:block md:space-y-1 md:gap-0">
          <li v-for="s in sub" :key="s.path">
            <button
              class="flex h-[38px] w-full shrink-0 items-center gap-2 whitespace-nowrap rounded-[var(--radius-md)] px-3 text-[13.5px] transition-all"
              :class="
                active === s.path
                  ? 'bg-primary font-semibold text-primary-foreground shadow-sm'
                  : 'text-text-secondary hover:bg-secondary hover:text-foreground'
              "
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
