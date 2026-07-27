<script setup lang="ts">
import { Icon } from "@iconify/vue";
import { VCard } from "@/components/ui";

const quickActions = [
  { label: "写文章", icon: "ri:quill-pen-line", to: "/posts/new" },
  { label: "审核评论", icon: "ri:chat-check-line", to: "/comments" },
  { label: "上传附件", icon: "ri:upload-cloud-line", to: "/attachments" },
  { label: "系统设置", icon: "ri:settings-3-line", to: "/settings" },
];
</script>

<template>
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
</template>
