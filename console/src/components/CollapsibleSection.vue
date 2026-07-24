<script setup lang="ts">
import { ref } from "vue";
import { Icon } from "@iconify/vue";

// 可折叠设置卡片：标题栏点击展开/收起，内部记忆状态（默认展开）。
// 用于文章编辑器右侧设置栏，减少纵向占位。
const props = withDefaults(
  defineProps<{
    title: string;
    icon?: string;
    defaultOpen?: boolean;
  }>(),
  { defaultOpen: true }
);
const open = ref(props.defaultOpen);
</script>

<template>
  <div class="bg-white rounded-xl border border-gray-200 dark:bg-gray-800 dark:border-gray-700">
    <button
      type="button"
      @click="open = !open"
      class="w-full flex items-center justify-between px-4 py-3 text-sm font-medium text-gray-700 dark:text-gray-300"
    >
      <span class="flex items-center gap-1.5">
        <Icon v-if="icon" :icon="icon" class="text-base text-gray-400 dark:text-gray-500" />
        {{ title }}
      </span>
      <Icon
        icon="ri:arrow-down-s-line"
        class="text-lg text-gray-400 transition-transform"
        :class="{ '-rotate-90': !open }"
      />
    </button>
    <div v-show="open" class="px-4 pb-4 space-y-3">
      <slot />
    </div>
  </div>
</template>
