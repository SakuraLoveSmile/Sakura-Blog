<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { Icon } from "@iconify/vue";
import { useRouter } from "vue-router";
import { useBreakpoint } from "../useBreakpoint";
import { dashboardApi } from "@/api";
import { formatRelative } from "@/lib/format";
import type { WidgetSizePreset } from "./layout";
import { useWidgetSize } from "../useWidgetSize";
import { DASHBOARD_REFRESH_EVENT } from "./useDashboardStats";

interface Props {
  sizePreset?: WidgetSizePreset;
}

const props = withDefaults(defineProps<Props>(), {
  sizePreset: "medium",
});
const sizePreset = useWidgetSize(props.sizePreset).preset;

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

function loadPosts() {
  loading.value = true;
  error.value = "";
  dashboardApi.recentPosts()
    .then((r: { data: PostsResp }) => (items.value = r.data.items))
    .catch((e) => (error.value = e instanceof Error ? e.message : "文章加载失败"))
    .finally(() => (loading.value = false));
}

onMounted(() => {
  loadPosts();
  window.addEventListener(DASHBOARD_REFRESH_EVENT, loadPosts);
});
onUnmounted(() => window.removeEventListener(DASHBOARD_REFRESH_EVENT, loadPosts));

const displayedItems = computed(() => {
  if (sizePreset.value === "small") return items.value.slice(0, 3);
  if (sizePreset.value === "medium") return items.value.slice(0, 5);
  return items.value.slice(0, 8);
});

function editPost(item: PostItem) {
  router.push(`/posts/edit/${item.slug}`);
}

function previewPost(item: PostItem) {
  window.open(`/post/${encodeURIComponent(item.slug)}`, "_blank", "noopener,noreferrer");
}
</script>

