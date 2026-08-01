<script setup lang="ts">
import { computed } from "vue";

// Halo VPagination 复刻（页码 + 总数 + 每页条数）
const props = withDefaults(
  defineProps<{
    page: number;
    size: number;
    total: number;
    sizeOptions?: number[];
  }>(),
  { sizeOptions: () => [20, 30, 50, 100] }
);

const emit = defineEmits<{
  (event: "update:page", page: number): void;
  (event: "update:size", size: number): void;
  (event: "change"): void;
}>();

const totalPages = computed(() => Math.max(1, Math.ceil(props.total / props.size)));

// 简化页码序列：首尾 + 当前页附近，超出用省略号
const pageItems = computed<(number | "...")[]>(() => {
  const tp = totalPages.value;
  const cur = props.page;
  if (tp <= 7) return Array.from({ length: tp }, (_, i) => i + 1);
  const items: (number | "...")[] = [1];
  if (cur > 3) items.push("...");
  for (let i = Math.max(2, cur - 1); i <= Math.min(tp - 1, cur + 1); i++) items.push(i);
  if (cur < tp - 2) items.push("...");
  items.push(tp);
  return items;
});

function goPage(p: number) {
  if (p < 1 || p > totalPages.value || p === props.page) return;
  emit("update:page", p);
  emit("change");
}

function changeSize(e: Event) {
  const val = Number((e.target as HTMLSelectElement).value);
  emit("update:size", val);
  emit("update:page", 1);
  emit("change");
}
</script>

<template>
  <div class="flex items-center justify-end gap-4">
    <span class="text-xs text-gray-500 dark:text-gray-400">共 {{ total }} 条</span>
    <div class="flex items-center gap-1">
      <button
        :disabled="page <= 1"
        class="flex h-7 w-7 items-center justify-center rounded text-xs text-gray-600 transition-colors hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-40 dark:text-gray-400 dark:hover:bg-gray-700"
        @click="goPage(page - 1)"
      >
        &lt;
      </button>
      <template v-for="(item, idx) in pageItems" :key="idx">
        <span v-if="item === '...'" class="px-1 text-xs text-gray-400">…</span>
        <button
          v-else
          :class="[
            'h-7 w-7 rounded text-xs transition-colors',
            page === item
              ? 'bg-secondary text-white'
              : 'text-gray-600 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700',
          ]"
          @click="goPage(item)"
        >
          {{ item }}
        </button>
      </template>
      <button
        :disabled="page >= totalPages"
        class="flex h-7 w-7 items-center justify-center rounded text-xs text-gray-600 transition-colors hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-40 dark:text-gray-400 dark:hover:bg-gray-700"
        @click="goPage(page + 1)"
      >
        &gt;
      </button>
    </div>
    <select
      :value="size"
      class="rounded border border-gray-200 px-2 py-1 text-xs text-gray-600 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-300"
      @change="changeSize"
    >
      <option v-for="opt in sizeOptions" :key="opt" :value="opt">{{ opt }} 条/页</option>
    </select>
  </div>
</template>
