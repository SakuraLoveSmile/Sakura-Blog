<script setup lang="ts">
import { onMounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import { useRouter } from "vue-router";
import { useBreakpoint } from "../useBreakpoint"
import { dashboardApi } from "@/api";
import { formatRelative } from "@/lib/format";

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


onMounted(() => {
  dashboardApi.recentPosts()
    .then((r: { data: PostsResp }) => (items.value = r.data.items))
    .catch((e) => (error.value = e.message))
    .finally(() => (loading.value = false));
});
</script>

<template>
  <div class="apple-card h-full" :data-bp="breakpoint">
    <div class="flex items-center justify-between mb-4">
      <div class="flex items-baseline gap-2.5">
        <h3 class="apple-section-title">最近文章</h3>
        <!-- 状态点图例：绿=已发布，黄=草稿 -->
        <span class="recent-legend">
          <span class="recent-legend-item"><span class="apple-status-dot published" />已发布</span>
          <span class="recent-legend-item"><span class="apple-status-dot draft" />草稿</span>
        </span>
      </div>
      <button @click="router.push('/posts')" class="apple-link">
        全部
        <Icon icon="ri:arrow-right-s-line" class="w-4 h-4" />
      </button>
    </div>

    <div v-if="loading" class="recent-placeholder flex items-center justify-center text-muted-foreground">
      <Icon icon="ri:loader-4-line" class="w-6 h-6 animate-spin" />
    </div>
    <div v-else-if="error" class="recent-placeholder flex items-center justify-center text-state-error">{{ error }}</div>
    <div v-else-if="items.length === 0" class="recent-placeholder flex items-center justify-center text-muted-foreground text-sm">
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
              :title="item.status === 'published' ? '已发布' : '草稿'"
            />
            <span class="text-foreground text-sm truncate">{{ item.title }}</span>
          </div>
          <div class="flex items-center gap-2 mt-1 text-xs text-muted-foreground">
            <span v-if="item.category" class="flex items-center gap-1">
              <Icon icon="ri:folder-3-line" class="w-3 h-3" />
              {{ item.category }}
            </span>
            <span class="flex items-center gap-1">
              <Icon icon="ri:time-line" class="w-3 h-3" />
              {{ formatRelative(item.updated_at) }}
            </span>
          </div>
        </div>
        <Icon icon="ri:arrow-right-s-line" class="w-4 h-4 text-text-quaternary apple-list-arrow" />
      </button>
    </div>
  </div>
</template>

<style scoped>
.recent-placeholder {
  min-height: 200px;
}
.recent-legend {
  display: inline-flex;
  align-items: center;
  gap: 0.6rem;
  font-size: 11px;
  color: var(--color-text-muted, var(--text-muted, #8e8e93));
}
.recent-legend-item {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  white-space: nowrap;
}
.apple-card:is([data-bp="md"], [data-bp="sm"]) .recent-placeholder,
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .recent-placeholder { min-height: 180px; }
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .recent-placeholder { min-height: 140px; }
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .recent-legend { display: none; }
</style>