<template>
  <div class="apple-card h-full flex flex-col justify-between" :class="`recent-posts-${sizePreset}`" :data-bp="breakpoint">
    <div class="flex items-center justify-between mb-3">
      <div class="flex items-baseline gap-2.5">
        <h3 class="apple-section-title" :class="{ 'text-sm': sizePreset === 'small' }">最近文章</h3>
        <!-- 状态点图例 -->
        <span v-if="sizePreset !== 'small'" class="recent-legend">
          <span class="recent-legend-item"><span class="apple-status-dot published" />已发布</span>
          <span class="recent-legend-item"><span class="apple-status-dot draft" />草稿</span>
        </span>
      </div>
      <button @click="router.push('/posts')" class="apple-link text-xs">
        全部
        <Icon icon="ri:arrow-right-s-line" class="w-4 h-4" />
      </button>
    </div>

    <div v-if="loading" class="dashboard-skeleton-list flex-1" aria-label="正在加载文章">
      <span v-for="index in (sizePreset === 'large' ? 5 : 3)" :key="index" class="dashboard-skeleton-row">
        <i class="dashboard-skeleton-avatar" />
        <i class="dashboard-skeleton-copy" />
      </span>
    </div>
    <div v-else-if="error" class="recent-placeholder flex items-center justify-center text-state-error flex-1">{{ error }}</div>
    <div v-else-if="items.length === 0" class="recent-placeholder flex items-center justify-center text-muted-foreground text-sm flex-1">
      暂无文章
    </div>

    <!-- Small 尺寸：单行精简列表 -->
    <div v-else-if="sizePreset === 'small'" class="space-y-1.5 flex-1">
      <button
        v-for="item in displayedItems"
        :key="item.slug"
        class="w-full flex items-center justify-between py-1.5 px-2.5 rounded-xl hover:bg-white/5 text-left transition-all border border-transparent hover:border-white/5"
        @click="editPost(item)"
      >
        <div class="flex items-center gap-2.5 min-w-0 flex-1 pr-2">
          <span
            class="apple-status-dot"
            :class="item.status === 'published' ? 'published' : 'draft'"
            :aria-label="item.status === 'published' ? '已发布' : '草稿'"
          />
          <span class="text-foreground text-xs font-medium truncate">{{ item.title }}</span>
        </div>
        <span class="text-[10px] text-muted-foreground whitespace-nowrap">{{ formatRelative(item.updated_at) }}</span>
      </button>
    </div>

    <!-- Large 尺寸：富列表 + 编辑/预览快捷入口 -->
    <div v-else-if="sizePreset === 'large'" class="space-y-1 flex-1 overflow-y-auto pr-0.5">
      <div v-for="item in displayedItems" :key="item.slug" class="apple-list-item group">
        <button class="recent-post-main" type="button" @click="editPost(item)">
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2.5">
              <span
                class="apple-status-dot"
                :class="item.status === 'published' ? 'published' : 'draft'"
                :title="item.status === 'published' ? '已发布' : '草稿'"
              />
              <span class="text-foreground text-sm font-medium tracking-tight truncate">{{ item.title }}</span>
            </div>
            <div class="flex items-center gap-2 mt-1.5 text-xs text-muted-foreground">
              <span v-if="item.category" class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded-md bg-white/[0.04] text-[11px] text-muted-foreground border border-white/5">
                <Icon icon="ri:folder-3-line" class="w-3 h-3 text-primary" />
                {{ item.category }}
              </span>
              <span class="flex items-center gap-1 text-[11px]">
                <Icon icon="ri:time-line" class="w-3 h-3" />
                {{ formatRelative(item.updated_at) }}
              </span>
            </div>
          </div>
        </button>
        <div class="recent-post-actions" aria-label="文章快捷操作">
          <button type="button" title="编辑" @click="editPost(item)">
            <Icon icon="ri:edit-line" />
          </button>
          <button type="button" title="前台预览" @click="previewPost(item)">
            <Icon icon="ri:external-link-line" />
          </button>
        </div>
      </div>
    </div>

    <!-- Medium 尺寸：标准富列表 -->
    <div v-else class="space-y-1 flex-1 overflow-y-auto pr-0.5">
      <button
        v-for="item in displayedItems"
        :key="item.slug"
        class="apple-list-item group"
        @click="editPost(item)"
      >
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2.5">
            <span
              class="apple-status-dot"
              :class="item.status === 'published' ? 'published' : 'draft'"
              :title="item.status === 'published' ? '已发布' : '草稿'"
            />
            <span class="text-foreground text-sm font-medium tracking-tight truncate">{{ item.title }}</span>
          </div>
          <div class="flex items-center gap-2 mt-1.5 text-xs text-muted-foreground">
            <span v-if="item.category" class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded-md bg-white/[0.04] text-[11px] text-muted-foreground border border-white/5">
              <Icon icon="ri:folder-3-line" class="w-3 h-3 text-primary" />
              {{ item.category }}
            </span>
            <span class="flex items-center gap-1 text-[11px]">
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
  min-height: 120px;
}
.recent-legend {
  display: inline-flex;
  align-items: center;
  gap: 0.8rem;
  font-size: 11px;
  color: var(--color-muted-foreground);
}
.recent-legend-item {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  white-space: nowrap;
}
.apple-card:is([data-bp="xs"], [data-bp="xxs"]) .recent-legend { display: none; }

.recent-post-main {
  display: flex;
  min-width: 0;
  flex: 1;
  color: inherit;
  font: inherit;
  text-align: left;
  background: transparent;
  border: 0;
  cursor: pointer;
}

.recent-post-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  opacity: 0;
  transition: opacity 150ms var(--ease-standard);
}

.apple-list-item:hover .recent-post-actions,
.apple-list-item:focus-within .recent-post-actions {
  opacity: 1;
}

.recent-post-actions button {
  width: 28px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid color-mix(in srgb, #fff 10%, transparent);
  border-radius: 50%;
  color: var(--color-muted-foreground);
  background: color-mix(in srgb, #fff 4%, transparent);
  backdrop-filter: blur(8px);
  cursor: pointer;
  transition: all var(--duration-fast) var(--ease-standard);
}

.recent-post-actions button:hover {
  color: var(--color-primary);
  border-color: var(--color-primary);
  background: color-mix(in srgb, var(--color-primary) 15%, transparent);
  transform: scale(1.05);
}

.recent-post-actions svg {
  width: 14px;
  height: 14px;
}
</style>
