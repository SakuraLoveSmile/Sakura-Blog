<script setup lang="ts">
import { onMounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import { useRouter } from "vue-router";
import { useBreakpoint } from "../useBreakpoint";

const { breakpoint } = useBreakpoint();
const router = useRouter();

interface PostItem {
  slug: string;
  title: string;
  category: string;
  status: string;
  updated_at: string;
}

interface PostsResp {
  items: PostItem[];
}

const items = ref<PostItem[]>([]);
const loading = ref(true);
const error = ref("");

function timeAgo(iso: string): string {
  const now = new Date();
  const d = new Date(iso);
  const diff = Math.floor((now.getTime() - d.getTime()) / 1000);
  if (diff < 60) return "刚刚";
  if (diff < 3600) return Math.floor(diff / 60) + " 分钟前";
  if (diff < 86400) return Math.floor(diff / 3600) + " 小时前";
  if (diff < 604800) return Math.floor(diff / 86400) + " 天前";
  return d.toLocaleDateString("zh-CN");
}

onMounted(() => {
  fetch("/api/v1/dashboard/recent-posts", { credentials: "include" })
    .then((r) => {
      if (!r.ok) throw new Error("加载文章失败");
      return r.json();
    })
    .then((resp: PostsResp) => (items.value = resp.items))
    .catch((e) => (error.value = e.message))
    .finally(() => (loading.value = false));
});
</script>

<template>
  <div class="apple-card h-full" :data-bp="breakpoint">
    <div class="flex items-center justify-between mb-4">
      <h3 class="apple-section-title">最近文章</h3>
      <button @click="router.push('/posts')" class="apple-link">
        全部
        <Icon icon="ri:arrow-right-s-line" class="w-4 h-4" />
      </button>
    </div>

    <div v-if="loading" class="recent-placeholder flex items-center justify-center text-[#8E8E93]">
      <Icon icon="ri:loader-4-line" class="w-6 h-6 animate-spin" />
    </div>
    <div v-else-if="error" class="recent-placeholder flex items-center justify-center text-red-400">{{ error }}</div>
    <div v-else-if="items.length === 0" class="recent-placeholder flex items-center justify-center text-[#8E8E93] text-sm">
      暂无文章
    </div>
    <div v-else class="space-y-1">
      <button
        v-for="item in items"
        :key="item.slug"
        class="apple-list-item"
        @click="router.push(`/posts/edit/${item.slug}`)"
      >
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2">
            <span
              class="apple-status-dot"
              :class="item.status === 'published' ? 'published' : 'draft'"
            />
            <span class="text-[#EBEBF5] text-sm truncate">{{ item.title }}</span>
          </div>
          <div class="flex items-center gap-2 mt-1 text-xs text-[#8E8E93]">
            <span v-if="item.category" class="flex items-center gap-1">
              <Icon icon="ri:folder-3-line" class="w-3 h-3" />
              {{ item.category }}
            </span>
            <span class="flex items-center gap-1">
              <Icon icon="ri:time-line" class="w-3 h-3" />
              {{ timeAgo(item.updated_at) }}
            </span>
          </div>
        </div>
        <Icon icon="ri:arrow-right-s-line" class="w-4 h-4 text-[#636366] apple-list-arrow" />
      </button>
    </div>
  </div>
</template>

<style scoped>
.recent-placeholder {
  min-height: 200px;
}
.apple-card[data-bp="m"] .recent-placeholder,
.apple-card[data-bp="s"] .recent-placeholder { min-height: 180px; }
.apple-card[data-bp="s"] .recent-placeholder { min-height: 140px; }
</style>
