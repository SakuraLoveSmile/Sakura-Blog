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
  // 同一档位盒子的可用高度固定：小档只放两条单行，中档四条，大档七条。
  if (sizePreset.value === "small") return items.value.slice(0, 2);
  if (sizePreset.value === "medium") return items.value.slice(0, 4);
  return items.value.slice(0, 7);
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
    <div class="flex items-center justify-between mb-3 recent-header">
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
      <span v-for="index in (sizePreset === 'small' ? 2 : sizePreset === 'medium' ? 4 : 5)" :key="index" class="dashboard-skeleton-row">
        <i class="dashboard-skeleton-avatar" />
        <i class="dashboard-skeleton-copy" />
      </span>
    </div>
    <div v-else-if="error" class="recent-placeholder flex items-center justify-center text-state-error flex-1">{{ error }}</div>
    <div v-else-if="items.length === 0" class="recent-placeholder flex items-center justify-center text-muted-foreground text-sm flex-1">
      暂无文章
    </div>

    <!-- Small 尺寸：单行精简列表 -->
    <div v-else-if="sizePreset === 'small'" class="recent-mini-list flex-1">
      <button
        v-for="item in displayedItems"
        :key="item.slug"
        class="recent-mini-row w-full flex items-center justify-between py-1.5 border-t border-border text-left transition-colors duration-150"
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
        <span class="text-[12px] text-muted-foreground whitespace-nowrap">{{ formatRelative(item.updated_at) }}</span>
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
              <span v-if="item.category" class="inline-flex items-center gap-1 text-[12px] text-muted-foreground">
                <Icon icon="ri:folder-3-line" class="w-3 h-3" />
                {{ item.category }}
              </span>
              <span class="flex items-center gap-1 text-[12px]">
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
            <span v-if="item.category" class="inline-flex items-center gap-1 text-[12px] text-muted-foreground">
              <Icon icon="ri:folder-3-line" class="w-3 h-3" />
              {{ item.category }}
            </span>
            <span class="flex items-center gap-1 text-[12px]">
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
/* 小档 107px 高：标题与两条单行记录必须正好放下，不留空间给 gap。 */
.recent-posts-small .recent-header {
  margin-bottom: 4px;
}
.recent-mini-list {
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}
.recent-posts-small .recent-mini-row {
  padding-block: 4px;
}
.recent-placeholder {
  min-height: 0;
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
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  color: var(--color-muted-foreground);
  background: transparent;
  cursor: pointer;
  transition: background-color var(--duration-fast) var(--ease-standard), border-color var(--duration-fast) var(--ease-standard), color var(--duration-fast) var(--ease-standard);
}

.recent-post-actions button:hover {
  color: var(--color-foreground);
  border-color: var(--color-border);
  background: var(--color-accent);
}

.recent-post-actions svg {
  width: 14px;
  height: 14px;
}
</style>
