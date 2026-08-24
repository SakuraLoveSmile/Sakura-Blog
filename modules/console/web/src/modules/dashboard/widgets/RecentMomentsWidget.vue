<script setup lang="ts">
import { onMounted, ref } from "vue";
import { momentApi } from "@/api";

interface MomentItem {
  id: number;
  content_md: string;
  content?: string;
  created_at: string;
  visibility?: string;
}

const items = ref<MomentItem[]>([]);
const loading = ref(true);

function excerpt(item: MomentItem): string {
  const text = (item.content || item.content_md || "")
    .replace(/[#*_`>\[\]()]/g, "")
    .replace(/\s+/g, " ")
    .trim();
  return text || "（图片说说）";
}

function dateLabel(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : date.toLocaleDateString("zh-CN", { month: "short", day: "numeric" });
}

async function load() {
  loading.value = true;
  try {
    const { data } = await momentApi.list({ page: 1, size: 5 });
    items.value = Array.isArray(data?.items) ? data.items : [];
  } catch {
    items.value = [];
  } finally {
    loading.value = false;
  }
}

onMounted(load);
</script>

<template>
  <div class="apple-card recent-moments-widget flex h-full flex-col" data-widget="recent-moments">
    <div class="flex items-center justify-between px-4 pt-4">
      <div>
        <div class="text-sm font-semibold text-foreground">最近说说</div>
        <div class="mt-0.5 text-[11px] text-text-tertiary">记录最近的瞬间</div>
      </div>
      <button type="button" class="apple-link text-xs" @click="$router.push('/moments')">查看全部</button>
    </div>
    <div v-if="loading" class="flex flex-1 items-center justify-center text-xs text-text-tertiary">加载中…</div>
    <div v-else-if="!items.length" class="flex flex-1 items-center justify-center text-xs text-text-tertiary">
      暂无说说
    </div>
    <ul v-else class="mt-3 flex-1 divide-y divide-border/60 overflow-hidden px-4">
      <li v-for="item in items" :key="item.id" class="flex gap-3 py-3 first:pt-0">
        <span class="mt-1.5 h-2 w-2 shrink-0 rounded-full bg-primary" />
        <div class="min-w-0 flex-1">
          <p class="line-clamp-2 text-xs leading-relaxed text-text-secondary">{{ excerpt(item) }}</p>
          <time class="mt-1 block text-[10px] text-text-tertiary">{{ dateLabel(item.created_at) }}</time>
        </div>
      </li>
    </ul>
  </div>
</template>
